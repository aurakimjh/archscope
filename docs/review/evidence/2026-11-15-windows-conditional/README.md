# Windows conditional acceptance follow-up — 2026-11-15

T-613 remains PENDING/T-627 REVIEW; original independent CONDITIONAL unchanged.
This increment records additional synthetic conditions, not full parity closure.
Private profiles/controllers/raw files stay local. Existing seven Docker/user
containers and other applications are preserved.

## Builder form, multipart and mode transition

Native Windows386 at6b3cf283-plus-settings-labels sends URL-encoded ordered
fields x=한글 + & = and x=empty. Independent Docker parser verifies exact35UTF8
bytes. Multipart preserves both text fields plus54binary bytes (including NUL,
CR/LF and high bytes) with exact SHA256; custom ..\renamed-proof.bin is transmitted
as basename renamed-proof.bin. Native file picker is used; no UI injection.

Switching multipart with attached file→form hid file controls but left the text
value disabled. Corrected disable condition is multipart AND file-associated;
form text is editable while the attachment association is retained for returning
to multipart. fbf2ce2c-plus-builder-mode native recheck edits formerly attached
field to converted + 한글 and sends it exactly as form data. Replacement-picker
Escape retains the previous file. Frontend state/build and guarded386build pass.
No ordinary IME or drag claim; Unicode is delivered via native set_value/readback.

builder-server-receipts.json contains independent ordered form/multipart parser
receipts; upload-proof.bin is synthetic. Native before/after cases are separate.

## Response envelopes and unsupported encodings

Native explicit Accept-Encoding:gzip + malformed Docker response exposed loss of
all response headers/content type/20B declared length. New failing regressions
also showed unsupported br bytes that happen to be PNG were decoded as a raster,
with a false decoded-byte count. Preserve the HTTP envelope before decoding;
unsupported or failed gzip payloads have encoded storage, unknown decoded bytes,
no text/raster/legacy preview. Partial gzip checksum errors (automatic and explicit
negotiation) are not presented as verified decoded prefixes.

Meaningful before-fix regression fails both branches, then complete requestbuilder
suites pass Windows386/amd64. Final native7117d654-plus-response-envelope preserves
HTTP200/Content-Length20/type/gzip; Body reports decoded unknown and explicit
encoded-unavailable, not a zero-byte response. Full guarded Windows app tests pass
amd6445.369s/38646.041s. This is Builder response handling; capture backend behavior
and raw-wire fidelity are not relabeled by this result.

## Six response boundaries and small English viewport

Windows386 requestbuilder sends six requests to the owned Docker fixture; the
independent verifier checks the canonical session before native controller-open.
This is backend-send plus native saved-file inspection, not six Builder UI sends.
Native Body views verify gzip46/26B, unsupported br21/unknown, interrupted12/5B,
large1048634/1048576B with cap notice, and complete HTTP50327B. Delay is2003ms in
this sample. Readback and helper source pin the source hash and exact conditions.
English1024x768 at OS100%/WebView125% retains six rows and stacked Body content;
native Up moves from503 to interrupted row. No ten-row/dense-layout claim.
Windows Settings launch and Start did not expose a targetable settings window;
actual OS125% remains unverified. No OS scale setting was changed.

## Compression summary completeness

Go and frontend regressions first fail: interrupted/aborted/in-progress reads and
preview caps create false compression differences. Both summaries now require a
complete transaction and non-truncated/non-encoded body size pair. Independently
observed byte aggregates remain unchanged. Complete503 and real gzip overhead
remain eligible. Known canonical HAR sizes can qualify without retained text.
The mixed regression includes eight rows; expected -20B/2known. The actual six
Docker responses display -20B/3known in native KO1280x800 OS100/WebView100.
Native screenshot and accessibility receipt pin6eb28f4f-plus-summary-pairs.
Full guarded app tests: amd6449.800s,38648.696s. Focused summary/requestbuilder
x64/x86, frontend state/build and guarded x64/x86 native builds pass.

## Final scoped receipt

Native summary5xx action filters6→1 complete503, updates received/decoded27B,
compression0B(1/1), and clears excluded selection. Source session SHA is unchanged.
Docker Linux CGO race passes summary1.042s/requestbuilder11.101s at Go1.26.10;
initial image GOTOOLCHAIN=local setup failure is retained separately, not a test
failure or Windows race claim. Final native cases16:14PASS plus2preserved pre-fix
FAILs with explicit corrected rechecks. Full app49.800s/48.696s and guarded x64/x86
builds pass. Private raw log/binary/source hashes are in final-validation.json.
Owned application/fixture/race containers are removed; seven preexisting Docker
containers remain running. Private test profiles and raw sessions stay local.
Current remaining-acceptance.json references the prior21-route inventory and
keeps original/client, actual IME/drag, full combinations, x64 body preflight,
native driver/foreign-client/isolated-host and bounded-memory limits explicit.
artifact-sha256.json hashes exact indexed evidence bytes, excluding itself.

Final byte-count review additionally corrects short/chunked/HEAD bodySize; see
observed-byte-size.md. This supersedes the earlier interrupted12B-body observation
with actual5B/header12, preserving the older receipts. Final17nativecases are
15PASS/2preserved pre-fixFAILs. Latest guarded app tests45.781s/49.029s and Docker
race1.026s/11.511s pass after the last Go change; final builds are
8c471359-plus-observed-bytes x64/x86. Owned cleanup was repeated after this check.
