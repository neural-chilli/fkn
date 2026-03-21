package brief

import (
	"fmt"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
	contextpkg "github.com/neural-chilli/fkn/internal/context"
	planpkg "github.com/neural-chilli/fkn/internal/plan"
)

type Generator struct {
	cfg      *config.Config
	repoRoot string
	context  *contextpkg.Generator
}

type Options struct {
	Task      string
	Files     []string
	Diff      bool
	MaxTokens int
}

type Output struct {
	Project  string                `json:"project,omitempty"`
	RepoRoot string                `json:"repo_root"`
	Task     string                `json:"task,omitempty"`
	Files    []string              `json:"files,omitempty"`
	Diff     bool                  `json:"diff,omitempty"`
	Context  contextpkg.JSONOutput `json:"context"`
	Plan     *planpkg.Output       `json:"plan,omitempty"`
	Markdown string                `json:"markdown"`
}

func New(cfg *config.Config, repoRoot string) *Generator {
	return &Generator{
		cfg:      cfg,
		repoRoot: repoRoot,
		context:  contextpkg.New(cfg, repoRoot),
	}
}

func (g *Generator) Generate(opts Options) (Output, error) {
	contextOpts := contextpkg.Options{MaxTokens: opts.MaxTokens}
	if opts.Task != "" {
		contextOpts.Agent = true
		contextOpts.Task = opts.Task
	}

	contextOut, err := g.context.GenerateJSON(contextOpts)
	if err != nil {
		return Output{}, err
	}

	var planOut *planpkg.Output
	switch {
	case opts.Diff:
		result, err := planpkg.GenerateFromGitDiff(g.cfg, g.repoRoot)
		if err != nil {
			return Output{}, err
		}
		planOut = &result
	case len(opts.Files) > 0:
		result, err := planpkg.Generate(g.cfg, g.repoRoot, opts.Files)
		if err != nil {
			return Output{}, err
		}
		planOut = &result
	}

	out := Output{
		Project:  g.cfg.Project,
		RepoRoot: g.repoRoot,
		Task:     opts.Task,
		Diff:     opts.Diff,
		Context:  contextOut,
		Plan:     planOut,
	}
	if planOut != nil {
		out.Files = append([]string(nil), planOut.Files...)
	}
	out.Markdown = render(out)
	return out, nil
}

func render(out Output) string {
	sections := []string{"# fkn agent brief", strings.TrimSpace(out.Context.Markdown)}
	if out.Plan != nil {
		sections = append(sections, "## Change Plan\n")
		sections = append(sections, strings.TrimSpace(out.Plan.Markdown))
	}
	sections = append(sections, "## Suggested Next Step\n\n"+suggestedNextStep(out))
	return strings.Join(filterEmpty(sections), "\n\n")
}

func suggestedNextStep(out Output) string {
	switch {
	case out.Task != "":
		return fmt.Sprintf("Start with `fkn help %s`, then run the task-specific workflow with `fkn context --agent --task %s` if you need a narrower brief.", out.Task, out.Task)
	case out.Diff && out.Plan != nil:
		return "Use the matching tasks and guards from the diff plan to verify the changed files before handing work to an agent."
	case len(out.Files) > 0 && out.Plan != nil:
		return "Use the matching scopes, tasks, and guards from the plan to focus an agent on the edited files."
	default:
		return "Pick a task or file set next if you want a narrower, execution-oriented brief."
	}
}

func filterEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
