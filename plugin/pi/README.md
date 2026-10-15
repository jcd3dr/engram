# Engram for Pi

Pi extension for Engram's hosted memory. Memory lives on the Engram MCP server
(`https://engram-mcp.jcdcruz.com/mcp`). This package adds the protocol, the project
rule and compaction recovery. Nothing runs locally: no Engram binary, no `engram serve`,
no local database.

## Install

```bash
pi install git:github.com/jcd3dr/engram
pi-engram init
```

Then restart Pi.

`pi-engram init` writes the `engram` entry into Pi's `mcp.json` (the remote MCP server)
and adds `pi-mcp-adapter` to `settings.json`, which is what exposes the `mem_*` tools.

## What the extension does

- Injects the Engram protocol and the project for the current directory into every agent turn.
- After a compaction, injects recovery instructions so the agent saves the compacted summary first.

## Project rule

Engram stores memory for many projects, so every `mem_*` call passes `project` explicitly.
The project is the basename of the git top-level directory (or of the directory outside git),
lowercased. This is the same rule as the Claude Code and OpenCode plugins.

## Configuration

- `ENGRAM_MCP_URL` sets the endpoint written by `pi-engram init` (for self-hosted servers).
- `pi-engram init --force` rewrites an existing `engram` entry.
- Entries left by earlier versions (with a local `command`) are replaced automatically.

## Troubleshooting

- `mem_*` tools missing: confirm `pi-mcp-adapter` is installed and `mcp.json` has the `engram` URL.
- `project is required`: the agent omitted `project`. The injected protocol tells it to pass it.
