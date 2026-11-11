# Data packs and offline distribution

T-632 introduces a bounded JSONL metric snapshot interpreter, not executable plugins.
The synthetic bootstrap pack qualifies only its literal producer `synthetic.metrics`
version `1.0.0`. It makes no vendor, time-series, converter or real workload claim.

## Pack ownership and activation

Use Settings → Data packs or `archscope-engine packs --help`. Installation validates
an exact-byte detached Ed25519 signature against application/owner public trust,
closed schemas, all declared file hashes and positive/negative fixture outputs.
Keys inside a ZIP never establish trust. An owner may register a public key;
management keys sign sequential offline rotation/revocation documents. Keep private
keys outside the application. The bundled bootstrap private key is not distributed.
A revoked key cannot activate or roll back a pack. Removing trust may make new
imports unavailable until the owner deactivates/replaces the affected pack.

Install stages data without activating it. Activation requires the current state
revision; errors leave the previous active state. Retained versions allow rollback
through the same signature, compatibility and conformance gates. Interrupted
staging is removed on reopen. State uses sync then atomic rename under a kernel
lock; process-restart recovery is tested, not an assurance about hardware power loss.
Dependencies require a pinned digest and version range. Application, interpreter
and extension ranges are checked. The application version is currently 0.3.5.

Unsigned packs require an explicit `--unsigned-local` plus `--scope user` or
`case:case-<32 lowercase hex digits>`. They are labelled `locally_authored` and
never enter application scope. Import requests must opt into that same scope.
Alias collisions require `--choices '{"alias":"canonical.mapping-id"}'`;
there is no automatic local-precedence policy. Canonical IDs cannot be shadowed.
Running jobs retain an immutable catalog and pack digests. Saved results retain
that provenance; later activation does not rewrite results.

Mappings accept literal producer/version guards, JSON pointer paths, ordered field
aliases, exact positive decimal scaling, declared units and gauge/counter kinds.
Executable code, dynamic expressions, regex, shell, SQL and templates are rejected.
Budgets: archive 8 MiB, expanded 16 MiB, entry 4 MiB, 128 ZIP entries, manifest
256 KiB, 64 retained versions, 32 MiB active snapshot, 10,000 conformance records,
64 KiB JSONL rows and the existing job/record/result budgets. Decimal strings
preserve fractional, zero and large values. This emits metrics_snapshot 0.1.1;
legacy metrics_snapshot 0.1.0 interpretation remains unchanged.

## Build and use a disconnected bundle

`go run ./cmd/archscope-engine offline build --engine /local/archscope-engine --out /new/bundle`
creates a new, self-contained CLI directory with a hashed inventory, release
fixture ZIP, local manuals, font/license, schemas and bootstrap pack. `--app`
optionally includes the same-platform desktop executable. A failed build removes
its new directory. No sibling checkout is read at runtime. The shipped fixture
ZIP is mandatory, pinned in the binary and covers every registered importer;
missing files, altered hashes or uncovered importers fail the release gate.
Run `offline fixtures` to execute standalone importer conformance.

Run the packaged executable with `--offline-root /bundle --offline-report /local/audit.json`.
Preflight verifies resources and binaries before any engine operation or WebView
creation; a package built for another OS/architecture is refused. The package
inventory detects accidental changes; authenticity relies on the distribution's
signature/trusted delivery, not a self-authenticating checksum manifest.
Use `ARCHSCOPE_STATE_ROOT=/new/local/state` for isolated clean-install validation.
Do not put writable state inside the read-only bundle.

Offline sessions deny capture, HTTP request builder, Ollama, automation control and
unqualified JFR subprocess conversion before allocation, recording operation counts
without endpoints or payloads. Import preconverted JFR JSON instead; converter
qualification is T-633. The UI applies a same-origin CSP and records violations.
Fonts and charts are local; no CDN, updater, telemetry or symbol downloader is
configured. Report app-level denied attempts separately from OS/WebView activity.
A zero blocked-attempt count alone does not prove zero outbound attempts. Release
validation uses syscall observation plus a failed-network positive control for the
CLI; desktop/platform execution needs its own observed evidence.

Windows desktop bundles **require** `--webview-runtime /local/fixed-runtime` and
`--webview-version <version>`. Supply the complete licensed Fixed Version runtime
for the target architecture, including `msedgewebview2.exe`, `msedge.dll`, the target architecture's
`EBWebView/{x86|x64|arm64}/EmbeddedBrowserWebView.dll`, locales,
resources and license. No bootstrapper/download/Evergreen fallback is used. The
builder checks PE architecture and native product version, records all runtime hashes, and the launcher sets
Wails `WebviewBrowserPath` only after preflight. Runtime servicing is the owner's
responsibility; no runtime is downloaded by this tool. Native Windows launch and
browser subprocess outbound observation remain Windows-PC acceptance gates.
See [Microsoft's distribution guidance](https://learn.microsoft.com/microsoft-edge/webview2/concepts/distribution).
macOS uses the OS WKWebView; desktop OS dependencies are not vendored. Linux CLI
needs no WebView. Linux desktop dependency qualification remains separate.

The embedded license inventory includes host compiled Go modules, installed npm
packages (including build tooling), original Inter notices and dependency notices.
Platform/system/Fixed Version runtime licenses need the matching distribution review.

Noto Sans KR is bundled unchanged from the pinned Google Fonts commit in its
font manifest, under OFL-1.1 with original notices. Settings displays font-load
status; a reported fallback must not be treated as font/layout acceptance.

## SQLite decision

The isolated `spikes/t632-sqlite` module pins pure-Go modernc.org/sqlite v1.53.0.
On this macOS arm64 host, 100,000 synthetic primary-key rows took 74.552 ms to
build and 5.934 ms for 1,000 lookups (943,280 allocated bytes); database 1,384,448
bytes, standalone binary 9,964,914 bytes. The stdlib JSONL/in-memory-index baseline
took 64.170 ms / 0.057 ms (5,184 allocated bytes), 2,877,780-byte JSONL and
3,144,994-byte binary. These are one-run warm local measurements, not benchmarks
of analytical queries or Windows runtime. Pure-Go Windows 386 cross-build passes.
Retain JSON/compact indexes in production for this slice; no SQLite production
dependency is added. Revisit with T-635 query/memory/concurrency requirements.
