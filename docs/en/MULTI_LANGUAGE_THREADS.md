# Multi-language Thread-Dump Framework

Phase 5 of ArchScope ships a language-agnostic thread-dump pipeline. A
single multi-dump run can ingest a mix of dumps from any of the
runtimes below, correlate threads across snapshots, and surface findings
that do not depend on JVM-specific frame names.

## Supported formats

| Format ID                       | Runtime           | Detection signature                                                |
| ------------------------------- | ----------------- | ------------------------------------------------------------------ |
| `java_jstack`                   | JVM               | `Full thread dump …` header **or** quoted-name + `nid=0x…` **or** quoted-name + state line (JDK 21+ no-`nid` variant) |
| `java_jcmd_json`                | JVM               | JSON object containing `"threadDump"`                              |
| `go_goroutine`                  | Go                | `^goroutine \d+ \[\w` (covers `runtime.Stack`, panic, debug.Stack) |
| `python_pyspy`                  | Python (py-spy)   | `Process N:` followed by `Python vX.Y`                             |
| `python_faulthandler`           | Python (stdlib)   | `Thread 0xHEX (most recent call first):`                           |
| `python_traceback`              | Python (custom)   | `Thread ID: <n>` block + `File "...", line N, in func`             |
| `nodejs_diagnostic_report`      | Node.js (12+)     | JSON object with `"header"` + `"javascriptStack"`                  |
| `nodejs_sample_trace`           | Node.js           | `Sample #N` blocks + `Error` + `at fn(file:line:col)`              |
| `dotnet_clrstack`               | .NET              | `OS Thread Id: 0xHEX` blocks with `Child SP / IP / Call Site`      |
| `dotnet_environment_stacktrace` | .NET              | `at Type.Method() in path:line N` (Environment.StackTrace form)    |

The registry probes the **first 4 KB** of every input. When two formats
might match the same head, we register the most specific plugin first;
if none match, the registry raises `UnknownFormatError`. Operators can
override detection with `--format` (CLI) or the `format` field on the
HTTP request — useful when a headerless dump fragment was extracted from
a larger log.

For `java_jstack`, one physical file may contain multiple `Full thread
dump` sections. The registry expands those sections into ordered
`ThreadDumpBundle` entries with sequential `dump_index`, labels such as
`server.log#1`, and bounded source metadata (`start_line`, `end_line`,
best-effort `raw_timestamp`). UTF-16 BOM and null-byte based UTF-16LE/BE
sniffing is supported before format detection.

A multi-dump request fails fast with `MixedFormatError` when its files
resolve to more than one format. Forcing `--format` skips the check
and parses every file with the chosen plugin.

## Normalized data model

Every plugin emits the same three records:

- **`StackFrame`** — `function`, `module`, `file`, `line`, `language`.
  The `language` discriminator lets enrichment plugins target only the
  frames they understand.
- **`ThreadSnapshot`** — `snapshot_id`, `thread_name`, `thread_id`,
  `state`, `category`, `stack_frames`, `lock_info`, `metadata`,
  `language`, `source_format`.
- **`ThreadDumpBundle`** — all snapshots from one logical dump plus
  `dump_index`, `dump_label`, `captured_at`, `metadata`.

The legacy single-dump `ThreadDumpRecord` (in `models/thread_dump.py`)
stays untouched so the original Java single-dump analyzer keeps its
byte-for-byte output.

## ThreadState enum

`models/thread_snapshot.ThreadState` is the union state model:

`RUNNABLE · BLOCKED · WAITING · TIMED_WAITING · NETWORK_WAIT · IO_WAIT
· LOCK_WAIT · CHANNEL_WAIT · DEAD · NEW · UNKNOWN`

The `coerce()` helper maps runtime aliases (`RUNNING`, `parked`,
`sleeping`, `chan receive`, `chan send`, `select`, …) into the canonical
states.

## Per-language enrichment matrix

Each parser plugin runs a language-only post-pass that promotes
generic `RUNNABLE` / `UNKNOWN` states into the more specific wait
categories so the multi-dump correlator can build language-agnostic
findings.

