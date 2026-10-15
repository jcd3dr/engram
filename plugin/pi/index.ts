/**
 * Engram — Pi extension adapter (remote MCP model)
 *
 * This extension does NOT start or talk to a local Engram binary, server or
 * database. Memory lives on the hosted Engram MCP server. Pi reaches it through
 * pi-mcp-adapter, using the remote entry that `pi-engram init` writes to mcp.json.
 *
 * The extension only:
 *   - injects the Engram protocol and the project for the current directory into
 *     every agent turn (same rule as the Claude Code and OpenCode plugins);
 *   - injects compaction-recovery instructions after a compaction.
 */

import { execFileSync } from "node:child_process";
import { basename } from "node:path";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { buildRecoveryNotice } from "./compaction-recovery.js";

interface SessionContext {
  cwd: string;
}

interface AgentStartEvent {
  systemPrompt: string;
  prompt?: string;
}

// ─── Memory Instructions ─────────────────────────────────────────────────────

const MEMORY_INSTRUCTIONS = ## Engram Persistent Memory — Protocol

You have access to Engram, a persistent memory system (remote MCP server) that survives across sessions and compactions.

### PROJECT RULE (mandatory)

Engram stores memory for many projects. Pass the \`project\` argument explicitly on EVERY mem_* call
(mem_save, mem_search, mem_context, mem_session_summary, mem_save_prompt, mem_timeline, mem_stats, mem_review, mem_doctor).
Do not rely on a default project: the server has none, and a call without \`project\` is rejected.

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

// ─── Project ─────────────────────────────────────────────────────────────────

/**
 * Project name for a directory: basename of the git top-level directory (or of the
 * directory itself outside git), lowercased. Same rule as the Claude Code and OpenCode plugins.
 */
export function projectNameForDirectory(directory: string): string {
  try {
    const root = execFileSync("git", ["-C", directory, "rev-parse", "--show-toplevel"], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
    if (root) return basename(root).toLowerCase();
  } catch {
    // Not a git repository: fall back to the directory name.
  }
  return (basename(directory) || "unknown").toLowerCase();
}

// ─── Plugin ──────────────────────────────────────────────────────────────────

let pendingRecoveryNotice: string | undefined;

export default function registerEngram(pi: ExtensionAPI) {
  pi.on("session_compact", async (_event: unknown, ctx: SessionContext) => {
    pendingRecoveryNotice = buildRecoveryNotice(projectNameForDirectory(ctx.cwd), undefined);
  });

  pi.on("before_agent_start", async (event: AgentStartEvent, ctx: SessionContext) => {
    const project = projectNameForDirectory(ctx.cwd);
    const block =
      MEMORY_INSTRUCTIONS +
      `\nPROJECT FOR THIS SESSION: "${project}". Pass project="${project}" explicitly on EVERY mem_* call.`;
    let systemPrompt = event.systemPrompt.length > 0 ? `${event.systemPrompt}\n\n${block}` : block;
    if (pendingRecoveryNotice !== undefined) {
      systemPrompt = `${systemPrompt}\n\n${pendingRecoveryNotice}`;
      pendingRecoveryNotice = undefined;
    }
    return { systemPrompt };
  });
}
