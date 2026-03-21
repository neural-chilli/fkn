package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/neural-chilli/fkn/internal/brief"
)

func runAgentBrief(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("agent-brief", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	taskName := fs.String("task", "", "")
	diff := fs.Bool("diff", false, "")
	maxTokens := fs.Int("max-tokens", 0, "")
	var files multiFlag
	fs.Var(&files, "file", "")
	fs.Var(&files, "files", "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{
		"--json":       false,
		"--task":       true,
		"--diff":       false,
		"--max-tokens": true,
		"--file":       true,
		"--files":      true,
	})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}

	files = append(files, fs.Args()...)
	if *taskName != "" && (*diff || len(files) > 0) {
		printError(stderr, fmt.Errorf("--task cannot be combined with --diff or file arguments"))
		return 1
	}
	if *diff && len(files) > 0 {
		printError(stderr, fmt.Errorf("--diff cannot be combined with file arguments"))
		return 1
	}

	cfg, repoRoot, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	result, err := brief.New(cfg, repoRoot).Generate(brief.Options{
		Task:      *taskName,
		Files:     files,
		Diff:      *diff,
		MaxTokens: *maxTokens,
	})
	if err != nil {
		printError(stderr, err)
		return 1
	}
	if *jsonOut {
		return printJSON(stdout, result)
	}
	fmt.Fprintln(stdout, result.Markdown)
	return 0
}
