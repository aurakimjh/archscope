# Windows remaining-task follow-ups - 2026-11-13

The user requested feasible work from T-601/T-627/T-611/T-613/T-654/T-662,
with a commit after each completed item and a single final push. These are
bounded follow-ups; all six overall acceptance tasks retain their open states.

## Completed work

- `be7ddb17` routes all file-dialog consumers through a common cancellation
  adapter. Actual native recheck exposed an additional Wails runtime wrapper;
  `649aa15f` recognizes exactly the three complete Windows wrapper messages as
  well. Other failures propagate unchanged. Regression state tests and production
  build pass. The final actual OpenFile cancellation shows no error banner:
  [native-cancel.jpg](native-cancel.jpg).
- `9953ad1b` localizes the MSA initial tabs and empty panels with the EN/KO catalog.
  Native View > English and Analyze > MSA Timeline show the English labels and
  empty message. Korean was restored afterwards:
  [native-msa-en.jpg](native-msa-en.jpg).
- `ca72da32` reads the installed **V7.6.4.508** EXE/core typelib without executing
  or registering the vendor. Both architectures compare **28 interfaces / 227
  own dual methods**, including slots, DISPIDs, calling conventions, HRESULTs,
  parameter flags/types and referenced IIDs. Two current appended ILogEntry slots
  were missing from the older sample: ConnectionId(Integer), Stage(WideString).
  They now exist; unsupported numeric vendor IDs/stages return explicit errors
  with cleared outputs, verified through actual cross-process custom-IID calls.
  Original opaque IDs/states are not substituted. All native COM package tests
  pass on both architectures. Sixteen installed presets are accepted; the cache
  metadata preset is correctly refused. [ABI receipt](reference-abi.json).
- `6bd12713` uses Docker Desktop Linux/amd64 for the repository's disconnected
  clean-install observer. **70 operations / zero inet syscalls** pass under
  network-none; the failed-connect positive control is detected. Existing user
  and Kubernetes containers were preserved. This supplies Linux CLI isolation
  evidence, not Windows desktop/driver reach: [Docker receipt](docker-offline.json).

## Integrated verification and limits

Both production Windows GUI architectures build. Whole app tests pass natively
on amd64 (38.761s) and 386 (43.865s), and both app/COM vet runs pass. Final state
regressions and frontend production build pass. Current binary identities and
native scope are in [native-final.json](native-final.json). The native386 GUI
also survives15 seconds; the existing smoke then forces its owned process after
the close timeout, so graceful shutdown is not claimed. No local race run is
claimed. Private logs, installed vendor binaries and controller data remain out
of Git.

T-601 requires a verified WinDivert package, elevated Windows execution and a
non-loopback peer; the current token is not elevated and no DLL is configured.
T-611 still lacks foreign-process session-cookie integration and full vendor
operational semantics. T-627 needs its remaining populated/conditional/error,
small-window and OS-DPI matrix; T-613 also requires multi-hour desktop acceptance.
T-654 requires permitted two-GPU Linux CUDA/NCCL execution; this host has one GPU.
T-662 requires real Apple Silicon natural swap/pressure observations. Docker
Desktop cannot substitute for those Windows kernel, GPU-count or Apple gates.
The original independent CONDITIONAL verdicts and all six task states remain.
