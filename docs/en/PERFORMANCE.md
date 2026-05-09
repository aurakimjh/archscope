# Performance

Performance measurement now targets the Go engine and Wails desktop
build.

## Baseline Commands

```bash
cd apps/engine-native
go test ./...
go test -bench=. -run=^$ ./internal/profiler
go build -trimpath -ldflags="-s -w" ./cmd/archscope-engine ./cmd/archscope-profiler
```

Frontend build size:

```bash
cd apps/engine-native/cmd/archscope-profiler-app/frontend
npm ci
npm run build
```

## Budget

- Keep the desktop binary small enough for direct field distribution.
- Avoid reintroducing Electron or an HTTP server into the release binary.
- Prefer streaming parsers and bounded diagnostics for large profiler,
  GC, access-log, and thread-dump inputs.
