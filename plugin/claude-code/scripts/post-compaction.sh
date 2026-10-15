#!/bin/bash
# Engram (remote) — compaction recovery hook for Claude Code.
set -u
INPUT=$(cat)
CWD=$(printf '%s' "$INPUT" | jq -r '.cwd // empty' 2>/dev/null)
[ -z "$CWD" ] && CWD="$PWD"
ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || printf '%s' "$CWD")
PROJECT=$(basename "$ROOT" | tr '[:upper:]' '[:lower:]')

cat <<PROTOCOL
## Engram — CONTEXT COMPACTED

PROJECT: "${PROJECT}" (pass project="${PROJECT}" on every mem_* call)

Recover memory in this order before continuing:
1. Call mem_session_summary (project="${PROJECT}") with the compacted summary, so the work done before compaction is persisted.
2. Call mem_context (project="${PROJECT}") to recover prior session context.
3. Then continue the task.

## Delivery

Memory operations are internal bookkeeping, never the user-facing answer.
Complete required memory work before composing the completed-task reply; then send the complete answer as the final message of the turn with no later tool calls.
If memory work fails or needs follow-up, still send the answer.
PROTOCOL
exit 0
