# T-608 native HALOG codec

This follow-up supersedes the initial `halog-reference.json` investigation's
UNVERIFIED codec verdict. It does not close the other Windows parity gates.

The private input is 35,463,916 bytes, 675 entries, SHA-256
`d2ed106e5bc8bc0f31509e6aad7fe09149b12d4ebd96fc19b3672f7858857bad`.
Its source file is unchanged. No private native log, XML, URL, process path or
body is committed. The checked-in 2,201-byte fixture contains synthetic data
saved by reference version 7.6.4.508.

The container is a typed attribute bag and `qh.IntfList` of request schema 7,
format version 6. The data-only reader permits the observed request, cache,
timing and memory-stream classes. It never instantiates native objects. Counts,
nesting, fields, strings, streams, input and decoded bodies are bounded. Unknown
classes/tags/versions, truncation, duplicates and trailing bytes fail explicitly.
Streams use variable-width length prefixes followed by raw bytes; the sampled
container is neither compressed nor encrypted.

Low-level decode/re-encode preserves all 35,463,916 source bytes exactly. This
private verification API is distinct from product exports: desktop/CLI/native
automation exports apply the existing redaction policy and rebuild HTTP framing
from retained text. Exported streams, content lengths and byte counts describe
that reconstruction. Original cache paths, handles and PID suffixes are omitted.
Process names are unverified labels with PID 0, not observed OS identities.
Binary, unavailable and unsupported encoded payloads are explicitly unavailable
in the semantic projection. Native export omits non-retained data and interim
responses, with a disclosure in each record's comment. HTTP/2/3 framing is
reconstructed as HTTP/1.1. Session files/HAR remain suitable for richer metadata.

Metadata uses Windows ANSI variants. The implementation supports CP949,
Windows-1252 and UTF-8 and defaults to the Windows ANSI codepage (1252 on other
hosts). ArchScope writes an advisory `ArchScopeCodePage` header for its own
portable reimport; the reference program reads metadata using its current ANSI
codepage. Unrepresentable metadata fails rather than replacing characters.
OLE DATE has no timezone; dates are interpreted in the current host timezone,
which the importer discloses. Move a legacy file between timezones only with
that qualification. Text bodies retain their UTF-8 bytes independently.

`verify-halog-codec.ps1` validates the reference executable's version and COM
registration, refuses active recording, preserves/restores an existing stopped
log, compares the private decoded projection against COM, opens the byte-exact
round trip, opens redacted sample exports and synthetic boundary/state cases,
then saves with NativeLog=6 and reimports those reference outputs. Its output is
metadata only. It generates all payload files in a fresh private directory.

Validation results are recorded in `halog-codec-interop.json` and
`halog-codec-validation.json`. The earlier malformed Full XML export is not used
as a mapping oracle; direct native COM values and the reference native writer
provide the independent checks.

Two native behaviors are covered by the implementation and harness: some POST
payloads exist only in the full request stream with an empty PostData stream;
and a pending request can already have a response status/body. Those bytes are
retained without finalizing the request. V7's COM wrapper also caches entry
objects across `LoadLog`; the harness reactivates COM after each load to compare
the current file, rather than a former entry snapshot.

A 16-record synthetic fixture, also saved by V7.6.4.508, checks completion
independently of the numeric request stage: native `IsComplete` is true for
stage 4 or a nonzero RequestComplete tick. TimeEnd and SR_Result alone do not
decide completion. All 675 private source records are complete according to
the original API. Separate synthetic records exercise truly pending responses.
