package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neural-chilli/fkn/internal/config"
)

const (
	StatusPass      = "pass"
	StatusFail      = "fail"
	StatusCancelled = "cancelled"
	StatusTimeout   = "timeout"
)

type Options struct {
	JSON   bool
	DryRun bool
	Stdout io.Writer
	Stderr io.Writer
	Env    map[string]string
	Params map[string]string
}

type Runner struct {
	cfg       *config.Config
	repoRoot  string
	globalEnv map[string]string
}

type Result struct {
	Task        string       `json:"task"`
	Type        string       `json:"type"`
	ResolvedCmd *string      `json:"resolved_cmd,omitempty"`
	Parallel    bool         `json:"parallel,omitempty"`
	Status      string       `json:"status"`
	ExitCode    int          `json:"exit_code"`
	Stdout      string       `json:"stdout,omitempty"`
	Stderr      string       `json:"stderr,omitempty"`
	Errors      []ErrorEntry `json:"errors,omitempty"`
	DurationMS  int64        `json:"duration_ms"`
	StartedAt   string       `json:"started_at"`
	FinishedAt  string       `json:"finished_at"`
	Steps       []StepResult `json:"steps,omitempty"`
}

type StepResult struct {
	Index       int          `json:"index"`
	Name        string       `json:"name"`
	ResolvedCmd *string      `json:"resolved_cmd"`
	Status      string       `json:"status"`
	ExitCode    int          `json:"exit_code"`
	Stdout      *string      `json:"stdout"`
	Stderr      *string      `json:"stderr"`
	Errors      []ErrorEntry `json:"errors,omitempty"`
	DurationMS  *int64       `json:"duration_ms"`
	StartedAt   *string      `json:"started_at"`
	FinishedAt  *string      `json:"finished_at"`
}

type ErrorEntry struct {
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"`
}

type runOutcome struct {
	status   string
	exitCode int
	stdout   string
	stderr   string
	started  time.Time
	finished time.Time
}

func New(cfg *config.Config, repoRoot string) *Runner {
	return &Runner{
		cfg:       cfg,
		repoRoot:  repoRoot,
		globalEnv: loadEnvFile(filepath.Join(repoRoot, cfg.EnvFile)),
	}
}

func (r *Runner) Run(taskName string, opts Options) (Result, error) {
	task, ok := r.cfg.Tasks[taskName]
	if !ok {
		return Result{}, fmt.Errorf("unknown task %q", taskName)
	}

	if task.Cmd != "" {
		stepName := taskName
		paramValues, err := resolveParamValues(task, opts.Params)
		if err != nil {
			return Result{}, fmt.Errorf("task %q: %w", taskName, err)
		}
		resolved := interpolateParams(task.Cmd, paramValues)
		outcome, err := r.runCommand(context.Background(), stepName, task, resolved, opts, "")
		if err != nil {
			return Result{}, err
		}
		return Result{
			Task:        taskName,
			Type:        "cmd",
			ResolvedCmd: strPtr(resolved),
			Status:      outcome.status,
			ExitCode:    outcome.exitCode,
			Stdout:      outcome.stdout,
			Stderr:      outcome.stderr,
			Errors:      extractErrors(task.ErrorFormat, outcome.stderr),
			DurationMS:  outcome.finished.Sub(outcome.started).Milliseconds(),
			StartedAt:   outcome.started.UTC().Format(time.RFC3339),
			FinishedAt:  outcome.finished.UTC().Format(time.RFC3339),
		}, nil
	}

	if task.Parallel {
		return r.runParallel(taskName, task, opts)
	}
	return r.runSequential(taskName, task, opts)
}

