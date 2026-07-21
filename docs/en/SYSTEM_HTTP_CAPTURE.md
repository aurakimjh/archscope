# System-Wide HTTP Capture and Analysis (Design Reference)

> The Korean note (`docs/ko/SYSTEM_HTTP_CAPTURE.md`) is the normative, fully
> detailed design record; this English pair is the condensed reference.

## Status

- **Phase 0 design gates 1-5 (T-566~T-570) are fixed as of 2026-07-20.** These
  five are the design artifacts: the source ledger, the Windows
  capability/fidelity matrix, the transaction/time contract, the bounded session
  store, and the security model. With them closed, no design gate blocks Phase 1
  HAR import. Gates 6-11 are measurement/implementation artifacts.
- **Runtime premise is Windows-first.** The first-class support and verification
  platform for the desktop UI and live capture is Windows. Cross-OS offline
  evidence remains compatible: HAR, profiles, and logs generated on Linux/macOS
  are valid inputs to the Windows UI.
- The Korean note is the detailed normative source. Its **Appendix A source
  ledger** is where the provenance of every `[V]` (verified) claim lives — claim
  ID, source URL or commit, retrieval date, cited fact, and design impact. This
  English reference does not restate that provenance; it points to it.

This document also serves three references required by task T-576, each given its
own titled section below: the **Data-Model Reference**, the **Importer / Dialect
Support Matrix**, and the **Security Guide**.

## 1. Positioning and Premises

The feature reintroduces the capability that HTTPAnalyzer-class tools once filled:
**system-wide HTTP/HTTPS capture with per-process analysis**. Three conditions
must hold at once — global capture (desktop apps, CLIs, background services, not
just a browser tab), clear attribution of which process issued each request, and
full HTTP semantics (headers, bodies, timings, status) in one view. Existing
tools satisfy pairs of these but rarely all three, and none of the surveyed
proxy/packet tools offers a **whole-capture time-series aggregate** (request
rate, error rate, throughput over the full session) — that gap is ArchScope's
distinguishing position.

This is the first feature that breaks ArchScope's standing architecture premises
(file-only input, one-shot batch analysis, no privilege elevation, no data
generation). The design manages that break with two principles:

- **Principle A — separate capture from analysis.** The capturer's whole job is
  to write a session file (`.har` or the session directory format); the analyzer
  reads files exactly as before. HAR import alone is a shippable product (Phase
  1) and is what keeps live capture optional rather than foundational.
- **Principle B — many capture modes, one data model.** Backends sit behind a
  `Capturer` interface and all emit the same `Transaction`. Every transaction is
  self-describing: it carries its own `captureMode` and `fidelity`.

### 1.1 Runtime Support vs Evidence Compatibility

R8 requires two things that must not be collapsed into one word "cross-platform":

| Axis | Meaning | First-release condition |
|---|---|---|
| Runtime support | Does that OS run live capture in ArchScope? | **Windows only** |
| Evidence compatibility | Can evidence *generated* on that OS be analyzed in the Windows UI? | **Linux/macOS/Windows all** |

HAR dialect differences are a function of the **generating tool**
(`creator.name`/`creator.version`), not the generating OS. Cross-OS fixtures are
needed for path, timezone, and encoding differences — not for dialect.

## 2. Capability Tiers and Fidelity Grades

The requirement is **not** "all processes." R1 is expressed as capability tiers,
each with its own acceptance criterion. A single requirement that is
simultaneously mandatory, partial, and future cannot serve as a release gate.

| Tier | Scope | Acceptance |
|---|---|---|
| A | Import HAR/profiles/logs in the Windows UI, **regardless of the OS that generated the evidence** | The dialect corpus normalizes and diagnoses |
| B | Local apps that honor the Windows explicit/system proxy | Browser, `curl`, JVM, Electron — one each |
| C | Windows ETW/WFP/Npcap **metadata** coverage | The counters are produced |
| D | Windows **decrypted semantic** coverage; pinning/QUIC excluded | `semantic` fidelity on Tier B targets |
| E | Linux/macOS live capture | Follow-on capability. **Not a Windows release gate** |

Each platform/protocol/app combination is marked `supported | partial |
unsupported | not_tested` with a fidelity grade. `not_tested` is never conflated
with `unsupported`.

Fidelity is graded in three levels rather than an absolute "raw header
preservation" claim, because a Go `net/http`-based proxy canonicalizes header
names and cannot reconstruct original ordering/casing after the `map` boundary:

| Grade | Guarantee | How obtained |
|---|---|---|
| `semantic` | Canonical header names, values, duplicate multi-values, decoded body, semantic timings | Achievable with a Go `net/http` proxy |
| `decoded_wire` | The above + observed transfer bytes, compression encoding, chunked/H2 framing byte separation | Disable auto-decompression and instrument wire bytes |
| `raw_wire` | The above + original field order/casing/duplication, original framing | Only with a dedicated raw parser/spool path |

Phase 2's default guarantee is `semantic`. `raw_wire` is **out of scope and is
promised nowhere in docs or UI** until a separate path exists. Byte counts are
stored as three distinct quantities — decoded entity bytes, transferred
(compressed) bytes, framing-inclusive wire bytes — never collapsed into one
`bodySize`.

## 3. Transaction and Time Model

### 3.1 Transaction Boundaries (ko 6.3.1)

**One transaction = one request message + its one final response.**

