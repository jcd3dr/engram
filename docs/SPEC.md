# Project Specification

> DARS Standard v1.17

## Purpose

Engram gives AI coding agents persistent memory that survives across sessions and context compaction. It stores observations (decisions, bug fixes, discoveries, conventions), session records, and user prompts in a local SQLite database with FTS5 full-text search, and exposes that memory through a CLI, a local HTTP API, a stdio MCP server, and a terminal UI.

This DadeCore fork adds a second MCP transport, Streamable HTTP, with a single-owner OAuth 2.1 authorization server, so that Engram can be used as a remote MCP connector from cloud clients (claude.ai, ChatGPT).

## Scope

### In scope

- Memory storage and retrieval: observations, topic-key upserts, sessions, prompts, search, timeline, lifecycle review, project merge and prune.
- Project scoping for every read and write.
- MCP tool surface (22 tools), identical over stdio and HTTP.
- Local HTTP API served by `engram serve`.
- Remote MCP transport: `engram mcp --transport=http`, owner-key OAuth gate, `MCP_BEARER_TOKEN` direct auth, health endpoint.
- Production deployment of the remote MCP on Coolify from `jcd3dr/engram` `main`.

### Out of scope

- `engram cloud serve`: Postgres-backed cross-machine replication and dashboard. It is a separate subsystem and is not part of the remote MCP runtime.
- Multi-user accounts on the remote MCP. The remote endpoint is single-owner.
- Agent plugin behavior beyond what the repository documents (`plugin/`).

## Requirements

- **R1 — Project scoping.** Every MCP call resolves a project in this order: explicit `project` argument (validated against the store), then `ENGRAM_PROJECT` or `--project`, then working-directory detection. Write tools never accept an unvalidated project name.
- **R2 — Remote endpoint.** `POST /mcp` serves MCP over Streamable HTTP. `GET /health` returns `{"status":"ok","service":"engram-remote-mcp"}` without authentication.
- **R3 — Authentication.** `/mcp` requires either a valid OAuth access token issued after the owner-key gate, or the `MCP_BEARER_TOKEN` bearer value. Owner-key comparison is constant-time.
- **R4 — Stateless tokens.** Authorization codes, access tokens, refresh tokens, and dynamic client identifiers are AES-256-GCM encrypted blobs. The remote auth layer keeps no session or token table.
- **R5 — Persistence.** Memory survives container restart and redeploy. The SQLite database must live on a Coolify persistent storage entry mounted at `ENGRAM_DATA_DIR`.
- **R6 — Transport isolation.** Remote transport code must not modify `internal/store` or `internal/mcp`.

## Behavioral boundaries

- `mem_get_observation` returns the complete stored content, never a summary.
- `mem_update` replaces the full `content` field. It does not merge partial text.
- `mem_save` with `topic_key` upserts the existing observation in the same project and scope, incrementing `revision_count`.
- Soft delete is the default for `mem_delete`. Hard delete requires an explicit flag.
- Secrets are never committed to git. Production secrets live only in Coolify.

## Success criteria

- A cloud client connected to `https://engram-mcp.jcdcruz.com/mcp` can list tools, save a memory, search it, and retrieve it.
- The same memory is still retrievable after a Coolify redeploy.
- `GET /health` returns status `ok` after deployment.
