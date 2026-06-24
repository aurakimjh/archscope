// ─────────────────────────────────────────────────────────────────────
// [한글] analyzer_test — AnalyzeCollapsedStacks 의 timeline base-method 동작 검증.
//
// 두 케이스를 cover:
//   1) BaseMethod 매치되는 경우 — base 부터 재루팅한 stack 만 timeline 에
//      포함, base 외부의 startup segment 가 제외, base ratio/segment 비율이
//      재루팅된 분모로 계산되는지 확인.
//   2) BaseMethod 매치되지 않는 경우 — timeline 빈 배열 + scope.warnings
//      에 TIMELINE_BASE_METHOD_NOT_FOUND 코드 포함.
// ─────────────────────────────────────────────────────────────────────

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

func TestAnalyzeCollapsedStacksTimelineSeparatesDBFetch(t *testing.T) {
	source := "fixture.collapsed"
	result := AnalyzeCollapsedStacks(
		map[string]int{
			"com.company.Repository.query;oracle.jdbc.Statement.executeQuery":        3,
			"com.company.Repository.query;oracle.jdbc.driver.OracleResultSet.next":   5,
			"com.company.Repository.query;org.postgresql.jdbc.PgResultSet.fetchRows": 2,
			"com.company.Repository.query;com.company.Service.mapFetchedRows":        4,
		},
		source,
		ParserDiagnostics{SourceFile: &source, Format: "async_profiler_collapsed", SkippedByReason: map[string]int{}},
		Options{IntervalMS: 100, TopN: 5, ProfileKind: "wall"},
	)

	bySegment := map[string]TimelineRow{}
	for _, row := range result.Series.TimelineAnalysis {
		bySegment[row.Segment] = row
	}
	if got := bySegment["SQL_EXECUTION"].Samples; got != 3 {
		t.Fatalf("SQL_EXECUTION samples = %d, want 3", got)
	}
	if got := bySegment["DB_FETCH"].Samples; got != 7 {
		t.Fatalf("DB_FETCH samples = %d, want 7", got)
	}
	if got := bySegment["INTERNAL_METHOD"].Samples; got != 4 {
		t.Fatalf("INTERNAL_METHOD samples = %d, want 4", got)
	}
}

func TestAnalyzeCollapsedStacksTimelineSeparatesLoggingFromBusinessLogic(t *testing.T) {
	source := "fixture.collapsed"
	result := AnalyzeCollapsedStacks(
		map[string]int{
			"com.company.OrderService.calculate;com.company.PricingAlgorithm.apply":                                13,
			"com.company.OrderService.calculate;org.slf4j.Logger.info;ch.qos.logback.classic.Logger.callAppenders": 7,
		},
		source,
		ParserDiagnostics{SourceFile: &source, Format: "async_profiler_collapsed", SkippedByReason: map[string]int{}},
		Options{IntervalMS: 100, TopN: 5, ProfileKind: "wall"},
	)

	bySegment := map[string]TimelineRow{}
	for _, row := range result.Series.TimelineAnalysis {
		bySegment[row.Segment] = row
	}
	if got := bySegment["LOGGING"].Samples; got != 7 {
		t.Fatalf("LOGGING samples = %d, want 7", got)
	}
	if got := bySegment["INTERNAL_METHOD"].Samples; got != 13 {
		t.Fatalf("INTERNAL_METHOD samples = %d, want 13", got)
	}
	if got := bySegment["LOGGING"].StageRatio; got != 35 {
		t.Fatalf("LOGGING stage ratio = %v, want 35", got)
	}

	byCategory := map[string]ExecutionBreakdownRow{}
	for _, row := range result.Series.ExecutionBreakdown {
		byCategory[row.Category] = row
	}
	if got := byCategory["LOGGING"].Samples; got != 7 {
		t.Fatalf("LOGGING breakdown samples = %d, want 7", got)
	}
	if got := byCategory["APPLICATION_LOGIC"].Samples; got != 13 {
		t.Fatalf("APPLICATION_LOGIC breakdown samples = %d, want 13", got)
	}
}

