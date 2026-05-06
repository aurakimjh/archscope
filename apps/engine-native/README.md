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
│  │                     ThreadState / StackFrame / ThreadSnapshot / ThreadDumpBundle (T-302)
│  ├ diagnostics/        ParserDiagnostics builder (matches Python JSON)
│  ├ statistics/         Average / Percentile / BoundedPercentile
│  ├ textio/             Encoding-safe text iterator
│  │                     (utf-8-sig / utf-8 / cp949 / utf-16-LE/BE / latin-1)
│  ├ timeutil/           ParseNginxTimestamp / MinuteBucket (T-310)
│  ├ parsers/
│  │  ├ accesslog/       Access log parser — 3 regex variants (T-310)
│  │  ├ exception/       Java stack trace + cause chain (T-311)
│  │  ├ gclog/           HotSpot unified / G1 legacy GC log + JVM info card (T-312)
│  │  ├ otel/            OTel JSONL trace/span parser (T-313)
│  │  ├ jfr/             JFR JSON parser + .jfr→JSON via `jfr` JDK CLI (T-314)
│  │  └ runtimestack/    .NET / Go panic / Node.js / Python traceback (T-315)
│  ├ threaddump/         Plugin interface + Registry + 4KB head sniff (T-320)
│  │  └ plugins/
│  │     ├ javajstack/      jstack + AOP cleanup + state inference + SMR + virtual threads + class histogram + monitors (T-321)
│  │     ├ javajcmdjson/    Java jcmd JSON output (T-322)
│  │     ├ gogoroutine/     Go goroutine dump + framework cleanup (T-323)
│  │     ├ pythondump/      py-spy + faulthandler + Python traceback (T-324)
│  │     ├ nodejsreport/    Node.js diagnostic-report + sample-trace (T-325)
│  │     └ dotnetclrstack/  WinDbg clrstack + .NET environment stacktrace (T-326)
│  ├ analyzers/
│  │  ├ accesslog/              22-metric summary + percentile timeline + findings (T-330)
│  │  ├ gclog/                  pause/heap series + JVM Info card + 7 findings (T-331)
│  │  ├ jfr/                    JFR + native memory analyzer + heatmap (T-332)
│  │  ├ exception/              Java exception type/root-cause aggregation (T-333)
│  │  ├ runtime/                Runtime stack trace findings + IIS / .NET (T-333)
│  │  ├ otel/                   Service-path DAG + failure propagation (T-334)
│  │  ├ threaddump/             Single-dump JVM state distribution (T-335)
│  │  ├ multithread/            Multi-dump correlation + 10 findings (T-336)
│  │  ├ lockcontention/         Owner/waiter graph + DFS deadlock detector (T-337)
│  │  ├ threaddumpcollapsed/    Bundle → flamegraph collapsed format (T-338)
│  │  └ profileclassification/  Config-driven runtime classification rules (T-339)
│  └ common/             RedactText / DebugLog (lifted from profiler-native)
└ cmd/
   └ archscope-engine/   CLI entry point — `accesslog` subcommand wired
                         for the parity gate; full Cobra surface lands
                         under T-360
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

- T-340 .. T-344 — exporters (JSON / HTML / PPTX / CSV / report
  diff).
- T-350 .. T-360 — net/http web server + cobra CLI.

See `work_status.md` for the full Tier-0..Tier-9 fan-out.
