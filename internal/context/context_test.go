package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
)

func TestGenerateAgentRequiresTask(t *testing.T) {
	t.Parallel()

	g := New(&config.Config{}, t.TempDir())
	_, err := g.Generate(Options{Agent: true})
	if err == nil {
		t.Fatal("Generate() error = nil, want missing task error")
	}
	if !strings.Contains(err.Error(), "--task") {
		t.Fatalf("Generate() error = %v, want task guidance", err)
	}
}

func TestGenerateIncludesTaskScopeAndLastGuard(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".fkn"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".fkn", "last-guard.json"), []byte(`{"guard":"default"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Project:     "demo",
		Description: "Demo repo",
		Tasks: map[string]config.Task{
			"check": {Desc: "Run checks", Steps: []string{"test"}, Scope: "cli"},
			"test":  {Desc: "Run tests", Cmd: "echo test"},
		},
		Scopes: map[string][]string{
			"cli": {"README.md"},
		},
		Context: config.ContextConfig{
			AgentFiles: []string{"README.md"},
			Caps: config.ContextCaps{
				FileTreeEntries: 20,
				AgentFileLines:  10,
				GitDiffLines:    10,
				DependencyLines: 10,
				GitLogLines:     10,
				FileLines:       10,
			},
		},
	}

	out, err := New(cfg, dir).Generate(Options{Agent: true, Task: "check"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !strings.Contains(out, "## Agent Task") {
		t.Fatalf("output missing Agent Task section:\n%s", out)
	}
	if !strings.Contains(out, "Scope `cli`: README.md") {
		t.Fatalf("output missing scope detail:\n%s", out)
	}
	if !strings.Contains(out, "## Last Guard") {
		t.Fatalf("output missing last guard:\n%s", out)
	}
}

func TestTruncateLinesAddsOmissionMarker(t *testing.T) {
	t.Parallel()

	got := truncateLines([]string{"1", "2", "3", "4"}, 2)
	want := "1\n2\n[Lines 3–4 omitted]"
	if got != want {
		t.Fatalf("truncateLines() = %q, want %q", got, want)
	}
}

func TestTruncateToTokenBudgetUsesApproximateTokenCount(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("a", 40)
	got := truncateToTokenBudget(text, 5)
	if !strings.Contains(got, "[output truncated by max token budget]") {
		t.Fatalf("truncateToTokenBudget() = %q, want truncation marker", got)
	}
	if got == text {
		t.Fatalf("truncateToTokenBudget() = %q, want truncated output", got)
	}
}

func TestApproximateTokenCount(t *testing.T) {
	t.Parallel()

	if got := approximateTokenCount(""); got != 0 {
		t.Fatalf("approximateTokenCount(\"\") = %d, want 0", got)
	}
	if got := approximateTokenCount("abcd"); got != 1 {
		t.Fatalf("approximateTokenCount(\"abcd\") = %d, want 1", got)
	}
	if got := approximateTokenCount("abcde"); got != 2 {
		t.Fatalf("approximateTokenCount(\"abcde\") = %d, want 2", got)
	}
}
