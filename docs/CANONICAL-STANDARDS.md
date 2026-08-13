# Canonical DadeCore standards migrated into Engram

Source: `jcd3dr/dadecore-standards-memory-test` (temporary staging repo, kept
until this migration was verified). Destination: the production Engram
Remote MCP at `https://engram-mcp.jcdcruz.com`, project `dadecore-standards`
(the fixed project pin for the remote transport — see `docs/REMOTE-MCP.md`).

## Mapping

| Staging file | Engram topic_key | type | Engram observation id |
|---|---|---|---|
| `README.md` | `dadecore/standards/readme` | discovery | 2 |
| `standards/mcp-private-standard.md` | `dadecore/standards/private-mcp` | decision | 3 |
| `standards/remote-mcp-with-auxiliary-api-pattern.md` | `dadecore/standards/remote-mcp-auxiliary-api` | architecture | 4 |
| `standards/remote-skill-bridge-pattern.md` | `dadecore/standards/remote-skill-bridge` | architecture | 5 |
| `standards/upstream-wrapper-hub-pattern.md` | `dadecore/standards/upstream-wrapper-hub` | architecture | 6 |

topic_key makes re-saving (`mem_save` with the same `topic_key`) an upsert —
running the migration twice produced the same observation ids both times,
confirming no duplication.

Two of the migrated documents (`remote-mcp-auxiliary-api`,
`upstream-wrapper-hub`) were saved with an added "Nota de aplicación"
section documenting the concrete architecture decision made for *this*
project (additive single-repo transport, not a Hub/Wrapper) and cross-
referencing `dadecore/standards/private-mcp` and each other — so a future
reader (human or model) querying either pattern immediately sees how it was
actually applied (or not applied) here, instead of only the abstract rule.

## Verified behavior

- `mem_search("private MCP auth owner key PKCE")` correctly surfaces
  `dadecore/standards/private-mcp` (#3) as the top/only result.
- `mem_get_observation(3)` returns the complete original Spanish-language
  standard text verbatim, byte-for-byte matching the source file — no
  summarization.
- `mem_update` was exercised on `dadecore/standards/upstream-wrapper-hub`
  (#6): a new "Nota adicional de validación" section was added via the
  `content` field passed to `mem_update`. Re-reading the record afterward
  confirmed BOTH the original content (including the closing line "Adaptar
  alrededor antes que modificar dentro.") AND the new section were present —
  in this Engram version, `mem_update`'s `content` argument is a *replace*
  of the full stored content, not a partial-field merge, so the migration
  script explicitly built the new full text by concatenation before calling
  `mem_update` rather than relying on partial-field semantics.
- All 5 records (ids 2–6) were confirmed present and byte-identical via
  `mem_get_observation` after both a container `restart` and a full
  `redeploy` of the production Coolify application.
- `mem_save`'s conflict-detection surfaced `judgment_required=true` against
  unrelated scratch observations left over from the `validation` task
  (e.g. a `PERSISTENCE-MARKER-v2` test record); these were resolved via
  `mem_judge(relation="not_conflict")` since they are unrelated content, and
  the leftover test observation was subsequently `mem_delete`d as cleanup.

## Do not delete the staging repo yet

Per the task constraint, `jcd3dr/dadecore-standards-memory-test` was left
untouched — this migration only reads from it. Deleting it is a follow-up
decision for the user once they're satisfied the canonical copies in Engram
are authoritative.
