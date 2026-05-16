package ingestion

import (
	"fmt"
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

const (
	FixtureValid         = "valid"
	FixturePartial       = "partial"
	FixtureMalformed     = "malformed"
	FixtureUnknownFormat = "unknown_format"
	FixtureLargeFile     = "large_file"
)

// FixtureObservation is the stable, parser-agnostic output a golden fixture
// test should compare. Parser-specific tests can add their own assertions
// around records after this common diagnostic check passes.
type FixtureObservation struct {
	Format      string
	Diagnostics *diagnostics.ParserDiagnostics
}

type FixtureExpectation struct {
	Kind                string
	WantFormat          string
	WantError           bool
	MinTotalLines       int
	MinParsedRecords    int
	MinSkippedLines     int
	MinWarningCount     int
	MinErrorCount       int
	WantSkippedByReason map[string]int
	WantWarningByReason map[string]int
	WantErrorByReason   map[string]int
}

// CheckFixture evaluates the shared diagnostic expectations for valid,
// partial, malformed, unknown-format, and large-file parser fixtures.
func CheckFixture(name string, observation FixtureObservation, parseErr error, want FixtureExpectation) []error {
	errs := []error{}
	if want.WantError && parseErr == nil {
		errs = append(errs, fmt.Errorf("%s: expected parser error", name))
	}
	if !want.WantError && parseErr != nil {
		errs = append(errs, fmt.Errorf("%s: unexpected parser error: %v", name, parseErr))
	}
	if want.WantFormat != "" && observation.Format != want.WantFormat {
		errs = append(errs, fmt.Errorf("%s: format = %q, want %q", name, observation.Format, want.WantFormat))
	}
	diags := observation.Diagnostics
	if diags == nil {
		return append(errs, fmt.Errorf("%s: diagnostics are required", name))
	}
	if diags.TotalLines < want.MinTotalLines {
		errs = append(errs, fmt.Errorf("%s: total_lines = %d, want >= %d", name, diags.TotalLines, want.MinTotalLines))
	}
	if diags.ParsedRecords < want.MinParsedRecords {
		errs = append(errs, fmt.Errorf("%s: parsed_records = %d, want >= %d", name, diags.ParsedRecords, want.MinParsedRecords))
	}
	if diags.SkippedLines < want.MinSkippedLines {
		errs = append(errs, fmt.Errorf("%s: skipped_lines = %d, want >= %d", name, diags.SkippedLines, want.MinSkippedLines))
	}
	if diags.WarningCount < want.MinWarningCount {
		errs = append(errs, fmt.Errorf("%s: warning_count = %d, want >= %d", name, diags.WarningCount, want.MinWarningCount))
	}
	if diags.ErrorCount < want.MinErrorCount {
		errs = append(errs, fmt.Errorf("%s: error_count = %d, want >= %d", name, diags.ErrorCount, want.MinErrorCount))
	}
	errs = append(errs, checkReasonCounts(name, "skipped", diags.SkippedByReason, want.WantSkippedByReason)...)
	errs = append(errs, checkReasonCounts(name, "warning", sampleReasonCounts(diags.Warnings), want.WantWarningByReason)...)
	errs = append(errs, checkReasonCounts(name, "error", sampleReasonCounts(diags.Errors), want.WantErrorByReason)...)
	return errs
}

func checkReasonCounts(name, label string, got, want map[string]int) []error {
	errs := []error{}
	for _, reason := range sortedReasonKeys(want) {
		if got[reason] < want[reason] {
			errs = append(errs, fmt.Errorf("%s: %s reason %s = %d, want >= %d", name, label, reason, got[reason], want[reason]))
		}
	}
	return errs
}

func sampleReasonCounts(samples []diagnostics.Sample) map[string]int {
	out := map[string]int{}
	for _, sample := range samples {
		out[sample.Reason]++
	}
	return out
}

func sortedReasonKeys(values map[string]int) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
