package profiler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// DebugLog captures a portable, redacted parser failure log that a field user
// can ship as a single artifact without sharing the original input.
//
// Mirrors archscope_engine.common.debug_log.DebugLogCollector — same JSON
// shape, same per-error-type sample cap, same 1 MiB output cap, same
// redact-by-default policy. Verdict / encoding metadata is decided at write
// time so callers don't have to know the policy.
type DebugLog struct {
	parser           string
	sourceFile       string
	encodingDetected string
	parseErrors      []DebugLogError
	exceptions       []DebugLogException
	hints            []string
	redactionSummary map[string]int
	startedAt        time.Time
	maxSamplesByType int
	maxBytes         int
}

// DebugLogError is a single parser failure record. `RawContext` keeps a small
// before/target/after window of the offending input region (already redacted)
// so the developer can reproduce the failure without the full file.
type DebugLogError struct {
	LineNumber    int               `json:"line_number"`
	Reason        string            `json:"reason"`
	Message       string            `json:"message"`
	FailedPattern string            `json:"failed_pattern,omitempty"`
	RawContext    map[string]string `json:"raw_context,omitempty"`
	FieldShapes   map[string]any    `json:"field_shapes,omitempty"`
}

// DebugLogException records a fatal parser exception (traceback / phase) so
// a crashing run still ships actionable evidence.
type DebugLogException struct {
	Phase     string `json:"phase"`
	Message   string `json:"message"`
	Traceback string `json:"traceback,omitempty"`
}

// NewDebugLog constructs a collector. `parser` should match the parser name
// recorded under `metadata.parser` in the AnalysisResult so the log can be
// correlated with the analyzer output.
func NewDebugLog(parser, sourceFile string) *DebugLog {
	return &DebugLog{
		parser:           parser,
		sourceFile:       sourceFile,
		parseErrors:      []DebugLogError{},
		exceptions:       []DebugLogException{},
		hints:            []string{},
		redactionSummary: map[string]int{},
		startedAt:        time.Now().UTC(),
		maxSamplesByType: 5,
		maxBytes:         1024 * 1024,
	}
}

// SetEncodingDetected records the text encoding the parser settled on.
func (l *DebugLog) SetEncodingDetected(encoding string) {
	if l == nil {
		return
	}
	l.encodingDetected = encoding
}

// AddHint adds a free-form developer hint shown next to the verdict.
func (l *DebugLog) AddHint(hint string) {
	if l == nil || strings.TrimSpace(hint) == "" {
		return
	}
	l.hints = append(l.hints, hint)
}

// AddParseError records a row-level parser failure. `rawContext` may include
// `before`, `target`, `after` strings; each is redacted in place. The total
// per-`reason` sample count is capped at `maxSamplesByType` so a failing
// 100k-line input does not bloat the log.
func (l *DebugLog) AddParseError(lineNumber int, reason, message, failedPattern string, rawContext map[string]string, fieldShapes map[string]any) {
	if l == nil {
		return
	}
	count := 0
	for _, existing := range l.parseErrors {
		if existing.Reason == reason {
			count++
		}
	}
	if count >= l.maxSamplesByType {
		return
	}
	redactedContext := map[string]string{}
	for key, value := range rawContext {
		result := RedactText(value)
		redactedContext[key] = result.Text
		l.redactionSummary = MergeRedactionSummaries(l.redactionSummary, result.Summary)
	}
	l.parseErrors = append(l.parseErrors, DebugLogError{
		LineNumber:    lineNumber,
		Reason:        reason,
		Message:       message,
		FailedPattern: failedPattern,
		RawContext:    redactedContext,
		FieldShapes:   fieldShapes,
	})
}

// AddException records a fatal exception during analysis.
func (l *DebugLog) AddException(phase, message, traceback string) {
	if l == nil {
		return
	}
	l.exceptions = append(l.exceptions, DebugLogException{
		Phase:     phase,
		Message:   message,
		Traceback: traceback,
	})
}

// HasContent reports whether the collector picked up anything worth writing.
func (l *DebugLog) HasContent() bool {
	if l == nil {
		return false
	}
	return len(l.parseErrors) > 0 || len(l.exceptions) > 0
}

// Verdict ranks the log: FATAL_ERROR if exceptions exist, PARSE_ISSUES if
// parse errors are present, CLEAN otherwise.
func (l *DebugLog) Verdict() string {
	if l == nil {
		return "CLEAN"
	}
	if len(l.exceptions) > 0 {
		return "FATAL_ERROR"
	}
	if len(l.parseErrors) > 0 {
		return "PARSE_ISSUES"
	}
	return "CLEAN"
}

// PortableFilename returns a deterministic file name suitable for shipping.
func (l *DebugLog) PortableFilename() string {
	stem := "archscope-debug"
	if l != nil && l.parser != "" {
		stem = stem + "-" + sanitizeFilenameComponent(l.parser)
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	return fmt.Sprintf("%s-%s.json", stem, stamp)
}

func sanitizeFilenameComponent(value string) string {
	out := []rune{}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// WriteJSON serializes the collected log into the configured directory. If
// `dir` is empty, defaults to `<execution-cwd>/archscope-debug/`. Returns the
// final output path. The payload is JSON-encoded and capped at `maxBytes`;
// when the cap kicks in the most recent samples are kept and `truncated:
// true` is emitted in the metadata.
func (l *DebugLog) WriteJSON(dir string) (string, error) {
	if l == nil {
		return "", fmt.Errorf("debug log not initialized")
	}
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(cwd, "archscope-debug")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, l.PortableFilename())

	payload := l.payload(false)
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	if len(body) > l.maxBytes {
		// Fall back to a truncated payload — drop oldest samples first.
		payload = l.payload(true)
		body, err = json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (l *DebugLog) payload(truncated bool) map[string]any {
	parseErrors := l.parseErrors
	if truncated && len(parseErrors) > l.maxSamplesByType*4 {
		parseErrors = parseErrors[len(parseErrors)-l.maxSamplesByType*4:]
	}
	reasons := map[string]int{}
	for _, err := range parseErrors {
		reasons[err.Reason]++
	}
	type reasonRow struct {
		Reason string `json:"reason"`
		Count  int    `json:"count"`
	}
	rows := []reasonRow{}
	for reason, count := range reasons {
		rows = append(rows, reasonRow{Reason: reason, Count: count})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
	return map[string]any{
		"version":           "0.1.0",
		"verdict":           l.Verdict(),
		"truncated":         truncated,
		"parser":            l.parser,
		"source_file":       sanitizeFilenameComponent(filepath.Base(l.sourceFile)),
		"encoding_detected": l.encodingDetected,
		"started_at":        l.startedAt.Format(time.RFC3339),
		"finished_at":       time.Now().UTC().Format(time.RFC3339),
		"environment": map[string]string{
			"go_version": runtime.Version(),
			"goos":       runtime.GOOS,
			"goarch":     runtime.GOARCH,
		},
		"redaction_version": RedactionVersion,
		"redaction_summary": l.redactionSummary,
		"reason_summary":    rows,
		"parse_errors":      parseErrors,
		"exceptions":        l.exceptions,
		"hints":             l.hints,
	}
}
