# ArchScope 사용자 가이드 (한국어)

[English](../en/USER_GUIDE.md)

ArchScope의 모든 페이지와 CLI 명령을 단계별로 설명합니다. 1분 안에
시작하고 싶으면 [최상위 README](../../README.md)의 빠른 시작을 보세요.

---

## 목차

1. [설치 및 실행](#설치-및-실행)
2. [UI 둘러보기](#ui-둘러보기)
3. [Dashboard](#dashboard)
4. [Access Log Analyzer](#access-log-analyzer)
5. [Profiler Analyzer](#profiler-analyzer)
6. [GC Log Analyzer](#gc-log-analyzer)
7. [Thread Dump Analyzer](#thread-dump-analyzer)
8. [Exception Analyzer](#exception-analyzer) · [JFR Analyzer](#jfr-analyzer)
9. [Demo Data Center](#demo-data-center)
10. [Export Center](#export-center)
11. [Chart Studio](#chart-studio)
12. [Settings](#settings)
13. [AI 해석 (선택 사항)](#ai-해석-선택-사항)
14. [이미지 내보내기 & "모든 차트 저장"](#이미지-내보내기--모든-차트-저장)
15. [Thread → Flamegraph 변환](#thread--flamegraph-변환)
16. [CLI 레퍼런스](#cli-레퍼런스)
17. [트러블슈팅 FAQ](#트러블슈팅-faq)

---

## 설치 및 실행

### 요구사항

| 항목 | 값 |
| --- | --- |
| OS | macOS / Linux / Windows |
| Python | 3.9 이상 (소스에서 엔진을 실행할 때만) |
| Node.js | 18+ (UI를 소스에서 빌드할 때만) |
| JDK | 11+ on PATH (선택 — 바이너리 `.jfr` 사용 시에만 필요) |
| RAM | 최소 4 GB, 대규모 플레임그래프는 8 GB 권장 |
| Disk | ~500 MB |

ArchScope는 Python 엔진이 PyInstaller로 번들된 Windows 인스톨러(NSIS)와
포터블 zip도 함께 배포합니다. 윈도우 사용자라면 Python · Node · 빌드
없이 바로 실행할 수 있습니다. 아래 "소스에서" 절차는 개발 흐름입니다.

### 엔진 설치

```bash
cd engines/python
python -m venv .venv
source .venv/bin/activate          # Windows: .venv\Scripts\activate
pip install -e .
```

`pip install -e .`이 `archscope-engine` 콘솔 스크립트를 등록하고
FastAPI / uvicorn / defusedxml / Typer / python-multipart를 설치합니다.

### UI 빌드 + 서버 기동

저장소에 한 번에 처리하는 헬퍼가 있습니다:

```bash
./scripts/serve-web.sh             # macOS / Linux
```

수동 실행:

```bash
cd apps/frontend
npm install
npm run build                      # apps/frontend/dist 생성
cd ../..
archscope-engine serve --static-dir apps/frontend/dist
```

브라우저에서 `http://127.0.0.1:8765` 접속. 기본 바인딩은
`127.0.0.1`이며 LAN에 노출하려면 `--host 0.0.0.0`을 명시해야 합니다.

### 핫 리로드 개발 루프

```bash
# 터미널 1 — 엔진 자동 리로드
archscope-engine serve --reload

# 터미널 2 — Vite dev 서버 (/api → :8765 프록시)
cd apps/frontend && npm run dev     # http://127.0.0.1:5173
```

---

## UI 둘러보기

```text
┌────────────────────────────────────────────────────────┐
│  TopBar — 브랜드 · 검색 · 테마 · 언어 · 설정              │
├────────────┬───────────────────────────────────────────┤
│ Sidebar    │  메인 콘텐츠                                │
│ (접기 가능)│  ┌─────────────────────────────────────┐  │
│            │  │ Sticky FileDock (드래그 & 드롭)      │  │
│            │  ├─────────────────────────────────────┤  │
│            │  │ Tabs (페이지별)                      │  │
│            │  │   summary / charts / tables / …     │  │
│            │  └─────────────────────────────────────┘  │
└────────────┴───────────────────────────────────────────┘
```

- **TopBar** — light/dark/system 테마(영구), 언어 토글(English / 한국어),
  설정 단축버튼.
- **Sidebar** — 접기 가능한 아이콘 레일. 접힘 상태는 `localStorage`에
  저장. 섹션: Analyzers, Workspace, Settings.
- **FileDock** — 모든 분석기 페이지 상단의 sticky 업로드 카드. 파일을
  드래그하거나 "Browse" 클릭. 업로드된 파일은 `~/.archscope/uploads/`
  에 저장되고, 분석기는 그 서버 측 경로를 받아서 동작합니다.
- **Tabs** — 결과를 Summary / Charts / 도메인별 / Diagnostics 등으로
  분할해서 한 페이지가 무한 스크롤되지 않게 합니다.

대부분의 페이지에는 TabsList 우측에 **"모든 차트 저장"** 버튼이
있습니다 — [이미지 내보내기](#이미지-내보내기--모든-차트-저장) 참고.

---

## Dashboard

마지막으로 실행한 분석 결과(`saveDashboardResult`를 호출하는 모든
분석기)를 기억하고 access-log 메트릭 카드와 표준 차트 패널을 보여
줍니다.

- **빈 상태** — 분석을 한 번도 안 돌렸으면 "분석 결과 없음" 안내.
  분석기 한 번 실행하고 다시 오세요.
- **모든 차트 저장** — 한 번 클릭으로 Dashboard의 모든 차트를 2× PNG로
  일괄 저장.

---

## Access Log Analyzer

### 입력

`.log` / `.txt` 액세스 로그를 FileDock에 드롭. **Analyzer options**
카드에서 포맷 선택:

- `nginx`(기본), `apache`, `ohs`, `weblogic`, `tomcat`,
  `custom-regex`(폴백 파서, 느림).

선택 옵션:

- **Max lines** — 빠른 테스트용 N라인 샘플링.
- **Start time / End time** — ISO `datetime-local`. 엔진이 그 범위로
  분석을 좁힙니다.

**Analyze** 클릭. 실행 중이면 **Cancel** 버튼이 보입니다.

### 탭

| 탭 | 내용 |
| --- | --- |
| Summary | 12-메트릭 그리드: 총 요청 수, 오류율, 평균 + **p50 / p90 / p95 / p99** 응답 시간, 총 바이트, 평균 req/s, 평균 throughput(bytes/s), 정적 요청 수, API 요청 수. |
| Charts | Request count trend (ECharts 패널 — D3 차트와 동일한 이미지 export 드롭다운 지원). |
| URLs | **정렬 가능한** per-URL 통계 테이블(`url_stats` 기반). 정렬 축(count / avg response / p95 response / total bytes / errors), 분류 필터(전체 / API only / static only). 각 행: method, URI, classification 배지, count, avg ms, p95 ms, total bytes, error count, 행마다 `2·3·4·5xx` 분포. |
| Status & errors | Status family 테이블, top status code(family 단위가 아닌 전체 코드), **분(分)당 타임라인**으로 2xx/3xx/4xx/5xx 카운트와 오류율. 50% 이상 오류율의 분(分)은 rose로 강조; 카드 헤더에 피크 표시("Peak: 75% errors at 14:23 (15/20 requests)"). |
| Parser Report | 파서 진단 — 스킵된 라인 수, 실패 샘플(custom format 디버깅용). |

분석기는 새로운 finding도 함께 emit합니다: `SLOW_URL_P95`(p95 ≥ 1 s,
샘플 ≥ 5) 와 `ERROR_BURST_DETECTED`(분당 오류율 ≥ 50%, 요청 ≥ 5).

---

## Profiler Analyzer

### 입력

**Profile format** 셀렉터로 파서 선택:

| `profileFormat` | 받는 형식 | 언제 |
| --- | --- | --- |
| `collapsed` | `*.collapsed`, `*.txt` | async-profiler `-o collapsed`, perf `stackcollapse-perf.pl`, jstack 변환 결과(아래 [Thread → Flamegraph](#thread--flamegraph-변환) 참고). |
| `jennifer_csv` | `*.csv` | Jennifer APM 플레임그래프 CSV. |
| `flamegraph_svg` | `*.svg` | FlameGraph.pl 또는 async-profiler `-o svg`. |
| `flamegraph_html` | `*.html`, `*.htm` | async-profiler self-contained HTML(인라인 SVG 또는 임베디드 JS 트리). |

포맷을 바꾸면 FileDock의 `accept`도 자동 전환되어 OS 파일 선택창이
관련 확장자만 보여줍니다.

`collapsed` 입력의 경우 **Profile kind**가 `wall` / `cpu` / `lock`
중 하나를 선택합니다(라벨과 finding에만 영향, 파싱은 동일).

기타 옵션:

- **Interval (ms)** — 샘플 간격(샘플 → 시간 변환에 사용).
- **Elapsed seconds** — wall-clock 윈도우 길이, 선택.
- **Top N** — breakdown에 표시할 상위 스택 개수.

### 탭

| 탭 | 내용 |
| --- | --- |
| Summary | 총 샘플 · interval · 추정 CPU/Wall 시간 + drill-down stage 메트릭. |
| Flame Graph | 인터랙티브 플레임그래프 + **Display 툴바**(highlight 정규식, 클래스명 단순화, 람다 정규화, 점 단위 패키지, **Icicle**(역방향), **Min width** %). 트리에 thread 브래킷이 있으면(async-profiler `-t`) **Filter by thread** 드롭다운이 서버 라운드트립 없이 단일 쓰레드로 re-root. 차트 위 **Export pprof** 버튼은 Pyroscope / Speedscope / `go tool pprof`에서 바로 쓸 수 있는 gzipped pprof을 다운로드. SVG 기본, ≥ 4 000 노드면 Canvas 자동 전환. 더블 클릭으로 줌 리셋. |
| **Tree** | 플레임 데이터를 sample desc로 정렬한 계층형 확장 가능 테이블. 컬럼: Frame · Samples · Self · % of total. Flame과 동일한 display + highlight 옵션 적용. |
| **Diff** | Baseline / Target 두 개의 FileDock + 측면별 포맷 셀렉터 + Normalize 토글. 분석 후 발산 색상의 플레임(빨강 = 증가, 파랑 = 감소)과 두 테이블("Biggest increases" 느려진 항목 / "Biggest decreases" 빨라진 항목)이 표시됨. **Recent profile files** 패널(localStorage에 마지막 10개 분석 파일 저장)도 포함 — 클릭으로 baseline, Shift+클릭으로 target 지정해 연속 세션 워크플로우. |
| Drill-down | 스택 프레임 필터(`include_text` / `exclude_text` / `regex_include` / `regex_exclude`), 매칭 모드(`anywhere` / `ordered` / `subtree`), 뷰 모드(`preserve_full_path` / `reroot_at_match`). |
| 타임라인 분석 | 플레임그래프 샘플을 기동/프레임워크, 내부 메소드, SQL, DB 네트워크 대기, 외부 호출, 외부 호출 네트워크 대기, 풀 대기, 락 대기 등 수행시간 구성으로 변환한 stacked chart와 근거 테이블. |
| Breakdown | ECharts 도넛 + 막대 — execution 카테고리 분포(SQL / external API / network I/O / connection-pool wait / 기타). 카테고리별 top 메소드 테이블. |
| Top Stacks | top N 스택 정렬 테이블(samples · estimated seconds · ratio). |
| Parser Report | 파서 진단. |

### 플레임그래프 저장

- **SVG 렌더러** — 차트 프레임 "Save image" 드롭다운에서
  PNG 1×/2×/3×, JPEG 2×, SVG 벡터 선택.
- **Canvas 렌더러** — 별도 **"Save PNG"** 버튼이 추가됨.
  `canvas.toDataURL()`로 현재 device pixel ratio에 맞춘 픽셀 퍼펙트
  스냅샷.
- **Export pprof** — `*.pb.gz`(gzipped Google pprof binary) 다운로드.
  `go tool pprof`, Pyroscope, Speedscope, `pprof.dev`에 그대로 사용
  가능.

---

## GC Log Analyzer

### 입력

`*.log` / `*.txt` / `*.gz` HotSpot unified GC 로그를 FileDock에
드롭하고 **Analyze**. 엔진이 pause / heap / allocation / promotion
타임라인과 collector별 카운트를 추출합니다.

### 탭

| 탭 | 내용 |
| --- | --- |
| **JVM Info** (기본) | 로그 헤더에서 추출한 JVM/시스템 메타데이터: Version, CPUs(total/available), Memory, Heap Min/Initial/Max/Region size, Parallel/Concurrent/Concurrent-refinement workers, Compressed Oops, NUMA, Pre-touch, Periodic GC. **CommandLine 플래그**는 한 줄씩 그대로 노출(**Copy** 버튼 제공)되며 raw 헤더 라인도 보존. 워커 vs CPU 미스매치(예: "9개 CPU 호스트에서 GC workers가 1로 제한됨") 경고 배너. |
| Summary | 15-stat 메트릭 그리드(throughput, p50/p95/p99/max/avg/total pause, young/full GC count, allocation/promotion rate, humongous count, concurrent-mode failures, promotion failures) + findings 카드. |
| GC Pauses | **인터랙티브 타임라인** — 플롯 안에서 드래그하면 파란 사각형이 보이며 그 범위로 줌. 휠은 in/out, 더블 클릭은 리셋. 4-stat 선택 요약(events in window / avg / p95 / max pause) 표시. Full-GC 이벤트는 주황 점선(호버 시 `cause`, `pause_ms`, before/after/committed heap). 큰 로그도 부드러운 시리즈당 최대 2 000 포인트 데시메이션. |
| Heap Usage | **9개 토글 가능 시리즈**(Heap before/after/committed, Young before/after, Old before/after, Metaspace before/after) + 우측 축 **Pause 오버레이** 옵션. 데이터 없는 시리즈는 자동 회색 처리. Pauses 탭과 동일한 드래그/휠 줌. |
| Algorithm comparison | 프론트엔드 집계 테이블 — `gc_type`별 pause 통계(count / avg / p95 / max / total ms) + 두 개의 horizontal D3 bar chart(avg + max). G1Young / G1Mixed / FullGC / ZGC / Shenandoah 비교. |
| Breakdown | `gc_type_breakdown` / `cause_breakdown` D3 horizontal bar. |
| Events | 최대 200개 이벤트 shadcn 테이블(timestamp · uptime · type · cause · pause · heaps). |
| Parser Report | 파서 진단. |

---

## Thread Dump Analyzer

ArchScope에서 유일한 "멀티 파일" 분석기이며 TDA에서 가장 큰 영감을
받은 페이지입니다.

### 파일 추가

FileDock이 **다중 선택**을 지원합니다 — OS 파일 선택창에서
`Ctrl/Shift+클릭`, 덤프 폴더 드래그-드롭, 또는 업로드 반복으로 추가
가능. 각 행에는 인덱스, 원본 파일명, 업로드 크기, `X` 제거 버튼.

지원되는 모든 런타임을 섞어도 됩니다. 파서 레지스트리가 첫 4 KB를
sniff하고 **9개 플러그인 변형**(`java_jstack`, `java_jcmd_json`,
`go_goroutine`, `python_pyspy`, `python_faulthandler`,
`python_traceback`, `nodejs_diagnostic_report`, `nodejs_sample_trace`,
`dotnet_clrstack`, `dotnet_environment_stacktrace`)을 매칭합니다. 두
파일이 다른 포맷으로 인식되면 `MIXED_THREAD_DUMP_FORMATS`로 즉시
실패. 강제 파서 적용은 **Format override** 입력에 `format_id`
(예: `java_jstack`). UTF-16 / BOM 인코딩 덤프는 자동 감지·디코딩.

**Consecutive-dump threshold** 입력(기본 `3`)은 persistence finding
임계치를 조절합니다.

**Correlate dumps** 클릭으로 멀티 덤프 분석 실행.

### 탭

| 탭 | 내용 |
| --- | --- |
| **Overview** | 덤프 단위 메타데이터 카드(개수 / 시간 범위 / unique 쓰레드명 / 총 관측치 / 주 런타임 / 파서 포맷)와 모든 덤프에 걸친 상태 빠른 분포. |
| Findings | 심각도별 색상 finding 카드(아래 카탈로그). 카드 아래에 `LONG_RUNNING_THREAD` / `PERSISTENT_BLOCKED_THREAD` 전체 테이블. |
| Charts | D3 vertical bar — 덤프별 쓰레드 수 + D3 horizontal sorted bar — 모든 덤프 top 쓰레드 출현 빈도. |
| Per dump | 덤프별 상태 분포 테이블. |
| Threads | 쓰레드명별 출현 덤프 수 정렬 테이블. |
| **Lock contention** | Owner / waiter 그래프(`lock_addr`별 노드)와 wait graph DFS로 검출한 **데드락 사이클**. |
| **JVM signals** | 4개 서브탭 — **Carrier-pinning**(가상 쓰레드 캐리어가 monitor에 묶인 경우), **SMR / Zombies**(SafeMemoryReclamation 미해결 쓰레드), **Native methods**(쓰레드별 top JNI 프레임), **Class histogram**(jcmd JSON 포함 시 가장 참조 많은 클래스). |

### Findings 카탈로그

- **`LONG_RUNNING_THREAD`** *(warning)* — 같은 쓰레드명이 같은 RUNNABLE
  스택을 ≥ N 연속 덤프 동안 유지.
- **`PERSISTENT_BLOCKED_THREAD`** *(critical)* — BLOCKED / LOCK_WAIT
  상태로 ≥ N 연속 덤프 동안 유지.
- **`LATENCY_SECTION_DETECTED`** *(warning)* — `NETWORK_WAIT`,
  `IO_WAIT`, `CHANNEL_WAIT` 중 하나로 ≥ N 연속 덤프 유지. Wait
  카테고리는 언어별 enrichment 플러그인이 채움.
- **`GROWING_LOCK_CONTENTION`** *(warning)* — 동일 lock 주소의 waiter
  수가 연속 덤프에 걸쳐 단조 증가.
- **`THREAD_CONGESTION_DETECTED`** *(warning)* — 단일 덤프에서 RUNNABLE
  쓰레드 수가 CPU 수의 한 자릿수 배 이상 초과.
- **`EXTERNAL_RESOURCE_WAIT_HIGH`** *(warning)* — 30%+ 쓰레드가
  `NETWORK_WAIT` / `IO_WAIT` 동시 점유.
- **`LIKELY_GC_PAUSE_DETECTED`** *(warning)* — 거의 모든 쓰레드가
  RUNNABLE이고 VM internal 쓰레드(`VM Thread` / `GC task thread`)가 GC
  프레임을 보유.
- **`VIRTUAL_THREAD_CARRIER_PINNING`** *(warning)* — Loom 캐리어
  쓰레드가 가상 쓰레드의 monitor에 의해 pinned.
- **`SMR_UNRESOLVED_THREAD`** *(warning)* — `_threads` SMR 리스트가
  덤프에 보이지 않는 쓰레드를 참조.

언어별 enrichment 매트릭스와 감지 시그니처는
[`MULTI_LANGUAGE_THREADS.md`](MULTI_LANGUAGE_THREADS.md) 참고.

---

## Exception Analyzer

전용 페이지(placeholder 셸 아님). Java exception stack 파일
(`*.txt` / `*.log`)을 드롭하면:

- **Summary 메트릭** — 총 이벤트, unique 타입, unique 시그니처, 첫/마지막 발생 시각.
- **Top types** — 예외 클래스 카운트 막대 + 테이블. 테이블은 **간단
  클래스명**(예: `NullPointerException`)을 보여주고 hover 시 풀 FQN.
  깊은 패키지 경로로 레이아웃이 깨지지 않습니다.
- **Top stack signatures** — 정규화된 스택 시그니처 top 10.
- **이벤트 테이블** — 페이지네이션 + 필터(메시지 / 타입 / 시그니처
  검색). 행 클릭 시 `Sheet` 팝업으로 풀 메시지, 시그니처, 스택을
  복사 가능한 형태로 표시.
- **Parser Report** — 파서 진단.

긴 이벤트 리스트는 페이지를 무한히 늘리지 않고 페이지네이션됩니다.

---

## JFR Analyzer

바이너리 `.jfr` 또는 `jfr print --json recording.jfr`의 JSON을 드롭.
ArchScope가 바이너리 헤더(`FLR\0`)를 감지하면 JDK `jfr` CLI를 자동
호출해 변환합니다. CLI 탐색 순서:

1. `ARCHSCOPE_JFR_CLI` 환경 변수(전체 경로).
2. `JAVA_HOME/bin/jfr`.
3. PATH 상의 `jfr`.

CLI를 찾지 못하면 깔끔한 오류와 함께 JDK 11+ 설치를 안내합니다.

### 필터

| 파라미터 | 값 | 효과 |
| --- | --- | --- |
| `event_modes` | `cpu` / `wall` / `alloc` / `lock` / `gc` / `exception` / `io` / `nativemem` (콤마) | 집계 대상 이벤트 카테고리 제한. |
| `time_from` / `time_to` | ISO / `HH:MM:SS` / 상대(`+30s`, `-2m`, `500ms`) | 분석 윈도우. |
| `thread_state` | 상태 콤마 | 해당 상태 샘플만 카운트(`RUNNABLE`, `BLOCKED`, `WAITING`, …). |
| `min_duration_ms` | 숫자 | 임계치 미만 이벤트 drop. |

페이지 상단 폼 입력으로 노출됩니다.

### 탭

| 탭 | 내용 |
| --- | --- |
| Summary | 이벤트 수, distinct 이벤트 타입, 시간 범위, 샘플 최다 쓰레드. |
| Event distribution | 이벤트 타입 카운트 D3 horizontal bar. |
| Top samples | top 50 실행 샘플(thread / state / sample / top frame). |
| **Heatmap** | 1D wall-clock 밀도 strip — 영역 드래그 시 다음 분석에 자동으로 `time_from` / `time_to`를 채움. |
| GC summary | `gc` 이벤트가 포함된 경우 pause percentile 요약. |
| **Native memory** | native-memory 이벤트가 있으면 alloc/free 페어링 — recording 안에서 free되지 않은 site를 tail-ratio cutoff로 표시. 바이트 가중 플레임 뷰. |
| Parser Report | 파서 진단. |

---

## Demo Data Center

본인 로그가 없으면 번들 fixture
(`projects-assets/test-data/demo-site/`)를 실행할 수 있습니다.

- 매니페스트 root 경로를 입력 또는 디렉토리 선택.
- **All / synthetic / real** 데이터 소스 필터.
- 단일 시나리오 드롭다운(선택).
- **Run demo data** 클릭.

결과 카드에 집계 통계 + 시나리오별 상세 카드와 artifact 테이블 표시.
JSON artifact의 **Send to Export Center**로 Export Center에 자동
주입.

---

## Export Center

완성된 `AnalysisResult` JSON을 최종 보고서로 렌더링:

| 포맷 | 결과 |
| --- | --- |
| `html` | 단일 파일 portable HTML 보고서(차트 + 테이블 + findings). |
| `pptx` | 요약 + 핵심 차트가 들어간 minimal PowerPoint 덱. |
| `diff` | Before-after 비교 — JSON diff와 HTML diff 동시 출력. |

포맷 선택 → JSON 파일 browse → (옵션) 제목 / diff label 입력 →
**Run export**. 결과 패널에 엔진이 작성한 모든 파일 표시.

---

## Chart Studio

내장 차트 템플릿을 샘플 `AnalysisResult`로 미리보고 편집:

- 엔진이 제공하는 모든 차트 레이아웃 확인.
- 템플릿별 SVG vs canvas 렌더러 토글.
- 인라인 ECharts option JSON 편집(Edit → Apply / Reset).
- **Load JSON** — 디스크의 분석기 JSON을 선택해서 현재 템플릿 다시
  렌더링. 파일은 `/api/files?path=...`로 가져옴.

---

## Settings

- **Engine path** — 비워두면 번들 엔진 사용. 별도
  `archscope-engine` 바이너리를 운영할 때만 입력.
- **Theme** — 3-button 선택(Light / Dark / System), 글로벌
  ThemeProvider와 연동, `localStorage`에 저장.
- **Default chart theme** — ECharts 패널의 light/dark 기본값.
- **Locale** — English 또는 한국어(저장됨).
- **Save settings** → `~/.archscope/settings.json`에 저장.
  **Reset to default**는 클라이언트 측만 초기화 — 저장하려면 Save 클릭.

---

## AI 해석 (선택 사항)

ArchScope는 deterministic 분석 위에 **로컬 LLM**을 활용한 AI 보조
finding을 선택적으로 생성할 수 있습니다. AI 출력은 항상 evidence에
바인딩되며 규칙 기반 finding과 분리되어 표시됩니다. 클라우드 API 호출
없이 [Ollama](https://ollama.com/)를 통해 사용자 머신에서 전부
실행됩니다.

### Ollama 설치

#### Windows

1. <https://ollama.com/download/windows>에서 Windows 인스톨러를
   다운로드합니다.
2. `OllamaSetup.exe`를 실행하고 설치 마법사를 따릅니다. Ollama는
   백그라운드 서비스로 설치되며 자동으로 시작됩니다.
3. **PowerShell** 또는 **명령 프롬프트**를 열고 확인합니다:

   ```powershell
   ollama --version
   ```

> **참고:** Windows에서 Ollama를 사용하려면 Windows 10 버전 22H2
> 이상이 필요합니다. 적절한 추론 속도를 위해 최신 드라이버가 설치된
> GPU를 권장하지만, CPU 전용 모드도 동작합니다(느림).

#### macOS

```bash
brew install ollama            # Homebrew
# — 또는 https://ollama.com/download/mac 에서 .dmg 다운로드
ollama serve                   # 로컬 서버 시작 (앱을 사용하지 않는 경우)
```

#### Linux

```bash
curl -fsSL https://ollama.com/install.sh | sh
systemctl start ollama         # 또는: ollama serve
```

### 권장 모델 다운로드

ArchScope는 시작 모델로 `qwen2.5-coder:7b`를 제안합니다(디스크 ~5 GB,
RAM 16 GB 권장):

```bash
ollama pull qwen2.5-coder:7b
```

Ollama 호환 모델이면 무엇이든 사용할 수 있습니다. 모델을 변경하려면
`~/.archscope/settings.json`의 `ai.model`을 수정하거나, UI의 Settings
페이지에서 AI 설정이 노출될 때 변경하세요.

### 연결 확인

Ollama가 실행되면 ArchScope가 `http://localhost:11434`를 자동으로
감지합니다. 아무 분석기나 실행하면 — 로컬 모델이 사용 가능할 경우 —
결과 페이지 deterministic finding 아래에 **AI Interpretation** 패널이
나타납니다. Ollama가 실행 중이 아니거나 모델이 pull되지 않은 경우, AI
해석은 자동으로 비활성화되며 나머지 분석은 정상 동작합니다.

### Windows 전용 팁

- **방화벽:** Ollama는 기본적으로 `127.0.0.1:11434`에 바인딩합니다.
  바인드 주소를 변경하지 않았다면 인바운드 방화벽 규칙이 필요 없습니다.
- **GPU 가속:** Ollama는 Windows에서 NVIDIA(CUDA)와 AMD(ROCm) GPU를
  자동 감지합니다. 최신 GPU 드라이버가 설치되어 있는지 확인하세요.
  GPU가 감지되지 않으면 CPU에서 추론이 실행됩니다.
- **서비스 실행:** Windows 인스톨러는 Ollama를 시작 서비스로
  등록합니다. **작업 관리자 → 시작 앱**에서 관리하거나, 관리자
  프롬프트에서 `sc stop ollama` / `sc start ollama`로 제어할 수
  있습니다.
- **프록시 / 에어갭 환경:** 인터넷이 없는 머신이라면 다른 머신에서
  모델 파일을 복사할 수 있습니다. 인터넷이 되는 머신에서 모델을 pull한
  뒤, `%USERPROFILE%\.ollama\models` 디렉토리를 에어갭 호스트로
  복사하세요.

전체 AI 해석 설계(evidence 요구사항, prompt 구조, validation 규칙)는
[`AI_INTERPRETATION.md`](AI_INTERPRETATION.md)를 참고하세요.

---

## 이미지 내보내기 & "모든 차트 저장"

### 차트별 export

모든 차트 프레임(D3 + ECharts + Canvas)의 헤더 우측에 다운로드 아이콘.
드롭다운 매트릭스:

| Preset | Format | Pixel ratio | Notes |
| --- | --- | --- | --- |
| `PNG · 1x` | PNG | 1 × | 가장 작음, 화면 해상도. |
| `PNG · 2x` | PNG | 2 × | 슬라이드 덱 기본. |
| `PNG · 3x` | PNG | 3 × | 인쇄 해상도. |
| `JPEG · 2x` | JPEG | 2 × | PNG보다 작음, 이메일에 유용. |
| `SVG (vector)` | SVG | n/a | 진짜 벡터 — Figma / Illustrator 편집에 최적. ECharts 패널과 D3-SVG 차트는 native SVG로 렌더, Canvas 차트는 `html-to-image`로 SVG 래스터화. |

Canvas 플레임그래프는 추가로 **"Save PNG"** 버튼 — native
`canvas.toDataURL()` 경로로 가장 빠르고 device pixel ratio에 맞춘
픽셀 퍼펙트.

### 일괄 export

대부분의 분석기 페이지에 TabsList 옆 **"모든 차트 저장"** 버튼.
클릭하면 현재 페이지의 모든 차트를 2× PNG로 작성하고 페이지별
파일명 prefix(`gc-log-…`, `profiler-…`, `thread-dump-multi-…`,
`access-log-…`) 적용.

---

## Thread → Flamegraph 변환

장기간 인시던트를 플레임그래프로 보고 싶으면 덤프를 collapsed로
배치 변환:

```bash
archscope-engine thread-dump to-collapsed \
    --input dump-2025-05-04T10-00.txt \
    --input dump-2025-05-04T10-05.txt \
    --input dump-2025-05-04T10-10.txt \
    --output incident-2025-05-04.collapsed \
    [--format <plugin-id>] \
    [--no-thread-name]
```

변환기 동작:

1. 파서 레지스트리 사용(Java / Go / Python / Node.js / .NET 자동 감지;
   `--format`으로 강제).
2. 모든 언어별 enrichment 적용 — CGLIB / Express layer alias /
   async state machine / 프레임워크 래퍼 정규화.
3. 스택을 root-left로 뒤집음(collapsed convention).
4. 같은 스택을 가진 다른 쓰레드가 합쳐지지 않게, sanitize된 쓰레드명을
   합성 root frame으로 prepend(`--no-thread-name`으로 끄기).
5. 모든 입력 파일에 걸쳐 동일 스택 자동 집계.

결과를 다시 프로파일러로:

```bash
archscope-engine profiler analyze-collapsed \
    --wall incident-2025-05-04.collapsed \
    --wall-interval-ms 1 \
    --out incident-flame.json
```

웹 UI는 동일 변환을 `POST /api/analyzer/execute`의
`type: "thread_dump_to_collapsed"`로 노출.
응답은 `~/.archscope/uploads/collapsed/`에 작성된 `outputPath`와
총 `uniqueStacks`를 포함합니다.

---

## CLI 레퍼런스

```bash
# ---------- 웹 서버 ----------
archscope-engine serve [--host 127.0.0.1] [--port 8765]
                       [--static-dir apps/frontend/dist] [--reload]
                       [--no-dev-cors]

# ---------- access log ----------
archscope-engine access-log analyze --file <log> --format <name> --out <result.json>
    [--max-lines N] [--start-time ISO] [--end-time ISO]

# ---------- profiler ----------
archscope-engine profiler analyze-collapsed --wall <collapsed> --out <result.json>
    [--wall-interval-ms 100] [--elapsed-sec N] [--top-n 20] [--profile-kind wall|cpu|lock]
archscope-engine profiler analyze-flamegraph-svg  --file <svg>  --out <result.json>
archscope-engine profiler analyze-flamegraph-html --file <html> --out <result.json>
archscope-engine profiler analyze-jennifer-csv    --file <csv>  --out <result.json>
archscope-engine profiler diff   --baseline <r1.json> --target <r2.json> --out <diff.json>
                                  [--normalize] [--top-n 50]
archscope-engine profiler export-pprof --input <result.json> --output <profile.pb.gz>
archscope-engine profiler drilldown   ...           # --help 참고
archscope-engine profiler breakdown   ...

# ---------- GC ----------
archscope-engine gc-log analyze --file <gc.log> --out <result.json> [--top-n 20]

# ---------- JFR ----------
archscope-engine jfr analyze      --file <jfr|jfr.json> --out <result.json>
    [--event-modes cpu,wall,alloc,lock,gc,exception,io,nativemem]
    [--time-from <ts>] [--time-to <ts>] [--thread-state RUNNABLE,BLOCKED,...]
    [--min-duration-ms N] [--top-n 20]
archscope-engine jfr analyze-json --file <jfr.json> --out <result.json> [--top-n 20]

# ---------- thread dump ----------
archscope-engine thread-dump analyze       --file <dump> --out <result.json>
archscope-engine thread-dump analyze-multi --input <f> --input <f> ... --out <multi.json>
    [--format <plugin-id>] [--consecutive-threshold 3] [--top-n 20]
archscope-engine thread-dump to-collapsed  --input <f> --input <f> ... --output <collapsed>
    [--format <plugin-id>] [--no-thread-name]

# ---------- exception / language stacks ----------
archscope-engine exception        analyze --file <ex>    --out <result.json>
archscope-engine nodejs           analyze --file <stack> --out <result.json>
archscope-engine python-traceback analyze --file <stack> --out <result.json>
archscope-engine go-panic         analyze --file <stack> --out <result.json>
archscope-engine dotnet           analyze --file <stack> --out <result.json>

# ---------- OpenTelemetry ----------
archscope-engine otel analyze --file <events.jsonl> --out <result.json>

# ---------- 보고서 ----------
archscope-engine report html --input <result.json> --out <report.html> [--title "..."]
archscope-engine report pptx --input <result.json> --out <report.pptx> [--title "..."]
archscope-engine report diff --before <before.json> --after <after.json>
                             --out <diff.json> [--label "..."] [--html-out <diff.html>]

# ---------- demo bundles ----------
archscope-engine demo-site mapping [--manifest-root <dir>]
archscope-engine demo-site run     --manifest-root <dir> --out <bundle-dir>
                                   [--scenario name] [--data-source real|synthetic] [--no-pptx]
```

`archscope-engine --help`로 모든 명령 확인. 각 서브커맨드도 `--help`
지원.

---

## 트러블슈팅 FAQ

**브라우저가 "ArchScope API is running" 메시지만 보여줘요.**
React 빌드 없이 엔진을 시작했습니다.
`./scripts/serve-web.sh`를 쓰거나, `npm --prefix apps/frontend run
build`를 한 번 실행한 뒤 `archscope-engine serve --static-dir
apps/frontend/dist`로 기동하세요. dev 루프(`archscope-engine serve
--reload` + `npm run dev`)는 UI를 `:5173`에서 엽니다.

**Thread dump 업로드 시 `UNKNOWN_THREAD_DUMP_FORMAT`.**
첫 4 KB가 어떤 등록된 파서와도 매칭되지 않았습니다. 덤프 자체가 아닌
주변 로그 파일을 올렸는지 확인. 헤더가 잘려 있으면 Thread Dump
페이지의 **Format override**에 `format_id`를 입력
(예: `java_jstack`).

**멀티 덤프 분석 시 `MIXED_THREAD_DUMP_FORMATS`.**
업로드한 두 파일이 다른 파서로 인식됨. 한 파일을 제거하거나 모든
파일에 같은 파서를 강제하려면 **Format override** 입력.

**SVG 렌더러로도 플레임그래프가 느려요.**
데이터를 thread-dump 스타일 번들로 변환(`thread-dump to-collapsed`
실행) 후 `flamegraph collapsed`로 분석하세요. 4 000 노드 이상이면
페이지가 자동으로 Canvas로 전환됩니다. 추가 단순화로 Display
툴바의 **Min width**를 0.5 % 등으로 올리면 그 미만 프레임은 `…`
집계로 접힙니다.

**바이너리 `.jfr` 분석 시 `JFR_CLI_NOT_FOUND`.**
ArchScope는 `.jfr` → 이벤트 JSON 변환을 위해 JDK `jfr` CLI를
호출합니다. JDK 11+ 설치 후 `JAVA_HOME`을 설정(그래야
`JAVA_HOME/bin/jfr`가 잡힘)하거나, `ARCHSCOPE_JFR_CLI`로 실행 파일
경로를 직접 지정하세요. 또는 미리 `jfr print --json recording.jfr >
out.json`으로 변환하고 그 JSON을 분석해도 됩니다.

**이미지 export가 빈 PNG를 다운로드합니다.**
차트가 렌더링을 끝내기 전이었습니다. 데이터가 안정될 때까지(차트는
`ResizeObserver` 사용) 기다린 후 다시 시도. 매우 큰 Canvas
플레임그래프는 전용 **Save PNG** 버튼 사용을 권장 —
`canvas.toDataURL()`을 직접 사용하므로 `html-to-image` rasterize를
우회.

**다크 테마에서 레거시 ECharts 패널이 깨져 보입니다.**
**Settings → Default chart theme**에서 ECharts 테마 전환. ECharts
패널은 글로벌 테마를 자동으로 따르지 않습니다 — 새 D3 차트만 자동
적용.

**분석 후 AI interpretation 패널이 나타나지 않아요.**
Ollama가 실행 중이 아니거나 설정된 모델이 pull되지 않은 상태입니다.
터미널에서 `ollama list`를 실행해 사용 가능한 모델을 확인하세요.
목록이 비어 있으면 `ollama pull qwen2.5-coder:7b`를 실행하세요.
Windows에서는 작업 관리자에서 Ollama 서비스가 실행 중인지 확인하거나
PowerShell에서 `ollama serve`로 시작하세요. ArchScope는
`http://localhost:11434`를 확인합니다 — Ollama가 다른 주소에 바인딩된
경우 `~/.archscope/settings.json`의 `ai.provider_url`을 수정하세요.

**Windows에서 AI 해석이 매우 느려요.**
Ollama가 CPU 전용 모드로 실행 중이면(GPU 미감지) 7B 모델의 해석에
30초 이상 걸릴 수 있습니다. 최신 NVIDIA 또는 AMD GPU 드라이버를
설치해서 Ollama가 GPU에 오프로드하게 하세요. 또는
`qwen2.5-coder:3b` 같은 작은 모델을 사용할 수 있습니다(정확도는
낮지만 빠름). `~/.archscope/settings.json`의 `ai.timeout_seconds`를
더 높은 값으로 설정해 타임아웃을 늘릴 수도 있습니다(기본: 30).

**업로드 파일은 어디에 저장되나요?**
`~/.archscope/uploads/<uuid>/<original-name>`. 언제든 디렉토리 삭제
가능. 다음 업로드 때 ArchScope가 다시 만듭니다.

**LAN의 동료가 엔진에 접근하게 하려면?**
`--host 0.0.0.0` 전달. 엔진에 인증이 없으니 신뢰할 수 있는 네트워크
에서만 사용. 공개 인터넷에 절대 두지 마세요.
