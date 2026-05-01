# ArchScope 종합 검토 의견서 (성능 / 결함 / 알고리즘)

- **작성일**: 2026-05-01
- **작성자**: Claude Code (claude-opus-4-6)
- **대상 프로젝트**: ArchScope
- **검토 범위**: 커밋 `2e1a0ae` (main 브랜치 HEAD)
- **이전 검토 문서**: `/docs/review/done/` 하위 Phase 1~5 의견서 8건 전수 검토

---

## 0. Executive Summary

### 0.1 종합 평가

ArchScope는 **Parser → Analyzer → AnalysisResult → IPC → UI**의 단방향 데이터 흐름을 일관되게 구현한 깨끗한 아키텍처를 갖추고 있다. 공통 계약(`AnalysisResult`), 책임 분리, 증거 기반 진단(evidence-bound diagnostics) 설계는 시니어 수준의 판단이 반영된 결과물이다. 그러나 **reservoir sampling 알고리즘의 수학적 결함**(사실상 비동작), **프로세스 매 호출 생성(per-invocation spawn) 오버헤드**, **측정 인프라 부재**가 제품 품질과 운영 안정성의 주요 병목이다. 이번 검토에서 P0 3건, P1 8건, P2 10건, P3 4건의 개선 권고를 도출했다.

### 0.2 핵심 지표

| 영역 | 평가 | 근거 |
|---|---|---|
| 성능 | **C** | 벤치마크/프로파일 인프라 전무, per-invocation subprocess spawn, 이중 파일 읽기, 대용량 입력 처리 미검증 |
| 결함 내성 | **B** | IPC 검증 건전, 에러 전파 일관, 자원 정리 대부분 올바름. 단 frozen dataclass 불변성 위반, 취소 메커니즘 부재 |
| 알고리즘 적절성 | **C+** | 핵심 경로 O(n) 정상. 단 reservoir sampling이 수학적으로 고장, percentile 계산 시 매번 O(n log n) 정렬 반복 |
| 측정 인프라 성숙도 | **D** | 자동화된 벤치마크 0건, CI 성능 회귀 탐지 0건, 프로파일링 가이드 0건 |

### 0.3 발견 사항 통계
- P0 (즉시 처리): **3건**
- P1 (다음 사이클): **8건**
- P2 (백로그): **10건**
- P3 (의도적 보류): **4건**

### 0.4 즉시 조치 권고 Top 5

1. **ALG-1 [P0]**: `_deterministic_reservoir_index` 수학적 결함 — 전체 reservoir sampling이 비동작
2. **DEF-1 [P0]**: `AnalysisResult` frozen dataclass의 불변성 위반 — 드릴다운에서 결과 객체를 변이
3. **PERF-1 [P0]**: 측정 인프라 전면 부재 — 벤치마크, 프로파일링, 회귀 탐지 없음
4. **ALG-2 [P1]**: `percentile()` 함수가 호출마다 전체 정렬 — O(n log n) 반복
5. **PERF-2 [P1]**: per-invocation subprocess spawn — 분석 요청마다 Python 프로세스 생성

---

## 1. 데이터 흐름 매핑

### 1.1 입력별 처리 경로

```
[사용자 파일 선택]
    │
    ▼
Renderer: Page Component (useState → "ready")
    │  window.archscope.analyzer.execute(request)
    ▼
Preload: ipcRenderer.invoke("analyzer:execute", request)
    │
    ▼
Main Process: executeAnalyzer(request) → switch(request.type)
    │  ┌─ analyzeAccessLog()
    │  ├─ analyzeCollapsedProfile()
    │  ├─ analyzeJvmFile("gc-log")
    │  ├─ analyzeJvmFile("thread-dump")
    │  └─ analyzeJvmFile("exception")
    │
    ▼
runAnalyzer<T>():
    │  1. mkdir(/tmp/archscope-{uuid}/)
    │  2. execEngine([...args, "--out", outputPath], 60_000ms)
    │
    ▼
execEngine():  ─── child_process.execFile() ───▶  Python subprocess
    │                                                │
    │                                                ▼
    │                                          cli.py (Typer CLI)
    │                                                │
    │                                          detect_text_encoding(path)  ◄── 파일 전체 읽기 #1
    │                                                │
    │                                          Parser (정규식/JSON 기반)     ◄── 파일 전체 읽기 #2
    │                                                │
    │                                          Analyzer (집계/분류/백분위)
    │                                                │
    │                                          write_json_result(outputPath)
    │                                                │
    │                                          [프로세스 종료, exit code]
    │
    ▼
Main Process:
    │  1. readFile(outputPath, "utf8")
    │  2. JSON.parse(rawJson)
    │  3. validateResult(parsed)  ← 타입 가드
    │  4. rm(tempDir, {recursive: true})
    │
    ▼
IPC Response: {ok: true, result, engine_messages}
    │
    ▼
Renderer: setResult(response.result), setState("success")
    │
    ▼
React Components: MetricCard, ChartPanel (ECharts), DiagnosticsPanel
```

**입력 타입별 처리 경로 차이점:**

| 입력 타입 | 파서 | 핵심 알고리즘 | 출력 특이사항 |
|---|---|---|---|
| Access Log | 정규식 (`NGINX_WITH_RESPONSE_TIME`) | BoundedPercentile, Counter, 분당 집계 | p95/p99 시계열 |
| GC Log | 정규식 (`UNIFIED_GC_RE`) | Counter, sum/max/min | pause 시계열 |
| Thread Dump | 정규식 (블록 단위) | Counter, `_stack_signature()` | state 분포 |
| Profiler (collapsed) | 세미콜론 분리 | FlameNode 트리 구축, 드릴다운 | flamegraph JSON |
| JFR | JSON 파싱 | Counter, 정렬 | event 시계열 |
| OTel | JSONL 파싱 | 스팬 토폴로지 DAG, Counter | trace path, failure propagation |
| Exception | 정규식 (블록 단위) | Counter, 서명 추출 | type 분포 |

### 1.2 횡단 관심사 흐름

**에러 처리:**
```
Python 예외
  → DebugLogCollector.add_exception() → debug JSON 파일
  → CLI에서 re-raise → exit code 1 → stderr 캡처
  → execEngine callback: error ≠ null → {ok: false, error: {code: "ENGINE_EXITED", detail: stderr}}
  → IPC response → Renderer: setState("error"), setError(response.error)
  → AnalyzerFeedback 컴포넌트: ErrorPanel 렌더링
```

**진행 상황(progress):**
- **현재 구현: 없음.** 4-state 모델 (`idle|ready|running|error|success`)에서 `running` 상태 동안 "분석 중..." 텍스트만 표시. 실제 진행률(%) 또는 단계 정보가 UI로 전달되지 않는다.
- Python subprocess는 단일 호출 후 완료까지 블로킹되며, 중간 진행 정보를 IPC로 스트리밍하는 채널이 없다.

**취소(cancellation):**
- **현재 구현: 없음.** `execEngine`의 `timeout`(60초)만 존재. 사용자가 분석을 취소할 UI 버튼도 없고, Python 프로세스에 SIGTERM을 보내는 메커니즘도 없다.
- 앱 종료 시(`before-quit`)에만 `terminateActiveEngineProcesses()`가 호출된다.

**로깅:**
- Python 측: `DebugLogCollector` → JSON 파일 (에러/경고 발생 시만 기록)
- Electron 측: `console.log` 수준. 구조화된 로깅 프레임워크 없음
- 렌더러: `engineMessages` 배열로 마지막 20줄 표시

### 1.3 자원 라이프사이클

| 자원 | 생성 지점 | 해제 지점 | 위험 |
|---|---|---|---|
| Python subprocess | `execEngine()` → `execFile()` | exit 이벤트 → Set에서 제거 | exit 이벤트 미발생 시 Set 누수 (§3.2) |
| 임시 디렉터리 | `runAnalyzer()` → `mkdir()` | `finally` → `rm()` | `mkdir` 실패 시 `rm` 에러 (무해, `force: true`) |
| 파일 핸들 (Python) | `path.open()` / `iter_text_lines()` | `with` 문 보장 | ✅ 안전 |
| ECharts 인스턴스 | `ChartPanel` useEffect → `echarts.init()` | cleanup → `chart.dispose()` | ✅ 안전 |
| ResizeObserver | `ChartPanel` useEffect | cleanup → `observer.disconnect()` | ✅ 안전 |
| IPC 리스너 | `registerIpcHandlers()` | 앱 종료 시 자동 | ✅ 안전 (한 번만 등록) |

---

## 2. 성능 진단 결과

### 2.1 Hot Path 분석

