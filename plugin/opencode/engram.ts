/**
 * Engram — OpenCode plugin (remote MCP model)
 *
 * This plugin does NOT start or talk to a local Engram binary, server or
 * database. Memory lives on the hosted Engram MCP server, which OpenCode
 * connects to through the remote entry in opencode.json:
 *
 *   "mcp": {
 *     "engram": {
 *       "type": "remote",
 *       "url": "https://engram-mcp.jcdcruz.com/mcp",
 *       "enabled": true
 *     }
 *   }
 *
 * The plugin only teaches the agent the protocol and the project rule:
 *   - the server is multi-project, so every mem_* call passes `project` explicitly;
 *   - the project name is derived from the working directory;
 *   - after compaction, the agent must persist the compacted summary.
 */

import type { Plugin } from "@opencode-ai/plugin"

// ─── Memory Instructions ─────────────────────────────────────────────────────
// Injected into the system prompt so the agent knows the protocol.

const MEMORY_INSTRUCTIONS = `## Engram Persistent Memory — Protocol

You have access to Engram, a persistent memory system (remote MCP server) that survives across sessions and compactions.

### PROJECT RULE (mandatory)

Engram stores memory for many projects. Pass the \`project\` argument explicitly on EVERY mem_* call
(mem_save, mem_search, mem_context, mem_session_summary, mem_save_prompt, mem_timeline, mem_stats, mem_review, mem_doctor).
Do not rely on a default project: the server has none, and a call without \`project\` is rejected.

### DELIVERY (mandatory)

Memory operations are internal bookkeeping, never the user-facing answer.
Complete required memory work before composing the completed-task reply; then send the complete answer as the final message of the turn with no later tool calls.
If memory work fails or needs follow-up, still send the answer.

### WHEN TO SAVE (mandatory — not optional)

Call \`mem_save\` IMMEDIATELY after any of these:
- Bug fix completed
- Architecture or design decision made
- Non-obvious discovery about the codebase
- Configuration change or environment setup
- Pattern established (naming, structure, convention)
- User preference or constraint learned

Format for \`mem_save\`:
- **title**: Verb + what — short, searchable (e.g. "Fixed N+1 query in UserList", "Chose Zustand over Redux")
- **type**: bugfix | decision | architecture | discovery | pattern | config | preference
- **scope**: \`project\` (default) | \`personal\`
- **topic_key** (optional, recommended for evolving decisions): stable key like \`architecture/auth-model\`
- **content**:
  **What**: One sentence — what was done
  **Why**: What motivated it (user request, bug, performance, etc.)
  **Where**: Files or paths affected
  **Learned**: Gotchas, edge cases, things that surprised you (omit if none)

Topic rules:
- Different topics must not overwrite each other (e.g. architecture vs bugfix)
- Reuse the same \`topic_key\` to update an evolving topic instead of creating new observations
- If unsure about the key, call \`mem_suggest_topic_key\` first and then reuse it
- Use \`mem_update\` when you have an exact observation ID to correct

### WHEN TO SEARCH MEMORY

When the user asks to recall something — any variation of "remember", "recall", "what did we do",
"how did we solve", or the equivalent in the user's language, or references to past work:
1. First call \`mem_context\` — checks recent session history (fast, cheap)
2. If not found, call \`mem_search\` with relevant keywords (FTS5 full-text search)
3. If you find a match, use \`mem_get_observation\` for full untruncated content

Also search memory PROACTIVELY when:
- Starting work on something that might have been done before
- The user mentions a topic you have no context on — check if past sessions covered it
- The user's FIRST message references the project, a feature, or a problem — call \`mem_search\` with keywords from their message to check for prior work before responding

### SESSION CLOSE PROTOCOL (mandatory)

Before ending a session or saying "done" / "that's it", you MUST:
1. Call \`mem_session_summary\` with this structure:

## Goal
[What we were working on this session]

## Instructions
[User preferences or constraints discovered — skip if none]

## Discoveries
- [Technical findings, gotchas, non-obvious learnings]

## Accomplished
- [Completed items with key details]

## Next Steps
- [What remains to be done — for the next session]

## Relevant Files
- path/to/file — [what it does or what changed]

This is NOT optional. If you skip this, the next session starts blind.

### AFTER COMPACTION

If you see a message about compaction or context reset, or if you see "FIRST ACTION REQUIRED" in your context:
1. IMMEDIATELY call \`mem_session_summary\` with the compacted summary content — this persists what was done before compaction
2. Then call \`mem_context\` to recover any additional context from previous sessions
3. Only THEN continue working

Do not skip step 1. Without it, everything done before compaction is lost from memory.
`

// ─── Helpers ─────────────────────────────────────────────────────────────────

/**
 * Project name for a directory: basename of the git top-level directory (or of the
 * directory itself outside git), lowercased. Same rule as the Claude Code and Pi plugins.
 */
function extractProjectName(directory: string): string {
  try {
    const result = Bun.spawnSync(["git", "-C", directory, "rev-parse", "--show-toplevel"])
    if (result.exitCode === 0) {
      const root = result.stdout?.toString().trim()
      if (root) return (root.split("/").pop() ?? "unknown").toLowerCase()
    }
  } catch {}
  return (directory.split("/").pop() ?? "unknown").toLowerCase()
}

// ─── Plugin Export ───────────────────────────────────────────────────────────

export const Engram: Plugin = async (ctx) => {
  const project = extractProjectName(ctx.directory)

  return {
    // ─── System Prompt: protocol + project for this session ─────────
    // Appends to the last existing system entry instead of pushing a new one:
    // some chat templates (Qwen3.5, Mistral via llama.cpp) accept only a single
    // system block at the beginning. See GitHub issue #23.
    "experimental.chat.system.transform": async (_input, output) => {
      const block =
        MEMORY_INSTRUCTIONS +
        `\nPROJECT FOR THIS SESSION: "${project}". Pass project="${project}" explicitly on EVERY mem_* call.`
      if (output.system.length > 0) {
        output.system[output.system.length - 1] += "\n\n" + block
      } else {
        output.system.push(block)
      }
    },

    // ─── Compaction: make the new agent persist the compacted summary ─
    // Compaction is system-triggered; the new agent starts from the compacted
    // summary, so the instruction must survive into that summary.
    "experimental.session.compacting": async (_input, output) => {
      output.context.push(
        `CRITICAL INSTRUCTION FOR COMPACTED SUMMARY:\n` +
          `The agent has access to Engram persistent memory via MCP tools.\n` +
          `You MUST include the following instruction at the TOP of the compacted summary:\n\n` +
          `"FIRST ACTION REQUIRED: Call mem_session_summary with the content of this compacted summary. ` +
          `Use project: '${project}'. This preserves what was accomplished before compaction. Do this BEFORE any other work."\n\n` +
          `This is NOT optional. Without this, everything done before compaction is lost from memory.`
      )
    },
  }
}
