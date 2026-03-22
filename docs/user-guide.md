# fkn User Guide

`fkn` is a repo-local task runner and agent integration layer driven by a single `fkn.yaml`.

This guide is the practical walkthrough: what to put in `fkn.yaml`, how the commands behave, and what a realistic setup looks like.

## Installing fkn

If you are evaluating whether `fkn` should replace or sit on top of an existing Makefile, read [docs/why-not-make.md](/Users/josephfrost/code/fkn/docs/why-not-make.md) too. The short version is that `fkn` is designed to layer over existing repo commands, not force a rip-and-replace.

If you just want the CLI:

```bash
go install github.com/neural-chilli/fkn/cmd/fkn@latest
```

If you prefer not to install with Go, tagged releases now also publish prebuilt archives for macOS, Linux, and Windows on GitHub.

If you are working from a local checkout:

```bash
make build
./bin/fkn list
```

For local cross-platform release artifacts:

```bash
make dist
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

When imported tasks look like helper or repo-mutating workflows such as `clean`, `ci-init`, `release`, `deploy`, or parameterized scaffold commands, `fkn` now tends to keep them in the generated config but mark them `agent: false`. That keeps the repo surface visible to humans without encouraging autonomous agent execution for risky helper tasks.

The goal is not to guess everything perfectly. The goal is to give you a believable first `fkn.yaml` that humans and agents can edit confidently.

Treat `init --from-repo` as a strong starting point, not a source of truth. Import heuristics are intentionally best-effort and should be reviewed before you rely on the generated config.

If you also want repo-specific human and agent docs generated from `fkn.yaml`:

```bash
fkn init --docs
```

That writes:

- `HUMANS.md` with a people-oriented repo workflow summary
- `AGENTS.md` with agent workflow guidance and managed `fkn` sections
- `CLAUDE.md` with the same repo-specific guidance for Claude-style agent flows

The generated agent docs include:

- task summaries, including scopes, commands, and pipeline steps
- guard summaries
- scopes and prompts
- context and watch configuration highlights

If you want agents to explicitly treat `fkn.yaml` as an accrual surface for learned repo knowledge, enable:

```yaml
agent:
  accrue_knowledge: true
```

When enabled, the generated agent docs tell agents to propose structured updates back into `fkn.yaml` for things like dependencies, params, codemap entries, conventions, and glossary terms.

## Validate Config

Use `fkn validate` to check `fkn.yaml` without running a task:

```bash
fkn validate
fkn validate --json
```

That is the simplest way to confirm a config edit before you try `list`, `guard`, `context`, or a task run.

## Editor Schema

The repo ships [fkn.schema.json](/Users/josephfrost/code/fkn/fkn.schema.json) so editors can validate `fkn.yaml` directly.

For YAML language server clients such as VS Code, add this at the top of `fkn.yaml`:

```yaml
# yaml-language-server: $schema=./fkn.schema.json
```

That gives you:

- field autocomplete
- enum suggestions such as `safety` and `error_format`
- inline validation for the common config shape

The schema is intentionally practical rather than exhaustive. It covers the public fields and common structural rules, while the CLI remains the final source of truth for runtime validation.

## Mental Model

Treat `fkn.yaml` as the operational API of your repository.

That means one file defines:

- what tasks exist
- what aliases map onto those tasks
- how validation runs
- what scopes an agent should touch
- what prompts should render
- what context gets handed to an agent
- what workflow guidance agents should follow

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

Nested pipeline:

```yaml
tasks:
  verify:
    desc: Shared verification workflow
    steps:
      - lint
      - test

  ci:
    desc: CI entrypoint
    steps:
      - verify
      - build
```

Nested pipeline steps now execute recursively, and the JSON output keeps the child step structure so agents can still see where failures happened inside the composed workflow.

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

This does not replace raw stderr. It adds a parsed `errors` array to task JSON output, `guard --json`, and `repair --json` so agents do not have to scrape diagnostics back out of plain text.

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
build  Build the binary [cmd | aliases:b | params:<target>]
```

That summary view now includes:

- task type
- whether the task is the configured default
- scope
- aliases
- dependencies
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

- `needs`
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
  setup:
    desc: Prepare generated assets
    cmd: make setup

  integration:
    desc: Run integration tests
    cmd: go test -tags=integration ./...
    needs:
      - setup
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

- `needs` runs reusable prerequisite tasks before the task itself.
- dependencies can point at either command tasks or pipeline tasks.
- pipeline steps can point at either command tasks, inline shell steps, or other pipeline tasks.
- `defaults.dir` sets the global working directory for tasks that do not declare their own `dir`.
- task `dir` overrides `defaults.dir`.
- `shell` and `shell_args` let a task opt into a specific interpreter or shell mode.
- `continue_on_error` only affects sequential pipelines.
- Parallel pipelines still fail fast in the current implementation.
- `agent: false` marks a task as not intended for autonomous agent use.
- `safety` describes how cautious agents should be:
  - `safe` for read-only or low-risk tasks
  - `idempotent` for tasks that are safe to rerun repeatedly
  - `destructive` for tasks that mutate repo state or delete things
  - `external` for tasks that reach outside the repo, such as deploy or publish flows

