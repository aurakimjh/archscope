# 로드맵

## Phase 1: Foundation

- Repository skeleton
- Desktop UI skeleton
- Python engine skeleton
- Access log parser MVP
- Collapsed profiler parser MVP
- Sample charts
- JSON result format
- English/Korean documentation and UI i18n foundation
- Engine-UI 브릿지 — 초기에는 Electron IPC + Python CLI 기반 PoC. 2026-05 웹 전환(T-206..T-209)에서 FastAPI HTTP 경계(`/api/...`) + in-process 분석기 dispatch로 교체
- Python runtime dependency 및 CLI entry point 명시
- Malformed record용 parser diagnostics
- Encoding fallback correctness
- Access Log와 Profiler용 type-specific `AnalysisResult` contract
- Parser, utility, JSON exporter 중심 테스트 확충

## Phase 2: Report-ready Charts

- Chart Studio
- Theme editor
- ECharts 6 upgrade 평가
- Dark mode 및 dynamic chart theme
- Broken-axis 및 distribution chart option
- PNG/SVG export
- CSV export
- Chart Studio template preview/edit MVP
- Access log advanced statistics
- Raw chart를 넘어선 Access Log diagnostic findings
- Profiler flamegraph drill-down, Jennifer CSV import, execution breakdown
- Custom regex parser
- Report label language toggle

## Phase 3: JVM Diagnostics and Distribution

- GC log analyzer MVP
- Java thread dump analyzer MVP
- Java exception analyzer MVP
- JFR recording parser 설계 및 feasibility spike
- Timeline correlation
- *(2026-05 웹 전환에서 폐기)* Electron 버전 업그레이드 및 Electron + PyInstaller 패키징 spike — `pip install -e .` + `archscope-engine serve --static-dir`로 교체. [PACKAGING_PLAN](./PACKAGING_PLAN.md) 참고.

## Phase 4: Multi-runtime and Observability Inputs

- Timeline correlation `AnalysisResult` 설계
- JDK `jfr` command spike path 기반 JFR recording parser 설계
- Trace/span context mapping을 포함한 OpenTelemetry log input 설계
- Node.js log and stack analyzer
- Python traceback analyzer
- Go panic/goroutine analyzer
- .NET exception/IIS analyzer
- OpenTelemetry JSONL log analyzer 및 cross-service trace correlation MVP
- OpenTelemetry parent-span service path 분석 및 failure propagation
- 더 넓은 OpenTelemetry envelope ingestion 및 span timing correlation
- Access log, GC, profiler, thread, JFR, OTel evidence를 아우르는 cross-evidence timeline correlation

## Phase 5: Report Automation

- Before/after diff
- HTML report generation
- `AnalysisResult` 및 parser debug JSON용 portable static HTML report MVP
- Profiler result JSON용 static HTML flamegraph rendering
- PowerPoint export
- Minimal PowerPoint `.pptx` report MVP
- Executive summary generator
- AI-assisted interpretation, optional and evidence-bound
- 검증된 evidence reference를 포함한 optional local LLM/Ollama interpretation
- AI interpretation hardening: canonical `evidence_ref` 문법, `InterpretationResult` contract, runtime validator, prompt-injection defense, local-only runtime policy, provenance UI, evaluation gate

## Phase 6: 외부 도구 패리티 (post-0.2.0-beta)

TDA(`C:\workspace\tda-main`)와 async-profiler를 기준으로 한 갭 분석에서
시작된 단계입니다. 마일스톤마다 엔진 + UI 작업이 함께 묶여 있습니다.

### Profiler (M1–M4 — 2026-04/05 완료)

- **M1 — JFR first class.** 바이너리 `.jfr`을 JDK `jfr` CLI로 자동
  변환(PATH / `JAVA_HOME` / `ARCHSCOPE_JFR_CLI`); 다중 이벤트 모드
  필터(`cpu` / `wall` / `alloc` / `lock` / `gc` / `exception` /
  `io` / `nativemem`); 시간 범위 필터(ISO, `HH:MM:SS`, 상대
  `+30s` / `-2m` / `500ms`); thread-state 필터; 최소 지속시간 필터.
- **M2 — Differential flame + display options.** 정규화된 totals 비교
  위에 발산 빨강/파랑 그라데이션을 입힌 양면 diff; flame 디스플레이
  툴바(highlight regex, 클래스명 단순화, 람다 정규화, 점 단위 패키지,
  icicle 역방향 보기, min-width 단순화).
