package repair

import (
	"io"
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
	"github.com/neural-chilli/fkn/internal/guard"
	"github.com/neural-chilli/fkn/internal/runner"
)

func TestGenerateIncludesFailuresScopesAndMarkdown(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {
				Desc:        "Run tests",
				Cmd:         `printf '%s\n' 'internal/repair/repair_test.go:12: boom' >&2; exit 1`,
				Scope:       "cli",
				ErrorFormat: "go_test",
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test"}},
		},
		Scopes: map[string]config.Scope{
			"cli": {Desc: "CLI and repair workflow code", Paths: []string{"cmd/fkn/", "internal/repair/"}},
		},
		Context: config.ContextConfig{
			Caps: config.ContextCaps{GitDiffLines: 10},
		},
	}

	out, err := New(cfg, repoRoot, guard.New(cfg, repoRoot, runner.New(cfg, repoRoot))).Generate(Options{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if out.Overall != runner.StatusFail {
		t.Fatalf("Overall = %q, want fail", out.Overall)
	}
	if len(out.Failures) != 1 {
		t.Fatalf("len(Failures) = %d, want 1", len(out.Failures))
	}
	failure := out.Failures[0]
	if failure.Task != "test" {
		t.Fatalf("Failure.Task = %q, want test", failure.Task)
	}
	if failure.Scope == nil || failure.Scope.Name != "cli" {
		t.Fatalf("Failure.Scope = %+v, want cli scope", failure.Scope)
	}
	if failure.Scope.Desc != "CLI and repair workflow code" {
		t.Fatalf("Failure.Scope.Desc = %q, want scope description", failure.Scope.Desc)
	}
	if len(failure.Errors) != 1 || failure.Errors[0].File != "internal/repair/repair_test.go" {
		t.Fatalf("Failure.Errors = %+v, want parsed error", failure.Errors)
	}
	for _, want := range []string{"# fkn repair", "## Guard Status", "## Failures", "## Suggested Next Action", "Scope intent: CLI and repair workflow code"} {
		if !strings.Contains(out.Markdown, want) {
			t.Fatalf("Markdown = %q, want %q", out.Markdown, want)
		}
	}
}

func TestGenerateReportsPassingGuard(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {Desc: "Run tests", Cmd: "printf ok"},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test"}},
		},
	}

	out, err := New(cfg, repoRoot, guard.New(cfg, repoRoot, runner.New(cfg, repoRoot))).Generate(Options{})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if out.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", out.ExitCode)
	}
	if len(out.Failures) != 0 {
		t.Fatalf("Failures = %+v, want none", out.Failures)
	}
	if !strings.Contains(out.Markdown, "No failing steps.") {
		t.Fatalf("Markdown = %q, want passing summary", out.Markdown)
	}
	if out.SuggestedNextAction != "Guard passed. No repair is needed right now." {
		t.Fatalf("SuggestedNextAction = %q, want passing message", out.SuggestedNextAction)
	}
}

func TestGuardRunnerAggregatesPipelineErrorsForRepair(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {
				Desc:        "Run tests",
				Cmd:         `printf '%s\n' 'internal/runner/runner_test.go:9: broken' >&2; exit 1`,
				ErrorFormat: "go_test",
			},
			"check": {
				Desc:  "Run checks",
				Steps: []string{"test"},
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"check"}},
		},
	}

	report, err := guard.New(cfg, repoRoot, runner.New(cfg, repoRoot)).Run("default", runner.Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(report.Steps) != 1 {
		t.Fatalf("len(report.Steps) = %d, want 1", len(report.Steps))
	}
	if len(report.Steps[0].Errors) != 1 {
		t.Fatalf("Errors = %+v, want aggregated child step errors", report.Steps[0].Errors)
	}
}
