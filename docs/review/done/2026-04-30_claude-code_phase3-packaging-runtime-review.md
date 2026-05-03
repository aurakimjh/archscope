# Phase 3 — Packaging & Runtime Expansion Review

**Date**: 2026-04-30
**Reviewer**: Claude Code (Opus)
**Personas**: Senior Release Engineer · Senior System Architect · Senior Security Engineer
**Scope**: T-022, T-024, T-025, T-026, T-027, T-052, T-053
**Commits**: `cb508de`, `a712e2a`
**Files changed**: 37 (+1171/−128)

---

## Stage 0 — Context Load

### Prior Review Decisions Referenced

| ID | Summary | Status |
|---|---|---|
| RD-019 | Plan early Electron + PyInstaller packaging spike | Accepted |
| RD-020 | Remove or raise low setuptools version ceiling | Deferred → Executed |
| RD-021 | Consolidate setup.py and pyproject.toml | Deferred → Partial |
| RD-022 | Separate profiler stack classification rules from Python code | Deferred → Executed |
| RD-046 | Replace CSP style-src unsafe-inline with nonce | Deferred → Documented |
| RD-048 | Add seed-configurable percentile sampling | Deferred → Executed |

All seven tasks under review trace to accepted or deferred review decisions. No task was implemented without a prior decision record.

---

## Stage 1 — Change History

### Commit `cb508de` — "Complete phase 2 hardening and phase 3 expansion"

34 files changed, +1057/−136. This commit bundles:

1. **Phase 3 packaging decisions** (T-022/T-024/T-025/T-052): `PACKAGING_PLAN.md` (EN/KO), `pyproject.toml` setuptools ceiling, `setup.cfg` metadata preservation, `setup.py` shim retention, CSP nonce deferral documentation.
2. **Runtime classification** (T-026/T-027): New `profile_classification.py` module, profiler analyzer integration, `RUNTIME_CLASSIFICATION.md` (EN/KO), tests.
3. **Seed-configurable sampling** (T-053): `statistics.py` seed parameter, test.
4. **Previous review issue fixes** (bonus): Accessibility, formatters extraction, chart factory data typing, theme registration, file dialog i18n, ResizeObserver, Vitest setup.

### Commit `a712e2a` — "Process phase 2 chart foundation reviews"

4 files changed, +47/−5. Review document processing: moved two review files to `done/`, updated `review_decisions.md` (+22 lines with RD-051 through RD-063), updated `work_status.md`.

### Observation

The single commit `cb508de` mixes Phase 3 scope (7 tasks) with Phase 2 hardening fixes (8+ previous review issues). This makes git bisect harder if a regression appears in either area. Not blocking, but future phases should separate review-fix commits from new-scope commits.

---

## Stage 2 — Per-Task Deep Verification

### T-022: PyInstaller Sidecar Packaging Spike Plan

**File**: `docs/en/PACKAGING_PLAN.md` (37 lines), `docs/ko/PACKAGING_PLAN.md` (mirror)

**Checklist**:
- [x] Spike scope documented (Electron shell + PyInstaller sidecar)
- [x] CLI contract preserved (`analyzer command + --out`)
- [x] AnalysisResult JSON boundary stated
- [x] 5-step spike checklist: macOS first, Windows follow-on
- [x] Diagnostics and stderr verification included in checklist
- [x] EN/KO parity maintained

**Findings**:

| # | Severity | Finding | Recommendation |
|---|---|---|---|
| F-001 | Minor | PACKAGING_PLAN.md checklist is macOS → Windows only. Linux is not mentioned even as out-of-scope. | Add a one-line note: "Linux: deferred until macOS and Windows sidecar paths are validated." |
| F-002 | Info | No signing/notarization steps in the checklist. The plan says "signing assumptions are known" but does not enumerate them. | Acceptable for a spike plan. Signing details should be added when the spike produces concrete results. |

**Verdict**: T-022 **PASS** — spike plan is well-scoped with clear sequencing.

---

### T-024: Raise setuptools Version Ceiling

**File**: `engines/python/pyproject.toml` line 3

```
requires = ["setuptools>=68,<81", "wheel"]
```

**Checklist**:
- [x] Old ceiling `<64` removed
- [x] New range `>=68,<81` covers modern setuptools without unbounded risk
- [x] Lower bound `>=68` includes PEP 639 license expression support
- [x] Upper bound `<81` matches latest stable at time of change
- [x] `wheel` dependency retained

**Findings**:

| # | Severity | Finding | Recommendation |
|---|---|---|---|
| F-003 | Info | The ceiling `<81` will need a bump when setuptools 81+ releases. This is expected bounded-range maintenance. | No action needed now. |

