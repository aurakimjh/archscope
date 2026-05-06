# ArchScope 아키텍처

ArchScope는 로컬 우선의 애플리케이션 아키텍처 진단 및 보고서 작성
toolkit입니다. 핵심 책임은 운영 데이터를 정규화된 `AnalysisResult`
JSON과 보고서용 시각화로 변환하는 것이며, 외부 SaaS로 데이터를 보내지
않습니다.

## 웹 플랫폼 pivot — 디자인 결정 (T-206)

**2026-05-06** 결정. **T-001** ("Electron + `child_process.execFile`
IPC")의 결정을 **supersede**. Phase 1의 나머지 작업(T-207–T-209)이 따르는
방향성 잠금.

### 런타임

- **FastAPI + uvicorn**, 분석기를 in-process로 dispatch (subprocess
  사용 안 함). 분석기는 일반 Python 함수 호출로 실행되며, T-001이 도입한
  IPC 경계는 같은 언어 안에 있는 엔진에 별다른 안정성 이득을 주지
  못했기에 거부함.
- 프로그래매틱 `archscope serve` 콘솔 진입점 (T-208)이 기본 `127.0.0.1:8765`에서
  uvicorn을 시작하고 첫 시작 시 시스템 브라우저를 엽니다.

### 전송

| 채널 | 운반 대상 | 이유 |
|---|---|---|
| **HTTP** (`/api/...`) | 분석기 실행, 설정, 파일 다이얼로그, 내보내기, 데모 실행, 파일 스트리밍. | 동기 요청/응답이 분석기 dispatch에 가장 단순한 fit이며 `apps/frontend/src/api/`의 기존 클라이언트 surface와 일치. |
| **WebSocket** (`/ws/progress`) | 엔진 진행률 이벤트, 취소 시그널, 파서 디버그 로그 스트리밍. | 시간이 오래 걸리는 분석기(멀티 덤프 상관, 대형 GC 로그)는 폴링 없이 중간 상태를 push해야 하고, 렌더러는 Wails 트랙(T-240f)과 대칭인 fire-and-forget 취소를 필요로 함. 단일 프로세스 취소도 동일한 task-registry 패턴 사용: 서버는 request id 키로 취소 채널을 매핑하고 `progress` / `done` / `cancelled` JSON 프레임을 emit. |

### 파일 선택

- **기본 — `POST /api/files/select`로 서버 측 절대 경로.**
  서버가 OS 파일 다이얼로그를 띄움 (엔진은 `127.0.0.1` bind 이므로 정의상
  로컬 전용)이며 절대 경로를 반환; 후속 분석기 호출은 그 경로를 직접
  참조. 같은 머신에서 사용하는 일반 케이스에서 multipart 업로드 왕복을
  회피.
- **폴백 — `POST /api/upload`로 브라우저 multipart 업로드.** 이미 구현됨;
  `~/.archscope/uploads/<uuid>/<orig>`에 작성. 엔진이 비-localhost 출처에서
  접근되거나 브라우저 sandbox가 다이얼로그 엔드포인트를 차단할 때 활성화.

### 패키징 (T-208 방향성)

- 단일 최상위 Python distribution `archscope`이 `archscope` 콘솔 스크립트를
  노출. 스크립트가 uvicorn을 wrap하므로
  `pip install archscope && archscope serve`가 전체 설치 경로.
- `apps/frontend/dist/`의 빌드된 React 번들은 wheel 패키지 데이터로 출하되며
  런타임에 `importlib.resources`로 해소 — 설치 시 복사 단계 없음, 별도
  static-file env var 없음.
- 기존 Electron 데스크톱 셸(`apps/desktop/`)은 T-207에서 폐기; 파일이
  삭제되고 React 셸은 `apps/frontend/`로 통합. `apps/profiler-native/`의
  Wails v3 네이티브 프로파일러는 영향 없음 (별도 트랙 — T-242에 결정 기록).

### CSP / CORS 방침

- **CORS** — Vite 개발 서버용으로 `allow_origins=["http://127.0.0.1:5173"]`.
  프로덕션은 동일 FastAPI 출처에서 React 번들을 서빙하므로 런타임에 CORS는
  사실상 사용되지 않음. `--no-dev-cors`로 dev allowlist를 완전 비활성화
  가능 (강화 배포 시).
- **CSP** — `default-src 'self'; img-src 'self' data:;
  style-src 'self' 'unsafe-inline'; script-src 'self';
  connect-src 'self' ws://127.0.0.1:8765`. `connect-src`의 ws:// 엔트리가
  렌더러의 `/ws/progress` 구독에 필요. `style-src 'unsafe-inline'`은
  shadcn/ui CSS 변수 때문에 유지; nonce 기반 강화는 T-052/T-071에서 별도
  추적.

### Apps 디렉토리 레이아웃 (T-207 이후)

```text
apps/
├ frontend/         # React 셸 — 웹 UI의 단일 소스
├ profiler-native/  # Wails v3 네이티브 프로파일러 (2026-05-05 결정, T-240a)
└ desktop/          # T-207에서 제거
```

기존 `apps/desktop/electron/`, `tsconfig.electron.json`,
`electron-builder` 설정, `release/`, `dist-electron/`은 삭제. 새 최상위
`archscope` Python distribution(패키지 데이터 + 콘솔 스크립트)은 T-208을
통해 리포 루트에 배치.

## 제품 위상

ArchScope의 위상은 **프라이버시 우선 로컬 전용 진단 워크벤치**입니다.

- 브라우저 UI의 편의성,
- 데스크톱 분석기의 로컬/오프라인 안전성,
- 보고서에 바로 쓸 수 있는 모던 시각화 (D3 + ECharts + Canvas),
- 5개 런타임을 지원하는 표준화된 evidence contract.

목표는 일반 log viewer나 본격적인 observability backend가 되는 것이
아닙니다. ArchScope는 오프라인 운영 데이터를 아키텍처 진단 결과 +
보고서 산출물로 정리하는 데 집중합니다.

## 시스템 흐름

```text
Raw Data
  → Parsing (per-format 파서 + plugin registry)
  → Analysis / Aggregation (도메인별 분석기 + 멀티 덤프 상관 분석)
  → Visualization (브라우저: D3 / Canvas / ECharts)
  → Report-ready Export (HTML / PowerPoint / diff)
```

## 런타임 토폴로지

```text
┌────────────────────────────────────────────────────────────────┐
│  브라우저 (React 18 + Vite + Tailwind v4 + shadcn/ui)           │
│   • AppShell  (TopBar + 접기 가능한 Sidebar + Tabs)             │
│   • httpBridge가 window.archscope를 FastAPI API에 연결           │
│   • 차트: D3 Flame / Canvas Flame / D3 Timeline / D3 Bar /     │
│          레거시 ECharts 패널                                    │
│   • 이미지 export: html-to-image + native canvas.toDataURL()   │
└──────────────────────────┬─────────────────────────────────────┘
                           │  fetch /api/...
                           ▼
┌────────────────────────────────────────────────────────────────┐
│  FastAPI 서버 (`archscope-engine serve`)                       │
│   • POST /api/upload                  multipart 업로드          │
│   • POST /api/analyzer/execute        dispatcher (type별)      │
│   • POST /api/analyzer/cancel         단일 프로세스 — no-op     │
│   • POST /api/export/execute          html / pptx / diff       │
│   • GET  /api/demo/list, POST /api/demo/run                    │
│   • GET  /api/files?path=…            artifact 스트리밍         │
│   • GET/PUT /api/settings             ~/.archscope/settings    │
│   • GET  /                            정적 React 빌드           │
└──────────────────────────┬─────────────────────────────────────┘
                           │  in-process 호출 (서브프로세스 없음)
                           ▼
┌────────────────────────────────────────────────────────────────┐
│  archscope_engine (pure Python)                                │
│                                                                │
│   parsers/                                                     │
│     access_log_parser, collapsed_parser, jennifer_csv_parser,  │
│     svg_flamegraph_parser, html_profiler_parser,               │
│     gc_log_parser + gc_log_header (JVM Info 추출),              │
│     jfr_recording (바이너리 `.jfr` → JSON, JDK `jfr` CLI),      │
│     jfr_parser (기존 JSON 경로),                                │
│     exception_parser, otel_parser,                             │
│     thread_dump/                                               │
│       registry.py     ← format-id, can_parse(head), parse(path)│
│       java_jstack.py  ← + AOP / IO enrichment + JDK 21+ no-`nid│
│       java_jcmd_json.py ← jcmd JSON.thread_dump_to_file        │
│       go_goroutine.py ← + 프레임워크 정리 + 상태 추론           │
│       python_dump.py  ← py-spy / faulthandler                  │
│       python_traceback.py ← Thread ID + File "...", line N     │
│       nodejs_report.py← diagnostic-report JSON + libuv state   │
│       nodejs_sample_trace.py ← Sample # + at fn(file:line:col) │
│       dotnet_clrstack ← + async state machine 정리             │
│       dotnet_environment_stacktrace ← Environment.StackTrace   │
│                                                                │
│   analyzers/                                                   │
│     access_log_analyzer, profiler_analyzer (collapsed/SVG/     │
│     HTML/Jennifer), profiler_diff (빨강=느려짐 / 파랑=빨라짐),  │
│     native_memory_analyzer (alloc/free 페어링),                 │
│     gc_log_analyzer, jfr_analyzer,                             │
│     thread_dump_analyzer (단일 덤프, JVM 전용),                 │
│     multi_thread_analyzer (LONG_RUNNING_THREAD,                │
│         PERSISTENT_BLOCKED_THREAD, LATENCY_SECTION_DETECTED,   │
│         GROWING_LOCK_CONTENTION, THREAD_CONGESTION_DETECTED,   │
│         EXTERNAL_RESOURCE_WAIT_HIGH, LIKELY_GC_PAUSE_DETECTED, │
│         VIRTUAL_THREAD_CARRIER_PINNING, SMR_UNRESOLVED_THREAD),│
│     lock_contention_analyzer (owner/waiter 그래프 + DFS 데드락),│
│     thread_dump_to_collapsed,                                  │
│     exception_analyzer, runtime_analyzer, otel_analyzer,       │
│     ai_interpretation, profiler_breakdown, profiler_drilldown  │
│                                                                │
│   exporters/                                                   │
│     json_exporter, html_exporter, pptx_exporter, report_diff,  │
│     pprof_exporter (자체 minimal protobuf, 의존성 없음)         │
│                                                                │
│   models/                                                      │
│     AnalysisResult (전송 boundary 단일),                        │
│     FlameNode (diff용 metadata: {a, b, delta}),                 │
│     ThreadSnapshot + ThreadDumpBundle + ThreadState,           │
│     StackFrame, ExceptionRecord, GcEvent, …                    │
│                                                                │
│   web/server.py     ← FastAPI factory + analyzer dispatcher    │
│   cli.py            ← Typer 명령 (분석기별 + serve)             │
└────────────────────────────────────────────────────────────────┘
```

## 컴포넌트

### 브라우저 앱 (`apps/frontend/`)

React 18 — FastAPI가 정적 번들로 서빙(개발 시에는 Vite dev 서버).
같은 번들이 `apps/desktop/`의 Electron 셸 안에도 패키징되어 `file://`
로 로드되며, `apiBase` 헬퍼가 번들된 엔진(`127.0.0.1:8765`)으로
라우팅합니다. `httpBridge`(`src/api/httpBridge.ts`)가 레거시 Electron
빌드와 동일한 `window.archscope.*` 표면을 노출하지만, 모든 호출이
FastAPI 엔진에 대한 `fetch()`로 변환됩니다. 페이지는 절대로 파서를
import하지 않으며 오직 정규화된 `AnalysisResult` JSON만 렌더합니다.

차트 계층 분리:

- **D3** — 신규 차트(`D3FlameGraph`, `D3TimelineChart`,
  `D3BarChart`)와, 4 000 노드 초과 시 자동 전환되는 Canvas 백엔드의
  `CanvasFlameGraph`.
- **ECharts** — 레거시 패널(access-log request rate trend, profiler
  breakdown 도넛/막대). `ChartPanel.tsx`가 동일한 shadcn 스타일
  툴바로 감싸서 차트별 export 드롭다운이 일관되게 동작.

셸은 Tailwind v4 + `@tailwindcss/vite` 플러그인 + shadcn/ui 토큰
시트(light/dark CSS 변수). `ThemeProvider`가 `<html>`의 `.dark` 클래스
를 토글하고 선택값을 `localStorage`에 저장.

### FastAPI 엔진 (`engines/python/archscope_engine/web/`)

`web.server.create_app()` 연결 라우팅:

- `/api/upload` — multipart, `~/.archscope/uploads/<uuid>/`에 저장,
  이후 분석기 호출이 사용할 서버 측 경로 반환.
- `/api/analyzer/execute` — `type` 기반 단일 dispatcher
  (`access_log`, `profiler_collapsed`, `profiler_diff`,
  `profiler_export_pprof`, `gc_log`, `thread_dump`, `thread_dump_multi`,
  `thread_dump_to_collapsed`, `exception_stack`, `jfr_recording`,
  `flamegraph_svg`, `flamegraph_html`).
  분석기를 **in-process**로 호출(서브프로세스 없음)하고 전체
  `AnalysisResult` JSON 반환. 엔진은 `127.0.0.1`에 바인딩되고 번들된
  Electron 빌드는 UI를 `file://`에서 로드하기 때문에 CORS는
  `allow_origins=["*"]`로 풀어둡니다.
- `/api/export/execute` — HTML / PPTX / before-after diff export.
- `/api/demo/list`, `/api/demo/run` — demo-site fixture runner.
- `/api/files?path=…` — 임의 로컬 파일 스트리밍(UI가 export된 보고서/
  artifact를 열 때 사용).
- `/api/settings` — `~/.archscope/settings.json`에 영속.
- `/` — `--static-dir` 지정 시 React 정적 빌드 서빙.

CORS allow-list는 기본적으로 `http://127.0.0.1:5173`(Vite dev
서버)에 활성화. 프로덕션 스타일 서빙에는 `--no-dev-cors`로 끄기.

### 엔진 패키지 (`engines/python/archscope_engine/`)

순수 Python, GUI 의존성 없음. 레이어드:

- **`parsers/`** — 원천 파일 → typed record. Thread-dump 계열은
  plugin 기반 — 각 런타임이 `ThreadDumpParserPlugin` (`format_id`,
  `can_parse(head: str)`, `parse(path) -> ThreadDumpBundle`)을
  레지스트리에 등록. 레지스트리는 모든 입력의 첫 4 KB를 검사해
  dispatch. 멀티 덤프에서 두 파일이 다른 포맷이면 `MixedFormatError`
  로 즉시 거부 — `format_override` 지정 시 우회.
- **`analyzers/`** — typed record → `AnalysisResult`. 멀티 덤프
  상관 분석기(`multi_thread_analyzer`)는 의도적으로 언어 비의존이며,
  런타임별 enrichment(CGLIB 정리, 네트워크/IO 상태 추론, async state
  machine demangling, …)는 파서 플러그인 안에 살기 때문에 상관
  분석기는 오직 `ThreadState` enum만 소비.
- **`exporters/`** — `AnalysisResult` → JSON / HTML / PPTX / diff.
- **`models/`** — 공유 dataclass. `AnalysisResult`가 엔진과 UI
  사이의 단일 전송 boundary.

### `AnalysisResult` contract

모든 분석기가 같은 envelope을 발행:

```text
AnalysisResult {
  type: str                  # 예: "profiler_collapsed", "thread_dump_multi"
  source_files: list[str]
  created_at: str            # ISO 8601
  summary: dict              # 메트릭 카드용 스칼라
  series: dict               # 차트 패널용 배열
  tables: dict               # shadcn / D3 테이블 row
  charts:  dict              # raw 차트 데이터 (예: flamegraph 트리)
  metadata: {
    parser: str,
    schema_version: "0.2.0",
    diagnostics: ParserDiagnostics,
    findings?: list[Finding],
    drilldown_current_stage?: …,
    detected_html_format?: …, ai_interpretation?: …,
  }
}
```

브라우저는 이 형태만 렌더합니다. 신규 분석기는 contract만 만족하면
별도 UI 배선이 필요 없음.

## 디스크 / 저장 위치

| 경로 | 소유자 | 용도 |
| --- | --- | --- |
| `~/.archscope/uploads/<uuid>/<orig>` | 업로드 엔드포인트 | multipart 업로드 — 분석기 dispatch 입력 |
| `~/.archscope/uploads/collapsed/<uuid>.collapsed` | thread→flamegraph 변환기 | 서버 측 변환 출력 |
| `~/.archscope/settings.json` | 설정 엔드포인트 | engine path / chart theme / locale |
| `<repo>/apps/frontend/dist/` | Vite | `--static-dir`이 서빙하는 React 빌드 (Electron 셸 안에도 동일하게 번들) |
| `<repo>/apps/desktop/dist/` | electron-builder | NSIS 인스톨러 + 포터블 zip 출력 |
| `<repo>/engines/python/dist/` | PyInstaller | Electron 패키지에 임베드되는 `archscope-engine` 단일 바이너리 |
| `<repo>/archscope-debug/` | 파서 debug log | 지원용 redacted 파싱 오류 컨텍스트 |

## 의도적인 범위 밖

- **힙 덤프 / `.hprof` 분석** — 범위 밖. ArchScope는 *왜 쓰레드가
  멈췄는가*를 보는 도구이지 *할당이 어디에 살아 있는가*를 보는 도구가
  아닙니다.
- **Live agent / 런타임 모니터링** — ArchScope는 수집된 artifact만
  소비합니다.
- **멀티 테넌트 SaaS / 인증** — 엔진은 기본적으로 `127.0.0.1`에
  바인딩되며 인증 레이어가 없습니다. `--host 0.0.0.0`은 신뢰할 수
  있는 LAN 전용.
- **async-profiler 3.x packed-binary HTML** — 지원하는 HTML 변형은
  인라인 SVG와 레거시 embedded-tree JS 형태입니다. 3.x에서는
  `asprof`에 `--format svg`로 추출하세요.

언어별 thread-dump 매트릭스와 감지 규칙은
[`MULTI_LANGUAGE_THREADS.md`](MULTI_LANGUAGE_THREADS.md) 참고.
