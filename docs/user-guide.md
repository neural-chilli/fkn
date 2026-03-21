# fkn User Guide

`fkn` is a repo-local task runner and agent integration layer driven by a single `fkn.yaml`.

This guide is the practical walkthrough: what to put in `fkn.yaml`, how the commands behave, and what a realistic setup looks like.

## Installing fkn

If you just want the CLI:

```bash
go install github.com/neural-chilli/fkn/cmd/fkn@latest
```

If you are working from a local checkout:

```bash
make build
./bin/fkn list
```

You can read the bundled docs from the installed binary too:

```bash
fkn docs
fkn docs user-guide
fkn docs --list
```

## Bootstrapping An Existing Repo

If a repo already has a `Makefile`, `package.json`, or a `go.mod`, start here:

```bash
fkn init --from-repo
```

That tells `fkn` to infer a first pass from what the repo already exposes instead of dropping in the generic starter.

Current inference sources:

- common `Makefile` targets like `test`, `build`, `check`, and `lint`
- `justfile` and `Justfile`
- common `package.json` scripts like `test`, `build`, `check`, `lint`, `dev`, and `start`
- Go module repos via `go.mod`

For Makefiles and justfiles, `fkn` now imports most regular targets instead of only a tiny built-in subset. It still skips obviously awkward scaffolding targets like `clean`, but it can now keep parameterized helper targets when it can infer env-style inputs such as `$(FEATURE)`.

For `justfile`s specifically, `fkn` now also carries over common recipe aliases, positional/defaulted recipe parameters, and skips `_hidden` or `[private]` recipes so the generated config matches what humans usually invoke.

For `package.json`, `fkn` now detects common `npm_config_*` patterns and turns them into declared task params. That means scripts like `vite build --mode=$npm_config_mode` can scaffold to a task that exposes `mode` explicitly instead of hiding it inside the raw npm script string.

When imported tasks look like helper or repo-mutating workflows such as `clean`, `ci-init`, `release`, `deploy`, or parameterized scaffold commands, `fkn` now tends to keep them in the generated config but mark them `agent: false`. That keeps the repo surface visible to humans without eagerly exposing risky helper tasks over MCP.

The goal is not to guess everything perfectly. The goal is to give you a believable first `fkn.yaml` that humans and agents can edit confidently.

If you also want agent-facing guidance files:

```bash
fkn init --agents
```

That writes:

- `AGENTS_FKN.md` with repo-specific `fkn` workflow guidance
- a small managed `fkn` section in `AGENTS.md` pointing agents at `AGENTS_FKN.md`

The generated `AGENTS_FKN.md` now includes:

- task summaries, including scopes, commands, and pipeline steps
- guard summaries
- scopes and prompts
- context, MCP, and watch configuration highlights

## Validate Config

Use `fkn validate` to check `fkn.yaml` without running a task:

```bash
fkn validate
fkn validate --json
```

That is the simplest way to confirm a config edit before you try `list`, `guard`, `context`, or a task run.

## Mental Model

Treat `fkn.yaml` as the operational API of your repository.

That means one file defines:

- what tasks exist
- what aliases map onto those tasks
- how validation runs
- what scopes an agent should touch
- what prompts should render
- what context gets handed to an agent
- what MCP tools should be exposed

If a repo has a `README`, `Makefile`, scattered scripts, and tribal knowledge, `fkn` aims to turn that into one structured interface.

## Minimal Example

```yaml
project: my-service
description: Example Go service
default: check

tasks:
  test:
    desc: Run the test suite
    cmd: go test ./...

  build:
    desc: Build the binary
    cmd: go build -o bin/my-service ./cmd/my-service

  check:
    desc: Run local verification
    steps:
      - test
      - build
```

With that file in place:

```bash
fkn list
fkn test
fkn check
fkn check --dry-run
fkn test --json
```

When `default:` is set, running plain `fkn` executes that task.

## Task Basics

Each task must have:

- a name
- a required `desc`
- either `cmd` or `steps`

Single command task:

```yaml
tasks:
  lint:
    desc: Run static analysis
    cmd: golangci-lint run
```

