package initcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
)

type Options struct {
	FromRepo bool
	Agents   bool
}

const starterConfig = `project: my-project
description: Describe your repository

tasks:
  test:
    desc: Run the test suite
    cmd: go test ./...

  build:
    desc: Build the application
    cmd: go build ./...

  check:
    desc: Run the default local verification pipeline
    steps:
      - test
      - build
`

const (
	agentsBlockStart = "<!-- fkn:agents:start -->"
	agentsBlockEnd   = "<!-- fkn:agents:end -->"
)

type inferredTask struct {
	Name  string
	Desc  string
	Cmd   string
	Steps []string
	Agent *bool
}

func Run(repoRoot string, opts Options) (string, error) {
	var messages []string

	cfgPath := filepath.Join(repoRoot, "fkn.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		messages = append(messages, "fkn.yaml already exists; leaving it unchanged")
	} else if os.IsNotExist(err) {
		configBody := starterConfig
		if opts.FromRepo {
			configBody = inferConfig(repoRoot)
		}
		if err := os.WriteFile(cfgPath, []byte(configBody), 0o644); err != nil {
			return "", err
		}
		messages = append(messages, "created fkn.yaml")
		if opts.FromRepo {
			messages = append(messages, "scaffolded tasks from existing repo files")
		}
	} else {
		return "", err
	}

	updated, err := ensureGitignoreEntry(filepath.Join(repoRoot, ".gitignore"), ".fkn/")
	if err != nil {
		return "", err
	}
	if updated {
		messages = append(messages, "updated .gitignore with .fkn/")
	} else {
		messages = append(messages, ".gitignore already includes .fkn/")
	}

	if opts.Agents {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return "", err
		}
		agentDoc, err := renderAgentsFKN(cfg)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "AGENTS_FKN.md"), []byte(agentDoc), 0o644); err != nil {
			return "", err
		}
		messages = append(messages, "wrote AGENTS_FKN.md")

		updatedAgents, err := ensureAgentsReference(filepath.Join(repoRoot, "AGENTS.md"))
		if err != nil {
			return "", err
		}
		if updatedAgents {
			messages = append(messages, "updated AGENTS.md with fkn guidance")
		} else {
			messages = append(messages, "AGENTS.md already includes fkn guidance")
		}
	}

	return strings.Join(messages, "\n"), nil
}

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
		}
		if len(task.Steps) > 0 {
			builder.WriteString(fmt.Sprintf("  Steps: `%s`\n", strings.Join(task.Steps, "`, `")))
		}
		if task.Cmd != "" {
			builder.WriteString(fmt.Sprintf("  Command: `%s`\n", task.Cmd))
		}
		builder.WriteString(fmt.Sprintf("  Agent visible: `%t`\n", task.AgentEnabled()))
	}

	if len(cfg.Guards) > 0 {
		builder.WriteString("\n## Guards\n\n")
		for _, name := range sortedGuardNames(cfg.Guards) {
			guardCfg := cfg.Guards[name]
			builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(guardCfg.Steps, "`, `")))
		}
	}

	if len(cfg.Scopes) > 0 {
		builder.WriteString("\n## Scopes\n\n")
		for _, name := range sortedScopeNames(cfg.Scopes) {
			paths := cfg.Scopes[name]
			builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(paths, "`, `")))
		}
	}

	if len(cfg.Prompts) > 0 {
		builder.WriteString("\n## Prompts\n\n")
		for _, name := range sortedPromptNames(cfg.Prompts) {
			promptCfg := cfg.Prompts[name]
			builder.WriteString(fmt.Sprintf("- `%s`: %s\n", name, promptCfg.Desc))
		}
	}

	builder.WriteString("\n## Context\n\n")
	builder.WriteString("- Use `fkn context` for a general repo briefing.\n")
	builder.WriteString("- Use `fkn context --agent --task <task>` when working on a specific task.\n")
	if len(cfg.Context.AgentFiles) > 0 {
		builder.WriteString(fmt.Sprintf("- Agent files: `%s`\n", strings.Join(cfg.Context.AgentFiles, "`, `")))
	}
	if len(cfg.Context.Include) > 0 {
		builder.WriteString(fmt.Sprintf("- Included paths: `%s`\n", strings.Join(cfg.Context.Include, "`, `")))
	}

	if cfg.Serve.Transport != "" || cfg.Serve.Port != 0 {
		builder.WriteString("\n## MCP\n\n")
		builder.WriteString(fmt.Sprintf("- Serve transport: `%s`\n", cfg.Serve.Transport))
		if cfg.Serve.Port != 0 {
			builder.WriteString(fmt.Sprintf("- HTTP port: `%d`\n", cfg.Serve.Port))
		}
		builder.WriteString("- Use `fkn list --mcp` to inspect exposed agent tools.\n")
	}

	if len(cfg.Watch.Paths) > 0 {
		builder.WriteString("\n## Watch\n\n")
		builder.WriteString(fmt.Sprintf("- Watched paths: `%s`\n", strings.Join(cfg.Watch.Paths, "`, `")))
		builder.WriteString(fmt.Sprintf("- Debounce: `%dms`\n", cfg.Watch.DebounceMS))
	}

	builder.WriteString("\n## Suggested Command Order\n\n")
	builder.WriteString("1. `fkn list`\n")
	builder.WriteString("2. `fkn help <task>`\n")
	builder.WriteString("3. `fkn context` or `fkn context --agent --task <task>`\n")
	builder.WriteString("4. `fkn <task>` or `fkn guard`\n")
	return builder.String(), nil
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

