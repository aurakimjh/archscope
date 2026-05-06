package javajstack

import (
	"regexp"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// threadDumpRecord mirrors archscope_engine.models.thread_dump.ThreadDumpRecord.
// Internal type: callers outside the package see the structured
// ThreadSnapshot only.
type threadDumpRecord struct {
	ThreadName string
	ThreadID   string
	State      string
	Stack      []string
	LockInfo   string
	Category   string
	RawBlock   string
}

// jstackSection is one logical dump within a (possibly concatenated)
// jstack file. Mirrors Python's _JstackSection dataclass.
type jstackSection struct {
	Records      []threadDumpRecord
	RawLines     []string
	StartLine    int
	EndLine      int
	RawTimestamp string
}

var (
	threadHeaderRE = regexp.MustCompile(`^"(?P<name>[^"]+)"(?P<rest>.*)$`)
	tidRE          = regexp.MustCompile(`\b(?:tid|nid)=(?P<tid>0x[0-9a-fA-F]+|\S+)`)
	stateRE        = regexp.MustCompile(`java\.lang\.Thread\.State:\s+(?P<state>[A-Z_]+)`)
	timestampRE    = regexp.MustCompile(`\d{4}[-/]\d{2}[-/]\d{2}|\d{2}:\d{2}:\d{2}`)
)

// parseThreadDump reads the file and returns one record per "Foo" block.
// Mirrors Python's parse_thread_dump (without the diagnostics surface).
func parseThreadDump(path string) ([]threadDumpRecord, error) {
	lines, err := textio.IterTextLines(path, "")
	if err != nil {
		return nil, err
	}
	records := []threadDumpRecord{}
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		if rec := parseThreadBlock(current); rec != nil {
			records = append(records, *rec)
		}
		current = nil
	}
	for _, line := range lines {
		if threadHeaderRE.MatchString(line) {
			flush()
			current = []string{line}
			continue
		}
		if len(current) > 0 {
			current = append(current, line)
		}
	}
	flush()
	return records, nil
}

// parseThreadBlock parses a single quoted JVM thread block. Returns nil
// when the leading line does not match the quoted-name pattern. Mirrors
// Python's parse_thread_block.
func parseThreadBlock(block []string) *threadDumpRecord {
	if len(block) == 0 {
		return nil
	}
	header := threadHeaderRE.FindStringSubmatch(block[0])
	if header == nil {
		return nil
	}
	name := header[threadHeaderRE.SubexpIndex("name")]
	rest := header[threadHeaderRE.SubexpIndex("rest")]

	var state string
	stack := []string{}
	var lockInfo string
	for _, line := range block[1:] {
		stripped := strings.TrimSpace(line)
		if m := stateRE.FindStringSubmatch(stripped); m != nil {
			state = m[stateRE.SubexpIndex("state")]
			continue
		}
		if strings.HasPrefix(stripped, "at ") {
			stack = append(stack, stripped[3:])
			continue
		}
		// Lock-info heuristic — Python checks any of these tokens.
		for _, tok := range []string{"waiting to lock", "waiting on", "locked", "parking to wait"} {
			if strings.Contains(stripped, tok) {
				lockInfo = stripped
				break
			}
		}
	}

	if state == "" {
		state = stateFromHeader(rest)
	}

	threadID := ""
	if m := tidRE.FindStringSubmatch(block[0]); m != nil {
		threadID = m[tidRE.SubexpIndex("tid")]
	}

	return &threadDumpRecord{
		ThreadName: name,
		ThreadID:   threadID,
		State:      state,
		Stack:      stack,
		LockInfo:   lockInfo,
		Category:   categoryForState(state),
		RawBlock:   strings.Join(block, "\n"),
	}
}