Sequential pipeline:

```yaml
tasks:
  check:
    desc: Run local verification
    steps:
      - lint
      - test
      - build
```

Parallel pipeline:

```yaml
tasks:
  ci:
    desc: Run fast checks in parallel
    parallel: true
    steps:
      - lint
      - test
      - build
```

## Structured Errors

If a task emits stable compiler or test diagnostics, you can tell `fkn` how to parse them:

```yaml
tasks:
  test:
    desc: Run Go tests
    cmd: go test ./...
    error_format: go_test
```

Supported formats today:

- `go_test`
- `pytest`
- `tsc`
- `eslint`
- `generic`

This does not replace raw stderr. It adds a parsed `errors` array to task JSON output, `guard --json`, `repair --json`, and MCP `tools/call` responses so agents do not have to scrape diagnostics back out of plain text.

Example:

```bash
fkn test --json
fkn guard --json
fkn repair
fkn repair --json
```

## Aliases

Aliases let you expose short or migration-friendly names for existing tasks.

Example:

```yaml
tasks:
  build:
    desc: Build the project
    cmd: go build ./...

aliases:
  b: build
  verify: build
```

Then either of these works:

```bash
fkn b
fkn help b
```

Aliases do not create separate tasks. They point at an existing task and reuse its behavior.

## Default Task

If you want `fkn` on its own to do something useful, set a top-level default task:

```yaml
default: check

tasks:
  test:
    desc: Run tests
    cmd: go test ./...

  build:
    desc: Build the app
    cmd: go build ./...

  check:
    desc: Run the default local verification pipeline
    steps:
      - test
      - build
```

Then this works:

```bash
fkn
```

The default can point at either a task name or an alias.

## Reading Tasks Quickly

`fkn list` is meant to be the fast discovery command.

Example output:

```text
check  Run local verification [pipeline | default | scope:cli]
build  Build the binary [cmd | aliases:b | params:--target]
```

That summary view now includes:

- task type
- whether the task is the configured default
- scope
- aliases
- declared params
- whether a task is hidden from agents

For a single task, `fkn help <task>` gives the fuller view, including a concrete usage line.

Example:

```text
build

Description: Build the binary
Aliases: b
Usage: fkn build --target <value> [--dry-run] [--json]
Type: cmd
```

## Task Options

Tasks can also include:

- `params`
- `env`
- `dir`
- `shell`
- `shell_args`
- `timeout`
- `continue_on_error`
- `agent`
- `scope`

Example:

```yaml
defaults:
  dir: services/api

tasks:
  integration:
    desc: Run integration tests
    cmd: go test -tags=integration ./...
    dir: tools
    shell: /bin/sh
    shell_args:
      - -eu
      - -c
    timeout: 5m
    env:
      APP_ENV: test
    scope: backend
```

Notes:

- `defaults.dir` sets the global working directory for tasks that do not declare their own `dir`.
- task `dir` overrides `defaults.dir`.
- `shell` and `shell_args` let a task opt into a specific interpreter or shell mode.
- `continue_on_error` only affects sequential pipelines.
- Parallel pipelines still fail fast in the current implementation.
- `agent: false` hides a task from the MCP tool manifest.

## Task Params

Tasks can declare named params that map to environment variables and can also be interpolated into commands via `{{params.<name>}}`.

Example:

```yaml
tasks:
  add-feature:
    desc: Add a feature scaffold
    cmd: make add-feature
    params:
      feature:
        desc: Feature name
        env: FEATURE
        required: true
```

Run it with:

```bash
fkn add-feature --feature auth
fkn add-feature --param feature=auth
```

Template interpolation also works:

```yaml
tasks:
  greet:
    desc: Greet someone
    cmd: echo hello {{params.name}}
    params:
      name:
        desc: Person to greet
        env: NAME
        default: world
```

Notes:

- declared params can be passed as direct flags like `--feature auth` or `--feature=auth`
- repeated `--param name=value` still works
- required params fail fast if omitted
- params are exposed in MCP tool schemas for agent callers

## Guards

Guards are validation-oriented step lists that always run all steps.

