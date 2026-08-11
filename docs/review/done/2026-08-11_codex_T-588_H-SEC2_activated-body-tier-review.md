# H-SEC2 — Activated Body-Tier Security Review (T-588)

- **Date:** 2026-08-11
- **Reviewer:** Codex, post-remediation security review
- **Scope:** SEC-10 Windows dump-policy preflight, body-enabled engine entry
  points, contract-v3 renderer gate, and forced-crash evidence
- **Verdict:** **PASS after remediation**
- **Product commit reviewed:** `7af0f7a`

## Result

The activated body tier remains fail closed unless the Windows SEC-10
preflight verifies the effective WER policy. Both engine entry points run the
preflight before authority generation or body retention, and the renderer
rejects absent, malformed, or contradictory contract state. No blocking
H-SEC2 finding remains.

The original verification was not strong enough: it crashed on a generic Go
panic while holding synthetic strings, considered only newly created files,
and did not scan the Application event log. The remediation now sends unique
canaries through real proxy request and response body recorders, holds the
live authority certificate and private key, calls `RaiseFailFastException`,
checks both new and modified artifacts, scans the Application log, and emits a
SHA-256 sidecar. CI also renames and preserves pre-existing machine and
per-executable `LocalDumps` values, installs the exact SEC-10 policy for the
probe, and restores the original policy in an `always()` step.

## Windows evidence

- Engine Native run: `31488911063` — **success**
- Windows job: `93770549911`
- Probe exit: `0xc0000602` (`RaiseFailFastException`)
- Request/response body recorders populated: yes
- Live CA certificate/private key held: yes
- New or modified crash artifacts: `0`
- Body-canary and CA-key matches in artifacts: `0`
- Application-log matches: `0`
- Archived evidence:
  `docs/review/evidence/2026-08-11_t588_sec10-dump-exclusion.json`
- Repository LF SHA-256:
  `959204cbdc08010bda65bec2584f10eed28e8bd4d5f3de9423e0f768ae901503`
- Original Windows CRLF SHA-256:
  `31748efb5a4e87cbf7cd907cecefc44f2c6b753e60c9032f61a62df6957267ed`

The repository digest matches its checked-in sidecar. Expanding the archived
LF bytes to Windows CRLF reproduces the runner digest, preserving provenance
without pretending the two byte representations are identical.

## Residual scope

SEC-10 does not claim to defend against:

- an active debugger reading process memory;
- an administrator enabling `LocalDumps` during the session;
- kernel or hypervisor memory acquisition; or
- plaintext reaching the pagefile or a hibernation image.

These are explicit scope boundaries, not unverified pass conditions. T-588 is
accepted for the stated Windows crash-dump exclusion threat model.