func inferConfig(repoRoot string) string {
	project := filepath.Base(repoRoot)
	tasks := inferTasks(repoRoot)
	if len(tasks) == 0 {
		return starterConfig
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("project: %s\n", project))
	builder.WriteString("description: Generated by fkn init --from-repo\n\n")
	builder.WriteString("tasks:\n")
	for _, task := range tasks {
		builder.WriteString(fmt.Sprintf("  %s:\n", task.Name))
		builder.WriteString(fmt.Sprintf("    desc: %s\n", task.Desc))
		if task.Cmd != "" {
			builder.WriteString(fmt.Sprintf("    cmd: %s\n", task.Cmd))
		}
		if task.Agent != nil {
			builder.WriteString(fmt.Sprintf("    agent: %t\n", *task.Agent))
		}
		if len(task.Steps) > 0 {
			builder.WriteString("    steps:\n")
			for _, step := range task.Steps {
				builder.WriteString(fmt.Sprintf("      - %s\n", step))
			}
		}
	}

	guardSteps := inferredGuardSteps(tasks)
	if len(guardSteps) > 0 {
		builder.WriteString("\nguards:\n")
		builder.WriteString("  default:\n")
		builder.WriteString("    steps:\n")
		for _, step := range guardSteps {
			builder.WriteString(fmt.Sprintf("      - %s\n", step))
		}
	}

	if watchPaths := inferredWatchPaths(repoRoot); len(watchPaths) > 0 {
		builder.WriteString("\nwatch:\n")
		builder.WriteString("  debounce_ms: 500\n")
		builder.WriteString("  paths:\n")
		for _, path := range watchPaths {
			builder.WriteString(fmt.Sprintf("    - %s\n", path))
		}
	}

	return builder.String()
}

