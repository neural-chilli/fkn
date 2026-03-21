package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fkn/internal/config"
	"fkn/internal/runner"
)

var version = "dev"

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	if args[0] == "--version" {
		fmt.Fprintf(stdout, "fkn %s\n", version)
		return 0
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "fkn %s\n", version)
		return 0
	case "help":
		printUsage(stdout)
		return 0
	case "list":
		return runList(args[1:], stdout, stderr)
	default:
		return runTask(args, stdout, stderr)
	}
}

func runList(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	mcpOut := fs.Bool("mcp", false, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, _, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	if *mcpOut {
		tools := make([]mcpTool, 0, len(cfg.Tasks))
		for _, name := range sortedTaskNames(cfg.Tasks) {
			task := cfg.Tasks[name]
			if !task.AgentEnabled() {
				continue
			}
			tools = append(tools, mcpTool{
				Name:        name,
				Description: task.Desc,
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"env": map[string]any{
							"type":        "object",
							"description": "Optional env var overrides for this invocation",
						},
						"dry_run": map[string]any{
							"type":        "boolean",
							"description": "Print the command without executing it",
						},
					},
				},
			})
		}

		return printJSON(stdout, map[string]any{"tools": tools})
	}

	items := make([]listTask, 0, len(cfg.Tasks))
	for _, name := range sortedTaskNames(cfg.Tasks) {
		task := cfg.Tasks[name]
		item := listTask{
			Name:  name,
			Desc:  task.Desc,
			Type:  task.Type(),
			Agent: task.AgentEnabled(),
		}
		if task.Scope != "" {
			item.Scope = &task.Scope
		}
		if len(task.Steps) > 0 {
			item.Parallel = task.Parallel
			item.Steps = task.Steps
		}
		items = append(items, item)
	}

	if *jsonOut {
		return printJSON(stdout, map[string]any{"tasks": items})
	}

	width := 0
	for _, item := range items {
		if len(item.Name) > width {
			width = len(item.Name)
		}
	}
	for _, item := range items {
		fmt.Fprintf(stdout, "%-*s  %s\n", width, item.Name, item.Desc)
	}
	return 0
}

func runTask(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("task", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	dryRun := fs.Bool("dry-run", false, "")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	result, err := runner.New(cfg, repoRoot).Run(args[0], runner.Options{
		JSON:   *jsonOut,
		DryRun: *dryRun,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		printError(stderr, err)
		return 1
	}

	if *jsonOut {
		return printJSON(stdout, result)
	}
	return result.ExitCode
}

func loadConfig() (*config.Config, string, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	cfgPath := filepath.Join(repoRoot, "fkn.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", fmt.Errorf("missing fkn.yaml in %s; run `fkn init` to scaffold one", repoRoot)
		}
		return nil, "", err
	}
	return cfg, repoRoot, nil
}

func sortedTaskNames(tasks map[string]config.Task) []string {
	names := make([]string, 0, len(tasks))
	for name := range tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func printUsage(stdout *os.File) {
	lines := []string{
		"fkn <task> [--dry-run] [--json]",
		"fkn list [--json] [--mcp]",
		"fkn version",
	}
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
}

func printError(stderr *os.File, err error) {
	fmt.Fprintf(stderr, "error: %v\n", err)
}

func printJSON(stdout *os.File, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return 1
	}
	return 0
}

type listTask struct {
	Name     string   `json:"name"`
	Desc     string   `json:"desc"`
	Type     string   `json:"type"`
	Parallel bool     `json:"parallel,omitempty"`
	Steps    []string `json:"steps,omitempty"`
	Scope    *string  `json:"scope"`
	Agent    bool     `json:"agent"`
}

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}