| Boundary | Definition |
|---|---|
| Start (`StartedAt`) | Time the **first byte of the request line (H1) or HEADERS frame (H2)** is read at the observation point |
| End (`EndedAt`) | Time the last byte of the response body (through trailers if present) is read, or the time a terminal state (`failed`/`aborted`) is decided |

Start is not "connection established" — on a reused connection two transactions
would otherwise share a start instant. `Blocked` (pool wait) is modeled as a
phase *after* start, not before. Ambiguous cases are all pinned: a redirect chain
(301 → 200) is **two** transactions; 1xx interim responses (100/103) merge into
the one final transaction with arrival times recorded separately; a 401→re-request
is two; a `CONNECT` tunnel is one (metadata-only) plus one per decrypted inner
transaction; a WebSocket upgrade handshake is one (message stream is Phase 5); an
H2 server push is one (no request); a connection dropped with no response is one,
`aborted`.

### 3.2 Time Base and Units (ko 6.3.2)

Three uses are kept separate so NTP adjustments or sleep/wake never produce
negative phases:

| Use | Source | Unit |
|---|---|---|
| Phase length | monotonic clock (`time.Since`) | ms, `float64` |
| Absolute instant (display/sort) | wall clock | RFC3339Nano string |
| Bucket index | monotonic elapsed from session epoch | ms, `int64` |

Contract: all phase lengths derive from a monotonic clock (never subtract two
wall-clock instants; monotonic readings are lost on serialization, so phases are
computed *then* stored). **The unit is milliseconds `float64`, locked** — sub-ms
phases are not floored to zero. Negative phases are disallowed; a negative phase
from an imported HAR lowers that field to `state: "unknown"` and emits
`HAR_TIMING_NEGATIVE` rather than clamping to 0. `TotalMs` is an **independent
measurement**, not the phase sum; disagreement is an observable fact (unmeasured
phases exist), not necessarily an error.

Timing values carry an explicit state so a single `float64` cannot conflate three
meanings:

```
TimingState = "known" | "not_applicable" | "unknown"
```

A reused connection's DNS is `not_applicable`, not `0 ms`.

### 3.3 State Machine (ko 6.3.3)

```
request_sent ──▶ receiving ──▶ complete
     │              │
     └──────────────┴──▶ failed
     └──────────────┴──▶ aborted
```

Terminal states are `complete`/`failed`/`aborted` only, and transitions are
one-way; a terminal transaction is never updated. **Aggregation reflects
`complete` and `failed` only.** `aborted` durations are set by user behavior and
would poison percentiles, so they are excluded — but an `aborted` counter is
exposed separately, because the abort rate is itself a diagnostic signal. A
request that reaches `request_sent` and then capture stops is `aborted`, not
`failed` (we stopped observing; the server did not fail to answer).

### 3.4 Perspectives (ko 6.3)

A single `Timings` cannot hold two perspectives. A MITM proxy cannot directly
observe the original client's DNS/TCP/TLS handshakes, so perspectives are typed:

- `ClientProxy` — client ↔ proxy
- `ProxyInternal` — proxy queue/processing
- `ProxyUpstream` — proxy ↔ server
- `ImportedHar` — foreign-tool observation

The observation point (`client_proxy` | `proxy_upstream` | `foreign_tool`) is
recorded in session metadata and not hidden by the UI. `Wait` is TTFB, **not
"server processing time."** Perspectives are never summed together; doing so
double-counts the proxy leg.

### 3.5 Invariants and Golden Sets (ko 6.3.4)

The model is a storage format, so it is versioned (`captureSchemaVersion`). The
version-common invariants — a violation is an implementation bug:

| ID | Invariant |
|---|---|
| INV-1 | `EndedAt ≥ StartedAt`, `TotalMs ≥ 0` |
| INV-2 | A terminal transaction has `known` `EndedAt` and `TotalMs` |
| INV-3 | A phase with `state != known` does not participate in the sum |
| INV-4 | Sum only within one perspective (one `TimingSet` field) |
| INV-5 | `UsedExistingConnection == true` ⇒ DNS/Connect/TLS are `not_applicable` |
| INV-6 | A `Sequence == 0` transaction has `UsedExistingConnection == false` |
| INV-7 | `sum(known phases) ≤ TotalMs + ε` (ε = 1 ms). **Exceeding is double-counting** |

INV-7 is an inequality on purpose: the sum may be *smaller* than `TotalMs` (some
phases were not measured); only *larger* is a bug.

The H1 and H2 golden sets are **deliberately opposed**, which is why they are
separate — a version-agnostic overlap check would false-positive on all of H2:

| ID | HTTP/1.1 golden |
|---|---|
| INV-H1-1 | Transactions on one connection have contiguous `Sequence` from 0 and **do not overlap in time** (no pipelining) |
| INV-H1-2 | `Sequence > 0` ⇒ `UsedExistingConnection == true` |
| INV-H1-3 | `StreamID` is empty |

| ID | HTTP/2 golden |
|---|---|
| INV-H2-1 | Transactions on one connection **may overlap in time**; overlap is not an error |
| INV-H2-2 | `StreamID` is unique within the connection |
| INV-H2-3 | All but the first stream have `UsedExistingConnection == true`; DNS/Connect/TLS `not_applicable` |
| INV-H2-4 | `Blocked` means stream-multiplexing wait, not pool wait; interpretation follows `HTTPVersion` |
| INV-H2-5 | A server-push transaction is valid with empty request headers |

## 4. Data-Model Reference (T-576)

