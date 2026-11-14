# ArchScope Documentation

This directory contains the English design documents for the current
Go/Wails ArchScope line. Python/FastAPI/browser implementation notes are
historical unless they explicitly reference `archive/`.

User-facing guides:

- [Native App](./NATIVE_APP.md) — build, run, package, and CLI entry points.
- [Desktop layout by feature](./DESKTOP_CONTENT_LAYOUT.md)
- [User Manual](./USER_GUIDE.md) — all 21 screens, analysis purpose/inputs/interpretation, workspace saving/search/baselines, and guided investigations. Start here for product use.
- [Multi-language Thread Dumps](./MULTI_LANGUAGE_THREADS.md) — six-format support matrix and per-language enrichment rules.

Engineering / design references:

- [Architecture](./ARCHITECTURE.md)
- [Local Diagnostics Evolution Design](./LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md) — profile breadth, offline AI/GPU diagnostics, extension contracts, and report-ready images/tables; proposal revised after review.
- [Native App](./NATIVE_APP.md)
- [Roadmap](./ROADMAP.md)
- [Data Model](./DATA_MODEL.md)
- [Importer Support Matrix](./IMPORTER_SUPPORT_MATRIX.md)
- [Parser Design](./PARSER_DESIGN.md)
- [Chart Design](./CHART_DESIGN.md)
- [Chart Studio Design](./CHART_STUDIO_DESIGN.md)
- [Runtime Classification](./RUNTIME_CLASSIFICATION.md)
- [Packaging Plan](./PACKAGING_PLAN.md)
- [Advanced Diagnostics](./ADVANCED_DIAGNOSTICS.md)
- [AI-Assisted Interpretation](./AI_INTERPRETATION.md)
- [OTLP Profiles Decision Note](./OTLP_PROFILES_DECISION.md)
- [Bridge PoC UX Flow](./BRIDGE_POC_UX_FLOW.md) — historical Electron/FastAPI bridge note
- [Report Export Design](./REPORT_EXPORT_DESIGN.md)
- [Demo-Site Data Workflow](./DEMO_SITE_DATA.md)

Korean documents are available in [`../ko`](../ko/README.md).
