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

## Mental Model

Treat `fkn.yaml` as the operational API of your repository.

That means one file defines:

- what tasks exist
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

## Task Options

Tasks can also include:

- `env`
- `dir`
- `timeout`
- `continue_on_error`
- `agent`
- `scope`

Example:

```yaml
tasks:
  integration:
    desc: Run integration tests
    cmd: go test -tags=integration ./...
    dir: .
    timeout: 5m
    env:
      APP_ENV: test
    scope: backend
```

Notes:

- `continue_on_error` only affects sequential pipelines.
- Parallel pipelines still fail fast in the current implementation.
- `agent: false` hides a task from the MCP tool manifest.

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

## Scopes

Scopes are named path lists. Nothing more.

Example:

```yaml
scopes:
  cli:
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

## Context

`fkn context` generates a bounded markdown briefing for humans or agents.

Default mode:

```bash
fkn context
```

Agent mode:

```bash
fkn context --agent --task check
```

Useful flags:

```bash
fkn context --out /tmp/context.md
fkn context --copy
fkn context --max-tokens 500
```

The current implementation can include:

- project summary
- file tree
- task and guard summaries
- dependency manifests
- recent git log
- configured files and agent files
- current git diff in agent mode
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
