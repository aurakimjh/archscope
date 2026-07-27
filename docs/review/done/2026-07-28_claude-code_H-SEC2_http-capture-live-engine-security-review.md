# H-SEC2 — Live-Capture Engine Security Review (T-580 / H-RG3)

- **Review group:** H-SEC2 (individual gate — CA/TLS/privilege boundary)
- **Target task:** T-580 / H-RG3 live-capture engine foundation
- **Reviewer:** claude-code (independent)
- **Date:** 2026-07-28
- **Verdict:** `PASS` (scoped to the H-RG3 engine foundation; two forward-looking
  conditions bind the H-RG4 live-UI handoff and any future body-capture tier)
- **Evidence base:** full read of
  `apps/engine-native/internal/capture/**` (certstore, proxy, session, store,
  stream, procmap, redact, aggregate, types) and
  `apps/engine-native/cmd/archscope-app/captureservice.go` / `main.go`;
  `go test ./internal/capture/...` and `go test ./cmd/archscope-app/...` pass;
  `go vet ./internal/capture/... ./cmd/archscope-app/...` clean;
  `go build ./...` succeeds.

## Scope

This gate validates the production CA/TLS/privilege boundary of the live MITM
capture engine against the applicable `SEC-1`–`SEC-15` cases from
`docs/en/SYSTEM_HTTP_CAPTURE.md` §11, plus the `SEC-16`/`SEC-17` no-headless and
scope-minimization contracts that H-SEC1 deferred to implementation time. The
offline HAR import path (`SEC-4`/`SEC-5`) was already accepted under H-RG1/H-SEC1
and is out of scope here.

## Verdict summary

The engine foundation's security-critical boundaries all hold in code and are
covered by tests. The MITM proxy structurally never persists plaintext HTTP
bodies (every transaction is written with `BodyStorage: "omitted"` and an empty
`BodyPreview`), the CA private key is memory-only and never exported, upstream
TLS is always verified against the host trust store, the proxy binds only to
loopback, and there is no CLI/headless/daemon path that can start a capture. The
two residual items — a crash-dump-exclusion preflight (`SEC-10`) and an explicit
opt-in gate for retaining unknown-attribution transactions (`SEC-17`) — protect
against exposure that this slice does not yet create, so they are recorded as
binding conditions for the next tier rather than blocking findings.

## SEC case dispositions

| Case | Area | Disposition | Evidence |
|---|---|---|---|
| SEC-1 | No reusable credentials in session files | PASS | `stream.Pipeline.Submit` calls `redact()` before `Store.Append`; `RedactHeaders` fully redacts `Authorization`/`Proxy-Authorization`/`Cookie`/`Set-Cookie`/`X-Api-Key`/`X-Auth-Token`; `applyText` strips Bearer/JWT/AWS keys. |
| SEC-2 | Query-string token / POST-body password | PASS | `RedactURL` drops user-info and redacts sensitive query values; `pipeline.redact` additionally sets `tx.Query = ""`. Proxy stores no request/response body (`BodyStorage: "omitted"`), so POST bodies never reach disk. |
| SEC-3 | No `commandLine`/`user` after export | PASS | `RedactProcess` clears `User`, basenames `ExecPath` to `.../<base>`, and redacts `CommandLine`. |
| SEC-6 | Redaction on the import path | PASS | Same `redact.Policy` drives both live and HAR paths; regression coverage in `redact_test.go`. |
| SEC-7 | Catastrophic-backtracking custom rule | PASS | Custom rules compile under RE2; `applyCustom` enforces a per-rule time budget and disables the rule with `HAR_REDACTION_RULE_DISABLED` on overrun. |
| SEC-8 | CA key location/permission/export | PASS | `proxy.Authority` generates and holds the CA key in memory only; only `CertificateDER()` (public) is exposed. `CaptureService` has no key-export method. `Authority.Close` zeroizes the DER buffer. The persisted `ca-trust.json` stores the **public** certificate only. |
| SEC-9 | Two machines produce different CAs | PASS | `NewAuthority` generates a fresh P-256 key + random 128-bit serial per process; no CA is bundled. |
| SEC-10 | Crash-dump policy before key load | CONDITIONAL → deferred | No dump-exclusion preflight exists in this slice, but the exposure it guards is absent: no plaintext bodies are ever stored, and the only long-lived secret (CA private key) is memory-only and zeroized on `Close`. Binding condition C1 below. |
| SEC-11 | Cross-account access to the session dir | PASS | Session root and per-session dirs are `0o700`; `transactions.ndjson`, index, manifest, blobs, and `ca-trust.json` are `0o600`. Session IDs are path-traversal-validated in `store.New`, `validSessionID`, and `Store.Body`. |
| SEC-12 | Removal from every mutated store | PASS | `certstore.Lifecycle` records each successfully mutated store, installs transactionally with reverse-order rollback on failure, removes in reverse order, and reports partial-removal state as `partial`. The persisted record enables post-crash removal; `Remove` deletes it. |
| SEC-13 | Pinned app — no auto-bypass | PASS | A failed client TLS handshake yields `Fidelity: "unsupported"` with a diagnostic instructing explicit scoped passthrough; there is no global auto-disable. Passthrough requires an explicit host allowlist with a bounded TTL. |
| SEC-14 | Upstream cert failure | PASS | Upstream `RoundTrip` failure produces a `TxFailed` transaction (`failureTransaction`) and a 502 to the client; there is no path that disables verification globally. |
| SEC-15 | Unauthorized process on helper IPC | N/A (this config) | Capture runs in-process at `PrivilegeNone` on a loopback proxy; there is no privilege-separated helper or IPC surface in this slice. `CAP-6` helper lifetime/privilege/IPC is explicitly deferred by the plan. |
| SEC-16 | CLI/noninteractive/detached capture start | PASS | The only capture-start surface is the Wails `CaptureService`, registered solely in the GUI `main.go`. The `archscope-engine` CLI exposes only offline `http-capture analyze`; no capture-start/daemon command exists. |
| SEC-17 | Out-of-scope / unknown-attribution traffic | CONDITIONAL → deferred | The proxy only observes traffic clients explicitly route through the loopback listener, and no plaintext body is ever stored. Unknown-attribution transactions are still persisted (as redacted metadata) and counted via `unattributed` rather than dropped behind an explicit opt-in. Binding condition C2 below. |

