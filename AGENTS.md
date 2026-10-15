# AGENTS.md

> DARS Standard v1.17

## Project

Engram is persistent memory for AI coding agents: a Go binary with SQLite and FTS5, exposed through a CLI, a local HTTP API (`engram serve`), a stdio MCP server (`engram mcp`), and a terminal UI.

This repository is DadeCore's fork of `Gentleman-Programming/engram` (remote `upstream-origin`). The fork adds a Streamable HTTP MCP transport with an embedded OAuth authorization server, deployed to Coolify as `engram-mcp` at https://engram-mcp.jcdcruz.com.

Stack: Go 1.25.10, `mark3labs/mcp-go` v0.44.0, SQLite (`modernc.org/sqlite`, FTS5). Postgres (`pgx`) is used only by `engram cloud serve`.

## Repository structure

- `cmd/engram/` — CLI entrypoint. `main.go` dispatches commands. `http_transport.go` is the fork's HTTP MCP entrypoint.
- `internal/mcp/` — MCP tool handlers, shared by stdio and HTTP.
- `internal/store/` — SQLite store and FTS5 search.
- `internal/server/` — local HTTP API.
- `internal/remoteauth/` — fork addition: OAuth authorization server for remote MCP.
- `internal/cloud/` — `engram cloud serve` (Postgres sync and dashboard). Not MCP.
- `plugin/` — agent plugins.
- `docker/mcp/` — production Dockerfile for the remote MCP.
- `docs/` — project documentation (see the context map below).

## Setup and commands

- Build: `go build -o engram ./cmd/engram`
- Unit tests: `go test ./...`
- End-to-end tests: `go test -tags e2e ./internal/server/...`
- Regenerate dashboard templates after editing `.templ` files: `make templ`
- Run the remote MCP locally: requires `ENGRAM_OWNER_KEY`, `ENCRYPTION_KEY`, and `ENGRAM_PUBLIC_URL` (see `docs/REMOTE-MCP.md`).

## Critical rules

- `main` deploys to production automatically. Make changes on a branch and merge through a PR.
- Never commit credentials. Production secrets live only in Coolify.
- Transport work is additive. Do not change `internal/store/` or `internal/mcp/mcp.go` to add transports.
- `engram cloud serve` is not MCP. Do not treat the two as one system.
- PRs need a linked approved issue and exactly one `type:*` label. Follow `docs/CONVENTIONS.md`.
- Do not reference endpoints, scripts, or flags that do not exist in code.

## Engineering conventions

`docs/CONVENTIONS.md` is authoritative for repository engineering practices.

## DARS governance activation

- Use the DARS governance skill when it is available.
- Otherwise read `docs/GOVERNANCE_SUMMARY.md`.

## Project knowledge and context map

| Need | Source | When to use it |
|---|---|---|
| Durable product meaning and requirements | `docs/SPEC.md` | Changing behavior or scope |
| Structure and navigation | `docs/REPO_MAP.md` | Finding where code or docs live |
| Current state and blockers | `docs/PLAN.md` | Resuming work |
| Engineering rules | `docs/CONVENTIONS.md` | Before implementation |
| Published history | `CHANGELOG.md` | Release or user-visible change |
| Significant decisions | `docs/adr/` | Before changing an area an ADR governs. Start with `0000-baseline.md` |
| Remote MCP design | `docs/REMOTE-MCP.md` | Changing transport or auth |
| Deployment | `docs/COOLIFY.md`, `docs/PRODUCTION.md` | Deploying or operating production |

## Before finishing work

1. Run `go test ./...` (and the e2e command if the change touches `internal/server`).
2. Evaluate durable impact against `docs/GOVERNANCE_SUMMARY.md`.
3. Update only the responsible knowledge that changed.

## Agent skills index

When working on this project, load the relevant skill(s) BEFORE writing any code.

1. Check the trigger column to find skills that match your current task
2. Load the skill by reading the SKILL.md file at the listed path
3. Follow ALL patterns and rules from the loaded skill
4. Multiple skills can apply simultaneously

