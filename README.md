# gitmate

> Your Git workflow, with a brain. And a conscience.

[![release](https://img.shields.io/github/v/release/krishyogee/gitmate)](https://github.com/krishyogee/gitmate/releases)
[![ci](https://github.com/krishyogee/gitmate/actions/workflows/ci.yml/badge.svg)](https://github.com/krishyogee/gitmate/actions)
[![license](https://img.shields.io/github/license/krishyogee/gitmate)](LICENSE)

Most "AI for Git" tools are commit-message generators in a trenchcoat. gitmate isn't.

Under the hood it's a small swarm of agents running a ReAct loop (think → act → observe → refine), with an evaluator that scores every output, a memory layer that picks up on your style, and a permission gate that refuses to touch your filesystem without a green light. On top sits a Bubble Tea TUI so you can actually watch the thing work instead of staring at a blinking cursor.

You stay in charge. The agent does the typing.

## What it actually does

Three things, well:

1. **Ships code.** `gitmate ship` reads your diff, drafts a commit message, scores it, refines if weak, asks before committing, optionally opens a PR.
2. **Resolves merge hell.** `gitmate resolve <file>` walks each conflict block, explains what each side was trying to do, proposes a patch with a confidence score, and waits for you.
3. **Predicts pain.** `gitmate check` scans hotspots and overlap zones with main so you know what's about to explode before it does.

Plus a TUI dashboard if you'd rather click than type.

## A taste

### `gitmate` (bare, opens dashboard)

```
gitmate 0.2.3
branch feature-payments (↑3 ↓1)   base main   risk MEDIUM   overlap 2

  [s]  ship        commit + optional PR
▸ [y]  sync        fetch + integrate origin + base
  [c]  check       predict merge pain
  [t]  status      branch + overlap + risk
  [x]  explain     explain a diff
  [p]  push        push branch to origin
  [m]  metrics     approval rate + latency
  [i]  init        configure provider + key
  [f]  config      show effective config

↑↓/jk navigate · enter select · letter shortcut · q quit
```

### `gitmate ship`

```
⠼ drafting commit message
✓ staged diff ready (2433 chars)
✓ draft scored 0.85
· refine kept original (0.72 vs 0.85)

╭ gitmate · action required ─────────────────────╮
│  action  git_commit                            │
│  risk    EXECUTE                               │
│  why     commit staged changes with this msg   │
╰────────────────────────────────────────────────╯
─── input ───
  feat(tui): add Bubble Tea dashboard + live stream

  - dashboard renders repo state + risk + action menu
  - lipgloss-styled approval card replaces ASCII box
  - stream wrapper for ship/sync long-running ops
─────────────

[y] yes  [a] allow session  [p] preview  [e] edit  [n] no  [?] explain  ›
› y
✓ commit landed
[main 97f57d7] feat(tui): add Bubble Tea dashboard + live stream
```

### `gitmate resolve src/payment.go`

```
found 1 conflict block in src/payment.go

=== Block 1/1 (line 42, complexity=complex) ===
--- ours ---
return retryWithBackoff(submit(payload))
--- theirs ---
return client.SubmitPayment(ctx, payload)

--- analysis ---
ours intent:    Adds retry logic to payment submission
theirs intent:  Refactors to new payments client
conflict type:  refactor_vs_feature
strategy:       combine_both
confidence:     0.72
rationale:      Keep new client; reapply retry wrapper around it
risk:           Method signatures changed. Run tests before continuing.

╭ gitmate · action required ─────────────────────╮
│  action  resolve_conflict                      │
│  risk    PROPOSE                               │
│  why     apply candidate patch to this block   │
╰────────────────────────────────────────────────╯
```

## Install

**Homebrew** (macOS / Linux):

```sh
brew install krishyogee/tap/gitmate
```

**Go:**

```sh
go install github.com/krishyogee/gitmate@latest
```

**Pre-built binary:** grab one from [Releases](https://github.com/krishyogee/gitmate/releases) and drop it on your `PATH`.

## Configure

The fast path:

```sh
gitmate init
```

Walks you through provider (anthropic / openai / groq), API key, and which shell rc to update. Writes:

- `~/.gitmate/config.json` for provider + model
- `~/.gitmate/credentials.json` for the API key (mode `0600`, survives shell restarts)
- An `export <PROVIDER>_API_KEY=...` line in your rc as a fallback

No `source` needed. It just works in new shells.

Prefer to set things by hand? Set the env vars yourself:

```sh
export ANTHROPIC_API_KEY=...   # primary (default)
export OPENAI_API_KEY=...      # fallback
export GROQ_API_KEY=...        # fallback
```

### Knobs

| Var | Purpose |
|-----|---------|
| `GITMATE_PROVIDER` | force `anthropic` / `openai` / `groq` |
| `GITMATE_PLANNING_MODEL` | model for reasoning |
| `GITMATE_DRAFTING_MODEL` | model for fast text gen |
| `GITMATE_FALLBACK_MODEL` | fallback model |
| `GITMATE_TEST_COMMAND` | test command for `run_tests` |
| `GITMATE_LINT_COMMAND` | lint command for `run_lint` |
| `GITMATE_DEFAULT_BASE` | default base branch |

### Config layering

Highest wins:

1. CLI flags (`--auto`, `--dry-run`, `--base`)
2. Env vars (`GITMATE_*`)
3. Repo-local: `<repo>/.gitmate/config.json`
4. Global: `~/.gitmate/config.json`
5. Defaults

`gitmate config` prints the effective merged config and where each value came from.

### Editing config without opening a JSON file like an animal

```sh
gitmate config set <key> <value>            # writes repo config (default)
gitmate config set <key> <value> --global   # writes ~/.gitmate/config.json
gitmate config get <key>                    # effective value (after layering)
gitmate config unset <key>                  # remove from file
```

```sh
# change base branch for this repo only
gitmate config set defaultBase develop

# switch to merge globally
gitmate config set syncMode merge --global

# nested keys via dot
gitmate config set models.drafting gpt-4o-mini
gitmate config set guardrails.maxLoopSteps 8

# values auto-parsed: bools, ints, floats, JSON arrays/objects
gitmate config set autoStash false
gitmate config set guardrails.minConfidenceToApply 0.7
gitmate config set guardrails.highRiskPatterns '["auth/","secrets/"]'
```

`gitmate config set` creates `<repo>/.gitmate/config.json` if it's missing.

## Commands

| Command | What it does |
|---------|--------------|
| `gitmate` | Open the TUI dashboard (bare invocation, TTY only) |
| `gitmate init` | Interactive provider + API key setup |
| `gitmate ship` | Diff → commit msg → score → refine → approve → commit → optional PR |
| `gitmate sync [base]` | Fetch + integrate `origin/<branch>` + integrate `<base>`; pause on conflict |
| `gitmate push` | Push current branch to origin (with approval) |
| `gitmate resolve <file>` | Explain each conflict block, propose patch, approve, write |
| `gitmate check` | Predict merge pain — overlap, hotspots, risk score |
| `gitmate status` | Branch state + overlap zones + risk indicator |
| `gitmate explain [file]` | Plain-language explanation of a diff |
| `gitmate metrics` | Approval rate, edit rate, latency, score distribution |
| `gitmate config` | Show effective config + paths |
| `gitmate config set/get/unset` | Edit repo or global config (`--global`) without hand-editing JSON |
| `gitmate version` | Print version, commit, build date |

Global flags: `--auto` (skip approvals — use sparingly), `--dry-run`, `--base`, `--no-ai`, `-v`.

## How it works

```
USER COMMAND  ──or──  TUI DASHBOARD (Bubble Tea)
         │                       │
         └───────────┬───────────┘
                     ▼
ORCHESTRATOR (ReAct loop, max 6 steps)
    ┌─────┴──────────────────────────────────────────┐
    │                                                │
    ▼                                                ▼
PLANNER (LLM)                              APPROVAL GATE
  - Receives: task + state + memory          - Tiers: READ / ADVISE / PROPOSE / EXECUTE
  - Returns: {thought, action, input}        - Lipgloss approval card
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

The orchestrator can self-correct. If the evaluator scores a draft below 0.4 it rotates models; between 0.4 and 0.8 it tries to refine; above 0.8 it ships. You see all of this stream live.

## The permission gate

Four tiers, escalating:

| Tier | Examples | Default |
|------|----------|---------|
| READ | `git_diff`, `git_status`, `parse_conflicts` | auto-allow |
| ADVISE | `generate_commit`, `explain_conflict`, `explain_diff` | ask once per session |
| PROPOSE | `create_pr`, `resolve_conflict` | always ask |
| EXECUTE | `git_commit`, `git_push`, `run_tests`, `write_file` | always ask, every time |

Approval card hotkeys: `y` yes · `a` allow session · `p` preview · `e` edit in `$EDITOR` · `n` no · `?` explain.

## What it will never do

These aren't config flags. They're hard-coded.

1. Auto-apply an AI-generated patch without approval
2. Auto-commit after a conflict resolution
3. Skip the diff preview before any file write
4. Take away your manual escape hatch (you can always abort)
5. Hide confidence and risk on conflict resolutions
6. Under-warn when in doubt — over-warning is the default
7. Send secrets to the LLM. The redactor in `internal/ai/compress.go` strips API keys, private keys, and AWS / Slack / GitHub / OpenAI token patterns before any prompt leaves the box
8. Treat `auth/`, `schema/`, `migrations/` as routine — those always escalate to Complex routing
9. Forget to recommend tests after applying a patch
10. Skip the audit log. Even denials are written

## Observability

Every AI call and every approval lands in `~/.gitmate/ai-log.jsonl`. Tail it, grep it, ship it to your favorite log eater.

```json
{"timestamp":"2026-05-01T12:00:00Z","provider":"anthropic","model":"claude-haiku-4-5-20251001","task":"commit_draft","input_tokens":312,"output_tokens":48,"latency_ms":612,"success":true}
{"timestamp":"2026-05-01T12:00:01Z","task":"ship","action":"generate_commit","score":0.85,"success":true}
{"timestamp":"2026-05-01T12:00:08Z","action":"git_commit","user_action":"approved","success":true}
```

`gitmate metrics` rolls it up:

```json
{
  "total_calls": 142,
  "success_rate": 0.972,
  "approval_rate": 0.91,
  "edit_rate": 0.07,
  "fallback_rate": 0.03,
  "avg_latency_ms": 814.5,
  "avg_score": 0.87,
  "score_by_action": { "generate_commit": 0.83, "refine_commit": 0.91 },
  "approval_by_action": { "git_commit": 0.95, "create_pr": 0.84 }
}
```

If your approval rate is high and your edit rate is low, gitmate has learned your style. If they're not, the memory layer hasn't caught up yet — give it a week.

## Why it's built this way

- **Subprocess `git` and `gh`, never reimplemented.** Semantics match exactly what you already know. No surprises.
- **JSONL over stdout.** Every AI call is auditable. Post-hoc analysis is grep + jq.
- **ReAct, not chains.** Agents that can't self-correct are just expensive autocomplete.
- **Evaluator before approval gate.** Catch slop without bothering you.
- **Memory as context, not training.** Your style biases the model, it doesn't constrain it.
- **Approval as a tiny form, not free-text Y/n.** Lipgloss card with structured fields means you read what you're approving.
- **TTY-aware TUI.** Auto-disables in CI and pipes. Falls back to plain output.
- **Persistent credentials at `0600`.** Survive shell restarts. Env vars still win.
- **One static binary.** Go. No Python, no Node, no Docker.

## Repo layout

```
gitmate/
├── cmd/                 # cobra CLI entry points
│   ├── root.go          # rootCmd + dashboard launch + shared App
│   ├── ship.go          # ship: diff→draft→score→refine→approve→commit→PR
│   ├── sync.go          # sync: fetch → integrate origin/<branch> → integrate base
│   ├── resolve.go       # resolve: per-block explain + approve + write
│   ├── check.go         # check: overlap + hotspot + risk
│   ├── status.go        # status: branch + overlap + risk
│   ├── explain.go       # explain: AI diff summary
│   ├── push.go          # push: branch → origin (with approval)
│   ├── init.go          # interactive setup (provider + key + rc)
│   ├── metrics.go       # log aggregation
│   ├── config.go        # show effective config
│   └── util.go          # helpers (scoreLabel, json)
├── internal/
│   ├── agent/           # ReAct loop: orchestrator, planner, executor, evaluator
│   ├── ai/              # multi-provider client + prompts + compression + redaction
│   ├── approval/        # permission tiers + manager + UI
│   ├── conflict/        # parser + classifier + AI explainer
│   ├── config/          # layered config + credentials.json
│   ├── memory/          # session (in-process) + store (~/.gitmate/memory.json)
│   ├── observability/   # JSONL logger + metrics computer
│   ├── tools/           # git, conflict, pr, shell tool implementations
│   └── tui/             # Bubble Tea dashboard + lipgloss styles + live stream
├── .github/workflows/
│   ├── ci.yml           # build + vet + test on PRs
│   └── release.yml      # GoReleaser on tag push
├── .goreleaser.yaml
├── main.go
└── go.mod
```

## Build from source

```sh
git clone https://github.com/krishyogee/gitmate.git
cd gitmate
go build -o gitmate .
go test ./...
```

Releases: tag a `vX.Y.Z`, push it. GoReleaser handles the rest. Then bump the [Homebrew tap formula](https://github.com/krishyogee/homebrew-tap/blob/main/Formula/gitmate.rb).

## Trivia: gitmate committed itself

The very first commit on this repo (`d6dc1d1` — `feat(cmd): add initial CLI implementation with agent and AI`) was written and committed **by gitmate**, into gitmate's own repo, before it had ever been released anywhere.

The bootstrap:

```sh
git init
go build -o gitmate .
git add .
./gitmate ship --no-pr
```

What ran inside that single command:

```
[git_diff]        ─→ 42 files, 3750 lines staged → compressed to file-level summary
[generate_commit] ─→ groq llama-3.3-70b drafted message
[evaluator]       ─→ score 1.00 (conventional, ≤72 chars, body, non-generic)
[approval card]   ─→ git_commit · EXECUTE → user approved
[git_commit]      ─→ root-commit d6dc1d1 created
```

It worked because `git diff --staged` works without a `HEAD`, and `git commit` happily creates a root-commit when no parent exists. gitmate doesn't assume the repo has any history — it just needs something staged.

Every commit and push to this repo since has gone through `gitmate ship` + `gitmate push`. Self-hosted from commit zero.

## License

MIT. See [LICENSE](LICENSE). Use it, fork it, ship it.
