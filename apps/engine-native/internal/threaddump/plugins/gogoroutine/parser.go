// Package gogoroutine ports archscope_engine.parsers.thread_dump.go_goroutine.
//
// Handles the dump format produced by runtime/debug.Stack(),
// runtime.Stack (panic / SIGQUIT), and GODEBUG=schedtrace style
// output. Each goroutine block looks like:
//
//	goroutine 17 [chan receive, 5 minutes]:
//	main.worker(0xc0000a8000)
//	    /app/main.go:88 +0x55
//	created by main.start
//	    /app/main.go:10 +0x33
//
// State inference (T-197) promotes Go runtime states that the
// multi-dump correlator should not treat as "running":
// netpoll/netpollBlock → NETWORK_WAIT, selectgo / chan ops →
// CHANNEL_WAIT, semacquire / mutex → LOCK_WAIT. Framework cleanup
// strips wrapper methods that obscure the real call site.
package gogoroutine

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump"
)

// FormatID and Language identifier constants exposed for callers that
// need to wire up override semantics by name. Mirrors the Python class
// attributes.
const (
	FormatID = "go_goroutine"
	Language = "go"
)

var (
	goroutineHeaderRE = regexp.MustCompile(
		`^goroutine\s+(?P<id>\d+)\s+\[(?P<status>[^\]]+)\]:\s*$`,
	)
	frameLocationRE = regexp.MustCompile(
		`^\t(?P<file>[^:]+\.go):(?P<line>\d+)(?:\s+\+0x[0-9a-fA-F]+)?\s*$`,
	)
	canParseRE = regexp.MustCompile(`(?m)^goroutine\s+\d+\s+\[\w`)
)

// Plugin is the goroutine-dump parser plugin. The zero value is the
// canonical instance — there is no configuration. Implements
// threaddump.Plugin.
type Plugin struct{}

// New returns a ready-to-use plugin pointer. Provided for symmetry
// with future plugins that may need wiring; the zero value is
// equivalent.
func New() *Plugin { return &Plugin{} }

// FormatID returns the stable identifier surfaced on
// ThreadDumpBundle.SourceFormat.
func (p *Plugin) FormatID() string { return FormatID }

// Language returns the runtime label this plugin emits.
func (p *Plugin) Language() string { return Language }

// CanParse mirrors the Python head-sniff: any line that starts with
// "goroutine N [<word-char>" claims the file. Multi-line so the header
// does not have to be the very first line.
func (p *Plugin) CanParse(head string) bool {
	return canParseRE.MatchString(head)
}

// Parse reads the whole file and returns a single bundle.
func (p *Plugin) Parse(path string) (models.ThreadDumpBundle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}
	encoding, err := textio.DetectFromBytes(raw, nil)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}
	text, err := textio.DecodeBytes(raw, encoding)
	if err != nil {
		return models.ThreadDumpBundle{}, err
	}

	bundle := models.NewThreadDumpBundle(path, FormatID, Language)
	for index, block := range splitGoroutineBlocks(text) {
		snap, ok := parseBlock(block, index)
		if ok {
			bundle.Snapshots = append(bundle.Snapshots, snap)
		}
	}
	return bundle, nil
}

// splitGoroutineBlocks mirrors Python `_split_goroutine_blocks`. The
// header line itself starts a new block; any following non-header
// lines (blank or otherwise) accrete into the current block until the
// next header.
func splitGoroutineBlocks(text string) [][]string {
	blocks := [][]string{}
	var current []string
	for _, line := range strings.Split(text, "\n") {
		stripped := strings.TrimRight(line, "\r")
		if goroutineHeaderRE.MatchString(stripped) {
			if len(current) > 0 {
				blocks = append(blocks, current)
			}
			current = []string{stripped}
			continue
		}
		if len(current) > 0 {
			current = append(current, stripped)
		}
	}
	if len(current) > 0 {
		blocks = append(blocks, current)
	}
	return blocks
}

