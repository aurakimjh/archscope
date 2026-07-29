# User Guide

This guide covers the current Go/Wails ArchScope line. The retired
Python/FastAPI browser app is preserved in `archive/` and is no longer
the recommended path.

## Build And Run

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```

For desktop packaging:

```bash
git clone --depth 1 --branch v3.0.0-alpha.87 https://github.com/wailsapp/wails.git /tmp/wails
(cd /tmp/wails/v3 && go install ./cmd/wails3)
cd apps/engine-native/cmd/archscope-app
task package
```

## CLI Examples

```bash
cd apps/engine-native

go run ./cmd/archscope-engine access-log analyze \
  --in ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out access.json

go run ./cmd/archscope-engine profiler analyze-collapsed \
  --in ../../examples/profiler/sample-wall.collapsed \
  --out profiler.json

go run ./cmd/archscope-engine thread-dump analyze \
  --in ../../examples/thread-dumps/sample-java-thread-dump.txt \
  --out thread.json

go run ./cmd/archscope-engine trace import \
  --in ../../examples/traces/sample-otlp-traces.jsonl \
  --format auto \
  --out trace.json

go run ./cmd/archscope-engine database-log analyze \
  --in ../../examples/database/sample-postgres.log \
  --format postgres-text \
  --out database.json

go run ./cmd/archscope-engine broker-log analyze \
  --in ../../examples/broker/sample-broker.log \
  --format auto \
  --out broker.json

# Chrome Performance trace or V8 .cpuprofile (Node --cpu-prof, CDP)
go run ./cmd/archscope-engine profile import \
  --in ./trace.json.gz \
  --format auto \
  --out browser-profile.json

# Local Lighthouse report (scores are preserved, URLs are redacted)
go run ./cmd/archscope-engine browser import \
  --in ./lighthouse-report.json \
  --format lighthouse-json \
  --out browser-audit.json

# Redacted HAR import (dialect auto-detection, bounded entry cap)
go run ./cmd/archscope-engine http-capture analyze \
  --in ./session.har \
  --out http-capture.json

go run ./cmd/archscope-engine api-contract analyze \
  --openapi ../../examples/api-contract/openapi-orders.json \
  --access-result ../../examples/api-contract/access-result.json \
  --asyncapi ../../examples/api-contract/asyncapi-orders.json \
  --broker-result ../../examples/api-contract/broker-result.json \
  --out contract.json

go run ./cmd/archscope-engine stitch analyze \
  --in ../../examples/stitching/access-result.json \
  --in ../../examples/stitching/trace-result.json \
  --in ../../examples/stitching/database-result.json \
  --time-window-seconds 60 \
  --out stitched.json

go run ./cmd/archscope-engine architecture-docs draft \
  --in contract.json --in stitched.json \
  --out architecture-docs.json

go run ./cmd/archscope-engine report html \
  --in architecture-docs.json \
  --out architecture-docs.html
