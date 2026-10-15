#!/bin/bash
# Engram (remote) — SessionStart hook for Claude Code.
# Talks to no local server and no local database. Derives the project name
# from the working directory and tells the agent to pass it on every mem_* call.
set -u
INPUT=$(cat)
CWD=$(printf '%s' "$INPUT" | jq -r '.cwd // empty' 2>/dev/null)
[ -z "$CWD" ] && CWD="$PWD"
ROOT=$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || printf '%s' "$CWD")
PROJECT=$(basename "$ROOT" | tr '[:upper:]' '[:lower:]')

cat <<PROTOCOL
## Engram Persistent Memory — ACTIVE PROTOCOL (remote)

Engram is a remote MCP server (connector "engram"). It stores memory for many projects.

PROJECT FOR THIS SESSION: "${PROJECT}"
Pass project="${PROJECT}" explicitly on EVERY mem_* call (mem_save, mem_search, mem_context, mem_session_summary, mem_save_prompt, mem_timeline, mem_stats, mem_review, mem_doctor). Never omit it.

### Save proactively with mem_save
After each decision, bug fix, non-obvious discovery, convention, or user preference, call mem_save at once. Use a stable topic_key for evolving topics.

### Search before answering
On the first message that references the project, a feature, or a past problem, call mem_search (or mem_context) with the project before responding.

### Close the session
Before saying the work is done, call mem_session_summary with: Goal, Discoveries, Accomplished, Next Steps, Relevant Files.
PROTOCOL
exit 0
