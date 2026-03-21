package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	fkndocs "github.com/neural-chilli/fkn"
	contextpkg "github.com/neural-chilli/fkn/internal/context"
	"github.com/neural-chilli/fkn/internal/guard"
	"github.com/neural-chilli/fkn/internal/initcmd"
	"github.com/neural-chilli/fkn/internal/prompt"
	"github.com/neural-chilli/fkn/internal/runner"
	"github.com/neural-chilli/fkn/internal/scope"
)

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
	if group, ok := cfg.Groups[name]; ok {
		printGroupHelp(stdout, name, group, cfg)
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
		fmt.Fprintln(stdout, scope.FormatPrompt(result.Scope, result.Desc, result.Paths))
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
	about := fs.String("about", "", "Generate topic-focused context using tasks, scopes, and codemap matches")
	outPath := fs.String("out", "", "Write rendered markdown to a file")
	copyOut := fs.Bool("copy", false, "Copy rendered markdown to the clipboard")
	maxTokens := fs.Int("max-tokens", 0, "Approximate token budget; uses a rough estimate rather than a model tokenizer")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--agent": false, "--json": false, "--task": true, "--about": true, "--out": true, "--copy": false, "--max-tokens": true})
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
		About:     *about,
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
