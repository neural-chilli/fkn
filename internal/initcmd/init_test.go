package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCreatesStarterFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	msg, err := Run(dir)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(msg, "created fkn.yaml") {
		t.Fatalf("Run() message = %q, want created fkn.yaml", msg)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(fkn.yaml) error = %v", err)
	}
	if !strings.Contains(string(cfg), "tasks:") {
		t.Fatalf("fkn.yaml = %q, want starter tasks", string(cfg))
	}

	gitignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile(.gitignore) error = %v", err)
	}
	if string(gitignore) != ".fkn/\n" {
		t.Fatalf(".gitignore = %q, want .fkn/ entry", string(gitignore))
	}
}

func TestRunIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	original := "project: keep-me\n\ntasks:\n  test:\n    desc: test\n    cmd: echo test\n"
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("bin/\n.fkn/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(msg, "fkn.yaml already exists") {
		t.Fatalf("Run() message = %q, want existing config note", msg)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg) != original {
		t.Fatalf("fkn.yaml changed unexpectedly: %q", string(cfg))
	}
}