func (r *Runner) RunGuardStep(stepName string, opts Options) (StepResult, error) {
	result, err := r.Run(stepName, opts)
	if err != nil {
		return StepResult{}, err
	}
	stderr := result.Stderr
	if stderr == "" && len(result.Steps) > 0 {
		var parts []string
		for _, step := range result.Steps {
			if step.Stderr != nil && *step.Stderr != "" {
				parts = append(parts, fmt.Sprintf("[%s]\n%s", step.Name, strings.TrimRight(*step.Stderr, "\n")))
			}
		}
		if len(parts) > 0 {
			stderr = strings.Join(parts, "\n")
		}
	}

	duration := result.DurationMS
	started := result.StartedAt
	finished := result.FinishedAt
	return StepResult{
		Index:       0,
		Name:        stepName,
		ResolvedCmd: result.ResolvedCmd,
		Status:      result.Status,
		ExitCode:    result.ExitCode,
		Stderr:      strPtr(stderr),
		Errors:      collectResultErrors(result),
		DurationMS:  &duration,
		StartedAt:   &started,
		FinishedAt:  &finished,
	}, nil
}

func (r *Runner) runSequential(taskName string, task config.Task, opts Options) (Result, error) {
	started := time.Now()
	steps := make([]StepResult, 0, len(task.Steps))
	overallStatus := StatusPass
	overallCode := 0

	for i, step := range task.Steps {
		stepRes, err := r.runStep(context.Background(), i, taskName, step, opts)
		if err != nil {
			return Result{}, err
		}
		steps = append(steps, stepRes)
		if stepRes.Status != StatusPass {
			overallStatus = stepRes.Status
			overallCode = stepRes.ExitCode
			if !task.ContinueOnError {
				for j := i + 1; j < len(task.Steps); j++ {
					steps = append(steps, cancelledStep(j, task.Steps[j]))
				}
				break
			}
		}
	}

	finished := time.Now()
	return Result{
		Task:       taskName,
		Type:       "pipeline",
		Parallel:   false,
		Status:     overallStatus,
		ExitCode:   overallCode,
		DurationMS: finished.Sub(started).Milliseconds(),
		StartedAt:  started.UTC().Format(time.RFC3339),
		FinishedAt: finished.UTC().Format(time.RFC3339),
		Steps:      steps,
	}, nil
}

func (r *Runner) runParallel(taskName string, task config.Task, opts Options) (Result, error) {
	started := time.Now()
	if task.ContinueOnError && !opts.JSON {
		fmt.Fprintln(opts.Stderr, "warning: continue_on_error is ignored for parallel tasks in v1")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	steps := make([]StepResult, len(task.Steps))
	errCh := make(chan error, len(task.Steps))
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := false

	for i, step := range task.Steps {
		wg.Add(1)
		go func(i int, step string) {
			defer wg.Done()
			stepRes, err := r.runStep(ctx, i, taskName, step, opts)
			if err != nil {
				errCh <- err
				return
			}

			mu.Lock()
			steps[i] = stepRes
			if stepRes.Status != StatusPass && stepRes.Status != StatusCancelled {
				failed = true
				cancel()
			}
			mu.Unlock()
		}(i, step)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return Result{}, err
		}
	}

	overallStatus := StatusPass
	overallCode := 0
	if failed {
		overallStatus = StatusFail
		overallCode = 1
	}
	for i, step := range steps {
		if step.Name == "" {
			steps[i] = cancelledStep(i, task.Steps[i])
		}
		if step.ExitCode != 0 && overallCode == 0 {
			overallCode = step.ExitCode
		}
	}

	finished := time.Now()
	return Result{
		Task:       taskName,
		Type:       "pipeline",
		Parallel:   true,
		Status:     overallStatus,
		ExitCode:   overallCode,
		DurationMS: finished.Sub(started).Milliseconds(),
		StartedAt:  started.UTC().Format(time.RFC3339),
		FinishedAt: finished.UTC().Format(time.RFC3339),
		Steps:      steps,
	}, nil
}

