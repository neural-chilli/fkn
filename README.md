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

YAML is a deliberate part of that story. `fkn.yaml` uses a format that is already common across developer tooling, easy for humans to scan, and straightforward for agents and automation to parse.

## Docs

Start here:

- [User Guide](/Users/josephfrost/code/fkn/docs/user-guide.md)
- [MCP Guide](/Users/josephfrost/code/fkn/docs/mcp.md)
- [Release Guide](/Users/josephfrost/code/fkn/docs/releasing.md)
- [Roadmap](/Users/josephfrost/code/fkn/docs/roadmap.md)

The README is the short version. The user guide is the practical walkthrough with realistic examples. The MCP guide is the current compatibility and transport reference.

## Current Status

Implemented today:

- task execution
- sequential and parallel pipelines
- guards
- scopes
- prompts
- context generation
- init scaffolding
- repo-aware init scaffolding from existing task surfaces
- generated agent guidance via `AGENTS_FKN.md`
- embedded offline docs in the CLI
- MCP serve mode
- watch mode
- help output and task suggestions
- JSON output for key commands
- broader Makefile/justfile task import during `init --from-repo`
- richer `justfile` import with aliases, params, and private-recipe filtering
- package.json argument inference for `npm_config_*`-style scripts
- task params with CLI, runner, and MCP support
- direct task param flags like `--feature auth`
- task aliases
- explicit default task behavior
- richer human-readable `list` and `help` output

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

For the latest version from GitHub:

```bash
go install github.com/neural-chilli/fkn/cmd/fkn@latest
```

For a tagged release:

```bash
go install github.com/neural-chilli/fkn/cmd/fkn@v0.1.0
```

### Build

```bash
make build
```

This produces `bin/fkn`.

The built binary reports a stamped version, and tagged `go install` builds report their module version instead of `dev`.

### Run

```bash
go run ./cmd/fkn list
go run ./cmd/fkn docs
go run ./cmd/fkn docs user-guide
go run ./cmd/fkn test
go run ./cmd/fkn add-feature --feature auth
go run ./cmd/fkn check --dry-run
go run ./cmd/fkn guard
go run ./cmd/fkn scope cli
go run ./cmd/fkn prompt continue-cli
go run ./cmd/fkn context
go run ./cmd/fkn context --agent --task check
go run ./cmd/fkn init
go run ./cmd/fkn init --from-repo
go run ./cmd/fkn init --agents
go run ./cmd/fkn serve
go run ./cmd/fkn serve --http --port 8080
go run ./cmd/fkn watch test --path README.md
go run ./cmd/fkn help check
go run ./cmd/fkn list --mcp
```

HTTP mode reads an optional bearer token from `FKN_MCP_TOKEN` by default.

For realistic examples and a full config walkthrough:

- [docs/user-guide.md](/Users/josephfrost/code/fkn/docs/user-guide.md)

## Example `fkn.yaml`

```yaml
project: my-service
description: Example repository
default: check

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

aliases:
  t: test
  b: build
```

Running `fkn` with no task name executes the configured default task when `default:` is set.

`fkn list` now also shows summary metadata like task type, default marker, aliases, scope, and params in the human-readable view, and `fkn help <task>` includes a concrete usage line.

## Commands Available Today

```text
fkn [<task>] [--name value] [--param name=value]
fkn <task> --dry-run
fkn <task> --json
fkn docs [name] [--list]
fkn help [task]
fkn guard
fkn guard --json
fkn context
fkn context --agent --task <name>
fkn context --out <file>
fkn context --copy
fkn init [--from-repo] [--agents]
fkn serve
fkn serve --http --port <n>
fkn watch <target>
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
internal/mcp/         # MCP manifest and transport handling
internal/prompt/      # prompt template rendering
internal/scope/       # named scope lookup and formatting
internal/runner/      # task and pipeline execution
fkn.yaml              # repo-local dogfood config
fkn-prd-v4.1.md       # product requirements document
```

## Compatibility

MCP status today:

- raw stdio MCP requests: tested
- raw HTTP+SSE MCP requests: tested
- GitHub Copilot integration: unverified
- Claude Code integration: unverified

That means `fkn serve` is usable now for experimentation, but client-specific compatibility should still be treated as provisional until tested directly.

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

`fkn docs` now serves embedded copies of the README, user guide, MCP guide, and release guide so installed binaries can explain themselves offline.

`fkn init --agents` now generates a richer `AGENTS_FKN.md` that summarizes tasks, guards, scopes, prompts, context, MCP settings, and watch paths for agents.

## Contributing

Contributions are welcome. For the best starting point, read [CONTRIBUTING.md](/Users/josephfrost/code/fkn/CONTRIBUTING.md).

If you want to help early, the highest-leverage areas are:

- real-world `fkn.yaml` examples
- command UX rough edges
- JSON contract feedback
- guard/context implementation
- expanded tests for init, context, watch, runner execution, MCP, and CLI edge cases

## License

This project is licensed under the MIT License. See [LICENSE](/Users/josephfrost/code/fkn/LICENSE).
