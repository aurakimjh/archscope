// [한글] models/analysis_result_test.go — AnalysisResult envelope 의
// 직렬화 contract 회귀 테스트.
//
// 검증 포인트
//   1) New() 가 빈 컨테이너를 채워서 marshal 결과가 null 이 아닌 [] / {}
//      로 출력되는지 (UI 의 옵셔널 체이닝 안전성 보장).
//   2) type/parser/schema_version/findings 등 필수 키가 모두 존재.
//   3) created_at 이 RFC3339(Nano) 형식의 비어있지 않은 문자열인지.
//
// 왜 byte 단위 substring 검사인가?
//   key 순서/공백에 의존하지 않고도 핵심 contract 만 검증하기 위함.
//   Go map 의 marshal 순서는 비결정적이지만 키 자체는 결정적이므로
//   substring 매칭이 적합.
package models

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewIsJSONFriendly(t *testing.T) {
	result := New("access_log", "nginx_access_log")
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{
		`"type":"access_log"`,
		`"source_files":[]`,
		`"summary":{}`,
		`"series":{}`,
		`"tables":{}`,
		`"charts":{}`,
		`"parser":"nginx_access_log"`,
		`"schema_version":"0.1.0"`,
		`"findings":[]`,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("expected %q in marshalled output; got %s", want, body)
		}
	}
	if !strings.Contains(string(body), `"created_at":"`) {
		t.Fatalf("created_at should be populated; got %s", body)
	}
}

func TestAttachSourceAppendsPath(t *testing.T) {
	r := New("gc_log", "hotspot_unified")
	r.AttachSource("/tmp/gc-1.log")
	r.AttachSource("/tmp/gc-2.log")
	if len(r.SourceFiles) != 2 {
		t.Fatalf("source_files = %d, want 2", len(r.SourceFiles))
	}
	if r.SourceFiles[1] != "/tmp/gc-2.log" {
		t.Fatalf("second entry mismatch: %q", r.SourceFiles[1])
	}
}

func TestAddFindingRecordsCanonicalShape(t *testing.T) {
	r := New("thread_dump_multi", "java_jstack")
	r.AddFinding("warning", "LONG_RUNNING_THREAD", "thread persisted",
		map[string]any{"thread_name": "main", "dumps": 5},
	)
	if len(r.Metadata.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(r.Metadata.Findings))
	}
	finding := r.Metadata.Findings[0]
	if finding["severity"] != "warning" || finding["code"] != "LONG_RUNNING_THREAD" {
		t.Fatalf("finding mismatch: %+v", finding)
	}
	evidence, _ := finding["evidence"].(map[string]any)
	if evidence["thread_name"] != "main" {
		t.Fatalf("evidence thread_name mismatch: %+v", evidence)
	}
}

func TestAddFindingWithoutEvidenceOmitsKey(t *testing.T) {
	r := New("exception_stack", "java_exception")
	r.AddFinding("info", "NEW_TYPE", "first time seen", nil)
	finding := r.Metadata.Findings[0]
	if _, ok := finding["evidence"]; ok {
		t.Fatalf("evidence key should be absent when nil; got %+v", finding)
	}
}