This section is the T-576 data-model reference. It documents the
`http_capture` `AnalysisResult` envelope and the versioned session store.

### 4.1 Layers (ko 6.1)

```
CaptureSession
└─ ProcessGroup        exec-path group        ← tree level 1
   └─ ProcessInstance  ProcessKey(pid, startTime) ← tree level 2
      └─ Connection    TCP/TLS (5-tuple, ALPN, cert)
         └─ Transaction  one request/response  ← minimal unit
```

`ProcessKey = (pid, startTime)` because PIDs are reused. The two-level tree
groups by exec path (level 1) and `ProcessKey` (level 2) so that Chrome does not
scatter into dozens of renderer roots and distinct JVM batches do not merge. An
`(unidentified)` root always exists and is shown even at zero count — a zero is
itself coverage information. Attribution is graded `confirmed | inferred |
unknown`; expired-cache attributions are marked `inferred`, never presented as
confirmed.

### 4.2 The `AnalysisResult` Envelope (ko 6.6)

**The envelope is a size-stable bounded object regardless of session size.** An
earlier design changed the contract at a 5,000-transaction threshold; that was
withdrawn — the same `http_capture` type must not have two different
self-contained contracts on either side of an arbitrary number.

| Envelope field | Content |
|---|---|
| `Type` | `"http_capture"` |
| `SourceFiles` | Session directory or `.har` path |
| `Summary` | Total transactions/processes/hosts, transfer volume, error rate, status distribution, duration percentiles, **coverage metrics** |
| `Tables` | `transactions`, `processes`, `hosts`, `slowest`, `errors` — **top-N only**, N stated in the envelope |
| `Series` | Time-axis request rate/transfer/error rate, per-process stacks (bounded) |
| `Charts` | Status distribution, per-host time contribution, timing-phase breakdown |
| `Metadata.Findings` | Diagnostics |
| `Metadata.Extra` | **Session reference and bounded summary only** — `captureSessionRef`. No raw tree |

```go
type CaptureSessionRef struct {
    SessionID     string `json:"sessionId"`
    SchemaVersion int    `json:"schemaVersion"`
    StorePath     string `json:"storePath"`
    Transactions  int64  `json:"transactions"`
    Truncated     bool   `json:"truncated"` // whether the envelope Tables were cut
}
```

Detail transactions/processes/connections/bodies are read from the versioned
store by **cursor pagination**. The Wails method is
`FetchCaptureTransactions(sessionID, filter, cursor)`; the cursor is an opaque
string carrying `(snapshotVersion, lastKey)` so that pages stay consistent while
a live capture grows. Report/export never inflate the envelope; they run a
dedicated projection that reads the store and produces bounded aggregates.

### 4.3 The `CaptureSessionStore` (ko 7.6)

Principle: **disk is the source of truth, memory is a cache.** No transaction
exists only in memory, so eviction is a cache miss, not data loss.

On-disk layout:

```
capture-YYYYMMDD-HHMMSS/
├─ manifest.json        schema version, redaction policy, state, corruption hashes
├─ transactions.ndjson  append-only. source of truth
├─ connections.ndjson   append-only
├─ processes.ndjson     append-only
├─ index/
│  ├─ offsets.bin        record ordinal → file offset (fixed-width int64)
│  └─ buckets.ndjson     aggregate snapshot (restart recovery)
└─ bodies/              blobs (random opaque IDs)
```

- **NDJSON append-only** because it makes crash recovery trivial: a truncated
  last record leaves everything before it valid. The access pattern is append +
  sequential scan + offset random-read only, so no embedded DB is needed.
- **`index/offsets.bin` makes cursor pagination O(1)** — record `n`'s offset is
  at `8n`. The index is derivable (rebuild by scanning NDJSON), not a source.
- **`buckets.ndjson` is an optimization**, recomputable from NDJSON thanks to the
  deterministic aggregator.

Three independent version fields are kept (`captureSchemaVersion`,
`storeLayoutVersion`, `redactionPolicyVersion`) so partial compatibility is
expressible. An unknown **higher** `captureSchemaVersion` is rejected with a
clear error; a lower version is read without migration (missing fields become
`unknown`) with a `CAPTURE_SCHEMA_OLD` finding; unknown **fields** are ignored.

Per-tier memory ceilings — each layer must be independently bounded for the whole
to be bounded:

| Layer | Ceiling |
|---|---|
| Live window | 5,000 records × truncated metadata (URL first 2 KiB, header count only, no body preview) |
| Aggregate | 2,000 buckets × (fixed fields + t-digest cap + top-K 32) |
| Write queue | high-water 64 MiB / hard limit |
| Blob cache | 32 MiB LRU |
| Offset index | 8 bytes × record count — **not fully resident**; `ReadAt` on demand |

Eviction: the live window is a **FIFO ring buffer** (re-fetch by `Seq` range from
the store); blob cache is LRU; **aggregate buckets are never evicted — they are
2:1 merged to lower resolution**, so the full timeline always covers the entire
session. Scroll lazy-loading reads `Seq`-range chunks (default 500) into a
separate scroll cache (not back into the ring), and filtered scroll is handled
store-side.

Disk spill and limits:

| Setting | Default | On exceed |
|---|---|---|
| Max session size | 2 GiB | Per policy below |
| Disk reserve | 512 MiB | **Stop capture** |
| Body total cap | 70% of session size | New bodies become `omitted` |

