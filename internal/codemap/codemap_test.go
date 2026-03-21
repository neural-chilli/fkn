package codemap

import (
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
)

func TestExplainPackageAndEntryPoint(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"check": {Desc: "Run checks", Cmd: "echo ok", Scope: "cli"},
		},
		Scopes: map[string][]string{
			"cli": {"cmd/fkn/", "internal/runner/"},
		},
		Codemap: config.CodemapConfig{
			Packages: map[string]config.CodemapPackage{
				"internal/runner": {
					Desc:        "Execution engine",
					KeyTypes:    []string{"Runner"},
					EntryPoints: []string{"Runner.Run"},
				},
			},
			Glossary: map[string]string{"guard": "Validation-oriented pipeline"},
		},
	}

	pkg, err := Explain(cfg, "internal/runner")
	if err != nil {
		t.Fatalf("Explain(package) error = %v", err)
	}
	if pkg.Kind != "package" || pkg.Package == nil || pkg.Package.Name != "internal/runner" {
		t.Fatalf("Explain(package) = %+v, want package explanation", pkg)
	}

	entry, err := Explain(cfg, "Runner.Run")
	if err != nil {
		t.Fatalf("Explain(entry point) error = %v", err)
	}
	if entry.Kind != "package" || entry.Package == nil || entry.Package.Name != "internal/runner" {
		t.Fatalf("Explain(entry point) = %+v, want package explanation", entry)
	}

	glossary, err := Explain(cfg, "guard")
	if err != nil {
		t.Fatalf("Explain(glossary) error = %v", err)
	}
	if glossary.Kind != "glossary" || glossary.Glossary == nil {
		t.Fatalf("Explain(glossary) = %+v, want glossary explanation", glossary)
	}

	rendered := RenderRelevantPackages(RelevantPackages(cfg, cfg.Scopes["cli"]))
	if !strings.Contains(rendered, "internal/runner") {
		t.Fatalf("RenderRelevantPackages() = %q, want matching package", rendered)
	}
}