| Skill | Trigger | Path |
|-------|---------|------|
| `engram-architecture-guardrails` | Any change that affects system boundaries, ownership, state flow, or cross-package responsibilities. | [`skills/architecture-guardrails/SKILL.md`](skills/architecture-guardrails/SKILL.md) |
| `engram-branch-pr` | When creating a pull request, opening a PR, or preparing changes for review. | [`skills/branch-pr/SKILL.md`](skills/branch-pr/SKILL.md) |
| `engram-business-rules` | Any change that affects sync behavior, project controls, permissions, or memory semantics. | [`skills/business-rules/SKILL.md`](skills/business-rules/SKILL.md) |
| `engram-commit-hygiene` | Any commit creation, review, or branch cleanup. | [`skills/commit-hygiene/SKILL.md`](skills/commit-hygiene/SKILL.md) |
| `engram-cultural-norms` | Starting substantial work, reviewing changes, or defining team conventions. | [`skills/cultural-norms/SKILL.md`](skills/cultural-norms/SKILL.md) |
| `engram-dashboard-htmx` | Any change to htmx attributes, partial updates, forms, or server-rendered browser UI. | [`skills/dashboard-htmx/SKILL.md`](skills/dashboard-htmx/SKILL.md) |
| `engram-docs-alignment` | Any code or workflow change that affects user or contributor behavior. | [`skills/docs-alignment/SKILL.md`](skills/docs-alignment/SKILL.md) |
| `engram-issue-creation` | When creating a GitHub issue, reporting a bug, or requesting a feature. | [`skills/issue-creation/SKILL.md`](skills/issue-creation/SKILL.md) |
| `engram-memory-protocol` | Decisions, bugfixes, discoveries, preferences, or session closure. | [`skills/memory-protocol/SKILL.md`](skills/memory-protocol/SKILL.md) |
| `engram-plugin-thin` | Changes in plugin scripts/hooks for Claude, OpenCode, Gemini, or Codex. | [`skills/plugin-thin/SKILL.md`](skills/plugin-thin/SKILL.md) |
| `engram-pr-review-deep` | Reviewing any external or internal contribution before merge. | [`skills/pr-review-deep/SKILL.md`](skills/pr-review-deep/SKILL.md) |
| `engram-project-structure` | Creating files, packages, handlers, templates, styles, or tests in this repo. | [`skills/project-structure/SKILL.md`](skills/project-structure/SKILL.md) |
| `engram-sdd-flow` | When user requests SDD or multi-phase implementation planning. | [`skills/sdd-flow/SKILL.md`](skills/sdd-flow/SKILL.md) |
| `engram-server-api` | Any route, handler, payload, or status code modification. | [`skills/server-api/SKILL.md`](skills/server-api/SKILL.md) |
| `engram-testing-coverage` | When implementing behavior changes in any package. | [`skills/testing-coverage/SKILL.md`](skills/testing-coverage/SKILL.md) |
| `engram-tui-quality` | Changes in model, update, view, navigation, or rendering. | [`skills/tui-quality/SKILL.md`](skills/tui-quality/SKILL.md) |
| `engram-ui-elements` | Adding or changing dashboard UI components or connected browsing flows. | [`skills/ui-elements/SKILL.md`](skills/ui-elements/SKILL.md) |
| `engram-visual-language` | Any dashboard styling, typography, spacing, or visual identity change. | [`skills/visual-language/SKILL.md`](skills/visual-language/SKILL.md) |
| `engram-backlog-triage` | Auditing open issues or PRs, triaging the backlog, or reviewing contributor submissions as a maintainer. | [`skills/backlog-triage/SKILL.md`](skills/backlog-triage/SKILL.md) |
| `gentleman-bubbletea` | When editing Go files in installer/internal/tui/, working on TUI screens, or adding new UI features. | [`skills/gentleman-bubbletea/SKILL.md`](skills/gentleman-bubbletea/SKILL.md) |