- **M3 — Heatmap + per-thread 격리.** 1D wall-clock 밀도 strip — 영역
  드래그 시 다음 분석에 자동으로 시간 범위 필터를 채움; `-t`
  collapsed 출력용 per-thread 필터 드롭다운(서버 라운드트립 없이 단일
  쓰레드로 re-root); min-width 단순화 마무리.
- **M4 — pprof export + tree view + native-mem leak + JFR 강화.**
  의존성 없는 자체 minimal protobuf encoder로 pprof 출력
  (gzipped, Pyroscope / Speedscope / `go tool pprof` 호환); 계층형
  확장 가능 트리 테이블; 네이티브 메모리 leak 검출(alloc/free 페어링,
  tail-ratio cutoff, 바이트 가중 플레임); 연속 세션 워크플로우용 최근
  파일 패널(localStorage).

### Thread dump — TDA 강화 (진행 중)

아래 항목 다수는 Codex가 단일 커밋(`e6e6f48`)으로 일괄 적용한 뒤
지속적으로 보강된 것입니다.

- 가상 쓰레드 캐리어 pinning 검출(`VIRTUAL_THREAD_CARRIER_PINNING`) —
  가상 쓰레드가 monitor를 보유해 Loom 캐리어가 pinned된 상황을 표시.
- SafeMemoryReclamation / 좀비 쓰레드 검출
  (`SMR_UNRESOLVED_THREAD`).
- Lock-contention owner/waiter 그래프 + DFS 데드락 사이클 검출.
- 휴리스틱 finding: `THREAD_CONGESTION_DETECTED`,
  `EXTERNAL_RESOURCE_WAIT_HIGH`, `LIKELY_GC_PAUSE_DETECTED`,
  `GROWING_LOCK_CONTENTION`.
- 9개 변형 파서 레지스트리: `java_jstack`(JDK 21+ no-`nid` 포함),
  `java_jcmd_json`, `go_goroutine`, `python_pyspy`,
  `python_faulthandler`, `python_traceback`, `nodejs_diagnostic_report`,
  `nodejs_sample_trace`, `dotnet_clrstack`,
  `dotnet_environment_stacktrace`. UTF-16 / BOM 자동 감지.
- 다중 파일 선택(폴더를 한 번에 드래그-드롭).
- JVM signals 탭 — Carrier-pinning / SMR / Native methods / Class
  histogram 서브탭과 Dump overview 카드.

### Access log 전면 개편 (2026-05 완료)

- URL별 통계를 count / avg / p95 / total bytes / errors로 정렬.
- 파일 확장자와 잘 알려진 asset 경로 기준의 정적/API 분류, 행마다
  비율 표시.
- Summary의 p50 / p90 / p95 / p99 + 분당 percentile 타임라인.
- Throughput(req/s, bytes/s) 요약 + 분당 시계열.
- HTTP status family + top status code 분포, 분당 status-class
  타임라인이 50%+ 오류율 분(分)을 강조.
- 새 finding: `SLOW_URL_P95`, `ERROR_BURST_DETECTED`.

### GC log 심화 (2026-05 완료)

- 로그 헤더에서 추출한 JVM Info 카드(Version, CPUs, Memory, Heap
  min/initial/max/region, Parallel & Concurrent workers, Compressed
  Oops, NUMA, Pre-touch, Periodic GC, 풀 CommandLine flags) +
  worker vs CPU 미스매치 경고 배너.
- 9개 토글 가능 힙 시리즈(Heap before/after/committed, Young, Old,
  Metaspace) + 우측 축 Pause 오버레이 옵션.
- 드래그 사각형 줌(파란 선택 사각형 표시) + 시리즈당 최대
  2 000 포인트 데시메이션.

### Windows 데스크톱 강화

- Python 엔진을 PyInstaller로 번들한 NSIS Windows 인스톨러 + 포터블
  zip(Electron 기반).
- Electron 메인 프로세스 ESM `__dirname` 수정 + static import 전환.
- 프리로드의 `window.archscope.engineUrl`로 해석되는 `apiBase` 헬퍼 —
  `file://` 렌더러가 번들된 엔진(`127.0.0.1:8765`)에 도달하게 함.
- 렌더러 안에 Pretendard Variable 번들로 한국어 폰트가 Malgun Gothic
  으로 폴백되지 않게.
- 차트 축 안정성을 위해 page zoom 1.0 고정.

### 향후 후보 (아직 미커밋)

- Access-log / GC / thread-dump / JFR evidence를 공유 시간 축으로
  묶는 연속 세션 타임라인.
- async-profiler 3.x packed-binary HTML 지원.
- Heap dump / `.hprof` 분석은 명시적으로 범위 밖.
