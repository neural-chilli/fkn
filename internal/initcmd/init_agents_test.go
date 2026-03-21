package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
)

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
	if !strings.Contains(gotAgentFKN, "Scope Description: Main CLI commands and closely-related execution packages.") {
		t.Fatalf("AGENTS_FKN.md = %q, want scope description", gotAgentFKN)
	}
	if !strings.Contains(gotAgentFKN, "## Context") {
		t.Fatalf("AGENTS_FKN.md = %q, want context section", gotAgentFKN)
	}
	if !strings.Contains(gotAgentFKN, "Command: `go test ./...`") {
		t.Fatalf("AGENTS_FKN.md = %q, want command detail", gotAgentFKN)
	}
	if !strings.Contains(gotAgentFKN, "## Groups") {
		t.Fatalf("AGENTS_FKN.md = %q, want groups section", gotAgentFKN)
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

func TestRenderAgentsFKNIncludesScopesAndPrompts(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Project:     "demo",
		Description: "Demo repo",
		Tasks: map[string]config.Task{
			"check": {
				Desc:  "Run checks",
				Needs: []string{"setup"},
				Steps: []string{"test", "build"},
				Scope: "backend",
			},
			"setup": {
				Desc: "Prepare fixtures",
				Cmd:  "make setup",
			},
			"test": {
				Desc: "Run tests",
				Cmd:  "go test ./...",
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test", "build"}},
		},
		Scopes: map[string]config.Scope{
			"backend": {Desc: "Backend workflows", Paths: []string{"cmd/", "internal/"}},
		},
		Prompts: map[string]config.Prompt{
			"continue-backend": {Desc: "Continue backend work"},
		},
		Groups: map[string]config.Group{
			"qa": {Desc: "Verification tasks", Tasks: []string{"test", "check"}},
		},
		Context: config.ContextConfig{
			AgentFiles: []string{"README.md"},
			Include:    []string{"cmd/", "internal/"},
		},
		Serve: config.ServeConfig{
			Transport: "stdio",
			Port:      8080,
		},
		Watch: config.WatchConfig{
			DebounceMS: 500,
			Paths:      []string{"cmd/", "internal/"},
		},
	}

	got, err := renderAgentsFKN(cfg)
	if err != nil {
		t.Fatalf("renderAgentsFKN() error = %v", err)
	}
	for _, want := range []string{
		"## Scopes",
		"`backend`: `cmd/`, `internal/`",
		"Description: Backend workflows",
		"## Groups",
		"`qa`: `test`, `check`",
		"Description: Verification tasks",
		"## Prompts",
		"`continue-backend`: Continue backend work",
		"Scope: `backend`",
		"Needs: `setup`",
		"Scope Description: Backend workflows",
		"Steps: `test`, `build`",
		"## MCP",
		"## Watch",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderAgentsFKN() = %q, want %q", got, want)
		}
	}
}
