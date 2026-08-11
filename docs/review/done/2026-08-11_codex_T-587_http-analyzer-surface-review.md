# T-587 — HTTP Analyzer UI/Engine Review

- **Date:** 2026-08-11
- **Reviewer:** Codex, post-remediation UI/engine review
- **Scope:** live timing, transaction budgeting/filtering, contract-v3 body
  state, packet grid, and reconstructed protocol detail
- **Verdict:** **PASS after remediation**
- **Product commit reviewed:** `7af0f7a`

## Blocking findings and closure

1. **Timing phase accounting:** `GotConnInfo.IdleTime` was incorrectly treated
   as the wait between connection acquisition and reuse. That value describes
   how long a reused connection had already been idle. The analyzer now records
   the blocked acquisition interval from `GetConn` to `GotConn`, with a focused
   regression test.
2. **Malformed contract fail-closed behavior:** a schema-v3 contract whose
   version/type did not match, or whose available flag contradicted its gate
   and unavailable reason, could disclose an inconsistent verified state. The
   renderer now rejects the mismatch and contradiction, reports the tier as
   unavailable, and preserves an explicit reason.
3. **Resource-kind filtering:** filter choices were derived from the already
   filtered transaction set, so selecting one kind could make the other valid
   choices disappear. Options are now derived from the complete transaction
   set while the table continues to apply the selected filter.

## Rechecked surface

- The transaction row budget is independent of aggregate top-N tables.
- The raw tab identifies its content as reconstructed protocol text rather
  than captured wire bytes.
- Body storage and truncation labels match the engine contract and fail closed
  when the contract cannot be adopted.
- Proxy-upstream and proxy-internal timing perspectives retain distinct,
  bounded meanings.
- Resource classification and the packet-grid/detail-pane interaction remain
  stable after the filter fix.

## Verification

Local Windows verification passed `go test ./...`, `go vet ./...`, engine/app
builds, frontend state tests, TypeScript checking, and the production frontend
build. Engine Native run `31488911063` passed all five jobs, including the
Windows Go race suite and native live-capture E2E. General CI run
`31488911030` passed as well. No blocking T-587 UI/engine finding remains.
