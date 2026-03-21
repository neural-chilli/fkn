package context

import (
	"fmt"
	"github.com/neural-chilli/fkn/internal/config"
	"strings"
)

type Generator struct {
	cfg      *config.Config
	repoRoot string
}

type Options struct {
	Agent     bool
	Task      string
	About     string
	MaxTokens int
}

type section struct {
	title string
	body  string
}

type JSONSection struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type JSONOutput struct {
	Project   string        `json:"project,omitempty"`
	RepoRoot  string        `json:"repo_root"`
	Agent     bool          `json:"agent"`
	Task      string        `json:"task,omitempty"`
	About     string        `json:"about,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Sections  []JSONSection `json:"sections"`
	Markdown  string        `json:"markdown"`
}

func New(cfg *config.Config, repoRoot string) *Generator {
	return &Generator{cfg: cfg, repoRoot: repoRoot}
}

func (g *Generator) Generate(opts Options) (string, error) {
	sections, err := g.sections(opts)
	if err != nil {
		return "", err
	}
	rendered := renderSections(sections)
	if opts.MaxTokens > 0 {
		rendered = truncateToTokenBudget(rendered, opts.MaxTokens)
	}
	return rendered, nil
}

func (g *Generator) GenerateJSON(opts Options) (JSONOutput, error) {
	sections, err := g.sections(opts)
	if err != nil {
		return JSONOutput{}, err
	}
	rendered := renderSections(sections)
	if opts.MaxTokens > 0 {
		rendered = truncateToTokenBudget(rendered, opts.MaxTokens)
	}

	out := JSONOutput{
		Project:   g.cfg.Project,
		RepoRoot:  g.repoRoot,
		Agent:     opts.Agent,
		Task:      opts.Task,
		About:     opts.About,
		MaxTokens: opts.MaxTokens,
		Sections:  make([]JSONSection, 0, len(sections)),
		Markdown:  rendered,
	}
	for _, sec := range sections {
		out.Sections = append(out.Sections, JSONSection{
			Title: sec.title,
			Body:  sec.body,
		})
	}
	return out, nil
}

func (g *Generator) sections(opts Options) ([]section, error) {
	if opts.About != "" {
		return g.aboutSections(opts)
	}

	sections := []section{
		{title: "Project", body: g.projectSection()},
	}

	if opts.Agent {
		agentBody, err := g.agentSection(opts.Task)
		if err != nil {
			return nil, err
		}
		sections = append(sections, section{title: "Agent Task", body: agentBody})
	}

	if g.contextFileTreeEnabled() {
		if body := g.fileTreeSection(); body != "" {
			sections = append(sections, section{title: "File Tree", body: body})
		}
	}

	if body := g.taskSection(); body != "" {
		sections = append(sections, section{title: "Tasks", body: body})
	}
	if body := g.guardSection(); body != "" {
		sections = append(sections, section{title: "Guards", body: body})
	}
	if body := g.dependenciesSection(); body != "" {
		sections = append(sections, section{title: "Dependencies", body: body})
	}
	if body := g.gitLogSection(); body != "" {
		sections = append(sections, section{title: "Recent Git Log", body: body})
	}
	if body := g.filesSection(g.cfg.Context.Files, g.cfg.Context.Caps.FileLines, g.cfg.Context.Caps.FilesMax); body != "" {
		sections = append(sections, section{title: "Configured Files", body: body})
	}
	if body := g.filesSection(g.cfg.Context.AgentFiles, g.cfg.Context.Caps.AgentFileLines, len(g.cfg.Context.AgentFiles)); body != "" {
		sections = append(sections, section{title: "Agent Files", body: body})
	}
	if g.cfg.Context.Todos {
		if body := g.todosSection(); body != "" {
			sections = append(sections, section{title: "TODOs", body: body})
		}
	}
	if opts.Agent || g.cfg.Context.GitDiff {
		if body := g.gitDiffSection(); body != "" {
			sections = append(sections, section{title: "Git Diff", body: body})
		}
	}
	if opts.Agent {
		if body := g.codemapSection(opts.Task); body != "" {
			sections = append(sections, section{title: "Codemap", body: body})
		}
		if body := g.lastGuardSection(); body != "" {
			sections = append(sections, section{title: "Last Guard", body: body})
		}
	}
	return filterEmptySections(sections), nil
}

func (g *Generator) aboutSections(opts Options) ([]section, error) {
	topic := strings.TrimSpace(opts.About)
	sections := []section{
		{title: "Project", body: g.projectSection()},
		{title: "Topic", body: fmt.Sprintf("- Query: %s", topic)},
	}

	if body := g.aboutTasksSection(topic); body != "" {
		sections = append(sections, section{title: "Matching Tasks", body: body})
	}
	if body := g.aboutScopesSection(topic); body != "" {
		sections = append(sections, section{title: "Matching Scopes", body: body})
	}
	if body := g.aboutCodemapSection(topic); body != "" {
		sections = append(sections, section{title: "Matching Codemap", body: body})
	}
	if body := g.aboutGlossarySection(topic); body != "" {
		sections = append(sections, section{title: "Glossary", body: body})
	}
	if body := g.aboutRelevantPathsSection(topic); body != "" {
		sections = append(sections, section{title: "Relevant Paths", body: body})
	}
	if body := g.gitDiffSection(); body != "" {
		sections = append(sections, section{title: "Git Diff", body: body})
	}

	return filterEmptySections(sections), nil
}

func renderSections(sections []section) string {
	var out []string
	out = append(out, "# fkn context")
	for _, sec := range sections {
		out = append(out, fmt.Sprintf("\n## %s\n\n%s", sec.title, sec.body))
	}
	return strings.Join(out, "\n")
}

func filterEmptySections(sections []section) []section {
	out := make([]section, 0, len(sections))
	for _, sec := range sections {
		if sec.body == "" {
			continue
		}
		out = append(out, sec)
	}
	return out
}
