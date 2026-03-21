package plan

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neural-chilli/fkn/internal/codemap"
	"github.com/neural-chilli/fkn/internal/config"
)

type Output struct {
	Files    []string              `json:"files"`
	Scopes   []ScopeMatch          `json:"scopes,omitempty"`
	Tasks    []TaskMatch           `json:"tasks,omitempty"`
	Groups   []GroupMatch          `json:"groups,omitempty"`
	Guards   []GuardMatch          `json:"guards,omitempty"`
	Packages []codemap.Explanation `json:"packages,omitempty"`
	Markdown string                `json:"markdown"`
}

type ScopeMatch struct {
	Name  string   `json:"name"`
	Desc  string   `json:"desc,omitempty"`
	Paths []string `json:"paths"`
}

type TaskMatch struct {
	Name   string   `json:"name"`
	Desc   string   `json:"desc"`
	Type   string   `json:"type"`
	Scope  string   `json:"scope,omitempty"`
	Safety string   `json:"safety"`
	Needs  []string `json:"needs,omitempty"`
	Steps  []string `json:"steps,omitempty"`
	Groups []string `json:"groups,omitempty"`
}

type GroupMatch struct {
	Name  string   `json:"name"`
	Desc  string   `json:"desc,omitempty"`
	Tasks []string `json:"tasks"`
}

type GuardMatch struct {
	Name  string   `json:"name"`
	Steps []string `json:"steps"`
}

func Generate(cfg *config.Config, repoRoot string, files []string) (Output, error) {
	normalized, err := normalizeFiles(repoRoot, files)
	if err != nil {
		return Output{}, err
	}
	if len(normalized) == 0 {
		return Output{}, fmt.Errorf("at least one file path is required")
	}

	scopeNames := matchedScopes(cfg, normalized)
	taskNames := matchedTasks(cfg, scopeNames)
	groupNames := matchedGroups(cfg, taskNames)
	guardNames := matchedGuards(cfg, taskNames)
	packages := matchedPackages(cfg, normalized, scopeNames)

	out := Output{
		Files:    normalized,
		Scopes:   buildScopeMatches(cfg, scopeNames),
		Tasks:    buildTaskMatches(cfg, taskNames),
		Groups:   buildGroupMatches(cfg, groupNames),
		Guards:   buildGuardMatches(cfg, guardNames),
		Packages: packages,
	}
	out.Markdown = render(out)
	return out, nil
}

func normalizeFiles(repoRoot string, files []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(files))
	for _, raw := range files {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		path := raw
		if filepath.IsAbs(path) {
			rel, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return nil, fmt.Errorf("normalize %q: %w", raw, err)
			}
			path = rel
		}
		path = filepath.ToSlash(filepath.Clean(path))
		path = strings.TrimPrefix(path, "./")
		if path == "." || path == "" {
			continue
		}
		if !seen[path] {
			seen[path] = true
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out, nil
}

func matchedScopes(cfg *config.Config, files []string) []string {
	var matches []string
	for name, scopeDef := range cfg.Scopes {
		for _, file := range files {
			if anyPathMatches(file, scopeDef.Paths) {
				matches = append(matches, name)
				break
			}
		}
	}
	sort.Strings(matches)
	return matches
}