func inferTasks(repoRoot string) []inferredTask {
	taskByName := map[string]inferredTask{}
	order := []string{}

	addTask := func(name, desc, cmd string, agent *bool) {
		if _, exists := taskByName[name]; exists {
			return
		}
		taskByName[name] = inferredTask{Name: name, Desc: desc, Cmd: cmd, Agent: agent}
		order = append(order, name)
	}

	for _, target := range findMakeTargets(repoRoot) {
		if shouldSkipInferredTarget(target) {
			continue
		}
		addTask(target, inferredTargetDesc("repository", target, "target"), fmt.Sprintf("make %s", target), inferredTargetAgent(target))
	}

	for _, target := range findJustTargets(repoRoot) {
		if shouldSkipInferredTarget(target) {
			continue
		}
		addTask(target, inferredTargetDesc("repository", target, "recipe"), fmt.Sprintf("just %s", target), inferredTargetAgent(target))
	}

	scripts := findPackageScripts(repoRoot)
	scriptNames := make([]string, 0, len(scripts))
	for name := range scripts {
		scriptNames = append(scriptNames, name)
	}
	sort.Strings(scriptNames)
	for _, name := range scriptNames {
		switch name {
		case "check":
			addTask("check", "Run the package.json check script", fmt.Sprintf("npm run %s", name), nil)
		case "test":
			addTask("test", "Run the package.json test script", fmt.Sprintf("npm run %s", name), nil)
		case "build":
			addTask("build", "Run the package.json build script", fmt.Sprintf("npm run %s", name), nil)
		case "lint":
			addTask("lint", "Run the package.json lint script", fmt.Sprintf("npm run %s", name), nil)
		case "dev":
			addTask("dev", "Run the package.json dev script", fmt.Sprintf("npm run %s", name), nil)
		case "start":
			addTask("start", "Run the package.json start script", fmt.Sprintf("npm run %s", name), nil)
		default:
			if strings.HasPrefix(name, "test:") {
				addTask(scriptTaskName(name), fmt.Sprintf("Run the package.json %s script", name), fmt.Sprintf("npm run %s", name), nil)
			}
			if strings.HasPrefix(name, "lint:") {
				addTask(scriptTaskName(name), fmt.Sprintf("Run the package.json %s script", name), fmt.Sprintf("npm run %s", name), nil)
			}
		}
	}

	if hasFile(repoRoot, "go.mod") {
		addTask("test", "Run the Go test suite", "go test ./...", nil)
		addTask("build", "Build the Go packages", "go build ./...", nil)
	}

	if _, ok := taskByName["check"]; !ok {
		steps := []string{}
		for _, name := range []string{"lint", "test", "build"} {
			if _, exists := taskByName[name]; exists {
				steps = append(steps, name)
			}
		}
		if len(steps) >= 2 {
			taskByName["check"] = inferredTask{
				Name:  "check",
				Desc:  "Run the default local verification pipeline",
				Steps: steps,
			}
			order = append(order, "check")
		}
	}

	tasks := make([]inferredTask, 0, len(order))
	for _, name := range order {
		task, ok := taskByName[name]
		if !ok {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks
}

func inferredTargetDesc(subject, name, kind string) string {
	return fmt.Sprintf("Run the %s %s %s", subject, name, kind)
}

func inferredTargetAgent(name string) *bool {
	if name == "clean" {
		value := false
		return &value
	}
	return nil
}

func shouldSkipInferredTarget(name string) bool {
	switch name {
	case "clean", "add-feature", "add-feature-git":
		return true
	default:
		return false
	}
}

func inferredGuardSteps(tasks []inferredTask) []string {
	names := map[string]bool{}
	for _, task := range tasks {
		names[task.Name] = true
	}
	steps := []string{}
	for _, name := range []string{"lint", "test", "build"} {
		if names[name] {
			steps = append(steps, name)
		}
	}
	if len(steps) == 0 && names["check"] {
		steps = []string{"check"}
	}
	return steps
}

func inferredWatchPaths(repoRoot string) []string {
	paths := []string{"README.md"}
	if hasFile(repoRoot, "go.mod") {
		paths = append(paths, "cmd/", "internal/", "go.mod")
	}
	if hasFile(repoRoot, "package.json") {
		paths = append(paths, "src/", "package.json")
	}
	if hasFile(repoRoot, "Makefile") {
		paths = append(paths, "Makefile")
	}

	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

func sortedTaskNames(tasks map[string]config.Task) []string {
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedGuardNames(guards map[string]config.Guard) []string {
	names := make([]string, 0, len(guards))
	for name := range guards {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedPromptNames(prompts map[string]config.Prompt) []string {
	names := make([]string, 0, len(prompts))
	for name := range prompts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedScopeNames(scopes map[string][]string) []string {
	names := make([]string, 0, len(scopes))
	for name := range scopes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func findMakeTargets(repoRoot string) []string {
	return findTargets(filepath.Join(repoRoot, "Makefile"))
}

func findJustTargets(repoRoot string) []string {
	return findTargets(filepath.Join(repoRoot, "justfile"))
}

func findTargets(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var targets []string
	for _, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		if line == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(line, "\t") {
			continue
		}
		colon := strings.Index(line, ":")
		if colon <= 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		rest := strings.TrimSpace(line[colon+1:])
		if name == "" || strings.Contains(name, " ") || strings.HasPrefix(name, ".") {
			continue
		}
		if strings.HasPrefix(rest, "=") {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		targets = append(targets, name)
	}
	return targets
}

func findPackageScripts(repoRoot string) map[string]string {
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
	return pkg.Scripts
}

func scriptTaskName(name string) string {
	replacer := strings.NewReplacer(":", "-", "/", "-")
	return replacer.Replace(name)
}

func hasFile(repoRoot, name string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, name))
	return err == nil
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
