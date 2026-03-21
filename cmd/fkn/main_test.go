package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fkn/internal/config"
)

func TestUnknownTaskErrorSuggestsNearestTask(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"check": {Desc: "Check", Cmd: "echo check"},
			"build": {Desc: "Build", Cmd: "echo build"},
		},
	}

	err := unknownTaskError("chek", cfg)
	if err == nil {
		t.Fatal("unknownTaskError() = nil, want error")
	}
	if !strings.Contains(err.Error(), "Did you mean: check?") {
		t.Fatalf("unknownTaskError() = %v, want suggestion", err)
	}
}

func TestRunHelpForTask(t *testing.T) {
	repoRoot := repoRootForTest(t)
	restore := chdirForTest(t, repoRoot)
	defer restore()

	stdout, readStdout := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"help", "check"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("run(help check) code = %d, want 0; stderr=%s", code, readStderr())
	}

	output := readStdout()
	if !strings.Contains(output, "Description: Run the default local verification pipeline") {
		t.Fatalf("stdout = %q, want task description", output)
	}
	if !strings.Contains(output, "Steps:\n- fmt\n- test\n- build") {
		t.Fatalf("stdout = %q, want steps", output)
	}
}

func TestRunUnknownTaskShowsSuggestion(t *testing.T) {
	repoRoot := repoRootForTest(t)
	restore := chdirForTest(t, repoRoot)
	defer restore()

	stdout, _ := tempOutputFile(t)
	stderr, readStderr := tempOutputFile(t)

	code := run([]string{"chek"}, stdout, stderr)
	if code == 0 {
		t.Fatal("run(chek) code = 0, want failure")
	}
	if !strings.Contains(readStderr(), `unknown task "chek". Did you mean: check?`) {
		t.Fatalf("stderr = %q, want suggestion", readStderr())
	}
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func chdirForTest(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatal(err)
		}
	}
}

func tempOutputFile(t *testing.T) (*os.File, func() string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "fkn-out-*")
	if err != nil {
		t.Fatal(err)
	}
	return f, func() string {
		if _, err := f.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
}