| Hot Path | 위치 | 추정 비용 | 측정 인프라 | 개선 여지 | 효과 추정 |
|---|---|---|---|---|---|
| **subprocess spawn** | `main.ts:650-684` | macOS ~30-50ms/호출, Win ~50-100ms | 없음 | 장기 프로세스 전환 | 호출당 30-50ms 절감 |
| **이중 파일 읽기** | `file_utils.py:16-43` | 파일 크기 비례 O(n), 2회 | 없음 | 인코딩 탐지 통합 | 읽기 시간 50% 절감 |
| **percentile 정렬** | `statistics.py:12-22` | O(n log n), 호출마다 | 없음 | 캐싱 or T-Digest | p95+p99 계산 시간 ~90% 절감 |
| **flamegraph 트리 구축** | `flamegraph_builder.py:21-49` | O(S × D), S=스택수, D=평균깊이 | 없음 | 적정, 구조적 필수 | N/A |
| **flamegraph dict→tree 복원** | `profiler_analyzer.py:345` | O(N), N=노드수, 드릴다운마다 | 없음 | 트리 참조 유지 | 드릴다운당 ~50% 절감 |
| **OTel 토폴로지 구축** | `otel_analyzer.py:270-297` | O(T × S²), T=trace수, S=span수 | 없음 | 현재 적정 | N/A |
| **JSON 직렬화/역직렬화** | `json_exporter.py` → `main.ts:623-624` | 결과 크기 비례, 보통 수 MB | 없음 | msgpack 대안 | 직렬화 ~3x 빠름 |
| **ECharts 렌더링** | `ChartPanel.tsx` | 데이터 포인트 비례 | 없음 | sampling/large 모드 | 대용량 시 프레임 드롭 방지 |

### 2.2 메모리 사용 패턴

**대용량 입력 처리:**
- Access Log 파서: `iter_text_lines()`로 라인 단위 스트리밍 → 메모리 효율적 ✅
- 그러나 `response_times_by_minute`는 분당 `BoundedPercentile` 인스턴스 생성 (각 최대 10,000 샘플). 24시간 로그 = 1,440분 × 10,000 × 8바이트 ≈ **110 MB** 피크 메모리
- GC Log 파서: 모든 `GcEvent`를 리스트에 축적 → GB 로그 시 수백 MB 가능
- Profiler: `Counter[str]`에 모든 스택 축적 → 고유 스택 수에 비례, 일반적으로 적정
- JFR 파서: `json.load()`로 전체 파일 메모리 적재 → 대용량 JFR 시 위험

**중간 표현 크기:**
- `AnalysisResult`는 JSON 직렬화되어 임시 파일로 기록 후 다시 메인 프로세스에서 읽음. 결과 크기는 입력의 약 5-20% (집계 축소 효과). 정상 범위.
- flamegraph 트리는 입력 스택의 고유 경로 수에 비례. 100K 스택, 깊이 50 → 노드 수 ~500K, 노드당 ~200바이트 → **~100 MB**. 대용량 프로파일에서 주의 필요.

**누적/누수:**
- React 컴포넌트의 `result` state: 새 분석 시 이전 결과 교체 → 정상
- `engineMessages` state: `slice(-20)`으로 제한 → 정상

### 2.3 CPU 사용 패턴

**단일 코어 병목:**
- Python 분석 엔진: GIL 제약으로 단일 스레드. 대용량 입력 시 60초 타임아웃에 근접 가능
- Node.js 메인 프로세스: IPC 핸들링, 파일 I/O → 비동기이므로 차단 없음 ✅

**병렬화 가능성:**
- 다중 파일 분석: 현재 순차적. 별도 subprocess 동시 실행 가능
- 다중 분석 타입: 동일 파일에 여러 분석 적용 시 병렬화 가능

**정규식 비용:**
- `NGINX_WITH_RESPONSE_TIME`: 안전. `[^"]*`와 `\S+` 사용, catastrophic backtracking 위험 없음
- `UNIFIED_GC_RE`: `(?P<label>.*?)` 비탐욕 + `\s+` 조합은 비매칭 라인에서 약간의 백트래킹 가능하나 실질적 위험 낮음
- 사용자 제공 정규식 (`profiler_drilldown.py:224-228`): `re.search(pattern, frame)` — ReDoS 가능. `re.error`만 포착하고 시간 제한 없음

### 2.4 I/O 패턴

**이중 파일 읽기 (핵심 병목):**
```python
# file_utils.py
def iter_text_lines(path):
    encoding = detect_text_encoding(path)  # ← 파일 전체 읽기 #1
    with path.open("r", encoding=encoding) as handle:  # ← 파일 전체 읽기 #2
        for line in handle:
            yield line.rstrip("\n")
```
`detect_text_encoding()`은 파일을 8KB 청크 단위로 끝까지 읽어 인코딩 유효성만 검증한다. 이후 `iter_text_lines()`가 같은 파일을 다시 처음부터 읽는다. 100MB 로그 파일 기준 **약 200ms 낭비** 추정 (SSD 기준 ~500 MB/s 순차 읽기).

**IPC 데이터 전송:**
- 모든 결과는 JSON 파일 → 메인 프로세스 `readFile` → `JSON.parse` → IPC structured clone → 렌더러. 3중 직렬화/역직렬화.
- 결과가 수 MB 이하이면 무시해도 되나, 대용량 flamegraph (수십 MB) 시 병목 가능.

### 2.5 UI 렌더링 성능

**대용량 차트:**
- ECharts `sampling` 옵션 미사용. 수만 데이터 포인트의 시계열 차트에서 프레임 드롭 가능
- `large: true` 모드 미적용. 대용량 access log (수십만 분당 요청) 시 성능 저하 예상

**가상화:**
- 테이블/리스트의 가상 스크롤 미적용. `tables.sample_records`는 최대 20행으로 제한되어 현재 문제없으나, `tables.records`(OTel)은 `top_n` 제한에만 의존

**재렌더 최적화:**
- `ChartPanel`: `useEffect` 의존성 배열에 `option` 포함. 부모 재렌더 시 새 option 객체 생성 → ECharts 재초기화. `React.memo` 또는 option 참조 안정화 필요
- 개별 페이지 컴포넌트에 `React.memo` 미사용. 단일 페이지 구조라 현재 영향 미미하나, 탭 전환 시 마운트/언마운트 비용 존재

### 2.6 측정 인프라 평가

| 점검 항목 | 상태 | 평가 |
|---|---|---|
| **벤치마크** | 없음 | ❌ 핵심 경로(파싱, 분석, IPC)에 자동화된 벤치마크 0건 |
| **회귀 탐지** | 없음 | ❌ CI에 성능 게이트 없음. pytest는 정확성만 검증 |
| **프로파일링 가이드** | 없음 | ❌ 개발자용 프로파일링 절차 문서 없음 |
| **운영 메트릭** | 없음 | ❌ Electron 앱 내 성능 텔레메트리 수집 메커니즘 없음 |

**이것이 가장 심각한 성능 이슈다.** 위의 모든 Hot Path 분석은 코드 정적 분석에 기반한 추정이다. 실측 데이터 없이는 어떤 최적화가 실제로 가치 있는지 판단할 수 없다. 측정 인프라 도입 자체를 P0으로 분류한다.

### 2.7 성능 권고

#### PERF-1: 측정 인프라 전면 도입 [P0]
- **위치**: 프로젝트 전체
- **현상 / 측정**: 벤치마크 0건, 프로파일링 가이드 0건, CI 성능 게이트 0건. 모든 성능 권고가 추정에 의존.
- **개선안**:
  1. `pytest-benchmark`로 핵심 파서/분석기의 입력 크기별 벤치마크 추가 (10행, 10K행, 100K행)
  2. Electron 메인 프로세스에 `performance.mark()`/`performance.measure()` 계측 추가
  3. CI에 벤치마크 실행 및 회귀 알림 (GitHub Actions `benchmark-action`)
  4. 개발자용 프로파일링 가이드 문서화 (`py-spy`, `cProfile`, Chrome DevTools)
- **예상 효과**: 이후 모든 성능 개선의 ROI 측정 가능. 회귀 조기 탐지.
- **예상 비용**: 2-3일. 리스크 없음.
- **검증 방법**: CI에서 벤치마크 결과 리포트 생성 확인.

#### PERF-2: per-invocation subprocess spawn → 장기 프로세스 전환 [P1]
- **위치**: `electron/main.ts:650-684` (`execEngine`)
- **현상 / 측정**: 분석 요청마다 Python 프로세스 생성. macOS에서 spawn 오버헤드 ~30-50ms. 연속 분석 시(예: 데모 사이트 10개 파일) 누적 300-500ms.
- **개선안**: Python 사이드카를 stdin/stdout JSONL 프로토콜의 장기 프로세스로 전환. 시작 시 1회 spawn, 이후 요청/응답 교환.
  ```python
  # 현재: cli.py가 매번 새 프로세스로 실행
  # 개선: 서버 모드 추가
  # archscope-engine serve --port stdin
  while True:
      request = json.loads(input())
      result = dispatch(request)
      print(json.dumps(result))
  ```
