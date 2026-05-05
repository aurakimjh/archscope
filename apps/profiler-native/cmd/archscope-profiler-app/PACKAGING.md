# Multi-platform packaging

Targets land under `bin/` — one binary plus the platform-native installer
shape. All commands assume `wails3` is on `PATH` (`go install
github.com/wailsapp/wails/v3/cmd/wails3@latest`).

## macOS — `.app` bundle

```bash
# host arch (arm64 on Apple Silicon, amd64 on Intel)
wails3 task package:darwin
# → bin/archscope-profiler.app
```

Cross-architecture macOS builds (arm64 ↔ amd64) require Xcode 26+ for the
target SDK; the CI matrix in `.github/workflows/profiler-native.yml` runs
each architecture on a host of the matching arch instead.

The .app is **ad-hoc signed**. For Mac App Store / Gatekeeper-friendly
distribution, point the Taskfile at a Developer ID + notarytool credentials.
Out of scope for this slice — see T-243 follow-ups in `work_status.md`.

## Windows — `.exe` (NSIS) and optional MSIX

```bash
wails3 task package:windows
# → bin/archscope-profiler.exe + NSIS installer
```

On non-Windows hosts the build is cross-compiled through Docker (~800 MB
image, downloaded once). Run `wails3 task setup:docker` first.

**WebView2 prerequisite.** Windows 11 ships with WebView2 by default. On
Windows 10, end-users must install the WebView2 Evergreen runtime first
(NSIS bootstrapper handles this automatically; standalone `.exe` does not).

## Linux — AppImage / .deb / .rpm

```bash
wails3 task package:linux
# → bin/archscope-profiler-x86_64.AppImage
```

WebKitGTK is required at runtime (`apt install libwebkit2gtk-4.1-0` on
Debian/Ubuntu, `dnf install webkit2gtk4.1` on Fedora). The AppImage bundles
everything else. nfpm targets for `.deb` / `.rpm` live under
`build/linux/nfpm/` if a per-distro package is preferred over AppImage.

## Cross-compile matrix (CI)

`.github/workflows/profiler-native.yml` exercises:

| Host | Targets |
|---|---|
| `ubuntu-latest` | linux/amd64 AppImage + windows/amd64 NSIS via docker |
| `macos-14` (arm64) | darwin/arm64 .app |
| `macos-13` (amd64) | darwin/amd64 .app |
| `windows-latest` | windows/amd64 NSIS (native) |

CI artifacts (`bin/*`) are uploaded on every push so reviewers can pull a
binary without setting up a build env.

## Build size targets (post-Python-removal)

| Target | Expected | Measured (Apple M5 Max) |
|---|---|---|
| darwin/arm64 raw | ≤ 20 MB | 8.4 MB |
| darwin/arm64 .app | ≤ 25 MB | 10 MB |
| windows/amd64 .exe | ≤ 22 MB | tbd (CI) |
| linux/amd64 AppImage | ≤ 30 MB | tbd (CI) |
