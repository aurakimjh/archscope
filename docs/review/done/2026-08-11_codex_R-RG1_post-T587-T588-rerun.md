# R-RG1 — Post-T-587/T-588 Integrated Release Re-Run

- **Date:** 2026-08-11
- **Reviewer:** Codex
- **Scope:** integrated acceptance after the HTTP Analyzer surface changes and
  SEC-10 body-tier activation
- **Verdict:** **PASS**
- **Product HEAD:** `7af0f7a`

## Evidence

| Gate | Run | Result |
|---|---:|---|
| Engine Native | `31488911063` | **PASS** — all five jobs |
| General CI | `31488911030` | **PASS** |
| Windows GUI Smoke | `31489687652` | **PASS** |

The Engine Native Windows job passed the Go race suite, native live-capture
E2E, resolver-cost check, SEC-10 policy preparation, forced fail-fast crash,
policy restoration, and evidence upload. The other jobs covered Linux and
macOS Go gates, frontend verification, and macOS packaging.

The Windows GUI workflow built a 19,003,904-byte executable from `7af0f7a`,
launched it, observed it alive for the full 15-second smoke interval, and then
shut it down cleanly. This satisfies the native desktop launch requirement for
the post-T-587/T-588 product surface.

## Security and review inputs

- T-588 H-SEC2 activated-tier review: **PASS after remediation**
- T-587 UI/engine review: **PASS after remediation**
- SEC-10 crash evidence: zero new or modified dump artifacts and zero body or
  CA-key canary matches in scanned artifacts or the Application log

Commits after `7af0f7a` only archive evidence and review/status documentation;
they do not change buildable product source. The tested product therefore
matches the product source being accepted by this re-run. No tag or GitHub
release was created.