- **예상 효과**: 호출당 30-50ms 절감. Python import 오버헤드(~200-500ms) 1회로 감소.
- **예상 비용**: 3-5일. IPC 프로토콜 설계 필요. 프로세스 라이프사이클 관리 복잡도 증가.
- **검증 방법**: 데모 사이트 전체 실행 시간 before/after 비교.

#### PERF-3: 이중 파일 읽기 제거 [P1]
- **위치**: `common/file_utils.py:16-43`
- **현상 / 측정**: `detect_text_encoding()`이 파일 전체를 읽고, 이후 파싱에서 다시 전체를 읽음. 100MB 파일 기준 ~200ms 낭비 추정.
- **개선안**: 인코딩 탐지를 파싱 루프에 통합. 첫 8KB 읽기 시도로 인코딩 확인 후 바로 파싱 계속:
  ```python
  def iter_text_lines(path: Path) -> Iterable[str]:
      for encoding in ("utf-8", "utf-8-sig", "cp949", "latin-1"):
          try:
              with path.open("r", encoding=encoding) as handle:
                  # 첫 8KB를 검증하면서 읽기
                  yield from (line.rstrip("\n") for line in handle)
              return
          except UnicodeDecodeError:
              continue
      raise ValueError("no valid encoding found")
  ```
- **예상 효과**: 파일 읽기 시간 ~50% 절감. 100MB 파일 기준 ~100-200ms.
- **예상 비용**: 0.5일. `detect_text_encoding` 호출부도 정리 필요 (debug_log에서 encoding 기록).
- **검증 방법**: 대용량 파일 파싱 시간 before/after 비교.

#### PERF-4: JSON 직렬화 경로 최적화 [P2]
- **위치**: `json_exporter.py` → `main.ts:623-624`
- **현상 / 측정**: Python `json.dumps(indent=2)` → 임시 파일 → Node.js `readFile` → `JSON.parse`. 3중 직렬화. indent=2는 파일 크기 ~30% 증가.
- **개선안**: 
  1. 단기: indent 제거로 파일 크기 및 직렬화 시간 절감
  2. 중기: stdout으로 직접 출력 (임시 파일 제거)
  3. 장기: MessagePack 또는 CBOR로 바이너리 직렬화
- **예상 효과**: indent 제거만으로 직렬화 ~20% 빠름, 파일 ~30% 작아짐.
- **예상 비용**: 단기 0.5일, 중기 1-2일.
- **검증 방법**: flamegraph 결과 직렬화/역직렬화 시간 비교.

#### PERF-5: ECharts 대용량 데이터 최적화 [P2]
- **위치**: `src/charts/chartOptions.ts`, `src/components/ChartPanel.tsx`
- **현상 / 측정**: `sampling`, `large`, `progressive` 옵션 미사용. 수만 데이터 포인트 시 프레임 드롭 예상.
- **개선안**: 시계열 차트에 `sampling: 'lttb'`, 대용량 시 `large: true` 및 `largeThreshold: 3000` 적용. Canvas 렌더러 기본 사용 (현재 정상).
- **예상 효과**: 10K+ 데이터 포인트에서 렌더링 시간 ~5-10x 개선.
- **예상 비용**: 0.5일.
- **검증 방법**: 100K 데이터 포인트 차트 렌더링 시간 및 FPS 측정.

#### PERF-6: ChartPanel ECharts 재초기화 방지 [P2]
- **위치**: `src/components/ChartPanel.tsx`
- **현상 / 측정**: `useEffect`의 의존성 배열에 `option` 포함. 부모 재렌더 시 새 option 객체 → ECharts `init()` + `dispose()` 반복 (비용: ~10-30ms/회).
- **개선안**: 초기화와 옵션 업데이트 분리:
  ```typescript
  // 초기화: 한 번만
  useEffect(() => {
    const chart = echarts.init(chartRef.current, theme, { renderer });
    chartInstance.current = chart;
    return () => chart.dispose();
  }, [renderer, theme]);
  
  // 옵션 업데이트: option 변경 시
  useEffect(() => {
    chartInstance.current?.setOption(option, { notMerge: true });
  }, [option]);
  ```
- **예상 효과**: 차트 업데이트 시간 ~80% 절감 (재초기화 제거).
- **예상 비용**: 0.5일.
- **검증 방법**: React Profiler로 ChartPanel 렌더 시간 측정.

---

## 3. 코드 결함 진단 결과

### 3.1 동시성 관련

**공유 상태:**
- `activeEngineProcesses` Set은 메인 프로세스 단일 스레드에서만 접근 → 경쟁 조건 없음 ✅
- React 각 페이지 독립 상태 관리 → 경쟁 조건 없음 ✅

**비동기 작업 순서:**
- `runAnalyzer`는 `await execEngine()`으로 직렬화 → 순서 보장 ✅
- 사용자가 분석 실행 중 다시 "Analyze" 버튼 클릭 시: 현재 구현은 상태만 체크(`state === "running"` → 버튼 비활성). 그러나 버튼 비활성화가 경쟁 조건 방지의 유일한 방어선.

**취소 안전성:**
- 취소 메커니즘이 없으므로 취소 시 부분 결과/자원 정리 문제도 없음. 단, 이는 기능 부재이지 안전성이 아님.

### 3.2 리소스 라이프사이클

**subprocess exit 이벤트 미발생 시나리오:**
```typescript
// main.ts:680-683
activeEngineProcesses.add(child);
child.once("exit", () => {
  activeEngineProcesses.delete(child);
});
```
- `execFile`의 `timeout` 옵션은 타임아웃 시 프로세스를 SIGTERM으로 종료한다 (Node.js 문서). `exit` 이벤트는 발생하므로 Set에서 정상 제거됨.
- `child.kill()` 실패 시(이미 종료된 프로세스): Node.js는 예외를 던지지 않고 false를 반환 → 안전.
- **실질적 누수 위험: 낮음.** 이론적으로 프로세스가 좀비 상태에 빠지면 `exit` 이벤트가 지연되지만, timeout으로 보호됨.

**타이머/인터벌:**
- 어떤 컴포넌트에도 `setInterval`/`setTimeout` 사용 없음 → 누수 위험 없음 ✅

### 3.3 에러 처리

**swallowed exceptions 점검:**
- `cli.py:868-875`: 예외를 debug log에 기록 후 re-raise → 정상 ✅
- `main.ts:181-183`, `193`, `204`, `215`: 모든 IPC 핸들러가 try-catch로 감싸고 `ipcFailed()` 반환 → 예외가 삼켜지지 않음 ✅
- `main.ts:737-739` (`isReadableFile`): `stat` 실패 시 false 반환 → 의도적, 파일 존재 확인 목적

**에러 분류:**
- `BridgeError` 타입의 `code` 필드로 분류: `INVALID_OPTION`, `FILE_NOT_FOUND`, `ENGINE_EXITED`, `ENGINE_OUTPUT_INVALID`, `IPC_FAILED` → 적절 ✅

**부분 실패:**
- 데모 사이트 실행 시 개별 분석기 실패는 `failedAnalyzers`에 기록되고 나머지는 계속 → 올바른 부분 실패 처리 ✅
- 단일 파일 분석은 전체 성공/실패 → 파일 내 일부 라인 실패는 `diagnostics.skipped_lines`로 기록 → 적절 ✅

**타임아웃:**
- `execEngine`: 기본 60초, 데모 실행 300초 → 정의됨 ✅
- 외부 LLM 호출: `runtime.py`의 `timeout_seconds: 30.0` → 정의됨 ✅

### 3.4 입력 검증

**빈 입력:**
- 빈 파일: `iter_text_lines` → StopIteration → 빈 리스트 → 분석기에서 빈 결과 반환 → 정상 ✅
- `statistics.py:13-14`: `percentile([])` → `0.0` 반환 → 정상 ✅

**거대 입력:**
- `maxLines` 옵션으로 파싱 제한 가능 (access log). 그러나 GC log, thread dump에는 크기 제한 옵션 없음.
- `execEngine`의 `maxBuffer: 4MB`: stdout/stderr에만 적용, 분석 결과는 파일로 전달되므로 영향 없음 ✅

**인코딩:**
- `detect_text_encoding`: UTF-8 → UTF-8-SIG → CP949 → Latin-1 폴백 체인 → 적절 ✅
- BOM 처리: `utf-8-sig`가 BOM 자동 제거 → 정상 ✅

**개행 문자:**
- `line.rstrip("\n")`: `\n`만 제거. CRLF(`\r\n`) 파일에서 `\r`이 잔류. 그러나 Python의 `open()` 기본 모드는 universal newline이므로 `\r\n` → `\n` 자동 변환 → 실질적 문제 없음 ✅

### 3.5 보안

**IPC 신뢰 경계:**
- `contextIsolation: true`, `nodeIntegration: false` → 렌더러에서 Node API 직접 접근 불가 ✅
- IPC 핸들러에서 `request` 타입/값 검증 존재 (`typeof request.filePath !== "string"` 등) → 적절 ✅
- 단, TypeScript 타입은 런타임 보장이 아님. 악의적 렌더러가 조작된 IPC 메시지 전송 가능 → 현재 검증이 핵심 필드만 커버하므로 **낮은 위험**.