func (r *Runner) runStep(ctx context.Context, index int, parentTaskName, step string, opts Options) (StepResult, error) {
	task, ok := r.cfg.Tasks[step]
	if ok {
		if task.Cmd == "" {
			return StepResult{}, fmt.Errorf("task %q references pipeline task %q, but nested pipeline steps are not implemented yet", parentTaskName, step)
		}
		paramValues, err := resolveParamValues(task, opts.Params)
		if err != nil {
			return StepResult{}, fmt.Errorf("task %q: %w", step, err)
		}
		outcome, err := r.runCommand(ctx, step, task, interpolateParams(task.Cmd, paramValues), opts, step)
		if err != nil {
			return StepResult{}, err
		}
		stepResult := toStepResult(index, step, interpolateParams(task.Cmd, paramValues), outcome)
		stepResult.Errors = extractErrors(task.ErrorFormat, outcome.stderr)
		return stepResult, nil
	}

	outcome, err := r.runCommand(ctx, inlineStepName(index), config.Task{}, step, opts, inlineStepName(index))
	if err != nil {
		return StepResult{}, err
	}
	return toStepResult(index, inlineStepName(index), step, outcome), nil
}

func (r *Runner) runCommand(parent context.Context, label string, task config.Task, command string, opts Options, prefix string) (runOutcome, error) {
	started := time.Now()
	if opts.DryRun {
		fmt.Fprintln(opts.Stdout, command)
		now := time.Now()
		return runOutcome{status: StatusPass, exitCode: 0, started: started, finished: now}, nil
	}

	ctx := parent
	var cancel context.CancelFunc
	if task.Timeout != "" {
		timeout, err := time.ParseDuration(task.Timeout)
		if err != nil {
			return runOutcome{}, fmt.Errorf("task %q: invalid timeout %q: %w", label, task.Timeout, err)
		}
		ctx, cancel = context.WithTimeout(parent, timeout)
		defer cancel()
	}

	shell, shellArgs := resolveShell(task)
	cmdArgs := append(append([]string{}, shellArgs...), command)
	cmd := exec.CommandContext(ctx, shell, cmdArgs...)
	cmd.Dir = r.resolveTaskDir(task)
	paramValues, err := resolveParamValues(task, opts.Params)
	if err != nil {
		return runOutcome{}, fmt.Errorf("task %q: %w", label, err)
	}
	cmd.Env = mergeEnv(os.Environ(), r.globalEnv, interpolateEnv(task.Env, paramValues), opts.Env, paramEnv(task, paramValues))

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&stdoutBuf, prefixedWriter(prefix, opts.Stdout))
	cmd.Stderr = io.MultiWriter(&stderrBuf, prefixedWriter(prefix, opts.Stderr))

	err = cmd.Run()
	finished := time.Now()
	if err == nil {
		return runOutcome{
			status:   StatusPass,
			exitCode: 0,
			stdout:   stdoutBuf.String(),
			stderr:   stderrBuf.String(),
			started:  started,
			finished: finished,
		}, nil
	}

	exitCode := 1
	status := StatusFail
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		exitCode = 124
		status = StatusTimeout
	} else if errors.Is(ctx.Err(), context.Canceled) {
		exitCode = 130
		status = StatusCancelled
	} else {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if errors.Is(err, exec.ErrNotFound) {
			exitCode = 127
		}
	}

	return runOutcome{
		status:   status,
		exitCode: exitCode,
		stdout:   stdoutBuf.String(),
		stderr:   stderrBuf.String(),
		started:  started,
		finished: finished,
	}, nil
}

func resolveParamValues(task config.Task, provided map[string]string) (map[string]string, error) {
	values := map[string]string{}
	for name, param := range task.Params {
		if provided != nil {
			if value, ok := provided[name]; ok {
				values[name] = value
				continue
			}
		}
		if param.Default != "" {
			values[name] = param.Default
			continue
		}
		if param.Required {
			return nil, fmt.Errorf("missing required param %q", name)
		}
	}
	for name, value := range provided {
		if _, ok := task.Params[name]; ok {
			values[name] = value
		}
	}
	return values, nil
}

func interpolateParams(value string, params map[string]string) string {
	out := value
	for name, paramValue := range params {
		out = strings.ReplaceAll(out, "{{params."+name+"}}", paramValue)
	}
	return out
}

