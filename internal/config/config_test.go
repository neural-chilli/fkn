package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
project: demo
tasks:
  test:
    desc: Run tests
    cmd: echo test
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Serve.Transport != "stdio" {
		t.Fatalf("Serve.Transport = %q, want stdio", cfg.Serve.Transport)
	}
	if cfg.Serve.Port != 8080 {
		t.Fatalf("Serve.Port = %d, want 8080", cfg.Serve.Port)
	}
	if cfg.Serve.TokenEnv != "FKN_MCP_TOKEN" {
		t.Fatalf("Serve.TokenEnv = %q, want FKN_MCP_TOKEN", cfg.Serve.TokenEnv)
	}
	if cfg.Watch.DebounceMS != 500 {
		t.Fatalf("Watch.DebounceMS = %d, want 500", cfg.Watch.DebounceMS)
	}
	if cfg.Context.Caps.GitDiffLines != 200 {
		t.Fatalf("Context.Caps.GitDiffLines = %d, want 200", cfg.Context.Caps.GitDiffLines)
	}
}

func TestLoadRejectsUnknownTaskScope(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  check:
    desc: Check the repo
    cmd: echo check
    scope: missing
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want scope validation error")
	}
	if !strings.Contains(err.Error(), `references unknown scope "missing"`) {
		t.Fatalf("Load() error = %v, want unknown scope", err)
	}
}

func TestLoadRejectsCircularTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  one:
    desc: one
    steps: [two]
  two:
    desc: two
    steps: [one]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want cycle error")
	}
	if !strings.Contains(err.Error(), "circular task dependency") {
		t.Fatalf("Load() error = %v, want cycle message", err)
	}
}

func TestLoadRejectsCircularDependencies(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  one:
    desc: one
    cmd: echo one
    needs: [two]
  two:
    desc: two
    cmd: echo two
    needs: [one]
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want cycle error")
	}
	if !strings.Contains(err.Error(), "circular task dependency") {
		t.Fatalf("Load() error = %v, want cycle message", err)
	}
}

func TestLoadRejectsParamWithoutEnv(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  add-feature:
    desc: Add feature
    cmd: make add-feature
    params:
      feature:
        required: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want param validation error")
	}
	if !strings.Contains(err.Error(), `param "feature": env is required`) {
		t.Fatalf("Load() error = %v, want param env validation", err)
	}
}

func TestLoadRejectsUnknownSafety(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
    safety: spicy
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want safety validation error")
	}
	if !strings.Contains(err.Error(), `unknown safety "spicy"`) {
		t.Fatalf("Load() error = %v, want safety validation", err)
	}
}

func TestLoadRejectsDuplicateParamPositions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  build:
    desc: Build
    cmd: echo build
    params:
      target:
        env: TARGET
        position: 1
      profile:
        env: PROFILE
        position: 1
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want duplicate position error")
	}
	if !strings.Contains(err.Error(), `share position 1`) {
		t.Fatalf("Load() error = %v, want duplicate position validation", err)
	}
}

func TestLoadRejectsVariadicParamWithoutPosition(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  build:
    desc: Build
    cmd: echo build
    params:
      files:
        env: FILES
        variadic: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want variadic position error")
	}
	if !strings.Contains(err.Error(), `variadic params must also declare a position`) {
		t.Fatalf("Load() error = %v, want variadic position validation", err)
	}
}

func TestLoadRejectsVariadicParamThatIsNotLast(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  build:
    desc: Build
    cmd: echo build
    params:
      files:
        env: FILES
        position: 1
        variadic: true
      target:
        env: TARGET
        position: 2
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want variadic ordering error")
	}
	if !strings.Contains(err.Error(), `variadic param must have the highest position`) {
		t.Fatalf("Load() error = %v, want variadic ordering validation", err)
	}
}

func TestLoadRejectsAliasToUnknownTask(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
aliases:
  t: missing
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want alias validation error")
	}
	if !strings.Contains(err.Error(), `alias "t" references unknown task "missing"`) {
		t.Fatalf("Load() error = %v, want alias validation", err)
	}
}

func TestLoadRejectsAliasConflictWithTaskName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
aliases:
  test: test
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want alias conflict error")
	}
	if !strings.Contains(err.Error(), `alias "test" conflicts with task of the same name`) {
		t.Fatalf("Load() error = %v, want alias conflict validation", err)
	}
}

func TestLoadAcceptsDefaultAlias(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
default: verify
tasks:
  check:
    desc: Check the repo
    cmd: echo check
aliases:
  verify: check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Default != "verify" {
		t.Fatalf("Default = %q, want verify", cfg.Default)
	}
}

func TestLoadRejectsUnknownDefaultTask(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
default: missing
tasks:
  check:
    desc: Check the repo
    cmd: echo check
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want default validation error")
	}
	if !strings.Contains(err.Error(), `default task "missing" does not match a task or alias`) {
		t.Fatalf("Load() error = %v, want default validation", err)
	}
}

func TestLoadRejectsReservedParamName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
    params:
      dry-run:
        env: MODE
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want reserved param validation error")
	}
	if !strings.Contains(err.Error(), `task "test" param "dry-run" uses a reserved CLI flag name`) {
		t.Fatalf("Load() error = %v, want reserved param validation", err)
	}
}

func TestLoadRejectsUnknownErrorFormat(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: go test ./...
    error_format: nope
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want error_format validation error")
	}
	if !strings.Contains(err.Error(), `task "test": unknown error_format "nope"`) {
		t.Fatalf("Load() error = %v, want error_format validation", err)
	}
}

func TestLoadRejectsGroupWithUnknownTask(t *testing.T) {
	t.Parallel()

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
      - missing
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want group validation error")
	}
	if !strings.Contains(err.Error(), `group "qa" references unknown task "missing"`) {
		t.Fatalf("Load() error = %v, want group validation", err)
	}
}

func TestLoadRejectsUnknownTaskDependency(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  build:
    desc: Build the app
    cmd: echo build
    needs:
      - setup
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want dependency validation error")
	}
	if !strings.Contains(err.Error(), `task "build" references unknown dependency "setup"`) {
		t.Fatalf("Load() error = %v, want dependency validation", err)
	}
}

func TestLoadRejectsUnknownDefaultDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
defaults:
  dir: missing
tasks:
  test:
    desc: Run tests
    cmd: echo test
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want defaults.dir validation error")
	}
	if !strings.Contains(err.Error(), `defaults.dir "missing"`) {
		t.Fatalf("Load() error = %v, want defaults.dir validation", err)
	}
}

func TestLoadRejectsCodemapPackageWithoutDesc(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  test:
    desc: Run tests
    cmd: echo test
codemap:
  packages:
    internal/runner: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(filepath.Join(dir, "fkn.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want codemap validation error")
	}
	if !strings.Contains(err.Error(), `codemap package "internal/runner": desc is required`) {
		t.Fatalf("Load() error = %v, want codemap validation", err)
	}
}
