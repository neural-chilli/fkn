package fkn

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed README.md docs/user-guide.md docs/mcp.md docs/releasing.md
var embeddedDocs embed.FS

var docPaths = map[string]string{
	"readme":     "README.md",
	"user-guide": "docs/user-guide.md",
	"mcp":        "docs/mcp.md",
	"releasing":  "docs/releasing.md",
	"release":    "docs/releasing.md",
}

var primaryDocNames = []string{"readme", "user-guide", "mcp", "releasing"}

func DocNames() []string {
	names := append([]string{}, primaryDocNames...)
	return names
}

func Doc(name string) (string, error) {
	if name == "" {
		name = "readme"
	}
	path, ok := docPaths[strings.ToLower(name)]
	if !ok {
		return "", fmt.Errorf("unknown docs page %q", name)
	}
	raw, err := embeddedDocs.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