func matchedTasks(cfg *config.Config, scopeNames []string) []string {
	scopeSet := makeSet(scopeNames)
	matched := map[string]bool{}
	queue := []string{}

	for name, task := range cfg.Tasks {
		if task.Scope != "" && scopeSet[task.Scope] {
			matched[name] = true
			queue = append(queue, name)
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for name, task := range cfg.Tasks {
			if matched[name] {
				continue
			}
			if contains(task.Needs, current) || contains(task.Steps, current) {
				matched[name] = true
				queue = append(queue, name)
			}
		}
	}

	names := mapsKeys(matched)
	sort.Strings(names)
	return names
}

func matchedGroups(cfg *config.Config, taskNames []string) []string {
	taskSet := makeSet(taskNames)
	var names []string
	for name, group := range cfg.Groups {
		for _, taskName := range group.Tasks {
			if taskSet[taskName] {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

func matchedGuards(cfg *config.Config, taskNames []string) []string {
	taskSet := makeSet(taskNames)
	var names []string
	for name, guard := range cfg.Guards {
		for _, step := range guard.Steps {
			if taskSet[step] {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

func matchedPackages(cfg *config.Config, files, scopeNames []string) []codemap.Explanation {
	seen := map[string]bool{}
	var matches []codemap.Explanation

	for name, entry := range cfg.Codemap.Packages {
		for _, file := range files {
			if pathMatches(file, name) {
				matches = append(matches, codemap.Explanation{
					Target:      name,
					Kind:        "package",
					Description: entry.Desc,
					Package: &codemap.PackageExplanation{
						Name:        name,
						Description: entry.Desc,
						KeyTypes:    append([]string(nil), entry.KeyTypes...),
						EntryPoints: append([]string(nil), entry.EntryPoints...),
						Conventions: append([]string(nil), entry.Conventions...),
						DependsOn:   append([]string(nil), entry.DependsOn...),
					},
				})
				seen[name] = true
				break
			}
		}
	}

	for _, scopeName := range scopeNames {
		scopeDef := cfg.Scopes[scopeName]
		for _, pkg := range codemap.RelevantPackages(cfg, scopeDef.Paths) {
			if seen[pkg.Name] {
				continue
			}
			matches = append(matches, codemap.Explanation{
				Target:      pkg.Name,
				Kind:        "package",
				Description: pkg.Description,
				Package:     &pkg,
			})
			seen[pkg.Name] = true
		}
	}

	sort.Slice(matches, func(i, j int) bool { return matches[i].Target < matches[j].Target })
	return matches
}

func buildScopeMatches(cfg *config.Config, names []string) []ScopeMatch {
	out := make([]ScopeMatch, 0, len(names))
	for _, name := range names {
		scopeDef := cfg.Scopes[name]
		out = append(out, ScopeMatch{
			Name:  name,
			Desc:  scopeDef.Desc,
			Paths: append([]string(nil), scopeDef.Paths...),
		})
	}
	return out
}

func buildTaskMatches(cfg *config.Config, names []string) []TaskMatch {
	out := make([]TaskMatch, 0, len(names))
	for _, name := range names {
		task := cfg.Tasks[name]
		out = append(out, TaskMatch{
			Name:   name,
			Desc:   task.Desc,
			Type:   task.Type(),
			Scope:  task.Scope,
			Safety: task.SafetyLevel(),
			Needs:  append([]string(nil), task.Needs...),
			Steps:  append([]string(nil), task.Steps...),
			Groups: groupNamesForTask(cfg.Groups, name),
		})
	}
	return out
}

func buildGroupMatches(cfg *config.Config, names []string) []GroupMatch {
	out := make([]GroupMatch, 0, len(names))
	for _, name := range names {
		group := cfg.Groups[name]
		out = append(out, GroupMatch{
			Name:  name,
			Desc:  group.Desc,
			Tasks: append([]string(nil), group.Tasks...),
		})
	}
	return out
}

func buildGuardMatches(cfg *config.Config, names []string) []GuardMatch {
	out := make([]GuardMatch, 0, len(names))
	for _, name := range names {
		guard := cfg.Guards[name]
		out = append(out, GuardMatch{
			Name:  name,
			Steps: append([]string(nil), guard.Steps...),
		})
	}
	return out
}

func render(out Output) string {
	var lines []string
	lines = append(lines, "# fkn plan", "")
	lines = append(lines, "## Files", "")
	for _, file := range out.Files {
		lines = append(lines, "- `"+file+"`")
	}
	if len(out.Scopes) > 0 {
		lines = append(lines, "", "## Matching Scopes", "")
		for _, scope := range out.Scopes {
			line := "- `" + scope.Name + "`: " + strings.Join(scope.Paths, ", ")
			if scope.Desc != "" {
				line += " (" + scope.Desc + ")"
			}
			lines = append(lines, line)
		}
	}
	if len(out.Tasks) > 0 {
		lines = append(lines, "", "## Relevant Tasks", "")
		for _, task := range out.Tasks {
			line := "- `" + task.Name + "`: " + task.Desc + " [" + task.Type + ", safety:" + task.Safety + "]"
			if task.Scope != "" {
				line += " (scope: `" + task.Scope + "`)"
			}
			lines = append(lines, line)
		}
	}
	if len(out.Guards) > 0 {
		lines = append(lines, "", "## Relevant Guards", "")
		for _, guard := range out.Guards {
			lines = append(lines, "- `"+guard.Name+"`: `"+strings.Join(guard.Steps, "`, `")+"`")
		}
	}
	if len(out.Groups) > 0 {
		lines = append(lines, "", "## Relevant Groups", "")
		for _, group := range out.Groups {
			line := "- `" + group.Name + "`: `" + strings.Join(group.Tasks, "`, `") + "`"
			if group.Desc != "" {
				line += " (" + group.Desc + ")"
			}
			lines = append(lines, line)
		}
	}
	if len(out.Packages) > 0 {
		lines = append(lines, "", "## Relevant Codemap", "")
		for _, pkg := range out.Packages {
			lines = append(lines, "- `"+pkg.Target+"`: "+pkg.Description)
		}
	}
	if len(out.Tasks) > 0 || len(out.Guards) > 0 {
		lines = append(lines, "", "## Suggested Next Commands", "")
		if len(out.Tasks) > 0 {
			lines = append(lines, "- `fkn help "+out.Tasks[0].Name+"`")
			lines = append(lines, "- `fkn context --agent --task "+out.Tasks[0].Name+"`")
		}
		if len(out.Guards) > 0 {
			lines = append(lines, "- `fkn guard "+out.Guards[0].Name+"`")
		}
	}
	return strings.Join(lines, "\n")
}

func pathMatches(file, raw string) bool {
	path := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(raw)), "/")
	file = filepath.ToSlash(filepath.Clean(file))
	return file == path || strings.HasPrefix(file, path+"/")
}

func anyPathMatches(file string, paths []string) bool {
	for _, path := range paths {
		if pathMatches(file, path) {
			return true
		}
	}
	return false
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func makeSet(items []string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, item := range items {
		out[item] = true
	}
	return out
}

func mapsKeys(items map[string]bool) []string {
	out := make([]string, 0, len(items))
	for item := range items {
		out = append(out, item)
	}
	return out
}

func groupNamesForTask(groups map[string]config.Group, taskName string) []string {
	var names []string
	for name, group := range groups {
		for _, member := range group.Tasks {
			if member == taskName {
				names = append(names, name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}
