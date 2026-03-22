package mcp

import (
	"encoding/json"
	"testing"

	"github.com/neural-chilli/fkn/internal/config"
	"github.com/neural-chilli/fkn/internal/runner"
)

func TestHandlePayloadParseError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Tasks: map[string]config.Task{"test": {Desc: "Test", Cmd: "echo test"}}}
	server := New(cfg, t.TempDir(), runner.New(cfg, t.TempDir()))

	resp, notify, err := server.HandlePayload([]byte(`{"jsonrpc":`), nil)
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
	if payload.Error == nil || payload.Error.Code != -32700 {
		t.Fatalf("payload.Error = %#v, want parse error", payload.Error)
	}
}

func TestHandlePayloadToolsCallRequiresToolName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Tasks: map[string]config.Task{"test": {Desc: "Test", Cmd: "echo test"}}}
	server := New(cfg, t.TempDir(), runner.New(cfg, t.TempDir()))

	resp, notify, err := server.HandlePayload([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arguments":{}}}`), nil)
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
	if payload.Error == nil || payload.Error.Message != "tool name is required" {
		t.Fatalf("payload.Error = %#v, want missing tool name error", payload.Error)
	}
}

func TestHandlePayloadResourceReadRequiresURI(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Tasks: map[string]config.Task{"test": {Desc: "Test", Cmd: "echo test"}}}
	server := New(cfg, t.TempDir(), runner.New(cfg, t.TempDir()))

	resp, notify, err := server.HandlePayload([]byte(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{}}`), nil)
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
	if payload.Error == nil || payload.Error.Message != "resource uri is required" {
		t.Fatalf("payload.Error = %#v, want missing resource uri error", payload.Error)
	}
}