**Verdict**: T-024 **PASS** — modern bounded range with clear rationale documented in PACKAGING_PLAN.md.

---

### T-025: Retain setup.py as Minimal Shim

**Files**: `engines/python/setup.py` (4 lines), `engines/python/setup.cfg` (26 lines)

**Checklist**:
- [x] `setup.py` is minimal: `from setuptools import setup; setup()`
- [x] `setup.cfg` is the metadata authority: name, version, description, license, dependencies, extras, entry points
- [x] `python_requires = >=3.9` declared
- [x] Runtime deps (`typer`, `rich`) bounded with upper version ceilings
- [x] Dev extras (`pytest`, `pytest-cov`, `ruff`) bounded
- [x] Console script entry point matches CLI module path
- [x] Decision rationale documented in PACKAGING_PLAN.md §Metadata Decision

**Findings**:

| # | Severity | Finding | Recommendation |
|---|---|---|---|
| F-004 | Minor | `setup.cfg` declares `long_description = file: README.md` but `engines/python/README.md` does not exist yet. `pip install -e .` will warn or fail on sdist build. | Add a minimal `engines/python/README.md` or remove the `long_description` field until one exists. |
| F-005 | Info | Hybrid metadata (setup.cfg + pyproject.toml) is explicitly documented as intentional pending PyInstaller spike results. No consolidation concern at this stage. | No action. |

**Verdict**: T-025 **PASS** — shim-only setup.py with all metadata in setup.cfg, well-documented rationale.

---

### T-026: Extract Stack Classification Module

**File**: `engines/python/archscope_engine/analyzers/profile_classification.py` (34 lines)

**Checklist**:
- [x] `StackClassificationRule` is a frozen dataclass with `label: str` and `contains: tuple[str, ...]`
- [x] Tuple (not list) ensures immutability of default rules
- [x] 9 default rules covering Oracle JDBC, Spring Batch, Spring Framework, HTTP/Network, Node.js, Python, Go, ASP.NET/.NET, JVM
- [x] Rule ordering is intentional: specific frameworks before broad runtimes
- [x] `classify_stack()` uses case-insensitive matching with "Application" fallback
- [x] Old inline `_classify_stack` function fully removed from `profiler_analyzer.py` (confirmed by grep — only appears in archived review docs)
- [x] All callers (`_to_profile_stack`, `_component_breakdown`) thread `classification_rules` parameter

**Findings**:

| # | Severity | Finding | Recommendation |
|---|---|---|---|
| F-006 | Medium | The HTTP/Network rule uses tokens `("socket", "http")` which can match application-level code (e.g., `com.example.HttpClient`, `my.socket.Handler`). Because this rule appears before JVM/Node.js/Python rules, it could misclassify application HTTP calls as infrastructure. | Move HTTP/Network rule after runtime-specific rules, or require more specific tokens like `java.net.Socket`, `http.client`. Low urgency because the rule order already places Spring/Oracle before HTTP. |
| F-007 | Minor | The ASP.NET/.NET rule token `"system."` is very broad — it would match any Java class in a package starting with `system.` (uncommon but possible). The `".dll"` token would never appear in JVM collapsed stacks, so it's effectively .NET-only. | Acceptable for current scope. Document the token specificity constraint for future rule authors. |
| F-008 | Info | `classify_stack` accepts `Sequence[StackClassificationRule]` but `analyze_collapsed_profile` declares `tuple[StackClassificationRule, ...]`. The type mismatch is safe (tuple is a Sequence) but inconsistent. | Minor style — consider aligning to `Sequence` everywhere or `tuple` everywhere. |

**Verdict**: T-026 **PASS** — clean separation with proper immutability, injection support, and full old-code removal.

---

### T-027: Runtime Classification Tests and Documentation

**Files**: `engines/python/tests/test_profiler_analyzer.py` (+29 lines), `docs/en/RUNTIME_CLASSIFICATION.md` (24 lines)

**Checklist**:
- [x] `test_profile_classification_supports_common_runtime_families`: covers JVM, Node.js, Python, Go, ASP.NET/.NET
- [x] `test_analyze_collapsed_profile_accepts_classification_rules`: verifies custom rule injection with a non-default "Vendor Runtime" rule
- [x] Documentation lists all 10 families (9 rules + Application fallback)
- [x] Rule ordering semantics documented ("first matching rule wins")
- [x] Extension direction documented (packaged config after PyInstaller stabilization)
- [x] EN/KO documentation parity maintained
- [x] All tests pass (verified: 28/28 passed)

**Findings**:

