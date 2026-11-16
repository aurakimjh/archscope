# Windows follow-up validation — 2026-11-16

Baseline98eeaf3e. All existing task/review states remain unchanged. Local Windows
acceptance is scoped separately from independent linguistic/review approval.
The initial Docker engine was stopped; the user started it during this task.

Windows physical Ctrl+comma failed to navigate Settings although Ctrl+F1 worked.
Wails3.1.0 converts VK_OEM_COMMA to oem_comma but does not accept that name in its
accelerator parser. Windows now uses one logical renderer shortcut owner, while
other platforms keep native ownership. Composition, handled input and extra
modifiers are excluded. Actual x64/x86 input reaches Settings after correction.
Before/after screenshots retain the original failure and pinned build labels.

The new ribbon exposed an existing accent-token collision: legacy foreground
indigo was overwritten by Tailwind's pale hover background. Selected text had
1.118:1 contrast. Separate the control-background token and use the appropriate
foreground for ribbon and selected locale/theme pills. Actual native x64/x86
three-size smoke passes180locale/tab combinations and18theme selections, with
selected text contrast>=4.5:1. Observed OSscale1, WebView1/1.25/1.25 and client
width1264/806/707 are recorded; WebView zoom is not a new OS-DPI qualification.
Screenshots verify scoped Korean/Japanese/Chinese glyph rendering, not grammar
or physical IME composition. The initial no-contrast smoke results are separate
from the final stricter receipts and not relabeled as contrast checks.

The Windows CP949 Python default also broke the translation coverage checker.
Explicit UTF-8 reads fix ordinary and optimized execution without changing
catalog content; all2197phrases per added language retain placeholder parity.

Initial full x86 tests failed the progress-batch test: after an assumed30ms,
the publisher had not run. A synctest bubble makes the existing10ms ticker and
30ms check use virtual time; the one-batch/two-item, stable-order and abort-on-
close assertions remain. Both architectures pass25targeted repeats, x64 stream
suite and final x86 full131packages. Initial failure receipts remain preserved.

Seven units now pass actual Windowsamd64 profiler→Chart Studio→native CSV Save
flow: samples/ns/µs/ms/s/bytes/unknown. The real analyzer reads a synthetic owned
collapsed file via a synthetic renderer drop (not physical OS drag acceptance).
Each case creates a Canvas surface, switches to SVG and checks the exact axis;
the operator completes the real Save dialog. Independent Python CSV parsing
checks value1000000, exact unit/template/type and the source-file array for all7
unmodified saved outputs. This does not qualify every PNG/SVG export or x86 chart
condition. Native UIA failed to address modal element indexes; a fresh separate
dialog screenshot and coordinate input completed the operation. Dialog shell
screenshots/trees stay private; only synthetic exported CSVs are published.

Earlier chart harness outcomes remain explicit: Settings lazy-load readiness
failure, an inconclusive run waiting for Summary while the app correctly opened
Flamegraph, and an operator timeout during modal control recovery. The final run
waits for enabled result tabs and permits15minutes per manual Save dialog; the
analyzer readiness and original native memory timing/budgets are unchanged.

Windowsx64/x86 and Linuxamd64 purego/headless Wails-service integration each pass
3saved cases/9exports and fresh backend/cache restart. All219independently produced
sanitized transaction projections/fingerprints match exactly across the3hosts.
Docker repeated/unique/body-rich/unshared-control diagnostics each run36cycles;
history remains80cards, repeated data uses3arrays, other scenarios80, and Clear
releases every entry. Explicit Node GC is diagnostic only. The Linux purego test
variant is not native GTK/UI or production CGO/race acceptance. An initial Docker
CLI flag-splitting failure is separate from the successful array-argument rerun.

Ordinary x86 native memory results are recorded separately when completed;
protocol.json declares their unchanged screening budgets. Original historical
memory failures and every independent review disposition remain unchanged.
