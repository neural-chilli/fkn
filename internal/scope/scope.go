package scope

import (
	"fmt"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
)

type Result struct {
	Scope string   `json:"scope"`
	Desc  string   `json:"desc,omitempty"`
	Paths []string `json:"paths"`
}

func Get(cfg *config.Config, name string) (Result, error) {
	scopeDef, ok := cfg.Scopes[name]
	if !ok {
		return Result{}, fmt.Errorf("unknown scope %q", name)
	}
	return Result{
		Scope: name,
		Desc:  scopeDef.Desc,
		Paths: append([]string(nil), scopeDef.Paths...),
	}, nil
}

func FormatPrompt(name, desc string, paths []string) string {
	if desc != "" && len(paths) == 0 {
		return fmt.Sprintf("Only modify files in the declared scope `%s`. Scope intent: %s", name, desc)
	}
	if desc != "" {
		return fmt.Sprintf("Only modify files in the declared scope `%s`: %s. Scope intent: %s", name, strings.Join(paths, ", "), desc)
	}
	if len(paths) == 0 {
		return fmt.Sprintf("Only modify files in the declared scope `%s`.", name)
	}
	return fmt.Sprintf("Only modify files in the declared scope `%s`: %s", name, strings.Join(paths, ", "))
}
