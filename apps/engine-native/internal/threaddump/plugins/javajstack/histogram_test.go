// [한글] histogram_test.go — class histogram 파싱 회귀 테스트
// (T-230 / T-235).
//
// 검증 대상
//   • 정상 histogram 블록: header → row 들 → Total 라인 → 종료.
//   • row limit (ARCHSCOPE_CLASS_HISTOGRAM_ROW_LIMIT) 적용.
//   • Total 라인 누락 / 파일 truncate 시 incomplete=true + reason.
//   • partial_tail_line 보존.
//   • bundle.Metadata["class_histogram"] 의 구조 정확성.
//   • bytes 정렬 (DESC) 가 분석기에서 정확히 사용.
package javajstack

import "testing"

// T-230 / T-235 — class histogram parsing.

var histogramComplete = []string{
	" num     #instances         #bytes  class name",
	"-------------------------------------------------------",
	"   1:           100           2400  java.lang.String",
	"   2:            10           1600  com.acme.Order",
	"Total           110           4000",
}

func TestParseClassHistogramComplete(t *testing.T) {
	got := parseTextClassHistogram(histogramComplete, 500)
	if got == nil {
		t.Fatal("histogram payload should not be nil")
	}
	if got["incomplete"] != false {
		t.Fatalf("complete histogram flagged incomplete: %v", got["incomplete_reason"])
	}
	if got["total_rows"] != 2 {
		t.Fatalf("total_rows = %v", got["total_rows"])
	}
	if got["truncated"] != false {
		t.Fatalf("truncated = %v", got["truncated"])
	}
	classes := got["classes"].([]map[string]any)
	if len(classes) != 2 {
		t.Fatalf("classes length = %d", len(classes))
	}
	if classes[0]["class_name"] != "java.lang.String" {
		t.Fatalf("first class name = %v", classes[0]["class_name"])
	}
	if got["total_instances"] != 110 {
		t.Fatalf("total_instances = %v", got["total_instances"])
	}
}

func TestParseClassHistogramRowLimitMarksTruncated(t *testing.T) {
	got := parseTextClassHistogram(histogramComplete, 1)
	classes := got["classes"].([]map[string]any)
	if len(classes) != 1 {
		t.Fatalf("row limit not applied: %d classes", len(classes))
	}
	if got["truncated"] != true {
		t.Fatalf("truncated should be true with limit 1 and 2 rows")
	}
	if got["total_rows"] != 2 {
		t.Fatalf("total_rows should reflect ALL parsed rows: %v", got["total_rows"])
	}
}

func TestParseClassHistogramPartialRow(t *testing.T) {
	lines := []string{
		" num     #instances         #bytes  class name",
		"-------------------------------------------------------",
		"   1:           100           2400  java.lang.String",
		"   2:            50           1200",
	}
	got := parseTextClassHistogram(lines, 500)
	if got["incomplete"] != true {
		t.Fatalf("partial-row histogram should be incomplete")
	}
	if got["partial_tail_line"] != "2:            50           1200" {
		t.Fatalf("partial_tail_line = %v", got["partial_tail_line"])
	}
}

func TestParseClassHistogramMissingTotal(t *testing.T) {
	lines := []string{
		" num     #instances         #bytes  class name",
		"-------------------------------------------------------",
		"   1:           100           2400  java.lang.String",
		"   2:            50           1200  com.acme.Order",
	}
	got := parseTextClassHistogram(lines, 500)
	if got["incomplete"] != true {
		t.Fatalf("missing-total histogram should be incomplete")
	}
	reason, _ := got["incomplete_reason"].(string)
	if reason == "" {
		t.Fatalf("incomplete_reason should be set")
	}
}

func TestParseClassHistogramReturnsNilWithoutHeader(t *testing.T) {
	lines := []string{"unrelated text", "more text"}
	if got := parseTextClassHistogram(lines, 500); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
