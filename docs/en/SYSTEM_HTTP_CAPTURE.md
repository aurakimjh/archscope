# System-Wide HTTP Capture and Analysis (Design Reference)

> The Korean note (`docs/ko/SYSTEM_HTTP_CAPTURE.md`) is the normative, fully
> detailed design record; this English pair is the condensed reference.

## Stand-alone desktop target (2026-11-09)

IEInspector HTTP Analyzer V7.6.4 Stand-alone defines the required Windows
desktop layout and observable behavior. The Wails v3 window must show incoming
requests grouped by process, with the selected request and response in a lower
inspector while capture continues. T-599 establishes the visual and command
inventory, T-603 verifies the primary workflow in packaged WebView2, and T-613
checks the complete inventory. The backlog includes Request Builder, legacy file
compatibility and visible desktop automation. IE/Firefox add-on features remain
outside this edition. [Execution backlog](../../work_status.md#open-work-snapshot) · [Vendor feature list](https://www.ieinspector.com/httpanalyzer/features)

### Stand-alone screen and command inventory (T-599)

The official [Stand-alone tour image](https://www.ieinspector.com/httpanalyzer/images/hs_main_l.gif)
shows a process-grouped grid above the inspector, a summary pane beside that
inspector, and a bottom recording status. The image itself is labelled V4.
The [V7 changes](https://www.ieinspector.com/httpanalyzer/newinv7) add comments,
connection ID and MIME icons. The table below uses the maintained V7 manual
to define the command behavior. Native Windows visual results belong to T-603.

| State or surface | Expected behavior | Implementation task |
|---|---|---|
| Idle | Recording commands visible, request grid and lower selection prompt immediately available | T-594, T-600 |
| Recording | New requests appear under the process name and PID. A selected row continues showing its own headers/body as subsequent requests arrive | T-594, T-595 |
| Selected | Header, Request Timing, Response Content, Post Data, Query String, Cookies, Raw Stream, Hints, Status Code Definition and Comment have a request-specific target | T-597, T-598, T-606, T-609 |
| Stopped | Rows and detail remain usable. Save and reload preserve recorded values | T-600, T-607 |
| Toolbar and menu | Start, Stop, Pause/Resume, Load, Save, Auto Scroll, Clear, Find, View, Filter, Tools, Help | T-600, T-604, T-607, T-610 |
| Start submenu | Current Session, All Sessions, New/Reopen Process, Select Running Processes | T-596, T-601 |
| Context commands | Grid/column customization, star/comment, selected-request copy, Request Builder and content viewer commands | T-604, T-609, T-610 |
| Summary | Current Process and All Captured scopes, elapsed time, round trips, bytes sent/received, compression saving, DNS, TCP connects and result distribution | T-605 |

[Toolbar](https://www.ieinspector.com/httpanalyzer/manual/toolbar.htm) · [Selection](https://www.ieinspector.com/httpanalyzer/manual/sessiongrid.htm) · [Shortcuts](https://www.ieinspector.com/httpanalyzer/manual/usingkeyboardshotcuts.htm)

The manual lists the following field inventory. The marketing count is 36;
the current manual includes later fields, so T-604 must reconcile visibility
against the target EXE. Standard fields are No., Offset, Timeline, Hints, Hint
List, Duration, Method, Result, Received, Type, URL, Redirect URL, Process,
Body-Size, Caching, Content-Length, Host, IP Address, Sent, TimeEnd, TimeStart,
Cookie, Set-Cookie, User-Agent, UA-CPU, SOAPAction, Date, Server, P3P,
Content-Type, Content-Encoding, Transfer-Encoding, ETag, Expires, Last-Modified,
X-Powered-By, ConnectId and Comment. Stage and Webpage are explicitly marked
Add-on-only by this manual. Internal progress states remain useful in ArchScope.
[Column definitions](https://www.ieinspector.com/httpanalyzer/manual/sessiongrid.htm)

Recording shortcuts are F9 Start, F2 Stop and Ctrl+L Clear. Ctrl+S Save,
Ctrl+F Find and F1 Help address their corresponding desktop commands.
Ctrl+Shift+Left collapses groups and Ctrl+Shift+Right expands them.
[Keyboard reference](https://www.ieinspector.com/httpanalyzer/manual/usingkeyboardshotcuts.htm)

Selective cache/cookie removal is also documented on the Stand-alone automation
interface. T-611 tracks that command family. Browser Tamper and before/after IE
cache comparison remain Add-on functions.
[Stand-alone interface](https://www.ieinspector.com/httpanalyzer/manual/ihttpanalyzerstandalone.htm)

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

The original R1 release contract uses capability tiers. T-601 tracks the wider
Stand-alone capture requirement and its independent Windows verification. Each tier below has its own acceptance criterion. A single requirement that is
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

This grade is what makes the UI's raw-protocol view a *reconstruction*. The
analyzer renders a request line, header block, and body assembled from the
canonical model, and labels it as such on every render. Presenting it as
captured bytes would claim `raw_wire`.

### 2.1 Body-Capture Tier (SEC-10 gated)

Request and response body capture is a per-session opt-in that a machine
qualifies for. A MITM proxy holding plaintext payloads is the exposure the
**SEC-10 crash-dump exclusion preflight** bounds, and the H-SEC2 C1 condition
requires failure to be closed rather than recorded as accepted risk. The
preflight is implemented (T-588); where it does not verify, the tier stays
unavailable and the renderer says why.

| Aspect | Contract |
|---|---|
| Default | `captureBodies = false`; every message is `omitted` |
| Enabling | `capture.Config.CaptureBodies = true` is rejected by both the session manager and the proxy constructor, before authority/session allocation, unless the preflight verifies. Each runs the check independently rather than trusting the other |
| Gate record | `LiveCaptureContract` publishes `bodyCaptureAvailable`, `bodyCaptureGate` (`sec10_verified` / `sec10_unavailable`), and `bodyCaptureGateReason`; the session and the store manifest record the gate the session actually enforced |
| Size bound | Processing is capped per message (default 256 KiB, ceiling 4 MiB), including decoded expansion; transferred byte counts are recorded even while bodies are omitted |
| Decoding | `identity`, `gzip`, `deflate`. Anything else is `encoded` — bytes were seen, no text is claimed |
| Redaction | Bodies run through the same policy as headers before persistence; a suppressed body becomes `redacted` |

An unavailable gate is a capability blocker, not an operator-risk
acknowledgement: the UI does not offer the checkbox, and no wording lets an
operator accept an exposure the preflight could not bound. Defense-in-depth
rejection at both engine boundaries prevents a stale or custom renderer from
bypassing that contract.

#### 2.1.1 SEC-10 Preflight

Scope is Windows only, because live capture is Windows only (§9.4); every other
platform reports `unsupported_platform` rather than pretending to have checked.
The preflight runs **before authority generation or any plaintext retention** and
returns a bound result rather than a promise.

| Step | Action | Fail-closed when |
|---|---|---|
| 1 | Read the WER `LocalDumps` machine policy and the per-executable subkey; interpret `DumpType` and `CustomDumpFlags` for heap inclusion | The policy cannot be read |
| 2 | Register the process exclusion and disable snapshot/heap reporting | Registration fails, or the API is unavailable on this OS build |
| 3 | Re-read the effective policy and confirm the exclusion took | The re-read does not show the expected state |
| 4 | Return `verified` or `unavailable(reason)`, bound to the running session | Any step errors |

The mechanism is deliberately **process-wide suppression, not per-region
exclusion**. `WerRegisterExcludedMemoryBlock` covers a fixed address range,
while body buffers are ordinary Go heap allocations the collector may move, so
no stable range can be registered — see `RD-135`. The chosen mechanism is
coarser but checkable: step 3 proves the resulting state instead of asserting
it. On a managed machine whose machine-wide policy forces heap-inclusive dumps
and cannot be overridden without administrator rights, body capture therefore
stays **unavailable** rather than proceeding on an unverifiable claim.

`bodyCaptureGate` carries the result into `capture.Session` and the store
manifest, and `bodyCaptureAvailable` is derived from a preflight run instead of
being hardcoded. The renderer additionally resolves a contract closed when
`bodyCaptureAvailable` and `bodyCaptureGate` disagree, and rejects any gate or
reason token outside the closed sets, so a version skew never renders as raw
machine text.

**Verification.** A policy read that returns the expected value is not
evidence. `scripts/verify-sec10-dump-exclusion.ps1` crashes a process that is
genuinely holding a canary in heap memory and then inspects
`%LOCALAPPDATA%\CrashDumps` and the WER `ReportQueue`/`ReportArchive`. The
harness fails if the preflight does not verify, if the probe exits cleanly, if
it crashes before holding the canary, if any new artifact appears, or if the
canary is found. Artifact absence is the pass condition; the canary scan is the
backstop. It runs on the Windows CI job and archives sanitized evidence
containing counts and leaf names only.

**Not defended by this mechanism**, and stated rather than implied: an active
debugger reading process memory (already excluded by the SEC-10 matrix row), an
administrator enabling `LocalDumps` mid-session, kernel or hypervisor memory
acquisition, and plaintext reaching the pagefile or a hibernation image.

`BodyStorage` is the answer to that question and its values are deliberately
distinct — `omitted`, `inline`, `truncated`, `encoded`, `binary`, `redacted`,
`blob`. "Not recorded", "capped", "undecodable", "not text", and "suppressed"
are five different claims and the UI renders five different sentences.

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

**What the live proxy actually fills.** The MITM path instruments its own round
trip with `net/http/httptrace` and populates two perspectives:

| Perspective | Source | Phases |
|---|---|---|
| `ProxyUpstream` | httptrace hooks on the outbound round trip | `blocked` from the connection-pool wait, `dns`/`connect`/`tls` from the dial and handshake, `send` from connection-acquired to request-written, `wait` as this leg's TTFB, `receive` until the last body byte is read |
| `ProxyInternal` | the proxy's own handling window | `blocked` before the transport engaged, `wait`, `receive`; `dns`/`connect`/`tls` are `not_applicable` at this observation point |

`ClientProxy` stays empty: the proxy did not observe the client's own resolution
or handshake and must not report the leg it can see as if it were that one.
`UsedExistingConnection` comes from the transport rather than being assumed, and
a reused connection reports `dns`/`connect`/`tls` as `not_applicable` (INV-5) —
`0 ms` would let a waterfall imply the work happened instantly. A phase whose
hook never fired stays `unknown`, so the known-phase sum the UI prints is
explicitly a lower bound.

The analyzer shows **one perspective at a time and names it**. Selecting rather
than merging is what enforces INV-4 in the presentation layer.

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
| `Tables` | `transactions` — the whole session up to its own row budget; `processes`, `hosts`, `slowest`, `errors` — **top-N only**, N stated in the envelope |
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

**The transaction list has its own budget, separate from top-N.** It used to
share the 50-row `TopN` that bounds the endpoint/host/slowest/error
aggregations, which silently dropped every image, stylesheet, script, and font
of an ordinary page load before the UI could show them. The two are different
kinds of table: the aggregations *are* rankings, and the transaction list is the
packet list, useful only if it is the whole session.

| Table | Budget | Option |
|---|---|---|
| `transactions` | `transaction_row_limit`, default 20,000, ceiling 200,000 | `TransactionRows` / `--transaction-rows` |
| `endpoints`, `hosts`, `slowest`, `errors` | `table_limit`, default 50, ceiling 500 | `TopN` / `--top-n` |

`transaction_rows` (rows actually emitted), `transaction_row_limit`, and
`truncated` all travel in `metadata.http_capture`, so the renderer can tell
"bounded" from "that was everything" rather than inferring it.

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

### 4.4 Analyzer Presentation Contract (ko 8.2-8.5)

The Korean note's §8 specifies an HTTPAnalyzer-style screen grammar: a packet
grid over a detail pane, with the timeline as a first-class brushable view. This
section records what the implementation guarantees, because several of those
guarantees are honesty constraints rather than layout choices.

**Layout.** The page is two layers: a collapsible session layer (capture source,
summary metrics, fidelity and redaction disclosure, timeline brush, process
tree, diagnostics, comparison, correlation) and the analyzer itself — a
virtualized grid over a resizable detail pane. The grid holds a whole session,
so only the visible window is mounted.

**Grid columns.** `#`, start clock, method, result, host, path, resource type,
size, time, a session-relative timeline bar, and process. Columns sort with a
stable total order (ties fall back to capture sequence, then id) so a refresh
never reshuffles equal rows. The default order is capture order.

**Resource type is derived, not reported.** The engine sends a MIME type and a
path; the category comes from the response MIME, then the path extension, then
the request MIME. The extension fallback is load-bearing: a `.js` response with
no `Content-Type` is a script, and a MIME-only classifier files it under
"other". `CONNECT` is its own kind — a passthrough tunnel is a different
observation from an unclassifiable response, not a worse one.

**Detail tabs.** Raw, Headers, Body, Timing, Cookies, Process.

| Tab | Guarantee |
|---|---|
| Raw | Request and response protocol text reconstructed from the canonical model, side by side, **labelled as reconstructed on every render** (§2). Field order, casing, and framing are not preserved; a captured `Host` header is never overwritten by the synthesized one |
| Body | Renders the payload when present; otherwise names the specific reason from `BodyStorage` (§2.1) |
| Timing | One perspective at a time, named (§3.4). `not_applicable` phases occupy no width but stay listed; an `unknown` phase marks the printed total as a floor |

**What the UI must not infer.** `capture.LiveCaptureContract` publishes whether
the body tier is available, which gate it records, and whether upstream timings
are emitted. The renderer reads these rather than deducing them from absent
data: "this session did not record bodies" and "this build cannot record bodies"
are different statements, and only the contract can tell them apart. A contract
whose schema this renderer does not implement is rejected wholesale rather than
half-honoured.

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
| TCP endpoint ownership (`GetExtendedTcpTable`) | expected unprivileged; reverify in H-RG3 production path | none expected | no helper expected |

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
tampering, injection or automatic replay in the capture path, no dedicated credential-harvesting feature, no
external transmission of capture data (AI interpretation gets **redacted summary
only** and keeps the localhost enforcement), no hidden/indicatorless operation.
T-610 covers an explicit user send from a separate Request Builder copy.
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
`CAP-3: N/A` is not a pass: a polling endpoint table can support a successful
individual persistent-endpoint lookup, but cannot create an absolute system or
process coverage ratio without a measured loss rate and denominator.

### 8.1 First Measurement (2026-07-20, loopback smoke)

The first measurement was run via `spikes/t571-windows-coverage` on a single
machine (Windows build 26200.8737), 5 control processes as loopback listeners at
~498 tps. Single-machine loopback is the key premise for reading these results.

| Candidate | Scope | Result |
|---|---|---|
| TCP endpoint ownership | Process attribution | **CAP-1 100% (5/5), CAP-2 0, CAP-4 detected, CAP-3 N/A**; individual persistent-endpoint lookup only, no absolute ratio |
| ETW Kernel-Network | Process attribution | Event header carries `Execution@ProcessID` and payload carries `sport`/`dport`; **0 loss over ~100k events** (100,541 in 30 s), but **does not observe loopback** — so ETW CAP-1/CAP-4 are **N/A in loopback mode** (not a failure) |
| WFP netevents | Process attribution | Default config logged **no ALLOW connections** (only 1 drop event); needs ALE connection audit enabled — undetermined |

Notes: real-NIC traffic (140 flows) was observed with PID/ports normally, so ETW
attribution is measurable only on a real NIC. CAP-5 for the TCP-owner probe sat at
the 9~13%p CPU boundary because the probe polls `Get-NetTCPConnection` via
PowerShell at 1 s intervals; the production path must call `GetExtendedTcpTable`
directly to remove that cost.

**Open after the first measurement (T-571):** run `loadgen.exe -role server` on a separate Windows
receiver and re-run ETW/WFP from the measurement host with
`run-spike.ps1 -Target host:port` to settle ETW CAP-1/CAP-4. Worker PID and local
port ground truth remain on the measurement host and do not depend on the remote
server. A `GetExtendedTcpTable`-direct rerun for TCP-owner CAP-5 and WFP
re-evaluation depending on ALE audit also remain. Until that re-measurement,
absolute coverage ratios are not exposed — only the five counters are kept. The
proxy mode's `confirmed` attribution basis (§4.1) is now measurement-backed,
though polling-based TCP ownership misses connections shorter than the polling
interval and does not replace full kernel observation for bypass detection.

### 8.2 Real-NIC and Direct TCP-Owner Measurement (2026-07-27)

The second measurement used a separate LAN receiver at `192.168.0.3:18080`
and Windows build 26200.8894 on the measurement host. Five control processes
drove approximately 497 tps, with all requests succeeding in each candidate
pass. Normalized observations and generated reports are retained under
`spikes/t571-windows-coverage/results-real-nic-20260727`.
The committed package does not contain the source ETL/tracerpt summary, WFP
XML/audit state, per-poll TCP tables, typeperf samples, command transcript, or
binary hashes, so it is not claimed as a complete source-level raw-evidence
reproduction package.

| Candidate | CAP-1 | CAP-2 | CAP-3 | CAP-4 | CAP-5 | Disposition |
|---|---|---|---|---|---|---|
| ETW Kernel-Network | **100% (5/5)** | **197 distinct false pairs** | **0% loss** (44,458 delivered) | Detected | 4.5%p¹ | **Discarded** under zero tolerance |
| WFP netevents | N/A — no relevant ALLOW event | N/A | N/A | N/A | 4.6%p¹ | Measured configuration unsupported; removed |
| TCP endpoint ownership | **100% (5/5 persistent endpoints)** | **0** | N/A | **Detected** | **4.8%p pass** | Individual endpoint attribution only; absolute ratio forbidden |

¹ ETW/WFP CAP-5 values are non-gating composite references after their
discarded/unsupported dispositions.

The ETW parser uses the `Event/System/Execution@ProcessID` header PID together
with event-payload `sport`/`dport`. It observed every control port but produced
197 distinct flow pairs with a PID other than the ground truth. PID presence is
therefore distinct
from correct process attribution, and ETW process attribution is discarded.
WFP had no relevant ALLOW events in the measured configuration. Because this
pass did not separately enable ALE audit policy, the product removes WFP as a
coverage candidate and makes no audit-enabled WFP capability claim; an
audit-enabled rerun is not a prerequisite for that conservative removal.

The TCP-owner probe now calls IPv4 and IPv6
`iphlpapi!GetExtendedTcpTable` directly, eliminating both the PowerShell table
query and per-PID subprocesses. It completed 30 polls in 30 seconds, retained
5/5 PID accuracy, zero false attribution, and bypass detection, and reduced
the measured CPU delta from 14.4%p to **4.8%p** (8.0% capture minus 3.2%
baseline). Connections shorter than the one-second polling interval remain an
explicit structural limitation.

The first independent `H-COV1` review on 2026-07-27 was `CONDITIONAL`. The judge
now treats `CAP-3: N/A` as non-ratio-bearing and sets `counter_fallback: true`;
only the five proxy self-observed integer counters and successful individual
persistent-endpoint attribution are exposable. CAP-6 is partial, with helper
lifetime/privilege/IPC/install validation deferred to H-SEC2/H-RG3. The
same-day independent re-review verified all COV-1 through COV-6 remediations
and returned `PASS`, closing T-571/H-RG2.

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

### Portable session files (T-607)

The desktop **Open session** command accepts native `.httpsession` JSON, full or
compact ArchScope XML, and HAR. Save/export offers an explicit choice between
all exchanges in the store (including rows outside the live window) and the
currently displayed rows. The latter preserves the grid order; it does not mean
all pages of a stored-search result. Native JSON, XML and ArchScope HAR preserve
exchange IDs, connection IDs, process instances, timing perspectives, retained
payloads, intermediate responses and the selected exchange. HAR 1.2 standard
fields support other readers; `_archscope` preserves additional canonical data.
These formats are unrelated to the legacy `.halog` format tracked by T-608.

URL lists, quoted tab-separated text, escaped HTML and binary BIFF8 `.xls` are
report exports rather than round-trip session formats. XLS writes string cells
(including leading `=`), supports 65,535 exchanges, at most 4,000 UTF-16 units per
cell and a 7 MiB compound container. Exceeding a limit returns an error before
replacing the destination; use HAR/XML or narrow the scope. The XLS regression
was independently read with xlrd 2.0.2, including Korean and a 1,000-row fixture.
Compact XML explicitly marks bodies `export_omitted`. Response-directory export
creates a new child directory containing retained redacted previews plus a
manifest of storage/encoding and omissions. It does not claim original wire
bytes or restore discarded binary data.

Files are re-redacted on import/export, bounded to 256 MiB/100,000 exchanges,
and contain no application blob paths. File writes use a private temporary file
and rename; canceling the native destination dialog creates no output. Local
round-trip, ordering, cancellation, original-session immutability and independent
HAR/XLS reader checks pass. Native Windows dialog acceptance remains part of
T-613, not evidence obtained on this macOS run.

### HTTP Request Builder (T-610)

The desktop provides an expandable Request Builder for new requests and copies
from the selected exchange. Dragging a grid row reveals a fixed drop target so
scrolling does not hide the destination. Live copies fetch the persisted detail,
then enable editing; late asynchronous copies cannot overwrite an edited send.
The original transaction stays in its capture session. Review the copy notice:
credentials are redacted and discarded/truncated payloads cannot be restored.

Edit method, absolute HTTP(S) URL, repeated headers, raw data, ordered/repeated
form fields, or multipart text/files selected through a native dialog. Form and
multipart modes generate their Content-Type; transport framing headers are
managed by Go. Only **Send request** performs network I/O. Redirects are returned
for inspection rather than replayed automatically; there is no shared cookie
jar. Requests use normal TLS validation and the environment proxy. Uploads are
regular files and total request data is bounded to 10 MiB. Cancellation and a
1–120 second total timeout apply to each send. Returned headers and up to 1 MiB
of decoded text are re-redacted; truncated/binary/unsupported-encoding states
remain explicit. Builder results are separate from the capture store.

Local HTTP fixtures verify edited raw requests, duplicate form values, multipart
file names/data, redirect preservation, cancellation, timeout, TLS rejection and
bounded gzip responses. Race detection passes. The browser UI check verifies
copy/edit/explicit send, a real mouse drag into the fixed target, and original
row preservation using a stubbed Wails bridge. It is not native Windows evidence;
packaged Windows acceptance remains T-613.

### macOS native UI preflight (2026-11-10)

The macOS preflight removes nested interactive help from the options button,
uses instance-specific inspector tab IDs when capture and Builder coexist, and
uses Command accelerators for Find/Clear on macOS. Production renderer and Go
builds validate the native Wails v3 path. Desktop automation twice reported a
locked Mac and failed automatic unlock; direct window, splitter, file-dialog and
keyboard acceptance is therefore still unverified. Shell/build access remains
available and is independent of the desktop lock. Do not treat build evidence
as an observed native UI pass. Windows acceptance remains T-613.

### Timing and summaries (T-605)

The shared inspector has a collapsible timing/session summary. Displayed-row
scope follows filters and progress updates; stored-all and selected-process
scopes page the authoritative snapshot, including rows outside the live ring.
Stored refresh runs only while expanded and ignores obsolete responses. Known
body sizes retain an unknown-count suffix. Compression is decoded minus received
bytes over measured pairs only; negative values indicate expansion. TCP IDs and
observed DNS phases are deduplicated, and elapsed time uses observed endpoints.
Phase totals never mix client/proxy/upstream/imported perspectives. Status
buttons narrow the grid. The bounded process waterfall uses PID plus start time
for separate origins and leaves unobserved phase space blank. Local fixtures
verify bytes, reused connections, unknown phases, progress and process reuse;
Windows/native interaction acceptance remains unverified in this Mac lock state.

### Extended content viewers (T-606)

Headers keep duplicates; cookies also carry ordered Path/Domain/Expires/Max-Age,
Secure/HttpOnly/SameSite attributes. Cookie values are masked before persistence.
The Stream tab selects sent/received/combined semantic text, with UTF-8 hex
positions explicitly distinguished from original wire offsets. Highlighting and
XML/SOAP views render escaped text. The formatting-only HTML preview admits a
small list of attribute-free tags in a sandbox with `default-src 'none'`, no
forms or navigation markup; source, scripts and external-resource markup remain
literal text. Text/hex/HTML rendering is capped at 64 KiB.

When body capture is enabled, complete PNG/JPEG/GIF payloads may produce a static
PNG preview. The same path supports HAR base64 raster input and Builder results.
Input/output are bounded to 1 MiB and one megapixel; SVG, malformed, truncated,
overlarge and MIME-mismatched images are rejected. Re-encoding removes metadata
and animation, but **does not redact pixels**. This is disclosed in the viewer
and export toolbar. Original raster bytes are not preserved. Preview data stays
out of the live event ring/search rows/metadata-only summaries and is removed by
body-only overflow/compact export. Native files retain the validated preview;
HAR/response-directory export uses re-encoded PNG. Local fixtures cover the body
opt-in, raster bounds, duplicate cookie attributes, masking, inert markup,
UTF-8 offsets and all existing capture/parser/session regressions. Native visual
acceptance is pending the unlocked Mac/Windows runs.

### Request hints and annotations (T-609)

Hints/Status/Comment detail explains the selected response class, recorded
failure, omitted/truncated/redacted data and uncertain process attribution.
Findings link to requests only when evidence explicitly contains their IDs;
session-wide findings do not acquire invented targets. Hints, hint lists,
comments and stars are available as grid fields, with a starred-only filter.

Live stars/comments are stored atomically in a bounded sidecar keyed by exchange
ID. The service verifies the ID still exists, masks comment text and clears notes
with the capture log. Finalized analysis and native JSON/XML/ArchScope HAR export
merge the annotations. Opened-file edits stay in memory until Save/export and
never overwrite the input file implicitly. Comment saves ignore responses from
obsolete sources. Sorting/filtering changes placement, not annotation identity.
Local service and round-trip fixtures verify ordering, retained stars, redacted
comments, unknown-ID rejection and reopened XML/native sessions; renderer state
regressions verify row hints and explicit finding links.

### AMF and ASP.NET ViewState inspection (T-612)

The Body tab includes AMF/ViewState tree and bounded text inspection. Captured
bodies, HAR payloads and Builder results feed the same inert decoder before
raw non-text suppression; metadata-only/event/search paths and compact export
remove decoded payloads too. Native JSON/XML/ArchScope HAR preserve the redacted
tree. A separate base64 editor allows explicit AMF0, AMF3, AMF packet or ViewState
inspection without sending a request.

AMF support includes scalar values, ordered objects/arrays, AMF0 references and
AMF3 string/object/trait references, vectors and dictionaries, plus bounded
remoting envelopes. References stay symbolic rather than recursively expanding
objects. ViewState reads ObjectStateFormatter v1 primitive values, pairs/triplets,
arrays/dictionaries, indexed strings and type-name tokens; form/quoted HTML
ViewState fields and numbered chunks are recognized. Class externalizers,
BinaryFormatter, TypeConverter execution, encrypted ViewState and unknown tokens
are explicitly unsupported. Type names are strings, never loaded classes.
Unparsed tails are reported without claiming a valid MAC/signature.

Limits are 1 MiB input/output text budget, 4,096 nodes, depth 32 and 64 KiB strings.
Opaque byte arrays are omitted. Secret-named subtrees are masked before storage;
raw ViewState form/HTML values are suppressed, including incomplete hidden-input
values. Known byte fixtures verify trees, references, remoting lengths, ViewState
chunks/tails, text forms, invalid/truncated/unsupported cases, source immutability
and portable round trips. A local 15-second fuzz run completed without a panic.
These are parser checks, not reference-EXE or native UI interoperability evidence.

Format references: [Adobe AMF3 specification](https://veovera.github.io/enhanced-rtmp/docs/legacy/amf3-file-format-spec.pdf)
and [Microsoft ObjectStateFormatter source](https://github.com/microsoft/referencesource/blob/main/System.Web/UI/ObjectStateFormatter.cs).

Expanded summary content overlays the grid rather than consuming its remaining
height. Inspector splitter bounds include every fixed toolbar row, including
wrapped export/filter/note controls; this prevents the new controls from silently
collapsing the grid. The visual result still needs unlocked native inspection.

### Desktop launch and local control (T-611 implementation)

The visible Wails app accepts `--capture`, `--process ABS_EXE_OR_LNK`, repeated `--pid`,
`--scope`, `--duration`, `--destination`, `--format`, `--open`, and
`--automation-descriptor ABS_PATH`. Legacy `/p`, `/d`, `/t` (minutes) and `/f`
spellings are recognized. Windows executable `.lnk` launch is implemented as described below;
`/f NATIVE` and `.halog` output remain unavailable. Format aliases select ArchScope exporters; they do not
claim the reference application's XML/native schema. A timed destination
requires a duration. Recording stops before the timed all-store export; errors
remain visible. Capture is still unavailable on macOS; the common command core
is locally testable without changing that capability.

Automation is opt-in and binds an ephemeral IPv4 loopback port. The private
JSON descriptor contains the URL and a per-instance bearer token: Unix mode
0600 or a protected Windows owner-only DACL is set before writing the token.
Do not publish or copy that descriptor into evidence. Browser origins, remote
hosts and unauthenticated calls are rejected. The control client never uses an
environment proxy or forwards credentials through redirects. Mutation retries
are deduplicated over the latest 128 request IDs; reusing an ID with different
content is an error. Request/results and the event journal are bounded; journal
gaps require resynchronization. Event rows omit payloads, headers and command
lines; selected detail remains an explicit read.

Build the client with `go build -o archscope-control ./cmd/archscope-control`
from `apps/engine-native`. Start the desktop with an absolute descriptor path,
then call `archscope-control --descriptor PATH --id UNIQUE --command status`.
Commands are capabilities, processes, attach/detach, start, pause/resume, clear,
stop, status/stats, events, list/detail/summary, save and open. Start requires an
explicit scope or previously attached PID/start-time instances. `--params` is a
JSON object matching the existing capture/file contracts. No automatic CA
installation, browser cache/cookie deletion or hidden capture mode is exposed.

Local real-proxy tests cover start/pause/resume/stop, stored reads, events,
save/reload; protocol tests cover authentication, origin/descriptor validation,
retry behavior, parsing and journal bounds. Windows x86/x64 compilation passes.
This is ArchScope local control, **not the legacy COM interface**; Legacy COM,
cache/cookie commands and native client/window acceptance keep
T-611 in REVIEW. [Reference switches](https://www.ieinspector.com/httpanalyzer/manual/commandlineinterface.htm).

### Stand-alone parity harness preparation (T-613)

`scripts/http-standalone-parity-contract.json` lists backend automation, process,
exchange, body and file cases separately from visible native UI and reference
interoperability cases. `node scripts/test-http-parity-report.mjs` checks verdict,
provenance, artifact-hash and evidence-type rules on any host. A non-Windows run,
unknown build commit, missing artifact, or control-only evidence cannot produce
an overall native parity PASS. Unobserved cases remain UNVERIFIED.

On Windows build the renderer, then from `apps/engine-native/cmd/archscope-app`
run `node build/windows/build.mjs -tags production -ldflags "-H windowsgui -X main.desktopBuildCommit=COMMIT" -o ABS_EXE .`,
substituting the exact current `git rev-parse HEAD` (set GOARCH=386 or amd64).
Run `scripts/verify-http-standalone.ps1 -ArchScopeExe ABS_EXE -OutputDirectory
ABS_OUTPUT`. Node 22+ is required. The harness starts only its own visible app,
attaches its real Node PID/start time, generates loopback-only 103/302/POST
fixtures, compares persisted attribution/headers and native save/reopen IDs,
then stops the capture and terminates only the spawned app. It makes no dump
policy, global proxy, trust-store or browser-cache changes. `-RequireBodies`
requires the existing verified body gate; it never changes that gate to pass.

Automation instances store sessions under `.archscope-captures` beside their
explicit descriptor, isolated from the normal desktop store/trust manifest.
The descriptor is private and removed after the owned process exits. Share only
`result.json` and the explicitly hashed fixture artifacts, not the private
capture directory or descriptor. Reports omit tokens, local paths, raw command
lines and private exception strings. The binary hash and embedded source commit
are recorded; the harness **does not observe window layout or COM/.halog parity**.
Those contract cases stay UNVERIFIED until independent native observations and
reference artifacts are attached. This commit completes preparation, not the
T-613 Windows acceptance gate.

### Capture backend boundary and launch cleanup (T-601 follow-up)

The production registry now names `explicit_proxy` and the unavailable
`windows_transparent` backend. Capability responses and the desktop source
selector distinguish their reach. Unknown/unavailable backends fail before
process lookup/launch, CA allocation, dump-policy/body activation or session-file
creation; they never fall back to the proxy. The chosen backend is recorded in
both runtime session and store manifest. The registry/factory boundary accepts
the common capture contract while transport-specific setup stays separate.

Partial capturer construction/start failures are stopped, their pipeline/store
is finalized and a newly allocated authority is released. An authority already
prepared by the user is preserved. Suspended child handles are synchronized,
released exactly once, and cannot terminate a resumed child through a reused
numeric handle. Windows CreateProcess receives an explicit application path;
resume verifies the expected suspend count. Common concurrency/failure tests and
Windows x86/x64 launch compilation pass. These changes do **not** implement or
validate a transparent driver, proxy-ignoring capture or service reach. The
reference vendor documents NetFilter technology; no vendor driver/SDK or WFP
capability was silently introduced. T-601 remains REVIEW pending an implemented
backend and native reach evidence.

### Legacy .halog evidence preparation (T-608)

Historical preparation notes below are superseded by the completed native codec
follow-up at the end of this document (2026-11-11).

No genuine sanitized V7.6.4 `.halog` sample is currently available. The official
Stand-alone ZIP was downloaded and fingerprinted; its outer archive contains an
installer and password notice. The installer was not executed and its nested
contents were not inspected, so this does not prove that the installed product
has no samples. Acquisition metadata is in
`docs/review/evidence/2026-11-10-halog-research/source-manifest.json`; vendor
binaries are not committed. The vendor documents Native as a distinct smaller
format, not a published HAR/JSON/XML container schema.
[Official download](https://www.ieinspector.com/httpanalyzer/download) ·
[Format choices](https://www.ieinspector.com/httpanalyzer/manual/commandlineinterface.htm).

`archscope-engine http-capture halog-probe --in FILE.halog --reference-xml
SAME_CAPTURE_FULL.xml --out PRIVATE_METADATA.json` records streaming SHA256,
size and a container signature only. The desktop has the same metadata probe.
Neither tool decompresses unknown data, exports payload snippets, infers fields
or claims interoperability. A pair remains UNVERIFIED until genuine writer
provenance, field mapping and both reference read/write directions are verified.
Probes are limited to regular files within 256 MiB.

`scripts/halog-reference-fixture.mjs --out PRIVATE_EXPECTATIONS.json` starts a
synthetic loopback fixture and prints its PID. In an already installed, licensed
reference V7.6.4, select that PID and start capture, then press Enter in the
fixture terminal. It generates 103/302, repeated query/form fields, duplicate
cookie attributes, gzip and Korean response text. Stop the reference capture
and save **the same rows** as Native `.halog` and Full XML; retain a version
observation and the generated expectations. If the reference does not capture
loopback traffic, record that limitation rather than substituting invented data.
Review/sanitize the resulting files before sharing; keep originals private.
`--run-now` is only a fixture self-test and never produces a genuine native log.

Until that evidence exists, `.halog` input returns `HALOG_FORMAT_UNVERIFIED` and
legacy/native output is rejected before any write. Saving ArchScope JSON/XML
under `.halog` is forbidden, including existing-file overwrite. Native ArchScope
sessions keep `.httpsession`. Local probe, refusal/immutability, CLI and fixture
tests pass; **the proprietary codec is not implemented**. T-608 remains PENDING
on genuine format evidence rather than being closed on synthetic self-roundtrips.

### Windows architecture builds (2026-11-10)

The native Windows Taskfile uses `node build/windows/build.mjs`. From the app
folder, cross-build with `GOARCH=386 node build/windows/build.mjs -tags production
-ldflags "-H windowsgui" -o /private/tmp/archscope-x86.exe .` (PowerShell:
`$env:GOARCH="386"` then the same Node command with a local output path).
Use `GOARCH=amd64` for x64. Build the frontend first with `npm run build` in
`frontend/`. The wrapper sets GOOS=windows and defaults CGO_ENABLED=0.

For 386 only, it creates a private module copy and temporary modfile for Wails alpha2.117,
verified by version and the updater source SHA256. Two fmt arguments cast the
existing 2 GiB constant to int64, preserving the archive limit. Unknown source,
module replacement or external modfile/overlay fails explicitly. The module cache and
SDK version remain untouched; temporary files are removed even on build failure.
Direct `go build` bypasses this fix. Docker CGO builds currently support only
amd64/arm64. Binary cross-build success does not prove native UI, installer,
auto-update execution or capture of 32-bit clients.

### Desktop command routing (2026-11-10)

The active HTTP workspace owns one native-menu/keyboard router for recording,
Save/Export, Find, Help and group collapse/expand. Save uses the displayed format
and all/filtered scope, preserving notes and selection; concurrent save dialogs
are suppressed. F1 opens local workflow/shortcut help. Ctrl/Cmd+Shift+Left/Right
acts on the current grouping and preserves text-editing selection. Builder-only
response grids do not take ownership of the main grid commands. Key repeat,
IME composition, modal background commands and native/DOM accelerator echoes are
filtered. Unmounting the workspace unregisters its handlers. State tests and
frontend/Mac production builds verify wiring; native Windows interaction remains
part of T-613.

### Windows shortcut launch (T-611 follow-up, 2026-11-10)

The launch file picker and `--process` / `/p` accept `.lnk` in addition to `.exe`
and `.com`. Windows loads a bounded regular shortcut through the built-in
[IShellLinkW](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nn-shobjidl_core-ishelllinkw)
and IPersistFile interfaces in an initialized COM apartment. It reads the target,
raw arguments and working directory without invoking ShellExecute, target search,
link updates or resolution dialogs. Target/directory environment variables expand;
shortcut arguments remain verbatim and extra UI arguments are appended with Windows
quoting. This preserves launcher-specific argument syntax rather than splitting it
with shell rules. All three values stay out of capture events.

Only existing absolute executable targets and existing absolute working directories
are accepted. Empty working directory retains normal inherited-directory behavior.
Broken, relative/unresolved, nested, advertised/MSI and elevated links fail before
session allocation; shell activation, elevation and shortcut window-state settings
are outside this direct-executable launcher. The manager creates the target suspended,
records PID/start time and only then resumes it with inherited proxy settings.
Portable resolution/header/refusal tests and Windows x86/x64 test compilation/vet
pass on Mac. The native test creates a Unicode shortcut, verifies its arguments/cwd,
and checks no execution before Resume; **it is prepared, not executed on Mac**.
Windows native execution and capture reach remain T-613 acceptance.

### Local resilience verification (T-613 preparation, 2026-11-10)

Regression tests first reproduced stale-stop CA removal, restart before trust
cleanup, canceled start allocation, cancellation during start and a canceled
running session that never finalized. Start/stop/CA operations now share the
service lifecycle lock; wrong session IDs cannot clean another session's trust.
CA removal requires stopped capture and shutdown removes prepared trust even
without a capture. Manager cancellation rejects before allocation or stops partial
resources; running-context cancellation finalizes the store. Recovery holds the
manager lock throughout its mutable store scan, excluding the active store.

A delayed re-entry snapshot also reproduced resurrecting stopped/cleared rows.
Hydration now ignores obsolete events/unmounts, filters generation and resyncs a
running session after subscription startup. Pause/clear publish current session
updates. Concurrent controller retries execute once, queued canceled commands do
not execute/consume IDs, descriptors are read with a 4096-byte bound, and cleanup
preserves replacement descriptors. Concurrent journal replay stays ordered/bounded.

`ARCHSCOPE_CAPTURE_SOAK=60s go test -race ./cmd/archscope-app -run
'^TestCaptureProxySoakAcrossClearPauseAndReentry$' -count=1 -timeout=120s -v`
(from `apps/engine-native`) passed in 60.251 seconds of fixture execution:
72 cycles, 18,720 synthetic HTTP requests. Each cycle checks pause/clear generation,
concurrent recovery/re-entry reads, a 64-row live bound, 128 captured/persisted
post-clear rows, duplicate stop and exact listener-port reuse. Interrupted-stop
fault injection preserves a recoverable record and permits a new session; repeated
re-entry and shutdown checks pass under race detection. This is a **60-second local
backend soak**, not a multi-hour test or native Windows/UI acceptance. Full Go
suite/vet, focused race checks, frontend state/production build and Mac/Windows
x86/x64 production cross-builds pass. T-613 remains PENDING on the native and
reference evidence in its existing acceptance contract.

### Wails 3.1 upgrade (2026-11-10)

The active app now pins Go Wails/CLI `v3.1.0` and npm `@wailsio/runtime` `3.1.0`.
CI installs the same CLI and bindings are regenerated with it. Wails 3.1 types
the updater's 2 GiB constant as int64, so the former guarded private-module patch
is removed; the Windows wrapper now invokes ordinary `go build`. Mac production
and Windows 386/amd64 GUI builds, frontend state/build and full Go tests pass.
Earlier alpha2.117 build sizes and workaround evidence above are historical.
Native window/runtime acceptance remains separate.
[Official release](https://github.com/wailsapp/wails/releases/tag/v3.1.0).

### Selective WinINet cleanup (T-611, 2026-11-10)

The inspector's cache tools and local `cache-capability`, `cache-plan`,
`cache-delete` commands now implement Windows WinINet/IE cache and persistent
cookie deletion. Preview selects recorded URLs or all WinINet entries and expires
in two minutes. Execution re-enumerates, skips changed/absent entries and caches
its result so retries cannot delete twice. Plans are bounded (32 plans, 10,000
entries each); preview lists cap at 256 and omit query strings/cookie usernames.
Recorded cache matches use exact URLs; cookie matches respect domain and path
boundaries. No profile files, Chromium/Firefox stores, cookie values or other
processes' in-memory session cookies are touched. The UI asks for confirmation
before execution. Local selection/refusal/expiry/idempotence tests and Windows
compile/vet pass; actual Windows WinINet mutation remains native acceptance.

### Opt-in transparent Windows adapter (T-601, 2026-11-10)

An experimental WinDivert 2.2.2-A adapter now redirects **new IPv4 TCP flows on
ports 80/443**, without changing system/browser proxy settings. It verifies the
DLL and matching driver hashes against the official archive, dynamically loads
only that absolute DLL, opens the driver and reflects admitted flows to a private
stream listener. The client/original destination 5-tuple and PID/start-time remain
attached. Port collisions receive distinct reflected tuples. Only confirmed
processes matching the selected/current-session/launch scope are admitted; the
capture process itself is excluded to avoid upstream loops. Existing/unattributed,
loopback, IPv6 and QUIC flows pass unchanged and are outside this adapter's reach.

Set `ARCHSCOPE_WINDIVERT_DLL` to the official package's `x64/WinDivert.dll` (or x86
for a 32-bit desktop), run the app with administrator rights and explicitly select
`windows_transparent`. DLL/driver hashes must match the 2.2.2-A archive SHA256
`63cb41763bb4b20f600b6de04e991a9c2be73279e317d4d82f237b150c5f3f15`.
No binaries are bundled, downloaded or installed by build scripts. At runtime,
WinDivertOpen may load its signed driver when the backend is explicitly started.
Driver/security-policy failures return errors and cleanup; there is no proxy fallback.
Use the official package with its LICENSE and distribution notices.

Redirected HTTP/TLS streams reuse the common proxy parser/body policy, with upstream
dialing fixed to the original IP/port while SNI/Host is preserved for verification.
TLS requires client trust in the capture CA; pinning/H2-only traffic retains explicit
unsupported/passthrough evidence. Stop closes admission/listeners/streams, then the
driver handles and DLL; background driver errors terminalize the session. Packet
mapping caps at 4096 flows and retains recently closed tuples for two minutes.

Local packet/reflection/collision/scope tests, official binary fingerprints, direct
HTTP/TLS stream integration, race checks and Windows x86 compilation/vet pass.
**Driver packet injection, HVCI/antivirus compatibility and native client reach are
unverified.** The opt-in `TestWindowsTransparentDirectClientE2E` launches its own
proxy-ignoring child against an unauthenticated peer HTTP fixture on port 80 and
asserts persisted PID/start-time attribution. Configure the DLL, peer fixture URL,
and `ARCHSCOPE_RUN_TRANSPARENT_E2E=1` on elevated Windows before running it.
[WinDivert API](https://reqrypt.org/windivert-doc.html) ·
[Official distribution](https://reqrypt.org/windivert.html).

### Native COM adapter and reference-contract inventory (T-611, 2026-11-10)

The desktop opt-in `--com-server` now hosts a real Windows IClassFactory/IDispatch
server on its own STA/message loop, with reference-counted dispatch/collection
objects and IEnumVARIANT. `archscope-control --register-com ABS_DESKTOP_EXE` creates
only ArchScope-owned per-user x86/x64 registry entries; `--unregister-com` removes
only entries pointing to that executable. The ProgID is
`ArchScope.HTTPAnalyzerStandAlone`; vendor ProgIDs/CLSID registrations are untouched.
Calls require the same user SID and at least the server's integrity/elevation level.
For transparent capture, prelaunch the configured desktop with `--com-server` and
run the COM client at the same administrator level.

Implemented members cover attach by exact PID/start time or name, session scopes,
new executable/shortcut launch, Start/Stop/Pause/Resume/Clear, requested Visible,
IsLogging/IsPaused, read-only log snapshots/Count/Item/ForEach and metadata/header
objects, LoadLog/SaveLog and WinINet cleanup. A controller targets the session it
started so stale commands cannot stop a replacement session. Attachment changes
require stopped recording. `Backend`, `SessionID`, `ProxyAddress`, `BuildCommit` and `CaptureBodies`
are ArchScope extensions. CaptureBodies defaults off and goes through the existing
independent body-policy preflight; it never grants a bypass. The default backend is `windows_transparent`; choose
`explicit_proxy` explicitly when that is the desired transport. Stable snapshots
are cached, bounded at 100,000 entries and fail explicitly above that limit.

Static extraction found the vendor automation TLBs and bundled Delphi declarations.
The generated declaration is dated 2009/library 5.0 and differs in hash from the
current shared TLB, so it is evidence for an API subset, not current binary parity.
`reference_contract.json` records its factual GUIDs/DISPIDs/export enums without
redistributing vendor source/binaries. Supported dispatch IDs and SaveLog enums
match that declaration; NativeLog=6 remains refused without a verified codec.
TimeDuration stays integer milliseconds and date getters use OLE DATE. Unknown
metadata fails explicitly rather than inventing timestamps. Log snapshots expose
canonical metadata/headers, not original raw binary streams.

**Legacy custom-IID dual vtables, ITypeInfo, connection-point events/ref-cancel,
filter-file commands, MaxLogSize retention and cross-process session-cookie cleanup
are not implemented.** The server exposes IUnknown/IDispatch and IEnumVARIANT,
not those unimplemented vendor interfaces. This is a functional late-bound adapter,
not a drop-in binary replacement. Windows activation/marshalling/registry/GUI checks
remain unverified. `scripts/verify-http-com.ps1` checks activation, lifecycle, native
enumeration, PID attribution and a saved session against a supplied synthetic HTTP
fixture. Its result explicitly excludes vendor interoperability and native UI parity.

### Native Windows follow-up (2026-11-11)

The [Windows evidence](../review/evidence/2026-11-11-windows/README.md) supersedes
the earlier unexecuted-native statements for the tested subset. x86/x64 native
COM activation, lifecycle, collection enumeration/indexing, caller attribution
and session save pass. Explicit IDispatch property reads are required by the
PowerShell smoke when ITypeInfo is absent. Unicode `.lnk` arguments/cwd and
suspended launch pass. Vendor dual interfaces/events/filter/retention and
cross-process session-cookie behavior remain outside the implemented subset.

Native x86 startup required new guarded Wails 3.1.0 fixes for POINT callbacks
and Win32 Ptr imports; use the checked-in Windows build wrapper. Both architecture
startup/control intervals pass. The 16-case parity report remains UNVERIFIED,
with four automation cases passing. Production live/process/reentry is complete
under T-623; the full menu/OS DPI matrix and multi-hour desktop soak remain open.

A genuine private legacy sample now loads 675 entries in reference 7.6.4.508.
Only metadata/hash evidence is archived. The private reference Full XML export
fails XML parsing, and native codec/mapping plus both `.halog` interoperability
directions remain unverified. T-608 is REVIEW; sample absence is resolved.
Portable core/contract/ownership tests and Windows ABI/test compilation/vet pass.

The Wails 3.1 build also pins Go toolchain 1.26.10. Frontend audit fixes stay within
the existing version ranges; the resulting npm audit reports zero vulnerabilities.

### Genuine reference-pair collection after static research (T-608)

The installer directory and its automation ZIP were inspected with a temporary
innoextract tool without running the installer/vendor executables. The directory
contains TLBs/examples but no `.halog` entry; the example ZIP likewise has none.
Only metadata/sample archive files were extracted; vendor drivers/keys were not
extracted or installed. Fingerprints and the older bundled-contract qualification
are recorded in `docs/review/evidence/2026-11-10-halog-research/source-manifest.json`.

On Windows with an already installed reference V7.6.4,
`scripts/collect-halog-reference.ps1 -ReferenceExe ABS_EXE -FixtureURL FIXTURE_URL
-OutputDirectory PRIVATE_OUTSIDE_REPO` validates the registered vendor executable
version/path, refuses an active/nonempty reference log, attaches its own caller PID
and generates only synthetic fixture traffic. It saves the same stopped capture
using verified NativeLog=6 and XMLLogFull=0 enum values and records hashes/version.
It never clears existing reference data or overwrites pair files. The default
fixture can be started with `node scripts/halog-reference-fixture.mjs --out PRIVATE_EXPECTATIONS`;
leave it waiting and use its loopback base URL from the collector. If the reference
cannot capture loopback, record that limitation and use a controlled peer fixture.
Review/sanitize both private files before sharing. The collector is prepared, not
executed here. No proprietary codec or original read/write round trip is claimed;
T-608 remains PENDING until genuine files and reference execution are available.

### Native HALOG codec completed (T-608, 2026-11-11)

T-608 is DONE for the observed V7.6.4.508 format 6/request schema 7. A bounded
data-only Go reader/writer handles the private 675-entry sample; low-level
decode/re-encode is byte-identical. Direct reference COM comparison verifies
methods, URLs, statuses, process labels, timings and available UTF-8 bodies.
Both interoperability directions include reference native re-save and ArchScope
reimport. Synthetic cases cover ordered duplicate headers, Korean text/metadata,
stream-length boundaries, redirects, failure and incomplete responses.

Desktop open/analyze/export, `http-capture analyze --format auto|halog`,
`http-capture convert --in INPUT --out OUTPUT.halog --format native`, and
`/f NATIVE` use the codec. Product imports and exports apply existing redaction.
Exports reconstruct HTTP/1.x framing and retain text previews. Binary/non-retained
data, interim responses, cache paths, original handles and verified OS identity
are not represented; comments/provenance disclose that semantic export contract.
Pending responses retain their available status/body without becoming complete.
Full session/HAR files preserve richer canonical metadata.

Native ANSI metadata defaults to the host Windows codepage; CP949, CP1252 and
UTF-8 are supported. The advisory `ArchScopeCodePage` attribute preserves decoding
on ArchScope reimport; original V7 uses its host ANSI codepage. Unrepresentable
metadata fails explicitly. OLE DATE has no timezone, so the current host timezone
is assumed and disclosed. Process path/PID suffixes become unverified basename
labels with no PID/start-time attribution.

The checked-in fixture contains only synthetic data saved by the original tool.
The private source, raw round trips and reference exports stay outside Git.
`scripts/verify-halog-codec.ps1` reproduces original reader/writer checks without
capturing network traffic and preserves/restores a stopped reference log.
See [codec evidence](../review/evidence/2026-11-11-windows/HALOG_CODEC.md).
T-601/T-611/T-613 and T-627 retain their separate remaining conditions.

### T-611 filter and log-view implementation (2026-11-11)

SetFilter reads bounded Delphi TPF0 TFilterWrapper criteria as data, without
creating vendor objects. The observed v1/v2 flat AND/OR rules support method,
status, response size and content type, exact/not-equal/greater-than/LIKE/not-LIKE
comparisons. Sixteen of seventeen official presets parse; the cache-origin
preset is refused because the common capture contract does not retain that
origin flag. Unknown columns, operators, nesting, value types and trailing data
fail explicitly. FilteredCount/FilteredItem and entry IsFiltered expose the view
without deleting original capture evidence.

MaxLogSize is a latest-row count (reference bundled manual), default zero. The
automation snapshot and SaveLog use that retained view; increasing the limit
does not resurrect evicted IDs. The implementation accepts 0..100000 rows, with
the existing independent durable session byte cap still in force. It does not
claim physical journal compaction. StillCaptureDataWhenHide controls automation
Visible changes: hide pauses only an owned running, previously unpaused session;
show resumes only that automatic pause. Manual pause and replacement-session
ownership are preserved. Mac common/race tests and Windows test cross-builds
cover these rules; native Windows reference behavior remains the T-611 gate.

### T-611 dual ABI and type descriptions (2026-11-11)

The adapter now provides the declared custom interface IIDs and inherited
safecall vtable prefixes, with HRESULT/out-result conversion and the existing
same-user/integrity gate. API-shape facts (28 interfaces, 225 declared methods)
are recorded in reference_contract.json; no vendor implementation or binary is
redistributed. ArchScope extensions follow the unchanged standalone prefix.

The Windows STA builds in-memory dual ITypeInfo through ICreateTypeLib2/
ICreateTypeInfo, using typed arguments and [out, retval] results rather than
CreateDispTypeInfo's incompatible simplified return convention. Type descriptions
are released with the server. This does not register vendor classes, replace
their registrations, or establish custom-IID cross-process marshaling on a
clean Windows host. That and current-reference binary/early-bound client checks
remain native acceptance gates. Getters without captured evidence still fail
explicitly; a declared interface does not invent missing raw/cache data.

Mac declaration/race regressions and Windows amd64/386 test compilation/vet
pass. Added native Windows tests inspect dual flags, inherited slot counts and
the SaveLog HRESULT/BSTR/enum signature. Their execution is not a Mac result.

### T-611 connection points and targeted cleanup (2026-11-11)

ILog now remains live while Count/Item/FilteredCount/FilteredItem and enumerators
read immutable snapshots. It exposes IConnectionPointContainer, IConnectionPoint,
IEnumConnectionPoints and IEnumConnections for the declared ILogEvents IID.
Advise/Unadvise maintain bounded (16) reference-counted sinks; polling and outgoing
OnNewEntry/OnUpdateEntry calls run on the server STA, without holding capture or
controller locks. OnNewEntry carries the reversed entry/by-ref VARIANT_BOOL
arguments and honors DiscardIt in the automation view and its exports. Existing
rows are not replayed at Advise. Session/generation changes reset the cursor and
stale cancellation cannot suppress a replacement session's entry.

These are post-persistence notifications: cancellation does not undo durable
evidence or claim original pre-storage admission parity. Callback timing,
cross-process marshalling, current vendor early-bound clients and native
connection lifetimes still need Windows execution. Added common cursor/discard
regressions and native callback ABI/lifetime tests; Mac common/race and Windows
x86/x64 compilation/vet pass. Native tests are not counted as executed on Mac.

DeleteUrlCache and DeletePersistentCookie use a validated single HTTP(S) URL,
the existing bounded WinINet plan, change recheck and actual deletion counts.
DeleteSessionCookies explicitly returns COM_SESSION_COOKIES_UNAVAILABLE: another
process's in-memory WinINet session requires integration inside that target.
No DLL injection, Chromium profile deletion or persistent-cookie fallback is
performed. Building this adapter on Mac does not complete that requirement.
The Windows COM smoke uses NativeLog enum 6 and a .halog destination, matching
the now-described typed SaveLog signature. Original raw IDispatch named export
formats remain available to callers that bypass type-based argument conversion.

### Native verification handoff

The adapter materializes its generated type library in an owned temporary
directory for the server lifetime, so external ITypeInfo clients can resolve
metadata; it cleans up after releasing its type descriptions. It does not
install a vendor typelib or overwrite registrations. A new isolated Windows
test uses a random test CLSID, a separate fixture process and authenticated
IDispatch/ITypeInfo calls; it never starts capture or modifies browser caches.
The dedicated Windows COM workflow also executes the suite as a real x86 process.
These tests are compiled locally and await hosted Windows execution after push.

[Mac verification](../review/evidence/2026-11-11-mac-automation/validation.json)
records full Go race/vet, frontend state/build, macOS package/signature, desktop-
only checks and both Windows GUI cross-builds. T-611 remains REVIEW for the
explicit remaining contracts above. macOS CI and Release now use macos-15;
GitHub retired macos-14 on 2026-11-02.

### Hosted native results (final product commit 7709787)

Engine Native run 42909617678, Windows COM contract 42909617794 and general CI
42909617793 all passed. Both macos-15 jobs now start and pass full Go checks,
offline imports, app packaging and strict signature validation. Linux/Windows
checks, frontend, both GUI cross-builds and existing Windows live/body/policy
checks pass.

The dedicated COM suite executes on native amd64 with race/checkptr and on 386.
It verifies dual metadata/HRESULT signatures, connection-point identity/lifetime,
event argument order/by-ref cancellation and real separate-process IDispatch/
ITypeInfo. Earlier failures exposed token-buffer pointer/alignment and metadata
parameter naming defects; both were corrected and the replacement runs passed.
The original presets remain private/optional in public CI; the read-only Mac
16-supported/1-explicit-refusal check is separate.

[Scoped CI results](../review/evidence/2026-11-11-mac-automation/ci-validation.json)
identify source, jobs and limits. T-611 remains REVIEW for clean-host custom-IID
marshaling/current vendor early-bound clients, exact pre-storage cancel/physical
retention, missing cache-origin filter evidence and target-process in-memory
session-cookie integration. The new native tests neither capture real traffic
nor delete browser data and are not a full GUI/reference parity verdict.

### Physical row retention (Mac follow-up 1, 2026-11-11)

A positive MaxLogSize now fixes a capture session's physical row limit at Start.
The store commits immutable sequence files and an atomic manifest selection,
then removes evicted files and unshared body blobs. Zero retains the original
NDJSON store. The append path is independent of the retained row count; recovery
validates committed counts, discards interrupted/unselected files and rebuilds
references. Cleanup failure is disclosed and prevents unchecked growth. Clear
commits its empty selection before unlinking data. Old paged cursors become
stale at eviction, so callers must refresh rather than mix snapshots.

Persisted counts remain cumulative, while retained/evicted counts are explicit
in status and analysis metadata. Portable session/XML files preserve source
retention counters; other export formats contain the retained rows and do not
provide the portable session's full metadata contract. Shared blobs survive
until their last retained owner is removed; blob names are never reused after
reclamation. Change MaxLogSize while stopped before starting the next capture.
Mac race tests exercise eviction, shared bodies, Clear, interrupted commits,
missing committed data and reopen counters; a real local-proxy test verifies
analysis and portable export. This is not Windows driver/UI acceptance.

### Pre-storage cancellation (Mac follow-up 2, 2026-11-11)

COM-owned captures now submit redacted, payload-free completion metadata to
OnNewEntry on the server STA before durable Append. Any DiscardIt, RPC error,
queue overflow or 500 ms deadline rejects that row; it never enters the durable
log, aggregate or export. Already-visible in-flight metadata is removed on
rejection. No subscriber preserves ordinary capture behavior. Pause, Clear,
Stop, last Unadvise and owner release cancel pending decisions; generation
checks prevent a pre-Clear completion from entering the new log. External calls
run without controller, manager or store locks. An uncooperative external RPC
can still occupy its STA, but capture waits and queue growth remain bounded.
Post-persistence notifications for admitted rows are updates, avoiding a second
new-entry cancellation. Public scripts do not receive the internal owner token.

Common race tests and real synthetic local-proxy Stop/Clear/Pause tests pass on
Mac. Native STA discard/reentrant-stop tests compile for amd64/386 and will run
in Windows CI after the final push; this is not original-client acceptance.

### Owned custom-IID registration (Mac follow-up 3, 2026-11-11)

Explicit COM registration now creates owned per-user interface mappings for the
universal Automation proxy and an ArchScope 1.0 type library. Separate win32 and
win64 files describe target vtable offsets, independent of the registering
process architecture. Files persist in the user's ArchScope Automation cache
until owned unregister/replacement; runtime ITypeInfo still has its own backing
file. The outgoing dispinterface uses the standard dispatch proxy.

Both registry views are preflighted before mutation. Existing user/machine
vendor IID mappings or another executable's owned mapping cause an explicit
refusal; neither vendor classes nor interface registrations are replaced.
A failed registry write rolls its individual view back; after a later-view
failure an already-published earlier view may remain and can be unregistered.
Unregister removes only known owned keys/files and never recursively deletes
unknown registry subtrees. No administrator-wide registration is installed.
This selective registration follows Microsoft's [proxy mapping contract](https://learn.microsoft.com/en-us/windows/win32/com/proxystubclsid32)
and [type library registration layout](https://learn.microsoft.com/en-us/previous-versions/windows/desktop/automat/registering-a-type-library);
a broad RegisterTypeLibForUser would conflict with the collision-preservation rule.

Native fixtures cover owned/foreign keys, both target offsets and real separate-
process custom-IID QI plus an inherited typed integer getter. Mac amd64/386
cross-build/vet pass; hosted execution is recorded after final push. These
fixtures do not establish compatibility with the current vendor binary/client.

### Normalized nested filters (Mac follow-up 4, 2026-11-11)

SetFilter accepts an explicit ArchScope JSON file in addition to the observed
native TPF0 presets. This JSON format does not claim vendor .flt compatibility.
For example, show failed POST requests from a known process:

```json
{
  "format": "archscope.http-filter",
  "version": 1,
  "active": true,
  "where": {
    "all": [
      {"field": "method", "op": "eq", "value": "POST"},
      {"field": "status", "op": "gte", "value": 400},
      {"not": {"field": "processName", "op": "contains", "value": "test", "ignoreCase": true}}
    ]
  }
}
```

Each node contains exactly one `all`, `any`, `not`, or field/operator/value
predicate. Empty groups, duplicate/unknown keys, unsupported types/operators and
trailing data are errors. Limits: 256 KiB file, 16 node levels, 256 total nodes,
4096-byte text values and 1024-byte RE2 patterns. Expressions are data, never
executable scripts. Missing strings/process/status are unknown: `not` and `neq`
cannot turn missing evidence into a positive match. AND/OR use three-valued
logic; only a final true includes a row. Body sizes compare reported contract
values, including zero, without inferring cache origin.

| Fields | Operators |
|---|---|
| status, requestSize, responseSize, processPID | eq, neq, lt, lte, gt, gte; nonnegative int64 value |
| url, host, path, method, mime, processName, scheme, httpVersion, coverage, fidelity | eq, neq, contains, startsWith, endsWith, regex, like; string value; optional ignoreCase |

LIKE uses `%` for any sequence and `_` for one character; MIME excludes its
parameter suffix. Filtering selects an immutable view and does not delete rows.
The 16 observed supported native presets still pass; the cache-origin preset and
unobserved native nested encodings retain explicit refusal.

### Repeatable stability runs (Mac follow-up 5, 2026-11-11)

From the repository root, run:

```sh
node scripts/capture-soak.mjs --duration 120s --output /tmp/capture-soak.json --race
```

The same Node command runs on macOS and Windows (choose a suitable output path).
Integer seconds/minutes/hours from 2s to 6h are accepted. The manual **Capture
common-engine soak** workflow offers 120s, 30m, 3h and 6h on macos-15 and Windows
with metrics artifacts. Duration controls workload time; build/shutdown add
overhead. Reports include revision, actual OS/architecture, cycles/requests,
sampled heap/goroutine peaks, store bytes and retention bounds. Samples occur at
cycle boundaries; they are observations, not a performance SLO or leak proof.

Each cycle uses four real loopback HTTP clients, pause/resume, Clear, concurrent
status/recovery queries, deterministic pre-storage rejection, graceful/idempotent
Stop, exact listener-port reuse, 64-row physical retention, counter invariants,
store reopening and a portable export/re-import. Finalized fixture files are
removed after verification so the harness's disk footprint stays bounded. The
ordinary short regression retains its three-cycle baseline. No system proxy,
trusted CA, user browser data, native GUI or transparent driver is configured.
The fixture uses the common Windows capture service on Mac; report OS makes that
scope explicit. Full native GUI/driver and original-client soak remain T-613.

### Hosted follow-up results (2026-11-11)

All five implementation commits at 3209bcc pass Engine Native 42928859923,
general CI 42928859913 and native amd64-race/386 COM 42928859864. Manual
common-engine soak 42928876605 passes on macOS (70 cycles, 18,200 requests)
and Windows (37 cycles, 9,620 requests), with a 64-row physical bound.
The test-only follow-up 4feed2d also passes COM 42929282301, including actual
public Register/replacement/Unregister, owned registry/file cleanup and
separate-process custom-IID typed calls on both architectures. These cases
executed; only the private presets/child entry have expected standalone skips.
[Scoped evidence and artifact digests](../review/evidence/2026-11-11-five-followups/ci.json)
separate product checks, local and hosted soak, and the test-only follow-up.
T-611 remains REVIEW for current vendor-client interoperability, missing native
filter/cache-origin evidence and foreign-process in-memory cookies. Transparent
driver reach, full native menu/DPI and GUI/driver long-run parity remain separate.


## 2026-11-14 remaining T-613 acceptance

The16-case contract is scripts/http-standalone-parity-contract.json. The detailed
case-by-case completed slice, missing observation and environment gate is in the
matching dated table in docs/ko/SYSTEM_HTTP_CAPTURE.md. Earlier4/16 automation
reports are immutable; later selected native receipts do not silently rewrite
them into an overall parity PASS.

Additional local cases can cover grid/group/search/keyboard, notes through
sort/save/reload, Builder explicit loopback sends with immutable source, timing,
inert content/legacy states and current-source crash/restart recovery. Genuine
675-entry HALOG codec byte roundtrip and the two-hour fixed capture runs on both
architectures are already recorded; they are not newly missing tests.

Full live BODY depends on the existing verified body gate (currently disabled).
Transparent/proxy-ignoring/service reach requires an implemented Windows backend;
it is not supplied by Linux Docker. Vendor COM foreign-process/session-cookie
semantics and collision-free public registration need the appropriate original
client and clean Windows user/VM, preserving current registrations. Full reference
interoperability is distinct from ABI/local-ArchScope contract tests.

Root memory guards do not establish total WebView or handle-leak acceptance:
previous two-hour tree private bytes and root handles increased. Longer/idle,
pressure and input comparisons with diagnosis remain separate; no Windows race
claim is made with CGO=0. Crash recovery with uncommitted staging/blob cleanup
also does not establish power-loss or every committed-corruption/low-disk fault.
T-613 stays PENDING; additional receipts are in the dated remaining-acceptance
folder under docs/review/evidence.


### 2026-11-14 exact recovery reporting

Forced termination plus27B uncommitted rolling staging and35B orphan blob exposed recovery reporting0B. OpenWithRecovery now forwards the actual discarded bytes and retained row count. Both capture suites, app42.123s/44.388s and frontend state/build pass. Current production crash/restart preserves12record bytes/IDs/selection and native62B on both arches; ambiguous concurrentamd64 images remain INCONCLUSIVE and a single-window rerun is separate. This is not power-loss, committed-row corruption or leak-free acceptance; earlier2h capture binaries predate this reporting fix.

Evidence: docs/review/evidence/2026-11-14-acceptance-remaining/recovery-summary.json.

### 2026-11-14 live columns, grouping, stored search and annotations

`d542b85a` fixes immediate annotation-save failure when a Windows reader briefly
denies delete sharing. The same250ms native reader lock fails on baseline and
passes on both fixed architectures; permanent-lock tests preserve committed
bytes and remove staging. `eab03d07` keeps Offset relative to each PID/start
process regardless of display grouping. Baseline changed the same row from0s
to0.015s; the six-group amd64 native matrix and state regression verify the fix.

Both production desktops capture600 real requests from two confirmed Node
producers to a Docker loopback origin. Stored search finds60 old requests outside
the500-row native tail. Ten isolated field searches per run, native25/25/10
pagination, six-group amd64 counts/bytes and both architectures' column order,
width and restart persistence are covered. Omitted body/empty POST searches are
negative checks, not positive retained-body search. Cookie-name search preserves
value redaction. The complete six-group native matrix is amd64;386 covers process
groups, pagination, columns and the annotation lifecycle.

Another80 requests produce680 persisted/640 retained/40 evicted with zero drops;
fresh search finds20 remaining archive rows. The old cursor refuses Next, and
manual Search recovers. Its message remains generic English; no automatic
refresh/localized guidance claim is made. Live star/Korean note on sequence50
survives arrivals, retention, selection, stopping, whole-store saving, CtrlO and
a fresh process. Independent readback verifies all640 IDs, selection and note.
Portable normalization intentionally derives host-with-port/query from retained
redacted URL; all other fields except the one star/comment match the live store.
This is not a raw-byte identity claim across representations.

Both24-package capture suites, production app36.431s/47.402s and state/build pass.
The40 native observations retain37 selected PASS and3 historical baseline FAIL.
Every optional column value, drag resize, filter/keyboard combination, annotation
length/redaction/permanent-error UI and full reference/locale/DPI matrix remain
separate. Vendor COM/foreign cookies, transparent/service reach, body policy and
longer memory gates remain; earlier2h receipts are unchanged. T-613 stays PENDING.

Evidence: docs/review/evidence/2026-11-14-live-grid/README.md.
