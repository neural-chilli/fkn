package main

import (
	stdcontext "context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"syscall"
	"time"

	fkndocs "github.com/neural-chilli/fkn"
	"github.com/neural-chilli/fkn/internal/config"
	contextpkg "github.com/neural-chilli/fkn/internal/context"
	"github.com/neural-chilli/fkn/internal/guard"
	"github.com/neural-chilli/fkn/internal/initcmd"
	"github.com/neural-chilli/fkn/internal/mcp"
	"github.com/neural-chilli/fkn/internal/prompt"
	"github.com/neural-chilli/fkn/internal/repair"
	"github.com/neural-chilli/fkn/internal/runner"
	"github.com/neural-chilli/fkn/internal/scope"
	watchpkg "github.com/neural-chilli/fkn/internal/watch"
)

var version = "dev"

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		cfg, _, err := loadConfig()
		if err == nil && cfg.Default != "" {
			return runTask([]string{cfg.Default}, stdout, stderr)
		}
		printUsage(stdout)
		return 0
	}

	if args[0] == "--version" {
		fmt.Fprintf(stdout, "fkn %s\n", resolvedVersion())
		return 0
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "fkn %s\n", resolvedVersion())
		return 0
	case "help":
		return runHelp(args[1:], stdout, stderr)
	case "docs":
		return runDocs(args[1:], stdout, stderr)
	case "guard":
		return runGuard(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "watch":
		return runWatch(args[1:], stdout, stderr)
	case "context":
		return runContext(args[1:], stdout, stderr)
	case "prompt":
		return runPrompt(args[1:], stdout, stderr)
	case "scope":
		return runScope(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "repair":
		return runRepair(args[1:], stdout, stderr)
	default:
		return runTask(args, stdout, stderr)
	}
}

func resolvedVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	var revision string
	var modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" {
		return version
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified == "true" {
		return revision + "-dirty"
	}
	return revision
}

func runHelp(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	cfg, _, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	name := args[0]
	if resolved, ok := cfg.ResolveTaskName(name); ok {
		printTaskHelp(stdout, name, resolved, cfg, cfg.Tasks[resolved], aliasesForTask(cfg.Aliases, resolved), isDefaultTask(cfg, resolved))
		return 0
	}
	if guardCfg, ok := cfg.Guards[name]; ok {
		printGuardHelp(stdout, name, guardCfg)
		return 0
	}

	printError(stderr, unknownTaskError(name, cfg))
	return 1
}

func runDocs(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("docs", flag.ContinueOnError)
	fs.SetOutput(stderr)
	listOnly := fs.Bool("list", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--list": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	if *listOnly {
		for _, name := range fkndocs.DocNames() {
			fmt.Fprintln(stdout, name)
		}
		return 0
	}

	name := "readme"
	if fs.NArg() > 0 {
		name = fs.Arg(0)
	}

	doc, err := fkndocs.Doc(name)
	if err != nil {
		printError(stderr, err)
		return 1
	}

	fmt.Fprintln(stdout, strings.TrimRight(doc, "\n"))
	return 0
}

func runInit(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fromRepo := fs.Bool("from-repo", false, "")
	agents := fs.Bool("agents", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--from-repo": false, "--agents": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	message, err := initcmd.Run(repoRoot, initcmd.Options{
		FromRepo: *fromRepo,
		Agents:   *agents,
	})
	if err != nil {
		printError(stderr, err)
		return 1
	}

	fmt.Fprintln(stdout, message)
	return 0
}

func runGuard(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("guard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
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
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false, "--format": true})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
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

func runValidate(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		if *jsonOut {
			_ = printJSON(stdout, map[string]any{
				"valid": false,
				"error": err.Error(),
			})
		}
		return 1
	}

	result := map[string]any{
		"valid":     true,
		"project":   cfg.Project,
		"repo_root": repoRoot,
	}
	if *jsonOut {
		return printJSON(stdout, result)
	}

	fmt.Fprintln(stdout, "fkn.yaml is valid")
	return 0
}

