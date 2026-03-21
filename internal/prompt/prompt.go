package prompt

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"fkn/internal/config"
)

var placeholderPattern = regexp.MustCompile(`\{\{([^}]+)\}\}`)

type Renderer struct {
	cfg      *config.Config
	repoRoot string
}

func New(cfg *config.Config, repoRoot string) *Renderer {
	return &Renderer{cfg: cfg, repoRoot: repoRoot}
}

func (r *Renderer) Render(name string) (string, []string, error) {
	promptCfg, ok := r.cfg.Prompts[name]
	if !ok {
		return "", nil, fmt.Errorf("unknown prompt %q", name)
	}

	warnings := []string{}
	rendered := placeholderPattern.ReplaceAllStringFunc(promptCfg.Template, func(match string) string {
		key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(match, "{{"), "}}"))
		value, ok := r.resolve(key)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("unknown prompt variable %q", match))
			return match
		}
		return value
	})

	return rendered, warnings, nil
}

func (r *Renderer) resolve(key string) (string, bool) {
	switch key {
	case "git_branch":
		return r.gitOutput("rev-parse", "--abbrev-ref", "HEAD"), true
	case "git_diff":
		return r.gitOutput("diff", "--"), true
	case "git_log":
		return r.gitOutput("log", "--oneline", "-n", "10"), true
	case "timestamp":
		return time.Now().UTC().Format(time.RFC3339), true
	case "os":
		return runtime.GOOS, true
	}

	if strings.HasPrefix(key, "scope.") {
		name := strings.TrimPrefix(key, "scope.")
		paths, ok := r.cfg.Scopes[name]
		if !ok {
			return "", false
		}
		return strings.Join(paths, ", "), true
	}

	if strings.HasPrefix(key, "task.") && strings.HasSuffix(key, ".desc") {
		name := strings.TrimSuffix(strings.TrimPrefix(key, "task."), ".desc")
		task, ok := r.cfg.Tasks[name]
		if !ok {
			return "", false
		}
		return task.Desc, true
	}

	return "", false
}

func (r *Renderer) gitOutput(args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.repoRoot
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimRight(stdout.String(), "\n")
}
