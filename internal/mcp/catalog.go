package mcp

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
)

func (s *Server) Tools() []Tool {
	names := make([]string, 0, len(s.cfg.Tasks))
	for name := range s.cfg.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)

	tools := make([]Tool, 0, len(names))
	for _, name := range names {
		task := s.cfg.Tasks[name]
		if !task.AgentEnabled() {
			continue
		}
		properties := map[string]any{
			"env": map[string]any{
				"type":        "object",
				"description": "Optional env var overrides for this invocation",
			},
			"dry_run": map[string]any{
				"type":        "boolean",
				"description": "Print the command without executing it",
			},
			"allow_unsafe": map[string]any{
				"type":        "boolean",
				"description": "Required for tasks marked destructive or external",
			},
		}
		required := []string{}
		for _, paramName := range sortedParamNames(task.Params) {
			param := task.Params[paramName]
			prop := map[string]any{
				"type":        "string",
				"description": param.Desc,
			}
			if param.Default != "" {
				prop["default"] = param.Default
			}
			properties[paramName] = prop
			if param.Required {
				required = append(required, paramName)
			}
		}
		inputSchema := map[string]any{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			inputSchema["required"] = required
		}
		tools = append(tools, Tool{
			Name:        name,
			Description: task.Desc,
			InputSchema: inputSchema,
			Annotations: safetyAnnotations(task),
		})
	}
	return tools
}

func safetyAnnotations(task config.Task) map[string]any {
	safety := task.SafetyLevel()
	annotations := map[string]any{
		"fknSafety": safety,
	}
	switch safety {
	case "safe":
		annotations["readOnlyHint"] = true
	case "idempotent":
		annotations["idempotentHint"] = true
	case "destructive":
		annotations["destructiveHint"] = true
	case "external":
		annotations["openWorldHint"] = true
	}
	return annotations
}

func (s *Server) instructions() string {
	var parts []string
	if s.cfg.Project != "" {
		parts = append(parts, "Project: "+s.cfg.Project)
	}
	if s.cfg.Description != "" {
		parts = append(parts, s.cfg.Description)
	}
	parts = append(parts, "Use tools/list to discover agent-visible tasks and resources/list for repo context resources.")
	if len(s.cfg.Guards) > 0 {
		parts = append(parts, "Common verification usually starts with fkn guard.")
	}
	return strings.Join(parts, " ")
}

func (s *Server) Prompts() []Prompt {
	names := make([]string, 0, len(s.cfg.Prompts))
	for name := range s.cfg.Prompts {
		names = append(names, name)
	}
	sort.Strings(names)

	prompts := make([]Prompt, 0, len(names))
	for _, name := range names {
		promptCfg := s.cfg.Prompts[name]
		prompts = append(prompts, Prompt{
			Name:        name,
			Description: promptCfg.Desc,
		})
	}
	return prompts
}

func (s *Server) Resources() []Resource {
	resources := []Resource{
		{
			URI:         "fkn://context",
			Name:        "Repository Context",
			Description: "Bounded markdown context for the repository",
			MimeType:    "text/markdown",
		},
		{
			URI:         "fkn://context.json",
			Name:        "Repository Context JSON",
			Description: "Structured bounded context for the repository",
			MimeType:    "application/json",
		},
	}

	if fileExists(filepath.Join(s.repoRoot, ".fkn", "last-guard.json")) {
		resources = append(resources, Resource{
			URI:         "fkn://guard/last",
			Name:        "Last Guard Result",
			Description: "Cached JSON result from the most recent guard run",
			MimeType:    "application/json",
		})
	}

	for _, name := range sortedScopeNames(s.cfg.Scopes) {
		description := "Path list for the " + name + " scope"
		if s.cfg.Scopes[name].Desc != "" {
			description = s.cfg.Scopes[name].Desc
		}
		resources = append(resources, Resource{
			URI:         "fkn://scope/" + name,
			Name:        "Scope " + name,
			Description: description,
			MimeType:    "text/plain",
		})
	}

	return resources
}

func sortedParamNames(params map[string]config.Param) []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedScopeNames(scopes map[string]config.Scope) []string {
	names := make([]string, 0, len(scopes))
	for name := range scopes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
