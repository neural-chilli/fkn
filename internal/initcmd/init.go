package initcmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/neural-chilli/fkn/internal/config"
)

type Options struct {
	FromRepo bool
	Docs     bool
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

agent:
  accrue_knowledge: false
`

const (
	humansBlockStart = "<!-- fkn:humans:start -->"
	humansBlockEnd   = "<!-- fkn:humans:end -->"
	agentsBlockStart = "<!-- fkn:agents:start -->"
	agentsBlockEnd   = "<!-- fkn:agents:end -->"
	claudeBlockStart = "<!-- fkn:claude:start -->"
	claudeBlockEnd   = "<!-- fkn:claude:end -->"
)

type inferredTask struct {
	Name   string
	Desc   string
	Cmd    string
	Steps  []string
	Agent  *bool
	Safety string
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
			if !opts.Docs {
				messages = append(messages, "tip: run `fkn init --docs` to generate HUMANS.md, AGENTS.md, and CLAUDE.md")
			}
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

	if opts.Docs {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return "", err
		}
		statuses, err := writeDocs(repoRoot, cfg)
		if err != nil {
			return "", err
		}
		messages = append(messages, statuses...)
	}

	return strings.Join(messages, "\n"), nil
}