Session-size overflow policy is user-chosen and **defaults to `stop`**:

| Policy | Behavior | Default |
|---|---|---|
| `stop` | Stop capture, finalize the session cleanly | **default** |
| `body_only` | Lower bodies to `omitted`, keep recording metadata | opt-in |
| `rotate` | Delete oldest metadata and continue (ring session) | opt-in |

`rotate` is not default because user data is never silently deleted; when chosen,
the deleted count is recorded in the manifest and a finding, and the timeline
shows "detail for this range deleted" while the aggregate remains intact.

Store contract: `Fetch` is safe during capture (the cursor's `snapshotVersion`
prevents shifted/duplicate pages); `limit` is capped (default 500, max 2,000);
`Recover()` is mandatory before reading a non-`finalized` session (drops the
truncated tail, rebuilds the index, promotes a recovery report to a finding);
`Append` assumes a **single writer** — one active session, no file-lock races —
while other processes may read (they discard anything past the last newline).

## 5. Importer / Dialect Support Matrix (T-576)

This section is the T-576 importer/dialect support matrix. HAR is the de-facto
standard for this space and, per Principle A, a first-class I/O format — but it
is **a set of dialects, not one format**, and the spec is unmaintained (the W3C
draft is abandoned and marked DO NOT USE; `har-schema` draft-06 is the only
normative artifact, and even it does not catch Firefox/Safari/Insomnia quirks).

There is **no fixed dialect count.** Dialects are defined by `DialectID` plus a
feature matrix; an unknown `creator` becomes the `generic`/`unknown` dialect with
conservative defaults (no normalization + a warning finding), because silently
guessing a dialect and altering values is the worst outcome.

```go
type DialectFeatures struct {
    ID              DialectID
    CreatorMatch    []string // log.creator.name matching rules
    SSLInConnect    bool     // is ssl included in entry.time
    BodySizeOnH2    bool     // does it give bodySize on HTTP/2 responses
    EncodesBase64   bool
    ProvidesProcess bool     // does it carry client process info
    // add a field when a new difference is found — do not count dialects
}
```

### 5.1 The 7-Step Pipeline (ko 6.5.4)

Normalization is a first-class, isolated stage — not logic scattered across the
parser.

1. **Format detect** — strip BOM, check `log.version`/`log.entries`, reject
   NetLog and other non-HAR. **By content, not extension** (`.json`-saved HAR and
   `.har`-saved NetLog both exist).
2. **Schema validation** — against the `har-schema` `required` array. A violation
   is a **finding, not a rejection**.
3. **Dialect identify** — from `log.creator.name`/`version`; unknown ⇒
   `generic` + conservative rules.
4. **Normalize** — ssl attribution unified; the **three size-field semantics**
   reconciled; timestamps to UTC absolute; **`-1` / absent → Unknown, not 0**
   (the spec distinguishes: absent = "this tool cannot measure this phase," `-1`
   = "measurable but not applicable to this request," e.g. a reused connection's
   connect). Information lost during normalization is returned as `Warning`s and
   reported to the user.
5. **Model map** — to the §4.1 `Transaction`/`Connection` layers.
6. **Redaction** — applied at import time too (see §7).
7. **Analysis** — the same `analyzers/httpcapture` as a live capture session.

### 5.2 Dialect Quirks (ko 6.5.2 / 6.5.3, verified `[V]`)

| | Chrome | Firefox | Safari | Insomnia |
|---|---|---|---|---|
| `startedDateTime` | UTC `Z` | **local offset `±HH:MM`** | UTC `Z` or **empty string** | **every entry identical** |
| `timings` | complete, `ssl` excluded from `entry.time` | **`{}` possible**, `ssl` included (TLS double-counted) | redirects all `-1`/`0` | **fabricated** |
| duplicate headers | preserved | preserved | **merged** | incomplete |
| `bodySize`/`headersSize` | **`-1` on all HTTP/2+ responses** (structural: no raw header text) | transfer size (closer to Chrome `_transferSize`) | `-1` on all redirects | always `-1` |
| `redirectURL` | present | present | present | **always empty** (chain unreconstructable) |

Key verified behaviors driving the matrix: **Firefox** writes local-offset
timestamps and can emit empty `timings` (`{}`), and double-counts TLS in
`entry.time`; **Safari** merges duplicate headers and emits `-1` redirect
timings; **Insomnia** uses one degenerate timestamp for all entries and an empty
`redirectURL`; **Chrome** returns structural `-1` sizes on H2. Size fields differ
in meaning and are not cross-tool comparable: Chrome `content.size` = decoded,
`bodySize` = transferred body, `_transferSize` = total; Firefox `bodySize` is a
transfer figure. Absence of `content.encoding` does not mean plaintext (Firefox
has shipped base64 bodies without it); absence of `content.text` is normal (evicted
from the inspector cache); strip BOM before `JSON.parse`.

### 5.3 HAR Diagnostic Codes (ko 6.5.5)

