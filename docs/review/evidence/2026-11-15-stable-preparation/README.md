# Stable v0.4.0 preparation — Mac and Docker

2026-11-15, baseline `70db13ba`. This is local release preparation, not a stable
release, a new independent PASS, or Windows native acceptance. The production
version remains `0.4.0-rc.1`; the existing release tag and trusted bootstrap bytes
are preserved. [Verification summary and source/log digests](verification-summary.json).

## Pack compatibility and upgrade gate

[Mac matrix](packs-mac.json) and [network-none Linux/arm64 matrix](packs-docker.json)
execute opt-in releasequalification tests of the actual validator/store with Go source overlays for RC.1, stable
0.4.0 and stable 0.4.1. Production AppVersion is never rewritten. The original
signed pack accepts RC.1 and rejects stable 0.4.0. The proposed 1.0.1 pack keeps
all original payloads, keeps the lower bound, and changes the upper bound to
`before 0.4.1`. It accepts RC.1/stable 0.4.0 and rejects stable 0.4.1. Bounds use
the existing SemVer semantics; this is not qualification of future prereleases.

Ephemeral in-memory test keys sign the candidate only for synthetic owned stores.
They are explicitly registered through the owner API and are rejected by the
unchanged production trust set. Tampered manifest signatures are rejected.
Install/activate/restart and a reversible RC-side activation pass. Private test
keys are never persisted. [Candidate manifest](manifest-candidate.UNSIGNED.json)
is **UNSIGNED and not installable release evidence**; its original signer still
needs an authorized signature. It must not be paired with the old signature.

Upgrade qualification exposes a release gate: an RC store with the old active
pack is incompatible under stable 0.4.0. A store explicitly prepared on RC with
the compatible candidate reopens correctly. Under stable, rollback to the
incompatible old revision is rejected without changing state. These checks use
the real manager, not a newly signed embedded production bundle or installer.

Before the stable cut, obtain the existing authorized signing procedure, sign
and qualify the actual pack, replace the embedded bootstrap, and verify fresh
`OpenDefault` startup plus the actual RC-to-stable installation path. Existing
owner-selected active state must not be silently overwritten. An explicit
compatible-pack transition on RC is a tested route; unattended migration,
recovery and installer behavior still require implementation/qualification.
The current old bundle cannot be made stable-ready just by changing AppVersion.

## Analysis and Workspace integration

[Mac](workspace-mac.json) and [Docker](workspace-docker.json) run the actual
compiled renderer Workspace/persistence/export modules against actual Wails
service methods over an authenticated loopback test transport. Synthetic
profile-set, server-log and normalized synthetic jstack-thread evidence are
imported into three durable cases. Each case is saved idempotently, reopened
with a fresh backend/cache and freshly opened case store, then exported as
JSON/HTML/CSV: nine exports per environment. Exact JSON including the integer
9007199254740993 is preserved. Display summaries, opaque backend export routing,
unsupported PPTX refusal, independent entry removal, lease release and durable
case survival after Workspace clear are checked. Existing full app regressions
also cover source query, verified baseline, search and evidence links.

Mac uses the native backend with race instrumentation. Docker executes a
cross-compiled Linux/arm64 **purego headless test variant** in Node22 bookworm;
it does not qualify production Linux CGO/GTK/WebView startup or native UI. A
plain CGO-disabled Wails build correctly hit its build guard; an Alpine attempt
could not load the glibc-linked purego binary. The final glibc container passes.
Current macOS SDK linker warnings do not establish minimum-macOS runtime coverage.

## Retained-memory attribution

[Model parity](model-parity.json): native Mac and Docker backends independently
produce 219 sanitized file-open projections. Every full transaction table and
its production fingerprint match across the two environments. Models and the
original synthetic session files stay in owned temporary output, not Git.

The actual compiled Workspace module is measured in a separate process for each
scenario: 36 cycles, three opens/cycle, 28 warmup and eight measured cycles.
Each process retains at most 80 cards. Actual model JSON is parsed on every open;
all result values are compared before/after Workspace admission. Diagnostic GC
is explicit, with a Node heap cap of 768MiB; Docker also has a 1GiB memory limit.
The script declares post-clear retained-heap delta <16MiB and saturated heap
slope <4MiB/cycle before execution. All eight runs pass both limits.

