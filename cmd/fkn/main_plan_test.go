package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPlanMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
    scope: cli
    safety: idempotent
  build:
    desc: Build binary
    cmd: go build ./...
    scope: cli
    safety: idempotent
    needs:
      - test
guards:
  default:
    steps:
      - test
      - build
groups:
  core:
    desc: Core workflows
    tasks:
      - test
      - build
scopes:
  cli:
    desc: CLI surface
    paths:
      - cmd/fkn/
codemap:
  packages:
    cmd/fkn:
      desc: CLI entrypoint
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"plan", "--file", "cmd/fkn/main.go"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(plan) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{"# fkn plan", "Matching Scopes", "Relevant Tasks", "Relevant Guards", "Relevant Codemap", "fkn help ", "fkn guard default"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunPlanJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
    scope: cli
scopes:
  cli:
    desc: CLI surface
    paths:
      - cmd/fkn/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"plan", "--json", "cmd/fkn/main.go"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(plan --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"files": [`, `"cmd/fkn/main.go"`, `"scopes": [`, `"tasks": [`, `"markdown":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunPlanRequiresFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"plan"}, stdout, stderr)
	if code == 0 {
		t.Fatal("run(plan) code = 0, want failure")
	}
	if !strings.Contains(readStderr(), "at least one file path is required") {
		t.Fatalf("stderr = %q, want file requirement error", readStderr())
	}
}