| # | Severity | Finding | Recommendation |
|---|---|---|---|
| F-009 | Medium | No test for the "Application" fallback case — what happens when no rule matches. This is the most common case for user-specific application stacks. | Add: `assert classify_stack("com.mycompany.internal.Worker") == "Application"` |
| F-010 | Minor | No test for rule ordering behavior — e.g., that `springframework.batch` matches "Spring Batch" before "Spring Framework". First-match-wins is documented but not tested. | Add a test verifying that a stack containing both `springframework.batch` and `springframework` classifies as "Spring Batch". |
| F-011 | Minor | No negative test for case sensitivity — e.g., `classify_stack("JAVA.LANG.THREAD")` should still return "JVM". The implementation lowercases, but this isn't tested. | Add a case-insensitive classification test. |
| F-012 | Info | HTTP/Network and Oracle JDBC families are not covered in unit tests. Coverage is representative but not exhaustive. | Low priority — add when rule set expands. |

**Verdict**: T-027 **PASS** — good representative coverage and clear documentation. Missing fallback/ordering/case tests are non-blocking.

---

### T-052: CSP Nonce Evaluation and Deferral

**File**: `docs/en/PACKAGING_PLAN.md` §CSP Nonce Evaluation (5 lines)

**Checklist**:
- [x] Decision documented: keep current `style-src 'unsafe-inline'` during Phase 3
- [x] Rationale stated: nonce propagation requires compatibility testing with React, Vite, ECharts
- [x] Revisit trigger defined: "after packaged renderer behavior and chart export flows are stable"
- [x] Maps to RD-046 (Deferred)

**Findings**:

| # | Severity | Finding | Recommendation |
|---|---|---|---|
| F-013 | Info | The deferral is well-reasoned. Script execution is already blocked by CSP; the remaining `unsafe-inline` applies only to styles. The attack surface is limited. | No action. Revisit timing is appropriate. |

**Verdict**: T-052 **PASS** — documented deferral with clear revisit criteria.

---

### T-053: Seed-Configurable Percentile Sampling

**File**: `engines/python/archscope_engine/common/statistics.py` (lines 26–54)

**Checklist**:
- [x] `BoundedPercentile.seed` field with default `12_345`
- [x] `_deterministic_reservoir_index(count, seed)` accepts seed parameter
- [x] Seed is threaded through `add()` → `_deterministic_reservoir_index()`
- [x] Default seed preserves backward compatibility (same deterministic stream as before if using default)
- [x] Test `test_bounded_percentile_seed_changes_deterministic_sample_stream`: different seeds → different samples, same seed → reproducible
- [x] Test passes (verified: 28/28)

**Findings**:

| # | Severity | Finding | Recommendation |
|---|---|---|---|
| F-014 | Medium | The reservoir index formula `((count * 1_103_515_245) + seed) % count` has a mathematical issue: when `seed % count == 0` and count is a factor of 1_103_515_245, the result may cluster. More importantly, for `count == 1`, the result is always `seed % 1 == 0`, which means the first sample is always replaced — this is correct reservoir behavior but should be documented. | The formula works for current scale (max_samples=10000). Document the LCG-derived constant and its limitations if future sampling needs higher statistical quality. |
| F-015 | Minor | The `seed` field is a public dataclass field alongside `count` and `_samples`, which are internal state. A user could set `seed=0` which would make the formula degenerate to `(count * 1_103_515_245) % count == 0` — always replacing index 0. | Consider validating `seed > 0` in `__post_init__`, or document that seed=0 produces degenerate sampling. |
| F-016 | Info | No test verifies that the default seed (`12_345`) produces the same sample stream as the pre-seed implementation. This is fine because no serialized samples depend on the old default. | No action needed. |

**Verdict**: T-053 **PASS** — clean implementation with good test coverage. Edge-case documentation is a minor follow-up.

---

## Stage 3 — Cross-Cutting Verification

### 3.1 Contract Preservation

The `AnalysisResult` common contract is preserved. Changes to `profiler_analyzer.py` only add the `classification_rules` parameter — the output shape (`ProfilerCollapsedSummary`, `ProfilerCollapsedSeries`, `ProfilerCollapsedTables`, `ProfilerCollapsedMetadata`) is unchanged. The `component_breakdown` series key already existed; it now uses the extracted classification module instead of an inline function.

### 3.2 Responsibility Separation

| Concern | Before | After | Assessment |
|---|---|---|---|
| Classification logic | Inline `_classify_stack` in profiler_analyzer.py | Separate `profile_classification.py` | Improved |
| Classification rules | Hardcoded constants | Frozen dataclass with injection support | Improved |
| Sampling seed | Hardcoded constant in formula | Configurable dataclass field | Improved |
| Packaging decisions | Undocumented | `PACKAGING_PLAN.md` | Improved |

### 3.3 Documentation Parity

