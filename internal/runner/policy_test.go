package runner

import (
	"io"
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
)

func TestRunUnsafeTaskRequiresApproval(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"deploy": {Desc: "deploy", Cmd: "printf deploy", Safety: "external"},
	})

	_, err := r.Run("deploy", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err == nil {
		t.Fatal("Run() error = nil, want safety approval error")
	}
	if !strings.Contains(err.Error(), "--allow-unsafe") {
		t.Fatalf("err = %v, want allow-unsafe guidance", err)
	}
}

func TestRunUnsafeTaskAllowsDryRunWithoutApproval(t *testing.T) {
	t.Parallel()

	outFile := mustTempFile(t)
	defer outFile.Close()

	r := newTestRunner(t, map[string]config.Task{
		"deploy": {Desc: "deploy", Cmd: "printf deploy", Safety: "external"},
	})

	result, err := r.Run("deploy", Options{DryRun: true, Stdout: outFile, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusPass {
		t.Fatalf("result = %+v, want pass dry run", result)
	}
}

func TestRunUnsafeTaskAllowsExplicitApproval(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"deploy": {Desc: "deploy", Cmd: "printf deploy", Safety: "external"},
	})

	result, err := r.Run("deploy", Options{AllowUnsafe: true, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "deploy" {
		t.Fatalf("Stdout = %q, want deploy", result.Stdout)
	}
}
