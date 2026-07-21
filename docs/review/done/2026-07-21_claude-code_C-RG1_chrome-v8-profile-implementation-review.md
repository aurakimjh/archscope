# C-RG1 — Chrome/V8 Browser CPU Profile Release Implementation Review

- Review ID: **C-RG1** (task **T-578**)
- Date: 2026-07-21
- Reviewer: Claude (independent implementation review, findings-only)
- Baseline: `main` @ `558e8d3`
- Scope authority: `docs/en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` §5 (C-RG1)
  and the locked design contract `docs/{en,ko}/CHROME_DEVTOOLS_CPUPROFILE.md` (T-558/T-559/T-560/T-561)
- Verdict: **CONDITIONAL** — does **not** unblock H-RG1

> This document contains findings only. Per the review workflow, no source was
> modified. Engine findings (A–E) are owned by **Codex**; UI findings (U1–U5)
> are owned by a **Claude** implementation session. After fixes, the original
> reviewer (this session) re-verifies before a re-issued verdict.

---

## 요약 (Korean TL;DR)

Chrome/V8 브라우저 CPU 프로파일 릴리스 구현(T-558~T-565)을 확정 계약과
독립 검증했다. 파서 정규화·그래프 검증·바운드 스트리밍/다운샘플·"sampled CPU run은
browser Long Task가 아니다" 계약·단일 `AnalyzeProfileEvidence` 경로·CLI/Wails 패리티는
견고하며 15개 픽스처 골든이 모두 통과한다. 다만 두 개의 P1이 남아 **PASS가 아니라
CONDITIONAL**이다.

1. **엔진 P1(A)** — 시간 귀속 방향이 확정 계약(4.3.1 결정 4: `timeDeltas[i]` → 샘플
   `i-1`)과 반대로 구현됨(`timeDeltas[i]` → 샘플 `i`, 후방 귀속). 구현·골든·픽스처의
   실제 `RunTask` 경계는 오히려 구현 쪽이 맞아 보이므로 **코드가 아니라 문서가 틀렸을
   가능성**이 크다. 문제는 골든 허용오차(1000µs = 1샘플 간격)가 두 규약을 구분하지
   못해 "골든이 이를 고정한다"는 계약 문장이 거짓이라는 점 — 계약과 구현/테스트를
   반드시 일치시켜야 한다.
2. **UI P1(U1, U2)** — `metadata.diagnostics` 경고를 화면에 전혀 렌더링하지 않아
   hitCount-only·음수델타·hitCount 불일치·다운샘플 경고가 조용히 사라진다. 또한
   플레임그래프/드릴다운이 이 페이지에 없다(엔진은 데이터를 제공).

CONDITIONAL이므로 H-RG1은 시작할 수 없다. 아래 A–E(Codex), U1–U5(Claude) 수정 후
재검증한다.

---

## 1. What was reviewed

Independent verification (not merely trusting existing tests) of:

- **Parser** `internal/parsers/profile/{parser.go,v8.go}` — normalization, units,
  V8 graph validation, gzip/JSON bounded streaming, downsampling, hitCount policy.
- **Analyzer** `internal/analyzers/profile/analyzer.go` — `AnalyzeProfileEvidence`
  path, summary/units, `cpu_sample_runs`/`cpu_activity`, `SAMPLED_CPU_HOTSPOT`,
  temporal suppression, bounded outputs, source-aware category/color.
- **Classification** `internal/analyzers/profileclassification/`.
- **Diff / Workspace / Export / CLI** wiring
  (`cmd/archscope-app/profilerservice.go`, `api/api.go`, `cmd/archscope-engine/cmd_profile.go`).
- **UI** `cmd/archscope-app/frontend/src/pages/BrowserCpuProfilePage.tsx` and state/export flow.
- **Goldens** the shared 15-fixture manifest
  (`projects-assets/test-data/browser-profile-fixtures/manifest.json`) via
  `internal/analyzers/profile/fixture_manifest_test.go`.

### Evidence run (2026-07-21, this session, go1.26.5 darwin/arm64)

- `go test ./internal/parsers/profile/... ./internal/analyzers/profile/... ./internal/analyzers/profileclassification/... ./internal/profiler/... ./cmd/archscope-app/... ./cmd/archscope-engine/...` → **all `ok`** (15/15 manifest fixtures pass).
- `go vet` (profile packages) → clean. `go build ./...` → success (only benign macOS linker version warnings).
- Frontend `node_modules` present; frontend build/tests not re-run this session (UI verified by source reading).

---

## 2. Confirmed PASS items (acceptance evidence)

These C-RG1 criteria were independently verified and hold:

- **Normalization & formats.** Chrome Performance `.json`/`.json.gz`, V8
  `.cpuprofile`/gzip, object- and array-form traces, Profile/ProfileChunk with
  sibling `timeDeltas` all parse (manifest `valid/*`, `parser_test.go`).