// stateFromHeader infers a state string from the unstructured tail of
// the thread header. Mirrors Python's _state_from_header.
func stateFromHeader(rest string) string {
	text := strings.ToLower(rest)
	switch {
	case strings.Contains(text, "waiting for monitor entry") || strings.Contains(text, " blocked"):
		return "BLOCKED"
	case strings.Contains(text, "timed_waiting") || strings.Contains(text, "timed waiting"):
		return "TIMED_WAITING"
	case strings.Contains(text, "waiting on condition"),
		strings.Contains(text, "parking"),
		strings.Contains(text, "object.wait"):
		return "WAITING"
	case strings.Contains(text, "runnable") || strings.Contains(text, "running"):
		return "RUNNABLE"
	}
	return ""
}

// categoryForState mirrors Python's _category_for_state. Returns the
// short label the multi-thread analyzer groups records by.
func categoryForState(state string) string {
	switch state {
	case "RUNNABLE":
		return "RUNNABLE"
	case "BLOCKED":
		return "BLOCKED"
	case "WAITING", "TIMED_WAITING":
		return "WAITING"
	case "NEW":
		return "NEW"
	case "TERMINATED":
		return "TERMINATED"
	}
	return "UNKNOWN"
}

// splitJstackSections splits a possibly concatenated jstack file into
// per-dump sections. Mirrors Python's _split_jstack_sections.
func splitJstackSections(path string) ([]jstackSection, error) {
	lines, err := textio.IterTextLines(path, "")
	if err != nil {
		return nil, err
	}
	sections := []jstackSection{}
	var currentSection *jstackSection
	var currentBlock []string
	var prefixLines []string
	var recentLines []string

	flushBlock := func() {
		if currentSection == nil || len(currentBlock) == 0 {
			currentBlock = nil
			return
		}
		if rec := parseThreadBlock(currentBlock); rec != nil {
			currentSection.Records = append(currentSection.Records, *rec)
		}
		currentBlock = nil
	}
	flushSection := func() {
		if currentSection == nil {
			return
		}
		flushBlock()
		if len(currentSection.Records) > 0 {
			sections = append(sections, *currentSection)
		}
		currentSection = nil
	}
	startSection := func(lineNumber int, timestampLines []string) {
		if timestampLines == nil {
			timestampLines = prefixLines
		}
		currentSection = &jstackSection{
			StartLine:    lineNumber,
			EndLine:      lineNumber,
			RawTimestamp: extractRawTimestamp(timestampLines),
		}
	}
	remember := func(line string) {
		if strings.TrimSpace(line) == "" {
			return
		}
		recentLines = append(recentLines, line)
		if len(recentLines) > 8 {
			recentLines = recentLines[len(recentLines)-8:]
		}
	}

	for i, line := range lines {
		lineNumber := i + 1
		isFullHeader := fullThreadHeaderRE.MatchString(line)
		isThreadHeader := threadBlockHeaderRE.MatchString(line)

		if isFullHeader {
			timestampLines := append([]string(nil), recentLines...)
			flushSection()
			startSection(lineNumber, timestampLines)
			currentSection.RawLines = append(currentSection.RawLines, line)
			currentSection.EndLine = lineNumber
			prefixLines = nil
			remember(line)
			continue
		}

		if currentSection == nil {
			if isThreadHeader {
				startSection(lineNumber, nil)
				currentBlock = []string{line}
				currentSection.RawLines = append(currentSection.RawLines, line)
				currentSection.EndLine = lineNumber
				remember(line)
			} else if strings.TrimSpace(line) != "" {
				prefixLines = append(prefixLines, line)
				if len(prefixLines) > 8 {
					prefixLines = prefixLines[len(prefixLines)-8:]
				}
				remember(line)
			}
			continue
		}

		currentSection.RawLines = append(currentSection.RawLines, line)
		currentSection.EndLine = lineNumber
		if isThreadHeader {
			flushBlock()
			currentBlock = []string{line}
		} else if len(currentBlock) > 0 {
			currentBlock = append(currentBlock, line)
		}
		remember(line)
	}
	flushSection()
	return sections, nil
}

// extractRawTimestamp returns the most recent non-empty line that looks
// like a date/time. Iterates from newest to oldest.
func extractRawTimestamp(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		text := strings.TrimSpace(lines[i])
		if text == "" {
			continue
		}
		if timestampRE.MatchString(text) {
			return text
		}
	}
	return ""
}
