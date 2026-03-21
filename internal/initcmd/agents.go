package initcmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
)

func renderAgentsFKN(cfg *config.Config) (string, error) {
	var builder strings.Builder
	builder.WriteString("# AGENTS_FKN\n\n")
	builder.WriteString("This repository uses `fkn` as its structured task interface.\n\n")
	builder.WriteString("## How To Work Here\n\n")
	builder.WriteString("- Start with `fkn list` to discover the available tasks.\n")
	builder.WriteString("- Use `fkn help <task>` before inventing an equivalent command.\n")
	builder.WriteString("- Use `fkn context` for a bounded repo summary.\n")
	builder.WriteString("- Use `fkn guard` when you want a validation report across multiple checks.\n")
	builder.WriteString("- Prefer `fkn` tasks over ad hoc shell commands when the task already exists.\n\n")

	builder.WriteString("## Project\n\n")
	builder.WriteString(fmt.Sprintf("- Project: `%s`\n", cfg.Project))
	if cfg.Description != "" {
		builder.WriteString(fmt.Sprintf("- Description: %s\n", cfg.Description))
	}
	builder.WriteString("\n## Tasks\n\n")
	for _, name := range sortedTaskNames(cfg.Tasks) {
		task := cfg.Tasks[name]
		builder.WriteString(fmt.Sprintf("- `%s`: %s\n", name, task.Desc))
		if task.Scope != "" {
			builder.WriteString(fmt.Sprintf("  Scope: `%s`\n", task.Scope))
			if scopeDef, ok := cfg.Scopes[task.Scope]; ok && scopeDef.Desc != "" {
				builder.WriteString(fmt.Sprintf("  Scope Description: %s\n", scopeDef.Desc))
			}
		}
		if len(task.Steps) > 0 {
			builder.WriteString(fmt.Sprintf("  Steps: `%s`\n", strings.Join(task.Steps, "`, `")))
		}
		if len(task.Needs) > 0 {
			builder.WriteString(fmt.Sprintf("  Needs: `%s`\n", strings.Join(task.Needs, "`, `")))
		}
		if task.Cmd != "" {
			builder.WriteString(fmt.Sprintf("  Command: `%s`\n", task.Cmd))
		}
		builder.WriteString(fmt.Sprintf("  Agent visible: `%t`\n", task.AgentEnabled()))
	}

	writeGuardSection(&builder, cfg)
	writeGroupSection(&builder, cfg)
	writeScopeSection(&builder, cfg)
	writePromptSection(&builder, cfg)
	writeContextSection(&builder, cfg)
	writeServeSection(&builder, cfg)
	writeWatchSection(&builder, cfg)

	builder.WriteString("\n## Suggested Command Order\n\n")
	builder.WriteString("1. `fkn list`\n")
	builder.WriteString("2. `fkn help <task>`\n")
	builder.WriteString("3. `fkn context` or `fkn context --agent --task <task>`\n")
	builder.WriteString("4. `fkn <task>` or `fkn guard`\n")
	return builder.String(), nil
}

func writeGuardSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Guards) == 0 {
		return
	}
	builder.WriteString("\n## Guards\n\n")
	for _, name := range sortedGuardNames(cfg.Guards) {
		guardCfg := cfg.Guards[name]
		builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(guardCfg.Steps, "`, `")))
	}
}

func writeGroupSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Groups) == 0 {
		return
	}
	builder.WriteString("\n## Groups\n\n")
	for _, name := range sortedGroupNames(cfg.Groups) {
		group := cfg.Groups[name]
		builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(group.Tasks, "`, `")))
		if group.Desc != "" {
			builder.WriteString(fmt.Sprintf("  Description: %s\n", group.Desc))
		}
	}
}

func writeScopeSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Scopes) == 0 {
		return
	}
	builder.WriteString("\n## Scopes\n\n")
	for _, name := range sortedScopeNames(cfg.Scopes) {
		scopeDef := cfg.Scopes[name]
		builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(scopeDef.Paths, "`, `")))
		if scopeDef.Desc != "" {
			builder.WriteString(fmt.Sprintf("  Description: %s\n", scopeDef.Desc))
		}
	}
}

func writePromptSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Prompts) == 0 {
		return
	}
	builder.WriteString("\n## Prompts\n\n")
	for _, name := range sortedPromptNames(cfg.Prompts) {
		promptCfg := cfg.Prompts[name]
		builder.WriteString(fmt.Sprintf("- `%s`: %s\n", name, promptCfg.Desc))
	}
}

func writeContextSection(builder *strings.Builder, cfg *config.Config) {
	builder.WriteString("\n## Context\n\n")
	builder.WriteString("- Use `fkn context` for a general repo briefing.\n")
	builder.WriteString("- Use `fkn context --agent --task <task>` when working on a specific task.\n")
	if len(cfg.Context.AgentFiles) > 0 {
		builder.WriteString(fmt.Sprintf("- Agent files: `%s`\n", strings.Join(cfg.Context.AgentFiles, "`, `")))
	}
	if len(cfg.Context.Include) > 0 {
		builder.WriteString(fmt.Sprintf("- Included paths: `%s`\n", strings.Join(cfg.Context.Include, "`, `")))
	}
}

func writeServeSection(builder *strings.Builder, cfg *config.Config) {
	if cfg.Serve.Transport == "" && cfg.Serve.Port == 0 {
		return
	}
	builder.WriteString("\n## MCP\n\n")
	builder.WriteString(fmt.Sprintf("- Serve transport: `%s`\n", cfg.Serve.Transport))
	if cfg.Serve.Port != 0 {
		builder.WriteString(fmt.Sprintf("- HTTP port: `%d`\n", cfg.Serve.Port))
	}
	builder.WriteString("- Use `fkn list --mcp` to inspect exposed agent tools.\n")
}

func writeWatchSection(builder *strings.Builder, cfg *config.Config) {
	if len(cfg.Watch.Paths) == 0 {
		return
	}
	builder.WriteString("\n## Watch\n\n")
	builder.WriteString(fmt.Sprintf("- Watched paths: `%s`\n", strings.Join(cfg.Watch.Paths, "`, `")))
	builder.WriteString(fmt.Sprintf("- Debounce: `%dms`\n", cfg.Watch.DebounceMS))
}

func ensureAgentsReference(path string) (bool, error) {
	block := strings.Join([]string{
		agentsBlockStart,
		"## fkn Workflow",
		"",
		"If `fkn.yaml` exists in this repo:",
		"- read `AGENTS_FKN.md`",
		"- start with `fkn list`",
		"- use `fkn help <task>` before guessing commands",
		"- use `fkn context` or `fkn guard` when you need bounded repo context or validation",
		agentsBlockEnd,
		"",
	}, "\n")

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	content := string(raw)
	if strings.Contains(content, agentsBlockStart) && strings.Contains(content, agentsBlockEnd) {
		start := strings.Index(content, agentsBlockStart)
		end := strings.Index(content, agentsBlockEnd)
		if start >= 0 && end >= start {
			end += len(agentsBlockEnd)
			updated := content[:start] + block + strings.TrimLeft(content[end:], "\n")
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return false, err
			}
			return false, nil
		}
	}

	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		content = block
	} else {
		content = trimmed + "\n\n" + block
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func ensureGitignoreEntry(path, entry string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	lines := []string{}
	if len(raw) > 0 {
		lines = strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == entry {
				return false, nil
			}
		}
	}

	content := strings.TrimRight(string(raw), "\n")
	if content == "" {
		content = entry + "\n"
	} else {
		content = content + "\n" + entry + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
