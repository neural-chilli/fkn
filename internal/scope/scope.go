package scope

import (
	"fmt"
	"strings"

	"fkn/internal/config"
)

type Result struct {
	Scope string   `json:"scope"`
	Paths []string `json:"paths"`
}

func Get(cfg *config.Config, name string) (Result, error) {
	paths, ok := cfg.Scopes[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown scope %q", name)
	}
	return Result{
		Scope: name,
		Paths: append([]string(nil), paths...),
	}, nil
}

func FormatPrompt(name string, paths []string) string {
	if len(paths) == 0 {
		return fmt.Sprintf("Only modify files in the declared scope `%s`.", name)
	}
	return fmt.Sprintf("Only modify files in the declared scope `%s`: %s", name, strings.Join(paths, ", "))
}
