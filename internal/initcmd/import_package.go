package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
	"github.com/neural-chilli/fkn/internal/ordered"
)

func findPackageScripts(repoRoot string) map[string]packageScript {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil
	}
	scripts := make(map[string]packageScript, len(pkg.Scripts))
	for name, command := range pkg.Scripts {
		scripts[name] = packageScript{
			Name:   name,
			Cmd:    buildPackageScriptCommand(name, command),
			Params: inferredPackageScriptParams(command),
		}
	}
	return scripts
}

func buildPackageScriptCommand(name, command string) string {
	params := inferredPackageScriptParams(command)
	if len(params) == 0 {
		return fmt.Sprintf("npm run %s", name)
	}
	paramNames := ordered.Keys(params)
	parts := []string{fmt.Sprintf("npm run %s --", name)}
	for _, paramName := range paramNames {
		parts = append(parts, fmt.Sprintf("--%s={{params.%s}}", paramName, paramName))
	}
	return strings.Join(parts, " ")
}

func inferredPackageScriptParams(command string) map[string]config.Param {
	names := findPackageScriptParamNames(command)
	if len(names) == 0 {
		return nil
	}
	params := make(map[string]config.Param, len(names))
	for _, name := range names {
		params[name] = config.Param{
			Desc:     fmt.Sprintf("Value for the %s package script argument", name),
			Env:      "npm_config_" + strings.ReplaceAll(name, "-", "_"),
			Required: true,
		}
	}
	return params
}

func findPackageScriptParamNames(command string) []string {
	patterns := []string{
		"npm_config_",
		"process.env.npm_config_",
	}
	seen := map[string]bool{}
	var names []string
	for _, pattern := range patterns {
		remaining := command
		for {
			index := strings.Index(remaining, pattern)
			if index < 0 {
				break
			}
			remaining = remaining[index+len(pattern):]
			var token strings.Builder
			for _, r := range remaining {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
					token.WriteRune(r)
					continue
				}
				break
			}
			rawName := token.String()
			if rawName == "" {
				continue
			}
			name := strings.ToLower(strings.ReplaceAll(rawName, "_", "-"))
			if seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func scriptTaskName(name string) string {
	replacer := strings.NewReplacer(":", "-", "/", "-")
	return replacer.Replace(name)
}