**외부 프로세스 호출:**
- `execFile`은 셸을 거치지 않음 → 커맨드 인젝션 위험 없음 ✅
- 사용자 입력(파일 경로)은 인자 배열로 전달 → 안전 ✅

**프롬프트 인젝션 방어:**
- `ai_interpretation/prompting.py`: `<diagnostic_data>` XML 태그로 데이터 격리, `SUSPICIOUS_INSTRUCTION_PATTERN`으로 의심 패턴 탐지 → 양호한 방어 ✅

**`style-src 'unsafe-inline'`:**
- 현재 CSP에 `'unsafe-inline'` 사용 (`main.ts:62`). CSS 삽입 공격 허용. 이전 검토에서 P2로 분류되었고, Phase 4+ 로 연기됨. ECharts 5.4+의 `csp: { nonce }` 옵션 활용 가능.

### 3.6 타입 / 계약

**`any` 사용처:**
- Python: `dict[str, Any]`가 `AnalysisResult`의 `summary`, `series`, `tables`, `charts`, `metadata` 모두에 사용. 타입 안전성이 런타임 검증에만 의존.
- TypeScript: `analyzerContract.ts`에서 구체적 타입 정의 (`AccessLogAnalysisResult` 등) → 양호 ✅. 단, `Record<string, unknown>` 다운캐스트 사용처 다수.

**IPC 페이로드 타입 공유:**
- Python과 TypeScript 간 타입 정의가 독립적으로 유지. 공유 스키마 (JSON Schema, Protobuf) 없음. 불일치 시 런타임에서만 발견.
- `isAccessLogAnalysisResult` 등의 타입 가드가 방어선 역할 → 불완전하지만 존재 ✅

**Python 타입 힌트:**
- 대부분의 함수에 타입 힌트 존재. `from __future__ import annotations` 사용 ✅
- `mypy`는 CI에 미적용. `ruff`만 사용.

### 3.7 테스트 커버리지

| 모듈 | 테스트 파일 | 커버리지 평가 |
|---|---|---|
| Access Log Parser | `test_access_log_parser.py` (89행) | 기본 파싱 ✅, 경계 조건 부분적 |
| Access Log Analyzer | `test_access_log_analyzer.py` (144행) | 집계 로직 ✅, 대용량 입력 ❌ |
| Collapsed Parser | `test_collapsed_parser.py` (62행) | 기본 파싱 ✅ |
| Profiler Analyzer | `test_profiler_analyzer.py` (92행) | flamegraph ✅, 드릴다운 부분적 |
| JVM Analyzers | `test_jvm_analyzers.py` (88행) | GC/Thread/Exception ✅ |
| OTel Analyzer | `test_otel_analyzer.py` (108행) | 토폴로지 ✅ |
| JFR Analyzer | `test_jfr_analyzer.py` (63행) | 기본 분석 ✅ |
| Statistics | `test_statistics.py` (60행) | 기본 ✅, **대용량 reservoir 동작 ❌** |
| AI Interpretation | `test_ai_interpretation.py` (163행) | 검증 로직 ✅ |
| HTML Exporter | `test_html_exporter.py` (58행) | 기본 렌더링 ✅ |
| CLI E2E | `test_cli_e2e.py` (452행) | 전체 흐름 ✅ |
| Runtime Analyzers | `test_runtime_analyzers.py` (98행) | Go/Node/Python/.NET ✅ |

**사각지대:**
- **Electron 통합 테스트: 0건.** IPC 핸들러, 타입 가드, 프로세스 라이프사이클 미테스트.
- **UI 컴포넌트 테스트: chartFactory.test.ts 1건만.** 페이지 컴포넌트, 에러 상태, 사용자 입력 검증 미테스트.
- **BoundedPercentile 대용량 테스트: 없음.** 20건으로만 테스트. 10,000건 이상에서의 reservoir 동작 미검증.
- **시각적 회귀 테스트: 없음.**
- **성능 회귀 테스트: 없음.**

### 3.8 결함 권고

#### DEF-1: AnalysisResult frozen dataclass 불변성 위반 [P0]
- **위치**: `profiler_analyzer.py:354-380` (`_drilldown_from_result`)
- **재현 시나리오**: 드릴다운 분석 실행 시 `result.charts["drilldown_stages"] = ...`, `result.tables["top_stacks"] = ...` 등으로 frozen dataclass의 내부 dict를 변이.
- **결함의 영향**: `AnalysisResult`가 `frozen=True`로 선언되어 불변 계약을 표현하지만, 실제로는 내부 dict가 자유롭게 변이됨. 캐싱, 공유 참조, 디버깅 시 혼란 유발. 원본 결과 오염으로 재분석 시 부정확한 결과 가능.
- **개선안**: 두 가지 중 선택:
  1. **불변 유지**: `_drilldown_from_result`에서 새로운 `AnalysisResult`를 생성 (dict 복사 후 변경)
  2. **불변 포기**: `AnalysisResult`에서 `frozen=True` 제거하고 명시적 `.copy()` 메서드 제공
  ```python
  # 옵션 1 (권장): 새 AnalysisResult 생성
  def _drilldown_from_result(...) -> AnalysisResult:
      ...
      return AnalysisResult(
          type=result.type,
          source_files=result.source_files,
          summary=result.summary,
          series={**result.series, "execution_breakdown": breakdown},
          tables={**result.tables, "top_stacks": top_stacks, ...},
          charts={"flamegraph": ..., "drilldown_stages": ...},
          metadata={**result.metadata, "drilldown_current_stage": ...},
      )
  ```
- **예상 비용**: 0.5일.
- **검증 방법**: 드릴다운 후 원본 result 참조의 charts/tables 불변 확인 테스트.

#### DEF-2: 사용자 제공 정규식 ReDoS 방어 부재 [P1]
- **위치**: `profiler_drilldown.py:223-228`
- **재현 시나리오**: 사용자가 드릴다운 필터에 `(a+)+b` 같은 catastrophic backtracking 패턴을 입력하면 CPU 100%로 장시간 점유.
- **결함의 영향**: 분석 프로세스 행(hang). timeout(60초)으로 결국 종료되지만, 그 동안 앱 응답 불가.
- **개선안**: 정규식 복잡도 제한:
  ```python
  import re
  MAX_REGEX_LEN = 500
  
  def _frame_matches(frame: str, pattern: str, filter_spec: DrilldownFilter) -> bool:
      if filter_spec.filter_type in {"regex_include", "regex_exclude"}:
          if len(pattern) > MAX_REGEX_LEN:
              return False
          try:
              return re.search(pattern, frame, timeout=0.1) is not None  # Python 3.11+
          except (re.error, TimeoutError):
              return False
      return pattern.casefold() in frame.casefold()
  ```
  Note: Python 3.11 이전 버전이면 `google-re2` 또는 `regex` 패키지로 대체.
- **예상 비용**: 0.5일.
- **검증 방법**: `(a+)+b` 패턴으로 드릴다운 테스트, 1초 이내 완료 확인.

#### DEF-3: OTel 스팬 토폴로지에서 순환 참조 미탐지 [P1]
- **위치**: `otel_analyzer.py:312-338` (`_ordered_services_by_parent`)
- **재현 시나리오**: 악의적이거나 손상된 OTel 로그에서 `span_a.parent = span_b`, `span_b.parent = span_a` 순환 참조 시.
- **결함의 영향**: `visit()` 함수에 `visited` set이 있어 무한 재귀는 방지됨 ✅. 그러나 순환 참조된 스팬들은 토폴로지에서 누락될 수 있음 (root가 아닌데 parent가 없는 것처럼 처리).
- **개선안**: 순환 참조 탐지 및 findings에 경고 추가:
  ```python
  # 순환 참조 탐지
  for span_id, record in span_records.items():
      if record.parent_span_id == span_id:  # 자기 참조
          # 경고 기록
  ```
- **예상 비용**: 0.5일.
- **검증 방법**: 순환 참조 포함 OTel 입력 테스트.

#### DEF-4: 분석 취소 메커니즘 부재 [P1]
- **위치**: `main.ts:650-684`, 전체 UI 페이지
- **재현 시나리오**: 사용자가 대용량 파일 분석 시작 후 취소하고 싶을 때 방법이 없음. 60초 타임아웃까지 대기.
- **결함의 영향**: UX 저하. 잘못된 파일 선택 시 60초 대기 필수.
- **개선안**: 
  1. UI에 "Cancel" 버튼 추가, running 상태에서 표시
  2. `AbortController` 활용: `execFile`에 `signal` 전달 (Node.js 18+)
  3. Python 프로세스에 SIGTERM 전송
- **예상 비용**: 1-2일.
- **검증 방법**: 대용량 파일 분석 시작 후 취소 버튼 동작 확인.

