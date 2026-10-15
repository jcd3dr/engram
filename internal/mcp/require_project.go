package mcp

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// projectScopedTools are the MCP tools that read or write a project's memory.
var projectScopedTools = map[string]bool{
	"mem_save":            true,
	"mem_search":          true,
	"mem_context":         true,
	"mem_session_summary": true,
	"mem_save_prompt":     true,
	"mem_timeline":        true,
	"mem_stats":           true,
	"mem_review":          true,
	"mem_doctor":          true,
}

// requireExplicitProjectMiddleware rejects project-scoped calls that do not
// name a project. It is installed only by the remote HTTP transport, where
// there is no meaningful cwd and no process-level default project.
func requireExplicitProjectMiddleware(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if projectScopedTools[req.Params.Name] {
			project, _ := req.GetArguments()["project"].(string)
			if strings.TrimSpace(project) == "" {
				return mcp.NewToolResultError(`project is required: this Engram server stores memory for many projects. Pass the project argument explicitly, for example project="my-repo".`), nil
			}
		}
		return next(ctx, req)
	}
}
