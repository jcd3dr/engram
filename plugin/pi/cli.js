#!/usr/bin/env node
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, join } from "node:path";

const PACKAGE_NAME = "git:github.com/jcd3dr/engram";
const MCP_ADAPTER_PACKAGE = "npm:pi-mcp-adapter";
const ENGRAM_MCP_URL = process.env.ENGRAM_MCP_URL?.trim() || "https://engram-mcp.jcdcruz.com/mcp";
const HELP = `pi-engram

Usage:
  pi-engram init [--force]

Writes Pi's Engram MCP entry (the hosted Engram server) into the Pi agent dir and
ensures pi-mcp-adapter is declared in settings.json. The Pi extension itself is
loaded by installing the package with: pi install git:github.com/jcd3dr/engram

Environment:
  ENGRAM_MCP_URL  Engram MCP endpoint to write (default: https://engram-mcp.jcdcruz.com/mcp)
`;

function getAgentDir() {
  return process.env.PI_CODING_AGENT_DIR || join(homedir(), ".pi", "agent");
}

function readJsonObject(filePath) {
  if (!existsSync(filePath)) return {};
  const parsed = JSON.parse(readFileSync(filePath, "utf-8"));
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(`${filePath} must contain a JSON object`);
  }
  return parsed;
}

function writeJsonObject(filePath, data) {
  mkdirSync(dirname(filePath), { recursive: true });
  writeFileSync(filePath, `${JSON.stringify(data, null, 2)}\n`, "utf-8");
}

function ensurePackage(settingsPath, packageName) {
  const settings = readJsonObject(settingsPath);
  const packages = Array.isArray(settings.packages) ? settings.packages : [];
  if (!packages.includes(packageName)) {
    settings.packages = [...packages, packageName];
    writeJsonObject(settingsPath, settings);
    return true;
  }
  return false;
}

function createEngramServerConfig() {
  return {
    url: ENGRAM_MCP_URL,
    lifecycle: "lazy",
    directTools: false,
  };
}

// Entries written by earlier versions launch a local `engram mcp` process (they carry
// `command`). They are replaced automatically so Pi does not keep a local binary path.
function isLegacyLocalEntry(entry) {
  return Boolean(entry) && typeof entry === "object" && "command" in entry;
}

function ensureMcpConfig(mcpPath, force) {
  const config = readJsonObject(mcpPath);
  const existingServers = config.mcpServers && typeof config.mcpServers === "object" && !Array.isArray(config.mcpServers)
    ? config.mcpServers
    : {};
  const existing = existingServers.engram;

  if (existing && !force && !isLegacyLocalEntry(existing)) {
    return "kept";
  }

  config.mcpServers = {
    ...existingServers,
    engram: createEngramServerConfig(),
  };
  writeJsonObject(mcpPath, config);
  return isLegacyLocalEntry(existing) ? "replaced-legacy" : "written";
}

function init() {
  const force = process.argv.includes("--force");
  const agentDir = getAgentDir();
  const settingsPath = join(agentDir, "settings.json");
  const mcpPath = join(agentDir, "mcp.json");

  const adapterChanged = ensurePackage(settingsPath, MCP_ADAPTER_PACKAGE);
  const packageChanged = ensurePackage(settingsPath, PACKAGE_NAME);
  const mcpResult = ensureMcpConfig(mcpPath, force);

  console.log(`Pi agent dir: ${agentDir}`);
  console.log(`${adapterChanged ? "Added" : "Kept"} ${MCP_ADAPTER_PACKAGE} in settings.json`);
  console.log(`${packageChanged ? "Added" : "Kept"} ${PACKAGE_NAME} in settings.json`);
  const mcpMessage = {
    written: "Wrote",
    "replaced-legacy": "Replaced legacy local entry with",
    kept: "Kept existing",
  }[mcpResult];
  console.log(`${mcpMessage} Engram MCP server (${ENGRAM_MCP_URL}) in mcp.json`);
  if (mcpResult === "kept") {
    console.log("Run pi-engram init --force to rewrite the entry (for example after changing ENGRAM_MCP_URL).");
  }
}

const command = process.argv[2];
if (command === "init") {
  init();
} else {
  console.log(HELP);
}
