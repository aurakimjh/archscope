package profiler

import "testing"

func TestAnalyzeCollapsedStacksTimelineBaseMethod(t *testing.T) {
	source := "fixture.collapsed"
	diagnostics := ParserDiagnostics{
		SourceFile:      &source,
		Format:          "async_profiler_collapsed",
		TotalLines:      3,
		ParsedRecords:   3,
		SkippedByReason: map[string]int{},
	}
	result := AnalyzeCollapsedStacks(
		map[string]int{
			"org.springframework.boot.SpringApplication.run;com.company.batch.OrderJob.execute;com.company.Service.process":        6,
			"org.springframework.boot.SpringApplication.run;com.company.batch.OrderJob.execute;oracle.jdbc.Statement.executeQuery": 4,
			"scheduler.IdleLoop.run;java.lang.Thread.sleep":                                                                        90,
		},
		source,
		diagnostics,
		Options{
			IntervalMS:         100,
			TopN:               5,
			ProfileKind:        "wall",
			TimelineBaseMethod: "OrderJob.execute",
		},
	)

	if result.Metadata.TimelineScope.BaseSamples != 10 {
		t.Fatalf("base samples = %d, want 10", result.Metadata.TimelineScope.BaseSamples)
	}
	if result.Metadata.TimelineScope.BaseRatioOfTotal == nil || *result.Metadata.TimelineScope.BaseRatioOfTotal != 10 {
		t.Fatalf("base ratio = %#v, want 10", result.Metadata.TimelineScope.BaseRatioOfTotal)
	}

	bySegment := map[string]TimelineRow{}
	for _, row := range result.Series.TimelineAnalysis {
		bySegment[row.Segment] = row
	}
	if _, ok := bySegment["STARTUP_FRAMEWORK"]; ok {
		t.Fatal("base-method timeline should be re-rooted and not include startup segment")
	}
	internal := bySegment["INTERNAL_METHOD"]
	if internal.Samples != 6 || internal.StageRatio != 60 || internal.TotalRatio != 6 {
		t.Fatalf("internal row = %#v, want samples=6 stage=60 total=6", internal)
	}
	sql := bySegment["SQL_EXECUTION"]
	if sql.Samples != 4 || sql.StageRatio != 40 || sql.TotalRatio != 4 {
		t.Fatalf("sql row = %#v, want samples=4 stage=40 total=4", sql)
	}
	if got := internal.TopStacks[0].Name; got != "com.company.batch.OrderJob.execute;com.company.Service.process" {
		t.Fatalf("top stack = %q, want re-rooted stack", got)
	}
}

func TestAnalyzeCollapsedStacksTimelineBaseMethodNotFound(t *testing.T) {
	source := "fixture.collapsed"
	result := AnalyzeCollapsedStacks(
		map[string]int{"com.company.OtherJob.execute;com.company.Service.process": 3},
		source,
		ParserDiagnostics{SourceFile: &source, Format: "async_profiler_collapsed", SkippedByReason: map[string]int{}},
		Options{IntervalMS: 100, TopN: 5, ProfileKind: "wall", TimelineBaseMethod: "MissingJob.execute"},
	)

	if len(result.Series.TimelineAnalysis) != 0 {
		t.Fatalf("timeline rows = %#v, want empty", result.Series.TimelineAnalysis)
	}
	warnings := result.Metadata.TimelineScope.Warnings
	if len(warnings) != 1 || warnings[0].Code != "TIMELINE_BASE_METHOD_NOT_FOUND" {
		t.Fatalf("warnings = %#v, want TIMELINE_BASE_METHOD_NOT_FOUND", warnings)
	}
}
