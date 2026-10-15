# Remote MCP (Streamable HTTP transport)

DadeCore fork addition. Engram's stdio MCP mode is unchanged; this adds a
second transport so Engram can be exposed over HTTPS and added as a
Remote MCP connector in Claude and ChatGPT.

## Enabling

```
engram mcp --transport=http
```

or set `ENGRAM_MCP_TRANSPORT=http` (any of `--transport=http`,
`ENGRAM_MCP_TRANSPORT=http` triggers HTTP mode; default remains stdio).

## Required environment variables

| Var | Purpose |
|---|---|
| `ENGRAM_OWNER_KEY` | Secret compared (constant-time) against the owner-key gate shown on `/authorize`. Mandatory first auth gate per DadeCore's `dadecore/standards/private-mcp` standard. |
| `ENCRYPTION_KEY` | AES-256-GCM key (base64-std or 32-byte hex) used to encrypt all issued codes/tokens/DCR client_ids as stateless blobs. No server-side session/token storage. |
| `ENGRAM_PUBLIC_URL` | Public HTTPS base URL (e.g. `https://engram.dadecore.dev`) used as the OAuth `issuer` and in RFC 8414/9728 discovery documents. |

## Optional environment variables

| Var | Default | Purpose |
|---|---|---|
| `MCP_BEARER_TOKEN` | unset | If set, allows direct `Authorization: Bearer <token>` access to `/mcp`, bypassing the OAuth flow entirely (CLI-style auth path). |
| `ENGRAM_HTTP_ADDR` / `ENGRAM_MCP_PORT` | `0.0.0.0:8080` | Listen address. |
| `ENGRAM_PROJECT` / `--project` | unset (recommended) | Do NOT set on the remote server. Engram is multi-project: every `mem_*` call must pass `project` explicitly. A process-level default would make every client write into one project. |

## Endpoints

- `GET /health` — liveness check, no auth.
- `GET /.well-known/oauth-authorization-server` — RFC 8414 metadata.
- `GET /.well-known/oauth-protected-resource` — RFC 9728 metadata.
- `POST /register` — RFC 7591 Dynamic Client Registration. Stateless: the returned `client_id` is itself an encrypted blob, nothing is persisted server-side.
- `GET/POST /authorize` — Authorization Code + PKCE (S256) flow. GET renders the owner-key gate HTML form; POST verifies the owner key (constant-time compare, explicit visible error on mismatch — never silent/hanging) and redirects with a code on success.
- `POST /token` — exchanges `authorization_code` or `refresh_token` grants for an access + refresh token pair (access TTL 1h, refresh TTL 90d).
- `ANY /mcp` — the actual MCP Streamable HTTP endpoint (`mark3labs/mcp-go`'s `NewStreamableHTTPServer`, same `*server.MCPServer` object used by stdio mode, so all 22 MCP tools are identical). Protected by `RequireBearer` middleware: accepts either `MCP_BEARER_TOKEN` (if configured) or a valid encrypted access token.

## Design notes

- Single repo, additive only: no fork of `internal/store` or `internal/mcp/mcp.go`, no separate Hub/wrapper repo. `mark3labs/mcp-go` supports stdio and Streamable HTTP on the same server object, so the transport is a thin new entrypoint (`cmd/engram/http_transport.go`) plus a new `internal/remoteauth` package.
- Distinct from `engram cloud serve`: that is an unrelated Postgres-backed sync/replication REST API (`/sync/*`, `/admin/*`, JWT + `ENGRAM_CLOUD_TOKEN_PEPPER`). This remote-mcp work does not touch it.
- Follows DadeCore's canonical `dadecore/standards/private-mcp` standard: single-owner embedded Authorization Server, owner-key as mandatory first gate with visible failure, OAuth 2.1 + PKCE S256, RFC 8414/7591/9728, refresh tokens, stateless AES-256-GCM token/client encoding (no DB-backed session or client storage), `MCP_BEARER_TOKEN` as a parallel direct-auth path, and no multiuser/session infrastructure.
- No secrets are hardcoded anywhere in the repo; `ENGRAM_OWNER_KEY`, `ENCRYPTION_KEY`, and `MCP_BEARER_TOKEN` are required to be supplied via environment at deploy time (see `docs/REMOTE-MCP.md` and the `coolify` TaskRelay task for production wiring).

## Testing

`scripts/test-remote-mcp.sh` is a parametrized smoke test:

```
BASE_URL=https://engram.dadecore.dev OWNER_KEY=... ./scripts/test-remote-mcp.sh
```

or, for the `MCP_BEARER_TOKEN` path only:

```
BASE_URL=... BEARER_TOKEN=... ./scripts/test-remote-mcp.sh --bearer-only
```

It exercises `/health`, discovery, DCR, the owner-key gate (wrong key then
correct key), PKCE token exchange, and MCP `initialize`/`tools/list`/
`mem_save`/`mem_search`.

## Project model (multi-project)

Engram stores memory for many projects. The remote server has no default project.
Clients pass the project on each call: `mem_save`, `mem_search`, `mem_context`,
`mem_session_summary`, `mem_save_prompt`, `mem_timeline`, `mem_stats`, `mem_review`,
`mem_doctor`. The server instructions (sent at MCP initialize) state this rule.

## Plugins

Every client connects to the same endpoint. The plugins only add the protocol, the project rule and compaction recovery. None of them runs a local binary or database.

| Client | Install |
|---|---|
| Claude Code | `/plugin marketplace add jcd3dr/engram`, then `/plugin install engram@engram` |
| Codex | `codex plugin marketplace add jcd3dr/engram --ref main`, then `codex plugin add engram@engram` |
| OpenCode | Remote entry in `opencode.json` plus `plugin/opencode/engram.ts` (see [PLUGINS.md](PLUGINS.md#opencode-plugin)) |
| Pi | `pi install git:github.com/jcd3dr/engram`, then `pi-engram init` |
| claude.ai, ChatGPT | Add the connector URL `https://engram-mcp.jcdcruz.com/mcp` in the client settings, then add `skills/memory-protocol/SKILL.md` as a custom skill. These clients cannot run local hooks, so the skill carries the protocol. |