- **Units.** `ValueUnit == "microseconds"` for browser/V8; `Sample.Value` is
  `int64` µs; ms conversion happens once at summary/display
  (`analyzer.go:291-295`). hitCount-only inputs use `ValueUnit=="samples"` and
  make no duration claim (`v8.go:192`).
- **Graph validation is strict, not silent.** node id 0, duplicate id, multiple
  parents, missing child, parent cycle, and unknown sampled node all error
  (`v8.go:205-256`); `broken-parent`/`sample-unknown-node` fixtures reject via
  `PROFILE_PARSE_ERROR`.
- **Bounded streaming.** 256 MiB stat guard + a `cappedProfileReader` that makes
  a gzip bomb and an over-limit plain file fail with the same message
  (`parser.go:237-288`); traces stream via `json.Decoder` token walk, never a
  whole-file `[]RawMessage` (`v8.go:441-505`).
- **Downsampling preserves total time.** 500k default cap; the weighted
  collector sums skipped intervals into bucket weights so duration stays exact
  (`v8.go:258-289`), and downsampled results set `partial_result` +
  `PROFILE_DOWNSAMPLED`.
- **"Sampled CPU run is not a browser Long Task."** Engine emits only
  `SAMPLED_CPU_HOTSPOT` at ≥100 ms with explicit "observed sample window, not a
  task boundary" text (`analyzer.go:104-106`); a Go golden fails if any
  sample-derived finding uses "long task" wording
  (`fixture_manifest_test.go:117-121`); UI copy repeats the disclaimer twice
  (`BrowserCpuProfilePage.tsx:35,39`). **This item is explicitly approved.**
- **Temporal suppression (engine side).** When `partial_result` or hitCount-only,
  `cpu_sample_runs`/`cpu_activity` are omitted and
  `TIMELINE_SUPPRESSED_DOWNSAMPLED` is raised (`analyzer.go:97-110`;
  `analyzer_test.go`). No time-axis claim is fabricated by the engine.
- **Single path + parity.** `api.AnalyzeProfileEvidence = analyzers/profile.Analyze`
  (`api/api.go:249`); CLI `profile import` calls the same `profile.Analyze`
  (`cmd_profile.go:28`); Diff routes V8/Chrome through
  `parsers/profile.ParseFile` rather than the legacy switch
  (`profilerservice.go:296-307`); manifest AC-1 asserts cpuprofile↔trace duration
  parity (`fixture_manifest_test.go:128-132`). Workspace/Export consume the
  standard `AnalysisResult`.

---

## 3. Findings

Severity: **P0** blocker · **P1** major (must fix before PASS) · **P2** minor · **P3** nit.
Owner: **Codex** (engine) · **Claude** (UI).

### A. [P1 · Codex] Time-attribution direction contradicts the locked contract, and the golden does not pin it

- **Where:** `internal/parsers/profile/v8.go:142-162` (and the identical trace
  loop `v8.go:409-428`); contract `docs/en/CHROME_DEVTOOLS_CPUPROFILE.md:149-155`
  ("Decision 4"), `docs/ko/…:532`.
- **Contract (locked, T-559):** "Cost of sample `i` = `ts[i+1] - ts[i]`; the last
  sample gets `endTime - ts[i]`. Thus **`timeDeltas[i]` is the cost of sample
  `i-1`, not sample `i`** — an off-by-one here shifts all attribution, so golden
  tests pin it."
- **Implementation:** assigns `Sample.Value = timeDeltas[i]` to sample `i`
  (sample 0 gets 0), i.e. **backward** attribution (`timeDeltas[i]` → sample `i`),
  the opposite of the contract. It also never attributes the trailing
  `endTime - ts[last]` interval.
- **Verification against the fixture (`scripts/generate_fixtures.py:83-99`):**
  `renderList` (node 3) is sampled at indices 3100–3309 with `startTime=1_000_000`
  and 1 ms deltas. The backward (code) attribution yields run start (relative)
  `3_100_000 µs` and duration `210_000 µs` — an **exact** match to the fixture's
  own `ph:"X"` RunTask boundary (`ts=start+3_100_000, dur=210_000`). The
  contract's forward attribution would yield start `3_101_000 µs`. The manifest
  tolerance is `> 1_000 µs` (`fixture_manifest_test.go:223,227`) — exactly one
  sample interval — so **both conventions pass**. The contract's claim that
  "golden tests pin it" is therefore false.
- **Impact:** A load-bearing measurement rule is (a) implemented opposite to its
  own locked contract and (b) not disambiguated by any test. On the evidence the
  **code appears correct** (it matches Chrome's own `timeDeltas[i]`→`samples[i]`
  behavior and the fixture RunTask), which points to the **doc** being wrong — but
  an acceptance gate cannot "explicitly approve the time invariant" while the
  normative text and the implementation disagree.
