# H-SEC2 — Body-Tier Independent Security Re-Review (T-587)

- **Review group:** H-SEC2 narrow re-review (body retention / SEC-10)
- **Target task:** T-587 HTTP Analyzer capture-detail rework
- **Reviewer:** Codex (independent of the 2026-07-28 H-SEC2 review)
- **Date:** 2026-08-10
- **Verdict:** `PASS AFTER REMEDIATION`, scoped to an unavailable/dormant body
  tier. This is not approval to enable body capture.
- **Ordering:** completed before any R-RG1 re-run.

## Scope and prior premise

The 2026-07-28 H-SEC2 `PASS` depended on an unconditional property: the MITM
proxy stored no plaintext request or response bodies. Its binding condition C1
required the SEC-10 crash-dump exclusion preflight to run and fail closed before
*any* future body-capture tier. T-587 added a per-session opt-in and recorded
`bodyCaptureGate = "sec10_preflight_pending"`, but did not implement that
preflight.

This review covered the product capability contract, session-start and direct
proxy-construction boundaries, recorder memory/size behavior, pipeline
redaction ordering, durable store policy, renderer fallback, acceptance
fixture, and paired design/plan documentation.

## Findings and disposition

| ID | Priority | Finding | Disposition |
|---|---|---|---|
| B1 | P0 | The pending gate was descriptive only. `CaptureBodies = true` still constructed a proxy and retained plaintext, contradicting H-SEC2 C1's required SEC-10 fail-closed behavior. Operator acknowledgement cannot substitute for a required security preflight. | **Resolved.** `LiveCaptureContract` and its renderer fallback publish `bodyCaptureAvailable = false`. `session.Manager.Start` rejects the request before CA/session allocation, and `proxy.New` independently rejects it so a stale/custom caller cannot bypass the manager. |
| B2 | P1 | Compressed input was bounded at the wire buffer, but decoded output used a separate 8 MiB ceiling. A highly compressible payload could therefore retain a preview larger than the advertised per-message limit and still be labeled inline. | **Resolved.** gzip/deflate expansion now uses the same normalized per-message cap and reports `truncated` when decoded output exceeds it. |
| B3 | P2 | The recorder retained an unredacted wire buffer after canonicalization for as long as the recorder remained reachable. | **Resolved as defense in depth.** The recorder zeroes and resets its captured buffer immediately after applying it. Plaintext still exists briefly in the canonical message before pipeline redaction, so SEC-10 remains mandatory before activation. |

## Verified security state

- Request and response bodies are unconditionally omitted in every startable
  product session.
- A body-enabled request returns
  `ErrBodyCapturePreflightPending` before authority generation, proxy
  construction, or session-file creation.
- The proxy constructor enforces the same rejection independently.
- The renderer fails closed if the live contract is absent or malformed.
- The pending gate remains published as the reason the capability is
  unavailable; ordinary manifests record the actually enforced disabled
  policy.
- The dormant body-processing core remains bounded and tested, but its presence
  is not represented as a shipping capability.

## Validation evidence

- `go test ./...` — pass.
- `go test -race ./internal/capture/proxy ./internal/capture/session
  ./internal/capture/stream` — pass.
- `go vet ./...` and `go build ./...` — pass (macOS SDK linker-version warnings
  only).
- `npm run test:state` and `npm run build` — pass.
- Targeted coverage includes direct-constructor rejection, rejection without
  capturer/authority/session artifacts, body omission, body-core
  decoding/truncation, compressed expansion, raw-buffer clearing, and the T-581
  `bodyCaptureAvailable = false` contract fixture.

## Residual condition

SEC-10 is still unimplemented. To enable body capture later, the change must:

1. implement and measure the Windows crash-dump exclusion preflight before key
   load/plaintext retention;
2. fail closed when the preflight cannot establish the required policy;
3. replace the pending capability state with a verified result bound to the
   running session and manifest; and
4. pass another independent H-SEC2 review before release.

## Conclusion

The body-tier security gate passes only because remediation restores the
unconditional no-plaintext-body property of all startable sessions. R-RG1 may
be re-run for the remaining T-587 surface after the independent UI/engine
review, but that acceptance must not be interpreted as permission to enable the
dormant body tier.
