package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
)

func TestUnknownTaskErrorSuggestsNearestTask(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"check": {Desc: "Check", Cmd: "echo check"},
			"build": {Desc: "Build", Cmd: "echo build"},
		},
	}

	err := unknownTaskError("chek", cfg)
	if err == nil {
		t.Fatal("unknownTaskError() = nil, want error")
	}
	if !strings.Contains(err.Error(), "Did you mean: check?") {
		t.Fatalf("unknownTaskError() = %v, want suggestion", err)
	}
}

func TestRunHelpForTask(t *testing.T) {
	repoRoot := repoRootForTest(t)
	restore := chdirForTest(t, repoRoot)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "check"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help check) code = %d, want 0; stderr=%s", code, readStderr())
	}

	output := readStdout()
	if !strings.Contains(output, "Description: Run the default local verification pipeline") {
		t.Fatalf("stdout = %q, want task description", output)
	}
	if !strings.Contains(output, "Default: true") {
		t.Fatalf("stdout = %q, want default marker", output)
	}
	if !strings.Contains(output, "Usage: fkn check [--dry-run] [--json]") {
		t.Fatalf("stdout = %q, want usage", output)
	}
	if !strings.Contains(output, "Steps:\n- fmt\n- test\n- build") {
		t.Fatalf("stdout = %q, want steps", output)
	}
}

func TestRunUnknownTaskShowsSuggestion(t *testing.T) {
	repoRoot := repoRootForTest(t)
	restore := chdirForTest(t, repoRoot)
	defer restore()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"chek"}, stdout, stderr)
	if code == 0 {
		t.Fatal("run(chek) code = 0, want failure")
	}
	if !strings.Contains(readStderr(), `unknown task "chek". Did you mean: check?`) {
		t.Fatalf("stderr = %q, want suggestion", readStderr())
	}
}

func TestRunHelpForAlias(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
default: b
tasks:
  build:
    desc: Build the project
    cmd: echo build
aliases:
  b: build
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "b"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help b) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, "Alias For: build") {
		t.Fatalf("stdout = %q, want alias target", output)
	}
	if !strings.Contains(output, "Default: true") {
		t.Fatalf("stdout = %q, want default marker", output)
	}
	if !strings.Contains(output, "Aliases: b") {
		t.Fatalf("stdout = %q, want aliases list", output)
	}
	if !strings.Contains(output, "Usage: fkn b [--dry-run] [--json]") {
		t.Fatalf("stdout = %q, want alias usage", output)
	}
}

func TestRunTaskViaAlias(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  build:
    desc: Build the project
    cmd: printf build
aliases:
  b: build
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"b"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(b) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "build") {
		t.Fatalf("stdout = %q, want task output", readStdout())
	}
}

func TestRunNoArgsUsesDefaultTask(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
default: verify
tasks:
  check:
    desc: Check the project
    cmd: printf check
aliases:
  verify: check
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run(nil, stdout, stderr)
	if code != 0 {
		t.Fatalf("run() code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "check") {
		t.Fatalf("stdout = %q, want default task output", readStdout())
	}
}

func TestRunTaskAcceptsDirectParamFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  add-feature:
    desc: Add a feature
    cmd: printf {{params.feature}}
    params:
      feature:
        desc: Feature name
        env: FEATURE
        required: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"add-feature", "--feature", "auth"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(add-feature --feature auth) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "auth") {
		t.Fatalf("stdout = %q, want direct param flag output", got)
	}
}

func TestRunTaskAcceptsDirectParamEqualsFlag(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  add-feature:
    desc: Add a feature
    cmd: printf {{params.feature}}
    params:
      feature:
        desc: Feature name
        env: FEATURE
        required: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"add-feature", "--feature=auth"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(add-feature --feature=auth) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "auth") {
		t.Fatalf("stdout = %q, want direct param flag output", got)
	}
}

