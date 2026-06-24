import { useI18n } from "@/i18n/I18nProvider";
import type { Locale } from "@/i18n/messages";

const en = {
  helpTrigger: "Show help",
  warningTrigger: "Show warning details",
  genericCard:
    "This section groups the controls, results, or details for {title}. Use it to inspect that part of the current analysis.",
  genericMetric:
    "{label} is a key number calculated from the current analysis result. Use it as a quick signal before opening the detailed tables.",
  genericChart:
    "{title} visualizes the selected analysis data. Hover the chart itself for exact values when the chart supports it.",
  genericTable:
    "This table shows detailed rows behind the summary. Sort columns where available and use filters above the table to narrow the result.",
  fileDock:
    "Choose or drop the input file for this analyzer. Desktop builds pass the local file path to the engine, so large files are read directly from disk.",
  recentFiles:
    "Recently analyzed files for this analyzer. Select one to restore its path, or remove entries you no longer need.",
  analyzerOptions:
    "Analyzer options change how the engine parses, filters, or limits the input before results are produced. Adjust them before running analysis.",
  analyzerFeedback:
    "Shows the analyzer request status, engine messages, warnings, and diagnostics returned with the latest run.",
  aiFindings:
    "AI-assisted findings summarize evidence from the analysis result. Review the evidence references before using a finding in a report.",
  themeControl:
    "Switches the application theme between light, dark, and the operating system setting.",
  localeControl:
    "Switches the interface language. Analysis data and imported file contents are not translated.",
  sidebarAnalysis:
    "Analysis tools open raw profiler, log, trace, exception, and dump files and produce diagnostic results.",
  sidebarWorkspace:
    "Workspace tools reuse saved analysis results for reports, comparisons, timelines, SLO review, service flow, and evidence packs.",
  aboutApp:
    "Opens version, build, license, repository, and release note information for this desktop build.",
  pageProfiler:
    "Profiles collapsed stacks, Jennifer exports, and flamegraph files. Use it to inspect hot paths, timelines, drill-down filters, and parser diagnostics.",
  pageDiff:
    "Compares two profiler inputs to find paths that increased, decreased, appeared, or disappeared between runs.",
  pageAccessLog:
    "Analyzes web access logs for request volume, latency percentiles, errors, status distribution, and URL-level behavior.",
  pageGcLog:
    "Analyzes JVM GC logs for pause time, throughput, allocation or promotion pressure, heap movement, warnings, and JVM metadata.",
  pageJfr:
    "Analyzes JFR recordings or exported JFR JSON by event mode, stack profile, event tables, heatmap, and native memory leak candidates.",
  pageException:
    "Parses stack trace logs and groups exceptions by type, signature, root cause, event details, and analyzer findings.",
  pageThreadDump:
    "Analyzes thread dumps in single, multi-dump, or lock-contention modes to find blocked, long-running, and deadlocked threads.",
  pageMsaProfile:
    "Analyzes Jennifer MSA profile exports to reconstruct transaction calls, SQL timing, network preparation time, missing profiles, and service edges.",
  pageTraceImport:
    "Imports trace exports and derives services, spans, dependencies, latency, critical paths, and high-error service edges.",
  pageAnalysisWorkspace:
    "Stores completed analyzer results for later reuse. Select a result here before exporting, comparing, charting, or building evidence.",
  pageExportCenter:
    "Exports the selected workspace result as report files or raw data. Pick the result, format, and output path before exporting.",
  pageReportDiff:
    "Compares two saved AnalysisResult JSON files and highlights metric and finding changes across analyzer runs.",
  pageChartStudio:
    "Builds reusable charts from compatible workspace results. Select a result, chart template, and renderer to preview or save charts.",
  pageIncidentTimeline:
    "Normalizes events from saved results into a time-ordered incident narrative with severity, analyzer, group, and source filters.",
  pageSloGoldenSignals:
    "Normalizes Golden Signals and SLI data from saved results to review SLO violations, affected scopes, and signal inventory.",
  pageServiceFlow:
    "Combines trace and MSA evidence into service edges, unmatched calls, error counts, and service-flow findings.",
  pageEvidenceBoard:
    "Collects selected findings, rows, and AI evidence into report-ready cards that can be exported as HTML or JSON.",
  pageSettings:
    "Configures default analyzer options, language, theme, recent files, and application information.",
  sectionSummary:
    "The high-level numbers for the latest result. Use these cards to decide where to inspect next.",
  sectionFindings:
    "Findings are analyzer-generated observations that point to performance, reliability, parsing, or data-quality issues.",
  sectionFlamegraph:
    "The flamegraph aggregates stack samples by call path. Click frames to zoom and reset to return to the full profile.",
  sectionTopStacks:
    "Top stacks rank the most frequent or most expensive call paths in the current profile.",
  sectionTopChildFrames:
    "Child frames show which callees contribute most under the current selected or inferred timeline scope.",
  sectionTimelineScope:
    "Timeline scope explains which base method and matching mode were used to build the execution timeline.",
  sectionTimeline:
    "The execution timeline breaks profile samples into ordered work segments such as business algorithm work, Java framework or middleware time, logging, database calls, and external calls.",
  sectionTimelineEvidence:
    "Timeline evidence lists the frames and ratios that support each execution segment.",
  sectionBreakdown:
    "Breakdown views group elapsed time or samples by category so you can see which class of work dominates.",
  sectionDiagnostics:
    "Parser diagnostics list skipped lines, warnings, errors, and representative raw samples that explain parser confidence.",
  sectionFilters:
    "Filters narrow the current result without re-running the analyzer. Clear them to return to the full view.",
  sectionUrls:
    "URL rows aggregate requests by method and URI so slow or error-heavy endpoints stand out.",
  sectionStatusFamilies:
    "Status family rows group responses into 2xx, 3xx, 4xx, and 5xx classes for a quick health view.",
  sectionStatusCodes:
    "Status code rows show the most common exact HTTP response codes.",
  sectionErrorTimeline:
    "Error timeline rows show when error rates peaked so you can correlate failures with deployments or incidents.",
  sectionGcEvents:
    "GC events list individual pauses and heap changes parsed from the log.",
  sectionGcAlerts:
    "GC alerts highlight notable pause, allocation, OOM, or collector-behavior risks detected by the analyzer.",
  sectionJvmInfo:
    "JVM information summarizes runtime, heap, worker, flag, and command-line metadata captured in the GC log.",
  sectionJfrContract:
    "JFR scope explains which event modes and fields were available in the selected recording or export.",
  sectionJfrNativeMemory:
    "Native memory analysis matches allocation and free events, then reports allocations that remain unfreed near the end of the recording.",
  sectionJfrTopSites:
    "Top sites rank native memory call sites by unfreed bytes or events.",
  sectionJfrEvents:
    "JFR events list individual records after mode, state, time-window, and top-N filtering.",
  sectionJfrBreakdown:
    "Event breakdown groups JFR events by event type so dense categories are visible.",
  sectionJfrHeatmap:
    "The heatmap groups wall-clock events into time buckets. Click a bucket to apply it as the analysis start time.",
  sectionExceptionEvents:
    "Exception events show parsed occurrences and open a detail panel for full message, stack, signature, and root cause.",
  sectionExceptionTypes:
    "Exception type rows group events by class or language-specific error type.",
  sectionExceptionSignatures:
    "Signatures group exceptions by type and representative stack shape to reduce duplicate noise.",
  sectionThreadMode:
    "Thread dump mode decides whether one dump, multiple dumps, or lock metadata is analyzed.",
  sectionThreadSelected:
    "Selected dumps are the files that will be analyzed in order. Multi-dump mode uses their sequence to detect persistence.",
  sectionThreadSignatures:
    "Thread signatures group similar stacks so repeated blocked or long-running patterns are easier to see.",
  sectionThreadPerDump:
    "Per-dump rows show source file metadata and thread counts for each imported dump.",
  sectionThreadRows:
    "Thread rows show parsed thread state, top frame, locks, category, and source dump details.",
  sectionThreadDeadlock:
    "Deadlock cycles show lock ownership and wait relationships that form a circular dependency.",
  sectionTraceDependencies:
    "Service dependencies show caller-to-callee edges derived from spans.",
  sectionTraceServices:
    "Service latency summarizes duration and error behavior by service.",
  sectionWorkspaceCard:
    "A workspace result card represents one completed analyzer run. Use it to activate, export, compare, or extract evidence.",
  sectionExportPreview:
    "The selected result preview confirms which analyzer output will be exported.",
  sectionReportMetricDeltas:
    "Metric deltas compare numeric summary fields between the two selected AnalysisResult files.",
  sectionIncidentGroups:
    "Groups cluster timeline events by correlation key or analyzer context.",
  sectionIncidentNarrative:
    "The narrative converts timeline events into ordered incident steps.",
  sectionIncidentTimeline:
    "Timeline rows show event time, source, severity, correlation, and evidence references.",
  sectionSloViolations:
    "SLO violations list metrics whose actual values missed target or threshold expectations.",
  sectionSliMetrics:
    "SLI metrics list normalized service-level indicators from the selected workspace results.",
  sectionSloAffected:
    "Affected scopes summarize which services, endpoints, or groups contributed to SLO risk.",
  sectionSignalInventory:
    "Signal inventory shows which Golden Signals were found and from which analyzer source.",
  sectionServiceFindings:
    "Service-flow findings highlight risky edges such as errors, latency, missing spans, or unmatched external calls.",
  sectionServiceEdges:
    "Service edges show caller, callee, counts, latency, errors, and supporting sources.",
  sectionEvidenceCard:
    "Evidence cards preserve important findings or rows with source metadata for report export.",
  sectionMsaSingleBaseline:
    "Shows the exact call order and elapsed values for one selected transaction GUID.",
  sectionMsaCaptureStatus:
    "Summarizes whether 2PC, check-query, external-call, network-preparation, and connection-acquire evidence was captured.",
  sectionMsaIncomplete:
    "Warns that some transaction profiles are incomplete, which can reduce confidence in reconstructed edges or timing.",
  sectionMsaTransactions:
    "Transaction profile rows list parsed profiles, validation metadata, and per-profile timing fields.",
  sectionMsaNetworkPrep:
    "Network preparation methods show wrapper time after subtracting nested external call time.",
  sectionMsaMethodResidual:
    "Method residual time is not a generic slow-method ranking. It shows METHOD-frame response time left after subtracting identified child work such as SQL, external calls, fetch, Network Prep, connection acquire, and user-defined method rules. Its total is meant to be compared with Method Time in the response-time breakdown, but can differ because the two views use different scopes, interval reconstruction, overlap handling, and row limits.",
  sectionMsaUnprofiledExternal:
    "Unprofiled external calls are calls where the caller exists but the matching callee profile was not captured.",
  sectionMsaExternalTop:
    "Aggregates API calls by URL with call count, cumulative API time, max, p95, and network timing.",
  sectionMsaSlowSql:
    "Slow SQL candidates highlight queries whose response time exceeds the current threshold.",
  sectionMsaCustomCards:
    "Custom card statistics aggregate rows matched by analyzer option patterns.",
  sectionMsaServiceNetworkTime:
    "Service-call network time groups matched calls by caller and callee URL, then shows call count, callee cumulative/max time, and network cumulative/avg/max time.",
  sectionMsaFileErrors:
    "File errors list inputs that could not be parsed or had serious validation issues.",
  sectionMsaFileSummary:
    "File summary shows profile counts, errors, and warnings per imported file.",
  sectionMsaSignatures:
    "MSA signatures group similar transaction shapes for average-mode analysis.",
  sectionMsaEdgeStats:
    "Edge statistics aggregate caller-to-callee timings, counts, and matching quality.",
  sectionMsaGuidGroups:
    "GUID groups list reconstructed transaction groups and their execution metadata.",
  sectionMsaExternalEdges:
    "External call edges show service boundaries detected from Jennifer profile rows.",
  sectionMsaTimelineMode:
    "Timeline mode chooses one real GUID or an averaged signature before rendering the MSA timeline.",
  optionMsaTimelineGuid:
    "Selects the concrete transaction GUID used for single-transaction MSA timeline rendering.",
  optionMsaTimelineSignature:
    "Selects the signature group used to average multiple similar transactions into one MSA timeline.",
  sectionMsaParserReport:
    "Parser report summarizes imported files, parsed profiles, profile completeness, and parser warnings.",
  sectionMsaProfileIssues:
    "Profile issues list errors and warnings attached to individual parsed profiles.",
  optionFormat:
    "Selects which file format the analyzer should expect. Wrong formats usually produce parser warnings or empty results.",
  optionProfileKind:
    "Controls whether collapsed stacks are interpreted as CPU, wall-clock, allocation, lock, or another profile kind.",
  optionInterval:
    "Sample interval converts sample counts into estimated elapsed time. Use the profiler's real interval when known.",
  optionElapsed:
    "Optional total elapsed time override. Leave it empty when the analyzer should infer duration from samples and interval.",
  optionTopN:
    "Limits ranked lists and tables to the most relevant rows. Higher values show more detail but can add noise.",
  optionTimelineBase:
    "Restricts timeline analysis to a method or frame pattern that represents the transaction boundary.",
  optionMemoryGuards:
    "Memory guards cap unique stacks, stack depth, RSS, and debug output so very large inputs remain responsive.",
  optionMaxLines:
    "Caps how many log lines the engine reads. Increase it for complete results on large files.",
  optionStartTime:
    "Optional lower time bound. Rows before this time are ignored when the parser can read timestamps.",
  optionEndTime:
    "Optional upper time bound. Rows after this time are ignored when the parser can read timestamps.",
  optionStrict:
    "Strict mode treats ambiguous GC log lines more conservatively and may report more parser issues.",
  optionJfrMode:
    "Filters JFR records by analysis mode such as CPU, wall-clock, allocation, lock, GC, exception, or I/O.",
  optionJfrTime:
    "Limits the JFR analysis window. ISO time, clock time, and relative offsets are supported where the parser can resolve them.",
  optionJfrState:
    "Filters JFR stack samples by thread state so runnable, blocked, or waiting behavior can be isolated.",
  optionNativeLeakOnly:
    "When enabled, matched allocation/free pairs are removed so only likely unfreed native memory remains.",
  optionNativeTail:
    "Ignores allocations near the end of the recording, where short-lived allocations may not have had time to free.",
  optionThreadThreshold:
    "Controls how many consecutive dumps a thread must appear in before it is reported as persistent.",
  optionThreadFormat:
    "Overrides automatic dump format detection. Use it only when auto-detection chooses the wrong parser.",
  optionNormalize:
    "Normalizes profiler diff totals before comparison so files with different sample counts are easier to compare.",
  optionWorkspaceResult:
    "Selects which saved analyzer result this tool should use as input.",
  optionExportFormat:
    "Selects the output format. Report formats are human-readable; JSON and CSV formats preserve structured data.",
  optionOutputPath:
    "Sets the file or directory destination. Leave it empty to choose during export.",
  optionChartTemplate:
    "Chooses which chart definition to build from the selected workspace result.",
  optionRenderer:
    "Canvas is faster for dense charts; SVG is easier to inspect and can be sharper for simple charts.",
  optionSearch:
    "Search filters the current table or timeline by label, source, category, service, or evidence text.",
  optionAnalyzer:
    "Filters rows by the analyzer that produced the event or evidence.",
  optionSeverity:
    "Filters rows by severity so critical or warning items can be reviewed first.",
  optionGroup:
    "Filters timeline events by correlation group.",
  optionSignalKind:
    "Filters SLO rows by Golden Signal kind such as latency, traffic, errors, or saturation.",
  optionDrillPattern:
    "The frame pattern used by the drill-down filter. Use a substring or regular expression depending on filter type.",
  optionDrillFilterType:
    "Choose include, exclude, regex include, or regex exclude to decide which stack paths remain.",
  optionDrillMatchMode:
    "Controls whether matches can appear anywhere or must appear in the given order.",
  optionDrillViewMode:
    "Controls whether matching paths keep their original ancestry or re-root around the match.",
  optionCaseSensitive:
    "When enabled, uppercase and lowercase letters must match exactly.",
  settingsDefaults:
    "Default values are used when analyzer pages initialize new options.",
  settingsRecent:
    "Recent files are stored locally for quick reuse and can be cleared here.",
  settingsAbout:
    "Application metadata for this installed build.",
} as const;

