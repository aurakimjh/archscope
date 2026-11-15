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