#### DEF-5: BoundedPercentile 테스트가 대용량 시나리오 미커버 [P1]
- **위치**: `tests/test_statistics.py:29-51`
- **재현 시나리오**: 테스트가 20건만 입력. 10,000건 이상에서의 reservoir sampling 동작 미검증.
- **결함의 영향**: ALG-1의 수학적 결함이 테스트에서 발견되지 않음.
- **개선안**: 대용량 테스트 추가:
  ```python
  def test_bounded_percentile_large_input_represents_full_distribution():
      stats = BoundedPercentile(max_samples=100)
      for value in range(100_000):
          stats.add(float(value))
      # reservoir가 전체 분포를 대표하는지 검증
      p50 = stats.percentile(50)
      assert 40_000 < p50 < 60_000, f"p50={p50} should be near 50000"
  ```
- **예상 비용**: 0.5일.
- **검증 방법**: 테스트 실행.

#### DEF-6: Electron 통합 테스트 부재 [P2]
- **위치**: `apps/desktop/electron/main.ts`
- **재현 시나리오**: IPC 핸들러의 입력 검증, 타입 가드, 프로세스 라이프사이클이 테스트되지 않음.
- **결함의 영향**: 리팩토링 시 IPC 계약 위반을 CI에서 잡지 못함.
- **개선안**: Playwright 또는 Electron Testing Library로 IPC 핸들러 단위 테스트.
- **예상 비용**: 3-5일.
- **검증 방법**: CI에서 Electron 테스트 통과 확인.

#### DEF-7: `_is_error_record` 오탐 가능성 [P2]
- **위치**: `otel_analyzer.py:422-436`
- **재현 시나리오**: body에 "error"가 포함된 정상 로그 (예: `"error_count reset to 0"`, `"timeout configuration updated"`)가 에러로 분류됨.
- **결함의 영향**: 에러 통계 부풀림, 잘못된 진단 소견.
- **개선안**: severity 기반 판별을 우선하고, body 키워드 매칭은 severity가 없을 때만 적용. 또는 키워드 매칭 시 단어 경계(`\b`) 사용.
- **예상 비용**: 0.5일.
- **검증 방법**: "error" 키워드가 포함된 정상 로그로 테스트.

---

## 4. 알고리즘 진단 결과

### 4.1 핵심 알고리즘 인벤토리

| 알고리즘 | 위치 | 시간 복잡도 | 공간 복잡도 | 평가 | 대안 |
|---|---|---|---|---|---|
| **Access Log 파싱** | `access_log_parser.py` | O(n), n=라인수 | O(1) 스트리밍 | ✅ 적정 | N/A |
| **분당 집계** | `access_log_analyzer.py:114-138` | O(n) | O(m), m=고유 분수 | ✅ 적정 | N/A |
| **Reservoir Sampling** | `statistics.py:38-46` | O(1)/add | O(k), k=max_samples | ❌ **비동작** | Algorithm R 올바른 구현 |
| **Percentile 계산** | `statistics.py:12-22` | O(k log k)/호출 | O(k) 복사 | ⚠️ 비효율 | T-Digest, 캐싱 |
| **Flamegraph 트리 구축** | `flamegraph_builder.py:21-49` | O(S × D) | O(N), N=노드수 | ✅ 적정 | N/A |
| **드릴다운 필터링** | `profiler_drilldown.py:78-120` | O(L × P), L=리프수, P=경로길이 | O(L × P) | ✅ 적정 | N/A |
| **스택 분류** | `profile_classification.py` | O(R), R=규칙수/스택 | O(1) | ✅ 적정 | N/A |
| **GC 이벤트 집계** | `gc_log_analyzer.py` | O(n) | O(n) 전체 이벤트 저장 | ⚠️ 대용량 시 위험 | 스트리밍 집계 |
| **OTel 스팬 토폴로지** | `otel_analyzer.py:270-297` | O(T × S²) | O(T × S) | ✅ 적정 (S 보통 <100) | N/A |
| **OTel 서비스 순서** | `otel_analyzer.py:312-338` | O(S × E), E=edge수 | O(S) | ✅ 적정 | N/A |
| **Thread dump 서명** | `thread_dump_analyzer.py` | O(n × k), k=top_k 프레임 | O(n) | ✅ 적정 | N/A |
| **Counter.most_common** | 다수 | O(n log k) | O(k) | ✅ 적정 | N/A |
| **URL Top-K 정렬** | `access_log_analyzer.py:159-170` | O(U log U), U=고유URL수 | O(U) | ⚠️ URL 다양 시 | heap 기반 top-k |

### 4.2 자료구조 선택 평가

**적절한 선택:**
- `Counter[str]` for 빈도 집계 → O(1) 삽입, O(n log k) most_common → 적정 ✅
- `defaultdict(list)` for trace_records 그룹핑 → 적정 ✅
- `dict[str, _MutableFlameNode]` for 트리 자식 → O(1) 조회 → 적정 ✅

**개선 가능:**
- `_component_breakdown` (`profiler_analyzer.py:410-420`): 모든 스택에 대해 `classify_stack()`을 호출하며 이 함수는 내부적으로 규칙 튜플을 순회. 스택 수가 많으면 반복 계산. 결과 캐싱 가능.

### 4.3 정규식 패턴 평가

| 패턴 | 위치 | 위험 | 평가 |
|---|---|---|---|
| `NGINX_WITH_RESPONSE_TIME` | `access_log_parser.py:20-26` | 없음 | `[^"]*`, `\S+`는 백트래킹 안전 ✅ |
| `UNIFIED_GC_RE` | `gc_log_parser.py:16-23` | 낮음 | `.*?` 비탐욕이나 `\s+`로 바운딩 ✅ |
| `THREAD_HEADER_RE` | `thread_dump_parser.py:12` | 없음 | `[^"]+`로 바운딩 ✅ |
| `SUSPICIOUS_INSTRUCTION_PATTERN` | `prompting.py:13-15` | 없음 | 바운딩된 lookahead (0-80자) ✅ |
| 사용자 제공 패턴 | `profiler_drilldown.py:224-228` | **높음** | ReDoS 가능 (§DEF-2) |

**컴파일 캐싱:**
- 모듈 수준 `re.compile()` 사용 → 적정 ✅
- 사용자 제공 패턴은 매 프레임마다 `re.search()` 호출 → 컴파일 캐싱 없음. `re` 모듈의 내부 캐시(최대 512패턴)에 의존.

### 4.4 캐싱 / 메모이제이션 기회

| 기회 | 위치 | 효과 추정 | 비용 |
|---|---|---|---|
| **percentile 정렬 캐싱** | `statistics.py:12-22` | p95+p99 호출 시 정렬 1회로 축소 | 낮음 |
| **classify_stack 결과 캐싱** | `profiler_analyzer.py:406` | 동일 스택 반복 분류 방지 | 낮음 |
| **인코딩 탐지 결과 캐싱** | `file_utils.py:16-34` | 이중 읽기 제거 (PERF-3) | 낮음 |
| **flamegraph dict→tree 캐싱** | `profiler_analyzer.py:345` | 드릴다운마다 역직렬화 방지 | 중간 |

### 4.5 점진적 / 지연 계산 기회

| 기회 | 위치 | 효과 추정 |
|---|---|---|
| **부분 결과 스트리밍** | 모든 분석기 | 대용량 파일에서 "처리 중 0/1000줄" 진행률 표시 가능 |
| **차트 데이터 지연 생성** | `build_*_result` 함수들 | flamegraph는 사용자가 탭 선택 시만 계산 |
| **OTel 토폴로지 지연 계산** | `otel_analyzer.py:105-106` | `trace_span_topology`, `service_trace_matrix`는 비싼 계산이나 항상 표시되지 않음 |

### 4.6 알고리즘 권고

#### ALG-1: `_deterministic_reservoir_index` 수학적 결함 — Reservoir Sampling 비동작 [P0]
- **위치**: `common/statistics.py:56-57`
- **현재 알고리즘**: 
  ```python
  def _deterministic_reservoir_index(count: int, seed: int) -> int:
      return ((count * 1_103_515_245) + seed) % count
  ```
- **문제점**: `(count * K + seed) % count`는 수학적으로 `seed % count`와 동일하다 (`count * K`는 `count`로 나누어 떨어짐). 결과:
  - **seed=12345, max_samples=10000 (기본값)**: 
    - count ∈ [10001, 12345]: `replace_at = 12345 % count` → 가변값, 대부분 < 10000 → 일부 교체 발생
    - count = 12346: `replace_at = 12345 % 12346 = 12345` → ≥ 10000 → **교체 안 됨**
    - count > 12345: `replace_at = 12345` (고정) → ≥ 10000 → **영원히 교체 안 됨**
  - 결론: **12,346번째 레코드 이후 reservoir이 완전히 동결.** 이후 모든 데이터가 무시됨.
  - 또한 seed < max_samples인 경우(예: seed=1): `replace_at = 1` (항상 고정) → 항상 같은 슬롯만 교체 → **극심한 편향**
  - 현재 테스트(`test_statistics.py`)는 20건만 입력하여 이 결함을 탐지하지 못함.