export type HelpKey = keyof typeof en;

const ko: Record<HelpKey, string> = {
  helpTrigger: "도움말 보기",
  warningTrigger: "경고 상세 보기",
  genericCard:
    "{title} 영역의 설정, 결과, 세부 정보를 모아 둔 섹션입니다. 현재 분석에서 이 부분이 무엇을 의미하는지 확인할 때 사용합니다.",
  genericMetric:
    "{label} 값은 현재 분석 결과에서 계산된 핵심 지표입니다. 상세 테이블을 열기 전에 빠르게 상태를 판단하는 신호로 사용합니다.",
  genericChart:
    "{title} 차트는 선택된 분석 데이터를 시각화합니다. 차트가 지원하는 경우 마우스를 올려 정확한 값을 확인할 수 있습니다.",
  genericTable:
    "요약 뒤의 상세 행을 보여주는 테이블입니다. 가능한 컬럼은 정렬하고, 위쪽 필터로 결과 범위를 좁힐 수 있습니다.",
  fileDock:
    "이 분석기에 넣을 파일을 선택하거나 드롭합니다. 데스크톱 빌드는 로컬 파일 경로를 엔진에 전달하므로 큰 파일도 디스크에서 직접 읽습니다.",
  recentFiles:
    "이 분석기에서 최근 사용한 파일 목록입니다. 항목을 선택하면 경로를 다시 불러오고, 필요 없는 항목은 삭제할 수 있습니다.",
  analyzerOptions:
    "분석 옵션은 엔진이 입력을 파싱, 필터링, 제한하는 방식을 바꿉니다. 분석을 실행하기 전에 조정합니다.",
  analyzerFeedback:
    "최근 실행의 상태, 엔진 메시지, 경고, 진단 정보를 보여줍니다.",
  aiFindings:
    "AI 보조 결과는 분석 증거를 요약합니다. 리포트에 사용하기 전에 반드시 evidence reference를 확인하세요.",
  themeControl:
    "앱 테마를 라이트, 다크, 운영체제 설정 중 하나로 전환합니다.",
  localeControl:
    "인터페이스 언어를 전환합니다. 분석 데이터와 가져온 파일 내용은 번역하지 않습니다.",
  sidebarAnalysis:
    "분석 도구는 프로파일러, 로그, 트레이스, 예외, 덤프 파일을 열어 진단 결과를 만듭니다.",
  sidebarWorkspace:
    "워크스페이스 도구는 저장된 분석 결과를 리포트, 비교, 타임라인, SLO, 서비스 흐름, 증거팩으로 재사용합니다.",
  aboutApp:
    "이 데스크톱 빌드의 버전, 빌드 정보, 라이선스, 저장소, 릴리즈 노트를 엽니다.",
  pageProfiler:
    "collapsed stack, Jennifer export, flamegraph 파일을 분석합니다. hot path, 타임라인, drill-down 필터, 파서 진단을 볼 때 사용합니다.",
  pageDiff:
    "두 프로파일러 입력을 비교해 증가, 감소, 신규, 제거된 call path를 찾습니다.",
  pageAccessLog:
    "웹 access log에서 요청량, 지연 percentile, 오류, 상태 코드 분포, URL별 동작을 분석합니다.",
  pageGcLog:
    "JVM GC 로그에서 pause 시간, 처리율, 할당/승격 압박, heap 변화, 경고, JVM 메타데이터를 분석합니다.",
  pageJfr:
    "JFR recording 또는 export JSON을 이벤트 모드, 스택 프로파일, 이벤트 테이블, heatmap, native memory 후보로 분석합니다.",
  pageException:
    "스택 트레이스 로그를 파싱하고 예외를 타입, 시그니처, 근본 원인, 이벤트 상세, finding 기준으로 묶습니다.",
  pageThreadDump:
    "쓰레드 덤프를 단일, 멀티 덤프, 락 경합 모드로 분석해 blocked, 장시간 실행, deadlock 쓰레드를 찾습니다.",
  pageMsaProfile:
    "Jennifer MSA profile export에서 트랜잭션 호출, SQL 시간, 네트워크 준비 시간, 미수집 profile, 서비스 edge를 재구성합니다.",
  pageTraceImport:
    "Trace export를 가져와 서비스, span, 의존성, 지연, critical path, 오류율 높은 service edge를 도출합니다.",
  pageAnalysisWorkspace:
    "완료된 분석 결과를 저장해 재사용합니다. export, 비교, 차트, evidence 작업 전에 여기서 결과를 선택합니다.",
  pageExportCenter:
    "선택한 workspace 결과를 리포트 파일이나 raw data로 export합니다. 결과, 형식, 출력 경로를 선택한 뒤 실행합니다.",
  pageReportDiff:
    "저장된 AnalysisResult JSON 두 개를 비교해 metric과 finding 변화를 보여줍니다.",
  pageChartStudio:
    "호환되는 workspace 결과에서 재사용 가능한 차트를 만듭니다. 결과, 템플릿, renderer를 선택해 미리보거나 저장합니다.",
  pageIncidentTimeline:
    "저장된 결과의 이벤트를 시간순 incident narrative로 정규화하고 severity, analyzer, group, source 필터를 제공합니다.",
  pageSloGoldenSignals:
    "저장된 결과의 Golden Signals와 SLI를 정규화해 SLO 위반, 영향 범위, signal inventory를 검토합니다.",
  pageServiceFlow:
    "Trace와 MSA 증거를 결합해 service edge, unmatched call, 오류 수, service-flow finding을 보여줍니다.",
  pageEvidenceBoard:
    "선택한 finding, 행, AI evidence를 리포트용 카드로 모아 HTML 또는 JSON으로 export합니다.",
  pageSettings:
    "기본 분석 옵션, 언어, 테마, 최근 파일, 앱 정보를 설정합니다.",
  sectionSummary:
    "최근 결과의 상위 요약 숫자입니다. 어떤 상세 화면을 볼지 판단하는 시작점입니다.",
  sectionFindings:
    "Finding은 분석기가 성능, 안정성, 파싱, 데이터 품질 이슈를 관찰해 표시한 항목입니다.",
  sectionFlamegraph:
    "Flamegraph는 stack sample을 call path 기준으로 합산합니다. frame을 클릭해 확대하고 reset으로 전체로 돌아갑니다.",
  sectionTopStacks:
    "Top stacks는 현재 profile에서 가장 자주 나타나거나 비용이 큰 call path를 순위로 보여줍니다.",
  sectionTopChildFrames:
    "Child frame은 현재 선택되거나 추론된 timeline scope 아래에서 어떤 callee가 가장 많이 기여하는지 보여줍니다.",
  sectionTimelineScope:
    "Timeline scope는 실행 타임라인을 만들 때 사용한 base method와 match mode를 설명합니다.",
  sectionTimeline:
    "실행 타임라인은 profile sample을 업무 알고리즘, Java 프레임워크/미들웨어, 로깅, database call, external call 같은 작업 구간으로 나눕니다.",
  sectionTimelineEvidence:
    "Timeline evidence는 각 실행 구간을 뒷받침하는 frame과 비율을 보여줍니다.",
  sectionBreakdown:
    "Breakdown은 elapsed time 또는 sample을 카테고리별로 묶어 어떤 작업이 지배적인지 보여줍니다.",
  sectionDiagnostics:
    "Parser diagnostics는 skipped line, warning, error, 원문 sample을 보여줘 파서 신뢰도를 판단하게 합니다.",
  sectionFilters:
    "필터는 재분석 없이 현재 결과의 표시 범위를 좁힙니다. 초기화하면 전체 결과로 돌아갑니다.",
  sectionUrls:
    "URL 행은 method와 URI별 요청을 합산해 느리거나 오류가 많은 endpoint를 드러냅니다.",
  sectionStatusFamilies:
    "상태 코드 그룹은 응답을 2xx, 3xx, 4xx, 5xx로 묶어 빠르게 상태를 봅니다.",
  sectionStatusCodes:
    "상태 코드 행은 가장 많이 나온 정확한 HTTP 응답 코드를 보여줍니다.",
  sectionErrorTimeline:
    "오류 타임라인은 오류율이 언제 최고였는지 보여줘 배포나 장애 시점과 맞춰 볼 수 있습니다.",
  sectionGcEvents:
    "GC 이벤트는 로그에서 파싱한 개별 pause와 heap 변화를 나열합니다.",
  sectionGcAlerts:
    "GC 알림은 pause, allocation, OOM, collector 동작에서 눈여겨볼 위험을 표시합니다.",
  sectionJvmInfo:
    "JVM 정보는 GC 로그에 기록된 runtime, heap, worker, flag, command line 메타데이터를 요약합니다.",
  sectionJfrContract:
    "JFR 범위는 선택한 recording 또는 export에서 사용할 수 있었던 event mode와 field를 설명합니다.",
  sectionJfrNativeMemory:
    "Native memory 분석은 allocation/free 이벤트를 매칭하고 recording 끝까지 해제되지 않은 allocation을 보고합니다.",
  sectionJfrTopSites:
    "Top site는 해제되지 않은 byte 또는 event가 큰 native memory call site를 순위로 보여줍니다.",
  sectionJfrEvents:
    "JFR 이벤트는 mode, state, time window, top-N 필터가 적용된 개별 record입니다.",
  sectionJfrBreakdown:
    "이벤트 breakdown은 JFR 이벤트를 type별로 묶어 밀도가 높은 카테고리를 보여줍니다.",
  sectionJfrHeatmap:
    "Heatmap은 wall-clock 이벤트를 시간 bucket으로 묶습니다. bucket을 클릭하면 분석 시작 시각으로 적용됩니다.",
  sectionExceptionEvents:
    "예외 이벤트는 파싱된 발생 건을 보여주고, 상세 패널에서 전체 message, stack, signature, root cause를 확인합니다.",
  sectionExceptionTypes:
    "예외 타입 행은 class 또는 언어별 error type 기준으로 이벤트를 묶습니다.",
  sectionExceptionSignatures:
    "시그니처는 예외를 type과 대표 stack 형태로 묶어 중복 noise를 줄입니다.",
  sectionThreadMode:
    "쓰레드 덤프 모드는 단일 덤프, 여러 덤프, 락 메타데이터 중 무엇을 분석할지 결정합니다.",
  sectionThreadSelected:
    "선택된 덤프는 순서대로 분석될 파일입니다. 멀티 덤프 모드는 이 순서로 지속 현상을 탐지합니다.",
  sectionThreadSignatures:
    "쓰레드 시그니처는 비슷한 stack을 묶어 반복되는 blocked 또는 장시간 실행 패턴을 보기 쉽게 합니다.",
  sectionThreadPerDump:
    "덤프별 행은 각 입력 파일의 metadata와 쓰레드 수를 보여줍니다.",
  sectionThreadRows:
    "쓰레드 행은 파싱된 state, top frame, lock, category, source dump 정보를 보여줍니다.",
  sectionThreadDeadlock:
    "Deadlock cycle은 순환 의존성을 만드는 lock owner와 waiter 관계를 보여줍니다.",
  sectionTraceDependencies:
    "서비스 의존성은 span에서 도출한 caller-to-callee edge입니다.",
  sectionTraceServices:
    "서비스 지연은 서비스별 duration과 error 동작을 요약합니다.",
  sectionWorkspaceCard:
    "Workspace 결과 카드는 완료된 분석 실행 하나를 나타냅니다. 활성화, export, 비교, evidence 추출에 사용합니다.",
  sectionExportPreview:
    "선택 결과 미리보기는 어떤 분석기 출력이 export될지 확인하는 영역입니다.",
  sectionReportMetricDeltas:
    "Metric delta는 선택한 두 AnalysisResult 파일의 숫자형 summary field를 비교합니다.",
  sectionIncidentGroups:
    "Group은 timeline event를 correlation key 또는 analyzer context 기준으로 묶습니다.",
  sectionIncidentNarrative:
    "Narrative는 timeline event를 순서 있는 incident 단계로 변환합니다.",
  sectionIncidentTimeline:
    "Timeline 행은 event time, source, severity, correlation, evidence reference를 보여줍니다.",
  sectionSloViolations:
    "SLO 위반은 실제 값이 target 또는 threshold 기대를 벗어난 metric 목록입니다.",
  sectionSliMetrics:
    "SLI metric은 선택된 workspace 결과에서 정규화한 service-level indicator입니다.",
  sectionSloAffected:
    "영향 범위는 SLO risk에 기여한 service, endpoint, group을 요약합니다.",
  sectionSignalInventory:
    "Signal inventory는 어떤 Golden Signal이 어떤 analyzer source에서 발견됐는지 보여줍니다.",
  sectionServiceFindings:
    "Service-flow finding은 error, latency, missing span, unmatched external call 같은 위험 edge를 강조합니다.",
  sectionServiceEdges:
    "Service edge는 caller, callee, count, latency, error, source 증거를 보여줍니다.",
  sectionEvidenceCard:
    "Evidence card는 리포트 export를 위해 중요한 finding 또는 행을 source metadata와 함께 보존합니다.",
  sectionMsaSingleBaseline:
    "선택한 트랜잭션 GUID 하나의 실제 호출 순서와 elapsed 값을 보여줍니다.",
  sectionMsaCaptureStatus:
    "2PC, check query, external call, network preparation, connection acquire 증거가 잡혔는지 요약합니다.",
  sectionMsaIncomplete:
    "일부 트랜잭션 profile이 불완전해 edge 재구성이나 timing 신뢰도가 낮아질 수 있음을 알립니다.",
  sectionMsaTransactions:
    "Transaction profile 행은 파싱된 profile, 검증 metadata, profile별 timing 값을 보여줍니다.",
  sectionMsaNetworkPrep:
    "네트워크 준비 method는 내부 external call 시간을 빼고 남은 wrapper 시간을 보여줍니다.",
  sectionMsaMethodResidual:
    "메소드 미분류 응답시간은 일반적인 느린 메소드 순위가 아닙니다. METHOD frame 응답시간에서 SQL, 외부 호출, fetch, Network Prep, connection acquire, 사용자 정의 메소드 규칙처럼 식별된 자식 작업 구간을 빼고도 남은 시간을 보여줍니다. 합계는 응답시간 구성의 Method Time과 비교하는 값이지만, 스코프, interval 복원, 겹침 처리, 표시 행 제한 차이 때문에 완전히 일치하지 않을 수 있습니다.",
  sectionMsaUnprofiledExternal:
    "프로파일 미수집 외부호출은 caller는 있지만 매칭되는 callee profile이 수집되지 않은 call입니다.",
  sectionMsaExternalTop:
    "API URL별 호출회수, 누적 API 시간, 최장, p95, 네트워크 timing을 함께 집계합니다.",
  sectionMsaSlowSql:
    "성능 저하 SQL 후보는 현재 threshold를 넘는 SQL 응답 시간을 강조합니다.",
  sectionMsaCustomCards:
    "사용자 정의 카드 통계는 분석 옵션의 pattern에 매칭된 행을 집계합니다.",
  sectionMsaServiceNetworkTime:
    "서비스 호출 네트워크 타임은 caller와 callee URL 기준으로 매칭 호출을 묶고 호출회수, callee 누적/최장 시간, 네트워크 누적/평균/최장 시간을 보여줍니다.",
  sectionMsaFileErrors:
    "파일 오류는 파싱하지 못했거나 심각한 검증 문제가 있는 입력을 보여줍니다.",
  sectionMsaFileSummary:
    "파일 요약은 입력 파일별 profile 수, error, warning을 보여줍니다.",
  sectionMsaSignatures:
    "MSA signature는 평균 모드 분석을 위해 비슷한 transaction shape을 묶습니다.",
  sectionMsaEdgeStats:
    "Edge statistics는 caller-to-callee timing, count, matching quality를 집계합니다.",
  sectionMsaGuidGroups:
    "GUID group은 재구성된 transaction group과 실행 metadata를 보여줍니다.",
  sectionMsaExternalEdges:
    "External call edge는 Jennifer profile 행에서 탐지한 service boundary입니다.",
  sectionMsaTimelineMode:
    "Timeline mode는 MSA timeline을 그리기 전에 실제 GUID 하나 또는 평균 signature를 선택합니다.",
  optionMsaTimelineGuid:
    "단일 transaction MSA timeline을 그릴 실제 transaction GUID를 선택합니다.",
  optionMsaTimelineSignature:
    "여러 유사 transaction을 평균 MSA timeline으로 합칠 signature group을 선택합니다.",
  sectionMsaParserReport:
    "Parser report는 가져온 파일, 파싱된 profile, profile 완성도, parser warning을 요약합니다.",
  sectionMsaProfileIssues:
    "Profile issue는 개별 profile에 붙은 error와 warning을 보여줍니다.",
  optionFormat:
    "분석기가 기대할 파일 형식을 선택합니다. 잘못 선택하면 parser warning 또는 빈 결과가 나올 수 있습니다.",
  optionProfileKind:
    "Collapsed stack을 CPU, wall-clock, allocation, lock 등 어떤 profile kind로 해석할지 정합니다.",
  optionInterval:
    "Sample interval은 sample count를 추정 elapsed time으로 바꿉니다. 실제 profiler interval을 알고 있으면 그 값을 사용하세요.",
  optionElapsed:
    "전체 elapsed time을 직접 지정합니다. sample과 interval로 추정하게 하려면 비워 둡니다.",
  optionTopN:
    "순위 목록과 테이블을 가장 관련 있는 행으로 제한합니다. 값을 키우면 상세가 늘지만 noise도 늘 수 있습니다.",
  optionTimelineBase:
    "트랜잭션 경계를 대표하는 method 또는 frame pattern으로 timeline 분석 범위를 제한합니다.",
  optionMemoryGuards:
    "매우 큰 입력에서도 반응성을 유지하도록 unique stack, stack depth, RSS, debug output을 제한합니다.",
  optionMaxLines:
    "엔진이 읽는 로그 행 수를 제한합니다. 큰 파일에서 전체 결과가 필요하면 값을 늘립니다.",
  optionStartTime:
    "선택적 시작 시각입니다. parser가 timestamp를 읽을 수 있으면 이 시각 이전 행은 제외합니다.",
  optionEndTime:
    "선택적 종료 시각입니다. parser가 timestamp를 읽을 수 있으면 이 시각 이후 행은 제외합니다.",
  optionStrict:
    "엄격 모드는 애매한 GC 로그 행을 더 보수적으로 처리하며 parser issue가 더 많이 나올 수 있습니다.",
  optionJfrMode:
    "CPU, wall-clock, allocation, lock, GC, exception, I/O 같은 분석 모드로 JFR record를 필터링합니다.",
  optionJfrTime:
    "JFR 분석 시간 범위를 제한합니다. parser가 해석할 수 있으면 ISO time, clock time, 상대 offset을 지원합니다.",
  optionJfrState:
    "Runnable, blocked, waiting 같은 thread state로 JFR stack sample을 필터링합니다.",
  optionNativeLeakOnly:
    "켜면 matching된 allocation/free pair를 제거하고 해제되지 않았을 가능성이 높은 native memory만 남깁니다.",
  optionNativeTail:
    "Recording 마지막 구간의 allocation을 무시합니다. 아직 free될 시간이 없었던 단기 allocation noise를 줄입니다.",
  optionThreadThreshold:
    "쓰레드가 몇 개의 연속 dump에 나타나야 persistent로 보고할지 정합니다.",
  optionThreadFormat:
    "덤프 format 자동 감지를 덮어씁니다. 자동 감지가 잘못된 parser를 고를 때만 사용하세요.",
  optionNormalize:
    "Sample 수가 다른 두 profile을 비교하기 쉽도록 diff total을 정규화합니다.",
  optionWorkspaceResult:
    "이 도구가 입력으로 사용할 저장된 분석 결과를 선택합니다.",
  optionExportFormat:
    "출력 형식을 선택합니다. 리포트 형식은 사람이 읽기 좋고 JSON/CSV는 구조화 데이터를 보존합니다.",
  optionOutputPath:
    "파일 또는 디렉터리 출력 위치를 지정합니다. 비워 두면 export 시 선택합니다.",
  optionChartTemplate:
    "선택한 workspace 결과에서 만들 차트 정의를 선택합니다.",
  optionRenderer:
    "Canvas는 조밀한 차트에 빠르고, SVG는 단순 차트에서 inspect가 쉽고 선명할 수 있습니다.",
  optionSearch:
    "검색어로 현재 테이블 또는 timeline을 label, source, category, service, evidence 텍스트 기준으로 필터링합니다.",
  optionAnalyzer:
    "이벤트 또는 evidence를 만든 analyzer 기준으로 행을 필터링합니다.",
  optionSeverity:
    "Severity 기준으로 행을 필터링해 critical 또는 warning 항목을 먼저 검토합니다.",
  optionGroup:
    "Correlation group 기준으로 timeline event를 필터링합니다.",
  optionSignalKind:
    "Latency, traffic, errors, saturation 같은 Golden Signal 종류로 SLO 행을 필터링합니다.",
  optionDrillPattern:
    "Drill-down 필터에 사용할 frame pattern입니다. 필터 타입에 따라 substring 또는 regex로 동작합니다.",
  optionDrillFilterType:
    "Include, exclude, regex include, regex exclude 중 stack path를 남기는 방식을 선택합니다.",
  optionDrillMatchMode:
    "Match가 어디에나 나타나도 되는지, 지정한 순서대로 나타나야 하는지 정합니다.",
  optionDrillViewMode:
    "Matching path가 원래 부모 경로를 유지할지, match 주변으로 re-root될지 정합니다.",
  optionCaseSensitive:
    "켜면 대소문자가 정확히 일치해야 합니다.",
  settingsDefaults:
    "기본값은 분석기 페이지가 새 옵션을 초기화할 때 사용합니다.",
  settingsRecent:
    "최근 파일은 빠른 재사용을 위해 로컬에 저장되며 여기에서 삭제할 수 있습니다.",
  settingsAbout:
    "설치된 앱 빌드의 metadata입니다.",
};

const catalog: Record<Locale, Record<HelpKey, string>> = { en, ko };

export function getHelpText(locale: Locale, key: HelpKey): string {
  return catalog[locale][key] ?? catalog.en[key];
}

export function useHelpText(key: HelpKey): string {
  const { locale } = useI18n();
  return getHelpText(locale, key);
}

export function getGenericCardHelpText(locale: Locale, title: string): string {
  return getHelpText(locale, "genericCard").replace("{title}", title);
}

export function getGenericChartHelpText(locale: Locale, title: string): string {
  return getHelpText(locale, "genericChart").replace("{title}", title);
}

export function getGenericMetricHelpText(locale: Locale, label: string): string {
  return getHelpText(locale, "genericMetric").replace("{label}", label);
}
