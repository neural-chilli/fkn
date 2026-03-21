package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Project     string              `yaml:"project"`
	Description string              `yaml:"description"`
	EnvFile     string              `yaml:"env_file"`
	Tasks       map[string]Task     `yaml:"tasks"`
	Aliases     map[string]string   `yaml:"aliases"`
	Guards      map[string]Guard    `yaml:"guards"`
	Scopes      map[string][]string `yaml:"scopes"`
	Prompts     map[string]Prompt   `yaml:"prompts"`
	Context     ContextConfig       `yaml:"context"`
	Serve       ServeConfig         `yaml:"serve"`
	Watch       WatchConfig         `yaml:"watch"`
}

type Task struct {
	Desc            string            `yaml:"desc"`
	Cmd             string            `yaml:"cmd"`
	Steps           []string          `yaml:"steps"`
	Parallel        bool              `yaml:"parallel"`
	Params          map[string]Param  `yaml:"params"`
	Env             map[string]string `yaml:"env"`
	Dir             string            `yaml:"dir"`
	Timeout         string            `yaml:"timeout"`
	ContinueOnError bool              `yaml:"continue_on_error"`
	Agent           *bool             `yaml:"agent"`
	Scope           string            `yaml:"scope"`
}

type Param struct {
	Desc     string `yaml:"desc"`
	Env      string `yaml:"env"`
	Required bool   `yaml:"required"`
	Default  string `yaml:"default"`
}

type Guard struct {
	Steps []string `yaml:"steps"`
}

type Prompt struct {
	Desc     string `yaml:"desc"`
	Template string `yaml:"template"`
}

type ContextConfig struct {
	FileTree     *bool       `yaml:"file_tree"`
	GitLogLines  int         `yaml:"git_log_lines"`
	GitDiff      bool        `yaml:"git_diff"`
	Todos        bool        `yaml:"todos"`
	Dependencies *bool       `yaml:"dependencies"`
	AgentFiles   []string    `yaml:"agent_files"`
	Files        []string    `yaml:"files"`
	Include      []string    `yaml:"include"`
	Exclude      []string    `yaml:"exclude"`
	Caps         ContextCaps `yaml:"caps"`
}

type ContextCaps struct {
	FileTreeEntries int `yaml:"file_tree_entries"`
	FilesMax        int `yaml:"files_max"`
	FileLines       int `yaml:"file_lines"`
	GitLogLines     int `yaml:"git_log_lines"`
	GitDiffLines    int `yaml:"git_diff_lines"`
	TodosMax        int `yaml:"todos_max"`
	AgentFileLines  int `yaml:"agent_file_lines"`
	DependencyLines int `yaml:"dependency_lines"`
}

type ServeConfig struct {
	Transport string `yaml:"transport"`
	Port      int    `yaml:"port"`
	TokenEnv  string `yaml:"token_env"`
}

type WatchConfig struct {
	DebounceMS int      `yaml:"debounce_ms"`
	Paths      []string `yaml:"paths"`
}

func (t Task) AgentEnabled() bool {
	return t.Agent == nil || *t.Agent
}

func (t Task) Type() string {
	if t.Cmd != "" {
		return "cmd"
	}
	return "pipeline"
}