// parseBlock turns one (already-split) block into a ThreadSnapshot.
// Returns ok=false when the header line fails to match — defensive,
// since the splitter only emits blocks that started on a header.
func parseBlock(block []string, index int) (models.ThreadSnapshot, bool) {
	header := goroutineHeaderRE.FindStringSubmatch(block[0])
	if header == nil {
		return models.ThreadSnapshot{}, false
	}
	idIdx := goroutineHeaderRE.SubexpIndex("id")
	statusIdx := goroutineHeaderRE.SubexpIndex("status")
	goroutineID := header[idIdx]
	status := header[statusIdx]

	stateStr, duration := splitStatus(status)
	state := models.CoerceThreadState(stateStr)

	body := block[1:]
	frames := []models.StackFrame{}
	i := 0
	for i < len(body) {
		line := body[i]
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}
		if strings.HasPrefix(line, "created by ") {
			parentText := strings.TrimSpace(line[len("created by "):])
			location := nextLocation(body, i+1)
			frames = append(frames, buildFrame(parentText, location))
			break
		}
		if strings.HasPrefix(line, "\t") {
			// Stray location without a preceding function call line — skip.
			i++
			continue
		}
		location := nextLocation(body, i+1)
		frames = append(frames, buildFrame(strings.TrimSpace(line), location))
		if location != nil {
			// Consume the location line we just attached.
			i += 2
		} else {
			i++
		}
	}

	for idx := range frames {
		frames[idx] = normalizeGoFrame(frames[idx])
	}
	state = inferGoState(state, frames)

	metadata := map[string]any{"raw_status": status}
	if duration != "" {
		metadata["duration"] = duration
	}

	threadName := "goroutine-" + goroutineID
	threadIDCopy := goroutineID
	categoryCopy := string(state)
	languageCopy := Language
	formatCopy := FormatID

	snap := models.NewThreadSnapshot(
		"go::"+strconv.Itoa(index)+"::goroutine-"+goroutineID,
		threadName,
		state,
	)
	snap.ThreadID = &threadIDCopy
	snap.Category = &categoryCopy
	snap.StackFrames = frames
	snap.Metadata = metadata
	snap.Language = &languageCopy
	snap.SourceFormat = &formatCopy
	return snap, true
}

// splitStatus mirrors Python's `state_str, _, duration = status.partition(",")`.
// Both halves are trimmed; duration is "" when no comma is present.
func splitStatus(status string) (string, string) {
	if idx := strings.Index(status, ","); idx >= 0 {
		return strings.TrimSpace(status[:idx]), strings.TrimSpace(status[idx+1:])
	}
	return strings.TrimSpace(status), ""
}

// frameLocation captures the file/line pair on the indented line that
// follows a function call. Nil signals "no location to attach".
type frameLocation struct {
	File string
	Line int
}

// nextLocation peeks at body[start] and returns the (file, line) pair
// when it matches the indented `\tfile.go:NN +0xNN` pattern.
func nextLocation(body []string, start int) *frameLocation {
	if start >= len(body) {
		return nil
	}
	match := frameLocationRE.FindStringSubmatch(body[start])
	if match == nil {
		return nil
	}
	fileIdx := frameLocationRE.SubexpIndex("file")
	lineIdx := frameLocationRE.SubexpIndex("line")
	lineNo, err := strconv.Atoi(match[lineIdx])
	if err != nil {
		return nil
	}
	return &frameLocation{File: match[fileIdx], Line: lineNo}
}

// buildFrame mirrors Python `_build_frame`. Function text "pkg.Type.Method"
// or "pkg.func" is split at the last dot; everything before becomes the
// module, everything after the function. Argument lists are stripped at
// the first `(`.
func buildFrame(functionText string, location *frameLocation) models.StackFrame {
	function := functionText
	if openParen := strings.Index(functionText, "("); openParen > 0 {
		function = functionText[:openParen]
	}
	var modulePtr *string
	if dot := strings.LastIndex(function, "."); dot >= 0 {
		mod := function[:dot]
		function = function[dot+1:]
		modulePtr = &mod
	}
	lang := Language
	frame := models.StackFrame{
		Function: function,
		Module:   modulePtr,
		Language: &lang,
	}
	if location != nil {
		fileCopy := location.File
		lineCopy := location.Line
		frame.File = &fileCopy
		frame.Line = &lineCopy
	}
	return frame
}

// ---------------------------------------------------------------------
// T-197 — Go framework cleanup + state inference
// ---------------------------------------------------------------------

// goWrapperPattern is one entry in the framework-cleanup table. Pattern
// is matched against the qualified `module.function` string; when it
// matches the whole match is replaced with Replacement. Mirrors the
// Python `_GO_WRAPPER_PATTERNS` tuple.
type goWrapperPattern struct {
	Pattern     *regexp.Regexp
	Replacement string
}

