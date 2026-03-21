package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/neural-chilli/fkn/internal/config"
)

func (r *Runner) runSequential(ctx context.Context, taskName string, task config.Task, needs []Result, started time.Time, opts Options) (Result, error) {
	steps := make([]StepResult, 0, len(task.Steps))
	overallStatus := StatusPass
	overallCode := 0

	for i, step := range task.Steps {
		stepRes, err := r.runStep(ctx, i, taskName, step, opts)
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
		Needs:      needs,
		Parallel:   false,
		Status:     overallStatus,
		ExitCode:   overallCode,
		DurationMS: finished.Sub(started).Milliseconds(),
		StartedAt:  started.UTC().Format(time.RFC3339),
		FinishedAt: finished.UTC().Format(time.RFC3339),
		Steps:      steps,
	}, nil
}

func (r *Runner) runParallel(parent context.Context, taskName string, task config.Task, needs []Result, started time.Time, opts Options) (Result, error) {
	if task.ContinueOnError && !opts.JSON {
		fmt.Fprintln(opts.Stderr, "warning: continue_on_error is ignored for parallel tasks in v1")
	}

	ctx, cancel := context.WithCancel(parent)
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
		Needs:      needs,
		Parallel:   true,
		Status:     overallStatus,
		ExitCode:   overallCode,
		DurationMS: finished.Sub(started).Milliseconds(),
		StartedAt:  started.UTC().Format(time.RFC3339),
		FinishedAt: finished.UTC().Format(time.RFC3339),
		Steps:      steps,
	}, nil
}

func (r *Runner) runNeeds(ctx context.Context, task config.Task, opts Options) ([]Result, *Result, error) {
	if len(task.Needs) == 0 {
		return nil, nil, nil
	}
	results := make([]Result, 0, len(task.Needs))
	for _, depName := range task.Needs {
		result, err := r.runTask(ctx, depName, opts)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, result)
		if result.Status != StatusPass {
			failed := result
			return results, &failed, nil
		}
	}
	return results, nil, nil
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
