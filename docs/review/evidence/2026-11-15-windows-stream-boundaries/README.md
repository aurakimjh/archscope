# Windows encoding, binary caps and streaming follow-up — 2026-11-15

Baseline3f55342b. T-613PENDING/T-627REVIEW and original independent CONDITIONAL
remain unchanged. Selected synthetic fixtures only; no reference parity claim.
Private profiles/controllers/raw sessions stay local. Seven preexisting Docker
containers and other user applications are preserved.

## Builder encoding envelope

Before-fix regression fails identity, short mixed-case identity, x-gzip alias and
repeated gzip+br headers: first three unreadable, last falsely decoded as gzip.
Fix honors identity/x-gzip and evaluates all header values as a coding stack.
Unsupported stacks remain encoded with decoded unknown and no text/image/legacy.
Default and explicit negotiation are both covered. Disable transparent net/http
inflation and explicitly retain its normal gzip negotiation (except HEAD/Range)
so original headers and known Content-Length survive before decode. Default gzip
now retains its known37B envelope/13B decoded in the prior regression; previous
unknown-wire receipts stay historical. Complete requestbuilder suites pass
Windows amd642.557s/3862.592s. Native/Docker follow-up is recorded separately below.

## Capped binary capture

Before-fix recorder+summary regression fails raw8192B and gzip47B→8192B at128B
preview cap: binary early return hides truncation and falsely includes partial
sizes in compression. Keep binary text absent but apply truncated classification
for wire or decoded cap. Exact128B binary remains binary/eligible; capped raw and
compressed binary are truncated/excluded. This is synthetic internal recorder
coverage, not a live-body SEC10 preflight pass. Full Windows proxy+summary suites
pass amd641.450s/0.422s and3861.474s/0.431s.