## Additional verified controls

- **Upstream TLS verify-always.** `server.transport()` sets `MinVersion: TLS 1.2`
  and `RootCAs: cfg.UpstreamRoots`. In production `UpstreamRoots` is unset, so Go
  falls back to the host root store — verification stays on. `InsecureSkipVerify`
  appears only in `server_test.go` for a self-signed fixture origin, never in
  product code.
- **Loopback-only bind.** `validateListenAddress` rejects any non-loopback host;
  `intercept`'s TLS server and the plain-HTTP path both run behind that guard.
- **Renderer cannot choose the store destination.** `StartCapture` forces
  `config.StoreRoot = ""`, so capture data always stays under the app-owned root.
- **Bounded passthrough scope.** Passthrough TTL defaults to 15 min and is capped
  at 24 h; approval is checked against `Now().Before(expires)` per connection.
- **Leaf key is memory-only.** A single leaf key is generated per `Authority`,
  reused for on-the-fly leaf certs, never persisted — consistent with the H-SEC1
  P3-1 memory-only-leaf disposition.
- **Store integrity.** Append-only NDJSON with rebuildable offset index,
  truncated-tail recovery, disk-reserve and session-size guards, and
  session-bound opaque cursors (`ErrStaleCursor`).

## Binding conditions (do not reopen this gate; gate the next tier)

- **C1 — SEC-10 dump-exclusion preflight.** Before any tier enables request/
  response **body** capture (inline or blob), implement and measure the
  crash-dump-exclusion preflight that fails closed when dump exclusion cannot be
  guaranteed. Not required while `BodyStorage` is unconditionally `"omitted"` as
  in this slice.
- **C2 — SEC-17 explicit unknown-attribution retention.** Before the H-RG4 live
  UI exposes stored transactions, gate retention of unknown-attribution
  transactions behind an explicit metadata-only opt-in (drop by default), or
  document in the paired contract that redacted-metadata retention of
  loopback-scoped traffic is the intended, disclosed behavior.

## Tests / build evidence

- `go test ./internal/capture/...` — all packages `ok` (aggregate, certstore,
  procmap, proxy, redact, session, store, stream).
- `go test ./cmd/archscope-app/...` — `ok` (CaptureService).
- `go vet ./internal/capture/... ./cmd/archscope-app/...` — clean.
- `go build ./...` — succeeds (only macOS SDK linker version warnings).

## Conclusion

H-SEC2 returns `PASS` for the T-580 / H-RG3 live-capture engine foundation. The
CA lifecycle, upstream verification, loopback confinement, redaction-before-
persistence, cross-user file protection, and no-headless-start guarantees are
implemented and tested. C1 and C2 are forward-looking conditions on the next
tier and do not block this gate. T-581 / H-RG4 is unblocked.
