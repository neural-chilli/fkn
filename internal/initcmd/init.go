package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const starterConfig = `project: my-project
description: Describe your repository

tasks:
  test:
    desc: Run the test suite
    cmd: go test ./...

  build:
    desc: Build the application
    cmd: go build ./...

  check:
    desc: Run the default local verification pipeline
    steps:
      - test
      - build
`

func Run(repoRoot string) (string, error) {
	var messages []string

	cfgPath := filepath.Join(repoRoot, "fkn.yaml")
	if _, err := os.Stat(cfgPath); err == nil {
		messages = append(messages, "fkn.yaml already exists; leaving it unchanged")
	} else if os.IsNotExist(err) {
		if err := os.WriteFile(cfgPath, []byte(starterConfig), 0o644); err != nil {
			return "", err
		}
		messages = append(messages, "created fkn.yaml")
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

	return strings.Join(messages, "\n"), nil
}

func ensureGitignoreEntry(path, entry string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	lines := []string{}
	if len(raw) > 0 {
		lines = strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == entry {
				return false, nil
			}
		}
	}

	content := strings.TrimRight(string(raw), "\n")
	if content == "" {
		content = entry + "\n"
	} else {
		content = content + "\n" + entry + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