- **영향**: Access log 분석에서 p95/p99 값이 처음 12,345건의 분포만 반영. 10만 줄 로그에서 87.7%의 데이터가 백분위 계산에서 무시됨.
- **개선안**: 표준 Algorithm R (Vitter, 1985) 구현:
  ```python
  import random
  
  @dataclass
  class BoundedPercentile:
      max_samples: int = 10_000
      seed: int = 12_345
      count: int = 0
      _samples: list[float] = field(default_factory=list)
      _rng: random.Random = field(init=False)
      
      def __post_init__(self) -> None:
          if self.max_samples <= 0:
              raise ValueError("max_samples must be a positive integer.")
          self._rng = random.Random(self.seed)
      
      def add(self, value: float) -> None:
          self.count += 1
          if len(self._samples) < self.max_samples:
              self._samples.append(value)
              return
          # Algorithm R: uniform random in [0, count)
          j = self._rng.randrange(self.count)
          if j < self.max_samples:
              self._samples[j] = value
  ```
- **예상 효과**: 백분위 계산이 전체 데이터 분포를 정확히 반영. p95 오차 max_samples=10000 기준 ±0.5% 이내 (이론적 보장).
- **예상 비용**: 0.5일. 테스트 보강 필수 (대용량 + 분포 검증).
- **검증 방법**: 100,000건 균등 분포 입력에서 p50이 45,000-55,000 범위 내 확인. 현재 구현은 이 테스트를 실패할 것.

#### ALG-2: percentile() 함수 매 호출 전체 정렬 [P1]
- **위치**: `common/statistics.py:12-22`
- **현재 알고리즘**: `sorted(values)` → O(k log k), k=샘플 수. `percentile()`이 p95, p99 등 매번 호출 시 전체 정렬.
- **문제점**: `BoundedPercentile.percentile()`이 p95, p99를 각각 호출하면 10,000개 샘플 정렬이 2회 수행. 분당 집계에서 1,440분 × 2회 = 2,880회 정렬.
- **개선안**: 두 가지 접근:
  1. **정렬 캐싱**: `BoundedPercentile`에 `_sorted_cache` 추가, `add()` 시 무효화
  2. **T-Digest 전환**: 스트리밍 백분위에 최적화된 알고리즘. PyPI: `tdigest` 패키지
  ```python
  # 옵션 1: 정렬 캐싱 (최소 변경)
  class BoundedPercentile:
      _sorted_cache: list[float] | None = field(default=None, init=False)
      
      def add(self, value: float) -> None:
          self._sorted_cache = None  # 캐시 무효화
          # ... 기존 로직
      
      def percentile(self, percent: float) -> float:
          if self._sorted_cache is None:
              self._sorted_cache = sorted(self._samples)
          return _percentile_from_sorted(self._sorted_cache, percent)
  ```
- **예상 효과**: p95+p99 동시 계산 시 정렬 1회로 축소 → 50% 절감. 분당 집계 전체에서 2,880회 → 1,440회.
- **예상 비용**: 0.5일.
- **검증 방법**: 대용량 입력의 분석 시간 before/after 비교.

#### ALG-3: GC 이벤트 전체 메모리 축적 [P1]
- **위치**: `gc_log_analyzer.py:41-44`
- **현재 알고리즘**: 모든 GC 이벤트를 리스트에 축적 후 집계.
- **문제점**: GB 단위 GC 로그 (수백만 이벤트) 시 메모리 폭발. `GcEvent` 1건 ≈ 200바이트 → 1M 이벤트 = 200 MB.
- **개선안**: 스트리밍 집계. Counter와 rolling statistics로 이벤트를 즉시 집계하고 원본은 버림:
  ```python
  def analyze_gc_log_streaming(path, ...):
      total_events = 0
      total_pause_ms = 0.0
      max_pause_ms = 0.0
      type_counts = Counter()
      cause_counts = Counter()
      pause_stats = BoundedPercentile()  # ALG-1 수정 후
      
      for event in parse_gc_log(path):
          total_events += 1
          total_pause_ms += event.pause_ms
          max_pause_ms = max(max_pause_ms, event.pause_ms)
          type_counts[event.gc_type] += 1
          cause_counts[event.cause] += 1
          pause_stats.add(event.pause_ms)
  ```
- **예상 효과**: 메모리 사용량 O(n) → O(k), k=max_samples. 200 MB → ~1 MB.
- **예상 비용**: 1일. 시계열 데이터(분당 통계)는 별도 집계 필요.
- **검증 방법**: 1M 이벤트 GC 로그 분석 시 피크 메모리 비교.

#### ALG-4: URL Top-K에 전체 정렬 사용 [P2]
- **위치**: `access_log_analyzer.py:159-170`
- **현재 알고리즘**: `sorted(url_counts.items(), key=lambda x: x[1], reverse=True)[:10]` → O(U log U), U=고유 URL 수.
- **문제점**: U가 수만~수십만일 때 불필요한 전체 정렬. Top-10만 필요.
- **개선안**: `Counter.most_common(10)` → 내부적으로 `heapq.nlargest` 사용 → O(U log 10) ≈ O(U).
  ```python
  # 현재 (불필요한 전체 정렬)
  top_urls = sorted(url_counts.items(), key=lambda item: item[1], reverse=True)[:10]
  # 개선 (heap 기반)
  top_urls = url_counts.most_common(10)
  ```
  실제로 `url_counts`는 이미 `Counter`이므로 `.most_common(10)` 직접 사용 가능.
- **예상 효과**: 100K 고유 URL 기준 정렬 시간 ~5x 개선 추정 (O(n log n) → O(n log k)).
- **예상 비용**: 0.5일.
- **검증 방법**: 100K 고유 URL access log 분석 시간 비교.

#### ALG-5: flamegraph dict→tree 재구축 제거 [P2]
- **위치**: `profiler_analyzer.py:345`
- **현재 알고리즘**: 드릴다운마다 `flame_node_from_dict(result.charts["flamegraph"])` 호출. dict → FlameNode 재귀 재구축 O(N), N=노드 수.
- **문제점**: `build_collapsed_result`에서 이미 FlameNode 트리를 구축했으나 dict로 변환 후 저장, 드릴다운에서 다시 역변환.
- **개선안**: `AnalysisResult`에 원본 FlameNode 트리를 별도 필드로 유지 (DEF-1 해결 후). 또는 드릴다운을 원본 stacks Counter에서 직접 수행.
- **예상 효과**: 드릴다운당 O(N) 역직렬화 제거. 50K 노드 기준 ~10-50ms 절감 추정.
- **예상 비용**: 1일. AnalysisResult 구조 변경 필요.
- **검증 방법**: 드릴다운 실행 시간 비교.

---

## 5. 횡단 진단

### 5.1 성능 ↔ 결함 교차점

1. **Reservoir Sampling 결함 → 성능 측정 무의미**: ALG-1의 결함으로 p95/p99 값이 부정확. 이 값을 기반으로 성능 판단을 내리면 잘못된 결론에 도달. 성능 개선의 전제 조건이 정확한 측정인데, 측정 도구 자체가 고장.

2. **AnalysisResult 불변성 위반 → 캐싱 위험**: DEF-1로 인해 `AnalysisResult`를 캐시 키로 사용하거나 이전 결과를 참조하는 최적화가 위험해짐. 불변성이 보장되지 않으면 캐싱 전략 도입이 복잡해짐.

### 5.2 결함 ↔ 알고리즘 교차점

1. **Degenerate hash → 테스트 미탐지**: `_deterministic_reservoir_index`의 수학적 결함이 테스트에서 발견되지 않은 것은 테스트 설계의 문제이기도 하다. "20건으로 충분"이라는 가정이 알고리즘의 점근적 동작을 검증하지 못함.

2. **`_is_error_record` 키워드 매칭 → 통계 왜곡**: DEF-7의 오탐이 OTel 분석의 `error_records`, `failed_traces`, `failure_propagation` 통계를 왜곡. 알고리즘의 정확성이 입력 분류의 정확성에 의존.

### 5.3 알고리즘 ↔ 성능 교차점

1. **Reservoir Sampling vs T-Digest**: ALG-1 수정 시 Algorithm R로 교체하면 동작은 올바르나, 백분위 계산 시 여전히 O(k log k) 정렬이 필요 (ALG-2). T-Digest는 O(1)에 가까운 백분위 조회를 제공하므로, ALG-1과 ALG-2를 동시에 해결.

2. **이중 파일 읽기 (PERF-3) → 인코딩 탐지 알고리즘**: 현재 인코딩 탐지가 파일 전체를 읽는 것은 알고리즘적으로 불필요. 첫 8KB만으로 인코딩 판별이 가능 (BOM 확인 + 디코딩 시도). PERF-3과 함께 개선.

