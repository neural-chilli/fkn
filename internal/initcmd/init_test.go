package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
)

func TestRunCreatesStarterFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	msg, err := Run(dir, Options{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(msg, "created fkn.yaml") {
		t.Fatalf("Run() message = %q, want created fkn.yaml", msg)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(fkn.yaml) error = %v", err)
	}
	if !strings.Contains(string(cfg), "tasks:") {
		t.Fatalf("fkn.yaml = %q, want starter tasks", string(cfg))
	}
	if !strings.Contains(string(cfg), "default: check") {
		t.Fatalf("fkn.yaml = %q, want starter default task", string(cfg))
	}

	gitignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("ReadFile(.gitignore) error = %v", err)
	}
	if string(gitignore) != ".fkn/\n" {
		t.Fatalf(".gitignore = %q, want .fkn/ entry", string(gitignore))
	}
}

func TestRunIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	original := "project: keep-me\n\ntasks:\n  test:\n    desc: test\n    cmd: echo test\n"
	if err := os.WriteFile(filepath.Join(dir, "fkn.yaml"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("bin/\n.fkn/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir, Options{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(msg, "fkn.yaml already exists") {
		t.Fatalf("Run() message = %q, want existing config note", msg)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg) != original {
		t.Fatalf("fkn.yaml changed unexpectedly: %q", string(cfg))
	}
}

func TestRunFromRepoInfersGoTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/demo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir, Options{FromRepo: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(msg, "scaffolded tasks from existing repo files") {
		t.Fatalf("Run() message = %q, want inferred scaffold note", msg)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	if !strings.Contains(got, "project: "+filepath.Base(dir)) {
		t.Fatalf("fkn.yaml = %q, want inferred project name", got)
	}
	if !strings.Contains(got, "cmd: go test ./...") {
		t.Fatalf("fkn.yaml = %q, want Go test task", got)
	}
	if !strings.Contains(got, "default: check") {
		t.Fatalf("fkn.yaml = %q, want inferred default task", got)
	}
	if !strings.Contains(got, "cmd: go build ./...") {
		t.Fatalf("fkn.yaml = %q, want Go build task", got)
	}
	if !strings.Contains(got, "steps:\n      - test\n      - build") {
		t.Fatalf("fkn.yaml = %q, want check pipeline", got)
	}
	if !strings.Contains(got, "go.mod") {
		t.Fatalf("fkn.yaml = %q, want inferred watch paths", got)
	}
}

func TestRunFromRepoPrefersMakeTargets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	makefile := ".PHONY: test build check\n\ntest:\n\tgo test ./...\n\nbuild:\n\tgo build ./...\n\ncheck:\n\ttest build\n"
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	if !strings.Contains(got, "cmd: make test") {
		t.Fatalf("fkn.yaml = %q, want make-backed test task", got)
	}
	if !strings.Contains(got, "default: check") {
		t.Fatalf("fkn.yaml = %q, want inferred default task", got)
	}
	if !strings.Contains(got, "cmd: make build") {
		t.Fatalf("fkn.yaml = %q, want make-backed build task", got)
	}
	if !strings.Contains(got, "cmd: make check") {
		t.Fatalf("fkn.yaml = %q, want make-backed check task", got)
	}
}

func TestRunFromRepoInfersJustRecipesAndAliases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	justfile := `
alias b := build

build target profile="debug":
	echo {{target}} {{profile}}

test:
	just build app

[private]
hidden:
	echo hidden

_helper:
	echo helper
`
	if err := os.WriteFile(filepath.Join(dir, "justfile"), []byte(justfile), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	for _, want := range []string{
		"cmd: just build {{params.target}} {{params.profile}}",
		"cmd: just test",
		"aliases:\n  b: build\n",
		"target:\n        desc: Value for the target recipe parameter\n        env: TARGET\n        required: true\n",
		"profile:\n        desc: Value for the profile recipe parameter\n        env: PROFILE\n        default: \"debug\"\n",
		"default: check",
		"justfile",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want %q", got, want)
		}
	}
	for _, unwanted := range []string{"hidden:", "_helper:"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("fkn.yaml = %q, did not want private recipe %q", got, unwanted)
		}
	}
}

func TestRunFromRepoReadsCapitalizedJustfile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Justfile"), []byte("build:\n\techo build\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	if !strings.Contains(got, "cmd: just build") {
		t.Fatalf("fkn.yaml = %q, want Justfile-backed task", got)
	}
	if !strings.Contains(got, "Justfile") {
		t.Fatalf("fkn.yaml = %q, want Justfile watch path", got)
	}
}