func runPrompt(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("prompt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	copyOut := fs.Bool("copy", false, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--copy": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
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
	agent := fs.Bool("agent", false, "Generate agent-focused context")
	jsonOut := fs.Bool("json", false, "Emit structured JSON")
	taskName := fs.String("task", "", "Task name required with --agent")
	outPath := fs.String("out", "", "Write rendered markdown to a file")
	copyOut := fs.Bool("copy", false, "Copy rendered markdown to the clipboard")
	maxTokens := fs.Int("max-tokens", 0, "Approximate token budget; uses a rough estimate rather than a model tokenizer")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--agent": false, "--json": false, "--task": true, "--out": true, "--copy": false, "--max-tokens": true})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	generator := contextpkg.New(cfg, repoRoot)
	options := contextpkg.Options{
		Agent:     *agent,
		Task:      *taskName,
		MaxTokens: *maxTokens,
	}
	if *jsonOut {
		result, err := generator.GenerateJSON(options)
		if err != nil {
			printError(stderr, err)
			return 1
		}
		return printJSON(stdout, result)
	}

	rendered, err := generator.Generate(options)
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
		tools := mcp.New(cfg, "", nil).Tools()
		return printJSON(stdout, map[string]any{"tools": tools})
	}

	items := make([]listTask, 0, len(cfg.Tasks))
	for _, name := range sortedTaskNames(cfg.Tasks) {
		task := cfg.Tasks[name]
		item := listTask{
			Name:    name,
			Desc:    task.Desc,
			Type:    task.Type(),
			Agent:   task.AgentEnabled(),
			Default: isDefaultTask(cfg, name),
		}
		item.Aliases = aliasesForTask(cfg.Aliases, name)
		if task.Scope != "" {
			item.Scope = &task.Scope
		}
		if len(task.Steps) > 0 {
			item.Parallel = task.Parallel
			item.Steps = task.Steps
		}
		if len(task.Params) > 0 {
			item.Params = make(map[string]listParam, len(task.Params))
			for _, paramName := range sortedParamNames(task.Params) {
				param := task.Params[paramName]
				item.Params[paramName] = listParam{
					Desc:     param.Desc,
					Env:      param.Env,
					Required: param.Required,
					Default:  param.Default,
				}
			}
		}
		items = append(items, item)
	}

	if *jsonOut {
		payload := map[string]any{"tasks": items}
		if cfg.Default != "" {
			payload["default"] = cfg.Default
		}
		return printJSON(stdout, payload)
	}

	width := 0
	for _, item := range items {
		if len(item.Name) > width {
			width = len(item.Name)
		}
	}
	for _, item := range items {
		fmt.Fprintf(stdout, "%-*s  %s\n", width, item.Name, formatListSummary(item))
	}
	return 0
}

func runServe(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	httpMode := fs.Bool("http", false, "")
	port := fs.Int("port", 0, "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--http": false, "--port": true})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	serveHTTP := *httpMode || strings.EqualFold(cfg.Serve.Transport, "http")
	servePort := cfg.Serve.Port
	if *port != 0 {
		servePort = *port
	}

	ctx, stop := signal.NotifyContext(stdcontext.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	taskRunner := runner.New(cfg, repoRoot)
	server := mcp.New(cfg, repoRoot, taskRunner)
	if serveHTTP {
		if err := server.ServeHTTP(ctx, servePort, stderr); err != nil {
			printError(stderr, err)
			return 1
		}
		return 0
	}

	if err := server.ServeStdio(ctx, os.Stdin, stdout, stderr); err != nil && !errors.Is(err, stdcontext.Canceled) {
		printError(stderr, err)
		return 1
	}
	return 0
}

func runWatch(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var paths multiFlag
	fs.Var(&paths, "path", "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--path": true})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		printError(stderr, fmt.Errorf("watch target is required"))
		return 1
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	cfg, _, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	watchPaths := paths
	if len(watchPaths) == 0 {
		watchPaths = append(watchPaths, cfg.Watch.Paths...)
	}
	if len(watchPaths) == 0 {
		watchPaths = []string{"."}
	}

	target := fs.Arg(0)
	ctx, stop := signal.NotifyContext(stdcontext.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	watcher := watchpkg.New(repoRoot)
	err = watcher.Run(ctx, watchpkg.Options{
		Paths:    watchPaths,
		Debounce: time.Duration(cfg.Watch.DebounceMS) * time.Millisecond,
		OnTrigger: func(triggeredAt time.Time) error {
			fmt.Fprintf(stdout, "\n[fkn watch %s]\n\n", triggeredAt.UTC().Format(time.RFC3339))
			return runWatchTarget(target, stdout, stderr)
		},
	})
	if err != nil {
		printError(stderr, err)
		return 1
	}
	return 0
}