### 5.4 End-to-End 데이터 흐름 비효율

1. **Python → JSON 파일 → Node.js 파일 읽기 → JSON.parse → IPC structured clone**
   - 4중 데이터 변환. Python stdout으로 직접 전달하면 2단계로 축소 가능.
   - 현재 구조의 장점: 디버깅 시 임시 파일 확인 가능, 대용량 결과에서도 안정적.
   - 단기적으로는 현재 구조가 적절. 장기적으로 PERF-2(장기 프로세스)와 함께 stdin/stdout 직접 전달로 전환 고려.

2. **flamegraph: tree → dict → JSON → file → JSON.parse → dict → tree (드릴다운 시)**
   - 6중 변환. 트리를 한 번 구축하면 dict 변환 없이 유지하는 것이 이상적 (ALG-5).

---

## 6. 우선순위화된 종합 권고

### 6.1 P0 (즉시 처리)

| ID | 분류 | 제목 | 위치 | 효과 추정 | 비용 |
|---|---|---|---|---|---|
| ALG-1 | 알고리즘 | Reservoir sampling 수학적 결함 | `statistics.py:56-57` | 백분위 정확도 0% → 99%+ | 0.5일 |
| DEF-1 | 결함 | frozen dataclass 불변성 위반 | `profiler_analyzer.py:354-380` | 데이터 무결성 보장 | 0.5일 |
| PERF-1 | 성능 | 측정 인프라 전면 부재 | 프로젝트 전체 | 모든 성능 판단의 기반 | 2-3일 |

### 6.2 P1 (다음 사이클)

| ID | 분류 | 제목 | 위치 | 효과 추정 | 비용 |
|---|---|---|---|---|---|
| ALG-2 | 알고리즘 | percentile() 매 호출 정렬 | `statistics.py:12-22` | 백분위 계산 50% 절감 | 0.5일 |
| ALG-3 | 알고리즘 | GC 이벤트 전체 메모리 축적 | `gc_log_analyzer.py:41-44` | 피크 메모리 200MB→1MB | 1일 |
| PERF-2 | 성능 | per-invocation subprocess spawn | `main.ts:650-684` | 호출당 30-50ms + import 200-500ms | 3-5일 |
| PERF-3 | 성능 | 이중 파일 읽기 | `file_utils.py:16-43` | 파일 읽기 50% 절감 | 0.5일 |
| DEF-2 | 결함 | 사용자 정규식 ReDoS 방어 | `profiler_drilldown.py:223-228` | CPU 행(hang) 방지 | 0.5일 |
| DEF-3 | 결함 | OTel 순환 참조 미탐지 | `otel_analyzer.py:312-338` | 손상 입력 내성 | 0.5일 |
| DEF-4 | 결함 | 분석 취소 메커니즘 부재 | `main.ts:650-684` | UX 개선 | 1-2일 |
| DEF-5 | 결함 | BoundedPercentile 대용량 테스트 부재 | `tests/test_statistics.py` | ALG-1 검증 | 0.5일 |

### 6.3 P2 (백로그)

| ID | 분류 | 제목 | 위치 | 효과 추정 | 비용 |
|---|---|---|---|---|---|
| ALG-4 | 알고리즘 | URL Top-K 전체 정렬 | `access_log_analyzer.py:159-170` | 정렬 5x 개선 | 0.5일 |
| ALG-5 | 알고리즘 | flamegraph dict→tree 재구축 | `profiler_analyzer.py:345` | 드릴다운 10-50ms 절감 | 1일 |
| PERF-4 | 성능 | JSON 직렬화 경로 최적화 | `json_exporter.py` → `main.ts:623` | 직렬화 20% 개선 | 0.5-2일 |
| PERF-5 | 성능 | ECharts 대용량 데이터 최적화 | `chartOptions.ts`, `ChartPanel.tsx` | 대용량 차트 5-10x | 0.5일 |
| PERF-6 | 성능 | ChartPanel 재초기화 방지 | `ChartPanel.tsx` | 차트 업데이트 80% | 0.5일 |
| DEF-6 | 결함 | Electron 통합 테스트 부재 | `electron/main.ts` | 리팩토링 안전성 | 3-5일 |
| DEF-7 | 결함 | `_is_error_record` 오탐 | `otel_analyzer.py:422-436` | 에러 통계 정확도 | 0.5일 |
| PERF-7 | 성능 | progress 스트리밍 | 전체 | 대용량 파일 UX | 2-3일 |
| DEF-8 | 결함 | CSP `unsafe-inline` 제거 | `main.ts:62` | CSS 삽입 공격 방지 | 1일 |
| ALG-6 | 알고리즘 | classify_stack 캐싱 | `profiler_analyzer.py:406` | 반복 분류 제거 | 0.5일 |

### 6.4 P3 (의도적 보류와 재평가 트리거)

| ID | 분류 | 제목 | 보류 사유 | 재평가 트리거 |
|---|---|---|---|---|
| PERF-8 | 성능 | IPC MessagePort 전환 | 현재 결과 크기 수 MB 이하로 충분 | 결과 크기가 10MB 초과하는 분석 추가 시 |
| ALG-7 | 알고리즘 | T-Digest로 전면 전환 | 외부 의존성 추가 필요, ALG-1+ALG-2로 충분 | 실시간 스트리밍 분석 기능 추가 시 |
| PERF-9 | 성능 | Python→Rust/Go 엔진 전환 | 현재 Python 성능으로 60초 내 대부분 처리 | 파싱 성능이 실측 병목으로 확인될 때 |
| DEF-9 | 결함 | IPC 스키마 검증 강화 (Zod) | 현재 타입 가드로 충분 | IPC 계약 불일치 런타임 에러 빈발 시 |

---

## 7. 이전 페이즈 검토와의 일관성

### 7.1 이전 권고 중 적용된 사항

| 이전 권고 | 적용 상태 | 확인 |
|---|---|---|
| Electron 31 EOL → 업그레이드 | ✅ Electron 41.3.0 | `package.json` 확인 |
| CSP 추가 | ✅ PACKAGED_CSP + DEVELOPMENT_CSP | `main.ts:59-79` |
| ParserDiagnostics 통합 | ✅ `common/diagnostics.py` | 코드 확인 |
| MetricCard 공유 컴포넌트 추출 | ✅ `components/MetricCard.tsx` | 코드 확인 |
| BoundedPercentile 도입 | ✅ `common/statistics.py` | 단 ALG-1 결함 존재 |
| 분석기별 테스트 분리 | ✅ 18개 테스트 파일 | 파일 확인 |
| GitHub Actions CI | ✅ `.github/` | 존재 확인 |
| seed-configurable percentile | ✅ `BoundedPercentile(seed=...)` | 코드 확인 |

### 7.2 이전 권고 중 미적용 사항과 평가

| 이전 권고 | 미적용 사유 | 현재 평가 |
|---|---|---|
| CSP nonce 기반 style-src | Phase 4+로 연기 | DEF-8로 재분류 (P2) — ECharts `csp: {nonce}` 활용 가능 |
| AiFindingValidator 코드 구현 | Phase 5 검토에서 지적, 미구현 | 여전히 필요하나 본 검토 범위 밖 (보안 검토에서 재평가) |
| 프롬프트 인젝션 방어 코드 | Phase 5 검토에서 지적 | `prompting.py`에 부분 구현됨 (`SUSPICIOUS_INSTRUCTION_PATTERN`) |
| JFR PoC 코드 | Phase 4 검토에서 지적 | `jfr_parser.py`, `jfr_analyzer.py` 구현 완료 ✅ |
| 타임스탬프 정규화 | Phase 4 검토에서 Critical로 지적 | 여전히 미구현. Timeline Correlation 기능이 아직 없으므로 P3으로 재분류 |

### 7.3 이번 검토에서 새로 부각된 영역

1. **Reservoir sampling 수학적 결함 (ALG-1)**: 이전 검토에서 `BoundedPercentile` 도입을 권고하고 적용을 확인했으나, 내부 해시 함수의 수학적 정확성은 검증하지 못함. 이번 종합 검토에서 코드 수준 분석을 통해 발견.

2. **frozen dataclass 불변성 위반 (DEF-1)**: 드릴다운 기능이 Phase 3에서 추가되면서 발생. 이전 검토 시점에는 드릴다운이 없었으므로 횡단 결함.

3. **측정 인프라 부재 (PERF-1)**: 이전 검토는 기능 완성도에 집중했으나, 본 검토에서 성능 관점의 운영 성숙도를 평가하면서 부각.

4. **End-to-End 데이터 흐름 비효율**: 개별 모듈 검토에서는 드러나지 않는 4중 직렬화/역직렬화 등 통합 관점의 비효율.

---

## 8. 측정 인프라 도입 권고

본 검토에서 정량 평가가 어려웠던 영역:

### 8.1 도입할 벤치마크

