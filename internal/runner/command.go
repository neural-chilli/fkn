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
	"runtime"
	"strings"
	"time"

	"github.com/neural-chilli/fkn/internal/config"
)

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
