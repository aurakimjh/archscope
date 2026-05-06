// Package dotnetclrstack ports
// archscope_engine.parsers.thread_dump.dotnet_clrstack to Go (T-326).
//
// The plugin handles two related .NET text-dump shapes that both
// surface stack frames as `module!Class.Method(...)`-style tokens:
//
//  1. WinDbg / dotnet-dump `!clrstack` (or `~* e !clrstack`) listings,
//     where each thread is introduced by an `OS Thread Id: 0x… (N)`
//     header followed by an optional `Child SP IP Call Site` column
//     header and one frame per line. A trailing `Sync Block Owner Info:`
//     section, when present, is captured verbatim under bundle
//     metadata for downstream lock-correlation analysis.
//
//  2. Ad-hoc `Environment.StackTrace` snapshots (no thread header) —
//     the file is treated as a single thread whose frames are the
//     captured stack at snapshot time.
//
// Both plugins land .NET frames with `language="dotnet"`, run the
// same async-state-machine + local-function name normalization the
// Python implementation does, and use frame-aware state inference
// (Monitor.* → LOCK_WAIT, Socket.* → NETWORK_WAIT, …).
package dotnetclrstack

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// ---------------------------------------------------------------------------
// Format-identifier constants. Exposed so registry callers / tests can
// refer to them without retyping the literal string.
// ---------------------------------------------------------------------------

const (
	// FormatID is the stable identifier surfaced on
	// ThreadDumpBundle.SourceFormat for clrstack-shaped dumps.
	FormatID = "dotnet_clrstack"
	// EnvironmentStackTraceFormatID is the format-id used for the
	// header-less Environment.StackTrace variant.
	EnvironmentStackTraceFormatID = "dotnet_environment_stacktrace"
	// Language matches Python's `language: str = "dotnet"`.
	Language = "dotnet"
)

// ---------------------------------------------------------------------------
// Regex catalogue. Mirrors the module-level Python regexes verbatim so
// the two parsers reject / accept the same inputs.
// ---------------------------------------------------------------------------

// osThreadHeaderRE matches `OS Thread Id: 0x1a4 (1)` headers.
// `(?P<managed>\d+)` is optional; when missing the thread name falls
// back to the hex tid.
var osThreadHeaderRE = regexp.MustCompile(
	`^OS Thread Id:\s+(?P<tid>0x[0-9a-fA-F]+)(?:\s+\((?P<managed>\d+)\))?\s*$`,
)

// frameHeaderRE matches the optional column header that clrstack
// emits between the thread header and the first frame.
var frameHeaderRE = regexp.MustCompile(`^\s*Child SP\s+IP\s+Call Site\s*$`)

// frameLineRE matches one stack frame line: SP, IP, then the call
// site. Both addresses are pure hex; the call site is everything that
// follows up to end of line.
var frameLineRE = regexp.MustCompile(
	`^(?P<sp>[0-9a-fA-F]+)\s+(?P<ip>[0-9a-fA-F]+)\s+(?P<call>.+)$`,
)

// syncBlockHeaderRE detects the start of the optional sync-block
// owner table. We don't deeply parse it but capture the lines that
// follow as evidence under bundle metadata.
var syncBlockHeaderRE = regexp.MustCompile(`^Sync Block Owner Info:\s*$`)

// canParseRE mirrors Python's `re.search(r"^OS Thread Id:\s+0x", head,
// re.MULTILINE)` — Go regexp's `(?m)` flag flips `^` to match after
// any newline, matching Python's `re.MULTILINE`.
var canParseRE = regexp.MustCompile(`(?m)^OS Thread Id:\s+0x`)

// dotnetStackFrameRE matches the `Environment.StackTrace` line shape
// (`   at Foo.Bar(...) [in path:line N]`).
var dotnetStackFrameRE = regexp.MustCompile(
	`^\s+at\s+(?P<call>[\w.<>+` + "`" + `\[\]]+(?:\([^)]*\))?)` +
		`(?:\s+in\s+(?P<file>[^:]+):line\s+(?P<line>\d+))?\s*$`,
)

