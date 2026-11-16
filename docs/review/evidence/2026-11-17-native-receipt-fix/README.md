# WREV3-01 native receipt remediation — 2026-11-17

The historical runner read an existing but incomplete JSON file once and failed
with SyntaxError before the producer completed it. The pinned original-source
reproduction and received Windows results remain in their original directories.

The layoutaudit-only producer now reuses the existing `analysis.AtomicExport`:
same-directory temporary file, owner-only mode, write, sync, close, then rename.
The consumer also supports older producers by retrying missing/partial JSON
while its owned child is alive, within the original 310-second deadline.
Well-formed invalid schemas and filesystem errors fail immediately. Valid
receipts still pass the wrapper's original zoom, passed flag, locale, theme,
geometry and pixel-ratio assertions; explicitly failed reports stay failed.

[Machine-readable validation](local.json) binds the changed sources and commands.
Mac and isolated Docker each pass 23 cases: 15 reader cases plus eight executions
of the current actual runner with controlled process/filesystem I/O. Partial and
missing files can complete; permanent invalid JSON, early exit, explicit failure,
wrong zoom and wrong locales remain failures. A real temporary-file case also
covers partial-to-complete publication. Docker provides a bounded writable
`/tmp` for that case; the repository and container root remain read-only and the
network is disabled.

Two actual Go publication tests pass 20 repeats with the race detector on Mac.
They check final byte completeness, replacement, permissions where supported,
failed publication and temporary-file cleanup. Both Windows test binaries
cross-compile through the existing guarded SDK runner. Offline assets also pass.
CI is extended for the Node cases and tagged publisher tests, including native
Windows amd64 and guarded 386; it has not run for this unpublished commit.

This is LOCALLY_VERIFIED, not a new native Windows WebView2 receipt or independent
acceptance. Execute the changed tagged app and runner on supported native Windows
x64/x86 before closing that qualification. Windows rename/filesystem behavior is
not established by cross-compilation. T-663 remains REVIEW. Ordinary production
builds exclude layoutaudit, and no AnalysisResult contract, memory budget,
version, original raw evidence or historical review verdict is changed.
