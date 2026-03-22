package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCompleteSuggestsTopLevelCommandsAndTasks(t *testing.T) {
	repoRoot := repoRootForTest(t)
	restore := chdirForTest(t, repoRoot)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"__complete", "he"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(__complete he) code = %d, want 0; stderr=%s", code, readStderr())
	}

	output := readStdout()
	if !strings.Contains(output, "help") {
		t.Fatalf("stdout = %q, want help completion", output)
	}
}

func TestRunCompleteSuggestsTaskParamFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(`
tasks:
  add-feature:
    desc: Add a feature
    cmd: make add-feature
    params:
      feature:
        env: FEATURE
        required: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	restore := chdirForTest(t, dir)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"__complete", "add-feature", "--f"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(__complete add-feature --f) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, "--feature") {
		t.Fatalf("stdout = %q, want task param flag completion", output)
	}
}

func TestRunCompletionDocsListIncludesShells(t *testing.T) {
	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"completion", "bash"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(completion bash) code = %d, want 0; stderr=%s", code, readStderr())
	}
	output := readStdout()
	if !strings.Contains(output, "complete -F _fkn_completion fkn") {
		t.Fatalf("stdout = %q, want bash completion script", output)
	}
}

func TestCompletionInstallBashWritesFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	installer, err := newCompletionInstaller("bash")
	if err != nil {
		t.Fatalf("newCompletionInstaller() error = %v", err)
	}
	result, err := installer.Install()
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !strings.Contains(result.Message, "Installed bash completion") {
		t.Fatalf("result.Message = %q, want install message", result.Message)
	}

	completionPath := filepath.Join(home, ".local", "share", "bash-completion", "completions", "fkn")
	rcPath := filepath.Join(home, ".bashrc")
	for _, path := range []string{completionPath, rcPath} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		if len(raw) == 0 {
			t.Fatalf("%s is empty", path)
		}
	}
}