func TestRunFromRepoInfersPackageScriptArgs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packageJSON := `{
  "scripts": {
    "build": "vite build --mode=$npm_config_mode",
    "test:e2e": "playwright test --project=$npm_config_project",
    "lint:fix": "eslint . --fix",
    "dev": "node ./scripts/dev.js"
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	for _, want := range []string{
		"cmd: npm run build -- --mode={{params.mode}}",
		"mode:\n        desc: Value for the mode package script argument\n        env: npm_config_mode\n        required: true\n",
		"test-e2e:\n    desc: Run the package.json test:e2e script\n    cmd: npm run test:e2e -- --project={{params.project}}\n",
		"project:\n        desc: Value for the project package script argument\n        env: npm_config_project\n        required: true\n",
		"dev:\n    desc: Run the package.json dev script\n    cmd: npm run dev\n",
		"lint-fix:\n    desc: Run the package.json lint:fix script\n    cmd: npm run lint:fix\n",
		"package.json",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want %q", got, want)
		}
	}
}

func TestRunFromRepoInfersNodeProcessEnvScriptArgs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packageJSON := `{
  "scripts": {
    "build": "node ./scripts/build.js --target=${process.env.npm_config_target}"
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	if !strings.Contains(got, "cmd: npm run build -- --target={{params.target}}") {
		t.Fatalf("fkn.yaml = %q, want npm_config target forwarding", got)
	}
	if !strings.Contains(got, "env: npm_config_target") {
		t.Fatalf("fkn.yaml = %q, want inferred npm_config env", got)
	}
}

func TestRunWithAgentsWritesCompanionFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	agents := "## Local Rules\n\nKeep this section.\n"
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir, Options{Agents: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(msg, "wrote AGENTS_FKN.md") {
		t.Fatalf("Run() message = %q, want AGENTS_FKN note", msg)
	}

	agentFKN, err := os.ReadFile(filepath.Join(dir, "AGENTS_FKN.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotAgentFKN := string(agentFKN)
	if !strings.Contains(gotAgentFKN, "# AGENTS_FKN") {
		t.Fatalf("AGENTS_FKN.md = %q, want header", gotAgentFKN)
	}
	if !strings.Contains(gotAgentFKN, "`test`: Run the test suite") {
		t.Fatalf("AGENTS_FKN.md = %q, want task summary", gotAgentFKN)
	}
	if !strings.Contains(gotAgentFKN, "## Context") {
		t.Fatalf("AGENTS_FKN.md = %q, want context section", gotAgentFKN)
	}
	if !strings.Contains(gotAgentFKN, "Command: `go test ./...`") {
		t.Fatalf("AGENTS_FKN.md = %q, want command detail", gotAgentFKN)
	}

	agentRoot, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	gotAgentRoot := string(agentRoot)
	if !strings.Contains(gotAgentRoot, "Keep this section.") {
		t.Fatalf("AGENTS.md = %q, want original content preserved", gotAgentRoot)
	}
	if !strings.Contains(gotAgentRoot, "## fkn Workflow") {
		t.Fatalf("AGENTS.md = %q, want fkn workflow block", gotAgentRoot)
	}
}

func TestRunWithAgentsIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := Run(dir, Options{Agents: true}); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := Run(dir, Options{Agents: true})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if !strings.Contains(msg, "AGENTS.md already includes fkn guidance") {
		t.Fatalf("Run() message = %q, want existing guidance note", msg)
	}
	after, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("AGENTS.md changed unexpectedly on second run:\nBEFORE:\n%s\nAFTER:\n%s", string(before), string(after))
	}
}

func TestRenderAgentsFKNIncludesScopesAndPrompts(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Project:     "demo",
		Description: "Demo repo",
		Tasks: map[string]config.Task{
			"check": {
				Desc:  "Run checks",
				Steps: []string{"test", "build"},
				Scope: "backend",
			},
			"test": {
				Desc: "Run tests",
				Cmd:  "go test ./...",
			},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test", "build"}},
		},
		Scopes: map[string][]string{
			"backend": {"cmd/", "internal/"},
		},
		Prompts: map[string]config.Prompt{
			"continue-backend": {Desc: "Continue backend work"},
		},
		Context: config.ContextConfig{
			AgentFiles: []string{"README.md"},
			Include:    []string{"cmd/", "internal/"},
		},
		Serve: config.ServeConfig{
			Transport: "stdio",
			Port:      8080,
		},
		Watch: config.WatchConfig{
			DebounceMS: 500,
			Paths:      []string{"cmd/", "internal/"},
		},
	}

	got, err := renderAgentsFKN(cfg)
	if err != nil {
		t.Fatalf("renderAgentsFKN() error = %v", err)
	}
	for _, want := range []string{
		"## Scopes",
		"`backend`: `cmd/`, `internal/`",
		"## Prompts",
		"`continue-backend`: Continue backend work",
		"Scope: `backend`",
		"Steps: `test`, `build`",
		"## MCP",
		"## Watch",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderAgentsFKN() = %q, want %q", got, want)
		}
	}
}

