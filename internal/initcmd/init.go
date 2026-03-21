package initcmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
)

type Options struct {
	FromRepo bool
	Agents   bool
}

const starterConfig = `project: my-project
description: Describe your repository
default: check

tasks:
  test:
    desc: Run the test suite
    scope: cli
    cmd: go test ./...

  build:
    desc: Build the application
    scope: cli
    cmd: go build ./...

  check:
    desc: Run the default local verification pipeline
    scope: cli
    steps:
      - test
      - build

groups:
  core:
    desc: Everyday local development commands.
    tasks:
      - test
      - build
      - check

scopes:
  cli:
    desc: Main CLI commands and closely-related execution packages.
    paths:
      - cmd/
      - internal/
`

const (
	agentsBlockStart = "<!-- fkn:agents:start -->"
	agentsBlockEnd   = "<!-- fkn:agents:end -->"
)

type inferredTask struct {
	Name   string
	Desc   string
	Cmd    string
	Steps  []string
	Agent  *bool
	Params map[string]config.Param
}

type makeTarget struct {
	Name   string
	Params []string
}

type justRecipe struct {
	Name   string
	Params []justParam
}

type justParam struct {
	Name     string
	Default  string
	Required bool
	Variadic bool
}

type packageScript struct {
	Name   string
	Cmd    string
	Params map[string]config.Param
}

func Run(repoRoot string, opts Options) (string, error) {
	var messages []string

	cfgPath := filepath.Join(repoRoot, "fkn.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		messages = append(messages, "fkn.yaml already exists; leaving it unchanged")
	} else if os.IsNotExist(err) {
		configBody := starterConfig
		if opts.FromRepo {
			configBody = inferConfig(repoRoot)
		}
		if err := os.WriteFile(cfgPath, []byte(configBody), 0o644); err != nil {
			return "", err
		}
		messages = append(messages, "created fkn.yaml")
		if opts.FromRepo {
			messages = append(messages, "scaffolded tasks from existing repo files")
		}
	} else {
		return "", err
	}

	updated, err := ensureGitignoreEntry(filepath.Join(repoRoot, ".gitignore"), ".fkn/")
	if err != nil {
		return "", err
	}
	if updated {
		messages = append(messages, "updated .gitignore with .fkn/")
	} else {
		messages = append(messages, ".gitignore already includes .fkn/")
	}

	if opts.Agents {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return "", err
		}
		agentDoc, err := renderAgentsFKN(cfg)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(repoRoot, "AGENTS_FKN.md"), []byte(agentDoc), 0o644); err != nil {
			return "", err
		}
		messages = append(messages, "wrote AGENTS_FKN.md")

		updatedAgents, err := ensureAgentsReference(filepath.Join(repoRoot, "AGENTS.md"))
		if err != nil {
			return "", err
		}
		if updatedAgents {
			messages = append(messages, "updated AGENTS.md with fkn guidance")
		} else {
			messages = append(messages, "AGENTS.md already includes fkn guidance")
		}
	}

	return strings.Join(messages, "\n"), nil
}
