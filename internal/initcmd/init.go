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
default: check

tasks:
  test:
    desc: Run the test suite
    scope: cli
    cmd: go test ./...

  build:
    desc: Build the application
    scope: cli
    cmd: go build ./...

  check:
    desc: Run the default local verification pipeline
    scope: cli
    steps:
      - test
      - build

groups:
  core:
    desc: Everyday local development commands.
    tasks:
      - test
      - build
      - check

scopes:
  cli:
    desc: Main CLI commands and closely-related execution packages.
    paths:
      - cmd/
      - internal/
`

const (
	agentsBlockStart = "<!-- fkn:agents:start -->"
	agentsBlockEnd   = "<!-- fkn:agents:end -->"
)

type inferredTask struct {
	Name   string
	Desc   string
	Cmd    string
	Steps  []string
	Agent  *bool
	Params map[string]config.Param
}

type makeTarget struct {
	Name   string
	Params []string
}

type justRecipe struct {
	Name   string
	Params []justParam
}

type justParam struct {
	Name     string
	Default  string
	Required bool
}

type packageScript struct {
	Name   string
	Cmd    string
	Params map[string]config.Param
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

	if len(cfg.Guards) > 0 {
		builder.WriteString("\n## Guards\n\n")
		for _, name := range sortedGuardNames(cfg.Guards) {
			guardCfg := cfg.Guards[name]
			builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(guardCfg.Steps, "`, `")))
		}
	}

	if len(cfg.Groups) > 0 {
		builder.WriteString("\n## Groups\n\n")
		for _, name := range sortedGroupNames(cfg.Groups) {
			group := cfg.Groups[name]
			builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(group.Tasks, "`, `")))
			if group.Desc != "" {
				builder.WriteString(fmt.Sprintf("  Description: %s\n", group.Desc))
			}
		}
	}

	if len(cfg.Scopes) > 0 {
		builder.WriteString("\n## Scopes\n\n")
		for _, name := range sortedScopeNames(cfg.Scopes) {
			scopeDef := cfg.Scopes[name]
			builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", name, strings.Join(scopeDef.Paths, "`, `")))
			if scopeDef.Desc != "" {
				builder.WriteString(fmt.Sprintf("  Description: %s\n", scopeDef.Desc))
			}
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
	aliases := inferAliases(repoRoot, tasks)
	if len(tasks) == 0 {
		return starterConfig
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("project: %s\n", project))
	builder.WriteString("description: Generated by fkn init --from-repo\n")
	if defaultTask := inferredDefaultTask(tasks); defaultTask != "" {
		builder.WriteString(fmt.Sprintf("default: %s\n", defaultTask))
	}
	builder.WriteString("\n")
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
		if len(task.Params) > 0 {
			builder.WriteString("    params:\n")
			for _, paramName := range sortedParamNames(task.Params) {
				param := task.Params[paramName]
				builder.WriteString(fmt.Sprintf("      %s:\n", paramName))
				if param.Desc != "" {
					builder.WriteString(fmt.Sprintf("        desc: %s\n", param.Desc))
				}
				builder.WriteString(fmt.Sprintf("        env: %s\n", param.Env))
				if param.Required {
					builder.WriteString("        required: true\n")
				}
				if param.Default != "" {
					builder.WriteString(fmt.Sprintf("        default: %s\n", param.Default))
				}
			}
		}
		if len(task.Steps) > 0 {
			builder.WriteString("    steps:\n")
			for _, step := range task.Steps {
				builder.WriteString(fmt.Sprintf("      - %s\n", step))
			}
		}
	}

	if len(aliases) > 0 {
		builder.WriteString("\naliases:\n")
		for _, name := range sortedAliasNames(aliases) {
			builder.WriteString(fmt.Sprintf("  %s: %s\n", name, aliases[name]))
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

	addTask := func(name, desc, cmd string, agent *bool, params map[string]config.Param) {
		if _, exists := taskByName[name]; exists {
			return
		}
		taskByName[name] = inferredTask{Name: name, Desc: desc, Cmd: cmd, Agent: agent, Params: params}
		order = append(order, name)
	}

	for _, target := range findMakeTargets(repoRoot) {
		if shouldSkipInferredTarget(target.Name) {
			continue
		}
		addTask(
			target.Name,
			inferredTargetDesc("repository", target.Name, "target"),
			fmt.Sprintf("make %s", target.Name),
			inferredTargetAgent(target.Name, len(target.Params) > 0),
			inferredParams(target.Params),
		)
	}

	for _, recipe := range findJustRecipes(repoRoot) {
		if shouldSkipInferredTarget(recipe.Name) {
			continue
		}
		addTask(
			recipe.Name,
			inferredTargetDesc("repository", recipe.Name, "recipe"),
			buildJustCommand(recipe),
			inferredTargetAgent(recipe.Name, len(recipe.Params) > 0),
			inferredJustParams(recipe.Params),
		)
	}

	scripts := findPackageScripts(repoRoot)
	scriptNames := make([]string, 0, len(scripts))
	for name := range scripts {
		scriptNames = append(scriptNames, name)
	}
	sort.Strings(scriptNames)
	for _, name := range scriptNames {
		script := scripts[name]
		if !shouldInferPackageScript(name, script) {
			continue
		}
		addTask(
			scriptTaskName(name),
			fmt.Sprintf("Run the package.json %s script", name),
			script.Cmd,
			inferredTargetAgent(name, len(script.Params) > 0),
			script.Params,
		)
	}

	if hasFile(repoRoot, "go.mod") {
		addTask("test", "Run the Go test suite", "go test ./...", nil, nil)
		addTask("build", "Build the Go packages", "go build ./...", nil, nil)
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

func inferredDefaultTask(tasks []inferredTask) string {
	for _, candidate := range []string{"check", "verify", "test", "build"} {
		for _, task := range tasks {
			if task.Name == candidate {
				return candidate
			}
		}
	}
	return ""
}

func inferredTargetDesc(subject, name, kind string) string {
	return fmt.Sprintf("Run the %s %s %s", subject, name, kind)
}

func inferAliases(repoRoot string, tasks []inferredTask) map[string]string {
	taskNames := map[string]bool{}
	for _, task := range tasks {
		taskNames[task.Name] = true
	}

	aliases := map[string]string{}
	for alias, target := range findJustAliases(repoRoot) {
		if strings.Contains(target, "::") {
			continue
		}
		if taskNames[target] {
			aliases[alias] = target
		}
	}
	return aliases
}

func inferredTargetAgent(name string, hasParams bool) *bool {
	if shouldHideFromAgents(name, hasParams) {
		value := false
		return &value
	}
	return nil
}

func shouldSkipInferredTarget(name string) bool {
	switch name {
	case "add-feature-git":
		return true
	default:
		return false
	}
}

func shouldHideFromAgents(name string, hasParams bool) bool {
	lower := strings.ToLower(name)
	if lower == "clean" {
		return true
	}
	if hasParams && (strings.Contains(lower, "add") || strings.Contains(lower, "create") || strings.Contains(lower, "init") || strings.Contains(lower, "generate") || strings.Contains(lower, "release") || strings.Contains(lower, "deploy") || strings.Contains(lower, "publish") || strings.Contains(lower, "migrate") || strings.Contains(lower, "seed") || strings.Contains(lower, "sync")) {
		return true
	}
	for _, token := range []string{"init", "sync", "release", "deploy", "publish", "migrate", "seed"} {
		if lower == token || strings.HasPrefix(lower, token+"-") || strings.HasSuffix(lower, "-"+token) {
			return true
		}
	}
	return false
}

func shouldInferPackageScript(name string, script packageScript) bool {
	switch name {
	case "check", "test", "build", "lint", "dev", "start", "release", "deploy", "publish", "generate":
		return true
	}
	if strings.HasPrefix(name, "test:") || strings.HasPrefix(name, "lint:") {
		return true
	}
	if len(script.Params) > 0 {
		return true
	}
	return false
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
	if hasFile(repoRoot, "justfile") {
		paths = append(paths, "justfile")
	}
	if hasFile(repoRoot, "Justfile") {
		paths = append(paths, "Justfile")
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

func sortedParamNames(params map[string]config.Param) []string {
	names := make([]string, 0, len(params))
	for name := range params {
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

func sortedGroupNames(groups map[string]config.Group) []string {
	names := make([]string, 0, len(groups))
	for name := range groups {
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

func sortedScopeNames(scopes map[string]config.Scope) []string {
	names := make([]string, 0, len(scopes))
	for name := range scopes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func findMakeTargets(repoRoot string) []makeTarget {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "Makefile"))
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
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
	return justParam{Name: name, Default: defaultValue, Required: required}, true
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
	for _, param := range params {
		out[param.Name] = config.Param{
			Desc:     fmt.Sprintf("Value for the %s recipe parameter", param.Name),
			Env:      strings.ToUpper(strings.ReplaceAll(param.Name, "-", "_")),
			Required: param.Required,
			Default:  param.Default,
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

func sortedAliasNames(aliases map[string]string) []string {
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
	paramNames := sortedParamNames(params)
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

func hasFile(repoRoot, name string) bool {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() == name {
			return true
		}
	}
	return false
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
