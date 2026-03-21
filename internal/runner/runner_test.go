package runner

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
)

func TestRunCmdTaskUsesInvocationEnvOverride(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"env": {
			Desc: "env",
			Cmd:  "printf %s \"$FOO\"",
			Env:  map[string]string{"FOO": "task"},
		},
	})

	result, err := r.Run("env", Options{Stdout: io.Discard, Stderr: io.Discard, Env: map[string]string{"FOO": "override"}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "override" {
		t.Fatalf("Stdout = %q, want override", result.Stdout)
	}
}

func TestRunDryRunPrintsCommand(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"build": {Desc: "build", Cmd: "echo hi"},
	}}, repoRoot)

	outFile := mustTempFile(t)
	defer outFile.Close()

	result, err := r.Run("build", Options{DryRun: true, Stdout: outFile, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.ExitCode != 0 || result.Status != StatusPass {
		t.Fatalf("result = %+v, want pass", result)
	}
	if got := readFile(t, outFile.Name()); got != "echo hi\n" {
		t.Fatalf("dry run output = %q, want command", got)
	}
}

func TestSequentialPipelineStopsAfterFailure(t *testing.T) {
	repoRoot := t.TempDir()
	marker := filepath.Join(repoRoot, "ran-second.txt")
	r := New(&config.Config{Tasks: map[string]config.Task{
		"fail":   {Desc: "fail", Cmd: "exit 1"},
		"second": {Desc: "second", Cmd: "echo ran > ran-second.txt"},
		"pipe":   {Desc: "pipe", Steps: []string{"fail", "second"}},
	}}, repoRoot)

	result, err := r.Run("pipe", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", result.Status)
	}
	if result.Steps[1].Status != StatusCancelled {
		t.Fatalf("second step status = %q, want cancelled", result.Steps[1].Status)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("second step should not have run; stat err = %v", err)
	}
}

func TestSequentialPipelineContinueOnErrorRunsAllSteps(t *testing.T) {
	repoRoot := t.TempDir()
	marker := filepath.Join(repoRoot, "ran-second.txt")
	r := New(&config.Config{Tasks: map[string]config.Task{
		"fail":   {Desc: "fail", Cmd: "exit 1"},
		"second": {Desc: "second", Cmd: "echo ran > ran-second.txt"},
		"pipe":   {Desc: "pipe", Steps: []string{"fail", "second"}, ContinueOnError: true},
	}}, repoRoot)

	result, err := r.Run("pipe", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", result.Status)
	}
	if result.Steps[1].Status != StatusPass {
		t.Fatalf("second step status = %q, want pass", result.Steps[1].Status)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("second step should have run; stat err = %v", err)
	}
}

func TestRunMapsTimeoutToTimeoutStatus(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"slow": {Desc: "slow", Cmd: "sleep 1", Timeout: "10ms"},
	})

	result, err := r.Run("slow", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusTimeout {
		t.Fatalf("Status = %q, want timeout", result.Status)
	}
	if result.ExitCode != 124 {
		t.Fatalf("ExitCode = %d, want 124", result.ExitCode)
	}
}

func TestParallelPipelineCancelsOtherStepOnFailure(t *testing.T) {
	repoRoot := t.TempDir()
	r := New(&config.Config{Tasks: map[string]config.Task{
		"fail": {Desc: "fail", Cmd: "exit 1"},
		"slow": {Desc: "slow", Cmd: "sleep 1; echo done > slow.txt"},
		"par":  {Desc: "par", Steps: []string{"fail", "slow"}, Parallel: true},
	}}, repoRoot)

	result, err := r.Run("par", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", result.Status)
	}
	if result.Steps[1].Status != StatusCancelled && result.Steps[1].Status != StatusFail {
		t.Fatalf("slow step status = %q, want cancelled or fail", result.Steps[1].Status)
	}
}