// envStackTraceClrHitsRE matches the `at <Type>.<Method>(` shape
// used as a heuristic when the head doesn't carry the more reliable
// `Environment.get_StackTrace` token.
var envStackTraceClrHitsRE = regexp.MustCompile(
	`(?m)^\s+at\s+[A-Z][\w.<>+]+\.[A-Za-z_<][\w<>+]*\(`,
)

// asyncStateMachineRE recognises C# async state-machine wrappers like
// `<DoWorkAsync>d__7` so they can be flattened back to the user-named
// method.
var asyncStateMachineRE = regexp.MustCompile(`<(?P<name>[^>]+)>d__\d+(?P<rest>.*)`)

// localFunctionRE recognises local-function synthesis tokens like
// `<Foo>g__Bar|3_0` so they can be rendered as `Foo.Bar`.
var localFunctionRE = regexp.MustCompile(
	`<(?P<outer>[^>]+)>g__(?P<inner>[A-Za-z_]\w*)\|\d+_\d+`,
)

// State-inference patterns — the (?i) prefix gives the Python
// `re.IGNORECASE` parity. Each pattern keeps the Python word-boundary
// `\b` so `Socket.Receive` matches but `MyKafkaSocketReceive` does not.
var (
	dotnetNetworkFunctionsRE = regexp.MustCompile(
		`(?i)\b(?:Socket\.Receive|Socket\.Send|Socket\.Accept|Socket\.Connect|` +
			`NetworkStream\.Read|NetworkStream\.Write|` +
			`HttpClient\.Send|HttpRequestMessage\.SendAsync)`,
	)
	dotnetLockFunctionsRE = regexp.MustCompile(
		`(?i)\b(?:Monitor\.Enter|Monitor\.Wait|SpinLock\.Enter|` +
			`SemaphoreSlim\.WaitAsync|ReaderWriterLockSlim\.Enter)`,
	)
	dotnetIOFunctionsRE = regexp.MustCompile(
		`(?i)\b(?:File\.Read|FileStream\.Read|StreamReader\.Read|` +
			`BufferedStream\.Read)`,
	)
)

// ---------------------------------------------------------------------------
// Plugin: dotnet_clrstack (WinDbg !clrstack output)
// ---------------------------------------------------------------------------

// Plugin parses WinDbg / dotnet-dump `!clrstack` listings. Mirrors
// Python's `DotnetClrstackParserPlugin`. The zero value is ready to
// use; callers typically construct one and Register() it on the
// shared threaddump.Registry.
type Plugin struct{}

// New returns a freshly-constructed plugin. Provided for parity with
// other plugin packages — `&Plugin{}` is equivalent.
func New() *Plugin { return &Plugin{} }

// FormatID returns the stable format identifier surfaced on the
// emitted bundle. Matches Python `format_id = "dotnet_clrstack"`.
func (p *Plugin) FormatID() string { return FormatID }

// Language returns the runtime label. Matches Python `language =
// "dotnet"`.
func (p *Plugin) Language() string { return Language }

// CanParse mirrors Python's `re.search(r"^OS Thread Id:\s+0x", head,
// re.MULTILINE)` — any line starting with `OS Thread Id: 0x…` claims
// the file. Cheap regex match against the head sample only.
func (p *Plugin) CanParse(head string) bool {
	return canParseRE.MatchString(head)
}

