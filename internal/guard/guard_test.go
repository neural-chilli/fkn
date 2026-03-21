package guard

import (
	"io"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
	"github.com/neural-chilli/fkn/internal/runner"
)

func TestRunAllowsPipelineGuardSteps(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {
				Desc: "Run tests",
				Cmd:  "printf test",
			},
			"build": {
				Desc: "Build",
				Cmd:  "printf build",
			},
			"check": {
				Desc:  "Run checks",
				Steps: []string{"test", "build"},
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"check"}},
		},
	}

	report, err := New(cfg, repoRoot, runner.New(cfg, repoRoot)).Run("default", runner.Options{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Overall != runner.StatusPass {
		t.Fatalf("Overall = %q, want pass", report.Overall)
	}
	if len(report.Steps) != 1 {
		t.Fatalf("len(report.Steps) = %d, want 1", len(report.Steps))
	}
	if report.Steps[0].Name != "check" {
		t.Fatalf("step name = %q, want check", report.Steps[0].Name)
	}
	if report.Steps[0].Status != runner.StatusPass {
		t.Fatalf("step status = %q, want pass", report.Steps[0].Status)
	}
}

func TestRunIncludesStructuredErrorsInGuardReport(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {
				Desc:        "Run tests",
				Cmd:         `printf '%s\n' 'internal/guard/guard_test.go:42: guard failed' >&2; exit 1`,
				ErrorFormat: "go_test",
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test"}},
		},
	}

	report, err := New(cfg, repoRoot, runner.New(cfg, repoRoot)).Run("default", runner.Options{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(report.Steps) != 1 {
		t.Fatalf("len(report.Steps) = %d, want 1", len(report.Steps))
	}
	if len(report.Steps[0].Errors) != 1 {
		t.Fatalf("len(report.Steps[0].Errors) = %d, want 1", len(report.Steps[0].Errors))
	}
	if got := report.Steps[0].Errors[0]; got.File != "internal/guard/guard_test.go" || got.Line != 42 || got.Message != "guard failed" {
		t.Fatalf("Errors[0] = %+v, want parsed guard error", got)
	}
}