Tasks marked `destructive` or `external` are blocked by default at execution time. To run them anyway:

```bash
fkn deploy --allow-unsafe
fkn guard --allow-unsafe
fkn repair --allow-unsafe
```

`--dry-run` still works without that override, so risky tasks can be inspected before they are executed.

## Task Dependencies

Use `needs` when one task should depend on another task but still remain its own command or pipeline.

Example:

```yaml
tasks:
  test:
    desc: Run tests
    cmd: go test ./...

  build:
    desc: Build the app
    cmd: go build ./...
    needs:
      - test

  release:
    desc: Build and publish a release
    cmd: ./scripts/release.sh
    needs:
      - build
      - check
```

`needs` is different from `steps`:

- `steps` makes the task itself a pipeline
- `needs` runs prerequisite tasks first, then runs the task itself

If a dependency fails, later dependencies and the main task are skipped.

## Task Params

Tasks can declare named or positional params that map to environment variables and can also be interpolated into commands via `{{params.<name>}}`.

Example:

```yaml
tasks:
  add-feature:
    desc: Add a feature scaffold
    cmd: make add-feature
    safety: destructive
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

Positional params use `position`, and the last positional param can be variadic:

```yaml
tasks:
  pack:
    desc: Pack release artifacts
    cmd: ./scripts/pack.sh {{params.target}} {{params.files}}
    params:
      target:
        env: TARGET
        required: true
        position: 1
      files:
        env: FILES
        position: 2
        variadic: true
```

Then this works naturally:

```bash
fkn pack release dist/a.tgz dist/b.tgz
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
- positional params are assigned by ascending `position`
- a variadic param must be the last positional param
- required params fail fast if omitted
- safety is exposed in `fkn help`, `fkn list`, and generated agent docs

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

## Groups

Groups are named task families. They do not change execution semantics, but they make discovery and repo documentation much easier once a config grows beyond a handful of tasks.

Example:

```yaml
groups:
  qa:
    desc: Verification and quality checks.
    tasks:
      - test
      - lint
      - check
```

Use them with:

```bash
fkn list
fkn help qa
fkn list --json
```

`fkn list` shows grouped sections in the human-readable view, and `fkn help <group>` prints the group description and member tasks.

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
      - internal/context/
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

## Version

Use:

```bash
fkn version
fkn version --json
```

`--json` is useful for scripts or agent checks that want structured version info without scraping plain text.

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

## Plan

`fkn plan` is the pre-edit companion to `fkn repair`.

It takes the files you expect to modify and returns the matching scopes, tasks, guards, groups, and codemap entries so you can decide what to read and rerun before touching code.

Examples:

```bash
fkn plan --file cmd/fkn/main.go --file internal/runner/runner.go
fkn plan --json cmd/fkn/main.go internal/runner/runner.go
```

The output is especially useful when you want to know:

- which tasks probably own the change
- which guard is the best verification target
- which codemap entries are likely relevant
- whether the impacted tasks are marked `safe`, `idempotent`, `destructive`, or `external`

`fkn plan` is intentionally scope-first, so the quality of the output improves as task scopes and codemap entries become more accurate.

## Diff Plan

`fkn diff-plan` is the git-aware companion to `fkn plan`.

Instead of passing explicit file paths, it reads:

- unstaged tracked changes
- staged tracked changes
- untracked files

and then runs the same matching logic against scopes, tasks, guards, groups, and codemap entries.

Examples:

```bash
fkn diff-plan
fkn diff-plan --json
```

This is especially useful after an edit, when you want a quick answer to “what should I rerun now?” without manually listing the files you changed.

## Agent Brief

`fkn agent-brief` is the higher-level handoff command. It reuses the existing context and planning features so you can generate one markdown or JSON payload for an agent instead of running multiple commands and stitching the results together yourself.

Examples:

```bash
fkn agent-brief
fkn agent-brief --task check
fkn agent-brief --file cmd/fkn/main.go
fkn agent-brief --diff
fkn agent-brief --json
```

Current behavior:

- with `--task <name>`, it emits the same task-focused context used by `fkn context --agent --task <name>`
- with `--file` or positional file arguments, it includes the same impact plan produced by `fkn plan`
- with `--diff`, it includes the same git-aware plan produced by `fkn diff-plan`
- with no selector, it emits a broad repo brief using the standard context generator
- `--json` returns the structured context payload, the optional structured plan payload, and the combined markdown brief

## Context

`fkn context` generates a bounded markdown briefing for humans or agents.

Default mode:

```bash
fkn context
```

Agent mode:

```bash
fkn context --agent --task check
fkn context --about "task safety"
```

Topic mode:

```bash
fkn context --about "task safety"
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

It now uses filesystem events by default for faster reruns and lower idle overhead, with the older polling path kept as a fallback if event watching is unavailable.

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

groups:
  core:
    desc: Everyday development and verification tasks.
    tasks:
      - test
      - build
      - run
      - check

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


## Status

This guide reflects the tool as currently implemented in this repository. It is not a promise that every future feature in the PRD is already complete.