func runWatchTarget(target string, stdout, stderr *os.File) error {
	cfg, repoRoot, err := loadConfig()
	if err != nil {
		return err
	}

	if target == "guard" || strings.HasPrefix(target, "guard:") {
		guardName := ""
		if strings.HasPrefix(target, "guard:") {
			guardName = strings.TrimPrefix(target, "guard:")
		}
		taskRunner := runner.New(cfg, repoRoot)
		report, err := guard.New(cfg, repoRoot, taskRunner).Run(guardName, runner.Options{
			Stdout: stdout,
			Stderr: stderr,
		})
		if err != nil {
			return err
		}
		printGuardReport(stdout, report)
		return nil
	}

	result, err := runner.New(cfg, repoRoot).Run(target, runner.Options{
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		fmt.Fprintf(stderr, "\nwatch target exited with code %d\n", result.ExitCode)
	}
	return nil
}

func runTask(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	taskName := args[0]
	resolvedTaskName, ok := cfg.ResolveTaskName(taskName)
	if !ok {
		printError(stderr, unknownTaskError(taskName, cfg))
		return 1
	}
	task := cfg.Tasks[resolvedTaskName]

	taskArgs, params, err := parseTaskInvocation(args[1:], task)
	if err != nil {
		printError(stderr, err)
		return 2
	}

	fs := flag.NewFlagSet("task", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	dryRun := fs.Bool("dry-run", false, "")
	if err := fs.Parse(taskArgs); err != nil {
		return 2
	}

	result, err := runner.New(cfg, repoRoot).Run(resolvedTaskName, runner.Options{
		JSON:   *jsonOut,
		DryRun: *dryRun,
		Stdout: stdout,
		Stderr: stderr,
		Params: params,
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
		"fkn [<task>] [--name value] [--param name=value] [--dry-run] [--json]",
		"If fkn.yaml sets `default`, running `fkn` with no task runs that task.",
		"fkn docs [name] [--list]",
		"fkn help [task]",
		"fkn context [--agent] [--json] [--task <name>] [--out <file>] [--copy] [--max-tokens <approx-n>]",
		"fkn guard [name] [--json]",
		"fkn repair [name] [--json] [--copy]",
		"fkn init [--from-repo] [--agents]",
		"fkn list [--json] [--mcp]",
		"fkn serve [--http] [--port <n>]",
		"fkn watch <target> [--path <glob>]",
		"fkn prompt <name> [--copy]",
		"fkn scope <name> [--json] [--format prompt]",
		"fkn validate [--json]",
		"fkn version",
	}
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
}

func printTaskHelp(stdout *os.File, invokedName, resolvedName string, cfg *config.Config, task config.Task, aliases []string, isDefault bool) {
	fmt.Fprintf(stdout, "%s\n\n", invokedName)
	fmt.Fprintf(stdout, "Description: %s\n", task.Desc)
	if invokedName != resolvedName {
		fmt.Fprintf(stdout, "Alias For: %s\n", resolvedName)
	}
	if isDefault {
		fmt.Fprintln(stdout, "Default: true")
	}
	if len(aliases) > 0 {
		fmt.Fprintf(stdout, "Aliases: %s\n", strings.Join(aliases, ", "))
	}
	fmt.Fprintf(stdout, "Usage: %s\n", taskUsage(invokedName, task))
	fmt.Fprintf(stdout, "Type: %s\n", task.Type())
	if task.Scope != "" {
		fmt.Fprintf(stdout, "Scope: %s\n", task.Scope)
	}
	fmt.Fprintf(stdout, "Agent: %t\n", task.AgentEnabled())
	if task.Dir != "" {
		fmt.Fprintf(stdout, "Dir: %s\n", task.Dir)
	} else if cfg.Defaults.Dir != "" {
		fmt.Fprintf(stdout, "Dir: %s (inherited default)\n", cfg.Defaults.Dir)
	}
	if task.Shell != "" {
		fmt.Fprintf(stdout, "Shell: %s\n", task.Shell)
	}
	if len(task.ShellArgs) > 0 {
		fmt.Fprintf(stdout, "Shell Args: %s\n", strings.Join(task.ShellArgs, " "))
	}
	if task.Timeout != "" {
		fmt.Fprintf(stdout, "Timeout: %s\n", task.Timeout)
	}
	if len(task.Params) > 0 {
		fmt.Fprintln(stdout, "Params:")
		for _, paramName := range sortedParamNames(task.Params) {
			param := task.Params[paramName]
			fmt.Fprintf(stdout, "- %s", paramName)
			if param.Env != "" {
				fmt.Fprintf(stdout, " (env: %s)", param.Env)
			}
			if param.Required {
				fmt.Fprint(stdout, " required")
			}
			if param.Default != "" {
				fmt.Fprintf(stdout, " default=%q", param.Default)
			}
			if param.Desc != "" {
				fmt.Fprintf(stdout, " - %s", param.Desc)
			}
			fmt.Fprintln(stdout)
		}
	}
	if task.Cmd != "" {
		fmt.Fprintf(stdout, "Command: %s\n", task.Cmd)
		return
	}
	fmt.Fprintf(stdout, "Parallel: %t\n", task.Parallel)
	fmt.Fprintln(stdout, "Steps:")
	for _, step := range task.Steps {
		fmt.Fprintf(stdout, "- %s\n", step)
	}
}

func parseTaskInvocation(args []string, task config.Task) ([]string, map[string]string, error) {
	var taskArgs []string
	params := map[string]string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--param" {
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--param requires name=value")
			}
			i++
			parts := strings.SplitN(args[i], "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return nil, nil, fmt.Errorf("--param requires name=value")
			}
			params[strings.TrimSpace(parts[0])] = parts[1]
			continue
		}
		name, value, ok, err := parseDirectParam(arg, args, i, task)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			params[name] = value
			if !strings.Contains(arg, "=") {
				i++
			}
			continue
		}
		taskArgs = append(taskArgs, arg)
	}

	return taskArgs, params, nil
}

