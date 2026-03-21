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