func TestRunHelpShowsShellConfiguration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  strict:
    desc: Run in strict shell mode
    cmd: printf ok
    shell: /bin/sh
    shell_args:
      - -eu
      - -c
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "strict"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help strict) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, "Shell: /bin/sh") {
		t.Fatalf("stdout = %q, want shell", output)
	}
	if !strings.Contains(output, "Shell Args: -eu -c") {
		t.Fatalf("stdout = %q, want shell args", output)
	}
}

func TestRunHelpShowsInheritedDefaultDir(t *testing.T) {
	dir := t.TempDir()
	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
defaults:
  dir: app
tasks:
  test:
    desc: Run tests
    cmd: printf ok
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "test"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help test) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, "Dir: app (inherited default)") {
		t.Fatalf("stdout = %q, want inherited default dir", output)
	}
}

func TestRunValidateReportsSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: printf ok
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"validate"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(validate) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if got := readStdout(); !strings.Contains(got, "fkn.yaml is valid") {
		t.Fatalf("stdout = %q, want validation success", got)
	}
}

func TestRunValidateJSONReportsFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  broken:
    desc: Broken
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"validate", "--json"}, stdout, stderr)
	if code == 0 {
		t.Fatal("run(validate --json) code = 0, want failure")
	}
	if !strings.Contains(readStderr(), `set exactly one of cmd or steps`) {
		t.Fatalf("stderr = %q, want validation error", readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, `"valid": false`) || !strings.Contains(output, `"error":`) {
		t.Fatalf("stdout = %q, want JSON failure payload", output)
	}
}

func TestRunContextJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
project: demo
tasks:
  test:
    desc: Run tests
    cmd: printf ok
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"context", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(context --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{`"project": "demo"`, `"repo_root":`, `"sections":`, `"markdown":`} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunListShowsReadableMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
default: verify
tasks:
  build:
    desc: Build the project
    cmd: echo build
    scope: cli
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
scopes:
  cli:
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
		"Build the project",
		"[cmd | default | scope:cli | aliases:b,verify | params:--profile?,--target]",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunDocsPrintsEmbeddedGuide(t *testing.T) {
	t.Parallel()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"docs", "user-guide"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(docs user-guide) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), "# fkn User Guide") {
		t.Fatalf("stdout = %q, want embedded docs output", readStdout())
	}
}

func TestRunDocsList(t *testing.T) {
	t.Parallel()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"docs", "--list"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(docs --list) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	for _, want := range []string{"readme", "user-guide", "mcp", "releasing"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestResolvedVersionPrefersExplicitLdflagsValue(t *testing.T) {
	t.Parallel()

	previous := version
	version = "v9.9.9"
	t.Cleanup(func() {
		version = previous
	})

	if got := resolvedVersion(); got != "v9.9.9" {
		t.Fatalf("resolvedVersion() = %q, want explicit version", got)
	}
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func chdirForTest(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatal(err)
		}
	}
}

func tempOutputFile(t *testing.T) (*os.File, func() string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fkn-out-*")
	if err != nil {
		t.Fatal(err)
	}
	return f, func() string {
		if _, err := f.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
}

func TestRunScopeAllowsFlagAfterPositional(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: printf ok
scopes:
  cli:
    - cmd/
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"scope", "cli", "--json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(scope cli --json) code = %d, want 0; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStdout(), `"scope": "cli"`) {
		t.Fatalf("stdout = %q, want JSON scope output", readStdout())
	}
}

func TestRunDocsRejectsUnknownFlag(t *testing.T) {
	t.Parallel()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"docs", "--wat", "user-guide"}, stdout, stderr)
	if code != 2 {
		t.Fatalf("run(docs --wat user-guide) code = %d, want 2; stderr=%s", code, readStderr())
	}
	if !strings.Contains(readStderr(), `unknown flag "--wat"`) {
		t.Fatalf("stderr = %q, want unknown flag error", readStderr())
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