- **Recommendation (Codex, joint with doc owner):** Decide the intended
  convention against real Chrome semantics. Almost certainly: correct Decision 4
  prose to backward attribution and keep the code. Then **tighten the golden** so
  it actually distinguishes the two (assert the boundary sample exactly, or
  tolerance `< 1000 µs`), and add the `endTime - ts[last]` tail handling (see B).

### B. [P2 · Codex] Contract-required duration decomposition is not emitted; total drops head/tail

- **Where:** `internal/analyzers/profile/analyzer.go:274-296` (summary);
  contract `CHROME_DEVTOOLS_CPUPROFILE.md:153-158`.
- Decision 4 requires four separately-recorded values: `recording_duration_us`
  (`endTime - startTime`, "constant regardless of display options"),
  `active_duration_us`, `idle_duration_us`, `sampled_duration_us`. A repo-wide
  grep finds **none** of them emitted.
- The summary exposes only `total_duration_us`, which equals `Σ timeDeltas[1..n-1]`
  — it silently drops `timeDeltas[0]` (start→first-sample gap) **and** the
  `endTime - ts[last]` tail, so it is neither the recording duration nor the
  active-CPU duration, and there is no idle accounting.
- **Impact:** UI/Export cannot honestly show recording length vs. active CPU vs.
  idle; the single conflated number is off from the true recording window by the
  untracked head+tail.
- **Recommendation:** Emit the four values per contract; keep `recording_duration_us`
  display-invariant.

### C. [P2 · Codex] chrome-trace path clamps negative deltas silently (no diagnostic parity)

- **Where:** `internal/parsers/profile/v8.go:412-417` vs. `v8.go:146-150`.
- The `.cpuprofile` path counts clamps and raises `PROFILE_NEGATIVE_DELTA_CLAMPED`
  (`parser.go:166-168`). The chrome-trace path clamps `delta < 0` to `0` **without
  counting or emitting any diagnostic**. The plan states both formats share one
  normalization core; a negative delta in a Chrome trace is swallowed with no
  user-visible signal.
- No chrome-trace negative-delta fixture exists, so this gap is untested.
- **Recommendation:** Share the clamp-count/diagnostic between both entry points;
  add a `malformed/trace-negative-delta.json` fixture.

### D. [P3 · Codex] chrome-trace path skips the hitCount cross-check (INV-C3)

- **Where:** `internal/parsers/profile/v8.go:337-439` vs. `v8.go:164-171`.
- `hit_count_mismatch_nodes` (→ `PROFILE_HITCOUNT_MISMATCH`, the INV-C3
  samples-vs-hitCount drop detector) is computed only in `parsedV8Profile`
  (cpuprofile), never in `parseChromeTraceReader`. Trace inputs get no dropped-
  subtree cross-check.
- **Recommendation:** Compute the cross-check in the trace path too.

### E. [P3 · Codex] First-sample guard breaks when `startTime == 0`

- **Where:** `internal/analyzers/profile/analyzer.go:212,238`
  (`value == 0 && sample.TimestampUS != 0`).
- The guard that drops the first (duration-0) V8 sample keys on `TimestampUS != 0`.
  A valid profile with `startTime: 0` gives the first sample `TimestampUS == 0`,
  so it is counted as `1` instead of skipped — a small count/summary skew.
- **Recommendation:** Track "is first sample" explicitly (e.g. an index or a
  `Value==0 && ProfileKind=="cpu"` marker) rather than inferring from timestamp.

### U1. [P1 · Claude] Engine diagnostics are never surfaced; degraded results look clean

- **Where:** `BrowserCpuProfilePage.tsx:28-39` — the page reads only
  `metadata.parser_metadata.partial_result`; it never reads
  `result.metadata.diagnostics` and mounts no `DiagnosticsPanel` (every other
  analyzer page does).
- Consequence: `PROFILE_HITCOUNT_ONLY`, `PROFILE_NEGATIVE_DELTA_CLAMPED`,
  `PROFILE_HITCOUNT_MISMATCH`, and even `PROFILE_DOWNSAMPLED` /
  `TIMELINE_SUPPRESSED_DOWNSAMPLED` are **silently dropped** from the UI.
- Worse for hitCount-only: the engine sets the warning but does **not** set
  `partial_result` (`parser.go:169` vs. `markProfileDownsample` at
  `parser.go:204-206`), so that path shows **no card and no diagnostic at all** —
  the "Sampled CPU runs" table is simply empty (`:39`) with a `Start (µs)` header
  and no explanation. This is the exact "make no time-axis claim" surface that
  must instead say *why* it is empty.
