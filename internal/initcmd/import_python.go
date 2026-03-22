package initcmd

import (
	"os"
	"path/filepath"
	"strings"
)

func findPythonTasks(repoRoot string) []inferredTask {
	hasPyproject := hasFile(repoRoot, "pyproject.toml")
	hasTox := hasFile(repoRoot, "tox.ini")
	if !hasPyproject && !hasTox {
		return nil
	}

	content := readOptionalFile(filepath.Join(repoRoot, "pyproject.toml"))
	tasks := []inferredTask{}
	seen := map[string]bool{}
	add := func(task inferredTask) {
		if seen[task.Name] {
			return
		}
		seen[task.Name] = true
		tasks = append(tasks, task)
	}

	if hasTox {
		add(inferredTask{Name: "test", Desc: "Run the Python test environments", Cmd: "tox", Safety: "idempotent"})
	}
	if strings.Contains(content, "[tool.pytest.ini_options]") || strings.Contains(content, "pytest") {
		add(inferredTask{Name: "test", Desc: "Run the Python test suite", Cmd: "pytest", Safety: "idempotent"})
	}
	if strings.Contains(content, "[build-system]") {
		add(inferredTask{Name: "build", Desc: "Build the Python package", Cmd: "python -m build", Safety: "idempotent"})
	}
	if strings.Contains(content, "[tool.ruff") {
		add(inferredTask{Name: "lint", Desc: "Run Ruff checks", Cmd: "ruff check .", Safety: "idempotent"})
		add(inferredTask{Name: "fmt", Desc: "Format the codebase with Ruff", Cmd: "ruff format .", Safety: "idempotent"})
	}
	if strings.Contains(content, "[tool.black]") {
		add(inferredTask{Name: "fmt", Desc: "Format the codebase with Black", Cmd: "black .", Safety: "idempotent"})
	}

	return tasks
}

func pythonWatchPaths(repoRoot string) []string {
	paths := []string{}
	if hasFile(repoRoot, "pyproject.toml") {
		paths = append(paths, "pyproject.toml")
	}
	if hasFile(repoRoot, "tox.ini") {
		paths = append(paths, "tox.ini")
	}
	for _, dir := range []string{"src", "tests"} {
		if hasDir(repoRoot, dir) {
			paths = append(paths, filepath.ToSlash(dir)+"/")
		}
	}
	return paths
}

func readOptionalFile(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}