| Code | Severity | Condition |
|---|---|---|
| `HAR_DIALECT_UNKNOWN` | info | Dialect not identified from `creator` — generic rules applied |
| `HAR_SCHEMA_VIOLATION` | warn | Required field missing; continue, do not reject |
| `HAR_TIMING_DIALECT_UNKNOWN` | warn | `ssl` attribution undecidable |
| `HAR_TIMINGS_MISSING` | warn | `timings` empty or required member absent |
| `HAR_SIZES_UNAVAILABLE` | info | `bodySize`/`headersSize` mostly `-1` |
| `HAR_BODIES_ABSENT` | info | No `content.text` — size-only analysis |
| `HAR_HEADERS_COLLAPSED` | warn | Duplicate headers merged (Safari) — R3 unmet |
| `HAR_TIMESTAMPS_DEGENERATE` | warn | All entries same instant (Insomnia) — no time series |
| `HAR_REDIRECTS_UNLINKABLE` | info | `redirectURL` absent, chain unreconstructable |
| `HAR_PRESANITIZED` | info | Appears already sanitized |

`HAR_TIMESTAMPS_DEGENERATE` matters: the full timeline is meaningless in that
case, so the view is disabled with the reason shown.

### 5.4 Rejections and Export

`chrome://net-export` produces **NetLog, not HAR**; it is detected by the
`log.version`+`log.entries` signature and rejected with a clear message. Export
is standard HAR 1.2 with ArchScope extensions under an `_archscope` prefix
(process info, `fidelity`, `captureMode`, redaction record); `ssl` is exported
Chrome-style (excluded from `entry.time`, placed in `timings.ssl` and summed into
`connect`) with the original split value preserved in `_archscope` for lossless
round-trip.

### 5.5 Fixture Corpus (T-572)

The T-572 corpus at `../projects-assets/test-data/har-fixtures/` is the golden
condition set for normalization and diagnostics. It holds 10 dialect fixtures
(Chrome H1/H2/sensitive, Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia,
generic), 8 malformed (BOM, schema violation, absent bodies, base64-no-encoding,
`bodySize=-1`, degenerate timestamps, truncated, NetLog rejection), and 2
adversarial (secrets-everywhere redaction golden, deep nesting), plus generators
for oversized/gzip/extreme-nesting inputs that are never committed. Fixtures are
synthetic-first with per-directory manifests fixing provenance, basis, and
expected dialect/diagnostic/redaction assertions; `CreatorMatch` strings stay
provisional until real-export collection runs.

## 6. HAR Interoperability Notes

The pipeline treats HAR as `.har` (or content-detected `.json`) input registered
through the ingestion `DetectorFunc` under a new `http_capture` source kind. A
fatal structural error (`log.entries` not an array) aborts; recoverable dialect
warnings (vendor semantics, optional-field absence) normalize and continue. The
Phase 1 parser is file-only, with dialect detection and a **100,000-entry cap**,
producing `http_capture` results. Detail storage is inline for Phase 1.

## 7. Security Guide (T-576)

This section is the T-576 security guide. This is the most dangerous feature in
ArchScope; import-time redaction is mandatory.

### 7.1 Threat Model (ko 11.0)

"Safe by default" means: a session file produced with no settings changed can be
handed to a third party and **contains no reusable credentials.**

| Actor | Capability | Primary defense |
|---|---|---|
| Local user (self) | Full app/session control | Redaction on by default at write time |
| Other user on a shared workstation | Different account, same machine | OS keychain, platform-specific owner-only fallback ACL, app-data path permissions |
| Recipient of an exported file | HAR/report only | Re-confirm on export + export-exclusion policy |
| Malicious HAR provider | Controls the input file | Import resource limits |
| CA private-key thief | Obtains the key file | Machine-unique generation, no export, 1-year expiry |
| Crash-dump / log collector | OS crash artifacts/logs (an active debugger with full process-memory rights is out of scope) | Disable dumps before key load, prefer keystore signing handles, minimize sensitive-buffer size/lifetime |

Live capture is **not exposed through the CLI**. `archscope-engine` remains a
file import/analysis surface; capture can start only from the Wails UI that has a
persistent indicator and stop control. No detached, daemon, or headless capture
start path is registered. Changing this is a separate security decision and must
re-pass `SEC-16`.

Live capture also has a pre-storage `CaptureScope`: host-pattern allowlist,
confirmed process-identity allowlist, or an explicitly acknowledged "all local
processes" selection. Out-of-scope transactions are dropped before headers or
bodies reach the session store. Under process scoping, unknown attribution fails
closed; only an explicit `metadata_only` choice may retain bounded connection
metadata, never headers/bodies. Scope and changes are recorded in the manifest
and remain visible during capture.

### 7.2 Why Import-Time Redaction Is Mandatory (ko 6.5.6)

A "sanitized" Chrome HAR (Chrome 130+, 2024-10) removes only `authorization`,
`cookie`, `set-cookie` headers and the cookie arrays. It **does not touch query-
string tokens, POST bodies, response bodies, or `_webSocketMessages`** (the
WebSocket omission is notable — `_eventSourceMessages` has a sanitize branch,
WebSocket does not). So a "sanitized" Chrome HAR still routinely contains
`access_token=` in URLs and form credentials in bodies. Imported HAR is never
assumed safe.

The rationale is a real incident `[V]`: the 2023 Okta support-system breach used
session tokens from uploaded HARs, and per BeyondTrust the attacker used those
cookies **within ~30 minutes of upload**. That latency is exactly why redaction
must be an import-time default, not an after-the-fact step.

### 7.3 Value-Preserving Techniques (ko 6.5.6 / 11.3)

Redaction preserves diagnostic value while defaulting to the safe side. It runs on
**parsed field paths** (ArchScope already parses the model), not regex over raw
JSON text (the Cloudflare approach), which is structurally superior.