func TestRunCmdTaskInjectsParamsIntoEnvAndTemplate(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"add-feature": {
			Desc: "add",
			Cmd:  `printf "%s|%s" "{{params.feature}}" "$FEATURE"`,
			Params: map[string]config.Param{
				"feature": {
					Env:      "FEATURE",
					Required: true,
				},
			},
		},
	})

	result, err := r.Run("add-feature", Options{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Params: map[string]string{"feature": "auth"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "auth|auth" {
		t.Fatalf("Stdout = %q, want interpolated/env param", result.Stdout)
	}
}

func TestRunCmdTaskRejectsMissingRequiredParam(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"add-feature": {
			Desc: "add",
			Cmd:  "echo hi",
			Params: map[string]config.Param{
				"feature": {
					Env:      "FEATURE",
					Required: true,
				},
			},
		},
	})

	_, err := r.Run("add-feature", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err == nil {
		t.Fatal("Run() error = nil, want missing param error")
	}
	if err.Error() != `task "add-feature": missing required param "feature"` {
		t.Fatalf("Run() error = %v, want missing param message", err)
	}
}

func TestRunCmdTaskUsesCustomShell(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"echo": {
			Desc:      "echo",
			Cmd:       "printf shell-ok",
			Shell:     "/bin/sh",
			ShellArgs: []string{"-c"},
		},
	})

	result, err := r.Run("echo", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "shell-ok" {
		t.Fatalf("Stdout = %q, want custom shell output", result.Stdout)
	}
}

func TestRunCmdTaskUsesCustomShellArgs(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"strict": {
			Desc:      "strict",
			Cmd:       "printf strict-ok",
			Shell:     "/bin/sh",
			ShellArgs: []string{"-eu", "-c"},
		},
	})

	result, err := r.Run("strict", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stdout != "strict-ok" {
		t.Fatalf("Stdout = %q, want custom shell arg output", result.Stdout)
	}
}

func TestRunCmdTaskUsesDefaultWorkingDir(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	workdir := filepath.Join(repoRoot, "app")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	r := New(&config.Config{
		Defaults: config.DefaultsConfig{Dir: "app"},
		Tasks: map[string]config.Task{
			"pwd": {Desc: "pwd", Cmd: "pwd"},
		},
	}, repoRoot)

	result, err := r.Run("pwd", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := strings.TrimSpace(result.Stdout)
	if resolved, err := filepath.EvalSymlinks(workdir); err == nil {
		workdir = resolved
	}
	if got != workdir {
		t.Fatalf("Stdout = %q, want default workdir %q", got, workdir)
	}
}

func TestRunCmdTaskDirOverridesDefaultWorkingDir(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	defaultDir := filepath.Join(repoRoot, "app")
	overrideDir := filepath.Join(repoRoot, "tools")
	for _, dir := range []string{defaultDir, overrideDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := New(&config.Config{
		Defaults: config.DefaultsConfig{Dir: "app"},
		Tasks: map[string]config.Task{
			"pwd": {Desc: "pwd", Cmd: "pwd", Dir: "tools"},
		},
	}, repoRoot)

	result, err := r.Run("pwd", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got := strings.TrimSpace(result.Stdout)
	if resolved, err := filepath.EvalSymlinks(overrideDir); err == nil {
		overrideDir = resolved
	}
	if got != overrideDir {
		t.Fatalf("Stdout = %q, want override workdir %q", got, overrideDir)
	}
}

func TestPrefixedWriterPrefixesFirstLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	writer := prefixedWriter("step-1", &buf)
	if _, err := writer.Write([]byte("hello\nworld\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if got := buf.String(); got != "[step-1] hello\n[step-1] world\n" {
		t.Fatalf("output = %q, want prefixed lines", got)
	}
}

func TestRunPipelineReportsNestedPipelineErrorWithParentName(t *testing.T) {
	t.Parallel()

	r := newTestRunner(t, map[string]config.Task{
		"inner": {Desc: "inner", Steps: []string{"leaf"}},
		"leaf":  {Desc: "leaf", Cmd: "echo leaf"},
		"outer": {Desc: "outer", Steps: []string{"inner"}},
	})

	_, err := r.Run("outer", Options{Stdout: io.Discard, Stderr: io.Discard})
	if err == nil {
		t.Fatal("Run() error = nil, want nested pipeline error")
	}
	if got := err.Error(); got != `task "outer" references pipeline task "inner", but nested pipeline steps are not implemented yet` {
		t.Fatalf("Run() error = %q, want parent-aware nested pipeline error", got)
	}
}

func newTestRunner(t *testing.T, tasks map[string]config.Task) *Runner {
	t.Helper()
	return New(&config.Config{Tasks: tasks}, t.TempDir())
}

func mustTempFile(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "runner-out-*")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