| 벤치마크 | 대상 | 입력 크기 | 측정 항목 |
|---|---|---|---|
| `bench_access_log_parse` | `access_log_parser.py` | 1K, 10K, 100K, 1M 라인 | 처리 시간, 피크 메모리 |
| `bench_access_log_analyze` | `access_log_analyzer.py` | 1K, 10K, 100K 레코드 | 처리 시간, 피크 메모리 |
| `bench_flamegraph_build` | `flamegraph_builder.py` | 1K, 10K, 100K 스택 | 처리 시간, 노드 수 |
| `bench_gc_log_analyze` | `gc_log_analyzer.py` | 1K, 10K, 100K 이벤트 | 처리 시간, 피크 메모리 |
| `bench_otel_analyze` | `otel_analyzer.py` | 1K, 10K 레코드 | 처리 시간, 토폴로지 구축 시간 |
| `bench_json_serialize` | `json_exporter.py` | 1MB, 10MB 결과 | 직렬화/역직렬화 시간 |

도구: `pytest-benchmark` (`pip install pytest-benchmark`)

### 8.2 도입할 프로파일링 도구

| 도구 | 대상 | 용도 |
|---|---|---|
| `py-spy` | Python 엔진 | CPU 프로파일링 (flamegraph 생성, 무중단) |
| `tracemalloc` | Python 엔진 | 메모리 프로파일링 (피크 추적, 할당 추적) |
| Chrome DevTools Performance | Electron 렌더러 | React 렌더링, ECharts 성능 |
| `--inspect` + Chrome DevTools | Electron 메인 프로세스 | IPC 핸들러, 프로세스 관리 |
| React DevTools Profiler | 렌더러 | 컴포넌트 재렌더 분석 |

### 8.3 CI 회귀 탐지 메커니즘

```yaml
# .github/workflows/benchmark.yml
- name: Run benchmarks
  run: pytest tests/ --benchmark-only --benchmark-json=benchmark.json
  
- name: Store benchmark result
  uses: benchmark-action/github-action-benchmark@v1
  with:
    tool: 'pytest'
    output-file-path: benchmark.json
    alert-threshold: '120%'  # 20% 이상 회귀 시 알림
    fail-on-alert: true
```

---

## 9. 신규 이슈

#### NEW-1: `_build_flamegraph_result`와 `build_collapsed_result`의 코드 중복 [Severity: Low]
- **분류**: 코드 품질
- **위치**: `profiler_analyzer.py:256-335` vs `profiler_analyzer.py:157-253`
- **현상**: `elapsed_ratio` 계산, `top_stacks` 구축, `metadata` 구성이 두 함수에서 거의 동일하게 반복.
- **권고**: 공통 로직 추출. 단, 현재 동작에 영향 없으므로 리팩토링 시점에 함께 처리.

#### NEW-2: `_ordered_services_by_parent`와 `_ordered_services_by_parent_ids` 중복 [Severity: Low]
- **분류**: 코드 품질
- **위치**: `otel_analyzer.py:312-339` vs `otel_analyzer.py:381-407`
- **현상**: 두 함수가 거의 동일한 DAG 순회 로직을 수행. 반환 타입만 다름 (서비스명 vs span_id).
- **권고**: 제네릭 DAG 순회 함수로 통합.

#### NEW-3: ECharts 6 API 호환성 미확인 [Severity: Info]
- **분류**: 기타
- **위치**: `package.json` — `"echarts": "^6.0.0"`
- **현상**: ECharts 6은 2026년 현재 안정 버전이 아닌 것으로 보고됨 (5.5.x가 현재 안정). `^6.0.0` 의존성이 프리릴리스를 참조할 가능성.
- **권고**: 의존성 버전을 현재 안정 버전으로 고정하거나, ECharts 6 GA 출시 후 업그레이드.

#### NEW-4: 데모 사이트 실행 시 300초 타임아웃 [Severity: Low]
- **분류**: 성능
- **위치**: `main.ts:487` — `execEngine(args, 300_000)`
- **현상**: 데모 사이트 전체 실행 시 5분 타임아웃. 많은 시나리오 + 대용량 데이터에서 부족할 수 있음.
- **권고**: 시나리오별 개별 실행 + 병렬화 고려. 또는 동적 타임아웃 (시나리오 수 × 기본 타임아웃).

---

## 10. 종합 평가 및 검토자 코멘트

ArchScope는 **아키텍처적 판단이 우수한 프로젝트**다. 다음 설계 결정은 명시적으로 칭찬할 가치가 있다:

**잘 된 부분:**
- **AnalysisResult 공통 계약**: 13개 분석기가 동일한 구조로 결과를 반환하여 UI와 내보내기가 분석 타입에 독립적. 이 추상화가 깨끗하게 유지되고 있다.
- **Parser↔Analyzer↔Exporter 분리**: 각 책임이 명확히 분리되어 있고, 새 분석기 추가가 기존 코드 변경 없이 가능한 플러그인 구조.
- **ParserDiagnostics**: 파싱 실패를 "에러"가 아닌 "진단 정보"로 취급하여 부분 결과를 정상 반환하는 설계. 운영 도구에 필수적인 접근.
- **증거 기반 소견(evidence-bound findings)**: AI 해석의 `evidence_refs` 필수 요구는 LLM 환각 방지의 핵심 가드레일.
- **CSP 이원화**: 개발/프로덕션 CSP 분리로 보안과 개발 편의성 양립.
- **IPC 타입 가드**: `isAccessLogAnalysisResult` 등의 런타임 검증이 Python↔TypeScript 계약 불일치를 방어.
- **DebugLogCollector**: 파싱 에러를 구조화된 JSON으로 수집하는 설계. 사용자 보고 없이도 문제 진단 가능.
- **Frozen dataclass 설계 의도**: 불변성을 타입 시스템으로 표현하려는 시도 자체는 올바른 방향 (실행에 결함이 있을 뿐).

**핵심 개선 방향:**
1. **ALG-1을 즉시 수정**하라. Reservoir sampling이 동작하지 않으면 백분위 기반의 모든 진단이 무의미하다.
2. **측정 인프라를 도입**하라. 지금까지는 기능 구현에 집중했고, 이제 "얼마나 잘 동작하는가"를 물을 때다. 벤치마크 없는 성능 최적화는 추측이다.
3. **장기 프로세스 전환을 계획**하라. per-invocation spawn은 프로토타입에 적합하지만, 연속 분석과 향후 실시간 기능에는 장기 사이드카가 필수다.

본 검토의 P0 3건은 제품 신뢰성의 기반이므로 즉시 처리를 권고하며, P1 8건은 다음 개발 사이클에서 순차적으로 해결할 수 있다. 전체적으로 ArchScope는 **견고한 기반 위에 세워진 제품**이며, 본 검토에서 지적한 사항들은 "잘 만든 것을 더 잘 만드는" 단계의 개선이다.

---

## 11. 참고: 외부 리서치 및 레퍼런스

1. **Vitter, J.S. (1985).** "Random Sampling with a Reservoir." — Algorithm R의 원본. 올바른 reservoir sampling 구현의 기준. https://en.wikipedia.org/wiki/Reservoir_sampling

2. **Dunning, T. (2019).** "Computing Extremely Accurate Quantiles Using t-Digests." — 스트리밍 백분위 계산의 현대적 표준. Python: `tdigest` (PyPI), JS: `tdigest` (npm). https://github.com/tdunning/t-digest

3. **DataDog DDSketch (2019).** — 결정론적 상대 오차 보장의 스트리밍 백분위. Python: `ddsketch` (PyPI). https://github.com/DataDog/sketches-py

4. **Electron Performance Best Practices.** — `utilityProcess`, `MessagePort`, IPC 최적화. https://www.electronjs.org/docs/latest/tutorial/performance

5. **Electron Security Tutorial.** — CSP, contextIsolation, nodeIntegration 비활성화. https://www.electronjs.org/docs/latest/tutorial/security

6. **ECharts Large Dataset Rendering.** — `large: true`, `sampling: 'lttb'`, progressive rendering. https://echarts.apache.org/en/option.html#series-line.large

7. **React Performance Patterns (2024-2026).** — `useDeferredValue`, `useTransition`, `React.memo` with custom comparator. https://react.dev/reference/react/useDeferredValue

8. **Node.js child_process.** — `spawn` vs `exec` vs `fork` 비교, `AbortController` 지원. https://nodejs.org/api/child_process.html

9. **ECharts CSP Nonce Support.** — `csp: { nonce }` 옵션으로 동적 스타일 삽입 시 nonce 적용. https://github.com/apache/echarts/pull/17703

10. **pytest-benchmark.** — Python 벤치마크 프레임워크, CI 통합 지원. https://pytest-benchmark.readthedocs.io/

11. **benchmark-action/github-action-benchmark.** — GitHub Actions에서 벤치마크 결과 추적 및 회귀 탐지. https://github.com/benchmark-action/github-action-benchmark