```

Run `go run ./cmd/archscope-engine --help` for the full command list. The
current supported evidence families are summarized in
`docs/en/IMPORTER_SUPPORT_MATRIX.md`.

## Supported Languages And Evidence

ArchScope support is evidence-based. It analyzes runtime artifacts, logs,
profiles, traces, and contracts; it does not perform static source-code review
or modify application source code.

| Area | Current support |
| --- | --- |
| ArchScope implementation | Go engine, Wails desktop app, React/TypeScript frontend |
| JVM / Java evidence | GC logs, JFR JSON, native-memory events, Java thread dumps, jcmd JSON thread dumps, Java exception stacks, async-profiler/Jennifer profile evidence |
| Go evidence | goroutine dumps, panic stacks, pprof-compatible profiles |
| Python evidence | traceback blocks, py-spy/faulthandler-style dumps, py-spy profile evidence |
| Node.js evidence | diagnostic reports, sample traces, JavaScript stack traces |
| .NET evidence | clrstack, Environment.StackTrace, exception/IIS evidence, dotnet-trace speedscope exports |
| Ruby / PHP / Swift / native profile evidence | rbspy, StackProf, PHP Excimer/Tideways/Xdebug, Swift/async stacks, perf collapsed/native stacks when supplied as supported profile artifacts |
| Browser / frontend evidence | Chrome Performance traces (`.json`/`.json.gz`), V8 `.cpuprofile` (browser, Node `--cpu-prof`, CDP `Profiler.stop`) with sampled CPU run analysis; note these are CPU samples only — no network, layout, or paint attribution |
| HTTP evidence | HAR 1.2 imports with dialect detection and import-time redaction (`http_capture`); Windows live HTTP/1.x metadata capture is implemented and awaiting H-RG4 Windows acceptance |
| Language-neutral evidence | access/edge logs, server logs, OpenTelemetry logs/traces, metrics snapshots, database/broker/platform evidence, OpenAPI, AsyncAPI, stitched evidence, architecture-doc drafts |

Unsupported or deferred:

- Static source-code analysis, AST indexing, repository-wide code search, code
  quality scanning, and automatic source modification.
- Heap dump parsing (`.hprof`) and process/system monitoring such as live CPU,
  RSS, or syscall sampling.
- Direct SaaS APM connectors unless promoted from the roadmap into an active
  implementation slice.

## Windows Live HTTP Capture

The HTTP Capture page includes the T-581 review candidate for Windows live
capture. H-RG4 has a third `CONDITIONAL` verdict dated 2026-07-29; use it for
acceptance work, not as a released capture tier:

1. Read and accept the first-use proxy/CA warning.
2. Install the temporary capture CA when HTTPS interception is required.
3. Leave unattributed retention off unless redacted metadata for unknown
   processes is explicitly needed. It is dropped by default.
4. Start capture and point the intended test client at the displayed loopback
   proxy.
5. Stop capture. ArchScope removes the temporary CA and can load the finalized
   session into the normal `http_capture` analysis view on the same page.

The live renderer keeps the newest 500 metadata-only rows and reloads its
authoritative live window if renderer events are skipped. Request/response
bodies are always omitted; body capture remains blocked until the SEC-10
crash-dump-exclusion preflight exists. The backend uses `pending` for an
undecided MITM progress row and `unsupported` for passthrough; it never labels
opaque in-flight traffic as semantic capture. The current-user Windows root
store covers clients that consume the Windows trust store. JVM/JSSE and
NSS-based clients need their own explicit CA import; the acceptance harness
therefore requires a JVM truststore for JVM HTTPS.

After stop, the finalized analysis derives its capture mode and weakest
fidelity from the stored rows. Mixed or unsupported sessions no longer inherit
HAR-import/foreign-tool/semantic metadata. TLS interception failures are
retained as attributed `proxy_not_captured` / `unsupported` rows, while failed
explicit or h2-only tunnels remain `proxy_passthrough` / `unsupported`.
Capture-time redaction counts are persisted in the manifest and carried into
the finalized analysis and acceptance evidence. The same flush checkpoint
persists capture counters for crash recovery. A stopped in-flight row becomes
`aborted` / `unsupported`; the progress-only `pending` grade never survives as
a finalized transaction. For legacy stores without a checkpoint, backend
evidence marks redaction as unknown and derives conservative counters from
persisted rows, and the finalized card reports that redaction summary as
`not recorded` — redaction still ran before every write, so a missing summary
is a missing record and is never presented as "no sensitive fields matched".

The finalized capture-fidelity card explains itself from the provenance the
engine reported: a live session is described as an ArchScope proxy capture with
proxy-measured timings, and only a HAR import is described as imported
foreign-tool evidence. Its fidelity, capture mode, observation point, and detail
storage render as localized labels in both languages, with the raw engine token
available on hover, and the per-transaction distribution behind the aggregate
grade is listed beneath them so a `mixed` or `unsupported` session shows how its
transactions actually divide.

`coverage: confirmed` is deliberately narrow: ArchScope observed the same
client/proxy endpoint tuple and owner PID in two immediate TCP-owner table reads
and obtained the same process start time before and after the second read.
Attribution is resolved once per accepted client connection rather than once
per HTTP request. This does not prove complete traffic coverage or eliminate
every PID-reuse race; missing or unstable rows are
`inferred`/`unknown` and are dropped unless SEC-17 metadata retention was
explicitly enabled.

For Windows acceptance and package-signature evidence:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-windows-live-capture.ps1 `
  -ProxyAddress 127.0.0.1:43123 `
  -HttpTargetUrl http://127.0.0.1:8080/health `
  -HttpsTargetUrl https://127.0.0.1:8443/health `
  -SessionPath "$env:LOCALAPPDATA\ArchScope\captures\cap-..." `
  -RecoverySessionPath "$env:LOCALAPPDATA\ArchScope\captures\cap-recovered-..." `
  -ArchScopeEngineExe .\bin\archscope-engine.exe `
  -WebViewDebugPort 9223 `
  -JavaTrustStore .\tmp\archscope-t581.jks `
  -ArchScopeExe .\bin\archscope.exe
```