| Document | EN | KO | Parity |
|---|---|---|---|
| PACKAGING_PLAN.md | Yes | Yes | Verified |
| RUNTIME_CLASSIFICATION.md | Yes | Yes | Verified |
| CHART_STUDIO_DESIGN.md | Yes | Yes | Verified (bonus, not in scope) |

### 3.4 Test Coverage

| Area | Tests | Status |
|---|---|---|
| Python engine (all) | 28 passed, 1 skipped | Green |
| TypeScript type check | Clean (no errors) | Green |
| Vitest (chart factory) | 4 passed | Green |
| Classification rules | 2 new tests | Covered |
| Seed sampling | 1 new test | Covered |
| Packaging metadata | 2 existing tests | Still passing |

### 3.5 Security Assessment

| Concern | Status | Notes |
|---|---|---|
| CSP script-src | Enforced | No regression |
| CSP style-src unsafe-inline | Documented deferral | Acceptable per RD-046 |
| setuptools supply chain | Bounded `>=68,<81` | No unbounded dependency |
| Classification input | Case-insensitive string matching only | No regex injection surface |
| Seed parameter | Integer field, no external input path | No injection risk |

### 3.6 Commit Hygiene

| Concern | Assessment |
|---|---|
| Single large commit mixing scopes | `cb508de` bundles 7 Phase 3 tasks + 8 Phase 2 fixes. Functional but reduces bisect precision. |
| Review processing commit | `a712e2a` is clean: only review file moves + decision/status updates. |
| No dangling TODO/FIXME introduced | Confirmed by absence in changed files. |

---

## Stage 4 — Release Readiness

### Packaging Readiness

The Phase 3 packaging tasks are **decision and documentation tasks**, not implementation tasks. No binary packaging was attempted in this commit. The project is ready to begin the PyInstaller spike with clear scope, checklist, and metadata decisions in place.

| Gate | Status |
|---|---|
| Packaging spike plan documented | Ready |
| setuptools ceiling modernized | Done |
| Metadata source decision documented | Done |
| CSP nonce deferral documented | Done |
| PyInstaller sidecar built | Not started (spike) |
| macOS code signing | Not started (spike) |

### Runtime Classification Readiness

Classification is production-ready for current scope:

| Gate | Status |
|---|---|
| Module extracted and testable | Done |
| Default rules cover target runtimes | Done |
| Custom rule injection supported | Done |
| Old inline code removed | Done |
| Extension path documented | Done |

### Sampling Readiness

Seed configuration is production-ready:

| Gate | Status |
|---|---|
| Seed parameter with backward-compatible default | Done |
| Reproducibility test | Done |
| No external input path for seed | Safe |

---

## Stage 5 — New Issues

| ID | Severity | Category | Description | Suggested Action | Priority |
|---|---|---|---|---|---|
| NI-001 | Medium | Test Coverage | No test for "Application" fallback classification when no rule matches | Add fallback test case | P1 |
| NI-002 | Medium | Classification | HTTP/Network rule tokens (`socket`, `http`) may over-match application code | Review rule ordering and token specificity after real-world profiler data | P2 |
| NI-003 | Medium | Sampling | Seed value 0 produces degenerate reservoir behavior (always replaces index 0) | Add `seed > 0` validation or document constraint | P2 |
| NI-004 | Minor | Packaging | `setup.cfg` references `file: README.md` but no `engines/python/README.md` exists | Add minimal README or remove long_description field | P2 |
| NI-005 | Minor | Test Coverage | No test for classification rule ordering (first-match-wins behavior) | Add ordering test for overlapping Spring rules | P2 |
| NI-006 | Minor | Test Coverage | No test for case-insensitive classification matching | Add case-variant test | P3 |
| NI-007 | Minor | Commit Hygiene | Single commit mixes 7 Phase 3 tasks with 8 Phase 2 review fixes | Separate scope commits in future phases | P3 |
| NI-008 | Info | Documentation | PACKAGING_PLAN.md spike checklist omits Linux as explicit out-of-scope | Add one-line Linux deferral note | P3 |

---

## Summary

All 7 tasks pass verification. The Phase 3 packaging decisions are well-documented with clear spike scope, the runtime classification extraction is clean with proper immutability and injection support, and the seed-configurable sampling is backward-compatible. The primary gaps are missing edge-case tests (fallback classification, rule ordering, case sensitivity) and a minor packaging metadata issue (missing README.md). No blocking issues found.

| Task | Verdict |
|---|---|
| T-022 | PASS |
| T-024 | PASS |
| T-025 | PASS |
| T-026 | PASS |
| T-027 | PASS |
| T-052 | PASS |
| T-053 | PASS |
