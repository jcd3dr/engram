# Repository Map

> DARS Standard v1.17

## Significant structure

| Path / area | Responsibility | Important relationships |
|---|---|---|
| `cmd/engram/main.go` | CLI entrypoint and command dispatch | Dispatches `mcp` to stdio (`cmdMCP`) or HTTP (`cmdMCPHTTP`) |
| `cmd/engram/http_transport.go` | Fork addition: Streamable HTTP MCP entrypoint | Uses `internal/mcp` and `internal/remoteauth`; shares the store with stdio |
| `cmd/engram/cloud*.go` | `engram cloud` subcommands | Unrelated to MCP transport |
| `internal/mcp/mcp.go` | MCP tool registration and handlers (22 tools) | Shared by stdio and HTTP. Must not be changed for transport work |
| `internal/store/` | SQLite store, FTS5 search, lifecycle, sync journal | Shared by every entrypoint. Must not be changed for transport work |
| `internal/server/` | Local HTTP API (`engram serve`) | Separate from the remote MCP |
| `internal/remoteauth/` | Fork addition: OAuth authorization server for remote MCP | Used only by `http_transport.go` |
| `internal/cloud/` | `engram cloud serve`: Postgres sync REST, dashboard | Separate subsystem |
| `internal/project/` | Project name detection and similarity | Used by MCP and CLI |
| `internal/setup/` | Agent plugin installer | Writes local plugin and MCP config |
| `internal/tui/` | Terminal UI | Reads the store |
| `plugin/` | Agent plugins: claude-code, opencode, pi, codex, gemini, others | Local protocol and hooks |
| `skills/` | Repository skills index and skill files | Indexed in `AGENTS.md` |
| `docker/mcp/Dockerfile` | Production image for the remote MCP | Built by Coolify from `main` |
| `docker/cloud/Dockerfile` | Image for `engram cloud serve` | Not used by `engram-mcp` |
| `docs/` | Project documentation | See knowledge map below |

## Repository knowledge map

| Path / record | Short description | Read when |
|---|---|---|
| `AGENTS.md` | Repository-wide agent entry point | Always first |
| `docs/SPEC.md` | Product purpose, scope, requirements | Changing behavior or scope |
| `docs/PLAN.md` | Current focus, blockers, next steps | Resuming work |
| `docs/CONVENTIONS.md` | Engineering and contribution rules | Before any implementation |
| `docs/GOVERNANCE_SUMMARY.md` | DARS operating rules | When the governance skill is unavailable |
| `docs/REMOTE-MCP.md` | Remote transport endpoints, env vars, auth design | Changing `http_transport.go` or `internal/remoteauth` |
| `docs/COOLIFY.md` | Deployment steps, persistence gotcha, connector setup | Deploying or reconfiguring Coolify |
| `docs/PRODUCTION.md` | Operational reference: architecture, backup, rollback | Operating production |
| `docs/CANONICAL-STANDARDS.md` | Standards migrated into Engram | Looking for DadeCore standards |
| `docs/adr/0000-baseline.md` | Reconciled starting point and known gaps | Before any change in the fork area |
| `CHANGELOG.md` | Project-visible changes | Publishing a release or user-visible change |

## Navigation notes

- The fork's transport work is additive. Its files are `cmd/engram/http_transport.go`, `internal/remoteauth/`, `docker/mcp/`, and the `docs/` files listed above.
- Upstream `Gentleman-Programming/engram` remains the reference for everything else. Compare against the `upstream-origin` remote before assuming a file is fork-specific.
