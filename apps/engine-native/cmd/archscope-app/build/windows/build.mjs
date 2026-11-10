// Windows build entry point shared by native/cross builds.
// Wails 3.1.0 fixes the former 386 updater overflow upstream. No local SDK
// copy, dependency rewrite or Go overlay is needed for either architecture.
import { spawnSync } from "node:child_process";
const env = { ...process.env, GOOS: "windows", CGO_ENABLED: process.env.CGO_ENABLED ?? "0" };
const arch = env.GOARCH ?? { x64: "amd64", ia32: "386", arm64: "arm64" }[process.arch];
if (!["386", "amd64", "arm64"].includes(arch)) throw new Error("Set GOARCH to 386, amd64 or arm64");
env.GOARCH = arch;
const result = spawnSync("go", ["build", ...process.argv.slice(2)], { env, stdio: "inherit" });
if (result.error) throw result.error;
process.exitCode = result.status ?? 1;
