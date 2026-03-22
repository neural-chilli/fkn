# fkn

`fkn` is Flow Kit Neu: a repo-local task runner and agent integration layer driven by a single `fkn.yaml`.

It is designed to make a repository's operational surface obvious to both humans and coding agents. The long-term model is simple: one file defines how the repo is built, checked, run, and exposed to agents.

It is also intentionally easy to install in locked-down environments: one Go-based binary, buildable from source with `go install`, no extra runtime, no container requirement, and now optional prebuilt release archives for teams that prefer them.

## What It Is

Most repositories accumulate commands, scripts, checks, conventions, and "run this exact thing" tribal knowledge. `fkn` turns that into an explicit interface:

- one config file
- one CLI
- stable JSON output for machine consumers
- task metadata that can guide human and agent execution safely
- a path toward agent handoff and repo-aware prompts without scattered docs

YAML is a deliberate part of that story. `fkn.yaml` uses a format that is already common across developer tooling, easy for humans to scan, and straightforward for agents and automation to parse.

## Docs

Start here:

- [User Guide](/Users/josephfrost/code/fkn/docs/user-guide.md)
- [Why Not Just Use make?](/Users/josephfrost/code/fkn/docs/why-not-make.md)
- [Release Guide](/Users/josephfrost/code/fkn/docs/releasing.md)
- [Roadmap](/Users/josephfrost/code/fkn/docs/roadmap.md)

The README is the short version. The user guide is the practical walkthrough with realistic examples.

## Current Status

Implemented today:

- task execution
- sequential and parallel pipelines
- nested pipeline composition
- guards
- scopes with optional descriptions and reusable path intent
- prompts
- context generation
- init scaffolding
- repo-aware init scaffolding from existing task surfaces
- guards can reuse pipeline tasks
- generated repo docs for humans and agents
- embedded offline docs in the CLI
- config validation via `fkn validate`
- structured context output via `fkn context --json`
- fsnotify-backed watch mode with polling fallback
- help output and task suggestions
- JSON output for key commands
- broader Makefile/justfile task import during `init --from-repo`
- richer `justfile` import with aliases, params, and private-recipe filtering
- package.json argument inference for `npm_config_*`-style scripts
- Rust, Python, Java, and Docker Compose task scaffolding during `init --from-repo`
- safer helper-task import with `agent: false` for mutating targets
- shell completions with install helpers for bash, zsh, fish, and PowerShell
- task safety annotations for humans and agents
- execution gating for `destructive` and `external` tasks
- task params with CLI and runner support
- direct task param flags like `--feature auth`
- structured error extraction in task and guard JSON output
- guided repair output via `fkn repair`
- file-targeted impact planning via `fkn plan`
- git-diff-aware planning via `fkn diff-plan`
- composed agent handoff briefs via `fkn agent-brief`
- JSON Schema for `fkn.yaml`
- codemap-backed repo explanations via `fkn explain`
- topic-targeted context via `fkn context --about`
- task-level shell configuration
- global default working directory with task overrides
- task aliases
- task groups for related command families
- reusable task dependencies via `needs`
- explicit default task behavior
- positional and variadic task params
- richer human-readable `list` and `help` output
- structured `fkn version --json`

## Why Open Source

`fkn` gets better when the config shape is tested across real repositories, stacks, and teams. Open source is a good fit because:

- the core problem is widely shared
- adoption matters more than lock-in
- community feedback will sharpen the schema and command UX
- trust is easier when the operational interface is transparent

## Quick Start

### Prerequisites

- Go 1.22+

### Install

For teams that already have Go approved, the simplest path is still source install:

```bash
go install github.com/neural-chilli/fkn/cmd/fkn@latest
```

For a tagged release:

```bash
go install github.com/neural-chilli/fkn/cmd/fkn@v0.1.0
```

Tagged releases also publish prebuilt archives for macOS, Linux, and Windows on the GitHub Releases page, along with a `checksums.txt` file.

### Build

```bash
make build
```

This produces `bin/fkn`.

To build cross-platform release archives locally:

```bash
make dist
```

That writes versioned `.tar.gz` and `.zip` artifacts plus checksums into `dist/`.

The built binary reports a stamped version, and tagged `go install` builds report their module version instead of `dev`.

### Run

```bash
go run ./cmd/fkn list
go run ./cmd/fkn docs
go run ./cmd/fkn docs user-guide
go run ./cmd/fkn test
go run ./cmd/fkn add-feature --feature auth
go run ./cmd/fkn build web
go run ./cmd/fkn pack release dist/a.tgz dist/b.tgz
go run ./cmd/fkn check --dry-run
go run ./cmd/fkn guard
go run ./cmd/fkn repair
go run ./cmd/fkn plan --file cmd/fkn/main.go
go run ./cmd/fkn diff-plan
go run ./cmd/fkn agent-brief --task check
go run ./cmd/fkn agent-brief --diff
go run ./cmd/fkn explain internal/runner
go run ./cmd/fkn context --about transport
go run ./cmd/fkn scope cli
go run ./cmd/fkn prompt continue-cli
go run ./cmd/fkn context
go run ./cmd/fkn context --agent --task check
go run ./cmd/fkn init
go run ./cmd/fkn init --from-repo
go run ./cmd/fkn init --docs
go run ./cmd/fkn completion bash
go run ./cmd/fkn completion install
go run ./cmd/fkn watch test --path README.md
go run ./cmd/fkn help check
```

For realistic examples and a full config walkthrough:

- [docs/user-guide.md](/Users/josephfrost/code/fkn/docs/user-guide.md)
- [docs/why-not-make.md](/Users/josephfrost/code/fkn/docs/why-not-make.md)

For editor support, the repo also ships [fkn.schema.json](/Users/josephfrost/code/fkn/fkn.schema.json).

## Example `fkn.yaml`

```yaml
project: my-service
description: Example repository
default: check

tasks:
  test:
    desc: Run the test suite
    cmd: go test ./...
    safety: idempotent

  build:
    desc: Build the application
    cmd: go build -o bin/my-service ./cmd/my-service
    safety: idempotent
    needs:
      - test

  check:
    desc: Run local verification
    steps:
      - build

aliases:
  t: test
  b: build

groups:
  core:
    desc: Everyday local development commands.
    tasks:
      - test
      - build
      - check

scopes:
  backend:
    desc: Backend application code and closely-related internals.
    paths:
      - cmd/my-service/
      - internal/
```

Running `fkn` with no task name executes the configured default task when `default:` is set.

The repo now also ships [fkn.schema.json](/Users/josephfrost/code/fkn/fkn.schema.json), so editors and language servers can validate `fkn.yaml` without needing to run the CLI first.

Scopes can still be simple path lists, but the richer object form lets you attach intent that shows up in `fkn scope`, `fkn help <task>`, repair briefs, and generated agent docs.

Groups give you a lightweight way to model task families. `fkn list` uses them to organize larger configs, and `fkn help <group>` prints the group description and member tasks.

`fkn list` now also shows summary metadata like task type, default marker, aliases, scope, dependencies, params, and safety in the human-readable view, and `fkn help <task>` includes a concrete usage line.

Tasks can declare positional params with `position`, and the last positional param can be variadic with `variadic: true`. Named `--param` and direct `--name value` flags still work too, so task authors can support both natural positional invocation and explicit named invocation.

`needs` gives a task reusable prerequisites without forcing it to become a pipeline. Dependencies run before the task itself, can point at either command tasks or pipeline tasks, and surface in JSON output as nested dependency results.

Pipeline steps can now also reference other pipeline tasks directly, so larger workflows can be composed hierarchically instead of flattening every step into one long task.

Tasks can also declare `safety` as one of `safe`, `idempotent`, `destructive`, or `external`. This shows up in `fkn help`, `fkn list`, and generated agent docs so agents can make better decisions about what to run autonomously.

Tasks marked `destructive` or `external` now fail closed by default. To execute them anyway, opt in explicitly with `--allow-unsafe` on the CLI. Dry runs still work without that override.

