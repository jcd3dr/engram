# DadeCore Engram — production reference

This is the top-level operational reference for the DadeCore-owned Engram
fork. It ties together `docs/REMOTE-MCP.md` (transport + auth design),
`docs/COOLIFY.md` (deployment), and `docs/CANONICAL-STANDARDS.md`
(the standards migration) into one place so nobody has to reconstruct the
history of decisions behind this deployment.

## 1. What this is

`jcd3dr/engram` is a fork of `Gentleman-Programming/engram` (attribution and
upstream reference kept — see the README banner and `upstream-origin` git
remote). It is DadeCore's persistent-memory MCP server, extended with a
second, network-accessible transport so it can be used as a Remote MCP
connector from Claude and ChatGPT, deployed publicly over HTTPS via
Coolify at **https://engram-mcp.jcdcruz.com**.

Its first production use is as DadeCore's canonical store for development
standards and agreements (`docs/CANONICAL-STANDARDS.md`) — the original
motivation: Claude/ChatGPT were losing or over-summarizing DadeCore's
standards between sessions. `mem_get_observation` always returns complete,
non-summarized content, which is the structural fix for that problem.

## 2. Architecture

One Go binary, two transports, one server object:

```
cmd/engram/main.go
  |
  +-- mcp.NewServer(...)  -->  *server.MCPServer (mark3labs/mcp-go)
  |                                |
  |                                +-- stdio: mcpserver.ServeStdio(mcpServer)         (unchanged, existing)
  |                                +-- HTTP:  mcpserver.NewStreamableHTTPServer(...)  (new, cmd/engram/http_transport.go)
  |
  +-- internal/mcp/mcp.go   — 22 mem_* tool handlers, UNCHANGED by this project
  +-- internal/store        — SQLite (modernc.org/sqlite, pure-Go), UNCHANGED by this project
```

**Why no separate Hub/wrapper repository.** DadeCore's default pattern for
adapting an upstream MCP to remote/authenticated deployment is a
Hub/Wrapper repo that spawns the upstream MCP as a stdio subprocess and
re-exposes its tools remotely (`dadecore/standards/upstream-wrapper-hub`,
migrated into Engram itself — see below). That pattern exists because most
upstream MCP servers only speak stdio. Engram is different: it already
depends on `mark3labs/mcp-go`, whose `server.NewStreamableHTTPServer(...)`
serves the *exact same* `*server.MCPServer` object stdio uses, over HTTP,
natively. Wrapping Engram in a subprocess-bridging Hub would have added a
second process boundary, a second failure surface, and was the likely root
cause of an earlier failed ChatGPT-assisted deployment attempt (two dead
`exited:unhealthy` Coolify apps — `engram` and `engram-standalone`,
pointing at the raw upstream repo with a nonexistent `engram serve`
command and no auth — were found already sitting in the Coolify project
when this work started; see `docs/COOLIFY.md`). This project used the
native dual-transport capability instead: one additive Go package
(`internal/remoteauth`) plus one new entrypoint file
(`cmd/engram/http_transport.go`). `internal/mcp/mcp.go` and `internal/store`
were never modified — this keeps future upstream diffs trivial to review
and selectively merge; there is no automated upstream sync, by design (see
`dadecore/standards/upstream-wrapper-hub` in Engram for when the wrapper
pattern *does* still apply — anything that doesn't natively support two
transports on one object).

## 3. Auth boundary

Implements DadeCore's canonical standard, `dadecore/standards/private-mcp`
(now itself stored in Engram, see `docs/CANONICAL-STANDARDS.md`):
single-owner Authorization Server embedded in the MCP (`internal/remoteauth`),
owner-key gate as the mandatory first check, OAuth 2.1 Authorization Code +
PKCE S256, RFC 8414 discovery, RFC 7591 DCR, refresh tokens, all tokens
stateless AES-256-GCM blobs (no session/token table), `MCP_BEARER_TOKEN` as
a parallel direct-auth path. The auth boundary lives entirely in the
transport layer (`cmd/engram/http_transport.go` + `internal/remoteauth`) —
`internal/store` has no knowledge of auth, keeping the memory core
independent of it, as required.

Full endpoint/env-var reference: `docs/REMOTE-MCP.md`.

## 4. Deployment

Live Coolify application `engram-mcp` (uuid `rqtlslch2kccwsmr1njlr3ft`,
project "MCP Servers"), building `docker/mcp/Dockerfile`, public domain
`https://engram-mcp.jcdcruz.com` with Traefik-managed Let's Encrypt TLS.
Full setup steps, the persistent-storage gotcha and its fix, and manual
connector-setup steps for Claude/ChatGPT: `docs/COOLIFY.md`.

### Persistent data

`ENGRAM_DATA_DIR=/home/engram/.engram` inside the container, backed by a
**Coolify-managed persistent storage entry** (not a `custom_docker_run_options`
volume flag — that was tried first and silently did not persist data; see
`docs/COOLIFY.md` for the full story). Confirmed to survive both `restart`
and full `redeploy` multiple times during this project, most recently while
validating the canonical standards records (`docs/CANONICAL-STANDARDS.md`).

