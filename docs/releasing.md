# Releasing fkn

This project does not have an automated release pipeline yet, but it is ready for a simple tagged release flow.

## Before Tagging

Make sure all of these are true:

- `go test ./...` passes
- `go build ./cmd/fkn` succeeds
- [README.md](/Users/josephfrost/code/fkn/README.md) matches current behavior
- [docs/user-guide.md](/Users/josephfrost/code/fkn/docs/user-guide.md) reflects any user-facing changes
- [docs/mcp.md](/Users/josephfrost/code/fkn/docs/mcp.md) reflects any MCP-facing changes
- CI is green on `main`

## Suggested First Release

Use a small explicit version such as `v0.1.0`.

That signals:

- the tool is usable
- the core surface exists
- some contracts may still evolve

## Tagging

From a clean `main` branch:

```bash
git pull origin main
go test ./...
go build ./cmd/fkn
git tag v0.1.0
git push origin v0.1.0
```

## Release Notes

For the first release, keep the notes practical:

- what `fkn` is
- which commands are implemented
- whether MCP compatibility is verified or still provisional
- any known limitations worth calling out

## Install Path

Once tagged, users can install the CLI with:

```bash
go install github.com/neural-chilli/fkn/cmd/fkn@v0.1.0
```

For development builds, `@latest` also works:

```bash
go install github.com/neural-chilli/fkn/cmd/fkn@latest
```
