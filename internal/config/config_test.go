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
