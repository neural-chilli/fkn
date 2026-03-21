package main

import (
	"os"
	"runtime/debug"
)

var version = "dev"

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		cfg, _, err := loadConfig()
		if err == nil && cfg.Default != "" {
			return runTask([]string{cfg.Default}, stdout, stderr)
		}
		printUsage(stdout)
		return 0
	}

	if args[0] == "--version" {
		_, _ = stdout.WriteString("fkn " + resolvedVersion() + "\n")
		return 0
	}

	switch args[0] {
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "help":
		return runHelp(args[1:], stdout, stderr)
	case "docs":
		return runDocs(args[1:], stdout, stderr)
	case "explain":
		return runExplain(args[1:], stdout, stderr)
	case "guard":
		return runGuard(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "watch":
		return runWatch(args[1:], stdout, stderr)
	case "context":
		return runContext(args[1:], stdout, stderr)
	case "prompt":
		return runPrompt(args[1:], stdout, stderr)
	case "plan":
		return runPlan(args[1:], stdout, stderr)
	case "diff-plan":
		return runDiffPlan(args[1:], stdout, stderr)
	case "scope":
		return runScope(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "repair":
		return runRepair(args[1:], stdout, stderr)
	default:
		return runTask(args, stdout, stderr)
	}
}

func resolvedVersion() string {
	if version != "" && version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	var revision string
	var modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision == "" {
		return version
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	if modified == "true" {
		return revision + "-dirty"
	}
	return revision
}
