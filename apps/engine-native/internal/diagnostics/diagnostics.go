// Package diagnostics ports archscope_engine.common.diagnostics —
// the ParserDiagnostics builder every parser uses to report total /
// parsed / skipped counts plus per-reason breakdowns and bounded
// sample lists. Same JSON shape as the Python implementation so
// cross-engine debug logs stay diff-able through the T-244 / T-390
// parity gate.
package diagnostics

import "strings"

const (
	// MaxDiagnosticSamples caps how many sample lines we keep per
	// kind (samples / warnings / errors) so a 50k-line input doesn't
	// blow the response payload.
	MaxDiagnosticSamples = 100
	// RawPreviewLimit truncates the raw line preview shipped with
	// each diagnostic sample.
	RawPreviewLimit = 200
)

// Sample is the per-incident record stored under
// `diagnostics.samples`/`warnings`/`errors`.
type Sample struct {
	LineNumber int    `json:"line_number"`
	Reason     string `json:"reason"`
	Message    string `json:"message"`
	RawPreview string `json:"raw_preview"`
}

// ParserDiagnostics mirrors the Python dataclass' JSON projection.
type ParserDiagnostics struct {
	SourceFile      *string        `json:"source_file"`
	Format          string         `json:"format"`
	TotalLines      int            `json:"total_lines"`
	ParsedRecords   int            `json:"parsed_records"`
	SkippedLines    int            `json:"skipped_lines"`
	SkippedByReason map[string]int `json:"skipped_by_reason"`
	Samples         []Sample       `json:"samples"`
	WarningCount    int            `json:"warning_count"`
	ErrorCount      int            `json:"error_count"`
	Warnings        []Sample       `json:"warnings"`
	Errors          []Sample       `json:"errors"`
}

// New constructs a fresh diagnostics builder with non-nil maps/slices
// so JSON serialization always emits `{}` and `[]` rather than `null`.
func New(format string) *ParserDiagnostics {
	return &ParserDiagnostics{
		Format:          format,
		SkippedByReason: map[string]int{},
		Samples:         []Sample{},
		Warnings:        []Sample{},
		Errors:          []Sample{},
	}
}

// SetSourceFile captures the absolute source path on the result so
// renderers can surface "where did this come from" without re-walking
// metadata.
func (d *ParserDiagnostics) SetSourceFile(path string) {
	d.SourceFile = &path
}

// AddSkipped records a row-level parser skip — increments the
// skipped counter, bumps the per-reason histogram, and appends to
// the bounded errors + samples lists.
func (d *ParserDiagnostics) AddSkipped(lineNumber int, reason, message, rawLine string) {
	d.SkippedLines++
	d.SkippedByReason[reason]++
	d.addIssue(lineNumber, reason, message, rawLine, "error", true)
}

// AddWarning records a non-fatal warning (e.g. ENCODING_FALLBACK,
// MISSING_PARENT). When `skipped=true` it also bumps the skip
// counter — Python's behaviour for things like CSV duplicate keys.
func (d *ParserDiagnostics) AddWarning(lineNumber int, reason, message, rawLine string, skipped bool) {
	if skipped {
		d.SkippedLines++
		d.SkippedByReason[reason]++
		d.addIssue(lineNumber, reason, message, rawLine, "warning", true)
		return
	}
	d.addIssue(lineNumber, reason, message, rawLine, "warning", true)
}

// AddError records a fatal-but-recoverable parser error without
// bumping the skip counter (e.g. INVALID_SVG when the whole file is
// rejected up front).
func (d *ParserDiagnostics) AddError(lineNumber int, reason, message, rawLine string) {
	d.addIssue(lineNumber, reason, message, rawLine, "error", true)
}

func (d *ParserDiagnostics) addIssue(lineNumber int, reason, message, rawLine, severity string, includeInSamples bool) {
	preview := rawLine
	if len(preview) > RawPreviewLimit {
		preview = preview[:RawPreviewLimit]
	}
	issue := Sample{
		LineNumber: lineNumber,
		Reason:     reason,
		Message:    strings.TrimSpace(message),
		RawPreview: preview,
	}
	if severity == "warning" {
		d.WarningCount++
		if len(d.Warnings) < MaxDiagnosticSamples {
			d.Warnings = append(d.Warnings, issue)
		}
	} else {
		d.ErrorCount++
		if len(d.Errors) < MaxDiagnosticSamples {
			d.Errors = append(d.Errors, issue)
		}
	}
	if includeInSamples && len(d.Samples) < MaxDiagnosticSamples {
		d.Samples = append(d.Samples, issue)
	}
}
