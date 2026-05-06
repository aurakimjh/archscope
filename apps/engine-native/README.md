# engine-native — Go port of the ArchScope Python engine

This module is the Tier-0 / Tier-9 home for the **Go Engine Full
Conversion** track in `work_status.md` (T-301..T-392). It owns
analyzer / parser / exporter / web / CLI Go code that supersedes the
matching Python module under `engines/python/archscope_engine/`.

The existing Wails v3 native profiler module
(`apps/profiler-native/`) is intentionally **not** merged into this
one — it ships a smaller binary tuned to the profiler-first slice.
Shared models (FlameNode, ParserDiagnostics, DebugLog, RedactText)
are duplicated for now; T-302 will lift them into a workspace `pkg/`
once both apps reference them in lockstep.

## Layout (T-301 foundation)

```
apps/engine-native/
├ internal/
│  ├ models/             AnalysisResult envelope + Metadata
│  ├ diagnostics/        ParserDiagnostics builder (matches Python JSON)
│  ├ statistics/         Average / Percentile / BoundedPercentile
│  ├ textio/             Encoding-safe text iterator
│  │                     (utf-8-sig / utf-8 / cp949 / utf-16-LE/BE / latin-1)
│  └ common/             RedactText / DebugLog (lifted from profiler-native)
└ cmd/
   └ archscope-engine/   (T-360) CLI entry point — empty until subcommands land
```

`internal/` is module-private; cross-app sharing happens through a
future `packages/` workspace (tracked under T-352).

## Build / test

```bash
cd apps/engine-native
go build ./...
go test ./...
```

CI runs `go test ./...` for this module under
`.github/workflows/profiler-native.yml` `go-test` matrix
(`ubuntu-latest`, `macos-14`, `windows-latest`).

## What's next

- T-302 — `ThreadSnapshot` / `ThreadDumpBundle` / `StackFrame` /
  `ThreadState` lifted from profiler-native into `internal/models`.
- T-310 .. T-315 — single-format parsers (access_log, exception,
  gc_log, otel, jfr, runtime stack parsers).
- T-320 .. T-326 — thread-dump registry + 6 language plugins.
- T-330 .. T-339 — analyzers.
- T-340 .. T-344 — exporters (JSON / HTML / PPTX / CSV / report
  diff).
- T-350 .. T-360 — net/http web server + cobra CLI.

See `work_status.md` for the full Tier-0..Tier-9 fan-out.
