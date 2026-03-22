package initcmd

import "path/filepath"

func findRustTasks(repoRoot string) []inferredTask {
	if !hasFile(repoRoot, "Cargo.toml") {
		return nil
	}

	return []inferredTask{
		{Name: "fmt", Desc: "Format the Rust workspace", Cmd: "cargo fmt --all", Safety: "idempotent"},
		{Name: "lint", Desc: "Run clippy across the Rust workspace", Cmd: "cargo clippy --all-targets --all-features -- -D warnings", Safety: "idempotent"},
		{Name: "test", Desc: "Run the Rust test suite", Cmd: "cargo test", Safety: "idempotent"},
		{Name: "build", Desc: "Build the Rust workspace", Cmd: "cargo build", Safety: "idempotent"},
	}
}

func rustWatchPaths(repoRoot string) []string {
	if !hasFile(repoRoot, "Cargo.toml") {
		return nil
	}

	paths := []string{"Cargo.toml"}
	for _, dir := range []string{"src", "crates"} {
		if hasDir(repoRoot, dir) {
			paths = append(paths, filepath.ToSlash(dir)+"/")
		}
	}
	return paths
}
