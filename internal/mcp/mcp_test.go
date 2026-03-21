package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
	"github.com/neural-chilli/fkn/internal/runner"
)

func TestToolsSkipsAgentFalseTasks(t *testing.T) {
	t.Parallel()

	agentFalse := false
	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"build": {Desc: "Build", Cmd: "echo build"},
			"fmt":   {Desc: "Format", Cmd: "echo fmt", Agent: &agentFalse},
		},
	}

	server := New(cfg, t.TempDir(), runner.New(cfg, t.TempDir()))
	tools := server.Tools()

	if len(tools) != 1 {
		t.Fatalf("len(Tools()) = %d, want 1", len(tools))
	}
	if tools[0].Name != "build" {
		t.Fatalf("Tools()[0].Name = %q, want build", tools[0].Name)
	}
}

func TestHandlePayloadToolsList(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {Desc: "Test", Cmd: "echo test"},
		},
	}
	server := New(cfg, t.TempDir(), runner.New(cfg, t.TempDir()))

	resp, notify, err := server.HandlePayload([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`), nil)
	if err != nil {
		t.Fatalf("HandlePayload() error = %v", err)
	}
	if notify {
		t.Fatal("HandlePayload() notify = true, want false")
	}

	var payload JSONRPCResponse
	if err := json.Unmarshal(resp, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Error != nil {
		t.Fatalf("payload.Error = %+v, want nil", payload.Error)
	}
}

func TestHandlePayloadInitializeIncludesInstructions(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Project:     "demo",
		Description: "Demo repository",
		Tasks: map[string]config.Task{
			"test": {Desc: "Test", Cmd: "echo test"},
		},
		Guards: map[string]config.Guard{
			"default": {Steps: []string{"test"}},
		},
	}
	server := New(cfg, t.TempDir(), runner.New(cfg, t.TempDir()))

	resp, notify, err := server.HandlePayload([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`), nil)
	if err != nil {
		t.Fatalf("HandlePayload() error = %v", err)
	}
	if notify {
		t.Fatal("HandlePayload() notify = true, want false")
	}

	var payload JSONRPCResponse
	if err := json.Unmarshal(resp, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	result := payload.Result.(map[string]any)
	instructions, ok := result["instructions"].(string)
	if !ok || instructions == "" {
		t.Fatalf("result.instructions = %#v, want non-empty string", result["instructions"])
	}
	if !strings.Contains(instructions, "Project: demo") || !strings.Contains(instructions, "fkn guard") {
		t.Fatalf("instructions = %q, want project and guard guidance", instructions)
	}
}

func TestHandlePayloadToolsCallDryRun(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {Desc: "Test", Cmd: "echo test"},
		},
	}
	repoRoot := t.TempDir()
	server := New(cfg, repoRoot, runner.New(cfg, repoRoot))

	resp, notify, err := server.HandlePayload([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test","arguments":{"dry_run":true,"env":{"FOO":"bar"}}}}`), nil)
	if err != nil {
		t.Fatalf("HandlePayload() error = %v", err)
	}
	if notify {
		t.Fatal("HandlePayload() notify = true, want false")
	}

	var payload JSONRPCResponse
	if err := json.Unmarshal(resp, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Error != nil {
		t.Fatalf("payload.Error = %+v, want nil", payload.Error)
	}
}

func TestHandlePayloadToolsCallIncludesStructuredErrors(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {
				Desc:        "Test",
				Cmd:         `printf '%s\n' 'internal/mcp/mcp_test.go:88: broken' >&2; exit 1`,
				ErrorFormat: "go_test",
			},
		},
	}
	repoRoot := t.TempDir()
	server := New(cfg, repoRoot, runner.New(cfg, repoRoot))

	resp, notify, err := server.HandlePayload([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test","arguments":{}}}`), nil)
	if err != nil {
		t.Fatalf("HandlePayload() error = %v", err)
	}
	if notify {
		t.Fatal("HandlePayload() notify = true, want false")
	}

	var payload JSONRPCResponse
	if err := json.Unmarshal(resp, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Error != nil {
		t.Fatalf("payload.Error = %+v, want nil", payload.Error)
	}
	result := payload.Result.(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	rawErrors := structured["errors"].([]any)
	if len(rawErrors) != 1 {
		t.Fatalf("len(errors) = %d, want 1", len(rawErrors))
	}
	entry := rawErrors[0].(map[string]any)
	if entry["file"] != "internal/mcp/mcp_test.go" || entry["message"] != "broken" {
		t.Fatalf("errors[0] = %#v, want parsed error", entry)
	}
}

func TestToolsExposeTaskParams(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"add-feature": {
				Desc: "Add feature",
				Cmd:  "make add-feature",
				Params: map[string]config.Param{
					"feature": {
						Desc:     "Feature name",
						Env:      "FEATURE",
						Required: true,
					},
				},
			},
		},
	}

	server := New(cfg, t.TempDir(), runner.New(cfg, t.TempDir()))
	tools := server.Tools()
	properties := tools[0].InputSchema["properties"].(map[string]any)
	if _, ok := properties["feature"]; !ok {
		t.Fatalf("properties = %#v, want feature param", properties)
	}
	required := tools[0].InputSchema["required"].([]string)
	if len(required) != 1 || required[0] != "feature" {
		t.Fatalf("required = %#v, want feature", required)
	}
}

func TestToolsExposeSafetyAnnotations(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"deploy": {
				Desc:   "Deploy",
				Cmd:    "./scripts/deploy.sh",
				Safety: "external",
			},
		},
	}

	server := New(cfg, t.TempDir(), runner.New(cfg, t.TempDir()))
	tools := server.Tools()
	if len(tools) != 1 {
		t.Fatalf("len(Tools()) = %d, want 1", len(tools))
	}
	annotations := tools[0].Annotations
	if annotations["fknSafety"] != "external" {
		t.Fatalf("annotations = %#v, want fknSafety external", annotations)
	}
	if annotations["openWorldHint"] != true {
		t.Fatalf("annotations = %#v, want openWorldHint", annotations)
	}
}

