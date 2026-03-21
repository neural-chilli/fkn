# fkn

`fkn` is Function Kit Nextgen: a repo-local task runner and agent integration layer driven by a single `fkn.yaml`.

It is designed to make a repository's operational surface obvious to both humans and coding agents. The long-term model is simple: one file defines how the repo is built, checked, run, and exposed to agents.

## What It Is

Most repositories accumulate commands, scripts, checks, conventions, and "run this exact thing" tribal knowledge. `fkn` turns that into an explicit interface:

- one config file
- one CLI
- stable JSON output for machine consumers
- task metadata that can later be exposed as MCP tools
- a path toward agent handoff and repo-aware prompts without scattered docs

## Current Status

This repository currently includes the first working slice:

- YAML-backed task configuration via `fkn.yaml`
- `fkn <task>` to run named command tasks
- sequential and parallel task pipelines
- `fkn guard [name]` to run validation-oriented guard pipelines
- `fkn scope <name>` to print a named path scope
- `fkn prompt <name>` to render a repo-versioned prompt template
- `fkn context` to generate a bounded markdown repo briefing
- `fkn init` to scaffold a starter config and ignore runtime state
- `fkn <task> --dry-run` to print resolved commands
- `fkn <task> --json` for machine-readable execution results
- `fkn guard --json` for structured guard reports
- `fkn scope <name> --json` for scope data
- `fkn list` and `fkn list --json`
- `fkn list --mcp` to preview the MCP manifest for agent-enabled tasks
- config validation for required task shape and circular task references

Planned but not built yet:

- `fkn watch`
- `fkn serve`

## Why Open Source

`fkn` gets better when the config shape is tested across real repositories, stacks, and teams. Open source is a good fit because:

- the core problem is widely shared
- adoption matters more than lock-in
- community feedback will sharpen the schema and command UX
- trust is easier when the operational interface is transparent

## Quick Start

### Prerequisites

- Go 1.22+

### Build

```bash
make build
```

This produces `bin/fkn`.

### Run

```bash
go run ./cmd/fkn list
go run ./cmd/fkn test
go run ./cmd/fkn check --dry-run
go run ./cmd/fkn guard
go run ./cmd/fkn scope cli
go run ./cmd/fkn prompt continue-cli
go run ./cmd/fkn context
go run ./cmd/fkn context --agent --task check
go run ./cmd/fkn init
go run ./cmd/fkn list --mcp
```

## Example `fkn.yaml`

```yaml
project: my-service
description: Example repository

tasks:
  test:
    desc: Run the test suite
    cmd: go test ./...

  build:
    desc: Build the application
    cmd: go build -o bin/my-service ./cmd/my-service

  check:
    desc: Run local verification
    steps:
      - test
      - build
```

## Commands Available Today

```text
fkn <task>
fkn <task> --dry-run
fkn <task> --json
fkn guard
fkn guard --json
fkn context
fkn context --agent --task <name>
fkn context --out <file>
fkn context --copy
fkn init
fkn prompt <name>
fkn prompt <name> --copy
fkn scope <name>
fkn scope <name> --json
fkn scope <name> --format prompt
fkn list
fkn list --json
fkn list --mcp
fkn version
fkn --version
```

## Project Layout

```text
cmd/fkn/              # CLI entrypoint
internal/config/      # fkn.yaml loading and validation
internal/context/     # bounded repo context generation
internal/prompt/      # prompt template rendering
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

The current product direction is described in [fkn-prd-v4.1.md](/Users/josephfrost/code/fkn/fkn-prd-v4.1.md).

## Contributing

Contributions are welcome. For the best starting point, read [CONTRIBUTING.md](/Users/josephfrost/code/fkn/CONTRIBUTING.md).

If you want to help early, the highest-leverage areas are:

- real-world `fkn.yaml` examples
- command UX rough edges
- JSON contract feedback
- guard/context implementation
- tests for config validation and runner behavior

## License

This project is licensed under the MIT License. See [LICENSE](/Users/josephfrost/code/fkn/LICENSE).
