# fkn MCP Guide

`fkn serve` exposes repo tasks as MCP tools.

This guide explains what is implemented today, how to run it, and what to expect from client compatibility.

## Current Compatibility Status

Current status:

- raw stdio MCP requests: tested
- raw HTTP+SSE MCP requests: tested
- `tools/list`: tested
- `tools/call`: tested
- `resources/list`: tested
- `resources/read`: tested
- GitHub Copilot custom MCP integration: unverified
- Claude Code custom MCP integration: unverified

That means the current implementation is suitable for experimentation and local workflows, but it should not yet be advertised as officially verified against specific third-party clients.

## What `fkn serve` Exposes

Every task with `agent: true` is exposed as an MCP tool.

`fkn serve` also exposes read-only MCP resources for repo state that agents may want to inspect without calling a task.

Example task:

```yaml
tasks:
  test:
    desc: Run the test suite
    cmd: go test ./...
```

That becomes a tool with:

- name: `test`
- description: `Run the test suite`
- annotations including `fknSafety` plus MCP hints like `readOnlyHint`, `idempotentHint`, `destructiveHint`, or `openWorldHint` when applicable
- input schema supporting:
  - named task params from `fkn.yaml`
  - `env`
  - `dry_run`

Tasks with `agent: false` are hidden from the MCP manifest.
Tasks can also declare `safety: safe | idempotent | destructive | external`, which is surfaced in tool annotations for agent callers.

Current resources:

- `fkn://context`
- `fkn://context.json`
- `fkn://guard/last` when a cached guard result exists
- `fkn://scope/<name>` for configured scopes

## Running the Server

### Stdio

```bash
fkn serve
```

This is the default transport and the best starting point for local agent integrations.

### HTTP + SSE

```bash
fkn serve --http --port 8080
```

Current endpoints:

- `GET /sse`
- `POST /messages`

The SSE endpoint emits:

```text
event: endpoint
data: /messages
```

## Config

Example:

```yaml
serve:
  transport: stdio
  port: 8080
  token_env: FKN_MCP_TOKEN
```

Defaults:

- `transport: stdio`
- `port: 8080`
- `token_env: FKN_MCP_TOKEN`

## HTTP Auth

HTTP mode can require a bearer token.

The server reads the token from the environment variable named by `serve.token_env`.

Example:

```bash
export FKN_MCP_TOKEN=secret-token
fkn serve --http --port 8080
```

Then clients should send:

```http
Authorization: Bearer secret-token
```

If the env var is unset, HTTP mode currently starts unauthenticated and prints a warning.

## Manifest Preview

You can inspect the MCP tool list without starting the server:

```bash
fkn list --mcp
```

This is the easiest way to confirm:

- which tasks are exposed
- which tasks are hidden by `agent: false`
- that task descriptions look good for tool consumers

## Methods Implemented Today

Current MCP methods:

- `initialize`
- `notifications/initialized`
- `ping`
- `tools/list`
- `tools/call`
- `resources/list`
- `resources/read`

That is enough for a useful first integration surface, but it is still intentionally small.

`initialize` now also includes a short `instructions` string summarizing the repo and pointing clients toward `tools/list`, `resources/list`, and `fkn guard` when guards are configured.

## Tool Call Behavior

`tools/call` routes directly into the `fkn` task runner.

Supported arguments today:

- named task params from the task schema
- `dry_run: boolean`
- `env: object`

Example request:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "test",
    "arguments": {
      "dry_run": true,
      "env": {
        "APP_ENV": "test"
      }
    }
  }
}
```

Example response shape:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "status=pass exit_code=0 duration_ms=0"
      }
    ],
    "structuredContent": {
      "task": "test",
      "status": "pass",
      "stdout": "",
      "stderr": "",
      "errors": [],
      "exit_code": 0,
      "duration_ms": 0,
      "started_at": "2026-03-21T12:15:29Z",
      "finished_at": "2026-03-21T12:15:29Z"
    },
    "isError": false
  }
}
```

If a task declares `error_format` in `fkn.yaml`, `structuredContent.errors` includes parsed diagnostics alongside raw stderr so MCP clients do not need to scrape compiler or test output themselves.

## Realistic Uses

Good current uses:

- exposing build/test/check tasks to an MCP-capable coding agent
- providing a repo-specific task manifest without a second config file
- letting an agent discover task descriptions via `tools/list`
- letting an agent read context, scope paths, and cached guard state via MCP resources
- allowing controlled `dry_run` task inspection

Less mature today:

- broad client compatibility claims
- advanced MCP protocol coverage
- production-hardened multi-user HTTP serving

## Can GitHub Copilot or Claude Code Use It?

Current honest answer:

- maybe, depending on their MCP connection support and expectations
- but this repo has not yet verified those integrations directly

So the right language today is:

- “MCP-compatible in principle”
- “raw protocol behavior tested”
- “client-specific compatibility unverified”

If you want to try it with a real client, start with:

1. `fkn list --mcp`
2. `fkn serve`
3. a minimal repo task set with clear descriptions

## Practical Advice

To make MCP use better:

- keep task names short and obvious
- make `desc` clear and action-oriented
- hide noisy/internal tasks with `agent: false`
- expose stable, safe workflows like `test`, `build`, `check`, and `guard`

Bad descriptions:

- `run stuff`
- `misc`
- `do thing`

Better descriptions:

- `Run the Go test suite`
- `Build the API binary`
- `Run local verification`

## Limitations

Current limitations worth documenting explicitly:

- hand-rolled MCP subset, not a full-featured SDK integration
- no client compatibility matrix yet
- HTTP mode is designed for local/team use, not public internet exposure
- no claim of production hardening

## Suggested Next Steps

If you want to strengthen the MCP story further:

- test against a real MCP-capable client
- document client-specific setup once verified
- expand protocol coverage as needed
- add more transport/auth tests
