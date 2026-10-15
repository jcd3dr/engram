# Engineering Conventions

> DARS Standard v1.17

This file is authoritative for the engineering practices that govern this repository.

## Contribution workflow

- Issue first. Open an issue from a template (Bug Report or Feature Request). Blank issues are disabled.
- Do not open a PR until the issue has `status:approved`.
- Branch from `main`. Keep one logical change per PR.
- Link the approved issue in the PR body with `Closes #N`, `Fixes #N`, or `Resolves #N`.
- Add exactly one `type:*` label to each PR: `type:bug`, `type:feature`, `type:docs`, `type:refactor`, `type:chore`, or `type:breaking-change`.
- Update documentation in the same PR when behavior changes.
- Do not reference endpoints, scripts, or flags that do not exist in code.

## Commits

- Conventional Commits: `<type>(<scope>): <short description>`.
- Breaking changes use `!` and a `BREAKING CHANGE:` footer.
- No `Co-Authored-By` trailers.

## Testing

- Unit: `go test ./...`.
- End-to-end: `go test -tags e2e ./internal/server/...`.
- Run both before pushing. Both are required status checks on `main`.

## Go

- Module path: `github.com/Gentleman-Programming/engram`. Go version `1.25.10`.
- Build the CLI with `go build -o engram ./cmd/engram`.
- Dashboard templates (`internal/cloud/dashboard/*.templ`) are regenerated with `make templ` before committing.
- SQLite is pure Go (`modernc.org/sqlite`). Do not add CGO-only dependencies to the store.

## Transport and fork boundary

- Remote transport changes are additive: `cmd/engram/http_transport.go`, `internal/remoteauth/`, and `docker/mcp/`.
- Do not change `internal/store/` or `internal/mcp/mcp.go` for transport work.
- Do not conflate `engram cloud serve` with the remote MCP. They are separate runtimes.

## Secrets and configuration

- Never commit credentials. `.env` and `*.env` are gitignored.
- Production secrets are set only in Coolify. Mark secrets runtime-only (`is_buildtime: false`).
- Environment variable names are documented in `docs/REMOTE-MCP.md` and `DOCS.md`.

## npm (plugin/pi, plugin/obsidian)

- Install new packages with `npq install <package>`.
- Honor `.npmrc`: `ignore-scripts=true`, `allow-git=none`, `min-release-age=3`. Do not edit `.npmrc` to bypass these.
- Paste the Snyk Advisor link for each new dependency in the PR description.

## Skills

- Repository skills live in `skills/`. Each uses a structured base followed by an If/Then/Example cookbook.
- Link skills into local agent folders with `./setup.sh`.
