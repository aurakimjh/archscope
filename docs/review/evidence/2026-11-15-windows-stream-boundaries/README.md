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

## Live proxy encoding and unknown sizes

Actual Windows386 SEC10-verified body capture confirms binary raw/gzip caps at
4096B but exposes repeated gzip+br falsely inline/decoded27, compression-40B/3known
instead of encoded/unknown and-20B/2known. live-proxy-before.json preserves it.
New failing request/response regressions reproduce the same stack interpretation;
a transport regression shows the proxy inventing Accept-Encoding:gzip and
inflating an unsolicited40B gzip into16B while stripping the coding header.
Disable proxy transport auto-negotiation/inflation and interpret every coding
header. Separate failing tests expose decoded/transfer zero for unavailable
bodies: missing/omitted/unsupported/corrupt sizes now unknown; observed empty is0.
Complete proxy/requestbuilder/summary suites pass Windowsx64/x86 after correction.
Actual gate/current runtime and native after-fix receipts remain separately pinned.

## Shared COM test registration

Broad capture suites ran concurrently across architectures and exposed a shared
TypeLib cleanup race in the real remoting fixture (x64 FAIL preserving foreign
owner). Add a named Windows mutex around preflight/register/owned-cleanup while
the fixture holds its OS thread. This changes tests only, not product registration.
Repeat concurrently three times each:386 remoting3PASS;amd64 protects preexisting
foreign Interface mapping with3SKIP. A protected skip is not a remoting acceptance
PASS. No existing registration is deleted or overwritten to force qualification;
full clean-host/client compatibility remains open. Initial logs are preserved.