- **Recommendation:** Mount `DiagnosticsPanel` reading `result.metadata.diagnostics`;
  add an explicit empty-state for suppressed/aggregated timelines. (Engine may
  also set `partial_result`/a suppression flag for hitCount-only — coordinate
  with Codex.)

### U2. [P1 · Claude] Flamegraph and drilldown are absent, though the acceptance item is checked

- **Where:** `BrowserCpuProfilePage.tsx` (whole file) and `AnalysisWorkspacePage.tsx`.
- The plan's Claude UI scope marks "Flamegraph, drilldown, and Workspace flow"
  as done, but the page renders only four `MetricCard`s and a 50-row table — no
  `CanvasFlameGraph`, no `DrilldownPanel` — and the Workspace card offers no
  flamegraph either. The engine already produces `charts.flamegraph` +
  `drilldown_stages` (`analyzer.go:77-80`), so the data exists but is unused.
- **Impact:** the primary CPU-profile visualization is missing from the feature.
- **Recommendation:** Render `CanvasFlameGraph` (+ optional drilldown) for the
  `profile_evidence` result, matching the other profiler pages.

### U3. [P2 · Claude] `.json.gz` is engine-supported but not offered by the UI

- **Where:** `BrowserCpuProfilePage.tsx:35` — `accept=".json,.cpuprofile"`,
  filter `*.json;*.cpuprofile`, and copy naming only trace/`.cpuprofile`.
- The engine parses gzipped Chrome traces (`parser_test.go:158,199`; manifest
  `valid/chrome-trace-object.json.gz`), and the C-RG1 scope names `.json.gz`. A
  user can only reach it via the catch-all `*.*` filter.
- **Recommendation:** Add `.json.gz`/`.gz` to `accept`, the file filter, and the
  description.

### U4. [P2/P3 · Claude] Missing empty-state, non-i18n copy, minor table a11y

- No pre-result empty/guidance state; the runs table renders an empty body with
  no note when `cpu_sample_runs` is absent (`:39`). (P2)
- All page-specific strings are hardcoded English ("Samples", "CPU duration (ms)",
  "Sampled CPU runs", the partial-result card, the disclaimers) — this page does
  not use `t()` for its own copy, unlike sibling pages (`:37-39`). (P2)
- Table `<th>` cells lack `scope="col"`; the `<table>` has no caption/aria (`:39`).
  (P3) The amber partial-result card pairs color with a text title (not color-only)
  — acceptable.

### U5. [P2 · Claude] No frontend regression test for this page

- No `*.test.ts(x)` references `BrowserCpu`, `analyzeProfileEvidence`,
  `cpu_sample_runs`, or the Long-Task wording. The "not a Long Task" copy and the
  diagnostic-visibility behavior are unguarded on the UI side (the only Long-Task
  guard is Go-side, `fixture_manifest_test.go:118`).
- **Recommendation:** Add a state/render test covering suppressed-timeline copy,
  diagnostic rendering, and the Long-Task disclaimer.

---

## 4. Verdict and gate decision

**CONDITIONAL.** Parsing, units, graph validation, bounded streaming/downsampling,
the sampled-CPU-vs-Long-Task contract, the single analysis path, and CLI/Wails
parity are solid and the 15-fixture goldens pass. However:

- **A (P1)** leaves the core time-attribution contract contradicted by the
  implementation and unpinned by the goldens, and
- **U1/U2 (P1)** leave degraded results silently unexplained and the primary
  flamegraph/drilldown visualization absent.

Per `BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` §3, **CONDITIONAL is not
a pass and does not unblock H-RG1.**

### Required before re-review

| # | Owner | Severity | Item |
|---|---|---|---|
| A | Codex | P1 | Reconcile time-attribution direction (doc vs. code) and tighten the golden so it distinguishes the convention; handle the `endTime` tail |
| B | Codex | P2 | Emit `recording/active/idle/sampled_duration_us`; stop conflating in `total_duration_us` |
| C | Codex | P2 | Chrome-trace negative-delta clamp diagnostic parity + fixture |
| D | Codex | P3 | Chrome-trace hitCount cross-check (INV-C3) |
| E | Codex | P3 | First-sample guard independent of `startTime==0` |
| U1 | Claude | P1 | Render `metadata.diagnostics`; explain suppressed/empty timeline (esp. hitCount-only) |
| U2 | Claude | P1 | Add flamegraph + drilldown |
| U3 | Claude | P2 | Advertise `.json.gz` support |
| U4 | Claude | P2/P3 | Empty-state, i18n copy, table a11y |
| U5 | Claude | P2 | Frontend regression test for the page |

After Codex (A–E) and Claude (U1–U5) land fixes, this reviewer re-verifies
(goldens + targeted re-reads) and re-issues the verdict. A PASS additionally
requires the tightened attribution golden from A to be green.
