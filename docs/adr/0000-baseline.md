# 0000. Baseline

> DARS Standard v1.17

Status: Accepted
Date: 2026-10-15

## Context

DadeCore maintains `jcd3dr/engram`, a fork of `Gentleman-Programming/engram`. The fork adds a Streamable HTTP MCP transport and an embedded OAuth authorization server, and is deployed to Coolify (`engram-mcp`, https://engram-mcp.jcdcruz.com). Before this baseline, the repository had no DARS documentary model and its documentation was partly contradictory. This record establishes the reconciled starting point from read-only discovery. It documents reality; it does not authorize fixes.

## Established system and architecture

- One Go binary (`cmd/engram`) with a shared store and tool layer. Go 1.25.10, `mark3labs/mcp-go` v0.44.0, SQLite with FTS5 via `modernc.org/sqlite`. Postgres (pgx) is used only by `engram cloud serve`.
- Transports on one `*server.MCPServer` object:
  - stdio: `engram mcp` (default), served with `mcpserver.ServeStdio`.
  - Streamable HTTP: `engram mcp --transport=http`, implemented in `cmd/engram/http_transport.go` with `NewStreamableHTTPServer`. This is an in-process second entrypoint. It is not a proxy and does not spawn a stdio subprocess.
- `internal/store` and `internal/mcp/mcp.go` are not modified by the fork's transport work. The fork's additions are `cmd/engram/http_transport.go`, `internal/remoteauth/`, `docker/mcp/Dockerfile`, and `docs/{REMOTE-MCP,COOLIFY,PRODUCTION,CANONICAL-STANDARDS}.md`.
- Remote auth: single-owner OAuth 2.1 authorization server with owner-key gate, PKCE S256, RFC 8414/7591/9728 metadata, and stateless AES-256-GCM tokens. `MCP_BEARER_TOKEN` is a parallel direct-auth path.
- Production deployment: Coolify application `engram-mcp` (uuid `rqtlslch2kccwsmr1njlr3ft`, project "MCP Servers"), built from `jcd3dr/engram` branch `main` via GitHub App `dadecore-github`, Dockerfile `docker/mcp/Dockerfile`, auto-deploy enabled. The container runs `engram mcp --transport=http`.
- Persistence: a Coolify persistent storage entry mounts `/home/engram/.engram` (`ENGRAM_DATA_DIR`). The SQLite database lives there.
- Production env vars (Coolify): `ENGRAM_OWNER_KEY`, `ENCRYPTION_KEY`, `MCP_BEARER_TOKEN`, `ENGRAM_PUBLIC_URL`, `ENGRAM_PROJECT` (`dadecore-standards`), `ENGRAM_HTTP_ADDR`, `ENGRAM_DATA_DIR`, each also present for preview scope.

## Established decisions and constraints

- Native dual transport was chosen over a subprocess-bridging Hub/wrapper, so the upstream MCP object is served over HTTP directly.
- The fork is additive so upstream changes can be reviewed and merged selectively. There is no automated upstream sync.
- `engram cloud serve` (Postgres REST sync) is a separate subsystem and is not part of the remote MCP runtime.
- `main` deploys to production automatically on push.
- Contribution workflow is issue-first: a PR needs a linked issue with `status:approved` and exactly one `type:*` label (`CONTRIBUTING.md`, `.github/workflows/pr-check.yml`).

## Documentation vs. repository discrepancies

1. README banner says the fork adds a Streamable HTTP transport. The README body states the MCP transport is "stdio only" and that there is no HTTP endpoint. The body text was inherited from upstream and is stale for this fork.
2. README says "MCP Tools (20)". `internal/mcp/mcp.go` registers 22 tools (`mem_search`, `mem_save`, `mem_update`, `mem_review`, `mem_suggest_topic_key`, `mem_delete`, `mem_save_prompt`, `mem_pin`, `mem_unpin`, `mem_context`, `mem_stats`, `mem_timeline`, `mem_get_observation`, `mem_session_summary`, `mem_session_start`, `mem_session_end`, `mem_capture_passive`, `mem_merge_projects`, `mem_current_project`, `mem_doctor`, `mem_judge`, `mem_compare`). `docs/REMOTE-MCP.md` says 22.
3. `docs/REMOTE-MCP.md` and `docs/COOLIFY.md` exist in the fork but are not linked from the README, and `docs/REMOTE-MCP.md` does not exist on upstream `main`.
4. `AGENTS.md` at repository root contained only a contributor skills index. It had no project identity, setup commands, or DARS context before this adoption.
5. No `CLAUDE.md` or `GEMINI.md` existed at the repository root before this adoption.
6. `docs/PRODUCTION.md` documents a manual backup command. No scheduled backup exists.

## Known gaps (documented, not fixed by this adoption)

- No automated backup of `/home/engram/.engram`. Backup is manual, per `docs/PRODUCTION.md`.
- Secrets `ENGRAM_OWNER_KEY`, `ENCRYPTION_KEY`, and `MCP_BEARER_TOKEN` are marked `is_buildtime: true`. With `inject_build_args_to_dockerfile` enabled, they are passed as build arguments.
- Two dead Coolify applications, `engram` (uuid `psw45dyyfebnwwm7nkmxz8nw`) and `engram-standalone` (uuid `b8b7gfxpnrlq2hrc26u9tp7c`), are `exited:unhealthy`. Both run `engram serve`, which is the local HTTP API and not MCP. They have leftover named volumes (`engram-data`, `engram-standalone-data`).
- Known open issue, from `docs/PRODUCTION.md`: `mem_update` sometimes reports `project_source: "dir_basename"` instead of `process_override`. The write itself has landed on the correct record in every test performed.
- Protocol delivery: the server sends `serverInstructions` at MCP initialize (`server.WithInstructions`), which holds the core rules. The full memory protocol from the upstream `engram-memory` skill (proactive triggers, compaction recovery, session summary format) reaches only local clients through plugins and hooks.
- Cloud sync via `engram cloud serve` is not deployed. Autosync (`ENGRAM_CLOUD_AUTOSYNC`) is not configured on `engram-mcp`.

## Unresolved facts

- The exact commit currently running in Coolify. The Coolify deployments list returned empty. `main` head at discovery time was `9cb5441553f09fa5a06ca32e36a1b52d334da771`.
- Whether the claude.ai connector named "Engram mcp" points at `https://engram-mcp.jcdcruz.com/mcp`. Connector URLs are not exposed by the connector listing. Strong inference only: the connector's tools resolve to project `dadecore-standards` and cwd `/home/engram`.
- Whether claude.ai and ChatGPT inject MCP server `instructions` into model context. Not verified.
- Contents of the leftover volumes `engram-data` and `engram-standalone-data`.
- Whether additional agent ecosystems (`.agents/`, `.windsurf/`, `.claude-plugin/`, `plugin/`) must be reconciled. Not confirmed at this checkpoint.

## Evidence used for recognition

- `cmd/engram/main.go`: dispatch of `mcp` to `cmdMCPHTTP` when `--transport=http`, else `cmdMCP` with `serveMCP = ServeStdio`.
- `cmd/engram/http_transport.go`: `NewStreamableHTTPServer`, `remoteauth` wiring, `ENGRAM_PROJECT` pinning, `/health`, `/.well-known/*`, `/register`, `/authorize`, `/token`, `/mcp`.
- `internal/mcp/mcp.go`: `server.WithInstructions(serverInstructions)`, 22 registered tools.
- `docker/mcp/Dockerfile`: `ENTRYPOINT ["engram"]`, `CMD ["mcp", "--transport=http"]`, `HOME=/home/engram`, healthcheck on `:8080/health`.
- `docs/REMOTE-MCP.md`, `docs/COOLIFY.md`, `docs/PRODUCTION.md`.
- Coolify API: application `engram-mcp` (`build_pack: dockerfile`, `git_repository: jcd3dr/engram`, `git_branch: main`, `source_type: GithubApp`, auto-deploy on), persistent storage `vygjn91sbd2imtg8bul65gx8` mounted at `/home/engram/.engram`, environment variables as listed above.
- Coolify API: applications `engram` and `engram-standalone` with their Dockerfiles and start commands.
- GitHub: `jcd3dr/engram` README, CONTRIBUTING, Makefile, CHANGELOG, `.github/` layout and workflows; `Gentleman-Programming/engram` upstream `docs/` and `cmd/engram/`.

## Consequences

- The repository now has a documented, reconciled baseline. Future agents can read the discrepancies above instead of rediscovering them.
- Fixes for the gaps above are separate development work and require their own issues and PRs.
- Documentation after this ADR must not claim the transport is stdio-only, and must not claim 20 tools.

## Suggestions (non-binding)

- Schedule a backup of the Coolify volume, or add `engram export` to a scheduled task that writes outside the volume.
- Change the three secrets to `is_buildtime: false` in Coolify.
- Delete the two dead applications after confirming the leftover volumes hold no data that must be kept.
- Verify the connector URL in claude.ai Settings → Connectors.
- Verify whether claude.ai injects server instructions by running one connector session and checking the first tool-call behavior.
