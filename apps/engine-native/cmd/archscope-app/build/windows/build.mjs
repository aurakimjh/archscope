// Windows build entry point. Never mutates the downloaded Go module cache.
import { createHash } from "node:crypto";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, writeFileSync, rmSync, cpSync, chmodSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, dirname } from "node:path";

const env = { ...process.env, GOOS: "windows", CGO_ENABLED: process.env.CGO_ENABLED ?? "0" };
const arch = env.GOARCH ?? { x64: "amd64", ia32: "386", arm64: "arm64" }[process.arch];
if (!["386", "amd64", "arm64"].includes(arch)) throw new Error("Set GOARCH to 386, amd64 or arm64");
env.GOARCH = arch;
const args = process.argv.slice(2);
if (args.some(arg => ["-overlay", "-modfile"].some(flag => arg === flag || arg.startsWith(flag + "="))) || /(?:^|\s)-(?:overlay|modfile)(?:=|\s)/.test(env.GOFLAGS ?? "")) {
  throw new Error("External Go overlays/modfiles cannot be combined with the guarded Wails patch");
}
let temporary;
try {
  const flags = [];
  if (arch === "386") {
    const module = JSON.parse(execFileSync("go", ["list", "-m", "-json", "github.com/wailsapp/wails/v3"], { env, encoding: "utf8" }));
    if (module.Version !== "v3.0.0-alpha2.117" || module.Replace || !module.Dir) {
      throw new Error("Re-evaluate the Wails 386 patch for this dependency version/replacement");
    }
    const source = join(module.Dir, "pkg", "updater", "extract.go");
    const original = readFileSync(source);
    const hash = createHash("sha256").update(original).digest("hex");
    if (hash !== "c859d00e444195504695f80ba9c96841ba187ac0ab5d168fd920851864436286") {
      throw new Error("Wails updater source hash changed; refusing to patch unknown source");
    }
    // fmt's interface arguments default the untyped 2 GiB constant to int.
    // Keep the extraction limit identical; only its formatting type changes.
    const patched = original.toString("utf8").replaceAll('bytes", maxArchiveTotalSize)', 'bytes", int64(maxArchiveTotalSize))');
    if ((patched.match(/int64\(maxArchiveTotalSize\)/g) ?? []).length !== 2) throw new Error("Unexpected Wails patch shape");
    temporary = mkdtempSync(join(tmpdir(), "archscope-wails386-"));
    // Go forbids overlays beneath GOMODCACHE. A private module copy plus a
    // private modfile keeps both the checkout and the shared cache immutable.
    const replacement = join(temporary, "wails");
    cpSync(module.Dir, replacement, { recursive: true });
    const extracted = join(replacement, "pkg", "updater", "extract.go");
    chmodSync(extracted, 0o600);
    writeFileSync(extracted, patched);
    const projectMod = execFileSync("go", ["env", "GOMOD"], { env, encoding: "utf8" }).trim();
    const modfile = join(temporary, "go.mod");
    cpSync(projectMod, modfile);
    cpSync(join(dirname(projectMod), "go.sum"), join(temporary, "go.sum"));
    execFileSync("go", ["mod", "edit", "-modfile", modfile,
      `-replace=github.com/wailsapp/wails/v3=${replacement}`], { env, stdio: "inherit" });
    env.GOWORK = "off"; // -modfile is incompatible with workspace mode.
    flags.push("-modfile", modfile);
    process.stderr.write("Applying guarded Wails alpha2.117 386 formatting patch in a private copy\n");
  }
  const result = spawnSync("go", ["build", ...flags, ...args], { env, stdio: "inherit" });
  if (result.error) throw result.error;
  process.exitCode = result.status ?? 1;
} finally {
  if (temporary) rmSync(temporary, { recursive: true, force: true });
}
