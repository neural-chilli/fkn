package codemap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
)

type Explanation struct {
	Target      string               `json:"target"`
	Kind        string               `json:"kind"`
	Description string               `json:"description,omitempty"`
	Task        *TaskExplanation     `json:"task,omitempty"`
	Package     *PackageExplanation  `json:"package,omitempty"`
	Glossary    *GlossaryExplanation `json:"glossary,omitempty"`
	Markdown    string               `json:"markdown"`
}

type TaskExplanation struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Scope       string   `json:"scope,omitempty"`
	ScopePaths  []string `json:"scope_paths,omitempty"`
	Steps       []string `json:"steps,omitempty"`
	Command     string   `json:"command,omitempty"`
}

type PackageExplanation struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	KeyTypes    []string `json:"key_types,omitempty"`
	EntryPoints []string `json:"entry_points,omitempty"`
	Conventions []string `json:"conventions,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
}

type GlossaryExplanation struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

func Explain(cfg *config.Config, target string) (Explanation, error) {
	if target == "" {
		return Explanation{}, fmt.Errorf("explain target is required")
	}
	if task, ok := cfg.ResolveTaskName(target); ok {
		return explainTask(cfg, target, task), nil
	}
	if pkg, ok := cfg.Codemap.Packages[target]; ok {
		return explainPackage(target, pkg), nil
	}
	for name, pkg := range cfg.Codemap.Packages {
		for _, entry := range pkg.EntryPoints {
			if entry == target {
				return explainPackage(name, pkg), nil
			}
		}
		for _, keyType := range pkg.KeyTypes {
			if keyType == target {
				return explainPackage(name, pkg), nil
			}
		}
	}
	if definition, ok := cfg.Codemap.Glossary[target]; ok {
		return explainGlossary(target, definition), nil
	}
	return Explanation{}, fmt.Errorf("unknown explain target %q", target)
}

func RelevantPackages(cfg *config.Config, scopePaths []string) []PackageExplanation {
	if len(scopePaths) == 0 {
		return nil
	}
	var out []PackageExplanation
	for name, entry := range cfg.Codemap.Packages {
		if matchesScope(name, scopePaths) {
			out = append(out, PackageExplanation{
				Name:        name,
				Description: entry.Desc,
				KeyTypes:    append([]string(nil), entry.KeyTypes...),
				EntryPoints: append([]string(nil), entry.EntryPoints...),
				Conventions: append([]string(nil), entry.Conventions...),
				DependsOn:   append([]string(nil), entry.DependsOn...),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func RenderRelevantPackages(pkgs []PackageExplanation) string {
	if len(pkgs) == 0 {
		return ""
	}
	var lines []string
	for _, pkg := range pkgs {
		line := fmt.Sprintf("- `%s`: %s", pkg.Name, pkg.Description)
		lines = append(lines, line)
		if len(pkg.EntryPoints) > 0 {
			lines = append(lines, fmt.Sprintf("  Entry points: `%s`", strings.Join(pkg.EntryPoints, "`, `")))
		}
		if len(pkg.KeyTypes) > 0 {
			lines = append(lines, fmt.Sprintf("  Key types: `%s`", strings.Join(pkg.KeyTypes, "`, `")))
		}
		if len(pkg.Conventions) > 0 {
			for _, convention := range pkg.Conventions {
				lines = append(lines, fmt.Sprintf("  Convention: %s", convention))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func explainTask(cfg *config.Config, invokedName, resolvedName string) Explanation {
	task := cfg.Tasks[resolvedName]
	taskOut := &TaskExplanation{
		Name:        resolvedName,
		Description: task.Desc,
		Type:        task.Type(),
		Scope:       task.Scope,
		Steps:       append([]string(nil), task.Steps...),
		Command:     task.Cmd,
	}
	if task.Scope != "" {
		taskOut.ScopePaths = append([]string(nil), cfg.Scopes[task.Scope].Paths...)
	}

	var lines []string
	lines = append(lines, "# fkn explain", "", fmt.Sprintf("Target: `%s`", invokedName), "", fmt.Sprintf("Task `%s`: %s", resolvedName, task.Desc))
	lines = append(lines, fmt.Sprintf("- Type: `%s`", task.Type()))
	if task.Scope != "" {
		scopeDef := cfg.Scopes[task.Scope]
		lines = append(lines, fmt.Sprintf("- Scope: `%s`", task.Scope))
		if scopeDef.Desc != "" {
			lines = append(lines, fmt.Sprintf("- Scope intent: %s", scopeDef.Desc))
		}
		lines = append(lines, fmt.Sprintf("- Paths: %s", strings.Join(scopeDef.Paths, ", ")))
	}
	if len(task.Steps) > 0 {
		lines = append(lines, fmt.Sprintf("- Steps: `%s`", strings.Join(task.Steps, "`, `")))
	}
	if task.Cmd != "" {
		lines = append(lines, fmt.Sprintf("- Command: `%s`", task.Cmd))
	}

	return Explanation{Target: invokedName, Kind: "task", Description: task.Desc, Task: taskOut, Markdown: strings.Join(lines, "\n")}
}

func explainPackage(name string, entry config.CodemapPackage) Explanation {
	pkg := &PackageExplanation{
		Name:        name,
		Description: entry.Desc,
		KeyTypes:    append([]string(nil), entry.KeyTypes...),
		EntryPoints: append([]string(nil), entry.EntryPoints...),
		Conventions: append([]string(nil), entry.Conventions...),
		DependsOn:   append([]string(nil), entry.DependsOn...),
	}
	var lines []string
	lines = append(lines, "# fkn explain", "", fmt.Sprintf("Package `%s`", name), "", entry.Desc)
	if len(entry.KeyTypes) > 0 {
		lines = append(lines, "", fmt.Sprintf("- Key types: `%s`", strings.Join(entry.KeyTypes, "`, `")))
	}
	if len(entry.EntryPoints) > 0 {
		lines = append(lines, fmt.Sprintf("- Entry points: `%s`", strings.Join(entry.EntryPoints, "`, `")))
	}
	if len(entry.DependsOn) > 0 {
		lines = append(lines, fmt.Sprintf("- Depends on: `%s`", strings.Join(entry.DependsOn, "`, `")))
	}
	if len(entry.Conventions) > 0 {
		lines = append(lines, "- Conventions:")
		for _, convention := range entry.Conventions {
			lines = append(lines, fmt.Sprintf("  - %s", convention))
		}
	}
	return Explanation{Target: name, Kind: "package", Description: entry.Desc, Package: pkg, Markdown: strings.Join(lines, "\n")}
}

func explainGlossary(term, definition string) Explanation {
	markdown := strings.Join([]string{"# fkn explain", "", fmt.Sprintf("Glossary `%s`", term), "", definition}, "\n")
	return Explanation{Target: term, Kind: "glossary", Description: definition, Glossary: &GlossaryExplanation{Term: term, Definition: definition}, Markdown: markdown}
}

func matchesScope(pkg string, scopePaths []string) bool {
	for _, raw := range scopePaths {
		path := strings.TrimSuffix(raw, "/")
		if path == "" {
			continue
		}
		if pkg == path || strings.HasPrefix(pkg, path+"/") || strings.HasPrefix(path, pkg+"/") {
			return true
		}
	}
	return false
}
