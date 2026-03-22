package runner

import (
	"io"
	"testing"
	"time"

	"github.com/neural-chilli/fkn/internal/config"
)

func TestParallelPipelineCancelsCommandTaskViaParentContext(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"fail": {Desc: "fail", Cmd: "exit 1"},
		"slow": {
			Desc:      "slow",
			Cmd:       "2",
			Shell:     "/bin/sleep",
			ShellArgs: []string{},
		},
		"par": {Desc: "par", Steps: []string{"fail", "slow"}, Parallel: true},
	}}, repoRoot)

	started := time.Now()
	result, err := r.Run("par", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("elapsed = %v, want cancellation before 1s", elapsed)
	}
	if result.Steps[1].Status != StatusCancelled && result.Steps[1].Status != StatusFail {
		t.Fatalf("slow step status = %q, want cancelled or fail after parent cancellation", result.Steps[1].Status)
	}
}