// Parse reads `path` end-to-end and emits one bundle. Matches
// Python's `parse(path)` exactly: snapshots are emitted in source
// order, sync-block owner table (if present) is captured verbatim
// under bundle metadata as `sync_block_owner_info`, and each
// snapshot's metadata records the optional `managed_id` (nil when
// absent so the JSON shape matches Python's `dict[str, Optional[str]]`).
func (p *Plugin) Parse(path string) (models.ThreadDumpBundle, error) {
	text, err := readUTF8File(path)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}

	bundle := models.NewThreadDumpBundle(path, FormatID, Language)

	lines := splitLines(text)
	syncBlockEvidence := []string{}
	threadIndex := 0
	index := 0
	for index < len(lines) {
		line := lines[index]

		// Sync-block owner table: capture the run of non-blank,
		// non-`OS Thread Id:` lines as evidence and keep iterating.
		if syncBlockHeaderRE.MatchString(line) {
			index++
			for index < len(lines) {
				sub := lines[index]
				if osThreadHeaderRE.MatchString(sub) || strings.TrimSpace(sub) == "" {
					break
				}
				syncBlockEvidence = append(syncBlockEvidence, sub)
				index++
			}
			continue
		}

		// Thread header — start of a new snapshot block.
		header := osThreadHeaderRE.FindStringSubmatch(line)
		if header == nil {
			index++
			continue
		}
		tid := header[osThreadHeaderRE.SubexpIndex("tid")]
		managedID := header[osThreadHeaderRE.SubexpIndex("managed")]

		index++
		// Optional `Child SP IP Call Site` column header — skip when present.
		if index < len(lines) && frameHeaderRE.MatchString(lines[index]) {
			index++
		}

		frames := []models.StackFrame{}
		for index < len(lines) {
			sub := lines[index]
			if osThreadHeaderRE.MatchString(sub) {
				break
			}
			if syncBlockHeaderRE.MatchString(sub) {
				break
			}
			frameMatch := frameLineRE.FindStringSubmatch(strings.TrimSpace(sub))
			if frameMatch != nil {
				call := strings.TrimSpace(frameMatch[frameLineRE.SubexpIndex("call")])
				frames = append(frames, parseCallSite(call))
			}
			index++
		}

		// Normalise async / local-function synthesised names before
		// running state inference, exactly as the Python code does.
		for i := range frames {
			frames[i] = normalizeDotnetFrame(frames[i])
		}
		state := inferDotnetState(models.ThreadStateRunnable, frames)

		threadName := tid
		if managedID != "" {
			threadName = managedID
		}

		snap := models.NewThreadSnapshot(
			snapshotID(threadIndex, tid),
			threadName,
			state,
		)
		snap.ThreadID = strPtr(tid)
		category := string(state)
		snap.Category = &category
		snap.StackFrames = frames

		// Python keeps `managed_id` as `None` when missing — we mirror
		// that with a typed nil interface so JSON emits `null`.
		if managedID != "" {
			snap.Metadata["managed_id"] = managedID
		} else {
			snap.Metadata["managed_id"] = nil
		}
		lang := Language
		snap.Language = &lang
		fmtID := FormatID
		snap.SourceFormat = &fmtID

		bundle.Snapshots = append(bundle.Snapshots, snap)
		threadIndex++
	}

	if len(syncBlockEvidence) > 0 {
		bundle.Metadata["sync_block_owner_info"] = syncBlockEvidence
	}

	return bundle, nil
}

// ---------------------------------------------------------------------------
// Plugin: dotnet_environment_stacktrace (header-less Environment.StackTrace)
// ---------------------------------------------------------------------------

// EnvironmentStackTracePlugin parses ad-hoc `Environment.StackTrace`
// dumps. Mirrors Python's `DotnetEnvironmentStackTraceParserPlugin`.
// Each file produces at most one snapshot whose name is "main" and
// thread-id is "0".
type EnvironmentStackTracePlugin struct{}

// NewEnvironmentStackTracePlugin returns a fresh plugin. Provided for
// parity with the package's other constructors.
func NewEnvironmentStackTracePlugin() *EnvironmentStackTracePlugin {
	return &EnvironmentStackTracePlugin{}
}

// FormatID returns the format identifier surfaced on emitted bundles.
// Matches Python `format_id = "dotnet_environment_stacktrace"`.
func (p *EnvironmentStackTracePlugin) FormatID() string { return EnvironmentStackTraceFormatID }

// Language returns the runtime label.
func (p *EnvironmentStackTracePlugin) Language() string { return Language }

