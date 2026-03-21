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

	"github.com/neural-chilli/fkn/internal/config"
)

type Generator struct {
	cfg      *config.Config
	repoRoot string
}

type Options struct {
	Agent     bool
	Task      string
	MaxTokens int
}

type section struct {
	title string
	body  string
}

func New(cfg *config.Config, repoRoot string) *Generator {
	return &Generator{cfg: cfg, repoRoot: repoRoot}
}

func (g *Generator) Generate(opts Options) (string, error) {
	sections := []section{
		{title: "Project", body: g.projectSection()},
	}

	if opts.Agent {
		agentBody, err := g.agentSection(opts.Task)
		if err != nil {
			return "", err
		}
		sections = append(sections, section{title: "Agent Task", body: agentBody})
	}

	if g.contextFileTreeEnabled() {
		if body := g.fileTreeSection(); body != "" {
			sections = append(sections, section{title: "File Tree", body: body})
		}
	}

	if body := g.taskSection(); body != "" {
		sections = append(sections, section{title: "Tasks", body: body})
	}
	if body := g.guardSection(); body != "" {
		sections = append(sections, section{title: "Guards", body: body})
	}
	if body := g.dependenciesSection(); body != "" {
		sections = append(sections, section{title: "Dependencies", body: body})
	}
	if body := g.gitLogSection(); body != "" {
		sections = append(sections, section{title: "Recent Git Log", body: body})
	}
	if body := g.filesSection(g.cfg.Context.Files, g.cfg.Context.Caps.FileLines, g.cfg.Context.Caps.FilesMax); body != "" {
		sections = append(sections, section{title: "Configured Files", body: body})
	}
	if body := g.filesSection(g.cfg.Context.AgentFiles, g.cfg.Context.Caps.AgentFileLines, len(g.cfg.Context.AgentFiles)); body != "" {
		sections = append(sections, section{title: "Agent Files", body: body})
	}
	if g.cfg.Context.Todos {
		if body := g.todosSection(); body != "" {
			sections = append(sections, section{title: "TODOs", body: body})
		}
	}
	if opts.Agent || g.cfg.Context.GitDiff {
		if body := g.gitDiffSection(); body != "" {
			sections = append(sections, section{title: "Git Diff", body: body})
		}
	}
	if opts.Agent {
		if body := g.lastGuardSection(); body != "" {
			sections = append(sections, section{title: "Last Guard", body: body})
		}
	}

	var out []string
	out = append(out, "# fkn context")
	for _, sec := range sections {
		if sec.body == "" {
			continue
		}
		out = append(out, fmt.Sprintf("\n## %s\n\n%s", sec.title, sec.body))
	}

	rendered := strings.Join(out, "\n")
	if opts.MaxTokens > 0 {
		rendered = truncateToTokenBudget(rendered, opts.MaxTokens)
	}
	return rendered, nil
}

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
		lines = append(lines, fmt.Sprintf("- Scope `%s`: %s", task.Scope, strings.Join(g.cfg.Scopes[task.Scope], ", ")))
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
	words := strings.Fields(text)
	if len(words) <= maxTokens {
		return text
	}
	return strings.Join(append(words[:maxTokens], "[output truncated by max token budget]"), " ")
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
