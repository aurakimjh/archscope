# Input assistance and manual collection

Use Analysis Workspace → Input assistance before importing a selected local file.
Inspect shows the current scope's registered generic importer candidates, confidence
and evidence grade, declared capabilities and limitations. It creates no analysis
result. A hint or extension match is not complete format/producer validation.
Import still performs the canonical parser/analyzer checks. Dedicated legacy
analysis views retain their existing support matrix; this feature does not register
new producers. Binary JFR has a conversion notice, and JFR JSON hints direct you to
the JFR view rather than advertising a generic importer.

Inspection owns a bounded source snapshot (64 MiB input, 30 seconds, at most two
concurrent inspections). It hashes the complete selected bytes for identity and
reads at most 64 KiB for detection. It does not scan directories, read adjacent
files or run tools. Temporary snapshots are removed on success/error/cancellation.
Large or changed inputs fail visibly instead of silently truncating evidence.

An optional, explicitly selected capture sidecar is limited to 2 MiB and validated
against the existing capture contract. Its file SHA-256 and decimal byte size must
match the selected source. Entity context is taken only from IDs linked to that
file. A match verifies file binding, not the truth of user-declared producer,
clock, process or workload claims. Missing producer/version, start/end, timezone,
clock domain, host and process lifetime remain visible. File modification time is
never used as collection time. Unknown context does not forbid basic analysis;
correlation and comparison still apply their own evidence requirements.

“Save new capture draft” writes a fresh sidecar skeleton with observed file hash
and size, a new artifact ID, and unknown capture identity/clock/producer fields.
It does not merge or overwrite supplied declarations. Review the draft in a text
editor, use explicit evidence bases, and attach it through the existing case flow.
A portable-unsafe filename is replaced by `selected-source.bin` with a review
warning; review that filename before use. Do not put credentials, environment
dumps, private keys or complete invocation strings in a sidecar.

## Separate manual kit

“Save manual collection kit” or `evidence collection-kit --out /new/manual.zip`
exports inert bilingual manuals, `recipes.json`, and the capture schema. The ZIP
is separate from data-only packs and contains no executable collector. Recipes
include manually reviewed JDK JFR collection/conversion commands and a generic
existing-export checklist. The app never runs recipe commands, attaches to a
process, elevates privilege or downloads tools. Select permissions, target PID,
output location, duration and operational overhead in the collection environment.
Inspect exported data for sensitive content before sharing it.

```sh
archscope-engine evidence discover --in /local/source --capture /local/capture.json
archscope-engine evidence discover --in /local/source --pack-store /local/packs --preset-scope user
archscope-engine evidence collection-kit --out /new/manual.zip
```

`--capture` is optional. Discovery JSON includes `capture_draft` for CLI use.
Use the supplied capture schema; absent metadata is not a measured zero.

## Explicit installed JDK conversion

Prefer preconversion with compatible tooling in the collection environment. For
local binary JFR conversion, choose an absolute path to a trusted installed JDK
`jfr` executable and an unused JSON output name. The app runs only `--version`
and `print --json <owned-input>` argv; no shell expansion or arbitrary options.
The version check is limited to five seconds, the whole operation to two minutes,
input/output to 64 MiB each, stderr to 16 KiB, and concurrency to two operations.
Output streams to a private file; excess output, invalid JSON/event-array shape,
nonzero exit, changed executable/source or cancellation removes scratch/output.
Cancellation kills the process group on Unix and the suspended-start Job Object
on Windows. Native Windows process execution remains a platform acceptance item.

```sh
archscope-engine jfr convert --in /local/recording.jfr --cli /installed/jdk/bin/jfr --out /new/recording.json
archscope-engine jfr analyze-json --in /new/recording.json --out /new/result.json
```

Successful conversion saves JSON plus `<output>.conversion.json` with tool/version,
executable SHA-256, fixed sanitized arguments, input/output digests, byte count and
local location. Both names must be new. If provenance publication fails, the owned
JSON is removed. Outputs inside managed state are refused. External stderr and
raw environment values are omitted from user diagnostics. The child environment
omits Java agent/tool-option injection variables. This is not an execution sandbox
or a cryptographic qualification of the entire JDK installation; the owner chooses
a trusted installed tool. Preserve original JFR and provenance with the JSON.
The downstream JFR parser still validates/interprets events; structural conversion
does not promise complete event/producer coverage.

Existing binary JFR analysis now uses the same bounded runner. Set an explicit
absolute `ARCHSCOPE_JFR_CLI`; PATH/JAVA_HOME discovery alone no longer authorizes
execution. Existing JSON analysis remains unchanged. Missing tool, unsupported
version output, signature/input mismatch, timeout, output limit and tool exit have
stable refusal codes and advise compatible preconversion. UI request cancellation
and CLI interrupt propagate to the converter. The native Go JFR decoder is deferred.
Explicit offline sessions continue to refuse subprocess conversion before tool
execution; bring preconverted JSON. Manual kit/inspection/export work offline.

Official syntax: [JDK jcmd](https://docs.oracle.com/en/java/javase/26/docs/specs/man/jcmd.html)
and [JDK jfr](https://docs.oracle.com/en/java/javase/21/docs/specs/man/jfr.html).
Check the installed tool's help for its actual supported options.

The recorded version is the `jfr --version` tool string (some older JDK tools
report `1.0`), not a separately measured full JDK distribution version. Owned input
keeps a `.jfr` suffix because installed JDK tools require it.

Active declarative mapping candidates can show at most four first-row previews:
exact source numeric text → normalized numeric text/unit/kind, bound to pack SHA.
They use the same constrained interpreter. This is a sample-only preview, never
a complete import or producer qualification; incomplete rows beyond the probe
budget cannot be previewed. Display escaping/length bounds are disclosed.
