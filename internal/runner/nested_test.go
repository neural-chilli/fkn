package runner

import (
	"io"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
)

func TestRunPipelineAllowsNestedPipelines(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := newTestRunner(t, map[string]config.Task{
		"leaf":   {Desc: "leaf", Cmd: `printf leaf > ready.txt`},
		"inner":  {Desc: "inner", Steps: []string{"leaf"}},
		"outer":  {Desc: "outer", Steps: []string{"inner", "report"}},
		"report": {Desc: "report", Cmd: `cat ready.txt`},
	})
	r.repoRoot = repoRoot

	result, err := r.Run("outer", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", result.Status)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("len(Steps) = %d, want 2", len(result.Steps))
	}
	if result.Steps[0].Type != "pipeline" {
		t.Fatalf("Steps[0].Type = %q, want pipeline", result.Steps[0].Type)
	}
	if len(result.Steps[0].Steps) != 1 || result.Steps[0].Steps[0].Name != "leaf" {
		t.Fatalf("nested Steps = %+v, want inner leaf step", result.Steps[0].Steps)
	}
	if result.Steps[1].Stdout == nil || *result.Steps[1].Stdout != "leaf" {
		t.Fatalf("Steps[1].Stdout = %#v, want leaf", result.Steps[1].Stdout)
	}
}

func TestRunNestedPipelineAggregatesChildErrors(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"leaf": {
			Desc:        "leaf",
			Cmd:         `printf '%s\n' 'pkg/mod.py:9: assertion failed' >&2; exit 1`,
			ErrorFormat: "pytest",
		},
		"inner": {Desc: "inner", Steps: []string{"leaf"}},
		"outer": {Desc: "outer", Steps: []string{"inner"}},
	})

	result, err := r.Run("outer", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(result.Steps))
	}
	if len(result.Steps[0].Errors) != 1 {
		t.Fatalf("len(Steps[0].Errors) = %d, want 1", len(result.Steps[0].Errors))
	}
	if got := result.Steps[0].Errors[0]; got.File != "pkg/mod.py" || got.Line != 9 || got.Message != "assertion failed" {
		t.Fatalf("Errors[0] = %+v, want aggregated nested error", got)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("len(Errors) = %d, want 1", len(result.Errors))
	}
}