// CanParse uses the two heuristics Python uses, in the same order:
//  1. literal substring `"Environment.get_StackTrace"` — the most
//     reliable single-line signature.
//  2. at least two `   at <Type>.<Method>(` frames whose types look
//     CLR-flavoured (PascalCase namespaces).
func (p *EnvironmentStackTracePlugin) CanParse(head string) bool {
	if strings.Contains(head, "Environment.get_StackTrace") {
		return true
	}
	hits := envStackTraceClrHitsRE.FindAllString(head, -1)
	return len(hits) >= 2
}

// Parse reads the file and emits a single snapshot whose stack is the
// concatenation of every matching `at …` line, in source order. Frames
// surface `file` / `line` (file is *string and line is *int) when the
// optional `in <path>:line N` suffix is present. When the file
// produces zero frames the bundle's snapshots slice is empty — same
// behaviour as Python (`snapshots=[snapshot] if frames else []`).
func (p *EnvironmentStackTracePlugin) Parse(path string) (models.ThreadDumpBundle, error) {
	text, err := readUTF8File(path)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}
	bundle := models.NewThreadDumpBundle(path, EnvironmentStackTraceFormatID, Language)

	frames := []models.StackFrame{}
	for _, raw := range splitLines(text) {
		match := dotnetStackFrameRE.FindStringSubmatch(raw)
		if match == nil {
			continue
		}
		call := strings.TrimSpace(match[dotnetStackFrameRE.SubexpIndex("call")])
		fileVal := match[dotnetStackFrameRE.SubexpIndex("file")]
		lineVal := match[dotnetStackFrameRE.SubexpIndex("line")]

		frame := models.StackFrame{
			Function: call,
			Language: strPtr(Language),
		}
		if fileVal != "" {
			frame.File = strPtr(fileVal)
		}
		if lineVal != "" {
			if n, err := strconv.Atoi(lineVal); err == nil {
				frame.Line = intPtr(n)
			}
		}
		frames = append(frames, normalizeDotnetFrame(frame))
	}

	if len(frames) == 0 {
		return bundle, nil
	}

	state := inferDotnetState(models.ThreadStateRunnable, frames)
	snap := models.NewThreadSnapshot("dotnet::0::main", "main", state)
	snap.ThreadID = strPtr("0")
	category := string(state)
	snap.Category = &category
	snap.StackFrames = frames
	snap.Metadata["format_variant"] = "environment_stacktrace"
	lang := Language
	snap.Language = &lang
	fmtID := EnvironmentStackTraceFormatID
	snap.SourceFormat = &fmtID

	bundle.Snapshots = append(bundle.Snapshots, snap)
	return bundle, nil
}

// ---------------------------------------------------------------------------
// Internal helpers — frame parsing, normalization, state inference.
// ---------------------------------------------------------------------------

// parseCallSite mirrors Python's `_parse_call_site`. Splits the call
// string at the first `(` to find the qualified name, then partitions
// the qualified name at the LAST `.` so module = `MyApp.Service` and
// function = `Process` for `MyApp.Service.Process(MyApp.Request)`.
//
// When the qualified name has no `.`, module is left nil and function
// is the whole identifier — matching Python `module, function = None,
// qualified`.
func parseCallSite(call string) models.StackFrame {
	open := strings.Index(call, "(")
	qualified := call
	if open > 0 {
		qualified = call[:open]
	}
	frame := models.StackFrame{Language: strPtr(Language)}
	if dot := strings.LastIndex(qualified, "."); dot >= 0 {
		module := qualified[:dot]
		function := qualified[dot+1:]
		frame.Module = strPtr(module)
		if function != "" {
			frame.Function = function
		} else {
			frame.Function = call
		}
	} else {
		if qualified != "" {
			frame.Function = qualified
		} else {
			frame.Function = call
		}
	}
	return frame
}

