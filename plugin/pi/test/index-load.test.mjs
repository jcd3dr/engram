import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

// Regression: index.ts had an unclosed template literal (MEMORY_INSTRUCTIONS),
// so Pi failed to load the extension while every other test still passed.
// Load the real entry point through Node's TypeScript stripping.
const here = dirname(fileURLToPath(import.meta.url));
const entry = join(here, "..", "index.ts");

test("index.ts parses and exports projectNameForDirectory", () => {
  const out = execFileSync(
    process.execPath,
    [
      "--experimental-strip-types",
      "--no-warnings",
      "--input-type=module",
      "-e",
      `const m = await import(${JSON.stringify(entry)}); console.log(JSON.stringify(Object.keys(m)));`,
    ],
    { encoding: "utf8" },
  );
  assert.match(out, /projectNameForDirectory/);
});

test("MEMORY_INSTRUCTIONS protocol is embedded in the extension source", () => {
  const out = execFileSync(
    process.execPath,
    [
      "--experimental-strip-types",
      "--no-warnings",
      "--input-type=module",
      "-e",
      `import { readFileSync } from "node:fs"; const src = readFileSync(${JSON.stringify(entry)}, "utf8"); console.log(src.includes("const MEMORY_INSTRUCTIONS = \`## Engram") ? "ok" : "missing");`,
    ],
    { encoding: "utf8" },
  );
  assert.equal(out.trim(), "ok");
});
