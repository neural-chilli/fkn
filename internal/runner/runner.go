package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
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
	Needs       []Result     `json:"needs,omitempty"`
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
	return r.runTask(context.Background(), taskName, opts)
}

func (r *Runner) runTask(ctx context.Context, taskName string, opts Options) (Result, error) {
	task, ok := r.cfg.Tasks[taskName]
	if !ok {
		return Result{}, fmt.Errorf("unknown task %q", taskName)
	}

	started := time.Now()
	needs, depFailure, err := r.runNeeds(ctx, task, opts)
	if err != nil {
		return Result{}, err
	}
	if depFailure != nil {
		finished := time.Now()
		return Result{
			Task:       taskName,
			Type:       task.Type(),
			Needs:      needs,
			Status:     depFailure.Status,
			ExitCode:   depFailure.ExitCode,
			Errors:     collectResultErrors(*depFailure),
			DurationMS: finished.Sub(started).Milliseconds(),
			StartedAt:  started.UTC().Format(time.RFC3339),
			FinishedAt: finished.UTC().Format(time.RFC3339),
		}, nil
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
			Needs:       needs,
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
		return r.runParallel(ctx, taskName, task, needs, started, opts)
	}
	return r.runSequential(ctx, taskName, task, needs, started, opts)
}

func (r *Runner) RunGuardStep(stepName string, opts Options) (StepResult, error) {
	result, err := r.runTask(context.Background(), stepName, opts)
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
