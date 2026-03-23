package main

import (
	stdcontext "context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/neural-chilli/fkn/internal/codemap"
	"github.com/neural-chilli/fkn/internal/config"
	"github.com/neural-chilli/fkn/internal/guard"
	"github.com/neural-chilli/fkn/internal/ordered"
	"github.com/neural-chilli/fkn/internal/repair"
	"github.com/neural-chilli/fkn/internal/runner"
	watchpkg "github.com/neural-chilli/fkn/internal/watch"
)

func runList(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, _, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	items := make([]listTask, 0, len(cfg.Tasks))
	for _, name := range ordered.Keys(cfg.Tasks) {
		task := cfg.Tasks[name]
		item := listTask{
			Name:    name,
			Desc:    task.Desc,
			Type:    task.Type(),
			Agent:   task.AgentEnabled(),
			Safety:  task.SafetyLevel(),
			Default: isDefaultTask(cfg, name),
		}
		item.Aliases = aliasesForTask(cfg.Aliases, name)
		item.Groups = config.GroupNamesForTask(cfg.Groups, name)
		if task.Scope != "" {
			item.Scope = &task.Scope
		}
		if len(task.Steps) > 0 {
			item.Parallel = task.Parallel
			item.Steps = task.Steps
		}
		if len(task.Needs) > 0 {
			item.Needs = append([]string(nil), task.Needs...)
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
					Position: param.Position,
					Variadic: param.Variadic,
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
		if len(cfg.Groups) > 0 {
			payload["groups"] = listGroups(cfg.Groups)
		}
		return printJSON(stdout, payload)
	}

	width := 0
	for _, item := range items {
		if len(item.Name) > width {
			width = len(item.Name)
		}
	}
	groupedTasks := map[string][]listTask{}
	seen := map[string]bool{}
	for _, item := range items {
		for _, groupName := range item.Groups {
			groupedTasks[groupName] = append(groupedTasks[groupName], item)
			seen[item.Name] = true
		}
	}

	if len(groupedTasks) > 0 {
		for _, groupName := range ordered.Keys(cfg.Groups) {
			group := cfg.Groups[groupName]
			fmt.Fprintf(stdout, "%s\n", groupName)
			if group.Desc != "" {
				fmt.Fprintf(stdout, "  %s\n", group.Desc)
			}
			for _, item := range groupedTasks[groupName] {
				fmt.Fprintf(stdout, "  %-*s  %s\n", width, item.Name, formatListSummary(item))
			}
			fmt.Fprintln(stdout)
		}
	}

	ungrouped := make([]listTask, 0, len(items))
	for _, item := range items {
		if !seen[item.Name] {
			ungrouped = append(ungrouped, item)
		}
	}
	if len(ungrouped) > 0 {
		if len(groupedTasks) > 0 {
			fmt.Fprintln(stdout, "ungrouped")
		}
		for _, item := range ungrouped {
			prefix := ""
			if len(groupedTasks) > 0 {
				prefix = "  "
			}
			fmt.Fprintf(stdout, "%s%-*s  %s\n", prefix, width, item.Name, formatListSummary(item))
		}
	}
	return 0
}

func runWatch(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	allowUnsafe := fs.Bool("allow-unsafe", false, "")
	var paths multiFlag
	fs.Var(&paths, "path", "")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--path": true, "--allow-unsafe": false})
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
			return runWatchTarget(target, *allowUnsafe, stdout, stderr)
		},
	})
	if err != nil {
		printError(stderr, err)
		return 1
	}
	return 0
}

func runWatchTarget(target string, allowUnsafe bool, stdout, stderr *os.File) error {
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
			AllowUnsafe: allowUnsafe,
			Stdout:      stdout,
			Stderr:      stderr,
		})
		if err != nil {
			return err
		}
		printGuardReport(stdout, report)
		return nil
	}

	result, err := runner.New(cfg, repoRoot).Run(target, runner.Options{
		AllowUnsafe: allowUnsafe,
		Stdout:      stdout,
		Stderr:      stderr,
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
	allowUnsafe := fs.Bool("allow-unsafe", false, "")
	if err := fs.Parse(taskArgs); err != nil {
		return 2
	}

	result, err := runner.New(cfg, repoRoot).Run(resolvedTaskName, runner.Options{
		JSON:        *jsonOut,
		DryRun:      *dryRun,
		AllowUnsafe: *allowUnsafe,
		Stdout:      stdout,
		Stderr:      stderr,
		Params:      params,
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

func runRepair(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("repair", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "Emit structured JSON")
	copyOut := fs.Bool("copy", false, "Copy rendered markdown to the clipboard")
	allowUnsafe := fs.Bool("allow-unsafe", false, "Allow destructive or external tasks inside the guard")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false, "--copy": false, "--allow-unsafe": false})
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
	out, err := repair.New(cfg, repoRoot, guard.New(cfg, repoRoot, taskRunner)).Generate(repair.Options{
		GuardName:   guardName,
		AllowUnsafe: *allowUnsafe,
	})
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

func runExplain(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "Emit structured JSON")
	parsedArgs, err := parseSubcommandArgs(args, map[string]bool{"--json": false})
	if err != nil {
		printError(stderr, err)
		return 2
	}
	if err := fs.Parse(parsedArgs); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		printError(stderr, fmt.Errorf("explain target is required"))
		return 1
	}

	cfg, _, err := loadConfig()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	result, err := codemap.Explain(cfg, fs.Arg(0))
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
