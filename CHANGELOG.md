# Changelog

All notable changes to `fkn` will be documented in this file.

The format is intentionally lightweight and based on tagged releases.

## [v0.3.2] - 2026-03-23

### Changed

- fixed README and docs links so they render correctly on GitHub
- added `go vet ./...` to CI
- expanded `fkn.schema.json` field descriptions for better editor support

## [v0.3.1] - 2026-03-23

### Fixed

- fixed Makefile import so assigned Make variables are no longer inferred as required task params

## [v0.3.0] - 2026-03-23

### Added

- shell completions plus best-effort completion installation for bash, zsh, fish, and PowerShell
- broader `init --from-repo` coverage for Rust, Python, Java, Docker Compose, richer `justfile`, and smarter `package.json` inference
- generated repo docs via `fkn init --docs` for `HUMANS.md`, `AGENTS.md`, and `CLAUDE.md`
- agent knowledge-accrual guidance via `agent.accrue_knowledge`

### Changed

- repositioned `fkn` around the repo-interface and docs-generation story
- hid MCP from the public CLI and docs surface while keeping the code dormant internally

## [v0.2.0] - 2026-03-22

### Added

- event-driven watch mode via `fsnotify` with polling fallback
- nested pipeline execution
- execution-policy gating for `destructive` and `external` tasks
- `fkn plan`, `fkn diff-plan`, `fkn repair`, and `fkn agent-brief`
- codemap-backed explanations and topic-targeted context
- task safety annotations
- task groups, reusable task dependencies, aliases, explicit default tasks, richer params, task-level shell config, and global default working directory
- release artifact workflow for tagged builds

## [v0.1.1] - 2026-03-22

### Fixed

- stamped binary versions correctly for release and local stamped builds
- fixed CI formatting issues so GitHub Actions passed cleanly

## [v0.1.0] - 2026-03-22

### Added

- initial public release of Flow Kit Neu
- YAML-driven task runner with command tasks and pipelines
- guards, scopes, prompts, context generation, watch mode, `init`, and validation
- JSON output for key commands
- repo-aware scaffolding from existing task surfaces
- generated docs and embedded offline docs
- initial release automation and install path via `go install`