| Language   | Frame normalization                                                                                                                    | State promotion                                                                                                                                                                                                                  |
| ---------- | -------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Java       | `$$EnhancerByCGLIB$$<hex>` strip · `$$FastClassByCGLIB$$<hex>` strip · `$$Proxy<digits>` digits drop · `(GeneratedMethodAccessor)<digits>` · `(Accessor)<digits>` | `EPoll.epollWait` / `EPollSelectorImpl.doSelect` / `socketAccept` / `socketRead0` / `NioSocketImpl.read` → `NETWORK_WAIT`; `FileInputStream.read*` / `FileChannelImpl.read` / `RandomAccessFile.read*` → `IO_WAIT`. `BLOCKED` always wins. |
| Go         | `gin.HandlerFunc.func1` → `gin.HandlerFunc`; trailing `.func1.func2` chains stripped; Echo / Chi / Fiber receivers preserved.          | `gopark` / `runtime.selectgo` / `chanrecv` / `chansend` → `CHANNEL_WAIT`; `runtime.netpoll` / `netpollblock` / `net.(*conn).Read` → `NETWORK_WAIT`; `semacquire` / `sync.(*Mutex).Lock` → `LOCK_WAIT`; file IO → `IO_WAIT`.        |
| Python     | Drop `MiddlewareMixin.__call__` / `solve_dependencies` / `run_endpoint_function` / `view_func` / generic `wrapper` / `dispatch_request` when the file path lives in starlette/fastapi/django/flask/gunicorn/uvicorn/werkzeug. | Socket `recv`/`send`/`accept`/`connect` or urllib3 / requests / httpx → `NETWORK_WAIT`; `threading.{acquire,wait}` / `queue.get` → `LOCK_WAIT`; `select.{select,poll,epoll,kqueue}` / asyncio `sleep`/`run_forever` / gevent → `IO_WAIT`. |
| Node.js    | Strip Express `Layer.handle [as handle_request]` aliases.                                                                              | Looks at `payload["libuv"]`: any active `tcp`/`udp`/`pipe` handle → `NETWORK_WAIT`; any active `timer`/`fs_event`/`fs_poll` → `IO_WAIT`. JS frames only — native frames (uv worker pool) keep their reported state.               |
| .NET       | `<Outer>g__Inner\|3_0` synthetic local function → `Outer.Inner`; `MyApp.<DoWorkAsync>d__0.MoveNext` → `MyApp.DoWorkAsync.MoveNext`.    | `Monitor.Enter` / `SpinLock` / `SemaphoreSlim` → `LOCK_WAIT`; `Socket.Receive`/`Send` / `HttpClient.Send` / `NetworkStream` → `NETWORK_WAIT`; `FileStream.Read` → `IO_WAIT`.                                                       |

## Multi-dump correlation findings

`analyzers/multi_thread_analyzer.analyze_multi_thread_dumps()` consumes
an ordered list of `ThreadDumpBundle` objects and emits an
`AnalysisResult(type="thread_dump_multi")` (`schema_version: "0.2.0"`)
with these core findings:

### Persistence (cross-dump)

- **`LONG_RUNNING_THREAD`** *(warning)* — a thread name keeps the same
  stack signature in `RUNNABLE` for ≥ N consecutive dumps (default
  threshold = 3).
- **`PERSISTENT_BLOCKED_THREAD`** *(critical)* — a thread stays
  `BLOCKED` or `LOCK_WAIT` for ≥ N consecutive dumps.
- **`LATENCY_SECTION_DETECTED`** *(warning)* — a thread stays in
  `NETWORK_WAIT`, `IO_WAIT`, or `CHANNEL_WAIT` for ≥ N consecutive
  dumps. Language-agnostic: relies only on the `ThreadState` populated
  by the per-language enrichment plugins. `LOCK_WAIT` is intentionally
  excluded because `PERSISTENT_BLOCKED_THREAD` already owns that
  signal.
- **`GROWING_LOCK_CONTENTION`** *(warning)* — the same lock address
  attracts strictly more waiters across consecutive dumps; uses the
  per-dump `lock_addr → waiters` graph built by
  `lock_contention_analyzer`.

### Snapshot heuristics (single-dump)

These run on every dump and surface even before the persistence
threshold is met:

- **`THREAD_CONGESTION_DETECTED`** *(warning)* — runnable thread count
  exceeds the available CPU count by an order of magnitude in a single
  dump. Suggests synchronous request fan-in.
- **`EXTERNAL_RESOURCE_WAIT_HIGH`** *(warning)* — > 30 % of threads sit
  in `NETWORK_WAIT` / `IO_WAIT` simultaneously. Suggests an upstream
  service or DB stall.
- **`LIKELY_GC_PAUSE_DETECTED`** *(warning)* — most threads are
  RUNNABLE and a VM-internal thread (`VM Thread` / `GC task thread`)
  carries a GC frame. Heuristic — confirm against the GC log.