| Environment | Scenario | Distinct arrays | Unique rows | Final JS heap MiB | Post-clear heap delta MiB |
|---|---|---:|---:|---:|---:|
| mac | repeated | 3 | 4,640 | 11.19 | 0.29 |
| mac | unique | 80 | 40,960 | 45.36 | 0.29 |
| mac | body-rich | 80 | 10,240 | 47.11 | 0.29 |
| mac | unshared-control | 80 | 121,184 | 92.97 | 0.25 |
| docker | repeated | 3 | 4,640 | 10.75 | -0.61 |
| docker | unique | 80 | 40,960 | 45.76 | 0.24 |
| docker | body-rich | 80 | 10,240 | 47.52 | 0.24 |
| docker | unshared-control | 80 | 121,184 | 93.38 | 0.18 |

Repeated inputs retain 121,184 logical rows in 80 independent history cards but
share only three immutable arrays/4,640 rows. The unshared control uses the same
current module with sharing explicitly omitted; it retains 80 arrays/121,184
rows. This is a matched ownership control, not a rerun of the historical binary.
Unique and body-rich inputs correctly retain 80 distinct arrays, then release
registry entries on clear. Full result values and caller mutability are preserved.
This confirms the known duplicate-table retention mechanism and bounds these
synthetic retained objects. It does not establish ordinary native memory
plateau, universal leak freedom, actual Windows working-set/private-memory
attribution, or an explanation of all historical late growth. The old failed
receipts and the unexplained OS-private/JS-heap gap remain unchanged.

Each scenario's complete numeric samples are in `memory-{mac,docker}-*.json`.
Explicit GC and synthetic inputs are **not T-662 natural pressure evidence**.
T-654/T-662 remain TODO. No stress-to-swap qualification or native Windows run
was performed in this preparation.

## Regression and remaining acceptance

Full targeted `go test -race` passes packs, cases and the app; the final packs
regression and the tagged renderer/backend integration race test pass. Frontend
state, tagged vet, script syntax, canonical version and bundled resource checks
pass. No product parser/analyzer/exporter contract or runtime implementation was
changed. [Remaining acceptance](remaining-acceptance.json) orders the remaining
Windows work, pack signing/migration and independent review gates. T-601 REVIEW,
T-627 REVIEW, T-611 REVIEW, T-613 PENDING, T-654 TODO and T-662 TODO are retained.

## Reproduction

Run from the repository root with Go, Node and Docker available. Use only a
new owned output folder; pack qualification refuses an existing output path.

```sh
PREP_OUT="$(mktemp -d /tmp/archscope-stable-recheck.XXXXXX)"
npm --prefix apps/engine-native/cmd/archscope-app/frontend run test:state
python3 scripts/qualify-stable-packs.py --out "$PREP_OUT/packs-mac"
python3 scripts/qualify-stable-packs.py --docker --out "$PREP_OUT/packs-docker"
ARCHSCOPE_STABLE_PREPARATION_OUT="$PREP_OUT" go -C apps/engine-native test -race -tags releasequalification ./cmd/archscope-app -run '^TestStablePreparation(WorkspaceRoundTrip|ImportModels)$' -count=1 -v
node --max-old-space-size=768 --expose-gc scripts/diagnose-workspace-retention.mjs apps/engine-native/cmd/archscope-app/frontend/.tmp/state-tests/state/analysisWorkspace.js "$PREP_OUT/models" repeated "$PREP_OUT/memory-repeated.json"
```

Repeat the memory command for unique, body-rich and unshared-control in separate
Node processes. For Docker backend integration, cross-compile the test with
`GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -c -tags releasequalification,purego`
inside the engine module, then execute it in node:22-bookworm-slim with the repo
read-only at `/repo`, a writable owned result mount, tmpfs `/tmp`, network none,
working directory `/repo/apps/engine-native/cmd/archscope-app` and
`ARCHSCOPE_STABLE_PREPARATION_OUT` pointing at the result mount. For retained-object
probes, node:22-alpine uses the independently generated Docker models, the same
read-only source, and memory/CPU limits described above. Container image IDs and
compiled source digests are retained in verification-summary.json.
