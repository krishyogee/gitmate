# gitmate

Multi-agent CLI that wraps Git with an evaluator-driven AI layer.

> Less Git thinking, more shipping — with approvals where it matters.

## What it is

gitmate is **not** a Git wrapper or a chatbot. It is a multi-agent system where:

- An orchestrator runs a ReAct loop (think → act → observe → refine)
- Sub-agents handle commits, PRs, conflict resolution, risk analysis
- A memory layer learns user and repo patterns across sessions
- An evaluator validates every AI output before it touches the filesystem
- A permission system gates every consequential action (READ / ADVISE / PROPOSE / EXECUTE)

## Install

```sh
go build -o gitmate .
mv gitmate /usr/local/bin/   # or anywhere on PATH
```

## Configure

Set at least one provider API key:

```sh
export ANTHROPIC_API_KEY=...   # primary (default)
export OPENAI_API_KEY=...      # fallback
export GROQ_API_KEY=...        # fallback
```

Optional env overrides:

| Var | Purpose |
|-----|---------|
| `GITMATE_PROVIDER` | force `anthropic` / `openai` / `groq` |
| `GITMATE_PLANNING_MODEL` | model for reasoning |
| `GITMATE_DRAFTING_MODEL` | model for fast text gen |
| `GITMATE_FALLBACK_MODEL` | fallback model |
| `GITMATE_TEST_COMMAND` | test command for `run_tests` |
| `GITMATE_LINT_COMMAND` | lint command for `run_lint` |
| `GITMATE_DEFAULT_BASE` | default base branch |

Config files (layered, repo > global):

- `~/.gitmate/config.json`
- `<repo>/.gitmate/config.json`

Run `gitmate config` to see effective config.

## Commands

| Command | What it does |
|---------|--------------|
| `gitmate ship` | Diff → commit msg → score → refine → approve → commit → optional PR |
| `gitmate sync [base]` | Fetch + rebase (or merge); pause on conflict |
| `gitmate resolve <file>` | Explain each conflict block, propose patch, approve, write |
| `gitmate check` | Predict merge pain — overlap, hotspots, risk score |
| `gitmate status` | Branch state + overlap zones + risk indicator |
| `gitmate explain [file]` | Plain-language explanation of a diff |
| `gitmate metrics` | Approval rate, edit rate, latency, score distribution |
| `gitmate config` | Show effective config + paths |

Global flags: `--auto` (skip approvals), `--dry-run`, `--base`, `--no-ai`, `-v`.

## Architecture

```
USER COMMAND (gitmate ship / sync / resolve / check)
         │
         ▼
ORCHESTRATOR (ReAct loop, max 6 steps)
    ┌─────┴──────────────────────────────────────────┐
    │                                                │
    ▼                                                ▼
PLANNER (LLM)                              APPROVAL GATE
  - Receives: task + state + memory          - Tiers: READ / ADVISE / PROPOSE / EXECUTE
  - Returns: {thought, action, input}        - Shortcuts: y / a / p / e / n / ?
  - Routes: fast (drafting) / strong (planning)
    │
    ▼
EXECUTOR (Tool dispatch)
  git_diff | git_commit | parse_conflicts
  resolve_conflict | create_pr | run_tests
    │
    ▼
EVALUATOR (Score output)
  CommitEvaluator | ConflictEvaluator | ExplainEvaluator | RiskEvaluator
  Score ≥ 0.8 → stop | 0.4–0.8 → refine | < 0.4 → rotate model
    │
    ▼
MEMORY
  Session: attempts, last output, repo context
  Long-term: ~/.gitmate/memory.json (commit style, hot files, approvals)
    │
    ▼
OBSERVABILITY
  ~/.gitmate/ai-log.jsonl
  Every step: action, model, tokens, latency, score, user_action
```

## Permission tiers

| Tier | Examples | Default behavior |
|------|----------|------------------|
| READ | `git_diff`, `git_status`, `parse_conflicts` | auto-allow |
| ADVISE | `generate_commit`, `explain_conflict`, `explain_diff` | ask once per session |
| PROPOSE | `create_pr`, `resolve_conflict` | always ask |
| EXECUTE | `git_commit`, `git_push`, `run_tests`, `write_file` | always ask, every time |

## Safety rules (hard-coded, non-configurable)

1. Never auto-apply any AI-generated patch without approval
2. Never auto-commit after conflict resolution
3. Always show diff preview before any file write
4. Always preserve manual escape hatches (user can always abort)
5. Always label confidence and risk on conflict resolutions
6. Prefer over-warn over under-warn on risk scoring
7. Never send secrets or `.env` contents to the LLM (regex redaction in `internal/ai/compress.go`)
8. High-risk file patterns (`auth/`, `schema/`, `migrations/`) always trigger Complex routing
9. After any AI-applied patch, recommend running tests
10. Audit log always written, even on user denial

## Sample log entry

```json
{"timestamp":"2026-04-30T12:00:00Z","provider":"anthropic","model":"claude-haiku-4-5-20251001","task":"commit_draft","input_tokens":312,"output_tokens":48,"latency_ms":612,"success":true}
{"timestamp":"2026-04-30T12:00:01Z","task":"ship","action":"generate_commit","score":0.85,"success":true}
{"timestamp":"2026-04-30T12:00:08Z","action":"git_commit","user_action":"approved","success":true}
```

## Design decisions

- **Subprocess Git, never reimplemented** — wraps `git` and `gh` so semantics match what the user already knows.
- **JSONL logs over stdout** — every AI call is auditable and post-hoc analyzable.
- **ReAct over chains** — agents can self-correct; chains can't.
- **Evaluator before approval gate** — catch low-quality output without bothering the user.
- **Memory injected as context, not training** — repo style is biasing, not constraining.
- **Approval card UI** — every consequential action is a tiny form, not a free-text Y/n.
