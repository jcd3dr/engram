---
name: engram-memory-protocol
description: "Persistent memory protocol for the remote Engram MCP shared by all DadeCore projects. Trigger: decisions, bug fixes, discoveries, preferences, session closure, compaction."
license: Apache-2.0
metadata:
  author: dadecore
  version: "2.0"
---

# Engram Persistent Memory — Protocol (remote)

Engram is a remote MCP server (connector "engram") that stores memory for many projects. It is shared by every client: Claude Code, claude.ai, ChatGPT.

## Delivery

Memory operations are internal bookkeeping, never the user-facing answer.
Complete required memory work before composing the completed-task reply; then send the complete answer as the final message of the turn with no later tool calls.
If memory work fails or needs follow-up, still send the answer.

## Project rule (mandatory)

Pass the `project` argument explicitly on EVERY mem_* call. The server has no default project. Use the name of the repository or project the work belongs to, in lowercase. Never omit it and never reuse the project of a different task.

Tools that take `project`: mem_save, mem_search, mem_context, mem_session_summary, mem_save_prompt, mem_timeline, mem_stats, mem_review, mem_doctor.

## Save (proactive)

Call mem_save right after: a decision or convention, a bug fix (include the root cause), a non-obvious discovery or gotcha, a configuration change, or a user preference. Use the structured format: What, Why, Where, Learned. Reuse a stable topic_key for topics that evolve, so the entry is updated instead of duplicated.

## Search (before answering)

- Recall requests: mem_context first, then mem_search.
- On the first message that mentions the project, a feature, or a past problem: mem_search with those keywords, before responding.
- For full content of a result: mem_get_observation.

## Close and compaction

- Before saying the work is done: mem_session_summary with Goal, Discoveries, Accomplished, Next Steps, Relevant Files.
- After compaction: mem_session_summary with the compacted summary, then mem_context, then continue.
