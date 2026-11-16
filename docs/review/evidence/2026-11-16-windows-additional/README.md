# Additional Windows/Docker acceptance — 2026-11-16

Product baseline635512cf. Existing eight open task states and independent
CONDITIONAL review dispositions remain unchanged. Private profiles, controller
credentials and native file-dialog shell listings stay local.

Chart Studio: seven units each on Windowsx64/x86, Canvas→SVG and SVG→PNG through
28real native Save dialogs. The actual profiler uses an owned synthetic collapsed
input via scripted drop; physical drag is not qualified. Independent XML/PNG
readback verifies exact axes/value1000000/bar/white background/2xPNG dimensions.
Seven PNG pairs match byte-for-byte across architectures. Native OSscale1 and
WebViewzoom1 are recorded. Office/reference/full provider/DPI combinations remain.

The initial x64 audit script measured a toolbar icon for the SVG on-screen bounds
metadata. Its original receipt remains unchanged; that field is not trusted.
All actual saved dimensions pass independent decoding. The corrected x86 script
measures data-chart-surface for both renderers and passes all seven cases. The
first x86 Save was deliberately cancelled before the script reload and no PASS
is asserted for that incomplete preliminary run. No product export defect was
observed in this selected matrix.

The memory protocol is declared in memory-protocol.json: ordinary production
Windows binaries, four sequential runs, no forcedGC or trim,36cycles/108opens,
28warmup/8measured,80retained-card capacity,3seconds after each open and15seconds
post-cycle idle. Unique uses108distinct512row files; body-rich uses108distinct
128row files with synthetic inline request/response previews exceeding1KiB each.
These are offline imports, not SEC-10 live x64 body recording qualification.

Windows and network-none, read-only Docker pass12workload/budget tests each.
They verify108distinct row hashes, coherent body lengths/headers, unchanged seed
and byte-compatible historical default generation. Docker generator checks are
separate from ordinary Windows memory acceptance. Used scripts and original
receipts are retained with source/binary hashes.

Completed ordinary x64 screens:

- unique512rows: PASS36cycles/108opens in912.965s. Tree delta-22.230MiB,
  slope0.052MiB/cycle; root/handle/thread budgets also pass. Native512/512rows,
  first selection, checked star and exact last-file Hangul note survive.
- body-rich128rows: FAIL36cycles/108opens in922.338s. Tree delta37.285MiB
  is within64MiB, but slope4.801MiB/cycle exceeds4MiB. Root/handles/threads pass.
  Renderer process attribution is+39.109MiB,5.213MiB/cycle; other roles are nearly
  flat or lower. The actual last request/response previews and annotations survive
  after native activation. Initial background accessibility reads file50, then
  activation reads107; this is a freshness observation, not a causal leak proof.

The same216actual source files produce identical sanitized projections and
fingerprints in Windowsamd64/386 and DockerLinuxamd64purego. The headless checker
initially assumed recursive key ordering; its failure/source are retained, and
the corrected contract-aware readback passes all3hosts. Matched Windows/Docker
Node diagnostics retain80arrays (40960unique rows or10240body-rich rows), then
Clear releases all entries. Explicit GC is diagnostic; it cannot replace the
failed ordinary renderer-private screening verdict. No product allocation patch
is justified by these counters alone.

Ordinary production results (all original budgets):

| Workload | Architecture | Seconds | Tree deltaMiB | Tree slopeMiB/cycle | Verdict |
|---|---|---:|---:|---:|---|
|108distinct512row files|x64|912.965|-22.230|0.052|PASS|
|108distinct128row body-rich files|x64|922.338|37.285|4.801|FAIL|
|108distinct512row files|x86|915.795|-25.445|-0.393|PASS|
|108distinct128row body-rich files|x86|926.549|46.012|8.764|FAIL|

All four final native readbacks preserve selected first request, total row count,
checked star and exact Hangul note. Both body-rich runs preserve the final request
and response previews. Controller-functional success is distinct from initial
background accessibility freshness and memory acceptance. Raw historical/current
FAIL reports stay unchanged. The real owned-app capability check still refuses
x64livebody recording with sec10_unavailable/local_dumps_collecting; transparent
capture lacks the official WinDivert DLL. No security/dump/driver policy changes.

Reproduction uses the exact synthetic memory-seed.httpsession (SHA256 matches
sourceFixtureSha256 in all reports). Build ordinary binaries from635512cf with
VITE_MEMORY_DIAGNOSTICS=0 and production tags through build/windows/go.mjs. From
the repository root, use a new output directory for every run:

```powershell
node scripts/verify-windows-import-memory.mjs --exe <owned-production.exe> --out <new-private-dir> --fixture docs/review/evidence/2026-11-16-windows-additional/memory-seed.httpsession --saturate-history true --scenario unique
# Use --scenario body-rich for the128row inline-body workload.
python scripts/acceptance/verify_import_memory.py --private .tmp/windows-acceptance-20261116b --public docs/review/evidence/2026-11-16-windows-additional --arch amd64 --scenario unique
```

The independent verifier deliberately exits1 for a faithfully reproduced FAIL;
this is not a reason to replace a receipt. Used harness/generator/sampler/budget
snapshots are retained. Headless projection reproduction uses the provided
qualification test and qualify-workload-used.py with freshly generated owned
fixture paths; releasequalification,purego is not LinuxGTK/CGO/race acceptance.
Exact ordinary native Workspace counting is still limited by accessibility;
explicit diagnostic counters and headless ownership are separately identified.

### Native read-only correlation completed 2026-11-17

A separate production,layoutaudit x64 binary embeds VITE_MEMORY_DIAGNOSTICS=1.
The normal frontend is rebuilt with0 after embedding. A small counter record
every5seconds is capped at4096bytes/5000records; changing heap values are excluded
from accumulated layout views. No payload, additional data owner, forcedGC or
working-set trim is introduced. Used sources, binary SHA and protocol are pinned.

The36cycle/108open diagnostic finishes and independently matches all8post-warmup
process samples to preceding native JS observations (maximumlag5.328s within the
predeclared10s). All retain80cards/80immutable transaction arrays/10240rows. JS
used-heap slope is0.004MiB/cycle (median delta-1.037MiB), while rendererprivate
slope is4.458MiB/cycle. This narrows the investigation beyond increasing workspace
card/row counts; it does not prove a particular native allocator or universal
leak freedom. The instrumented tree screen happens to pass3.755MiB/cycle and is
explicitly not a replacement ordinary acceptance run. Both original ordinary
body-rich FAILs stay unchanged. Further renderer allocation tracing/repetition
and longer ordinary screens remain T-613 PENDING.

All owned test apps/WebViews and labeled containers are stopped; seven preexisting
containers are preserved. Official signer, driver/vendor/original/full UI/IME/DPI
and isolated-fault gates remain in remaining-acceptance.json. No task/review
status, production version, trust policy, release or tag is promoted.