Build the acceptance app with the `t581e2e` tag and launch it with
`ARCHSCOPE_E2E_CDP_PORT=9223`; production builds do not expose this debugging
port. Both target URLs must be loopback fixture origins, and the HTTPS fixture
must support h2 for the h2-only probe. The schema-v4 harness requires
browser/curl/JVM/Electron over HTTP and HTTPS, an attributed pinning failure,
h2-only passthrough, explicit QUIC/UDP invisibility, at least 1,000 long-session requests,
WebView page re-entry, and a separate real crash-recovery session. It waits for
the operator to stop the main UI capture, invokes the read-only
`http-capture acceptance-evidence` command for both stores, and fails on absent
or contradictory evidence. The command never starts capture and writes
metadata-only evidence with owner-only permissions. The artifact omits local
session paths, caps rows at 2,000, declares its loopback-fixture privacy scope,
and must be reviewed before public archiving. Archive the generated JSON or
record its repository path and checksum before requesting re-review. T-581
remains in `REVIEW` until that Windows artifact and an independent H-RG4
re-review pass.

## Native App

Use `docs/en/NATIVE_APP.md` for the desktop UI and packaging
workflow. The Wails app exposes profiler analysis plus the broader Go
engine analyzers through Wails services. The active workspace surfaces are
Analysis Workspace, Evidence Board, Incident Timeline, SLO/Golden Signals,
Service Flow, stitched-evidence drilldown state, Export Center, Report Pack,
and Chart Studio.

## AI Interpretation

AI interpretation is optional and local-only. The Go implementation under
`internal/aiinterpretation` builds evidence-bound prompts, redacts
sensitive data, validates model findings against registered evidence
references, and accepts only localhost Ollama URLs.

This feature is not a source-editing coding agent. It is an evidence-bound
interpretation assistant for already-produced `AnalysisResult` data. The
deterministic analyzer output remains the source of truth.

User-facing workflow:

1. Run one or more deterministic analyzers and add the results to Analysis
   Workspace.
2. If an AI interpretation payload is present, Analysis Workspace shows provider,
   model, prompt version, disabled state, finding count, and gate status.
3. AI findings are rendered in a separate AI-assisted panel and can be added to
   Evidence Board or Report Pack only when the evidence gate passes.
4. If Ollama or the configured model is unavailable, deterministic analysis and
   exports still work.

Local runtime requirements:

```bash
ollama serve
ollama pull qwen2.5-coder:7b
```

The initial policy allows only `localhost`, `127.0.0.1`, or `::1` Ollama
endpoints. Models are user-installed and are not bundled with ArchScope. See
`docs/en/AI_INTERPRETATION.md` for the full gate, redaction, prompt-injection,
and reporting policy.
