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
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/neural-chilli/fkn/internal/config"
	contextpkg "github.com/neural-chilli/fkn/internal/context"
	"github.com/neural-chilli/fkn/internal/guard"
	"github.com/neural-chilli/fkn/internal/initcmd"
	"github.com/neural-chilli/fkn/internal/mcp"
	"github.com/neural-chilli/fkn/internal/prompt"
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
		return runHelp(args[1:], stdout, stderr)
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
	default:
		return runTask(args, stdout, stderr)
	}
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
	if task, ok := cfg.Tasks[name]; ok {
		printTaskHelp(stdout, name, task)
		return 0
	}
	if guardCfg, ok := cfg.Guards[name]; ok {
		printGuardHelp(stdout, name, guardCfg)
		return 0
	}

	printError(stderr, unknownTaskError(name, cfg))
	return 1
}

func runInit(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fromRepo := fs.Bool("from-repo", false, "")
	if err := fs.Parse(reorderSubcommandArgs(args, map[string]bool{"--from-repo": true})); err != nil {
		return 2
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		printError(stderr, err)
		return 1
	}

	message, err := initcmd.Run(repoRoot, initcmd.Options{
		FromRepo: *fromRepo,
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
		tools := mcp.New(cfg, "", nil).Tools()
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

func runServe(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	httpMode := fs.Bool("http", false, "")
	port := fs.Int("port", 0, "")
	if err := fs.Parse(reorderSubcommandArgs(args, map[string]bool{"--http": true, "--port": false})); err != nil {
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
	if err := fs.Parse(reorderSubcommandArgs(args, map[string]bool{"--path": false})); err != nil {
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

	if _, ok := cfg.Tasks[args[0]]; !ok {
		printError(stderr, unknownTaskError(args[0], cfg))
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
		"fkn help [task]",
		"fkn context [--agent] [--task <name>] [--out <file>] [--copy] [--max-tokens <n>]",
		"fkn guard [name] [--json]",
		"fkn init [--from-repo]",
		"fkn list [--json] [--mcp]",
		"fkn serve [--http] [--port <n>]",
		"fkn watch <target> [--path <glob>]",
		"fkn prompt <name> [--copy]",
		"fkn scope <name> [--json] [--format prompt]",
		"fkn version",
	}
	fmt.Fprintln(stdout, strings.Join(lines, "\n"))
}

func printTaskHelp(stdout *os.File, name string, task config.Task) {
	fmt.Fprintf(stdout, "%s\n\n", name)
	fmt.Fprintf(stdout, "Description: %s\n", task.Desc)
	fmt.Fprintf(stdout, "Type: %s\n", task.Type())
	if task.Scope != "" {
		fmt.Fprintf(stdout, "Scope: %s\n", task.Scope)
	}
	fmt.Fprintf(stdout, "Agent: %t\n", task.AgentEnabled())
	if task.Dir != "" {
		fmt.Fprintf(stdout, "Dir: %s\n", task.Dir)
	}
	if task.Timeout != "" {
		fmt.Fprintf(stdout, "Timeout: %s\n", task.Timeout)
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
			current[j] = min3(
				current[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
		}
		prev = current
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
