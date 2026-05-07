// Package jenniferprofile parses Jennifer Profile Export files into
// JenniferTransactionProfile records — see specs §3, §7-§11 for the
// authoritative grammar.
//
// The parser is split across files for legibility:
//
//	parser.go             — public entry (ParseFile / ParseString)
//	splitter.go           — Total + TXID block splitter
//	header_parser.go      — 2-column key:value header
//	body_parser.go        — body event lines + START/END/TOTAL
//	event_classifier.go   — priority-ordered event-type classifier
//
// MVP1 covers Acceptance #1-#8: file/blocks → header → body → event
// classification (SQL / Check Query / 2PC / Fetch / External Call).
// MSA grouping, network-gap, signature stats, parallelism are layered
// on top in MVP2-MVP4.
package jenniferprofile

import (
	"fmt"
	"os"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

// Options tweaks parsing behaviour. The zero value behaves like
// STRICT_MODE = true (specs §10): a missing FULL-Profile field flags
// the profile, callers decide whether to discard the GUID group.
type Options struct {
	// FallbackCorrelationToTxid mirrors the spec's
	// `--fallback-correlation=txid` switch — when GUID is missing
	// we'd normally emit MISSING_GUID and skip MSA grouping; with
	// this flag a downstream MSA stage can use TXID instead.
	FallbackCorrelationToTxid bool
	// SourceFile is stamped onto every parsed profile so a multi-file
	// batch can attribute back to its origin without re-reading.
	SourceFile string
}

// FileResult is the per-file parse output.
type FileResult struct {
	SourceFile               string                              `json:"source_file"`
	DeclaredTransactionCount int                                 `json:"declared_transaction_count"`
	DetectedTransactionCount int                                 `json:"detected_transaction_count"`
	Profiles                 []models.JenniferTransactionProfile `json:"profiles"`
	FileErrors               []models.JenniferProfileIssue       `json:"file_errors,omitempty"`
}

// ParseFile reads a Jennifer profile export from disk, decodes it
// (UTF-8 → MS949 → EUC-KR fallback per §6.2), normalises line endings,
// then delegates to ParseString.
func ParseFile(path string, opts Options) (FileResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FileResult{SourceFile: path}, fmt.Errorf("read %s: %w", path, err)
	}
	enc, _ := textio.DetectFromBytes(raw, []string{"utf-8", "utf-8-sig", "cp949", "euc-kr", "latin-1"})
	if enc == "" {
		enc = "utf-8"
	}
	text, decodeErr := textio.DecodeBytes(raw, enc)
	if decodeErr != nil {
		return FileResult{
			SourceFile: path,
			FileErrors: []models.JenniferProfileIssue{
				{Code: "FILE_DECODE_ERROR", Message: decodeErr.Error()},
			},
		}, nil
	}
	if opts.SourceFile == "" {
		opts.SourceFile = path
	}
	return ParseString(text, opts), nil
}

// ParseString runs the parsing pipeline against an in-memory string —
// useful for tests and for streaming callers that already decoded
// the bytes.
func ParseString(text string, opts Options) FileResult {
	res := FileResult{SourceFile: opts.SourceFile}

	normalized := normalizeLineEndings(text)

	declared, declaredOK := parseTotalTransaction(normalized)
	res.DeclaredTransactionCount = declared
	if !declaredOK {
		res.FileErrors = append(res.FileErrors, models.JenniferProfileIssue{
			Code:    "TOTAL_TRANSACTION_NOT_FOUND",
			Message: "Total Transaction header not found",
		})
	}

	blocks := splitByTxid(normalized)
	res.DetectedTransactionCount = len(blocks)

	if declaredOK && declared != len(blocks) {
		res.FileErrors = append(res.FileErrors, models.JenniferProfileIssue{
			Code: "TRANSACTION_COUNT_MISMATCH",
			Message: fmt.Sprintf(
				"Total Transaction declared %d but found %d TXID blocks",
				declared, len(blocks),
			),
		})
	}

	for _, block := range blocks {
		profile := parseTransactionBlock(block, opts)
		res.Profiles = append(res.Profiles, profile)
	}

	return res
}

// normalizeLineEndings collapses \r\n / \r / \n into \n per §6.3.
func normalizeLineEndings(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

// parseTransactionBlock owns the per-block pipeline: header → body →
// event classification → FULL-Profile validation.
func parseTransactionBlock(block string, opts Options) models.JenniferTransactionProfile {
	profile := models.JenniferTransactionProfile{SourceFile: opts.SourceFile}

	headerText, bodyText, hasBodyHeader := splitHeaderBody(block)
	profile.Body.HasBodyHeader = hasBodyHeader

	parseHeader(headerText, &profile)

	if hasBodyHeader {
		parseBody(bodyText, &profile)
	}

	classifyEvents(&profile)
	calculateOffsets(&profile)
	validateFullProfile(&profile, opts)

	return profile
}
