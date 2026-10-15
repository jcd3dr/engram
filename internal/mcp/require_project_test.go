package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestRequireExplicitProjectMiddleware(t *testing.T) {
	called := false
	var next server.ToolHandlerFunc = func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("ok"), nil
	}
	h := requireExplicitProjectMiddleware(next)

	call := func(name string, args map[string]any) (*mcp.CallToolResult, bool) {
		called = false
		var req mcp.CallToolRequest
		req.Params.Name = name
		req.Params.Arguments = args
		res, _ := h(context.Background(), req)
		return res, called
	}

	if res, ran := call("mem_save", map[string]any{}); !res.IsError || ran {
		t.Fatalf("mem_save without project must be rejected before the handler runs")
	}
	if res, ran := call("mem_search", map[string]any{"project": "   "}); !res.IsError || ran {
		t.Fatalf("blank project must be rejected")
	}
	if res, ran := call("mem_save", map[string]any{"project": "engram"}); res.IsError || !ran {
		t.Fatalf("mem_save with explicit project must reach the handler")
	}
	if res, ran := call("mem_get_observation", map[string]any{}); res.IsError || !ran {
		t.Fatalf("non project-scoped tool must not require a project")
	}
}
