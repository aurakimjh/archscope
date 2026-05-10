# Packaging Plan

ArchScope is packaged as a Go/Wails desktop application. The former
Python wheel and FastAPI browser server have been retired and moved
under `archive/`.

## Current Model

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build

cd ..
GOCACHE=/tmp/aiservice-go-cache task package
```

Artifacts are produced from
`apps/engine-native/cmd/archscope-app/bin/`.

Latest local verification (2026-05-09):

- `task` 3.50.0 and `wails3` v3.0.0-alpha.87 installed under
  `/opt/homebrew/bin`.
- Vite upgraded to 8.0.11 and `@vitejs/plugin-react` to 6.0.1.
- `npm audit` reports 0 vulnerabilities.
- macOS package build succeeds: raw binary 11 MB, `.app` bundle 13 MB.
- The remaining Vite warning is bundle-size related, not a security
  vulnerability.

## CI And Release

- `.github/workflows/ci.yml` runs Go tests and Wails frontend build.
- `.github/workflows/engine-native.yml` runs the cross-platform
  Go/Wails validation matrix.
- `.github/workflows/release.yml` packages the Wails app from
  `apps/engine-native/cmd/archscope-app`.

## Historical Notes

Electron was removed because of distribution size. The Python
wheel/FastAPI web path was later retired after the Go/Wails binary
proved small enough for the target deployment model.