// normalizeDotnetFrame mirrors Python `_normalize_dotnet_frame`. It
// strips C# async state-machine and local-function synthesised
// tokens out of the module identifier so equivalent stacks across
// runs collapse onto the same StackSignature.
func normalizeDotnetFrame(frame models.StackFrame) models.StackFrame {
	if frame.Language == nil || *frame.Language != Language {
		return frame
	}
	if frame.Module == nil {
		return frame
	}

	originalModule := *frame.Module
	newModule := originalModule

	if loc := localFunctionRE.FindStringSubmatchIndex(newModule); loc != nil {
		matched := newModule[loc[0]:loc[1]]
		outer := newModule[loc[localFunctionRE.SubexpIndex("outer")*2]:loc[localFunctionRE.SubexpIndex("outer")*2+1]]
		inner := newModule[loc[localFunctionRE.SubexpIndex("inner")*2]:loc[localFunctionRE.SubexpIndex("inner")*2+1]]
		newModule = strings.Replace(newModule, matched, outer+"."+inner, 1)
	}
	if loc := asyncStateMachineRE.FindStringSubmatchIndex(newModule); loc != nil {
		matched := newModule[loc[0]:loc[1]]
		name := newModule[loc[asyncStateMachineRE.SubexpIndex("name")*2]:loc[asyncStateMachineRE.SubexpIndex("name")*2+1]]
		rest := newModule[loc[asyncStateMachineRE.SubexpIndex("rest")*2]:loc[asyncStateMachineRE.SubexpIndex("rest")*2+1]]
		newModule = strings.Replace(newModule, matched, name+rest, 1)
	}

	if newModule == originalModule {
		return frame
	}
	out := frame
	out.Module = strPtr(newModule)
	return out
}

// inferDotnetState mirrors Python `_infer_dotnet_state`. The TOP frame
// drives state inference — the order of the three regex tests
// (network → lock → io) matches the Python file so we promote the
// same way on overlapping signatures.
func inferDotnetState(state models.ThreadState, frames []models.StackFrame) models.ThreadState {
	if len(frames) == 0 {
		return state
	}
	top := frames[0]
	qualified := top.Function
	if top.Module != nil && *top.Module != "" {
		qualified = *top.Module + "." + top.Function
	}
	if dotnetNetworkFunctionsRE.MatchString(qualified) {
		return models.ThreadStateNetworkWait
	}
	if dotnetLockFunctionsRE.MatchString(qualified) {
		return models.ThreadStateLockWait
	}
	if dotnetIOFunctionsRE.MatchString(qualified) {
		return models.ThreadStateIOWait
	}
	return state
}

// snapshotID is the canonical `dotnet::<index>::<tid>` identifier
// the Python plugin generates. Centralised to keep the format under
// one knob.
func snapshotID(index int, tid string) string {
	return "dotnet::" + strconv.Itoa(index) + "::" + tid
}

// readUTF8File reads `path` and decodes using textio's encoding
// fallback chain. Matches Python's `Path(path).read_text(
// encoding="utf-8", errors="replace")` for utf-8 inputs and
// transparently handles latin-1 fallbacks the textio package
// inherits from the rest of the codebase.
func readUTF8File(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	encoding, err := textio.DetectFromBytes(raw, nil)
	if err != nil {
		return "", err
	}
	return textio.DecodeBytes(raw, encoding)
}

// splitLines mirrors Python's `str.splitlines()` followed by
// `rstrip("\r")` on each yielded line — Go's bufio.Scanner already
// strips trailing `\r` so a plain strings.Split on `\n` plus a manual
// trim for embedded `\r` covers all the line endings clrstack emits
// (CRLF on Windows-captured dumps, LF on Linux-captured ones).
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	// Strip the final trailing newline so we don't emit a phantom
	// empty line — matches Python's `splitlines()` behaviour.
	trimmed := strings.TrimRight(text, "\n")
	parts := strings.Split(trimmed, "\n")
	for i, p := range parts {
		parts[i] = strings.TrimRight(p, "\r")
	}
	return parts
}

// strPtr / intPtr are the package's tiny helpers for taking the
// address of a literal — needed because the model uses pointer
// fields to round-trip Python's `Optional[...]` defaults.
func strPtr(s string) *string { return &s }
func intPtr(n int) *int       { return &n }
