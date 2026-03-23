package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		"      app-name:\n",
		"      go:\n",
		"      ketuu:\n",
		"      bin:\n",
		"  add-feature-git:\n",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("fkn.yaml = %q, did not want inferred target %q", got, unwanted)
		}
	}
	if strings.Contains(got, "build:\n    desc: Run the repository build target\n    cmd: make build\n    agent: false\n") {
		t.Fatalf("fkn.yaml = %q, did not want build target treated as parameterized helper", got)
	}
	if !strings.Contains(got, "  add-feature:\n") {
		t.Fatalf("fkn.yaml = %q, want parameterized add-feature target", got)
	}
	if !strings.Contains(got, "      feature:\n") || !strings.Contains(got, "        env: FEATURE\n") {
		t.Fatalf("fkn.yaml = %q, want inferred FEATURE param", got)
	}
	for _, want := range []string{
		"  add-feature:\n    desc: Run the repository add-feature target\n    cmd: make add-feature\n    agent: false\n    safety: destructive\n",
		"  codemap-sync:\n    desc: Run the repository codemap-sync target\n    cmd: make codemap-sync\n    agent: false\n    safety: external\n",
		"  ci-init:\n    desc: Run the repository ci-init target\n    cmd: make ci-init\n    agent: false\n    safety: destructive\n",
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
	if !strings.Contains(got, "release:\n    desc: Run the package.json release script\n    cmd: npm run release -- --version={{params.version}}\n    agent: false\n    safety: external\n") {
		t.Fatalf("fkn.yaml = %q, want release helper marked agent:false", got)
	}
	if !strings.Contains(got, "build:\n    desc: Run the package.json build script\n    cmd: npm run build\n") {
		t.Fatalf("fkn.yaml = %q, want regular build script", got)
	}
}

func TestRunFromRepoInfersRustTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cargo := `[package]
name = "demo"
version = "0.1.0"
edition = "2021"
`
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "src"), 0o755); err != nil {
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
		"fmt:\n    desc: Format the Rust workspace\n    cmd: cargo fmt --all\n    safety: idempotent\n",
		"lint:\n    desc: Run clippy across the Rust workspace\n    cmd: cargo clippy --all-targets --all-features -- -D warnings\n    safety: idempotent\n",
		"test:\n    desc: Run the Rust test suite\n    cmd: cargo test\n    safety: idempotent\n",
		"build:\n    desc: Build the Rust workspace\n    cmd: cargo build\n    safety: idempotent\n",
		"default: check",
		"Cargo.toml",
		"src/",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want %q", got, want)
		}
	}
}

func TestRunFromRepoInfersPythonTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pyproject := `[build-system]
requires = ["setuptools"]
build-backend = "setuptools.build_meta"

[tool.pytest.ini_options]
testpaths = ["tests"]

[tool.ruff]
line-length = 100
`
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "tests"), 0o755); err != nil {
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
		"test:\n    desc: Run the Python test suite\n    cmd: pytest\n    safety: idempotent\n",
		"build:\n    desc: Build the Python package\n    cmd: python -m build\n    safety: idempotent\n",
		"lint:\n    desc: Run Ruff checks\n    cmd: ruff check .\n    safety: idempotent\n",
		"fmt:\n    desc: Format the codebase with Ruff\n    cmd: ruff format .\n    safety: idempotent\n",
		"pyproject.toml",
		"tests/",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want %q", got, want)
		}
	}
}

func TestRunFromRepoPrefersToxForPythonTests(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[build-system]\nrequires = [\"setuptools\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tox.ini"), []byte("[tox]\nenvlist = py\n"), 0o644); err != nil {
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
	if !strings.Contains(got, "test:\n    desc: Run the Python test environments\n    cmd: tox\n    safety: idempotent\n") {
		t.Fatalf("fkn.yaml = %q, want tox-backed test task", got)
	}
	if !strings.Contains(got, "tox.ini") {
		t.Fatalf("fkn.yaml = %q, want tox.ini watch path", got)
	}
}

func TestRunFromRepoInfersComposeTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	compose := `services:
  db:
    image: postgres:16
`
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0o644); err != nil {
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
		"compose-up:\n    desc: Start the Docker Compose services\n    cmd: docker compose up -d\n    agent: false\n    safety: external\n",
		"compose-down:\n    desc: Stop the Docker Compose services\n    cmd: docker compose down\n    agent: false\n    safety: external\n",
		"compose-logs:\n    desc: Stream Docker Compose service logs\n    cmd: docker compose logs -f\n    agent: false\n    safety: external\n",
		"docker-compose.yml",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want %q", got, want)
		}
	}
}

func TestRunFromRepoInfersMavenTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pom := `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>demo</artifactId>
  <version>1.0.0</version>
</project>
`
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(pom), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "src"), 0o755); err != nil {
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
		"test:\n    desc: Run the Maven test suite\n    cmd: mvn test\n    safety: idempotent\n",
		"build:\n    desc: Build the Maven project\n    cmd: mvn package\n    safety: idempotent\n",
		"default: check",
		"pom.xml",
		"src/",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want %q", got, want)
		}
	}
}

func TestRunFromRepoInfersGradleTasks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "build.gradle"), []byte("plugins { id 'java' }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gradlew"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "app"), 0o755); err != nil {
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
		"test:\n    desc: Run the Gradle test suite\n    cmd: ./gradlew test\n    safety: idempotent\n",
		"build:\n    desc: Build the Gradle project\n    cmd: ./gradlew build\n    safety: idempotent\n",
		"build.gradle",
		"gradlew",
		"app/",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("fkn.yaml = %q, want %q", got, want)
		}
	}
}