func TestAnalyzeCollapsedStacksTimelineSeparatesDTOMappingFromBusinessLogicAndSQL(t *testing.T) {
	source := "fixture.collapsed"
	result := AnalyzeCollapsedStacks(
		map[string]int{
			"com.company.OrderService.calculate;com.company.PricingAlgorithm.apply":                                                                              11,
			"com.company.OrderService.calculate;com.company.OrderRequestDto.<init>":                                                                              3,
			"com.company.OrderRepository.find;org.apache.ibatis.executor.resultset.DefaultResultSetHandler.applyPropertyMappings;com.company.OrderDto.setAmount": 5,
			"com.company.OrderRepository.find;oracle.jdbc.Statement.executeQuery":                                                                                7,
			"com.company.OrderRepository.find;oracle.jdbc.driver.OracleResultSet.next":                                                                           4,
		},
		source,
		ParserDiagnostics{SourceFile: &source, Format: "async_profiler_collapsed", SkippedByReason: map[string]int{}},
		Options{IntervalMS: 100, TopN: 5, ProfileKind: "wall"},
	)

	bySegment := map[string]TimelineRow{}
	for _, row := range result.Series.TimelineAnalysis {
		bySegment[row.Segment] = row
	}
	if got := bySegment["INTERNAL_METHOD"].Samples; got != 11 {
		t.Fatalf("INTERNAL_METHOD samples = %d, want 11", got)
	}
	if got := bySegment["DTO_MAPPING"].Samples; got != 8 {
		t.Fatalf("DTO_MAPPING samples = %d, want 8", got)
	}
	if got := bySegment["SQL_EXECUTION"].Samples; got != 7 {
		t.Fatalf("SQL_EXECUTION samples = %d, want 7", got)
	}
	if got := bySegment["DB_FETCH"].Samples; got != 4 {
		t.Fatalf("DB_FETCH samples = %d, want 4", got)
	}

	byCategory := map[string]ExecutionBreakdownRow{}
	for _, row := range result.Series.ExecutionBreakdown {
		byCategory[row.Category] = row
	}
	if got := byCategory["DTO_MAPPING"].Samples; got != 8 {
		t.Fatalf("DTO_MAPPING breakdown samples = %d, want 8", got)
	}
	if got := byCategory["APPLICATION_LOGIC"].Samples; got != 11 {
		t.Fatalf("APPLICATION_LOGIC breakdown samples = %d, want 11", got)
	}
}

func TestAnalyzeCollapsedStacksTimelineSeparatesSpringFrameworkFromBusinessLogic(t *testing.T) {
	source := "fixture.collapsed"
	result := AnalyzeCollapsedStacks(
		map[string]int{
			"org.apache.catalina.core.ApplicationFilterChain.doFilter;org.springframework.web.servlet.DispatcherServlet.doDispatch":               6,
			"org.springframework.web.servlet.DispatcherServlet.doDispatch;com.company.OrderController.list;com.company.OrderService.calculate":    14,
			"org.springframework.boot.SpringApplication.run;org.springframework.batch.core.launch.support.SimpleJobLauncher.run":                  3,
			"com.company.OrderRepository.find;org.mybatis.spring.SqlSessionTemplate.selectList;org.apache.ibatis.executor.SimpleExecutor.doQuery": 5,
		},
		source,
		ParserDiagnostics{SourceFile: &source, Format: "async_profiler_collapsed", SkippedByReason: map[string]int{}},
		Options{IntervalMS: 100, TopN: 5, ProfileKind: "wall"},
	)

	bySegment := map[string]TimelineRow{}
	for _, row := range result.Series.TimelineAnalysis {
		bySegment[row.Segment] = row
	}
	if got := bySegment["FRAMEWORK_MIDDLEWARE"].Samples; got != 6 {
		t.Fatalf("FRAMEWORK_MIDDLEWARE samples = %d, want 6", got)
	}
	if got := bySegment["INTERNAL_METHOD"].Samples; got != 14 {
		t.Fatalf("INTERNAL_METHOD samples = %d, want 14", got)
	}
	if got := bySegment["STARTUP_FRAMEWORK"].Samples; got != 3 {
		t.Fatalf("STARTUP_FRAMEWORK samples = %d, want 3", got)
	}
	if got := bySegment["SQL_EXECUTION"].Samples; got != 5 {
		t.Fatalf("SQL_EXECUTION samples = %d, want 5", got)
	}
}