func (c *Config) ResolveTaskName(name string) (string, bool) {
	if _, ok := c.Tasks[name]; ok {
		return name, true
	}
	target, ok := c.Aliases[name]
	if !ok {
		return "", false
	}
	_, ok = c.Tasks[target]
	return target, ok
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}

	cfg.applyDefaults()

	if err := cfg.Validate(filepath.Dir(path)); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Context.GitLogLines == 0 {
		c.Context.GitLogLines = 10
	}
	if c.Context.Caps.FileTreeEntries == 0 {
		c.Context.Caps.FileTreeEntries = 200
	}
	if c.Context.Caps.FilesMax == 0 {
		c.Context.Caps.FilesMax = 5
	}
	if c.Context.Caps.FileLines == 0 {
		c.Context.Caps.FileLines = 100
	}
	if c.Context.Caps.GitLogLines == 0 {
		c.Context.Caps.GitLogLines = 30
	}
	if c.Context.Caps.GitDiffLines == 0 {
		c.Context.Caps.GitDiffLines = 200
	}
	if c.Context.Caps.TodosMax == 0 {
		c.Context.Caps.TodosMax = 20
	}
	if c.Context.Caps.AgentFileLines == 0 {
		c.Context.Caps.AgentFileLines = 500
	}
	if c.Context.Caps.DependencyLines == 0 {
		c.Context.Caps.DependencyLines = 100
	}
	if c.Serve.Transport == "" {
		c.Serve.Transport = "stdio"
	}
	if c.Serve.Port == 0 {
		c.Serve.Port = 8080
	}
	if c.Serve.TokenEnv == "" {
		c.Serve.TokenEnv = "FKN_MCP_TOKEN"
	}
	if c.Watch.DebounceMS == 0 {
		c.Watch.DebounceMS = 500
	}
}

func (c *Config) Validate(repoRoot string) error {
	if len(c.Tasks) == 0 {
		return fmt.Errorf("fkn.yaml must define at least one task")
	}

	for name, task := range c.Tasks {
		if task.Desc == "" {
			return fmt.Errorf("task %q: desc is required", name)
		}
		if (task.Cmd == "") == (len(task.Steps) == 0) {
			return fmt.Errorf("task %q: set exactly one of cmd or steps", name)
		}
		if task.Dir != "" {
			target := filepath.Join(repoRoot, task.Dir)
			info, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("task %q: dir %q: %w", name, task.Dir, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("task %q: dir %q is not a directory", name, task.Dir)
			}
		}
		if task.Scope != "" {
			if _, ok := c.Scopes[task.Scope]; !ok {
				return fmt.Errorf("task %q references unknown scope %q", name, task.Scope)
			}
		}
		for paramName, param := range task.Params {
			if param.Env == "" {
				return fmt.Errorf("task %q param %q: env is required", name, paramName)
			}
		}
	}

	for alias, target := range c.Aliases {
		if _, ok := c.Tasks[alias]; ok {
			return fmt.Errorf("alias %q conflicts with task of the same name", alias)
		}
		if _, ok := c.Tasks[target]; !ok {
			return fmt.Errorf("alias %q references unknown task %q", alias, target)
		}
	}

	for name, guard := range c.Guards {
		if len(guard.Steps) == 0 {
			return fmt.Errorf("guard %q: steps are required", name)
		}
		for _, step := range guard.Steps {
			if _, ok := c.Tasks[step]; !ok {
				return fmt.Errorf("guard %q references unknown task %q", name, step)
			}
		}
	}

	for name, prompt := range c.Prompts {
		if prompt.Desc == "" {
			return fmt.Errorf("prompt %q: desc is required", name)
		}
		if prompt.Template == "" {
			return fmt.Errorf("prompt %q: template is required", name)
		}
	}

	return c.validateCycles()
}

func (c *Config) validateCycles() error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	stack := []string{}

	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			start := 0
			for i, item := range stack {
				if item == name {
					start = i
					break
				}
			}
			cycle := append(append([]string{}, stack[start:]...), name)
			return fmt.Errorf("circular task dependency: %s", joinArrow(cycle))
		}
		if visited[name] {
			return nil
		}

		task, ok := c.Tasks[name]
		if !ok {
			return fmt.Errorf("task %q references unknown task", name)
		}

		visiting[name] = true
		stack = append(stack, name)
		for _, step := range task.Steps {
			if _, ok := c.Tasks[step]; ok {
				if err := visit(step); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		visiting[name] = false
		visited[name] = true
		return nil
	}

	names := make([]string, 0, len(c.Tasks))
	for name := range c.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func joinArrow(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, part := range parts[1:] {
		out += " -> " + part
	}
	return out
}