func TestHandlePayloadResourcesList(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Tasks: map[string]config.Task{
			"test": {Desc: "Test", Cmd: "echo test"},
		},
		Scopes: map[string]config.Scope{
			"cli": {Desc: "CLI and internal implementation paths", Paths: []string{"cmd/", "internal/"}},
		},
	}
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".fkn"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".fkn", "last-guard.json"), []byte(`{"guard":"default"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	server := New(cfg, repoRoot, runner.New(cfg, repoRoot))

	resp, notify, err := server.HandlePayload([]byte(`{"jsonrpc":"2.0","id":1,"method":"resources/list","params":{}}`), nil)
	if err != nil {
		t.Fatalf("HandlePayload() error = %v", err)
	}
	if notify {
		t.Fatal("HandlePayload() notify = true, want false")
	}

	var payload JSONRPCResponse
	if err := json.Unmarshal(resp, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Error != nil {
		t.Fatalf("payload.Error = %+v, want nil", payload.Error)
	}
	result := payload.Result.(map[string]any)
	rawResources := result["resources"].([]any)
	if len(rawResources) < 3 {
		t.Fatalf("resources = %#v, want context/scope/guard resources", rawResources)
	}
	foundScope := false
	for _, raw := range rawResources {
		resource := raw.(map[string]any)
		if resource["uri"] == "fkn://scope/cli" {
			foundScope = true
			if resource["description"] != "CLI and internal implementation paths" {
				t.Fatalf("scope resource = %#v, want scope description", resource)
			}
		}
	}
	if !foundScope {
		t.Fatalf("resources = %#v, want cli scope resource", rawResources)
	}
}

func TestHandlePayloadResourcesRead(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".fkn"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".fkn", "last-guard.json"), []byte(`{"guard":"default"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Project: "demo",
		Tasks: map[string]config.Task{
			"test": {Desc: "Test", Cmd: "echo test"},
		},
		Scopes: map[string]config.Scope{
			"cli": {Desc: "CLI docs and task-facing files", Paths: []string{"README.md"}},
		},
		Context: config.ContextConfig{
			Files: []string{"README.md"},
			Caps: config.ContextCaps{
				FileLines:       10,
				FilesMax:        5,
				FileTreeEntries: 10,
				DependencyLines: 10,
				GitLogLines:     10,
			},
		},
	}
	server := New(cfg, repoRoot, runner.New(cfg, repoRoot))

	resp, notify, err := server.HandlePayload([]byte(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"fkn://scope/cli"}}`), nil)
	if err != nil {
		t.Fatalf("HandlePayload() error = %v", err)
	}
	if notify {
		t.Fatal("HandlePayload() notify = true, want false")
	}

	var payload JSONRPCResponse
	if err := json.Unmarshal(resp, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Error != nil {
		t.Fatalf("payload.Error = %+v, want nil", payload.Error)
	}
	result := payload.Result.(map[string]any)
	contents := result["contents"].([]any)
	if len(contents) != 1 {
		t.Fatalf("contents = %#v, want 1 item", contents)
	}
	item := contents[0].(map[string]any)
	for _, want := range []string{"README.md", "CLI docs and task-facing files"} {
		if !strings.Contains(item["text"].(string), want) {
			t.Fatalf("resource text = %#v, want %q", item, want)
		}
	}
}
