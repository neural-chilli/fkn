package mcp

import (
	"encoding/json"
	"testing"

	"fkn/internal/config"
	"fkn/internal/runner"
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
