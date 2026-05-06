# Multi-platform packaging (v0.3.0+)

Targets land under `bin/` — one binary plus the platform-native installer
shape. All commands assume `wails3` is on `PATH` (`go install
github.com/wailsapp/wails/v3/cmd/wails3@latest`) and `task` is on `PATH`
(`brew install go-task` on macOS, `choco install go-task` on Windows).

## macOS — `.app` bundle + `.dmg`

```bash
# host arch (arm64 on Apple Silicon, amd64 on Intel)
task package
# → bin/archscope-profiler.app  (ad-hoc signed)

task darwin:package:dmg
# → bin/archscope-profiler-<arch>.dmg
```

DMG is built via stdlib `hdiutil` (UDZO compression) — no `create-dmg` or
similar third-party tool needed. Typical size: ~5 MB (compressed from
~11 MB raw .app).

Cross-architecture macOS builds (arm64 ↔ amd64) require Xcode 26+ for the
target SDK; the GHA release matrix runs each architecture on a host of the
matching arch (`macos-14` for arm64, `macos-13` for amd64) instead.

### Codesign + notarize (production releases)

The release workflow honors these repo secrets:

| Secret | Purpose |
|---|---|
| `APPLE_DEVELOPER_ID` | `"Developer ID Application: Your Name (TEAMID)"` — sets the codesign identity |
| `APPLE_DEVELOPER_ID_CERT_P12` | base64-encoded `.p12` of the cert (CI imports into a temp keychain) |
| `APPLE_DEVELOPER_ID_CERT_PASSWORD` | password for the `.p12` |
| `APPLE_NOTARY_KEYCHAIN_PROFILE` | name of a `xcrun notarytool store-credentials` profile |
| or: `APPLE_ID` / `APPLE_TEAM_ID` / `APPLE_APP_SPECIFIC_PASSWORD` | alternate notary auth (no keychain profile needed) |

When secrets are absent the workflow falls back to **ad-hoc signing** and
**skips notarization**. The release still ships, but Gatekeeper warns the
user; opening via right-click → Open works around it. This makes the
workflow useful for early RCs without blocking on Apple credentials.

Local production-grade signing:

```bash
DEVELOPER_ID="Developer ID Application: Your Name (TEAMID)" \
  task darwin:codesign:developer

# notarize (one of these auth modes)
KEYCHAIN_PROFILE="archscope-notary" task darwin:notarize
# or
APPLE_ID="you@example.com" \
APPLE_TEAM_ID="ABCDE12345" \
APPLE_APP_SPECIFIC_PASSWORD="abcd-efgh-ijkl-mnop" \
  task darwin:notarize
```

`task darwin:notarize` zips the .app, submits to Apple, waits for the
result, and staples the ticket to the .app on success.

## Windows — `.exe` (NSIS) and optional MSIX

```bash
task package
# → bin/archscope-profiler.exe + NSIS installer
```

On non-Windows hosts the build is cross-compiled through Docker (~800 MB
image, downloaded once). Run `wails3 task setup:docker` first.

**WebView2 prerequisite.** Windows 11 ships with WebView2 by default. On
Windows 10, end-users must install the WebView2 Evergreen runtime first
(NSIS bootstrapper handles this automatically; standalone `.exe` does not).

## Linux — AppImage / `.deb` / `.rpm`

```bash
task package
# → bin/archscope-profiler-x86_64.AppImage

# per-distro packages via nfpm (config at build/linux/nfpm/nfpm.yaml)
task linux:package:deb
task linux:package:rpm
```

WebKitGTK is required at runtime (`apt install libwebkit2gtk-4.1-0` on
Debian/Ubuntu, `dnf install webkit2gtk4.1` on Fedora). The AppImage bundles
everything else; the .deb and .rpm declare their dependencies (see
`build/linux/nfpm/nfpm.yaml`).

## CI — release pipeline

`.github/workflows/release.yml` fires on `v*` tags (or via
`workflow_dispatch` with a manual tag input). It:

1. Builds the Wails desktop binary on each platform host
   (`macos-14` arm64, `macos-13` amd64, `windows-latest`, `ubuntu-latest`).
2. Codesigns + notarizes (macOS only, gated by secrets — falls back to
   ad-hoc if absent).
3. Builds the platform installer (`.dmg` for macOS, NSIS `.exe` for
   Windows, `.deb` + `.rpm` + raw binary for Linux).
4. Computes SHA256SUMS of all release artifacts.
5. Creates a GitHub Release with auto-generated notes; pre-release flag
   set when the tag matches `-rc` / `-alpha` / `-beta`.

```bash
# cut a release candidate
git tag v0.3.0-rc1
git push origin v0.3.0-rc1
# → workflow runs, GitHub Release created at /releases/tag/v0.3.0-rc1
```

## Build size budget

The Wails desktop binary covers the **full feature surface** (all
analyzers + exporters from `apps/engine-native/`) thanks to T-350' service
bindings. Size measurement on darwin/arm64 (proper `-tags production
-trimpath -ldflags="-w -s"` flags):

| Target | Budget | Measured |
|---|---|---|
| darwin/arm64 raw binary | ≤ 12 MB | **11 MB** ✅ |
| darwin/arm64 .app bundle | ≤ 14 MB | **11 MB** ✅ |
| darwin/arm64 .dmg (UDZO) | ≤ 8 MB | **5.3 MB** ✅ |
| windows/amd64 .exe | ≤ 13 MB | tbd (CI) |
| linux/amd64 AppImage | ≤ 30 MB | tbd (CI) |

Reproducible local build (without `task`):

```bash
# 1. Frontend — go:embed picks up dist/
cd apps/profiler-native/cmd/archscope-profiler-app/frontend
npm install && npm run build

# 2. Go binary
cd ..
GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 \
  CGO_CFLAGS="-mmacosx-version-min=12.0" \
  CGO_LDFLAGS="-mmacosx-version-min=12.0" \
  MACOSX_DEPLOYMENT_TARGET="12.0" \
  go build -tags production -trimpath -buildvcs=false -ldflags="-w -s" \
    -o bin/archscope-profiler .
```
