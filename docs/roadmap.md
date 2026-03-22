# fkn Roadmap

This roadmap focuses on the major features that would make `fkn` feel more complete as a day-to-day task runner and repo interface.

`fkn` is intentionally YAML-driven. That is a product choice, not a temporary compromise. The goal is to use a format that is already familiar to teams, easy for tooling to parse, and consistent with the wider ecosystem of config-driven developer tools.

## Near-Term

- richer task parameters
  - clearer defaults and validation rules
  - richer mixed positional/named invocation patterns
- richer help and list output
  - ordering and filtering options
- improved importer coverage

## Workflow Features

- dotenv and environment behavior
  - richer `.env` support
  - clearer precedence rules

## Authoring Features

- reusable values and variables
  - shared values without repeating command fragments everywhere
- imports or multi-file config
  - useful for larger repos without turning `fkn.yaml` into a monolith
- comments, metadata, and docs fields with stronger conventions
  - more room for human and agent guidance without cluttering task execution
- schema and editor support
  - shipped: base JSON schema in `fkn.schema.json`
  - next: smoother editor integration guidance and broader validation coverage

## Agent Features

- richer MCP schemas
  - task params, defaults, and validation surfaced more explicitly
- safer task capabilities
  - clearer distinction between safe, destructive, and environment-sensitive tasks
- generated human and agent docs with stronger repo awareness
  - richer `HUMANS.md`, `AGENTS.md`, and `CLAUDE.md` output
  - more scope-, prompt-, and codemap-aware guidance
- compatibility validation against real external MCP clients
  - not just raw protocol smoke tests

## Distribution Features

- packaged releases
  - GitHub release assets
  - Homebrew and other install paths
- shell completions
  - zsh, bash, fish, PowerShell
- better version and release metadata
  - clearer binary provenance
  - easier upgrade visibility

## Principle

The long-term goal is not to become a clever DSL for its own sake.

The goal is:

- one obvious repo interface
- strong machine-readable structure
- low-friction authoring in YAML
- enough runner features that teams do not need to fall back to ad hoc scripts for common workflows
