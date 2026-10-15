# Coolify deployment (production)

Live instance: `engram-mcp` in the "MCP Servers" Coolify project, public URL
`https://engram-mcp.jcdcruz.com`.

## Setup

1. Application: Dockerfile build pack, source `docker/mcp/Dockerfile`, port 8080.
2. Env vars (set via Coolify's env-var UI, `is_secret=true` for the three
   secrets — never commit real values): `ENGRAM_OWNER_KEY`, `ENCRYPTION_KEY`,
   `MCP_BEARER_TOKEN`, `ENGRAM_PUBLIC_URL` (do NOT set `ENGRAM_PROJECT`: the remote server is multi-project),
   `ENGRAM_HTTP_ADDR`, `ENGRAM_DATA_DIR`. See `docs/REMOTE-MCP.md`.
3. Domain + TLS: set the application's FQDN to the desired subdomain;
   Coolify's Traefik integration issues a Let's Encrypt certificate
   automatically and redirects HTTP→HTTPS.
4. Healthcheck: GET `/health`, port 8080, 200 expected.

## Persistent storage — important gotcha found during validation

Setting `custom_docker_run_options: "--volume <name>:/home/engram/.engram"`
on the application does **not** reliably persist data across restarts/
redeploys in this Coolify setup — a first validation pass lost all data on
restart because no real Coolify-tracked volume existed (confirmed via the
`storages` list returning empty despite `custom_docker_run_options` being
set).

The fix: create the volume through Coolify's own persistent-storage
mechanism (Storages tab in the UI, or the `storages`/`create` API action)
targeting the exact container path `/home/engram/.engram`, and leave
`custom_docker_run_options` empty. After doing this and forcing a fresh
deploy, data was confirmed to survive both a `restart` and a full
`redeploy` (verified via a live `mem_save` → restart → `mem_get_observation`
→ redeploy → `mem_get_observation` round trip against the public HTTPS
endpoint).

## Known non-blocking anomaly

One `mem_update` call during validation returned a response envelope with
`project_source: "dir_basename"` / `project: "engram"` instead of the
expected `process_override` / `dadecore-standards` seen on every other
call (mem_save, mem_search, mem_get_observation, mem_context,
mem_current_project). The actual update still landed on the correct
observation (confirmed by immediately re-reading it), so this did not
cause data loss or a wrong-record edit, but the metadata inconsistency is
worth investigating — it suggests `mem_update`'s response-envelope project
resolution doesn't always honor `ENGRAM_PROJECT`/`--project` the same way
the other tool handlers do. Flagged for the `production` task /
maintenance follow-up.

## Adding the connector in Claude or ChatGPT (manual steps)

**Claude (claude.ai / Claude apps → Settings → Connectors):**
1. Add a custom connector with URL `https://engram-mcp.jcdcruz.com/mcp`.
2. Claude will discover the OAuth endpoints via
   `/.well-known/oauth-authorization-server` and redirect you through
   `/authorize`.
3. Enter the owner key when prompted by the gate page — this is the
   `ENGRAM_OWNER_KEY` value (shared with the user out of band, not stored
   in this repo).
4. Approve; Claude receives an access + refresh token pair and the
   connector should show Engram's 22 tools.

**ChatGPT (Settings → Connectors → Add custom connector / MCP):**
Same flow — point it at `https://engram-mcp.jcdcruz.com/mcp`, let it
follow the OAuth discovery + owner-key gate, and complete authorization.

**CLI-style / non-OAuth clients:** use `Authorization: Bearer
<MCP_BEARER_TOKEN>` directly against `/mcp` — no browser flow needed.
