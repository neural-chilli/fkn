package context

import (
	"sort"
	"strings"

	"github.com/neural-chilli/fkn/internal/codemap"
)

func (g *Generator) codemapSection(taskName string) string {
	if taskName == "" {
		return ""
	}
	task, ok := g.cfg.Tasks[taskName]
	if !ok || task.Scope == "" {
		return ""
	}
	return codemap.RenderRelevantPackages(codemap.RelevantPackages(g.cfg, g.cfg.Scopes[task.Scope].Paths))
}

func (g *Generator) aboutTasksSection(topic string) string {
	query := strings.ToLower(topic)
	var lines []string
	for _, name := range sortedKeys(g.cfg.Tasks) {
		task := g.cfg.Tasks[name]
		if !matchesTopic(query, name, task.Desc, task.Scope, task.Cmd, strings.Join(task.Steps, " ")) {
			continue
		}
		line := "- `" + name + "`: " + task.Desc
		if task.Scope != "" {
			line += " (scope: `" + task.Scope + "`)"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (g *Generator) aboutScopesSection(topic string) string {
	query := strings.ToLower(topic)
	var lines []string
	for _, name := range sortedKeys(g.cfg.Scopes) {
		scopeDef := g.cfg.Scopes[name]
		if !matchesTopic(query, name, scopeDef.Desc, strings.Join(scopeDef.Paths, " ")) {
			continue
		}
		line := "- `" + name + "`: " + strings.Join(scopeDef.Paths, ", ")
		if scopeDef.Desc != "" {
			line += " (" + scopeDef.Desc + ")"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (g *Generator) aboutCodemapSection(topic string) string {
	query := strings.ToLower(topic)
	var matches []codemap.PackageExplanation
	for _, name := range sortedKeys(g.cfg.Codemap.Packages) {
		entry := g.cfg.Codemap.Packages[name]
		if !matchesTopic(query, name, entry.Desc, strings.Join(entry.KeyTypes, " "), strings.Join(entry.EntryPoints, " "), strings.Join(entry.Conventions, " "), strings.Join(entry.DependsOn, " ")) {
			continue
		}
		matches = append(matches, codemap.PackageExplanation{
			Name:        name,
			Description: entry.Desc,
			KeyTypes:    append([]string(nil), entry.KeyTypes...),
			EntryPoints: append([]string(nil), entry.EntryPoints...),
			Conventions: append([]string(nil), entry.Conventions...),
			DependsOn:   append([]string(nil), entry.DependsOn...),
		})
	}
	return codemap.RenderRelevantPackages(matches)
}

func (g *Generator) aboutGlossarySection(topic string) string {
	query := strings.ToLower(topic)
	var lines []string
	for _, term := range sortedKeys(g.cfg.Codemap.Glossary) {
		definition := g.cfg.Codemap.Glossary[term]
		if !matchesTopic(query, term, definition) {
			continue
		}
		lines = append(lines, "- `"+term+"`: "+definition)
	}
	return strings.Join(lines, "\n")
}

func (g *Generator) aboutRelevantPathsSection(topic string) string {
	query := strings.ToLower(topic)
	seen := map[string]bool{}
	var paths []string

	for _, name := range sortedKeys(g.cfg.Tasks) {
		task := g.cfg.Tasks[name]
		if task.Scope == "" || !matchesTopic(query, name, task.Desc, task.Scope) {
			continue
		}
		for _, path := range g.cfg.Scopes[task.Scope].Paths {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	for _, name := range sortedKeys(g.cfg.Scopes) {
		scopeDef := g.cfg.Scopes[name]
		if !matchesTopic(query, name, scopeDef.Desc, strings.Join(scopeDef.Paths, " ")) {
			continue
		}
		for _, path := range scopeDef.Paths {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	for _, name := range sortedKeys(g.cfg.Codemap.Packages) {
		entry := g.cfg.Codemap.Packages[name]
		if !matchesTopic(query, name, entry.Desc, strings.Join(entry.KeyTypes, " "), strings.Join(entry.EntryPoints, " ")) {
			continue
		}
		if !seen[name] {
			seen[name] = true
			paths = append(paths, name)
		}
	}

	sort.Strings(paths)
	if len(paths) == 0 {
		return ""
	}
	var lines []string
	for _, path := range paths {
		lines = append(lines, "- `"+path+"`")
	}
	return strings.Join(lines, "\n")
}

func matchesTopic(query string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(strings.ToLower(part), query) {
			return true
		}
	}
	return false
}