### Health verification

```
curl -sS https://engram-mcp.jcdcruz.com/health
# {"service":"engram-remote-mcp","status":"ok"}
```

Coolify also runs this as the container healthcheck (30s interval).

### Backup and restore

The entire persistent state is the SQLite database (plus FTS5 index files)
under `/home/engram/.engram` in the named Coolify volume
(`rqtlslch2kccwsmr1njlr3ft-engram-mcp-data`).

Backup: `docker run --rm -v rqtlslch2kccwsmr1njlr3ft-engram-mcp-data:/data -v $(pwd):/backup alpine tar czf /backup/engram-data-$(date +%Y%m%d).tar.gz -C /data .`
run on the Coolify host (or via Coolify's own volume/backup tooling if
preferred).

Restore: stop the application, extract the tarball back into the same
named volume (`docker run --rm -v rqtlslch2kccwsmr1njlr3ft-engram-mcp-data:/data -v $(pwd):/backup alpine sh -c "cd /data && tar xzf /backup/<file>.tar.gz"`),
start the application again, verify with `mem_stats` / `mem_context` over
the Remote MCP endpoint.

### Update / rollback

Update: push to `main` on `jcd3dr/engram` — the GitHub webhook triggers an
automatic Coolify rebuild+redeploy (rolling update, old container removed
only after the new one passes its healthcheck). To deploy a specific
commit without waiting for a push, use Coolify's `deploy` action against
application `rqtlslch2kccwsmr1njlr3ft`.

Rollback: Coolify keeps prior deployment images; use the Deployments tab
(or the `deployment`/`list_deployments` API) to find the last known-good
deployment and redeploy from that commit SHA. Because persistent data lives
in the separate named volume, rolling the application back does not affect
stored memories.

## 5. Connecting a client

See `docs/COOLIFY.md` → "Adding the connector in Claude or ChatGPT" for the
exact manual steps (OAuth discovery + owner-key gate flow), and
`docs/REMOTE-MCP.md` for the raw endpoint reference and the
`MCP_BEARER_TOKEN` direct-auth path for CLI-style clients.

## 6. Testing end-to-end

`scripts/test-remote-mcp.sh` is the canonical smoke test (health, discovery,
DCR, owner-key gate, PKCE exchange, `initialize`/`tools/list`/`mem_save`/
`mem_search`). It was exercised manually and extended during the
`validation` and `canonical` TaskRelay tasks with additional live checks
not baked into the script (mem_update semantics, restart/redeploy
persistence, conflict-judgment resolution) — see those tasks' recorded
outputs in the TaskRelay project `3baagztx` for the full transcript of
what was actually run against the production endpoint.

## 7. `engram cloud serve` is unrelated — do not confuse the two

Engram ships a second, entirely separate subsystem: `engram cloud serve`
(`internal/cloud/*`), a Postgres-backed REST sync/replication API
(`/sync/*`, `/admin/*`, `/dashboard/*`, JWT + `ENGRAM_CLOUD_TOKEN_PEPPER`
auth) for syncing memory across machines. It is **not MCP**, was not
touched, modified, or reused by any part of this project, and is not part
of this deployment's runtime path — the Coolify application described here
runs `engram mcp --transport=http`, never `engram cloud serve`. If a future
change needs cross-machine sync, that is a distinct, separate evaluation —
it should not be conflated with the Remote MCP work documented here.

## 8. Known non-blocking findings (carried forward, not yet fixed)

- `mem_update`'s response envelope occasionally reports
  `project_source: "dir_basename"` instead of the expected
  `"process_override"` seen on every other tool call in this deployment.
  The actual write always landed on the correct record in every test
  performed (verified by immediate re-read), so this is a response-metadata
  inconsistency, not a data-integrity bug — but worth root-causing in
  `internal/mcp/mcp.go`'s project-resolution path for `mem_update`
  specifically, without violating the "no changes to internal/mcp/mcp.go"
  constraint of this project (i.e. as a separate, deliberate follow-up).
- `mem_update`'s `content` argument replaces the full stored content; it is
  not a partial-field merge. Anything editing a canonical record must
  fetch the current full content via `mem_get_observation` first and send
  back the complete desired text, not just the changed portion.

## 9. Repository cleanliness

`internal/remoteauth/*.go`, `cmd/engram/http_transport.go`,
`docker/mcp/*`, and `docs/{REMOTE-MCP,COOLIFY,CANONICAL-STANDARDS,PRODUCTION}.md`
are the only additions made by this project; all are committed on `main`.
No experimental scratch files, alternate transport implementations, or
half-finished Hub/wrapper scaffolding were left in the repository — working
files used during validation (shell scripts hitting the live endpoint,
temporary standards-migration source files) were kept in the deployment
sandbox, not committed. The two large prebuilt binaries under `cmd/engram/`
(`main`, `gentle-creation`, ~17MB each) are pre-existing upstream artifacts
from before this fork and are out of this project's scope.