func interpolateEnv(env map[string]string, params map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for key, value := range env {
		out[key] = interpolateParams(value, params)
	}
	return out
}

func paramEnv(task config.Task, params map[string]string) map[string]string {
	if len(task.Params) == 0 {
		return nil
	}
	out := map[string]string{}
	for name, param := range task.Params {
		if value, ok := params[name]; ok {
			out[param.Env] = value
		}
	}
	return out
}

func toStepResult(index int, name, resolved string, outcome runOutcome) StepResult {
	duration := outcome.finished.Sub(outcome.started).Milliseconds()
	started := outcome.started.UTC().Format(time.RFC3339)
	finished := outcome.finished.UTC().Format(time.RFC3339)
	return StepResult{
		Index:       index,
		Name:        name,
		ResolvedCmd: strPtr(resolved),
		Status:      outcome.status,
		ExitCode:    outcome.exitCode,
		Stdout:      strPtr(outcome.stdout),
		Stderr:      strPtr(outcome.stderr),
		Errors:      nil,
		DurationMS:  &duration,
		StartedAt:   &started,
		FinishedAt:  &finished,
	}
}

func cancelledStep(index int, name string) StepResult {
	return StepResult{
		Index:    index,
		Name:     name,
		Status:   StatusCancelled,
		ExitCode: 130,
	}
}

func inlineStepName(index int) string {
	return fmt.Sprintf("step-%d", index+1)
}

func mergeEnv(base []string, layers ...map[string]string) []string {
	merged := map[string]string{}
	for _, item := range base {
		if key, value, ok := strings.Cut(item, "="); ok {
			merged[key] = value
		}
	}
	for _, layer := range layers {
		for key, value := range layer {
			merged[key] = value
		}
	}
	out := make([]string, 0, len(merged))
	for key, value := range merged {
		out = append(out, key+"="+value)
	}
	return out
}

func loadEnvFile(path string) map[string]string {
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			env[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return env
}

func defaultShellCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd.exe"
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/sh"
}

func defaultShellArgs() []string {
	if runtime.GOOS == "windows" {
		return []string{"/C"}
	}
	return []string{"-c"}
}

func resolveShell(task config.Task) (string, []string) {
	command := defaultShellCommand()
	if task.Shell != "" {
		command = task.Shell
	}
	args := defaultShellArgs()
	if len(task.ShellArgs) > 0 {
		args = append([]string{}, task.ShellArgs...)
	}
	return command, args
}

func (r *Runner) resolveTaskDir(task config.Task) string {
	if task.Dir != "" {
		return filepath.Join(r.repoRoot, task.Dir)
	}
	if r.cfg.Defaults.Dir != "" {
		return filepath.Join(r.repoRoot, r.cfg.Defaults.Dir)
	}
	return r.repoRoot
}

func prefixedWriter(prefix string, target io.Writer) io.Writer {
	if target == nil || prefix == "" {
		return target
	}
	return &linePrefixWriter{prefix: "[" + prefix + "] ", target: target, atLineStart: true}
}

type linePrefixWriter struct {
	prefix      string
	target      io.Writer
	atLineStart bool
}

func (w *linePrefixWriter) Write(p []byte) (int, error) {
	if w.target == nil {
		return len(p), nil
	}
	written := 0
	for len(p) > 0 {
		if w.atLineStart {
			if _, err := io.WriteString(w.target, w.prefix); err != nil {
				return written, err
			}
			w.atLineStart = false
		}
		i := bytes.IndexByte(p, '\n')
		if i == -1 {
			n, err := w.target.Write(p)
			written += n
			return written, err
		}
		chunk := p[:i+1]
		n, err := w.target.Write(chunk)
		written += n
		if err != nil {
			return written, err
		}
		p = p[i+1:]
		w.atLineStart = true
	}
	return written, nil
}

func strPtr(s string) *string {
	return &s
}

