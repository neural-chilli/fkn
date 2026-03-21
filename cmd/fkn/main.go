package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"fkn/internal/config"
	contextpkg "fkn/internal/context"
	"fkn/internal/guard"
	"fkn/internal/prompt"
	"fkn/internal/runner"
	"fkn/internal/scope"
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
	case "guard":
		return runGuard(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "context":
		return runContext(args[1:], stdout, stderr)
	case "prompt":
		return runPrompt(args[1:], stdout, stderr)
	case "scope":
		return runScope(args[1:], stdout, stderr)
	default:
		return runTask(args, stdout, stderr)
	}
}

func runGuard(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("guard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	if err := fs.Parse(reorderSubcommandArgs(args, map[string]bool{"--json": true})); err != nil {
		return 2
	}

	var name string
	if fs.NArg() > 0 {
		name = fs.Arg(0)
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	taskRunner := runner.New(cfg, repoRoot)
	report, err := guard.New(cfg, repoRoot, taskRunner).Run(name, runner.Options{
		JSON:   *jsonOut,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		printError(stderr, err)
		return 1
	}

	if *jsonOut {
		return printJSON(stdout, report)
	}

	printGuardReport(stdout, report)
	return report.ExitCode
}

func runScope(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("scope", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	format := fs.String("format", "", "")
	if err := fs.Parse(reorderSubcommandArgs(args, map[string]bool{"--json": true, "--format": false})); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		printError(stderr, fmt.Errorf("scope name is required"))
		return 1
	}

	cfg, _, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	result, err := scope.Get(cfg, fs.Arg(0))
	if err != nil {
		printError(stderr, err)
		return 1
	}

	if *jsonOut {
		return printJSON(stdout, result)
	}

	if *format == "prompt" {
		fmt.Fprintln(stdout, scope.FormatPrompt(result.Scope, result.Paths))
		return 0
	}
	if *format != "" {
		printError(stderr, fmt.Errorf("unknown scope format %q", *format))
		return 1
	}

	for _, path := range result.Paths {
		fmt.Fprintln(stdout, path)
	}
	return 0
}

func runPrompt(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("prompt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	copyOut := fs.Bool("copy", false, "")
	if err := fs.Parse(reorderSubcommandArgs(args, map[string]bool{"--copy": true})); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		printError(stderr, fmt.Errorf("prompt name is required"))
		return 1
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	rendered, warnings, err := prompt.New(cfg, repoRoot).Render(fs.Arg(0))
	if err != nil {
		printError(stderr, err)
		return 1
	}
	for _, warning := range warnings {
		fmt.Fprintf(stderr, "warning: %s\n", warning)
	}

	fmt.Fprintln(stdout, rendered)

	if *copyOut {
		if err := copyToClipboard(rendered); err != nil {
			printError(stderr, err)
			return 1
		}
	}

	return 0
}

func runContext(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("context", flag.ContinueOnError)
	fs.SetOutput(stderr)
	agent := fs.Bool("agent", false, "")
	taskName := fs.String("task", "", "")
	outPath := fs.String("out", "", "")
	copyOut := fs.Bool("copy", false, "")
	maxTokens := fs.Int("max-tokens", 0, "")
	if err := fs.Parse(reorderSubcommandArgs(args, map[string]bool{"--agent": true, "--task": false, "--out": false, "--copy": true, "--max-tokens": false})); err != nil {
		return 2
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	rendered, err := contextpkg.New(cfg, repoRoot).Generate(contextpkg.Options{
		Agent:     *agent,
		Task:      *taskName,
		MaxTokens: *maxTokens,
	})
	if err != nil {
		printError(stderr, err)
		return 1
	}

	if *outPath != "" {
		if err := os.WriteFile(*outPath, []byte(rendered+"\n"), 0o644); err != nil {
			printError(stderr, err)
			return 1
		}
	}

	fmt.Fprintln(stdout, rendered)

	if *copyOut {
		if err := copyToClipboard(rendered); err != nil {
			printError(stderr, err)
			return 1
		}
	}

	return 0
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
		"fkn context [--agent] [--task <name>] [--out <file>] [--copy] [--max-tokens <n>]",
		"fkn guard [name] [--json]",
		"fkn list [--json] [--mcp]",
		"fkn prompt <name> [--copy]",
		"fkn scope <name> [--json] [--format prompt]",
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

func reorderSubcommandArgs(args []string, flags map[string]bool) []string {
	var flagArgs []string
	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		if hasValue, ok := flags[arg]; ok {
			flagArgs = append(flagArgs, arg)
			if !hasValue && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}

		flagArgs = append(flagArgs, arg)
	}

	return append(flagArgs, positionals...)
}

func copyToClipboard(value string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("clipboard copy is unavailable on this system")
		}
	}

	cmd.Stdin = strings.NewReader(value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("copy to clipboard failed: %w", err)
	}
	return nil
}

func printGuardReport(stdout *os.File, report guard.Report) {
	fmt.Fprintf(stdout, "[fkn guard: %s]\n\n", report.Guard)
	width := 0
	for _, step := range report.Steps {
		if len(step.Name) > width {
			width = len(step.Name)
		}
	}
	for _, step := range report.Steps {
		fmt.Fprintf(stdout, "  %-*s  %s  %.1fs\n", width, step.Name, strings.ToUpper(step.Status), float64(step.DurationMS)/1000)
	}

	fmt.Fprintln(stdout)
	if report.Overall == runner.StatusPass {
		fmt.Fprintf(stdout, "PASSED in %.1fs\n", float64(report.DurationMS)/1000)
	} else {
		fmt.Fprintf(stdout, "FAILED in %.1fs\n", float64(report.DurationMS)/1000)
	}

	for _, step := range report.Steps {
		if step.Stderr == "" {
			continue
		}
		fmt.Fprintf(stdout, "\n--- %s stderr ---\n%s", step.Name, step.Stderr)
		if !strings.HasSuffix(step.Stderr, "\n") {
			fmt.Fprintln(stdout)
		}
	}
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