func parseDirectParam(arg string, args []string, index int, task config.Task) (string, string, bool, error) {
	if !strings.HasPrefix(arg, "--") || arg == "--" {
		return "", "", false, nil
	}

	nameValue := strings.TrimPrefix(arg, "--")
	if nameValue == "json" || nameValue == "dry-run" {
		return "", "", false, nil
	}

	if strings.Contains(nameValue, "=") {
		parts := strings.SplitN(nameValue, "=", 2)
		if _, ok := task.Params[parts[0]]; ok {
			return parts[0], parts[1], true, nil
		}
		return "", "", false, nil
	}

	if _, ok := task.Params[nameValue]; !ok {
		return "", "", false, nil
	}
	if index+1 >= len(args) {
		return "", "", false, fmt.Errorf("missing value for --%s", nameValue)
	}
	return nameValue, args[index+1], true, nil
}

func sortedParamNames(params map[string]config.Param) []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func printGuardHelp(stdout *os.File, name string, guardCfg config.Guard) {
	fmt.Fprintf(stdout, "guard %s\n\n", name)
	fmt.Fprintln(stdout, "Steps:")
	for _, step := range guardCfg.Steps {
		fmt.Fprintf(stdout, "- %s\n", step)
	}
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

func parseSubcommandArgs(args []string, flags map[string]bool) ([]string, error) {
	var flagArgs []string
	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		name := arg
		value := ""
		hasInlineValue := false
		if strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			name = parts[0]
			value = parts[1]
			hasInlineValue = true
		}

		takesValue, ok := flags[name]
		if !ok {
			return nil, fmt.Errorf("unknown flag %q", name)
		}

		flagArgs = append(flagArgs, name)
		if takesValue {
			if hasInlineValue {
				flagArgs = append(flagArgs, value)
				continue
			}
			if i+1 >= len(args) {
				return nil, fmt.Errorf("missing value for %s", name)
			}
			i++
			flagArgs = append(flagArgs, args[i])
			continue
		}
		if hasInlineValue {
			return nil, fmt.Errorf("flag %s does not take a value", name)
		}
	}

	return append(flagArgs, positionals...), nil
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
	Name     string               `json:"name"`
	Desc     string               `json:"desc"`
	Type     string               `json:"type"`
	Parallel bool                 `json:"parallel,omitempty"`
	Steps    []string             `json:"steps,omitempty"`
	Scope    *string              `json:"scope"`
	Agent    bool                 `json:"agent"`
	Default  bool                 `json:"default,omitempty"`
	Aliases  []string             `json:"aliases,omitempty"`
	Params   map[string]listParam `json:"params,omitempty"`
}

