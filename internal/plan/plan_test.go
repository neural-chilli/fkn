package plan

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
)

func TestGenerateMatchesScopesTasksGuardsAndPackages(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {
				Desc:   "Run tests",
				Cmd:    "go test ./...",
				Scope:  "cli",
				Safety: "idempotent",
			},
			"build": {
				Desc:   "Build binary",
				Cmd:    "go build ./...",
				Scope:  "cli",
				Safety: "idempotent",
				Needs:  []string{"test"},
			},
			"check": {
				Desc:   "Run checks",
				Steps:  []string{"build"},
				Scope:  "cli",
				Safety: "idempotent",
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test", "build"}},
		},
		Groups: map[string]config.Group{
			"core": {Desc: "Core workflows", Tasks: []string{"test", "build", "check"}},
		},
		Scopes: map[string]config.Scope{
			"cli": {Desc: "CLI work", Paths: []string{"cmd/fkn/", "internal/runner/"}},
		},
		Codemap: config.CodemapConfig{
			Packages: map[string]config.CodemapPackage{
				"cmd/fkn":         {Desc: "CLI entrypoint"},
				"internal/runner": {Desc: "Execution engine"},
			},
		},
	}

	out, err := Generate(cfg, "/repo", []string{"cmd/fkn/main.go"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(out.Scopes) != 1 || out.Scopes[0].Name != "cli" {
		t.Fatalf("Scopes = %+v, want cli", out.Scopes)
	}
	if len(out.Tasks) != 3 {
		t.Fatalf("Tasks = %+v, want 3 related tasks", out.Tasks)
	}
	if len(out.Guards) != 1 || out.Guards[0].Name != "default" {
		t.Fatalf("Guards = %+v, want default", out.Guards)
	}
	if len(out.Groups) != 1 || out.Groups[0].Name != "core" {
		t.Fatalf("Groups = %+v, want core", out.Groups)
	}
	if len(out.Packages) == 0 || out.Packages[0].Target != "cmd/fkn" {
		t.Fatalf("Packages = %+v, want cmd/fkn", out.Packages)
	}
	if !strings.Contains(out.Markdown, "## Suggested Next Commands") {
		t.Fatalf("Markdown = %q, want next commands", out.Markdown)
	}
}

func TestGenerateNormalizesAbsolutePaths(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {Desc: "Run tests", Cmd: "go test ./...", Scope: "cli"},
		},
		Scopes: map[string]config.Scope{
			"cli": {Paths: []string{"cmd/fkn/"}},
		},
	}

	out, err := Generate(cfg, "/repo", []string{filepath.Join("/repo", "cmd/fkn/main.go")})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(out.Files) != 1 || out.Files[0] != "cmd/fkn/main.go" {
		t.Fatalf("Files = %+v, want normalized relative path", out.Files)
	}
}

func TestGenerateRequiresFiles(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {Desc: "Run tests", Cmd: "go test ./..."},
		},
	}

	_, err := Generate(cfg, "/repo", nil)
	if err == nil {
		t.Fatal("Generate() error = nil, want file validation")
	}
}