// goWrapperPatterns mirrors Python `_GO_WRAPPER_PATTERNS`. The Python
// table uses `\1` backreferences; Go regexp expansion uses `$1`. The
// trailing chained-closure entry strips any number of `.funcN` suffixes
// so receiver-bearing handlers stay grouped together.
var goWrapperPatterns = []goWrapperPattern{
	{regexp.MustCompile(`^(gin\.HandlerFunc)\.func\d+`), `$1`},
	{regexp.MustCompile(`^(gin\.\(\*Engine\))\.handleHTTPRequest`), `$1.handleHTTPRequest`},
	{regexp.MustCompile(`^(echo\.\(\*Echo\))\.ServeHTTP`), `$1.ServeHTTP`},
	{regexp.MustCompile(`^(chi\.\(\*Mux\))\.routeHTTP`), `$1.routeHTTP`},
	{regexp.MustCompile(`^(fiber\.\(\*App\))\.Handler`), `$1.Handler`},
	// Anonymous closure suffix .func1.func2... → strip trailing
	// `.funcN` chain to keep the receiver visible.
	{regexp.MustCompile(`(.+?)(?:\.func\d+)+$`), `$1`},
}

// normalizeGoFrame mirrors Python `_normalize_go_frame`. Frames whose
// Language is not "go" are returned untouched. The qualified name is
// rebuilt as `module.function`, run through every wrapper pattern in
// order, then re-split if the result changed.
func normalizeGoFrame(frame models.StackFrame) models.StackFrame {
	if frame.Language == nil || *frame.Language != Language {
		return frame
	}
	qualified := frame.Function
	if frame.Module != nil && *frame.Module != "" {
		qualified = *frame.Module + "." + frame.Function
	}
	newQualified := qualified
	for _, wp := range goWrapperPatterns {
		newQualified = wp.Pattern.ReplaceAllString(newQualified, wp.Replacement)
	}
	if newQualified == qualified {
		return frame
	}
	out := frame
	if dot := strings.LastIndex(newQualified, "."); dot >= 0 {
		mod := newQualified[:dot]
		out.Function = newQualified[dot+1:]
		out.Module = &mod
	} else {
		out.Function = newQualified
		out.Module = nil
	}
	return out
}

// State-inference patterns mirror the Python `_GO_*_PATTERNS` regexes.
// Compiled with case-insensitive flags to match Python's `re.IGNORECASE`.
var (
	goNetworkPatternRE = regexp.MustCompile(
		`(?i)(?:netpollblock|runtime\.netpoll|net\.\(\*netFD\)\.Read|` +
			`net\.\(\*conn\)\.Read|net\.\(\*TCPConn\)\.Read|net\.\(\*UDPConn\)\.Read|` +
			`http\.\(\*persistConn\)\.readResponse)`,
	)
	goLockPatternRE = regexp.MustCompile(
		`(?i)(?:semacquire|sync\.\(\*Mutex\)\.Lock|sync\.\(\*RWMutex\)\.Lock|` +
			`sync\.runtime_Semacquire)`,
	)
	goChannelPatternRE = regexp.MustCompile(
		`(?i)(?:gopark|chanrecv|chansend|runtime\.selectgo|runtime\.chansend|` +
			`runtime\.chanrecv)`,
	)
	goIOPatternRE = regexp.MustCompile(
		`(?i)(?:os\.\(\*File\)\.Read|bufio\.\(\*Reader\)\.Read|` +
			`io\.copyBuffer|io\.\(\*pipe\)\.read)`,
	)
)

// inferGoState mirrors Python `_infer_go_state`. The original raw state
// is returned unchanged when there are no frames or no pattern matches
// the top frame's qualified name.
func inferGoState(state models.ThreadState, frames []models.StackFrame) models.ThreadState {
	if len(frames) == 0 {
		return state
	}
	top := frames[0]
	qualified := top.Function
	if top.Module != nil && *top.Module != "" {
		qualified = *top.Module + "." + top.Function
	}
	switch {
	case goNetworkPatternRE.MatchString(qualified):
		return models.ThreadStateNetworkWait
	case goLockPatternRE.MatchString(qualified):
		return models.ThreadStateLockWait
	case goChannelPatternRE.MatchString(qualified):
		return models.ThreadStateChannelWait
	case goIOPatternRE.MatchString(qualified):
		return models.ThreadStateIOWait
	}
	return state
}

// Compile-time assertion: Plugin satisfies threaddump.Plugin.
var _ threaddump.Plugin = (*Plugin)(nil)

func init() {
	threaddump.DefaultRegistry.Register(New())
}