Example:

```yaml
guards:
  default:
    steps:
      - test
      - build
```

Run them with:

```bash
fkn guard
fkn guard default
fkn guard --json
```

Why guards exist:

- task pipelines are for doing work
- guards are for reporting breakage

If one guard step fails, later steps still run so you get a fuller report.

Guard steps can now reference either cmd tasks or pipeline tasks, so you can reuse something like `check` directly instead of duplicating its underlying steps.

## Scopes

Scopes are named path groups, and they can also carry a short description of why that part of the repo exists.

Example:

```yaml
scopes:
  cli:
    desc: Main CLI commands and the execution packages they depend on.
    paths:
      - cmd/fkn/
      - internal/runner/
      - internal/mcp/
```

Use them with:

```bash
fkn scope cli
fkn scope cli --json
fkn scope cli --format prompt
```

Tasks can reference them:

```yaml
tasks:
  fix-cli:
    desc: Work on the CLI
    cmd: go test ./cmd/fkn ./internal/runner
    scope: cli
```

The older flat-list form still works:

```yaml
scopes:
  cli:
    - cmd/fkn/
    - internal/runner/
```

If a scope has a description, `fkn scope cli --format prompt`, `fkn help <task>`, generated repair briefs, and generated agent docs include that intent alongside the path list.

## Codemap

`codemap` lets you add a lightweight semantic layer to `fkn.yaml` so agents can ask what a package or concept means without reading half the repo first.

Example:

```yaml
codemap:
  packages:
    internal/runner:
      desc: Core task execution engine
      key_types:
        - Runner
      entry_points:
        - Runner.Run
      conventions:
        - All execution flows through runCommand

  glossary:
    guard: A validation-oriented step list that always runs all steps
```

Use it with:

```bash
fkn explain internal/runner
fkn explain Runner.Run
fkn explain guard --json
```

In agent mode, `fkn context --agent --task <name>` now also includes matching codemap entries for the task scope when they exist.

## Prompts

Prompts let you keep reusable agent handoff templates in the repo.

Example:

```yaml
prompts:
  continue-cli:
    desc: Continue work on the CLI
    template: |
      Continue work on branch {{git_branch}}.
      OS: {{os}}
      Scope: {{scope.cli}}
      Check task: {{task.check.desc}}
      Current diff:
      {{git_diff}}
```

Render with:

```bash
fkn prompt continue-cli
fkn prompt continue-cli --copy
```

Supported variables today:

- `{{git_branch}}`
- `{{git_diff}}`
- `{{git_log}}`
- `{{scope.<name>}}`
- `{{task.<name>.desc}}`
- `{{timestamp}}`
- `{{os}}`

Unknown variables are left unchanged and emit a warning.

## Repair

`fkn repair` is the shortest path from a failing guard to an agent-ready fix brief. It reruns the target guard, keeps the latest guard cache up to date, and emits either markdown or structured JSON.

Examples:

```bash
fkn repair
fkn repair default --json
```

Current repair output includes:

- current guard status and step timings
- failing steps only
- parsed structured errors when `error_format` is configured
- relevant task scopes and scoped paths
- current git diff, bounded by the context diff cap
- a suggested next action

## Context

`fkn context` generates a bounded markdown briefing for humans or agents.

Default mode:

```bash
fkn context
```

Agent mode:

```bash
fkn context --agent --task check
fkn context --about "MCP transport"
```

Topic mode:

```bash
fkn context --about "MCP transport"
```

Structured mode:

```bash
fkn context --json
```

Useful flags:

```bash
fkn context --out /tmp/context.md
fkn context --copy
fkn context --max-tokens 500
```

`--max-tokens` is a rough character-based token estimate, not an exact model tokenizer count.

`--about <topic>` matches against task names and descriptions, scope names and paths, codemap package entries, and glossary terms, then builds a tighter context document around those matches.

The current implementation can include:

- project summary
- file tree
- task and guard summaries
- dependency manifests
- recent git log
- configured files and agent files
- current git diff in agent mode
- a structured JSON form via `--json` with section titles, bodies, and rendered markdown
- cached last guard output in agent mode