var (
	genericErrorPattern = regexp.MustCompile(`^(.+?):(\d+)(?::(\d+))?:\s*(.+)$`)
	goTestPattern       = regexp.MustCompile(`^\s*(.+?\.go):(\d+):\s*(.+)$`)
	pytestPattern       = regexp.MustCompile(`^(.+?):(\d+):\s+(.+)$`)
	tscPattern          = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\):\s+error\s+[^:]+:\s+(.+)$`)
	eslintPattern       = regexp.MustCompile(`^(.+)$`)
)

func extractErrors(format, stderr string) []ErrorEntry {
	if stderr == "" || format == "" {
		return nil
	}
	switch format {
	case "go_test":
		return parseGoTestErrors(stderr)
	case "pytest":
		return parsePytestErrors(stderr)
	case "tsc":
		return parseTscErrors(stderr)
	case "eslint":
		return parseEslintErrors(stderr)
	case "generic":
		return parseGenericErrors(stderr)
	default:
		return nil
	}
}

func parseGoTestErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, line := range strings.Split(stderr, "\n") {
		match := goTestPattern.FindStringSubmatch(line)
		if len(match) != 4 {
			continue
		}
		lineNo, _ := strconv.Atoi(match[2])
		errors = append(errors, ErrorEntry{
			File:     match[1],
			Line:     lineNo,
			Message:  strings.TrimSpace(match[3]),
			Severity: "error",
		})
	}
	return errors
}

func parsePytestErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, line := range strings.Split(stderr, "\n") {
		match := pytestPattern.FindStringSubmatch(line)
		if len(match) != 4 {
			continue
		}
		lineNo, _ := strconv.Atoi(match[2])
		errors = append(errors, ErrorEntry{
			File:     match[1],
			Line:     lineNo,
			Message:  strings.TrimSpace(match[3]),
			Severity: "error",
		})
	}
	return errors
}

func parseTscErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, line := range strings.Split(stderr, "\n") {
		match := tscPattern.FindStringSubmatch(line)
		if len(match) != 5 {
			continue
		}
		lineNo, _ := strconv.Atoi(match[2])
		columnNo, _ := strconv.Atoi(match[3])
		errors = append(errors, ErrorEntry{
			File:     match[1],
			Line:     lineNo,
			Column:   columnNo,
			Message:  strings.TrimSpace(match[4]),
			Severity: "error",
		})
	}
	return errors
}

func parseEslintErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, rawLine := range strings.Split(stderr, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "/") || strings.HasPrefix(line, "✖") {
			continue
		}
		match := genericErrorPattern.FindStringSubmatch(line)
		if len(match) == 5 {
			lineNo, _ := strconv.Atoi(match[2])
			columnNo := 0
			if match[3] != "" {
				columnNo, _ = strconv.Atoi(match[3])
			}
			errors = append(errors, ErrorEntry{
				File:     match[1],
				Line:     lineNo,
				Column:   columnNo,
				Message:  strings.TrimSpace(match[4]),
				Severity: "error",
			})
			continue
		}
		if eslintPattern.MatchString(line) && strings.Contains(line, "error") {
			errors = append(errors, ErrorEntry{
				Message:  line,
				Severity: "error",
			})
		}
	}
	return errors
}

func parseGenericErrors(stderr string) []ErrorEntry {
	var errors []ErrorEntry
	for _, line := range strings.Split(stderr, "\n") {
		match := genericErrorPattern.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 5 {
			continue
		}
		lineNo, _ := strconv.Atoi(match[2])
		columnNo := 0
		if match[3] != "" {
			columnNo, _ = strconv.Atoi(match[3])
		}
		errors = append(errors, ErrorEntry{
			File:     match[1],
			Line:     lineNo,
			Column:   columnNo,
			Message:  strings.TrimSpace(match[4]),
			Severity: "error",
		})
	}
	return errors
}

func collectResultErrors(result Result) []ErrorEntry {
	if len(result.Errors) > 0 {
		return append([]ErrorEntry(nil), result.Errors...)
	}
	var errors []ErrorEntry
	for _, step := range result.Steps {
		errors = append(errors, step.Errors...)
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}
