package context

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func (g *Generator) projectSection() string {
	lines := []string{}
	if g.cfg.Project != "" {
		lines = append(lines, fmt.Sprintf("- Project: %s", g.cfg.Project))
	}
	if g.cfg.Description != "" {
		lines = append(lines, fmt.Sprintf("- Description: %s", g.cfg.Description))
	}
	lines = append(lines, fmt.Sprintf("- Repo root: `%s`", g.repoRoot))
	return strings.Join(lines, "\n")
}

func (g *Generator) agentSection(taskName string) (string, error) {
	if taskName == "" {
		return "", fmt.Errorf("agent mode requires --task <name>")
	}
	task, ok := g.cfg.Tasks[taskName]
	if !ok {
		return "", fmt.Errorf("unknown task %q", taskName)
	}
	lines := []string{
		fmt.Sprintf("- Task: `%s`", taskName),
		fmt.Sprintf("- Description: %s", task.Desc),
	}
	if task.Scope != "" {
		scopeDef := g.cfg.Scopes[task.Scope]
		lines = append(lines, fmt.Sprintf("- Scope `%s`: %s", task.Scope, strings.Join(scopeDef.Paths, ", ")))
		if scopeDef.Desc != "" {
			lines = append(lines, fmt.Sprintf("- Scope intent: %s", scopeDef.Desc))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (g *Generator) taskSection() string {
	names := sortedKeys(g.cfg.Tasks)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		task := g.cfg.Tasks[name]
		line := fmt.Sprintf("- `%s`: %s", name, task.Desc)
		if task.Scope != "" {
			line += fmt.Sprintf(" (scope: `%s`)", task.Scope)
		}
		if len(task.Needs) > 0 {
			line += fmt.Sprintf(" (needs: `%s`)", strings.Join(task.Needs, "`, `"))
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (g *Generator) guardSection() string {
	if len(g.cfg.Guards) == 0 {
		return ""
	}
	names := sortedKeys(g.cfg.Guards)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, fmt.Sprintf("- `%s`: %s", name, strings.Join(g.cfg.Guards[name].Steps, ", ")))
	}
	return strings.Join(lines, "\n")
}

func (g *Generator) dependenciesSection() string {
	if g.cfg.Context.Dependencies != nil && !*g.cfg.Context.Dependencies {
		return ""
	}
	var parts []string
	for _, depFile := range []string{"go.mod", "package.json", "requirements.txt", "pyproject.toml"} {
		path := filepath.Join(g.repoRoot, depFile)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		parts = append(parts, fmt.Sprintf("### `%s`\n\n```text\n%s\n```", depFile, truncateLines(lines, g.cfg.Context.Caps.DependencyLines)))
	}
	return strings.Join(parts, "\n\n")
}

func (g *Generator) gitLogSection() string {
	lines := g.gitLines("log", "--oneline", fmt.Sprintf("-%d", min(g.cfg.Context.GitLogLines, g.cfg.Context.Caps.GitLogLines)))
	if len(lines) == 0 {
		return ""
	}
	return "```text\n" + strings.Join(lines, "\n") + "\n```"
}

func (g *Generator) gitDiffSection() string {
	lines := g.gitLines("diff", "--")
	if len(lines) == 0 {
		return ""
	}
	return "```diff\n" + truncateLines(lines, g.cfg.Context.Caps.GitDiffLines) + "\n```"
}

func (g *Generator) filesSection(paths []string, lineCap, fileCap int) string {
	if len(paths) == 0 || fileCap == 0 {
		return ""
	}
	parts := []string{}
	for i, path := range paths {
		if i >= fileCap {
			break
		}
		raw, err := os.ReadFile(filepath.Join(g.repoRoot, path))
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
		parts = append(parts, fmt.Sprintf("### `%s`\n\n```text\n%s\n```", path, truncateLines(lines, lineCap)))
	}
	return strings.Join(parts, "\n\n")
}

func (g *Generator) todosSection() string {
	var matches []string
	_ = filepath.WalkDir(g.repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(g.repoRoot, path)
		if relErr != nil || rel == "." || g.isExcluded(rel) {
			if d.IsDir() && rel != "." && g.isExcluded(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, "TODO") || strings.Contains(line, "FIXME") {
				matches = append(matches, fmt.Sprintf("- `%s:%d` %s", rel, i+1, strings.TrimSpace(line)))
				if len(matches) >= g.cfg.Context.Caps.TodosMax {
					return fmt.Errorf("stop")
				}
			}
		}
		return nil
	})
	return strings.Join(matches, "\n")
}

func (g *Generator) fileTreeSection() string {
	entries := []string{}
	_ = filepath.WalkDir(g.repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, relErr := filepath.Rel(g.repoRoot, path)
		if relErr != nil || rel == "." {
			return nil
		}
		if g.isExcluded(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if len(g.cfg.Context.Include) > 0 && !g.isIncluded(rel) {
			if d.IsDir() && !g.mightContainIncluded(rel) {
				return filepath.SkipDir
			}
			if !d.IsDir() {
				return nil
			}
		}
		suffix := ""
		if d.IsDir() {
			suffix = "/"
		}
		entries = append(entries, rel+suffix)
		if len(entries) >= g.cfg.Context.Caps.FileTreeEntries {
			return fmt.Errorf("stop")
		}
		return nil
	})
	sort.Strings(entries)
	if len(entries) == 0 {
		return ""
	}
	return "```text\n" + strings.Join(entries, "\n") + "\n```"
}

func (g *Generator) lastGuardSection() string {
	path := filepath.Join(g.repoRoot, ".fkn", "last-guard.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return ""
	}
	return "```json\n" + string(raw) + "\n```"
}

func (g *Generator) gitLines(args ...string) []string {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.repoRoot
	raw, err := cmd.Output()
	if err != nil {
		return nil
	}
	text := strings.TrimRight(string(raw), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func (g *Generator) isExcluded(rel string) bool {
	if strings.HasPrefix(rel, ".git/") || strings.HasPrefix(rel, ".idea/") || strings.HasPrefix(rel, "bin/") || strings.HasPrefix(rel, ".fkn/") {
		return true
	}
	for _, pattern := range g.cfg.Context.Exclude {
		if matchPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func (g *Generator) isIncluded(rel string) bool {
	for _, pattern := range g.cfg.Context.Include {
		trimmed := strings.TrimSuffix(pattern, "/")
		if rel == pattern || rel == trimmed || strings.HasPrefix(rel, trimmed+"/") || matchPattern(pattern, rel) {
			return true
		}
	}
	return false
}

func (g *Generator) mightContainIncluded(rel string) bool {
	for _, pattern := range g.cfg.Context.Include {
		pattern = strings.TrimSuffix(pattern, "/")
		if strings.HasPrefix(pattern, rel+"/") {
			return true
		}
	}
	return false
}

func (g *Generator) contextFileTreeEnabled() bool {
	return g.cfg.Context.FileTree == nil || *g.cfg.Context.FileTree
}

func matchPattern(pattern, rel string) bool {
	pattern = strings.TrimPrefix(pattern, "./")
	rel = filepath.ToSlash(rel)
	pattern = filepath.ToSlash(pattern)
	pattern = strings.TrimPrefix(pattern, "**/")
	if ok, _ := filepath.Match(pattern, filepath.Base(rel)); ok {
		return true
	}
	if ok, _ := filepath.Match(pattern, rel); ok {
		return true
	}
	return false
}

func truncateLines(lines []string, cap int) string {
	if cap <= 0 || len(lines) <= cap {
		return strings.Join(lines, "\n")
	}
	marker := fmt.Sprintf("[Lines %d–%d omitted]", cap+1, len(lines))
	return strings.Join(append(lines[:cap], marker), "\n")
}

func truncateToTokenBudget(text string, maxTokens int) string {
	if approximateTokenCount(text) <= maxTokens {
		return text
	}
	maxChars := maxTokens * 4
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	marker := "\n[output truncated by max token budget]"
	available := maxChars - len([]rune(marker))
	if available <= 0 {
		return marker
	}
	return string(runes[:available]) + marker
}

func approximateTokenCount(text string) int {
	runes := len([]rune(text))
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}

func sortedKeys[T any](items map[string]T) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
