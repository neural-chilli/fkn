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
	if !strings.Contains(got, "cmd: make build") {
		t.Fatalf("fkn.yaml = %q, want make-backed build task", got)
	}
	if !strings.Contains(got, "cmd: make check") {
		t.Fatalf("fkn.yaml = %q, want make-backed check task", got)
	}
}

func TestRunWithAgentsWritesCompanionFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	agents := "## Local Rules\n\nKeep this section.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir, Options{Agents: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(msg, "wrote AGENTS_FKN.md") {
		t.Fatalf("Run() message = %q, want AGENTS_FKN note", msg)
	}

	agentFKN, err := os.ReadFile(filepath.Join(dir, "AGENTS_FKN.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotAgentFKN := string(agentFKN)
	if !strings.Contains(gotAgentFKN, "# AGENTS_FKN") {
		t.Fatalf("AGENTS_FKN.md = %q, want header", gotAgentFKN)
	}
	if !strings.Contains(gotAgentFKN, "`test`: Run the test suite") {
		t.Fatalf("AGENTS_FKN.md = %q, want task summary", gotAgentFKN)
	}

	agentRoot, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotAgentRoot := string(agentRoot)
	if !strings.Contains(gotAgentRoot, "Keep this section.") {
		t.Fatalf("AGENTS.md = %q, want original content preserved", gotAgentRoot)
	}
	if !strings.Contains(gotAgentRoot, "## fkn Workflow") {
		t.Fatalf("AGENTS.md = %q, want fkn workflow block", gotAgentRoot)
	}
}

func TestRunWithAgentsIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := Run(dir, Options{Agents: true}); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir, Options{Agents: true})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if !strings.Contains(msg, "AGENTS.md already includes fkn guidance") {
		t.Fatalf("Run() message = %q, want existing guidance note", msg)
	}
	after, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("AGENTS.md changed unexpectedly on second run:\nBEFORE:\n%s\nAFTER:\n%s", string(before), string(after))
	}
}
