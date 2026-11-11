#!/usr/bin/env node
// Real synthetic loopback proxy; no browser, GUI, system proxy or trust install.
import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
let duration = "120s", output = path.join(tmpdir(), `archscope-capture-soak-${Date.now()}.json`), race = false;
for (let i = 0; i < args.length; i++) {
  if (args[i] === "--duration") duration = args[++i];
  else if (args[i] === "--output") output = args[++i];
  else if (args[i] === "--race") race = true;
  else throw new Error(`Unknown argument: ${args[i]}`);
}
const match = /^(\d+)(s|m|h)$/.exec(duration ?? "");
const seconds = match ? Number(match[1]) * ({ s: 1, m: 60, h: 3600 }[match[2]]) : 0;
if (!Number.isSafeInteger(seconds) || seconds < 2 || seconds > 21600) throw new Error("Duration must be 2s..6h (integer s/m/h)");
if (!output) throw new Error("Output path is required");
output = path.resolve(output);
mkdirSync(path.dirname(output), { recursive: true });
const revision = spawnSync("git", ["rev-parse", "HEAD"], { cwd: root, encoding: "utf8" });
if (revision.error || revision.status !== 0) throw new Error("Cannot determine source revision");
console.log(`Running ${duration} common-engine soak; report: ${output}`);
const result = spawnSync("go", ["test", ...(race ? ["-race"] : []), "./cmd/archscope-app", "-run", "^TestCaptureProxySoakAcrossClearPauseAndReentry$", "-count=1", "-v", `-timeout=${seconds + 120}s`], {
  cwd: path.join(root, "apps/engine-native"), stdio: "inherit", timeout: (seconds + 180) * 1000,
  env: { ...process.env, ARCHSCOPE_CAPTURE_SOAK: duration, ARCHSCOPE_SOAK_REPORT: output, ARCHSCOPE_SOAK_COMMIT: revision.stdout.trim() },
});
if (result.error) throw result.error;
process.exitCode = result.status ?? 1;
