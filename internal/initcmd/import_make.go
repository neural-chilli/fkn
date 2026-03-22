package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
)

func findMakeTargets(repoRoot string) []makeTarget {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "Makefile"))
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	assigned := findAssignedMakeVariables(lines)
	seen := map[string]bool{}
	var targets []makeTarget
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		name, ok := parseTargetName(line)
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		paramsSet := map[string]bool{}
		for j := i + 1; j < len(lines); j++ {
			next := lines[j]
			if next == "" {
				continue
			}
			if !strings.HasPrefix(next, "\t") {
				if _, ok := parseTargetName(next); ok {
					break
				}
				if strings.TrimSpace(next) != "" {
					break
				}
			}
			for _, param := range findMakeVariables(next) {
				if assigned[param] {
					continue
				}
				paramsSet[param] = true
			}
		}
		params := make([]string, 0, len(paramsSet))
		for param := range paramsSet {
			params = append(params, param)
		}
		sort.Strings(params)
		targets = append(targets, makeTarget{Name: name, Params: params})
	}
	return targets
}

func findAssignedMakeVariables(lines []string) map[string]bool {
	assigned := map[string]bool{}
	for _, line := range lines {
		name, ok := parseAssignedMakeVariable(line)
		if !ok {
			continue
		}
		assigned[name] = true
	}
	return assigned
}

func parseAssignedMakeVariable(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "\t") {
		return "", false
	}
	for _, op := range []string{":=", "?=", "+=", "="} {
		index := strings.Index(trimmed, op)
		if index <= 0 {
			continue
		}
		name := strings.TrimSpace(trimmed[:index])
		if name == "" || !isUpperSnake(name) {
			return "", false
		}
		return name, true
	}
	return "", false
}

func parseTargetName(line string) (string, bool) {
	colon := strings.Index(line, ":")
	if colon <= 0 {
		return "", false
	}
	name := strings.TrimSpace(line[:colon])
	rest := strings.TrimSpace(line[colon+1:])
	if name == "" || strings.Contains(name, " ") || strings.HasPrefix(name, ".") {
		return "", false
	}
	if strings.HasPrefix(rest, "=") {
		return "", false
	}
	return name, true
}

func findMakeVariables(line string) []string {
	var names []string
	seen := map[string]bool{}
	for {
		start := strings.Index(line, "$(")
		if start < 0 {
			break
		}
		line = line[start+2:]
		end := strings.Index(line, ")")
		if end < 0 {
			break
		}
		name := strings.TrimSpace(line[:end])
		line = line[end+1:]
		if name == "" || seen[name] {
			continue
		}
		if !isUpperSnake(name) {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func inferredParams(names []string) map[string]config.Param {
	if len(names) == 0 {
		return nil
	}
	params := make(map[string]config.Param, len(names))
	for _, name := range names {
		paramName := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
		params[paramName] = config.Param{
			Desc:     fmt.Sprintf("Value for %s", name),
			Env:      name,
			Required: true,
		}
	}
	return params
}

func isUpperSnake(value string) bool {
	for _, r := range value {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
