# Product Requirements Document: `fkn` — Flow Kit Neu

**Version:** 0.4.1  
**Status:** Ready to Build  
**Author:** TBD  
**Last Updated:** 2026-03-21  
**Changelog:** v0.4.1 — added exit code contract table, `fkn list --mcp`, global `env_file`, `{{os}}` prompt variable, `fkn init` adds `.fkn/` to `.gitignore`, tightened truncation marker wording to address agent file-end assumption risk, parallel `continue_on_error` added to open questions.

---

## Table of Contents

1. [Overview](#1-overview)
2. [Problem Statement](#2-problem-statement)
3. [Goals and Non-Goals](#3-goals-and-non-goals)
4. [Target Users](#4-target-users)
5. [Core Concepts](#5-core-concepts)
6. [Feature Specification](#6-feature-specification)
7. [JSON Output Contracts](#7-json-output-contracts)
8. [Configuration Schema](#8-configuration-schema)
9. [CLI Reference](#9-cli-reference)
10. [Architecture](#10-architecture)
11. [Error Handling and UX](#11-error-handling-and-ux)
12. [Installation and Distribution](#12-installation-and-distribution)
13. [Open Questions](#13-open-questions)
14. [Appendix: Example Config Files](#14-appendix-example-config-files)

---

## 1. Overview

`fkn` is a single interface that exposes how a repository is built, checked, run, and governed.

**`fkn.yaml` is the operational API of the repo.** The task manifest, MCP tool surface, validation pipeline, context document, and agent prompts are all derived from one file. There is no second config, no separate manifest, no generated code to keep in sync.

`fkn` is a repo-agnostic, YAML-configured task runner and AI agent integration tool. It simplifies complex or frequently forgotten commands, orchestrates multi-step workflows with optional parallelism, and makes repository capabilities available to AI coding agents — as callable MCP tools, as structured JSON output, and as bounded context documents.

The binary is named `fkn`. All configuration lives in `fkn.yaml` at the repo root.

### The Three Tests

Every feature in `fkn` must pass these tests:

1. **Can a human understand it in 60 seconds?** If explaining it requires a taxonomy, it is too diffuse.
2. **Can an agent use it without scraping prose?** Structured output matters more than clever markdown.
3. **Does it reinforce the same mental model?** The model is: *this is how the repo works.* Not: *here are some useful tools.*

### Status and Exit Code Contract

Formalised here once; applied consistently throughout. This is a v1 stability commitment.

| State | Terminal | JSON `status` | Process exit code |
|---|---|---|---|
| Success | `PASS` | `pass` | `0` |
| Command failed | `FAIL` | `fail` | `1+` (actual exit code) |
| Binary not found | `FAIL` | `fail` | `127` (standard) |
| Cancelled (parallel) | `CANCELLED` | `cancelled` | `130` (standard) |
| Timeout | `TIMEOUT` | `timeout` | `124` (standard) |

There is no `SKIP` or `skip`. A missing binary is a broken environment — it is a `fail` with exit code `127`, not an intentional non-execution.

---

## 2. Problem Statement

Modern software projects involve a growing number of tools, CLIs, and services — each with their own flags, conventions, and invocation patterns. Developers waste cognitive load remembering exact build commands, multi-step deployment sequences, and the specific incantation to stand up a local environment.

This problem is compounded in AI-assisted development:

- Coding agents need structured, unambiguous context about how the repo works
- There is no standard way to expose repo capabilities to an agent as callable tools
- Agent prompts describing repository conventions are written ad-hoc and discarded

`fkn` addresses all of the above with a single config file and a single binary.

---

## 3. Goals and Non-Goals

### Goals

- Provide a simple, memorable CLI for running repo-defined tasks
- Support sequential and parallel multi-step task pipelines
- Be language- and framework-agnostic; work in any repo
- Expose tasks as MCP tools for AI coding agent consumption
- Generate bounded, high-signal context documents for LLM consumption
- Define stable JSON output contracts for agent-facing commands
- Keep configuration simple: single-file YAML with obvious structure
- Distribute as a single static binary (Go)

### Non-Goals

- Not a replacement for CI/CD systems (GitHub Actions, CircleCI)
- Not a build system with dependency graphs or incremental builds
- Not a secrets manager
- Not opinionated about any language, framework, or cloud provider
- Not a package manager
- Not a git helper
- Not responsible for portability of task commands (see [Section 6.1](#61-task-runner))

---

## 4. Target Users

### Primary: Solo/Small-Team Developers

Engineers who work across multiple repos with different toolchains and want a single, consistent interface. Comfortable with YAML and the command line. May be using AI coding agents as a core part of their workflow.

### Secondary: Team Leads / Platform Engineers

Engineers who want to standardise how tasks are invoked across a team. They define `fkn.yaml` and commit it alongside code, reducing onboarding friction and eliminating "how do I run this?" questions.

### Tertiary: AI Coding Agents

MCP-compatible agents consuming the `fkn serve` endpoint. From the agent's perspective, `fkn` is a set of callable tools that map directly to how the repo works.

---

## 5. Core Concepts

### Task

The fundamental unit. A task has a name, a description, and either a single `cmd` string or a list of `steps`. Steps may reference other tasks by name or be inline shell commands. Tasks may be composed into pipelines using `steps`.

### Guard

A validation-specialised task pipeline with standardised reporting semantics. A guard is not a second orchestration system — it is a step list that always runs to completion (reporting each step result individually) regardless of individual step failures. Where a task pipeline fails fast, a guard reports everything.

### Context

A configurable, explicitly bounded snapshot of the repository for LLM consumption. Default output is short and high-signal. Agent mode (`--agent`) is richer but still bounded by explicit per-section caps. `fkn context` is not a generic repo summariser; it exists to answer: *what does an agent need to know to work here right now?*

### Scopes

Named lists of paths declared in `fkn.yaml`. Nothing more. Used to constrain what files an agent should touch. Tasks may declare an associated scope by name.

### Prompt

A named text template stored in `fkn.yaml` with a fixed, minimal set of interpolation variables. Your prompting strategy lives in the repo, versioned alongside the code.

---

## 6. Feature Specification

---

### 6.1 Task Runner

**Priority:** P0 — Core feature.

Define named tasks in `fkn.yaml`, run them with `fkn <task-name>`.

**Cross-platform note:** `fkn` is cross-platform, but task commands are whatever the repo author writes. They execute via the system shell (`$SHELL` on Unix, `cmd.exe` on Windows) and are not interpreted or transformed by `fkn`. Portability of task commands is the repo author's responsibility. Repos intended for cross-platform use should define portable commands or provide OS-specific task variants. A `check-env` task that verifies required tools are available is a recommended convention for repos with complex toolchains.

**Requirements:**

- Tasks support a single `cmd` (shell string) or a list of `steps`; not both
- Steps may be a reference to another named task or an inline shell command string
- `desc` field is required; shown in help output and MCP manifest
- `env` map of key-value pairs injected as environment variables for this task
- `dir` field sets working directory relative to repo root (default: repo root)
- Task output is streamed in real time, not buffered
- Non-zero exit codes and missing binaries are both `fail`; see exit code table in [Section 1](#1-overview)
- `timeout` field (e.g. `"5m"`) kills the process after the specified duration; step status is `timeout`, exit code `124`
- `agent: false` excludes a task from the MCP manifest (default: `true`)
- `scope` field associates a named scope with this task, used in `fkn context --agent`

**Pipeline semantics (`continue_on_error`):**

`continue_on_error` applies **only to sequential pipelines** in v1. Parallel pipelines always fail fast in v1.

| Pipeline type | Default behaviour | `continue_on_error: true` |
|---|---|---|
| Sequential | Halt on first failure | Run all steps; overall exit non-zero if any fail |
| Parallel | Cancel remaining on first failure | Not supported in v1; ignored with a warning |

---

### 6.2 Pipeline Execution (Sequential and Parallel)

**Priority:** P0 — Core feature.

**Requirements:**

- A task with `steps` and no `parallel` flag executes steps in order
- A task with `parallel: true` executes all steps concurrently
- Parallel step output is prefixed with the step name to distinguish interleaved output
- A parallel pipeline fails if any step fails; remaining in-progress steps are `cancelled` (exit code `130`)
- `continue_on_error: true` on a parallel task is accepted in config but ignored at runtime with a printed warning
- Step results in `--json` output include index, `resolved_cmd`, status, and timestamps (see [Section 7](#7-json-output-contracts))

---

### 6.3 MCP Server Mode (`fkn serve`)

**Priority:** P0 — Killer feature.

Exposes all defined tasks as callable MCP tools. `fkn.yaml` becomes the tool manifest.

**Requirements:**

- `fkn serve` starts an MCP server; default transport is **stdio** (standard for local agent use)
- `fkn serve --http --port 8080` starts an HTTP+SSE server; intended for local team use, not public hosting
- Each task where `agent: true` is exposed as an MCP tool; `desc` is the tool description
- The MCP tool manifest is auto-generated from `fkn.yaml`; no separate file required
- Tool responses include stdout, stderr, exit code, and duration
- **Auth (HTTP mode):** Token is read from the environment variable named in `serve.token_env` (default: `FKN_MCP_TOKEN`). If the env var is unset, HTTP mode starts unauthenticated with a warning. Tokens are never stored in `fkn.yaml`

**MCP Tool Schema (per task):**

```json
{
  "name": "wheel",
  "description": "Build Python distribution wheel",
  "inputSchema": {
    "type": "object",
    "properties": {
      "env": {
        "type": "object",
        "description": "Optional env var overrides for this invocation"
      },
      "dry_run": {
        "type": "boolean",
        "description": "Print the command without executing it"
      }
    }
  }
}
```

---

### 6.4 Context and Agent Handoff (`fkn context`)

**Priority:** P1.

Generates a structured, markdown-formatted repository document for LLM consumption. A single command with flags for depth. Output is always bounded; the default is short and high-signal.

**Design constraints:**

- Default output must be brief enough to read in one scroll
- Agent mode (`--agent`) may include more, but is still bounded by explicit per-section caps
- Truncation is at the section level. Every truncated section must include a marker in the form `[Lines 101–500 omitted]` at the exact point of truncation. This is a hard requirement: without it, an agent reading a truncated file may assume the file ends at the truncated line and attempt to append code that already exists
- `--max-tokens` is a global budget applied *after* per-section caps; it does not relax them

**Configurable sections:**

| Section | Default | Cap (configurable via `caps`) |
|---|---|---|
| `file_tree` | ✅ enabled | Max 200 entries |
| `files` (verbatim) | none configured | Max 5 files, 100 lines each |
| `git_log` | 10 lines | Max 30 lines |
| `git_diff` | ❌ disabled | Max 200 lines |
| `dependencies` | ✅ auto-detect | Max 100 lines per file |
| `todos` | ❌ disabled | Max 20 entries |
| `agent_files` | none configured | Max 500 lines each |

**Agent mode** (`--agent --task <n>`):

Adds to the standard context:
- Active task name and description
- Associated scope — only included when the task has an explicit `scope` field pointing to a named scope; no inference from task name or prefix matching
- Current git diff (bounded by `caps.git_diff`)
- Last cached guard output (`.fkn/last-guard.json`) if it exists

**Flags:**

```bash
fkn context                            # Default: brief, high-signal summary
fkn context --agent --task <n>         # Full agent handoff briefing
fkn context --out <file>               # Write to file
fkn context --copy                     # Copy to clipboard
fkn context --max-tokens <n>           # Global token budget (after per-section caps)
```

---

### 6.5 Guard / Validate (`fkn guard [name]`)

**Priority:** P1.

A validation-specialised pipeline with standardised reporting. A guard always runs all steps regardless of failures and produces a consistent, structured report (see [Section 7](#7-json-output-contracts)).

Guards answer: *did the agent break anything?* Their output is designed to be fed directly back to an agent.

**Requirements:**

- Guards are defined under `guards` in `fkn.yaml` as named step lists
- `fkn guard` runs the default guard; `fkn guard <n>` runs a named guard
- All steps always run; no step is cancelled because a prior step failed
- A missing binary is `fail` with exit code `127`
- Output: one line per step with `PASS`/`FAIL` and duration; failing step stderr appended below
- Exit code is non-zero if any step fails
- `--json` outputs the structured report (schema in [Section 7](#7-json-output-contracts))
- Last guard result is cached to `.fkn/last-guard.json`; only `stderr` per step is cached (stdout is noise for validation reporting)

**Example output:**

```
[fkn guard: default]

  lint    PASS   1.2s
  test    FAIL   8.4s
  build   PASS   3.1s

FAILED in 12.7s

--- test stderr ---
FAILED tests/test_auth.py::test_token_expiry
AssertionError: expected 401, got 200
```

`build` runs and passes despite `test` failing. This is the defining behavioural difference from a task pipeline.

---

### 6.6 Scopes (`fkn scope <n>`)

**Priority:** P1 — trivial to implement, useful for agents.

A named list of paths. Nothing more.

**Requirements:**

- Defined in `fkn.yaml` under `scopes` as named lists of file paths and globs
- `fkn scope <n>` prints the path list, one entry per line
- `--format prompt` wraps output as a natural language constraint sentence
- `--json` outputs a JSON object (see [Section 7.4](#74-fkn-scope-name---json))
- Referenced in prompt templates via `{{scope.<n>}}`
- Tasks associate a scope by name via the `scope` field: `scope: api`

---

### 6.7 Prompt Templates (`fkn prompt <n>`)

**Priority:** P2.

Named prompt templates stored in `fkn.yaml`. Versioned alongside the code.

Interpolation is intentionally minimal. No conditionals, no loops, no filters.

**Supported variables (fixed list, not extensible in v1):**

| Variable | Value |
|---|---|
| `{{git_branch}}` | Current branch name |
| `{{git_diff}}` | Current uncommitted diff (bounded by `caps.git_diff`) |
| `{{git_log}}` | Recent git log (last 10 commits) |
| `{{scope.<n>}}` | Inline scope path list for named scope |
| `{{task.<n>.desc}}` | Description of a named task |
| `{{timestamp}}` | Current UTC timestamp |
| `{{os}}` | Current OS: `linux`, `darwin`, or `windows` |

**Requirements:**

- `fkn prompt <n>` renders and prints the template to stdout
- `--copy` copies to clipboard
- Unknown variables render unchanged (`{{unknown_var}}`) with a stderr warning; never an error

---

### 6.8 Watch Mode (`fkn watch <task>`)

**Priority:** P2.

Re-runs a task on file system changes. Especially useful as `fkn watch guard` during agent iteration.

**Requirements:**

- Default watch path: **repo root** — not `context.include`; these are independent concerns and must not be coupled
- `watch.paths` in `fkn.yaml` configures default watched paths
- `--path <glob>` overrides watched paths for a single invocation
- Debounce delay: 500ms default, configurable via `watch.debounce_ms`
- Prints a timestamp separator between runs

---

### 6.9 Machine-Readable Output

**Priority:** P1.

`--json` is supported on key commands. This is the primary interface for agent consumption outside of MCP. Schemas are stable and versioned (see [Section 7](#7-json-output-contracts)).

| Command | JSON content |
|---|---|
| `fkn list --json` | Array of task descriptors |
| `fkn list --mcp` | MCP tool manifest preview (same payload `fkn serve` broadcasts) |
| `fkn <task> --json` | Task execution result with per-step detail |
| `fkn guard --json` | Guard report |
| `fkn scope <n> --json` | Scope object with name and path array |

`fkn list --mcp` is a dry-run preview of the MCP manifest — it prints exactly what `fkn serve` would broadcast without starting the server. Use it to verify `agent: true/false` configuration before connecting an agent.

---

## 7. JSON Output Contracts

These schemas are stable commitments as of v1. Breaking changes require a major version bump.

`status` values in all schemas: `pass` | `fail` | `cancelled` | `timeout`

See exit code table in [Section 1](#1-overview) for the mapping to process exit codes.

---

### 7.1 `fkn list --json`

```json
{
  "tasks": [
    {
      "name": "wheel",
      "desc": "Build Python distribution wheel",
      "type": "cmd",
      "scope": null,
      "agent": true
    },
    {
      "name": "ci",
      "desc": "Full CI pipeline",
      "type": "pipeline",
      "parallel": true,
      "steps": ["lint", "test", "build"],
      "scope": null,
      "agent": true
    },
    {
      "name": "fix-auth",
      "desc": "Fix auth middleware",
      "type": "cmd",
      "scope": "api",
      "agent": true
    }
  ]
}
```

### 7.2 `fkn list --mcp`

Prints the MCP tool manifest as it would be broadcast by `fkn serve`. Same JSON structure as the MCP tools array. Does not start a server.

```json
{
  "tools": [
    {
      "name": "wheel",
      "description": "Build Python distribution wheel",
      "inputSchema": {
        "type": "object",
        "properties": {
          "env": { "type": "object" },
          "dry_run": { "type": "boolean" }
        }
      }
    }
  ]
}
```

---

### 7.3 `fkn <task> --json`

Single `cmd` task:

```json
{
  "task": "wheel",
  "type": "cmd",
  "resolved_cmd": "python -m build --wheel",
  "status": "pass",
  "exit_code": 0,
  "stdout": "...",
  "stderr": "",
  "duration_ms": 3421,
  "started_at": "2026-03-21T10:00:00Z",
  "finished_at": "2026-03-21T10:00:03Z"
}
```

Pipeline task:

```json
{
  "task": "ci",
  "type": "pipeline",
  "parallel": false,
  "status": "fail",
  "exit_code": 1,
  "duration_ms": 12300,
  "started_at": "2026-03-21T10:00:00Z",
  "finished_at": "2026-03-21T10:00:12Z",
  "steps": [
    {
      "index": 0,
      "name": "lint",
      "resolved_cmd": "ruff check src/ tests/",
      "status": "pass",
      "exit_code": 0,
      "stdout": "",
      "stderr": "",
      "duration_ms": 1200,
      "started_at": "2026-03-21T10:00:00Z",
      "finished_at": "2026-03-21T10:00:01Z"
    },
    {
      "index": 1,
      "name": "test",
      "resolved_cmd": "pytest tests/ -v",
      "status": "fail",
      "exit_code": 1,
      "stdout": "",
      "stderr": "FAILED tests/test_auth.py::test_token_expiry\n...",
      "duration_ms": 8400,
      "started_at": "2026-03-21T10:00:01Z",
      "finished_at": "2026-03-21T10:00:09Z"
    },
    {
      "index": 2,
      "name": "build",
      "resolved_cmd": null,
      "status": "cancelled",
      "exit_code": 130,
      "stdout": null,
      "stderr": null,
      "duration_ms": null,
      "started_at": null,
      "finished_at": null
    }
  ]
}
```

Cancelled steps carry `null` for all execution fields except `exit_code` which is `130`. `resolved_cmd` is `null` for cancelled steps (they never ran).

---

### 7.4 `fkn guard --json`

```json
{
  "guard": "default",
  "overall": "fail",
  "exit_code": 1,
  "duration_ms": 12700,
  "ran_at": "2026-03-21T10:00:00Z",
  "steps": [
    {
      "index": 0,
      "name": "lint",
      "resolved_cmd": "ruff check src/ tests/",
      "status": "pass",
      "exit_code": 0,
      "stderr": "",
      "duration_ms": 1200
    },
    {
      "index": 1,
      "name": "test",
      "resolved_cmd": "pytest tests/ -v",
      "status": "fail",
      "exit_code": 1,
      "stderr": "FAILED tests/test_auth.py::test_token_expiry\nAssertionError: expected 401, got 200\n",
      "duration_ms": 8400
    },
    {
      "index": 2,
      "name": "build",
      "resolved_cmd": "go build -o bin/svc ./cmd/svc",
      "status": "pass",
      "exit_code": 0,
      "stderr": "",
      "duration_ms": 3100
    }
  ]
}
```

Guard steps are never `cancelled`. `stdout` is omitted; only `stderr` is retained.

---

### 7.5 `fkn scope <n> --json`

```json
{
  "scope": "api",
  "paths": [
    "src/api/",
    "src/models/"
  ]
}
```

---

## 8. Configuration Schema

```yaml
# fkn.yaml

project: "my-api"
description: "Python FastAPI microservice"

# Optional: load a .env file into the environment for all tasks.
# fkn is not a secrets manager — this is a convenience for local dev.
# Never commit files containing real secrets.
env_file: .env

# --- Tasks ---
tasks:
  <n>:
    desc: "Human-readable description (required)"
    cmd: "shell command"              # use cmd OR steps, not both
    steps:                            # ordered list of task refs or inline commands
      - <task-name>
      - "inline shell command"
    parallel: false                   # run steps concurrently (default: false)
    env:
      KEY: value                      # task-level env; merged with global env_file
    dir: "relative/path"              # working directory (default: repo root)
    timeout: "5m"                     # kill after duration; step status becomes "timeout" (exit 124)
    continue_on_error: false          # sequential pipelines only; ignored for parallel with warning
    agent: true                       # expose in MCP manifest (default: true)
    scope: api                        # associated scope name (optional); used in context --agent

# --- Guards ---
guards:
  default:                            # run with `fkn guard`
    steps: [lint, test, build]
  pre-commit:
    steps: [lint, test]

# --- Context ---
context:
  file_tree: true
  git_log_lines: 10
  git_diff: false
  todos: false
  dependencies: true
  agent_files:
    - AGENTS.md
    - CLAUDE.md
  include:
    - README.md
    - src/
  exclude:
    - "**/__pycache__"
    - "**/*.pyc"
  caps:                               # per-section truncation limits
    file_tree_entries: 200            # max file tree entries
    files_max: 5                      # max verbatim files included
    file_lines: 100                   # max lines per verbatim file
    git_log_lines: 30                 # max git log lines
    git_diff_lines: 200               # max git diff lines
    todos_max: 20                     # max TODO/FIXME entries
    agent_file_lines: 500             # max lines per agent guidance file
    dependency_lines: 100             # max lines per dependency manifest

# --- Scopes ---
scopes:
  api:
    - src/api/
    - src/models/
  infra:
    - terraform/
    - docker/

# --- Prompts ---
prompts:
  fix-bug:
    desc: "Bug fix agent handoff"
    template: |
      Fix the failing test. Only modify files in the declared scope.
      OS: {{os}}
      Scope: {{scope.api}}
      Branch: {{git_branch}}
      {{git_diff}}

# --- MCP Server ---
serve:
  transport: stdio                    # stdio | http (default: stdio)
  port: 8080                          # used when transport: http
  token_env: FKN_MCP_TOKEN            # env var name for bearer token (HTTP mode only)
                                      # tokens are never stored in fkn.yaml

# --- Watch ---
watch:
  debounce_ms: 500
  paths: []                           # default watch paths (default: repo root)
```

---

## 9. CLI Reference

```
fkn <task>                           Run a named task
fkn <task> --dry-run                 Print command without executing
fkn <task> --json                    Output result as JSON
fkn list                             List all tasks with descriptions
fkn list --json                      Task list as JSON
fkn list --mcp                       Preview MCP tool manifest without starting server
fkn help [task]                      Show help, or detail for a named task
fkn --version                        Print fkn version (also: fkn version)

fkn guard [name]                     Run default or named guard
fkn guard [name] --json              Guard report as JSON

fkn context                          Print repo context (default: brief)
fkn context --agent --task <n>       Full agent handoff briefing
fkn context --out <file>             Write to file
fkn context --copy                   Copy to clipboard
fkn context --max-tokens <n>         Global token budget (applied after per-section caps)

fkn scope <n>                        Print scope path list (one per line)
fkn scope <n> --format prompt        As a natural language constraint sentence
fkn scope <n> --json                 As a JSON object

fkn prompt <n>                       Render and print a prompt template
fkn prompt <n> --copy                Copy to clipboard

fkn watch <task>                     Re-run task on file changes
fkn watch <task> --path <glob>       Watch specific paths

fkn serve                            Start MCP server (stdio)
fkn serve --http --port 8080         Start MCP server (HTTP+SSE)

fkn init                             Scaffold a starter fkn.yaml and update .gitignore
fkn version                          Print fkn version (also: fkn --version)
```

---

## 10. Architecture

### Technology

- **Language:** Go 1.22+
- **Distribution:** Single static binary via GoReleaser; `brew`, `curl | sh`, direct download
- **Config:** `gopkg.in/yaml.v3`
- **File watching:** `github.com/fsnotify/fsnotify`
- **MCP:** Evaluate available Go MCP libraries at implementation time; surface area is small enough to hand-roll cleanly
- **Clipboard:** `github.com/atotto/clipboard`

### Package Structure

```
fkn/
├── cmd/
│   └── fkn/
│       └── main.go               # Entry point, CLI wiring
├── internal/
│   ├── config/                   # fkn.yaml loading, validation, defaults
│   ├── runner/                   # Task execution, pipeline orchestration
│   ├── parallel/                 # Concurrent step execution, output muxing
│   ├── mcp/                      # MCP server, manifest generation, list --mcp
│   ├── context/                  # Context document generation, section caps
│   ├── guard/                    # Guard runner, report formatter, result cache
│   ├── prompt/                   # Prompt template rendering
│   ├── scope/                    # Scope path resolution and formatting
│   ├── watch/                    # File watcher
│   └── shell/                    # Cross-platform shell invocation
├── .fkn/                         # Runtime state — add to .gitignore
│   └── last-guard.json           # Cached guard result for context --agent
├── fkn.yaml                      # fkn's own tasks (dogfood)
├── go.mod
└── README.md
```

### `fkn init` behaviour

Scaffolds a starter `fkn.yaml` in the current directory. Also appends `.fkn/` to `.gitignore` (creating the file if it does not exist). The runtime state directory contains ephemeral cached output and must not be committed.

### Key Design Principles

- **`fkn.yaml` is the operational API of the repo.** The MCP manifest, help text, context documents, and JSON schemas are all derived from one file.
- **Shell transparency.** Commands are passed to the system shell as-is. `fkn` does not parse or interpret them.
- **Fail loudly.** Missing tasks, malformed config, missing binaries, and non-zero exits are all `fail`. No silent failures, no benign-looking status codes for broken environments.
- **Task output is streamed** in real time, not buffered.
- **No daemon.** `fkn serve` is the only long-running mode.
- **JSON contracts are stable.** The schemas in Section 7 are v1 commitments. Breaking changes require a major version bump.

---

## 11. Error Handling and UX

### Config Errors

- Missing `fkn.yaml`: clear message with suggestion to run `fkn init`
- Unknown task name: suggest similar names via fuzzy match
- Invalid YAML: show line number and parse error description
- Circular task dependencies: detected at load time, reported with the full cycle path before any execution begins
- `continue_on_error: true` on a parallel task: accepted in config, ignored at runtime with a printed warning

### Runtime Errors

- Command not found in PATH: `FAIL` with exit code `127`; message shows the missing binary name
- Timeout exceeded: step status `TIMEOUT`, exit code `124`; message shows task name, command, and limit
- Parallel step failure: failing step identified; other in-progress steps `CANCELLED` with exit code `130`

### Status Display Consistency

- Terminal output: `PASS` `FAIL` `CANCELLED` `TIMEOUT` (uppercase)
- JSON `status` fields: `pass` `fail` `cancelled` `timeout` (lowercase)
- Prose and inline documentation: lowercase

### General UX

- Coloured output: `PASS` green, `FAIL` red, `CANCELLED`/`TIMEOUT` yellow; disabled with `NO_COLOR=1` or non-TTY
- `fkn list` is tabular: task name left-aligned, description after a fixed column separator
- `--quiet` suppresses informational output; only task output and final status shown
- `--verbose` shows the full resolved command before executing

---

## 12. Installation and Distribution

```bash
# Homebrew
brew install fkn

# Shell installer
curl -fsSL https://fkn.dev/install.sh | sh

# Go install
go install github.com/<org>/fkn@latest
```

Supported platforms: macOS (arm64, amd64), Linux (amd64, arm64), Windows (amd64).

GoReleaser manages cross-compilation, checksums, and Homebrew tap updates.

---

## 13. Open Questions

| # | Question | Notes |
|---|----------|-------|
| 1 | MCP Go SDK or hand-roll? | Surface area is small; hand-rolling is reasonable if no stable Go SDK exists |
| 2 | Monorepo: per-subdirectory `fkn.yaml` with inheritance? | Strong candidate for v1.1 |
| 3 | Should `fkn init` offer language-specific starter templates? | Nice to have; not critical path |
| 4 | `.fkn/` directory — should location be configurable? | Defaulting to repo root is simplest; revisit for monorepo support |
| 5 | Should guard cache include `resolved_cmd` per step? | Low cost; useful for context doc accuracy |
| 6 | Parallel `continue_on_error` for v1.1? | Expected to be the #1 feature request from human users once the tool ships; semantics are clear (wait for all, non-zero if any fail) — candidate for early v1.1 |

---

## 14. Appendix: Example Config Files

### Python FastAPI Service

```yaml
project: my-api
description: Python FastAPI service

env_file: .env

tasks:
  check-env:
    desc: "Verify required tools are available"
    cmd: "command -v ruff && command -v pytest && command -v docker"
    agent: false

  install:
    desc: "Install dependencies"
    cmd: "pip install -e '.[dev]'"

  wheel:
    desc: "Build distribution wheel"
    cmd: "python -m build --wheel"

  lint:
    desc: "Run ruff linter"
    cmd: "ruff check src/ tests/"

  format:
    desc: "Auto-format with black"
    cmd: "black src/ tests/"

  test:
    desc: "Run test suite"
    cmd: "pytest tests/ -v"
    timeout: "10m"

  docker-build:
    desc: "Build Docker image"
    cmd: "docker build -t my-api:local ."

  ci:
    desc: "Full CI — checks in parallel, then build"
    steps: [checks, wheel, docker-build]

  checks:
    desc: "Lint and test in parallel"
    parallel: true
    steps: [lint, test]

guards:
  default:
    steps: [lint, test]

context:
  include: [README.md, src/, pyproject.toml]
  git_log_lines: 10
  agent_files: [AGENTS.md]

scopes:
  api: [src/api/]
  db: [src/db/, alembic/]

prompts:
  fix-bug:
    desc: "Bug fix agent handoff"
    template: |
      Fix the failing test. Only modify files in the declared scope.
      OS: {{os}}
      Scope: {{scope.api}}
      Branch: {{git_branch}}
      {{git_diff}}
```

### Go Microservice

```yaml
project: svc-orders
description: Go orders microservice

tasks:
  build:
    desc: "Compile binary"
    cmd: "go build -o bin/svc-orders ./cmd/svc-orders"

  test:
    desc: "Run tests with race detector"
    cmd: "go test -race ./..."

  lint:
    desc: "Run golangci-lint"
    cmd: "golangci-lint run ./..."

  generate:
    desc: "Run go generate"
    cmd: "go generate ./..."

  run:
    desc: "Run service locally"
    cmd: "go run ./cmd/svc-orders"
    env:
      PORT: "8080"
      LOG_LEVEL: "debug"

  ci:
    desc: "Lint and test in parallel"
    parallel: true
    steps: [lint, test]

guards:
  default:
    steps: [lint, test, build]

context:
  include: [README.md, cmd/, internal/, go.mod]
  git_log_lines: 10
  exclude:
    - "vendor/"
    - "**/*.pb.go"
    - "**/*_generated.go"
```

### JUCE / C++ Plugin (Trigenetix)

```yaml
project: trigenetix
description: Generative drum sequencer MIDI plugin (JUCE 8 / C++17)

tasks:
  configure:
    desc: "Configure CMake (Debug)"
    cmd: "cmake -B build -DCMAKE_BUILD_TYPE=Debug"

  build:
    desc: "Build plugin (Debug)"
    cmd: "cmake --build build --config Debug -j$(nproc)"

  build-release:
    desc: "Build plugin (Release)"
    cmd: "cmake --build build --config Release -j$(nproc)"

  clean:
    desc: "Clean build directory"
    cmd: "rm -rf build"
    agent: false

  rebuild:
    desc: "Clean, configure, and build"
    steps: [clean, configure, build]

  install:
    desc: "Copy VST3 to system plugin directory (macOS)"
    cmd: "cp -r build/Trigenetix_artefacts/Debug/VST3/Trigenetix.vst3 ~/Library/Audio/Plug-Ins/VST3/"
    agent: false

  test:
    desc: "Run unit tests"
    cmd: "ctest --test-dir build -C Debug --output-on-failure"

guards:
  default:
    steps: [build, test]

context:
  include:
    - README.md
    - CMakeLists.txt
    - Source/
  agent_files: [AGENTS.md]
  exclude:
    - build/
    - "**/*.o"

scopes:
  clock: [Source/Clock/]
  sequencer: [Source/Sequencer/]
  ui: [Source/UI/]
  midi: [Source/MIDI/]
```

---

*End of Document — v0.4.1 — Ready to Build*