type listParam struct {
	Desc     string `json:"desc,omitempty"`
	Env      string `json:"env"`
	Required bool   `json:"required,omitempty"`
	Default  string `json:"default,omitempty"`
}

func aliasesForTask(aliases map[string]string, taskName string) []string {
	names := []string{}
	for alias, target := range aliases {
		if target == taskName {
			names = append(names, alias)
		}
	}
	sort.Strings(names)
	return names
}

func isDefaultTask(cfg *config.Config, taskName string) bool {
	if cfg.Default == "" {
		return false
	}
	resolved, ok := cfg.ResolveTaskName(cfg.Default)
	return ok && resolved == taskName
}

func taskUsage(name string, task config.Task) string {
	parts := []string{"fkn", name}
	for _, paramName := range sortedParamNames(task.Params) {
		flag := "--" + paramName + " <value>"
		if task.Params[paramName].Required {
			parts = append(parts, flag)
			continue
		}
		parts = append(parts, "["+flag+"]")
	}
	parts = append(parts, "[--dry-run]", "[--json]")
	return strings.Join(parts, " ")
}

func formatListSummary(item listTask) string {
	parts := []string{item.Desc}
	meta := []string{item.Type}
	if item.Default {
		meta = append(meta, "default")
	}
	if item.Scope != nil && *item.Scope != "" {
		meta = append(meta, "scope:"+*item.Scope)
	}
	if len(item.Aliases) > 0 {
		meta = append(meta, "aliases:"+strings.Join(item.Aliases, ","))
	}
	if len(item.Params) > 0 {
		params := make([]string, 0, len(item.Params))
		for _, name := range sortedListParamNames(item.Params) {
			param := item.Params[name]
			label := "--" + name
			if !param.Required {
				label += "?"
			}
			params = append(params, label)
		}
		meta = append(meta, "params:"+strings.Join(params, ","))
	}
	if !item.Agent {
		meta = append(meta, "agent:false")
	}
	return strings.Join([]string{parts[0], "[" + strings.Join(meta, " | ") + "]"}, " ")
}

func sortedListParamNames(params map[string]listParam) []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func unknownTaskError(name string, cfg *config.Config) error {
	suggestions := nearestTaskNames(name, cfg)
	if len(suggestions) == 0 {
		return fmt.Errorf("unknown task %q", name)
	}
	return fmt.Errorf("unknown task %q. Did you mean: %s?", name, strings.Join(suggestions, ", "))
}

func nearestTaskNames(name string, cfg *config.Config) []string {
	type candidate struct {
		name  string
		score int
	}

	candidates := []candidate{}
	for taskName := range cfg.Tasks {
		score := levenshtein(name, taskName)
		if strings.Contains(taskName, name) || strings.Contains(name, taskName) {
			score--
		}
		candidates = append(candidates, candidate{name: taskName, score: score})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].score < candidates[j].score
	})

	limit := 3
	if len(candidates) < limit {
		limit = len(candidates)
	}
	out := []string{}
	for i := 0; i < limit; i++ {
		if candidates[i].score > max(3, len(name)/2+1) {
			continue
		}
		out = append(out, candidates[i].name)
	}
	return out
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(current[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev = current
	}
	return prev[len(b)]
}

func runRepair(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("repair", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "Emit structured JSON")
	copyOut := fs.Bool("copy", false, "Copy rendered markdown to the clipboard")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false, "--copy": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	guardName := ""
	if fs.NArg() > 0 {
		guardName = fs.Arg(0)
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	taskRunner := runner.New(cfg, repoRoot)
	out, err := repair.New(cfg, repoRoot, guard.New(cfg, repoRoot, taskRunner)).Generate(repair.Options{GuardName: guardName})
	if err != nil {
		printError(stderr, err)
		return 1
	}

	if *jsonOut {
		if code := printJSON(stdout, out); code != 0 {
			return code
		}
		return out.ExitCode
	}

	fmt.Fprintln(stdout, out.Markdown)
	if *copyOut {
		if err := copyToClipboard(out.Markdown); err != nil {
			printError(stderr, err)
			return 1
		}
	}
	return out.ExitCode
}
