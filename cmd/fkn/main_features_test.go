package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunListShowsReadableMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
default: verify
tasks:
  test:
    desc: Run tests
    cmd: echo test
  build:
    desc: Build the project
    cmd: echo build
    scope: cli
    needs:
      - test
    params:
      target:
        desc: Build target
        env: TARGET
        required: true
      profile:
        desc: Build profile
        env: PROFILE
aliases:
  b: build
  verify: build
groups:
  core:
    desc: Everyday development commands
    tasks:
      - build
scopes:
  cli:
    desc: CLI command surface
    paths:
      - cmd/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"list"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(list) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{
		"core",
		"Everyday development commands",
		"Build the project",
		"[cmd | default | scope:cli | aliases:b,verify | groups:core | needs:test | params:--profile?,--target]",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunListJSONIncludesGroups(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
groups:
  qa:
    desc: Verification tasks
    tasks:
      - test
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"list", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(list --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"groups": [`, `"name": "qa"`, `"desc": "Verification tasks"`, `"groups": [`, `"qa"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunRepairJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: |
      printf '%s\n' 'cmd/fkn/main_test.go:1: boom' >&2
      exit 1
    scope: cli
    error_format: go_test
guards:
  default:
    steps:
      - test
scopes:
  cli:
    desc: CLI command surface
    paths:
      - cmd/fkn/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"repair", "--json"}, stdout, stderr)
	if code == 0 {
		t.Fatal("run(repair --json) code = 0, want failure exit code")
	}
	if readStderr() != "" {
		t.Fatalf("stderr = %q, want empty", readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"guard": "default"`, `"failures":`, `"scope":`, `"markdown":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunExplainJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  check:
    desc: Run checks
    cmd: printf ok
    scope: cli
scopes:
  cli:
    desc: Runner-facing CLI scope
    paths:
      - internal/runner/
codemap:
  packages:
    internal/runner:
      desc: Execution engine
      key_types:
        - Runner
      entry_points:
        - Runner.Run
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"explain", "internal/runner", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(explain internal/runner --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"kind": "package"`, `"target": "internal/runner"`, `"markdown":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunContextAboutJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
project: demo
tasks:
  check:
    desc: Run MCP transport checks
    cmd: printf ok
    scope: mcp
scopes:
  mcp:
    desc: MCP transport and protocol work
    paths:
      - internal/mcp/
codemap:
  packages:
    internal/mcp:
      desc: MCP transport and JSON-RPC handling
      entry_points:
        - Server.ServeStdio
  glossary:
    transport: How MCP messages move between client and server
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"context", "--about", "transport", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(context --about transport --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"about": "transport"`, `"Matching Codemap"`, `internal/mcp`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}