Tasks can also declare `error_format` when they emit machine-parseable diagnostics. Supported values today are `go_test`, `pytest`, `tsc`, `eslint`, and `generic`. When set, task JSON, `guard --json`, and `repair --json` include a parsed `errors` array alongside raw stderr.

`fkn repair` builds on that by running a guard, collecting the failing steps, surfacing relevant scopes, and generating a repair-oriented markdown brief for the next agent loop.

`fkn plan` works the other direction: give it the files you expect to touch, and it will tell you which scopes, tasks, guards, groups, and codemap entries are likely relevant before you start editing.

`fkn diff-plan` uses the current git diff, staged changes, and untracked files to answer the same question after you have already started editing.

`fkn agent-brief` composes those pieces into one handoff artifact. Point it at a task, a file set, or the current diff and it will combine the existing context output with a relevant change plan when one exists.

`codemap` adds a semantic layer to `fkn.yaml`, and `fkn explain` turns those entries into targeted repo briefings for packages, entry points, glossary terms, and tasks.

## Commands Available Today

```text
fkn [<task>] [--name value] [--param name=value]
fkn <task> --dry-run
fkn <task> --allow-unsafe
fkn <task> --json
fkn completion <bash|zsh|fish|powershell>
fkn completion install [--shell <bash|zsh|fish|powershell>]
fkn docs [name] [--list]
fkn diff-plan [--json]
fkn agent-brief [--task <name> | --diff | --file <path>...] [--json] [--max-tokens <approx-n>]
fkn explain <target> [--json]
fkn help [task|group]
fkn plan [--json] [--file <path>] [files...]
fkn guard
fkn guard --json
fkn repair [name] [--json] [--copy]
fkn context
fkn context --json
fkn context --agent --task <name>
fkn context --about <topic>
fkn context --out <file>
fkn context --copy
fkn init [--from-repo] [--docs]
fkn watch <target>
fkn prompt <name>
fkn prompt <name> --copy
fkn scope <name>
fkn scope <name> --json
fkn scope <name> --format prompt
fkn validate
fkn validate --json
fkn list
fkn list --json
fkn version
fkn --version
fkn version --json
```

## Project Layout

```text
cmd/fkn/              # CLI entrypoint
internal/config/      # fkn.yaml loading and validation
internal/context/     # bounded repo context generation
internal/codemap/     # semantic repo explanations and matching
internal/prompt/      # prompt template rendering
internal/repair/      # guard-driven repair brief generation
internal/scope/       # named scope lookup and formatting
internal/runner/      # task and pipeline execution
fkn.yaml              # repo-local dogfood config
fkn-prd-v4.1.md       # product requirements document
```

## Development

Useful local commands:

```bash
make build
make test
go test ./...
go run ./cmd/fkn check --dry-run
```

GitHub Actions now verifies formatting, runs `go test ./...`, and builds `./cmd/fkn` on pushes to `main` and on pull requests.

The current product direction is described in [fkn-prd-v4.1.md](/Users/josephfrost/code/fkn/fkn-prd-v4.1.md).

The forward-looking feature roadmap is in [docs/roadmap.md](/Users/josephfrost/code/fkn/docs/roadmap.md).

`fkn docs` now serves embedded copies of the README, user guide, and release guide so installed binaries can explain themselves offline.

`fkn init --docs` now generates `HUMANS.md`, `AGENTS.md`, and `CLAUDE.md` directly from `fkn.yaml`, so repo workflow guidance stays aligned with the actual task/config surface.

## Contributing

Contributions are welcome. For the best starting point, read [CONTRIBUTING.md](/Users/josephfrost/code/fkn/CONTRIBUTING.md).

If you want to help early, the highest-leverage areas are:

- real-world `fkn.yaml` examples
- command UX rough edges
- JSON contract feedback
- guard/context implementation
- expanded tests for init, context, watch, runner execution, and CLI edge cases

## License

This project is licensed under the MIT License. See [LICENSE](/Users/josephfrost/code/fkn/LICENSE).
