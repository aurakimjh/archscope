// Package javajstack ports archscope_engine.parsers.thread_dump.java_jstack.
//
// The Plugin satisfies the threaddump.Plugin contract for JVM jstack
// output and additionally implements MultiBundlePlugin so concatenated
// jstack files (one capture per "Full thread dump …" header) expand
// into one bundle per dump.
//
// Sub-features ported alongside the bare parse:
//   - T-194 — strip CGLIB / JDK proxy / accessor synthetic suffixes from
//     stack frames so the same logical call site collapses to a single
//     stack signature regardless of which proxy hash showed up.
//   - T-195 — promote RUNNABLE threads stuck in epoll/socket frames to
//     NETWORK_WAIT, file-IO frames to IO_WAIT. Other states are never
//     touched (the runtime knows better than our heuristic).
//   - T-219 / T-231 — extract structured monitor handles ("locked",
//     "waiting to lock", "waiting on", "parking to wait for").
//   - T-227 — tag virtual-thread carrier pinning blocks with metadata
//     describing the candidate user-frame method.
//   - T-228 / T-234 — parse "Threads class SMR info:" diagnostics and
//     cross-reference addresses against parsed thread tids.
//   - T-229 — pick the first "(Native Method)" frame as native_method
//     metadata.
//   - T-230 / T-235 — parse the optional class histogram block and
//     surface incomplete-block markers.
//
// Enrichment runs only on Java frames so the registry can mix this
// plugin with Go/Python/etc. parsers without cross-contamination.
package javajstack

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump"
)

// FormatID matches the Python plugin's class attribute byte-for-byte so
// CLI / registry override strings stay compatible.
const FormatID = "java_jstack"

// Language matches Python's `language = "java"`.
const Language = "java"

// classHistogramLimitEnv mirrors Python's environment variable knob.
const classHistogramLimitEnv = "ARCHSCOPE_CLASS_HISTOGRAM_ROW_LIMIT"

// defaultClassHistogramRowLimit matches the Python constant.
const defaultClassHistogramRowLimit = 500

// Detector heuristics — mirror the Python regexes verbatim.
var (
	fullThreadHeaderRE = regexp.MustCompile(`(?i)Full thread dump\b`)
	jstackNidLineRE    = regexp.MustCompile(`(?m)^"[^"]+".*\bnid=0x[0-9a-fA-F]+`)
	// Loose detector for jstack-like dumps that omit "Full thread dump".
	jstackLooseHeaderRE = regexp.MustCompile(
		`(?m)^"[^"]+"\s+#\d+\s+prio=\d+(?:\s+\w+)*\s+tid=\S+\s+(?:RUNNABLE|BLOCKED|WAITING|TIMED_WAITING|NEW|TERMINATED)`,
	)
	threadBlockHeaderRE = regexp.MustCompile(`^"[^"]+"`)
)

// Plugin is the Java jstack parser plugin. It is value-safe; callers
// can register a single instance with the default registry.
type Plugin struct{}

// New returns a fresh Plugin. Plugin has no configuration so the zero
// value is also fine — this constructor only exists to keep the call
// site symmetric with other plugin packages.
func New() *Plugin { return &Plugin{} }

// FormatID returns the plugin's stable identifier.
func (p *Plugin) FormatID() string { return FormatID }

// Language returns the runtime label.
func (p *Plugin) Language() string { return Language }

// CanParse implements threaddump.Plugin. The head sample is asked
// against three increasingly loose heuristics:
//
//  1. The classic `Full thread dump …` banner.
//  2. Any quoted-name line carrying a `nid=0x…` field.
//  3. At least two `"Name" #id prio=n tid=… STATE` rows.
func (p *Plugin) CanParse(head string) bool {
	if fullThreadHeaderRE.MatchString(head) {
		return true
	}
	if jstackNidLineRE.MatchString(head) {
		return true
	}
	hits := jstackLooseHeaderRE.FindAllStringIndex(head, -1)
	return len(hits) >= 2
}

// Parse implements threaddump.Plugin. Returns one bundle covering every
// thread block in the file. The class-histogram row limit comes from
// the env var (or the package default).
func (p *Plugin) Parse(path string) (models.ThreadDumpBundle, error) {
	rowLimit, err := classHistogramRowLimit()
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}
	records, err := parseThreadDump(path)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}
	bundle := models.NewThreadDumpBundle(path, FormatID, Language)
	bundle.Metadata["class_histogram_row_limit"] = rowLimit
	for index, record := range records {
		bundle.Snapshots = append(bundle.Snapshots, recordToSnapshot(record, index, 0))
	}
	return bundle, nil
}

// ParseAll implements threaddump.MultiBundlePlugin. Splits the file at
// every `Full thread dump …` banner into a separate bundle so the
// multi-dump pipeline correlates threads across captures.
//
// Returns an empty slice (and lets the registry fall back to a single
// Parse call) when the file does not contain any explicit section
// boundaries — the same control-flow Python uses.
func (p *Plugin) ParseAll(path string) ([]models.ThreadDumpBundle, error) {
	rowLimit, err := classHistogramRowLimit()
	if err != nil {
		return nil, err
	}
	sections, err := splitJstackSections(path)
	if err != nil {
		return nil, err
	}
	if len(sections) == 0 {
		bundle, err := p.Parse(path)
		if err != nil {
			return nil, err
		}
		return []models.ThreadDumpBundle{bundle}, nil
	}

	bundles := make([]models.ThreadDumpBundle, 0, len(sections))
	for sectionIndex, section := range sections {
		bundle := models.NewThreadDumpBundle(path, FormatID, Language)
		for index, record := range section.Records {
			bundle.Snapshots = append(bundle.Snapshots, recordToSnapshot(record, index, sectionIndex))
		}
		metadata := sectionMetadata(section, rowLimit)
		// T-234: cross-reference SMR addresses with parsed thread tids
		// so resolved/unresolved counts reflect real matches.
		if smr, ok := metadata["smr"].(map[string]any); ok {
			metadata["smr"] = postProcessSMR(smr, section.Records)
		}
		for k, v := range metadata {
			bundle.Metadata[k] = v
		}
		bundles = append(bundles, bundle)
	}
	return bundles, nil
}

