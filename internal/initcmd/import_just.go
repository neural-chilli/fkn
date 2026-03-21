package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
)

func findJustRecipes(repoRoot string) []justRecipe {
	path := findJustfilePath(repoRoot)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	seen := map[string]bool{}
	privateNext := false
	var recipes []justRecipe

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "  ") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			privateNext = strings.Contains(trimmed, "private")
			continue
		}
		if strings.HasPrefix(trimmed, "alias ") || strings.HasPrefix(trimmed, "set ") || strings.Contains(trimmed, ":=") {
			privateNext = false
			continue
		}

		recipe, ok := parseJustRecipe(trimmed)
		if !ok {
			privateNext = false
			continue
		}
		if privateNext || strings.HasPrefix(recipe.Name, "_") || seen[recipe.Name] {
			privateNext = false
			continue
		}
		privateNext = false
		seen[recipe.Name] = true
		recipes = append(recipes, recipe)
	}

	return recipes
}

func findJustAliases(repoRoot string) map[string]string {
	path := findJustfilePath(repoRoot)
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	privateNext := false
	aliases := map[string]string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			privateNext = strings.Contains(trimmed, "private")
			continue
		}
		if privateNext {
			privateNext = false
			if strings.HasPrefix(trimmed, "alias ") {
				continue
			}
		}
		if !strings.HasPrefix(trimmed, "alias ") {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(trimmed, "alias "), ":=", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		target := strings.TrimSpace(parts[1])
		if name == "" || target == "" || strings.HasPrefix(name, "_") {
			continue
		}
		aliases[name] = target
	}
	return aliases
}

func findJustfilePath(repoRoot string) string {
	for _, name := range []string{"justfile", "Justfile"} {
		path := filepath.Join(repoRoot, name)
		if hasFile(repoRoot, name) {
			return path
		}
	}
	return ""
}

func parseJustRecipe(line string) (justRecipe, bool) {
	colon := strings.Index(line, ":")
	if colon <= 0 {
		return justRecipe{}, false
	}
	header := strings.TrimSpace(line[:colon])
	if header == "" {
		return justRecipe{}, false
	}
	fields := strings.Fields(header)
	if len(fields) == 0 {
		return justRecipe{}, false
	}
	name := fields[0]
	if strings.Contains(name, "=") || strings.HasPrefix(name, "@") {
		return justRecipe{}, false
	}

	params := make([]justParam, 0, len(fields)-1)
	for _, token := range fields[1:] {
		param, ok := parseJustParam(token)
		if !ok {
			break
		}
		params = append(params, param)
	}
	return justRecipe{Name: name, Params: params}, true
}

func parseJustParam(token string) (justParam, bool) {
	token = strings.TrimSpace(token)
	if token == "" || strings.HasPrefix(token, "(") || strings.HasPrefix(token, "[") {
		return justParam{}, false
	}
	required := true
	defaultValue := ""
	variadic := false
	switch {
	case strings.HasPrefix(token, "+"):
		variadic = true
	case strings.HasPrefix(token, "*"):
		variadic = true
		required = false
	}
	name := strings.TrimLeft(token, "+*$")
	if parts := strings.SplitN(name, "=", 2); len(parts) == 2 {
		name = strings.TrimSpace(parts[0])
		defaultValue = strings.TrimSpace(parts[1])
		required = false
	}
	name = sanitizeParamName(name)
	if name == "" {
		return justParam{}, false
	}
	return justParam{Name: name, Default: defaultValue, Required: required, Variadic: variadic}, true
}

func buildJustCommand(recipe justRecipe) string {
	parts := []string{"just", recipe.Name}
	for _, param := range recipe.Params {
		parts = append(parts, "{{params."+param.Name+"}}")
	}
	return strings.Join(parts, " ")
}

func inferredJustParams(params []justParam) map[string]config.Param {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]config.Param, len(params))
	for i, param := range params {
		out[param.Name] = config.Param{
			Desc:     fmt.Sprintf("Value for the %s recipe parameter", param.Name),
			Env:      strings.ToUpper(strings.ReplaceAll(param.Name, "-", "_")),
			Required: param.Required,
			Default:  param.Default,
			Position: i + 1,
			Variadic: param.Variadic,
		}
	}
	return out
}

func sanitizeParamName(name string) string {
	name = strings.TrimSpace(strings.Trim(name, `"'`))
	replacer := strings.NewReplacer("-", "_", ".", "_")
	name = replacer.Replace(name)
	return strings.Trim(name, "_")
}
