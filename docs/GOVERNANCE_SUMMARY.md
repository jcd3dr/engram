# GOVERNANCE_SUMMARY

> DARS Standard v1.17

Generated from: DARS v1.17

## Entry

1. Read `AGENTS.md` (repository-wide instructions and context map).
2. Use the DARS governance skill when available. Otherwise use this file.
3. For implementation work, read `docs/CONVENTIONS.md`.
4. Read `docs/PLAN.md` to learn current state before resuming work.

## Progressive context

Do not read every deep artifact on every task. Load only what the task needs:

- Meaning of the product or a requirement: `docs/SPEC.md`.
- Where code lives and what depends on what: `docs/REPO_MAP.md`.
- Why something is the way it is: `docs/adr/` (read the relevant ADRs; do not read all of them).
- Current state and blockers: `docs/PLAN.md`.

## Durable-impact maintenance

Before finishing a task, decide whether it changed something durable:

- product behavior, scope, or a contract → `docs/SPEC.md`;
- repository structure, ownership, or navigation → `docs/REPO_MAP.md`;
- current focus, blockers, or next steps → `docs/PLAN.md`;
- an engineering rule → `docs/CONVENTIONS.md`;
- a significant, hard-to-reverse decision → a new ADR under `docs/adr/`;
- user-visible or published behavior → `CHANGELOG.md`;
- agent entry points or instructions → `AGENTS.md` and the native agent surfaces.

Use meaning, not line count, to decide. A trivial fix changes none of these.

## PLAN rule

`docs/PLAN.md` is the persistent current-state record. If work stops mid-task, update it so another agent can resume from it without this session's memory.

## Decision-history rule

Read the relevant ADR before changing an area it governs. Never edit an accepted ADR. A changed decision gets a new ADR that supersedes the old one. `docs/adr/0000-baseline.md` records the reconciled starting point of this repository.

## Non-modification principle

DARS documentation and context surfaces do not authorize changes to source code, behavior, dependencies, or deployment. Defects found while reading documents are recorded, not silently fixed.

## Recovery

If a document contradicts the code, the code wins for the current behavior. Record the contradiction in `docs/PLAN.md` or as an ADR, then correct the document in the same change that resolves it. If a document is missing, check `docs/adr/0000-baseline.md` before creating a new one.