// recordToSnapshot converts one parsed thread record into a
// ThreadSnapshot with all enrichment applied. sectionIndex is the
// position of the parent dump in the file (0 for single-dump files).
func recordToSnapshot(record threadDumpRecord, index, sectionIndex int) models.ThreadSnapshot {
	state := models.CoerceThreadState(record.State)
	frames := make([]models.StackFrame, 0, len(record.Stack))
	for _, line := range record.Stack {
		frames = append(frames, frameFromJstack(line))
	}
	// T-194: collapse CGLIB / proxy hash variants.
	for i := range frames {
		frames[i] = normalizeProxyFrame(frames[i])
	}
	// T-195: promote RUNNABLE epoll/socket/file-IO threads.
	state = inferJavaState(state, frames)
	// T-219 / T-231: extract structured lock handles.
	lockHolds, lockWaiting := extractLockHandles(record.RawBlock)
	if lockWaiting != nil && lockWaiting.WaitMode == waitModeLockEntryWait &&
		(state == models.ThreadStateRunnable || state == models.ThreadStateUnknown) {
		state = models.ThreadStateLockWait
	} else if lockWaiting != nil && state == models.ThreadStateUnknown {
		state = models.ThreadStateWaiting
	}

	snapshot := models.NewThreadSnapshot(
		fmt.Sprintf("java::%d::%d::%s", sectionIndex, index, record.ThreadName),
		record.ThreadName,
		state,
	)
	if record.ThreadID != "" {
		threadID := record.ThreadID
		snapshot.ThreadID = &threadID
	}
	if record.Category != "" {
		category := record.Category
		snapshot.Category = &category
	}
	snapshot.StackFrames = frames
	if record.LockInfo != "" {
		lockInfo := record.LockInfo
		snapshot.LockInfo = &lockInfo
	}
	language := Language
	snapshot.Language = &language
	sourceFormat := FormatID
	snapshot.SourceFormat = &sourceFormat
	snapshot.LockHolds = lockHolds
	snapshot.LockWaiting = lockWaiting

	snapshot.Metadata["raw_block"] = record.RawBlock
	if isVirtualThread(record) {
		snapshot.Metadata["is_virtual_thread"] = true
	}
	if native := nativeMethod(record); native != "" {
		snapshot.Metadata["native_method"] = native
	}
	if pinning := carrierPinning(record.RawBlock, frames); pinning != nil {
		snapshot.Metadata["carrier_pinning"] = pinning
	}
	if lockWaiting != nil && lockWaiting.WaitMode != "" {
		snapshot.Metadata["monitor_wait_mode"] = lockWaiting.WaitMode
	}
	return snapshot
}

// classHistogramRowLimit reads the env var override or returns the
// package default. Errors mirror Python's ValueError shape.
func classHistogramRowLimit() (int, error) {
	raw := os.Getenv(classHistogramLimitEnv)
	if raw == "" {
		return defaultClassHistogramRowLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, fmt.Errorf(
			"%s / class_histogram_limit must be a positive integer.",
			classHistogramLimitEnv,
		)
	}
	return limit, nil
}

// frameFromJstack parses a single `foo.Bar.method(File.java:42)` style
// stack line into StackFrame. Unparseable lines fall through with the
// raw text in Function.
var javaFrameRE = regexp.MustCompile(`^([\w$./<>]+)\(([^)]*)\)\s*$`)

func frameFromJstack(line string) models.StackFrame {
	text := trimSpaces(line)
	match := javaFrameRE.FindStringSubmatch(text)
	language := Language
	if match == nil {
		return models.StackFrame{Function: text, Language: &language}
	}
	qualified := match[1]
	if idx := indexOf(qualified, '/'); idx >= 0 {
		qualified = qualified[idx+1:]
	}
	location := match[2]
	var module *string
	function := qualified
	if dot := lastIndexOf(qualified, '.'); dot >= 0 {
		mod := qualified[:dot]
		module = &mod
		function = qualified[dot+1:]
	}

	var filePtr *string
	var linePtr *int
	if location != "" {
		if colon := lastIndexOf(location, ':'); colon >= 0 {
			fileText := location[:colon]
			lineText := location[colon+1:]
			if n, err := strconv.Atoi(lineText); err == nil {
				filePtr = &fileText
				linePtr = &n
			} else {
				loc := location
				filePtr = &loc
			}
		} else {
			loc := location
			filePtr = &loc
		}
	}

	return models.StackFrame{
		Function: function,
		Module:   module,
		File:     filePtr,
		Line:     linePtr,
		Language: &language,
	}
}

// trimSpaces trims leading/trailing ASCII whitespace. Avoids importing
// strings just for one helper, keeping this file's dependency set
// honest.
func trimSpaces(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

func indexOf(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func lastIndexOf(s string, b byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// init registers the plugin with the default registry so callers that
// just import the package pick up the Java plugin without touching the
// registry directly. Mirrors Python's module-level
// DEFAULT_REGISTRY.register(...) call.
func init() {
	threaddump.DefaultRegistry.Register(New())
}

// Compile-time interface checks.
var (
	_ threaddump.Plugin            = (*Plugin)(nil)
	_ threaddump.MultiBundlePlugin = (*Plugin)(nil)
)
