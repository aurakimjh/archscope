#!/usr/bin/env node
// Repository guard: SDK template updates must preserve the desktop-only product.
import assert from "node:assert/strict";
import { existsSync, readFileSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const app = path.join(root, "apps/engine-native/cmd/archscope-app");
const read = (name) => readFileSync(path.join(app, name), "utf8");
for (const name of ["Taskfile.yml", "build/Taskfile.yml"]) {
  assert.doesNotMatch(read(name), /^  (?:(?:build|run):(?:server|docker)|(?:ios|android)(?::[^\n]*)?|frontend:vendor:puppertino|update:build-assets):/m,
    `${name}: unsupported product target restored`);
}
for (const name of ["build/android", "build/ios", "build/docker/Dockerfile.server"]) {
  assert.equal(existsSync(path.join(app, name)), false, `Unsupported template restored: ${name}`);
}
const pkg = JSON.parse(read("frontend/package.json"));
assert.equal(pkg.scripts.preview, undefined, "Standalone browser preview restored");
const lock = JSON.parse(read("frontend/package-lock.json"));
for (const name of Object.keys(lock.packages)) {
  assert.doesNotMatch(name, /(?:^|\/)node_modules\/(?:electron(?:-.*)?|playwright(?:-.*)?|@playwright\/[^/]+|puppeteer(?:-.*)?)$/i,
    `Browser/Electron product dependency: ${name}`);
}
assert.match(read("main.go"), /app\.Window\.NewWithOptions\(/, "Native window entry missing");
assert.doesNotMatch(read("main.go"), /Server\s*:\s*application\.ServerOptions/, "Server product entry restored");
assert.match(read("frontend/src/main.tsx"), /hasNativeDesktopBridge\(window\)/, "Native frontend gate missing");

// Optional integration check: exercise the real compiler, without launching UI.
if (process.argv.includes("--check-server-build")) {
  const temporary = mkdtempSync(path.join(tmpdir(), "archscope-desktop-only-"));
  try {
    // The development asset FS keeps this check usable before frontend build.
    const result = spawnSync("go", ["build", "-tags", "server", "-o", path.join(temporary, "forbidden-server"), "."], {
      cwd: app, encoding: "utf8", timeout: 180_000,
    });
    assert.ifError(result.error);
    assert.notEqual(result.status, 0, "Server product build unexpectedly succeeded");
    assert.match(result.stderr, /undefined: ArchScope_requires_native_Wails_desktop_server_and_mobile_builds_are_disabled/,
      `Server build failed for an unrelated reason: ${result.stderr}`);
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
}
console.log("Desktop-only checks passed" + (process.argv.includes("--check-server-build") ? " (server compilation rejected)" : ""));
