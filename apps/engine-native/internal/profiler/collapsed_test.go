// [한글] collapsed_test — ParseCollapsedFile 의 정상/에러 라인 처리 검증.
// 동일 stack 의 중복 라인은 누적되어 sample 합산되는지, INVALID_SAMPLE_COUNT
// (whitespace 만 있는 라인) / NEGATIVE_SAMPLE_COUNT 진단이 정확히 카운트되는지 확인.

package profiler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCollapsedFileAggregatesAndReportsBadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wall.collapsed")
	err := os.WriteFile(path, []byte(
		"com.example.Job;com.example.Service.process 3\n"+
			"com.example.Job;com.example.Service.process 4\n"+
			"broken without count\n"+
			"com.example.Other -1\n",
	), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	stacks, diagnostics, err := ParseCollapsedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := stacks["com.example.Job;com.example.Service.process"]; got != 7 {
		t.Fatalf("aggregated samples = %d, want 7", got)
	}
	if diagnostics.ParsedRecords != 2 {
		t.Fatalf("parsed records = %d, want 2", diagnostics.ParsedRecords)
	}
	if diagnostics.SkippedByReason["INVALID_SAMPLE_COUNT"] != 1 {
		t.Fatalf("invalid sample count diagnostics missing: %#v", diagnostics.SkippedByReason)
	}
	if diagnostics.SkippedByReason["NEGATIVE_SAMPLE_COUNT"] != 1 {
		t.Fatalf("negative sample count diagnostics missing: %#v", diagnostics.SkippedByReason)
	}
}

func TestTruncateStackDepthPreservesTimelineBaseMethod(t *testing.T) {
	stack := "bootstrap;framework;com.example.OrderJob.execute;service;dao;jdbc;socket"
	got := truncateStackDepth(stack, 4, "OrderJob.execute")
	want := "com.example.OrderJob.execute;...truncated...;jdbc;socket"
	if got != want {
		t.Fatalf("truncated stack = %q, want %q", got, want)
	}
}

func TestTruncateStackDepthFallsBackToLeafTail(t *testing.T) {
	stack := "a;b;c;d;e"
	got := truncateStackDepth(stack, 3, "MissingBase")
	want := "...truncated;c;d;e"
	if got != want {
		t.Fatalf("truncated stack = %q, want %q", got, want)
	}
}
