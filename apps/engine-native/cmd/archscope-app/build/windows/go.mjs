// Shared Windows build/test runner for native and cross builds.
// Wails 3.1.0 registers an eight-byte POINT value as one callback argument.
// Go's 386 callback bridge only accepts word-sized arguments. Split the native
// x86 stack slots in a guarded temporary SDK copy; never edit the module cache.
import { spawnSync, execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, mkdtempSync, rmSync, cpSync, chmodSync, readdirSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
export function runWindowsGo(command, inputArgs) {
  if (!['build', 'test'].includes(command)) throw new Error('Expected a Windows build or test command');
  const env = { ...process.env, GOOS: "windows", CGO_ENABLED: process.env.CGO_ENABLED ?? "0" };
  const arch = env.GOARCH ?? { x64: "amd64", ia32: "386", arm64: "arm64" }[process.arch];
  if (!["386", "amd64", "arm64"].includes(arch)) throw new Error("Set GOARCH to 386, amd64 or arm64");
  env.GOARCH = arch;
  let temporary;
  function writableDirectories(directory) {
    // Go's module cache uses read-only directories. cpSync retains those modes
    // on Unix, where rm's force option does not grant directory write access.
    chmodSync(directory, 0o700);
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      if (entry.isDirectory()) writableDirectories(path.join(directory, entry.name));
    }
  }
  try {
    const args = [...inputArgs];
    if (arch === "386") {
      if (args.some(arg => /^-modfile(?:=|$)/.test(arg))) throw new Error("An additional modfile must be merged with the Wails 386 modfile");
      const module = JSON.parse(execFileSync("go", ["list", "-m", "-json", "github.com/wailsapp/wails/v3"], { env, encoding: "utf8" }));
      if (module.Version !== "v3.1.0" || module.Replace) throw new Error("Revalidate the 386 POINT callback fix for this Wails version");
      // go list omits Dir when a clean module cache contains only metadata.
      // Download the selected pinned version before inspecting its source.
      if (!module.Dir) {
        const downloaded = JSON.parse(execFileSync("go", ["mod", "download", "-json", `${module.Path}@${module.Version}`], { env, encoding: "utf8" }));
        if (downloaded.Path !== module.Path || downloaded.Version !== module.Version || !downloaded.Dir) throw new Error("Wails source download did not resolve the pinned module");
        module.Dir = downloaded.Dir;
      }
      const original = path.join(module.Dir, "pkg", "w32", "idroptarget.go");
      const bytes = readFileSync(original);
      if (createHash("sha256").update(bytes).digest("hex") !== "c2ba3162f5697609df5158902c4ba1400dad839279ecc06175fd72e74eb7ed12") throw new Error("Unexpected Wails IDropTarget source; refusing an unverified patch");
      let source = bytes.toString("utf8");
      for (const method of ["DragEnter", "DragOver", "Drop"]) {
        const registration = `\t\t_iDropTarget${method},`;
        if (!source.includes(registration)) throw new Error(`Missing ${method} registration`);
        source = source.replace(registration, `\t\t_iDropTarget${method}386,`);
      }
      source += `
// POINTL occupies two consecutive 32-bit stack slots in the x86 COM ABI.
func _iDropTargetDragEnter386(this uintptr, data *IDataObject, keys DWORD, x, y int32, effect *DWORD) uintptr {
    return _iDropTargetDragEnter(this, data, keys, POINT{X: x, Y: y}, effect)
}
func _iDropTargetDragOver386(this uintptr, keys DWORD, x, y int32, effect *DWORD) uintptr {
    return _iDropTargetDragOver(this, keys, POINT{X: x, Y: y}, effect)
}
func _iDropTargetDrop386(this uintptr, data *IDataObject, keys DWORD, x, y int32, effect *DWORD) uintptr {
    return _iDropTargetDrop(this, data, keys, POINT{X: x, Y: y}, effect)
}
`;
      temporary = mkdtempSync(path.join(tmpdir(), "archscope-wails386-"));
      const sdk = path.join(temporary, "wails");
      cpSync(module.Dir, sdk, { recursive: true });
      const replacement = path.join(sdk, "pkg", "w32", "idroptarget.go");
      chmodSync(replacement, 0o600);
      writeFileSync(replacement, source);
      // The Ptr API names are C macros on Win32; user32 exports the Long names.
      const user32 = path.join(module.Dir, "pkg", "w32", "user32.go");
      const userBytes = readFileSync(user32);
      if (createHash("sha256").update(userBytes).digest("hex") !== "42d07d118ffc065ff408b57ab224c637f57e0bb4452b38da04708e5ec8e93d77") throw new Error("Unexpected Wails user32 source; refusing an unverified patch");
      let userSource = userBytes.toString("utf8");
      for (const name of ["GetWindowLong", "SetWindowLong", "SetClassLong"]) {
        const from = `NewProc("${name}PtrW")`;
        if (!userSource.includes(from)) throw new Error(`Missing ${name}PtrW import`);
        userSource = userSource.replace(from, `NewProc("${name}W")`);
      }
      const userReplacement = path.join(sdk, "pkg", "w32", "user32.go");
      chmodSync(userReplacement, 0o600);
      writeFileSync(userReplacement, userSource);
      const mainMod = execFileSync("go", ["env", "GOMOD"], { env, encoding: "utf8" }).trim();
      const modfile = path.join(temporary, "go.mod");
      writeFileSync(modfile, readFileSync(mainMod, "utf8") + `\nreplace github.com/wailsapp/wails/v3 => ${JSON.stringify(sdk.replaceAll("\\", "/"))}\n`);
      cpSync(mainMod.replace(/\.mod$/, ".sum"), path.join(temporary, "go.sum"));
      env.GOWORK = "off"; // Go rejects -modfile in workspace mode.
      args.unshift(`-modfile=${modfile}`);
    }
    const result = spawnSync("go", [command, ...args], { env, stdio: "inherit" });
    if (result.error) throw result.error;
    return result.status ?? 1;
  } finally {
    if (temporary) {
      writableDirectories(temporary); // Confined to our mkdtemp tree; no symlinks.
      rmSync(temporary, { recursive: true, force: true });
    }
  }
}
