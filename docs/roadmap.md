# fkn Roadmap

This roadmap focuses on the major features that would make `fkn` feel more complete as a day-to-day task runner and repo interface.

`fkn` is intentionally YAML-driven. That is a product choice, not a temporary compromise. The goal is to use a format that is already familiar to teams, easy for tooling to parse, and consistent with the wider ecosystem of config-driven developer tools.

## Near-Term

- richer task parameters
  - positional arguments
  - optional variadic arguments
  - clearer defaults and validation rules
  - better command-line ergonomics than repeated `--param`
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
  - JSON schema
  - autocomplete and inline validation in editors

## Agent Features

- richer MCP schemas
  - task params, defaults, and validation surfaced more explicitly
- safer task capabilities
  - clearer distinction between safe, destructive, and environment-sensitive tasks
- generated agent guidance with stronger repo awareness
  - better `AGENTS_FKN.md` content
  - more scope- and prompt-aware output
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