| Technique | Effect |
|---|---|
| Mask header **values**, keep names | Header set preserved, most diagnostic signal survives |
| Delete the **entire JWT payload** | Safe default; only minimal non-identifying metadata such as `alg`/`typ` and expiry may be retained |
| Delete the **entire cookie value** | Safe default; session-keyed HMAC correlation requires explicit opt-in |
| MIME-based body drop (JS/CSS/image/font) | Bulk noise removed |
| **Detect-then-confirm** for `key=` patterns (≤64 chars) | Surfaces candidates beyond a static block list, user confirms |

The older signature-strip and unkeyed cookie hash techniques are superseded by
ko 11.3.1 and are not defaults. Blob identifiers are random opaque IDs, not
content hashes. Base defaults are delete; correlation-preserving forms are opt-in.
Redaction is applied at write time (memory holds the original briefly, then it is
gone), recorded in `manifest.json`, and re-confirmed on export. Process metadata
is also sensitive: `commandLine` values are redacted and excluded from export by
default, `user` is excluded, `execPath` is path-abbreviated. The manifest hash is
**corruption detection, not tamper protection** — it is never called an
"integrity guarantee."

Free-text bodies, URL fragments, and nonstandard header values use the same
sensitive-key coverage as structured JSON/form/query fields (`token`, `*token`,
`session`, `sessionId`, `code`, `auth`, `*secret`, `*credential`, and peers).
There is no narrower channel-specific denylist. Payment-card candidates are
redacted only after a valid Luhn check.

### 7.4 CA Lifecycle (ko 11.2.1)

The MITM CA is the single most dangerous artifact. Its state machine:

```
none ─generate→ generated ─install→ trusted
                    ▲                  │
                    └──── uninstall ───┘
       expired ─regenerate→ generated
```

| State | Capture ability |
|---|---|
| `none` | HTTPS impossible in proxy mode; HTTP only |
| `generated` | Key/cert created, not trusted; client rejects HTTPS |
| `trusted` | Registered in OS/browser trust store; normal |
| `expired` | New leaf issuance stops; regeneration needed |

Transition rules: `install` happens **only on explicit user action** (no auto-
registration) with admin auth; `generate` is automatic on first proxy start.
**Partial failure is the core problem** — Windows and per-browser stores are not
one, so registration/removal spans stores and can partially succeed. Removal
reports per-store results (no single "done" message); a partially-removed state
is **not** shown as `none` (still `trusted` with a warning); a partial install is
**rolled back**. **App removal does not remove the CA** (the uninstaller is not
guaranteed admin rights) — both the settings and first-registration screens state
this, and reinstall removes any prior CA before creating a new one. Expiry
auto-regenerates but does **not** auto-re-register.

Additional CA rules (ko 11.2): stored in OS keychain / DPAPI / Secret Service;
file fallback fails closed unless it can enforce owner-only access (Windows:
inheritance-disabled DACL for the current user and `SYSTEM`; macOS/Linux: file
`0600` plus parent app-data directory `0700`). There is **no export or backup**
(regeneration is the answer); CAs are machine-unique, **never bundled into the
build**, and expire after one year. Upstream TLS verification stays on (the proxy is
not a real MITM path); verification failure is recorded as a failed transaction,
with host-scoped explicit exceptions only, no global disable.

Go GC memory cannot promise reliable physical zeroization. Phase 2 therefore
disables OS crash dumps **before** loading key material (Linux `RLIMIT_CORE=0`
and `PR_SET_DUMPABLE=0` where supported, Windows WER/local-dump exclusion, and
the macOS core-dump policy) and refuses HTTPS plaintext capture if that setup
fails. Non-exportable OS-keystore signing handles are preferred where available;
otherwise plaintext buffers stay bounded, short-lived, and absent from logs.
This does not claim protection from an active same-user debugger. Per-host leaf
private keys are memory-only, never written to disk, and their cache references
are discarded at session end.

"Every trust store" means the union of stores recorded at install time and all
stores discovered again at removal time: Windows/macOS OS trust targets plus all
discovered Firefox NSS profiles, and on Linux system anchors, `~/.pki/nssdb`,
and every discovered Firefox/Chromium user/profile NSS DB. Enumeration or removal
failure is not success: the UI reports the store and manual procedure and keeps
the CA state `trusted`.

### 7.5 Privilege Contract (ko 11.6)

The point is not what requires privilege but **what privileged code may do.**

| Action | Privilege | Persistence | Scope of elevated code |
|---|---|---|---|
| HAR import/analysis | none | — | — |
| Proxy capture | none | — | — |
| CA install/remove | admin auth | **one-shot** | Cert-store manipulation only |
| System proxy setting | admin auth (if OS requires) | one-shot | Proxy-setting key only |
| ETW/WFP/pcap session | admin | **session only** | Helper — capture then file write only |

Three invariants: **no resident elevated process** (the helper lives only for the
session, never a service/daemon); **the helper only captures** (no analysis, no
network send — parsing/redaction/UI/AI all happen in the unprivileged process);
**IPC verifies peer credentials** (an unauthorized process attaching to the local
socket/named pipe would be a privilege-escalation hole). Proxy mode requires no
resident elevation at all — its only risk concentrates in the one-shot CA
registration.

### 7.6 Adversarial Test Matrix (ko 11.7)

Phase 0 gate 3's completion condition is that this matrix passes. **`SEC-4`~`SEC-7`
apply from Phase 1** (HAR import) — they are valid without any capture.

