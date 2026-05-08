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
//
// ─────────────────────────────────────────────────────────────────────
// [한글] jenniferprofile parser — Jennifer APM Profile Export 파서.
//
// 7개 파일로 분리
//   parser.go           : 공개 진입점 ParseFile / ParseString.
//   splitter.go         : 파일을 Total Transaction 안내 + N개 TXID
//                         블록으로 분할.
//   header_parser.go    : 2-column key:value 헤더 파싱.
//   body_parser.go      : 본문 이벤트 라인 + START/END/TOTAL 파싱.
//   event_classifier.go : 이벤트 우선순위 기반 분류 (SQL / 2PC /
//                         FETCH / EXTERNAL_CALL / METHOD ...).
//   validator.go        : §10 FULL-Profile 검증 (TXID/GUID/Application
//                         /TIME 필드 누락 검출).
//
// 처리 흐름
//   1) ParseFile : textio.ReadAll 로 파일 전체 읽음 (인코딩 폴백).
//   2) splitTotalAndBlocks : `Total Transaction : N` 와 N개 TXID
//      블록으로 분할.
//   3) 각 블록에 대해:
//        a) parseHeader : 2-column key:value → JenniferProfileHeader.
//        b) parseBody   : 본문 이벤트 라인 → JenniferProfileBody.
//        c) classifyEvents : 이벤트 메시지를 우선순위 규칙으로 분류.
//        d) validateFullProfile : §10 강제 필드 검증.
//   4) 결과를 JenniferTransactionProfile + FileResult 로 반환.
//
// STRICT_MODE 기본값
//   §10 위반 = profile.Errors. STRICT_MODE 에서는 분석기가 그 GUID
//   그룹을 실패로 표시. 호출자가 비활성화하면 warning 으로 격하.
//
// MSA 단계 분리
//   본 파서는 MVP1 — 단일 트랜잭션 단위 파싱까지만. GUID 그룹핑 /
//   caller-callee 매칭 / 시그니처 통계 / 병렬도는 분석기 (analyzers/
//   jenniferprofile) 가 처리.
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
	// NetworkPrepPatterns are case-insensitive substrings that mark a
	// METHOD line as a "network prep" wrapper (e.g.
	// IntegrationUtil.sendToService). When the message contains any
	// of these, the event type becomes JenniferEventNetworkPrep and
	// the analyzer can subtract embedded EXTERNAL_CALL elapsed.
	// Empty means "use defaults".
	NetworkPrepPatterns []string
	// EventCategoryPatterns lets users extend the event classifier.
	// Keys are JenniferEventType values (e.g.
	// "EXTERNAL_CALL", "TWO_PC_UNKNOWN", "CHECK_QUERY",
	// "NETWORK_PREP_METHOD"); values are case-insensitive substrings
	// matched against the event message. The first matching rule
	// wins, evaluated AFTER the built-in classifier so the defaults
	// still bracket well-known forms.
	EventCategoryPatterns map[string][]string
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

	classifyEventsWithOptions(&profile, opts)
	calculateOffsets(&profile)
	validateFullProfile(&profile, opts)

	return profile
}
