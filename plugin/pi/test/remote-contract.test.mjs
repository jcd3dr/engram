import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const source = readFileSync(join(ROOT, "index.ts"), "utf8");
const REMOTE_URL = "https://engram-mcp.jcdcruz.com/mcp";

test("index.ts has no local Engram binary, server or HTTP path", () => {
  for (const forbidden of ["engram serve", "ENGRAM_URL", "ENGRAM_BIN", "127.0.0.1", "fetch(", "spawn("]) {
    assert.equal(source.includes(forbidden), false, `index.ts must not contain ${forbidden}`);
  }
});

test("index.ts keeps the project rule and the compaction recovery hooks", () => {
  assert.ok(source.includes("PROJECT RULE"));
  assert.ok(source.includes('Pass project="${project}" explicitly on EVERY mem_* call.'));
  assert.ok(source.includes('pi.on("before_agent_start"'));
  assert.ok(source.includes('pi.on("session_compact"'));
});

test("pi-engram init writes the remote MCP entry and replaces legacy local entries", () => {
  const agentDir = mkdtempSync(join(tmpdir(), "pi-engram-"));
  writeFileSync(
    join(agentDir, "mcp.json"),
    JSON.stringify({ mcpServers: { engram: { command: "node", args: ["-e", "x"] } } }),
  );
  execFileSync(process.execPath, [join(ROOT, "cli.js"), "init"], {
    env: { ...process.env, PI_CODING_AGENT_DIR: agentDir },
    stdio: "ignore",
  });
  const mcp = JSON.parse(readFileSync(join(agentDir, "mcp.json"), "utf8"));
  assert.deepEqual(mcp.mcpServers.engram, { url: REMOTE_URL, lifecycle: "lazy", directTools: false });
});