| ID | Actor | Test | Pass |
|---|---|---|---|
| SEC-1 | Local user | grep session file for auth headers/JWT/cookies/card numbers with default settings | **Zero reusable credentials**; no JWT payload |
| SEC-2 | Local user | query-string token, POST-body password | Redacted; not in any blob either |
| SEC-3 | Export recipient | same grep after HAR export | As SEC-1; no `commandLine`/`user` |
| SEC-4 | Malicious HAR | compression bomb (≥1:1000), 100k entries, depth-1000 nesting, 100 MiB single string | Rejected before parsing; terminates within memory ceiling |
| SEC-5 | Malicious HAR | structural corruption (`log.entries` an object) | Fatal abort; no partial result shown as normal |
| SEC-6 | Malicious HAR | secret-bearing HAR import | Redaction applied on the import path |
| SEC-7 | User regex rule | catastrophic-backtracking pattern | Impossible under RE2; rule disabled on time-limit + finding |
| SEC-8 | CA key thief | key file location/permission, export feature | Keychain/DPAPI/Secret Service; owner-only DACL or file 0600 + parent 0700 fallback; **no export path** |
| SEC-9 | CA key thief | compare CAs from two machines | Different; no bundled CA |
| SEC-10 | Crash-dump collector | verify dump policy before key load, force crash, inspect OS artifact/logs | Refuse HTTPS plaintext capture if dump exclusion fails; artifact/log contains no CA key/plaintext body; active-debugger defense is not claimed |
| SEC-11 | Other workstation user | access session dir from another account | Blocked by OS permissions |
| SEC-12 | — | verify install-recorded plus currently discovered OS/NSS stores after removal | Removed from every discovered store; enumeration/removal failure reported and state remains `trusted` |
| SEC-13 | — | capture a pinned app | No auto-bypass; diagnosed reason shown |
| SEC-14 | — | host with failing upstream cert | Recorded as failed transaction; no global-disable path |
| SEC-15 | — | unauthorized process connects to helper IPC | Rejected by peer-credential check |
| SEC-16 | Local user | attempt CLI/noninteractive/detached live-capture start | No capture-start/daemon command exists; capture requires the persistent Wails indicator |
| SEC-17 | Local user | generate traffic outside host/process scope or with unknown process attribution | No out-of-scope headers/bodies in any store; unknown is dropped unless explicit metadata-only retention was selected |

Related "does not do" rules (ko 11.4): no certificate-pinning bypass, no traffic
tampering/injection/replay, no dedicated credential-harvesting feature, no
external transmission of capture data (AI interpretation gets **redacted summary
only** and keeps the localhost enforcement), no hidden/indicatorless operation.
User regex rules run on a backtracking-free engine (Go `regexp`/RE2) with length,
count, per-rule time, and scan-size limits. Imported HAR is bounded before
parsing (file size, entry count, field count, string/body size, nesting depth,
decompression ratio).

### 7.7 Retention and Deletion (ko 11.5)

Session deletion unlinks indexes, manifests, session files, and blobs. It is
logical deletion, **not** a promise of medium-level secure erase on SSDs or
copy-on-write filesystems. If a future release permits redaction-off sessions,
they are excluded from normal default retention and require explicit save, a
short expiry, and a residual-media warning. Phase 1 HAR import has no
redaction-off path.

## 8. Windows Coverage Proof Status (ko 10.4)

Coverage honesty is a product feature: the tool must distinguish "not captured"
from "no traffic." Absolute coverage ratios are only exposed once a Windows scope
is proven; **an unverified denominator would be a new lie, not an honesty device.**

The T-571 proof-of-capability acceptance criteria:

| ID | Criterion | Pass |
|---|---|---|
| CAP-1 | Attribution accuracy | ≥ 95% match vs a known control group |
| CAP-2 | False attribution | **0 cases** — zero tolerance |
| CAP-3 | Loss rate | < 1% at 500 tps, exposable as a counter |
| CAP-4 | Bypass detection | Detects a proxy-ignoring app's connection |
| CAP-5 | Cost | < 10% CPU overhead during capture |
| CAP-6 | Privilege/install | Matches the §7.5 requirements |

CAP-2 is zero-tolerance because a false attribution directly destroys the purpose
of the coverage machinery — wrong is worse than merely inaccurate. If every
candidate fails, absolute ratios are removed and only the five self-observed
counters (`captured`/`passthrough`/`unattributed`/`dropped`/`unsupported`) remain,
computed from the proxy's own observation with no verified denominator.

### 8.1 First Measurement (2026-07-20, loopback smoke)

The first measurement was run via `spikes/t571-windows-coverage` on a single
machine (Windows build 26200.8737), 5 control processes as loopback listeners at
~498 tps. Single-machine loopback is the key premise for reading these results.

| Candidate | Scope | Result |
|---|---|---|
| TCP endpoint ownership | Process attribution | **CAP-1 100% (5/5), CAP-2 0, CAP-4 detected → CAP-1~4 pass** (Confidence: high); reconfirms `V-WIN-TCPTABLE` owner-PID lookup |
| ETW Kernel-Network | Process attribution | Payload carries `Execution` PID + `sport`/`dport`, **0 loss over ~100k events** (100,541 in 30 s), but **does not observe loopback** — so ETW CAP-1/CAP-4 are **N/A in loopback mode** (not a failure) |
| WFP netevents | Process attribution | Default config logged **no ALLOW connections** (only 1 drop event); needs ALE connection audit enabled — undetermined |

