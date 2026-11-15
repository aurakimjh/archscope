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
