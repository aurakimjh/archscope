// Package models defines the AnalysisResult envelope every analyzer
// emits. The shape is the engine ↔ UI contract; all parsers /
// analyzers feed the same struct so the renderer can render any
// result type uniformly.
//
// The profiler-track Go binary already has its own AnalysisResult
// (apps/profiler-native/internal/profiler) tuned to the profiler
// shape — this package generalises that shape so Access Log / GC
// Log / Thread Dump analyzers can also use it. We keep the JSON
// keys identical to the Python implementation so the T-244 / T-390
// parity job can compare outputs byte-for-byte.
package models

import (
	"encoding/json"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

// AnalysisResult is the universal envelope. Each analyzer populates
// only the fields it needs; the rest stay as their zero value.
//
// `Series` / `Tables` / `Charts` / `Summary` are kept as
// `map[string]any` rather than typed structs because the Python
// implementation projects different shapes per analyzer (access log
// = per-minute timeline; profiler = flame tree; thread dump = state
// distribution). The renderer uses the `Type` field to decide which
// keys to look for; the parity job just diffs the JSON.
type AnalysisResult struct {
	Type        string         `json:"type"`
	SourceFiles []string       `json:"source_files"`
	CreatedAt   string         `json:"created_at"`
	Summary     map[string]any `json:"summary"`
	Series      map[string]any `json:"series"`
	Tables      map[string]any `json:"tables"`
	Charts      map[string]any `json:"charts"`
	Metadata    Metadata       `json:"metadata"`
}

// Metadata carries per-analyzer plumbing (parser name, schema
// version, parser diagnostics, findings list, AI interpretation
// payload, etc.). `Findings` and `Extra` stay open-ended for the
// same reason the section dicts above do.
type Metadata struct {
	Parser        string                         `json:"parser"`
	SchemaVersion string                         `json:"schema_version"`
	Diagnostics   *diagnostics.ParserDiagnostics `json:"diagnostics,omitempty"`
	// Findings is always emitted (even when empty) so the renderer
	// can rely on `metadata.findings.length` without null-checking.
	Findings []map[string]any `json:"findings"`
	// Extra carries analyzer-specific metadata that doesn't fit any
	// of the above (e.g. profiler timeline_scope, JFR command info,
	// OTel cross-service paths). Marshalled inline.
	Extra map[string]any `json:"-"`
}

// New constructs an envelope with non-nil section maps + a fresh
// RFC3339 timestamp so `json.Marshal` always emits `{}` rather than
// `null`. Mirrors Python's `AnalysisResult.__init__` defaults.
func New(resultType, parser string) AnalysisResult {
	return AnalysisResult{
		Type:        resultType,
		SourceFiles: []string{},
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		Summary:     map[string]any{},
		Series:      map[string]any{},
		Tables:      map[string]any{},
		Charts:      map[string]any{},
		Metadata: Metadata{
			Parser:        parser,
			SchemaVersion: "0.1.0",
			Findings:      []map[string]any{},
			Extra:         map[string]any{},
		},
	}
}

// AttachSource appends a path to `SourceFiles`.
func (r *AnalysisResult) AttachSource(path string) {
	r.SourceFiles = append(r.SourceFiles, path)
}

// AddFinding appends a finding row in the canonical
// {severity, code, message, evidence?} shape.
func (r *AnalysisResult) AddFinding(severity, code, message string, evidence map[string]any) {
	row := map[string]any{
		"severity": severity,
		"code":     code,
		"message":  message,
	}
	if evidence != nil {
		row["evidence"] = evidence
	}
	r.Metadata.Findings = append(r.Metadata.Findings, row)
}

// MarshalJSON merges the typed fields with `Extra` so analyzer-specific
// keys (access_log: format / analysis_options; profiler:
// timeline_scope; etc.) appear at the same level as `parser` /
// `schema_version`, matching the Python `metadata` dict shape.
//
// Extra never overrides typed fields — if the analyzer accidentally
// writes "parser" into Extra, the typed Parser still wins.
func (m Metadata) MarshalJSON() ([]byte, error) {
	out := make(map[string]any, len(m.Extra)+4)
	for k, v := range m.Extra {
		out[k] = v
	}
	out["parser"] = m.Parser
	out["schema_version"] = m.SchemaVersion
	if m.Diagnostics != nil {
		out["diagnostics"] = m.Diagnostics
	}
	findings := m.Findings
	if findings == nil {
		findings = []map[string]any{}
	}
	out["findings"] = findings
	return json.Marshal(out)
}