Notes: real-NIC traffic (140 flows) was observed with PID/ports normally, so ETW
attribution is measurable only on a real NIC. CAP-5 for the TCP-owner probe sat at
the 9~13%p CPU boundary because the probe polls `Get-NetTCPConnection` via
PowerShell at 1 s intervals; the production path must call `GetExtendedTcpTable`
directly to remove that cost.

**Still open (T-571):** re-run ETW/WFP against a real-NIC target
(`-Target host:port`) to settle ETW CAP-1/CAP-4; a `GetExtendedTcpTable`-direct
rerun for TCP-owner CAP-5; re-evaluate WFP depending on ALE audit. Until that
re-measurement, absolute coverage ratios are not exposed — only the five counters
are kept. The proxy mode's `confirmed` attribution basis (§4.1) is now
measurement-backed, though polling-based TCP ownership misses connections shorter
than the polling interval and does not replace full kernel observation for bypass
detection.

## 9. HTTP Session Diff Contract (ko 12.4.1, T-575)

A session Diff cannot be the generic report diff (which only compares summary
numeric fields and finding counts). The HTTP-specific Diff analyzer is versioned
and deterministic.

- **URL templating** is versioned and deterministic: `{id}`/`{uuid}`/`{hash}`/
  `{token}`/`{email}` templates, **query keys only**, with a top-K template cap and
  an `{other}` bucket. Without normalization `/user/1` and `/user/2` become
  separate rows and the diff is meaningless.
- **Comparison dimensions** are endpoint/host/process, with **process disabled for
  HAR pseudo-process sessions** (they carry no real PID), and cross-dimension sum
  checks.
- **Explicit numerator/denominator** on every rate; per-minute normalization is
  skipped for degenerate-timestamp sessions; percentiles are computed **per
  session and never averaged** across sessions.
- **Time-alignment grades** `aligned`/`duration_only`/`none`, with overlay views
  suppressed at `none`.
- Output is a **bounded `http_capture_diff` `AnalysisResult`** (top-K 50 tables)
  with `HTTP_DIFF_*` findings. Drilldown uses per-session store cursors, and
  export never rescans the stores (the T-460 principle).
- Wails entry is via the `HttpCapturePage` compare action and Workspace comparison
  routing — **no new `NavKey`**, no browser-menu assumption, legacy `DiffPage`
  untouched. Implementation is Phase 4 item 31.

## 10. Implementation Phases and Current Status

Task ownership and the sequential review gates are tracked in
[Browser Profile and HTTP Capture Implementation and Review-Gate Plan](./BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md).

Phases are cut so each ships independent value.

- **Phase 0 — design fixed.** Gates 1-5 (T-566~T-570) are the design artifacts
  and are closed; gate 8 (proxy build-vs-integrate, T-574) is decided; gate 9
  (Windows coverage, T-571) has its first measurement. No design gate blocks Phase
  1; gates 8-10 must precede Phase 2.
- **Phase 1 — HAR import (MVP exists; design acceptance incomplete).** A
  file-only HAR parser with dialect detection and a 100,000-entry cap produces
  `http_capture` results through a deterministic capture aggregation protocol
  with offline/live parity tests, surfaced by a minimal HAR-import desktop page.
  Detail storage is inline for Phase 1. Manifest-driven dialect/security
  goldens, dedicated redaction/resource guards, and the designed
  timeline/brush/detail/filter UX remain in `H-RG1`. This breaks no architecture
  premise (file input, batch analysis, no privilege).
- **MITM proxy build-vs-integrate spike (T-574) — decided: option 3.** An
  H1-limited MVP with H2 passthrough, behind a `Proxy`/`Interceptor` interface so
  a native-H2 layer (option 4) can be grafted later without a rewrite; the
  mitmproxy subprocess (option 2) is a documented fallback. Chosen for a single
  signed Go binary, direct contract fit (§3, §4.3, §7 implemented on the proxy
  loop rather than reverse-fitted to a library or IPC), reuse of the T-571-verified
  `GetExtendedTcpTable` attribution, and honest H2 degradation (passthrough marked
  `unsupported`, better than broken interception). A pure-stdlib PoC
  (`spikes/t574-mitm-proxy-poc`) passes an offline MITM self-test and
  cross-compiles for Windows, with a measured risk register (R-574-1..7).
- **Phase 2 — live capture — gated.** Remains blocked on coverage-proof completion
  (T-571) and the store/streaming gates. Windows is the release gate; live mode is
  the same UI with data flowing into it (Principle A / R12), so Phase 2 builds only
  the capturer and streaming layers. A required completion condition is the
  live-replay vs file-batch aggregate parity test.
- **Phases 3-5** — coverage honesty (paired with Phase 2), cross-analysis (the
  reason the feature exists), and deep capture backends (first privilege
  elevation, only after Phase 3 data proves the proxy is insufficient).

## 11. Source Ledger Pointer

The **Appendix A source ledger** of the Korean note remains the auditable claim
registry. Every `[V]` claim's provenance — claim ID, source URL or commit,
retrieval date, cited fact, design impact, and `fixed`/`partial`/`open` status —
lives there and is updated as claims are re-verified (for example, the T-571
measurement moved `Q-WIN-ETW-PAYLOAD` and `Q-WIN-WFP-ATTR` from `open` to
`partial`). The gate condition is not "all rows verified" but **"no design
decision rests on an `open` row."** New `[V]` claims without a ledger row are
rejected in review.