### JVM-specific (Java only, optional)

- **`VIRTUAL_THREAD_CARRIER_PINNING`** *(warning)* — a Java dump includes
  virtual-thread carrier or pinning markers; evidence includes the dump
  index, carrier thread, top frame, and first non-JDK candidate frame.
- **`SMR_UNRESOLVED_THREAD`** *(warning)* — `Threads class SMR info`
  contains unresolved or zombie thread evidence. Raw evidence is bounded
  in bundle metadata.

Supporting tables: `native_method_threads` and
`class_histogram_top_classes`. The class histogram parser handles text
`num  #instances  #bytes  class name` blocks only; it does not parse
HPROF heap dumps. Parser metadata keeps up to 500 histogram rows by
default. Increase it with CLI `--class-histogram-limit N`, HTTP
`classHistogramLimit`, or environment variable
`ARCHSCOPE_CLASS_HISTOGRAM_ROW_LIMIT`.

### Lock-contention graph

`lock_contention_analyzer.build_lock_graph()` runs alongside the
correlator. It builds a directed graph from each `lock_addr` to its
waiters, then runs a DFS to detect cycles — exposed in the UI's
**Lock contention** tab as a graph + list of deadlock cycles. Java
jstack monitor semantics are split into `lock_entry_wait`,
`object_wait`, and `parking_condition_wait`. Pure `Object.wait()`
sleep is not reported as a lock-contention hotspot unless there is
true lock-entry wait evidence.

Virtual-thread scale note: parser and multi-dump analysis are designed
as linear passes over thread snapshots. Synthetic validation on this
workstation completed 10,000 virtual-thread snapshots in about 0.7s and
50,000 in about 4.1s for parse+analyze. UI rendering and JSON payload
size should still stay behind `topN`/table limits for very large dumps.

Tunable via `--consecutive-threshold` (CLI) or `consecutiveThreshold`
(HTTP). Findings are also reflected on the `summary` (counts) and
`tables` (per-finding rows) of the result.

## CLI

Single-dump (legacy, Java only):

```bash
archscope-engine thread-dump analyze --file dump.txt --out result.json
```

Multi-dump (any combination of languages):

```bash
archscope-engine thread-dump analyze-multi \
  --input dump-1.txt --input dump-2.txt --input dump-3.txt \
  --out multi-result.json \
  [--format <plugin-id>] \
  [--consecutive-threshold N] \
  [--top-n N]
```

The CLI prints a one-line summary on success
(`<dumps> dumps, <threads> threads, <findings> findings`).

## HTTP / UI

The FastAPI engine accepts the same multi-dump request via
`POST /api/analyzer/execute` with body:

```json
{
  "type": "thread_dump_multi",
  "params": {
    "filePaths": ["/tmp/uploads/d1.txt", "/tmp/uploads/d2.txt"],
    "consecutiveThreshold": 3,
    "format": null,
    "topN": 20
  }
}
```

Errors map to `UNKNOWN_THREAD_DUMP_FORMAT` and
`MIXED_THREAD_DUMP_FORMATS` so the UI can surface a clear message.

The redesigned `Thread Dump` page (Phase 2 shell) accepts cumulative
file uploads, exposes the threshold and format-override inputs, and
renders the three findings as severity-colored cards plus dedicated
tables.

## Profiler SVG / HTML inputs (Phase 4 cross-reference)

ArchScope also accepts FlameGraph.pl / async-profiler SVG and HTML
inputs (T-184…T-187). Those files plug into the existing collapsed
profile pipeline rather than the thread-dump framework:

- `archscope-engine profiler analyze-flamegraph-svg --file flame.svg --out result.json`
- `archscope-engine profiler analyze-flamegraph-html --file flame.html --out result.json`

In the UI, the `profileFormat` selector exposes
`flamegraph_svg`/`flamegraph_html`; FileDock’s `accept` adapts to
`.svg` / `.html,.htm` automatically.

## Out of scope (deferred)

- **Heap dump analysis.** ArchScope currently does not parse `.hprof`
  files. The thread-dump framework is the right place to look at *why*
  threads are stuck, not at *where allocations live*.
- **Process / system monitoring.** No CPU%, RSS, or syscall counts;
  feed those signals from APM tools as side-by-side context.
- **Async-profiler 3.x packed JSON.** Inline-SVG HTML and the legacy
  embedded-tree HTML are supported; the packed-binary HTML format is
  not — emit `--format svg` from `asprof` instead.
