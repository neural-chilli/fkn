package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/neural-chilli/fkn/internal/config"
	"github.com/neural-chilli/fkn/internal/runner"
)

const ProtocolVersion = "2024-11-05"

type Server struct {
	cfg      *config.Config
	repoRoot string
	runner   *runner.Runner
}

type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id,omitempty"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func New(cfg *config.Config, repoRoot string, taskRunner *runner.Runner) *Server {
	return &Server{cfg: cfg, repoRoot: repoRoot, runner: taskRunner}
}

func (s *Server) Tools() []Tool {
	names := make([]string, 0, len(s.cfg.Tasks))
	for name := range s.cfg.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)

	tools := make([]Tool, 0, len(names))
	for _, name := range names {
		task := s.cfg.Tasks[name]
		if !task.AgentEnabled() {
			continue
		}
		properties := map[string]any{
			"env": map[string]any{
				"type":        "object",
				"description": "Optional env var overrides for this invocation",
			},
			"dry_run": map[string]any{
				"type":        "boolean",
				"description": "Print the command without executing it",
			},
		}
		required := []string{}
		for _, paramName := range sortedParamNames(task.Params) {
			param := task.Params[paramName]
			prop := map[string]any{
				"type":        "string",
				"description": param.Desc,
			}
			if param.Default != "" {
				prop["default"] = param.Default
			}
			properties[paramName] = prop
			if param.Required {
				required = append(required, paramName)
			}
		}
		inputSchema := map[string]any{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			inputSchema["required"] = required
		}
		tools = append(tools, Tool{
			Name:        name,
			Description: task.Desc,
			InputSchema: inputSchema,
		})
	}
	return tools
}

func (s *Server) HandlePayload(payload []byte, errOut io.Writer) ([]byte, bool, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		resp, marshalErr := json.Marshal(JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32700, Message: "parse error"},
		})
		if marshalErr != nil {
			return nil, false, marshalErr
		}
		return resp, false, nil
	}

	if len(req.ID) == 0 {
		_ = s.handleRequest(req, errOut)
		return nil, true, nil
	}

	resp := s.handleRequest(req, errOut)
	raw, err := json.Marshal(resp)
	return raw, false, err
}

func (s *Server) handleRequest(req JSONRPCRequest, errOut io.Writer) JSONRPCResponse {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: rawID(req.ID)}

	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "fkn",
				"version": "dev",
			},
		}
	case "notifications/initialized":
		return JSONRPCResponse{}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": s.Tools()}
	case "tools/call":
		result, err := s.callTool(req.Params)
		if err != nil {
			resp.Error = &JSONRPCError{Code: -32000, Message: err.Error()}
			return resp
		}
		resp.Result = result
	default:
		resp.Error = &JSONRPCError{Code: -32601, Message: fmt.Sprintf("method %q not found", req.Method)}
	}

	_ = errOut
	return resp
}

func (s *Server) callTool(raw json.RawMessage) (map[string]any, error) {
	var params toolCallParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("invalid tool call params: %w", err)
	}
	if params.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	env := map[string]string{}
	dryRun := false
	paramValues := map[string]string{}
	task, ok := s.cfg.Tasks[params.Name]
	if !ok {
		return nil, fmt.Errorf("unknown task %q", params.Name)
	}
	for key, value := range params.Arguments {
		switch key {
		case "dry_run":
			if boolValue, ok := value.(bool); ok {
				dryRun = boolValue
			}
		case "env":
			if rawMap, ok := value.(map[string]any); ok {
				for envKey, envValue := range rawMap {
					env[envKey] = fmt.Sprint(envValue)
				}
			}
		default:
			if _, ok := task.Params[key]; ok {
				paramValues[key] = fmt.Sprint(value)
			}
		}
	}

	result, err := s.runner.Run(params.Name, runner.Options{
		DryRun: dryRun,
		Stdout: io.Discard,
		Stderr: io.Discard,
		Env:    env,
		Params: paramValues,
	})
	if err != nil {
		return nil, err
	}

	text := fmt.Sprintf("status=%s exit_code=%d duration_ms=%d", result.Status, result.ExitCode, result.DurationMS)
	if result.Stdout != "" {
		text += "\n\nstdout:\n" + result.Stdout
	}
	if result.Stderr != "" {
		text += "\n\nstderr:\n" + result.Stderr
	}

	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"structuredContent": map[string]any{
			"task":        result.Task,
			"status":      result.Status,
			"stdout":      result.Stdout,
			"stderr":      result.Stderr,
			"exit_code":   result.ExitCode,
			"duration_ms": result.DurationMS,
			"started_at":  result.StartedAt,
			"finished_at": result.FinishedAt,
		},
		"isError": result.ExitCode != 0,
	}, nil
}

func sortedParamNames(params map[string]config.Param) []string {
	names := make([]string, 0, len(params))
	for name := range params {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func rawID(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(id, &v); err != nil {
		return string(id)
	}
	return v
}