func TestRunFromRepoIncludesMostMakeTargets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	makefile := ".PHONY: tidy fmt lint vet vulncheck verify verify-strict check build run clean add-feature add-feature-git codemap-check codemap-sync map ci-init test\n\n" +
		"tidy:\n\tgo mod tidy\n\n" +
		"fmt:\n\tgofmt -w .\n\n" +
		"lint:\n\tgolangci-lint run\n\n" +
		"vet:\n\tgo vet ./...\n\n" +
		"vulncheck:\n\tgovulncheck ./...\n\n" +
		"verify:\n\tketuu verify\n\n" +
		"verify-strict:\n\tketuu verify --strict\n\n" +
		"codemap-check:\n\tketuu verify --codemap\n\n" +
		"codemap-sync:\n\tketuu codemap sync\n\n" +
		"map:\n\tketuu map\n\n" +
		"ci-init:\n\tketuu ci init --github\n\n" +
		"check:\n\tgo test ./...\n\n" +
		"build:\n\tgo build ./...\n\n" +
		"run:\n\t./bin/demo\n\n" +
		"test:\n\tgo test ./...\n\n" +
		"add-feature:\n\t@if [ -z \"$(FEATURE)\" ]; then echo missing; exit 1; fi\n"
	if err := os.WriteFile(filepath.Join(dir, "Makefile"), []byte(makefile), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	for _, want := range []string{
		"  tidy:\n",
		"  fmt:\n",
		"  lint:\n",
		"  vet:\n",
		"  vulncheck:\n",
		"  verify:\n",
		"  verify-strict:\n",
		"  codemap-check:\n",
		"  codemap-sync:\n",
		"  map:\n",
		"  ci-init:\n",
		"  run:\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want inferred target %q", got, want)
		}
	}
	for _, unwanted := range []string{
		"  GO:\n",
		"  APP_NAME:\n",
		"  KETUU:\n",
		"  BIN:\n",
		"  add-feature-git:\n",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("fkn.yaml = %q, did not want inferred target %q", got, unwanted)
		}
	}
	if !strings.Contains(got, "  add-feature:\n") {
		t.Fatalf("fkn.yaml = %q, want parameterized add-feature target", got)
	}
	if !strings.Contains(got, "      feature:\n") || !strings.Contains(got, "        env: FEATURE\n") {
		t.Fatalf("fkn.yaml = %q, want inferred FEATURE param", got)
	}
	for _, want := range []string{
		"  add-feature:\n    desc: Run the repository add-feature target\n    cmd: make add-feature\n    agent: false\n",
		"  codemap-sync:\n    desc: Run the repository codemap-sync target\n    cmd: make codemap-sync\n    agent: false\n",
		"  ci-init:\n    desc: Run the repository ci-init target\n    cmd: make ci-init\n    agent: false\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want agent-safe helper target %q", got, want)
		}
	}
}

func TestRunFromRepoMarksParameterizedPackageHelpersAgentFalse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	packageJSON := `{
  "scripts": {
    "release": "node ./scripts/release.js --version=$npm_config_version",
    "build": "vite build"
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Run(dir, Options{FromRepo: true}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, "fkn.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(cfg)
	if !strings.Contains(got, "release:\n    desc: Run the package.json release script\n    cmd: npm run release -- --version={{params.version}}\n    agent: false\n") {
		t.Fatalf("fkn.yaml = %q, want release helper marked agent:false", got)
	}
	if !strings.Contains(got, "build:\n    desc: Run the package.json build script\n    cmd: npm run build\n") {
		t.Fatalf("fkn.yaml = %q, want regular build script", got)
	}
}