### Context Config

Example:

```yaml
context:
  file_tree: true
  git_log_lines: 10
  git_diff: false
  dependencies: true
  agent_files:
    - README.md
  include:
    - README.md
    - cmd/
    - internal/
    - fkn.yaml
  exclude:
    - .git/
    - .idea/
    - bin/
  caps:
    file_tree_entries: 80
    file_lines: 80
    git_diff_lines: 120
    agent_file_lines: 120
```

Truncation is explicit. When a section is capped, `fkn` inserts a marker like:

```text
[Lines 121–165 omitted]
```

That matters because an agent reading truncated file content should never assume the file ends at the truncation point.

## Init

`fkn init` scaffolds a starter config.

```bash
fkn init
```

It currently:

- creates `fkn.yaml` if it does not exist
- creates or updates `.gitignore`
- ensures `.fkn/` is ignored
- leaves an existing `fkn.yaml` unchanged

## Watch

`fkn watch` reruns a task or guard when watched files change.

Examples:

```bash
fkn watch test
fkn watch test --path README.md
fkn watch guard
fkn watch guard:default
```

Config:

```yaml
watch:
  debounce_ms: 500
  paths:
    - README.md
    - cmd/
    - internal/
    - fkn.yaml
```

Behavior notes:

- default watch path is repo root if no configured or CLI paths are given
- `--path` overrides the configured list for that run
- a timestamp separator is printed before each rerun
- `.git/`, `.fkn/`, and `bin/` are ignored

## Serve

`fkn serve` exposes agent-enabled tasks as MCP tools.

Examples:

```bash
fkn serve
fkn serve --http --port 8080
fkn list --mcp
```

See the dedicated MCP guide:

- [docs/mcp.md](/Users/josephfrost/code/fkn/docs/mcp.md)

## Realistic Example

Here is a more realistic `fkn.yaml` for a Go API:

```yaml
project: payments-api
description: Internal payments service

tasks:
  test:
    desc: Run unit tests
    cmd: go test ./...

  build:
    desc: Build the API binary
    cmd: go build -o bin/payments ./cmd/payments

  run:
    desc: Start the API locally
    cmd: go run ./cmd/payments
    env:
      APP_ENV: local

  check:
    desc: Run local verification
    steps:
      - test
      - build
    scope: backend

guards:
  default:
    steps:
      - test
      - build

scopes:
  backend:
    desc: Payments API handlers, application wiring, and supporting internals.
    paths:
      - cmd/payments/
      - internal/

prompts:
  fix-backend-bug:
    desc: Backend bugfix prompt
    template: |
      Fix the bug in the backend.
      Scope: {{scope.backend}}
      Task: {{task.check.desc}}
      Branch: {{git_branch}}
      Diff:
      {{git_diff}}

context:
  file_tree: true
  dependencies: true
  agent_files:
    - README.md
  include:
    - README.md
    - cmd/
    - internal/
    - fkn.yaml

watch:
  debounce_ms: 500
  paths:
    - cmd/
    - internal/
    - fkn.yaml

serve:
  transport: stdio
  port: 8080
  token_env: FKN_MCP_TOKEN
```

Typical workflow:

```bash
fkn init
fkn list
fkn check
fkn guard
fkn context --agent --task check
fkn prompt fix-backend-bug
fkn watch guard
```

## Troubleshooting

Common issues:

- `missing fkn.yaml`
  Run `fkn init` or create the file manually.

- `unknown task`
  Run `fkn list` or `fkn help <task>`. The CLI now suggests close task names.

- guard or task exits non-zero
  That is treated as failure, by design. `fkn` does not hide broken commands behind soft statuses.

- command not found
  This is a broken environment, not a skip. Fix the underlying toolchain.

- prompt variable not rendering
  Unknown variables are left unchanged and produce a warning.

- HTTP serve mode starts unauthenticated
  Set the env var named by `serve.token_env`, which defaults to `FKN_MCP_TOKEN`.

## Status

This guide reflects the tool as currently implemented in this repository. It is not a promise that every future feature in the PRD is already complete.
