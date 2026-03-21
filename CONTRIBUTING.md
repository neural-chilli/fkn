# Contributing

Thanks for taking an interest in `fkn`.

## Philosophy

`fkn` should stay easy to explain, easy to adopt, and easy for agents to consume. When contributing, prefer changes that reinforce the core model:

- `fkn.yaml` is the operational API of the repo
- structured output is more important than clever formatting
- simple, predictable behavior beats feature sprawl

## Ways To Help

- report bugs or confusing CLI behavior
- propose schema improvements with concrete examples
- add tests around runner/config behavior
- improve docs and examples for real repositories
- implement roadmap items from the PRD in small, reviewable slices

## Development Setup

Prerequisites:

- Go 1.22+

Common commands:

```bash
make build
make test
go test ./...
go run ./cmd/fkn list
go run ./cmd/fkn check --dry-run
```

## Before Opening A PR

- keep changes focused
- update docs if behavior changes
- add or update tests when practical
- avoid changing JSON output shape casually
- prefer aligning with the PRD unless you are explicitly proposing a change

## Pull Requests

Good PRs usually include:

- the problem being solved
- the intended user-facing behavior
- tradeoffs or follow-up work
- sample command output if CLI behavior changed

## Design Notes

Please be cautious with:

- adding multiple overlapping ways to do the same thing
- inventing extra config files
- introducing hidden behavior that is hard for agents to infer
- breaking output contracts without a very explicit reason

For major changes, opening an issue or draft PR first is encouraged.
