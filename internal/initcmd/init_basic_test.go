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
	msg, err := Run(dir, Options{})
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
	if !strings.Contains(string(cfg), "desc: Main CLI commands and closely-related execution packages.") {
		t.Fatalf("fkn.yaml = %q, want starter scope description", string(cfg))
	}
	if !strings.Contains(string(cfg), "groups:\n  core:\n") {
		t.Fatalf("fkn.yaml = %q, want starter task group", string(cfg))
	}
	if !strings.Contains(string(cfg), "default: check") {
		t.Fatalf("fkn.yaml = %q, want starter default task", string(cfg))
	}
	if !strings.Contains(string(cfg), "agent:\n  accrue_knowledge: false\n") {
		t.Fatalf("fkn.yaml = %q, want starter agent config", string(cfg))
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

	msg, err := Run(dir, Options{})
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

func TestRunFromRepoInfersGoTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir, Options{FromRepo: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(msg, "scaffolded tasks from existing repo files") {
		t.Fatalf("Run() message = %q, want inferred scaffold note", msg)
	}
	if !strings.Contains(msg, "tip: run `fkn init --docs`") {
		t.Fatalf("Run() message = %q, want docs hint", msg)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	if !strings.Contains(got, "project: "+filepath.Base(dir)) {
		t.Fatalf("fkn.yaml = %q, want inferred project name", got)
	}
	if !strings.Contains(got, "cmd: go test ./...") {
		t.Fatalf("fkn.yaml = %q, want Go test task", got)
	}
	if !strings.Contains(got, "default: check") {
		t.Fatalf("fkn.yaml = %q, want inferred default task", got)
	}
	if !strings.Contains(got, "agent:\n  accrue_knowledge: true\n") {
		t.Fatalf("fkn.yaml = %q, want inferred agent accrual enabled", got)
	}
	if !strings.Contains(got, "cmd: go build ./...") {
		t.Fatalf("fkn.yaml = %q, want Go build task", got)
	}
	if !strings.Contains(got, "steps:\n      - test\n      - build") {
		t.Fatalf("fkn.yaml = %q, want check pipeline", got)
	}
	if !strings.Contains(got, "go.mod") {
		t.Fatalf("fkn.yaml = %q, want inferred watch paths", got)
	}
}

func TestRunFromRepoPrefersMakeTargets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	makefile := ".PHONY: test build check\n\ntest:\n\tgo test ./...\n\nbuild:\n\tgo build ./...\n\ncheck:\n\ttest build\n"
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	if !strings.Contains(got, "cmd: make test") {
		t.Fatalf("fkn.yaml = %q, want make-backed test task", got)
	}
	if !strings.Contains(got, "default: check") {
		t.Fatalf("fkn.yaml = %q, want inferred default task", got)
	}
	if !strings.Contains(got, "cmd: make build") {
		t.Fatalf("fkn.yaml = %q, want make-backed build task", got)
	}
	if !strings.Contains(got, "cmd: make check") {
		t.Fatalf("fkn.yaml = %q, want make-backed check task", got)
	}
}

func TestRunFromRepoInfersJustRecipesAndAliases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	justfile := `
alias b := build

build target profile="debug":
	echo {{target}} {{profile}}

test:
	just build app

[private]
hidden:
	echo hidden

_helper:
	echo helper
`
	if err := os.WriteFile(filepath.Join(dir, "justfile"), []byte(justfile), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	for _, want := range []string{
		"cmd: just build {{params.target}} {{params.profile}}",
		"cmd: just test",
		"aliases:\n  b: build\n",
		"target:\n        desc: Value for the target recipe parameter\n        env: TARGET\n        required: true\n        position: 1\n",
		"profile:\n        desc: Value for the profile recipe parameter\n        env: PROFILE\n        default: \"debug\"\n",
		"default: check",
		"justfile",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want %q", got, want)
		}
	}
	for _, unwanted := range []string{"hidden:", "_helper:"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("fkn.yaml = %q, did not want private recipe %q", got, unwanted)
		}
	}
}

func TestRunFromRepoReadsCapitalizedJustfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Justfile"), []byte("build:\n\techo build\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	if !strings.Contains(got, "cmd: just build") {
		t.Fatalf("fkn.yaml = %q, want Justfile-backed task", got)
	}
	if !strings.Contains(got, "Justfile") {
		t.Fatalf("fkn.yaml = %q, want Justfile watch path", got)
	}
}
