# Project Plan

> DARS Standard v1.17

## Current focus

Make the fork's documentation describe what is actually deployed, then clean up the leftover deployment attempts. DARS adoption is in progress on branch `dars/adoption`.

## Active work

- DARS adoption PR (branch `dars/adoption`). Documentary changes only.
- Fix README contradictions (transport and tool count). Included in the same branch.

## Blockers

- PR validation requires a linked issue with `status:approved` and one `type:*` label. Requires a maintainer to approve the issue before the PR can pass checks.
- Coolify cleanup of `engram` and `engram-standalone` is pending owner confirmation of their leftover volumes.

## Relevant pending work

- Backup: no scheduled backup of `/home/engram/.engram`. Only the manual procedure in `docs/PRODUCTION.md` exists.
- Secrets: change `ENGRAM_OWNER_KEY`, `ENCRYPTION_KEY`, `MCP_BEARER_TOKEN` to runtime-only (`is_buildtime: false`) in Coolify, then redeploy and verify `/health` and `/mcp`.
- Protocol delivery to cloud clients: confirm whether claude.ai injects server `instructions`. If not, publish the full memory protocol as a skill for claude.ai.
- `mem_update` project-resolution inconsistency (`dir_basename` vs `process_override`), documented in `docs/PRODUCTION.md`.
- Connector verification: confirm the claude.ai connector "Engram mcp" points at `https://engram-mcp.jcdcruz.com/mcp`.
- Commit alignment: confirm the Coolify deployment matches `main`. Deployments list returned empty during discovery.

## Next steps

1. Owner approves the linked issue so the DARS PR can pass checks.
2. Owner confirms whether the leftover volumes `engram-data` and `engram-standalone-data` can be deleted.
3. Delete Coolify applications `engram` and `engram-standalone`.
4. Set the three secrets to runtime-only in Coolify and redeploy `engram-mcp`.
5. Run one connector session and record whether server instructions reach the model.
6. Configure a scheduled backup of the volume.

## Continuation context

- Production MCP: Coolify app `engram-mcp` (uuid `rqtlslch2kccwsmr1njlr3ft`), built from `jcd3dr/engram` `main`. Health: `https://engram-mcp.jcdcruz.com/health`.
- Discovery evidence and every known gap are recorded in `docs/adr/0000-baseline.md`. Do not re-derive them.
- Storage: Coolify persistent storage `vygjn91sbd2imtg8bul65gx8` mounted at `/home/engram/.engram`.
- The `main` branch auto-deploys to production. Make changes on a branch and merge through a PR.
