# gitmate

Multi-agent AI CLI for Git workflows with approval gates and a Bubble Tea TUI.

> Less Git thinking, more shipping — with approvals where it matters.

[![release](https://img.shields.io/github/v/release/krishyogee/gitmate)](https://github.com/krishyogee/gitmate/releases)
[![ci](https://github.com/krishyogee/gitmate/actions/workflows/ci.yml/badge.svg)](https://github.com/krishyogee/gitmate/actions)
[![license](https://img.shields.io/github/license/krishyogee/gitmate)](LICENSE)

---

## What it is

gitmate is **not** a Git wrapper or a chatbot. It is a multi-agent system where:

- An orchestrator runs a ReAct loop (think → act → observe → refine)
- Sub-agents handle commits, PRs, conflict resolution, risk analysis
- A memory layer learns user and repo patterns across sessions
- An evaluator validates every AI output before it touches the filesystem
- A permission system gates every consequential action (READ / ADVISE / PROPOSE / EXECUTE)
- A Bubble Tea + Lip Gloss TUI gives you a live dashboard, streamed agent steps, and rich approval cards

---

## CLI snapshots

### Dashboard (bare `gitmate`)

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

Pick a letter or `enter` → spawns the matching command in a fresh process with full stdio.

### Live agent stream (`gitmate ship`)

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

Spinner runs while AI is thinking. Final icon (`✓` / `✗` / `·`) marks each step.

### Conflict resolver (`gitmate resolve src/payment.go`)

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

---

## Install

### Homebrew (macOS / Linux)

```sh
brew install krishyogee/tap/gitmate
```

### `go install`

```sh
go install github.com/krishyogee/gitmate@latest
```

### Pre-built binary

Download from [Releases](https://github.com/krishyogee/gitmate/releases) and put on `PATH`.

---

## Configure

### Interactive setup (recommended)

```sh
gitmate init
```

Prompts for provider (anthropic / openai / groq), API key, shell rc. Writes:

- `~/.gitmate/config.json` — provider + model selection
- `~/.gitmate/credentials.json` — API key (mode `0600`, persisted across shells)
- `~/.zshrc` (or detected rc) — `export <PROVIDER>_API_KEY=...` as fallback

Persistent — no `source ~/.zshrc` needed.

### Manual

```sh
export ANTHROPIC_API_KEY=...   # primary (default)
export OPENAI_API_KEY=...      # fallback
export GROQ_API_KEY=...        # fallback
```

### Env overrides

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

Priority (highest first):
1. CLI flags (`--auto`, `--dry-run`, `--base`)
2. Env vars (`GITMATE_*`)
3. Repo-local: `<repo>/.gitmate/config.json`
4. Global: `~/.gitmate/config.json`
5. Defaults

`gitmate config` prints the effective merged config + all paths.

---

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
| `gitmate version` | Print version, commit, build date |

Global flags: `--auto` (skip approvals), `--dry-run`, `--base`, `--no-ai`, `-v`.

---

## Architecture

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

---

## Permission tiers

| Tier | Examples | Default behavior |
|------|----------|------------------|
| READ | `git_diff`, `git_status`, `parse_conflicts` | auto-allow |
| ADVISE | `generate_commit`, `explain_conflict`, `explain_diff` | ask once per session |
| PROPOSE | `create_pr`, `resolve_conflict` | always ask |
| EXECUTE | `git_commit`, `git_push`, `run_tests`, `write_file` | always ask, every time |

Approval card shortcuts: `y` yes · `a` allow session · `p` preview · `e` edit in `$EDITOR` · `n` no · `?` explain.

---

## Safety rules (hard-coded, non-configurable)

1. Never auto-apply any AI-generated patch without approval
2. Never auto-commit after conflict resolution
3. Always show diff preview before any file write
4. Always preserve manual escape hatches (user can always abort)
5. Always label confidence and risk on conflict resolutions
6. Prefer over-warn over under-warn on risk scoring
7. Never send secrets or `.env` contents to the LLM (regex redaction in `internal/ai/compress.go` — strips api_key / private key / AWS / Slack / GitHub / OpenAI token patterns)
8. High-risk file patterns (`auth/`, `schema/`, `migrations/`) always trigger Complex routing
9. After any AI-applied patch, recommend running tests
10. Audit log always written, even on user denial

---

## Observability

Every AI call + every approval decision lands in `~/.gitmate/ai-log.jsonl`:

```json
{"timestamp":"2026-05-01T12:00:00Z","provider":"anthropic","model":"claude-haiku-4-5-20251001","task":"commit_draft","input_tokens":312,"output_tokens":48,"latency_ms":612,"success":true}
{"timestamp":"2026-05-01T12:00:01Z","task":"ship","action":"generate_commit","score":0.85,"success":true}
{"timestamp":"2026-05-01T12:00:08Z","action":"git_commit","user_action":"approved","success":true}
```

`gitmate metrics` aggregates the log:

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

---

## Design decisions

- **Subprocess Git, never reimplemented** — wraps `git` and `gh` so semantics match what the user already knows.
- **JSONL logs over stdout** — every AI call is auditable and post-hoc analyzable.
- **ReAct over chains** — agents can self-correct; chains can't.
- **Evaluator before approval gate** — catch low-quality output without bothering the user.
- **Memory injected as context, not training** — repo style is biasing, not constraining.
- **Lipgloss approval card** — every consequential action is a tiny form, not a free-text Y/n.
- **Bubble Tea dashboard** — bare `gitmate` opens an interactive console; selecting an action `exec`s a fresh subprocess so stdio stays clean.
- **TTY-aware** — TUI auto-disables in CI, pipes, and non-terminal contexts; falls back to plain output.
- **Persistent credentials** — keys stored in `~/.gitmate/credentials.json` (mode `0600`); survive shell restarts; env vars still take precedence.
- **Single binary, no runtime** — Go, statically linked, no Python / Node / Docker.

---

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

---

## Build

```sh
git clone https://github.com/krishyogee/gitmate.git
cd gitmate
go build -o gitmate .
go test ./...
```

Release flow: tag a `vX.Y.Z` and push — GitHub Action runs GoReleaser, attaches binaries to a Release, then bump the [Homebrew tap formula](https://github.com/krishyogee/homebrew-tap/blob/main/Formula/gitmate.rb).

---

## License

MIT — see [LICENSE](LICENSE).
