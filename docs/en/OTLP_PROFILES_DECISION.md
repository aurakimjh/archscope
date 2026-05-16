# OTLP Profiles Decision Note

Status: track, do not make active ingestion a release blocker yet.

ArchScope now imports common offline profile artifacts through
`profile_evidence`, including pprof, async-profiler, speedscope, py-spy/rbspy,
StackProf, PHP/Xdebug, Swift/generic async stacks, Pyroscope/Phlare, and
Parca-style snapshots. That covers the customer handoff files we can parse
without running a collector.

Move OTLP Profiles from radar to active ingestion when at least one of these is
true:

- the OpenTelemetry Profiles signal reaches a stable cross-vendor file contract;
- incident evidence commonly arrives as OTLP profile export files rather than
  pprof/speedscope/Pyroscope/Parca snapshots;
- trace-to-profile correlation metadata is consistently present in exported
  OTLP profile samples.

Until then, keep OTLP Profiles mapped as a profile-schema compatibility target:
the unified frame/sample schema already carries runtime, language, native,
managed, async, labels, and correlation fields needed by a future OTLP importer.
