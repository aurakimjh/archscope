# ArchScope 아키텍처 및 기술 진단 보고서

**작성일:** 2026-04-28
**작성자:** Claude Opus 4.6 (Senior Software Architect)
**대상:** archscope (설계 초기 단계)
**검토 범위:** 설계 문서 7건, Python 엔진 전체 소스, Desktop UI 전체 소스, 테스트 코드, 예시 출력

---

## 1. 총평

ArchScope는 "운영 환경 원천 데이터 → 표준화 분석 → 보고서용 시각화"라는 파이프라인을 명확히 정의하고, 그 설계를 코드로 충실히 반영한 초기 단계 프로젝트입니다. 특히 **AnalysisResult라는 단일 계약(contract)으로 엔진과 UI를 분리한 설계**는 이 프로젝트의 가장 강력한 기반이며, 향후 진단 유형 확장 시 큰 장점이 될 것입니다.

다만 설계 초기 단계인 만큼, 아래에서 아키텍처적으로 **지금 결정해야 할 사항**과 **나중에 결정해도 되는 사항**을 구분하여 의견을 드립니다.

---

## 2. 아키텍처 진단

### 2.1 강점 — 잘 설계된 부분

**AnalysisResult 계약의 일관성**
- `summary`, `series`, `tables`, `charts`, `metadata` 구조가 문서(`DATA_MODEL.md`)와 코드(`analysis_result.py`)에서 정확히 일치합니다.
- Access Log Analyzer와 Profiler Analyzer 모두 이 계약을 준수하여, UI 측이 데이터 유형에 무관하게 동일한 렌더링 로직을 사용할 수 있습니다.

**Parser → Analyzer → Exporter 책임 분리**
- Parser는 "파일 → typed record", Analyzer는 "typed record → AnalysisResult", Exporter는 "AnalysisResult → artifact"로 역할이 명확합니다.
- 이 분리 덕분에 Access Log Parser의 테스트(`test_access_log_parser.py`)가 Analyzer와 독립적으로 작성될 수 있었고, 이는 올바른 구조입니다.

**Desktop UI의 초기 설계**
- i18n이 1일차부터 내장되어 있고, `messages.ts`의 타입 안전성(`MessageKey`)이 확보되어 있어 누락 번역이 컴파일 타임에 잡힙니다.
- Sidebar 네비게이션이 데이터 기반(`navItems` 배열)으로 구성되어 확장이 용이합니다.

### 2.2 지금 결정이 필요한 아키텍처 이슈

#### Issue 1: Engine-UI Bridge 방식 미정

**현황:** `analyzerClient.ts`가 하드코딩된 sample data를 반환하고 있어, Python 엔진과의 실제 통합 경로가 없습니다.

**왜 지금 결정해야 하는가:** 이 결정이 Desktop 앱의 프로세스 모델, 에러 핸들링, 진행률 표시 등 UI 전체 구조에 영향을 미칩니다. Phase 2(Chart Studio)로 넘어가면 변경 비용이 급격히 증가합니다.

**선택지:**

| 방식 | 장점 | 단점 |
|---|---|---|
| A. Electron IPC + Child Process | 단순, 패키징 용이 | Python 바이너리 번들링 필요 (PyInstaller) |
| B. Local HTTP Server (FastAPI) | 프론트엔드-백엔드 분리 명확, 웹 전환 용이 | 포트 충돌, 프로세스 관리 복잡 |
| C. WASM (Pyodide) | 설치 불필요 | 파일 시스템 접근 제한, 성능 제약 |

**의견:** 데스크톱 전용 도구의 성격상 **방식 A**가 가장 현실적입니다. PyInstaller로 단일 바이너리를 만들고, Electron의 `child_process.execFile`로 호출하면 됩니다. 다만 방식 B를 선택하면 향후 웹 버전으로의 전환이 쉬워지므로, 프로젝트의 배포 전략에 따라 달라집니다.

#### Issue 2: AnalysisResult의 `dict[str, Any]` 타입 안전성

**현황:** `AnalysisResult`의 `summary`, `series`, `tables`, `charts`, `metadata`가 모두 `dict[str, Any]`입니다. 현재는 Access Log과 Profiler 두 유형뿐이라 문제가 없지만, GC/Thread Dump/Exception이 추가되면 각 유형의 필수 키가 무엇인지 코드만으로는 알 수 없게 됩니다.

**왜 지금 결정해야 하는가:** Phase 3(JVM Diagnostics)에서 3개 유형이 동시에 추가됩니다. 그때 구조를 변경하면 기존 2개 유형도 마이그레이션해야 합니다.

**제안:**
```python
# 옵션 A: 유형별 TypedDict
class AccessLogSummary(TypedDict):
    total_requests: int
    avg_response_ms: float
    p95_response_ms: float
    p99_response_ms: float
    error_rate: float

# 옵션 B: Pydantic discriminated union
class AccessLogResult(BaseModel):
    type: Literal["access_log"]
    summary: AccessLogSummary
    ...
```

**의견:** 현재 `dataclass(frozen=True)` 기반 구조도 나쁘지 않지만, 엔진 출력이 UI의 TypeScript 타입과 1:1 대응해야 하므로, 최소한 **각 유형별 summary/series의 필수 키를 TypedDict로 정의**하는 것을 권장합니다. 전면 Pydantic 전환은 이 단계에서 반드시 필요하지는 않습니다.

#### Issue 3: 대용량 파일 처리 전략

**현황:** `iter_text_lines`는 generator 기반으로 파일을 한 줄씩 읽지만, `parse_access_log`가 결과를 `list[AccessLogRecord]`로 모두 메모리에 적재합니다. `analyze_access_log`에서도 전체 리스트를 순회합니다.

**왜 지금 결정해야 하는가:** 운영 환경의 Access Log는 수 GB가 일반적입니다. 100만 줄 이상의 로그에서 OOM이 발생할 수 있고, 이는 사용자가 가장 먼저 마주칠 문제입니다.

**제안:** 두 단계로 접근할 수 있습니다.
1. **단기:** Analyzer에 sampling 옵션 추가 (예: 처음 N줄, 또는 시간 범위 필터)
2. **중기:** Parser를 iterator 기반으로 유지하고, Analyzer가 streaming aggregation을 수행 (Counter/defaultdict를 record별로 업데이트)

### 2.3 나중에 결정해도 되는 사항

- **전역 상태 관리 (Zustand 등):** 현재 `useState`로 페이지 전환만 관리하고 있고, 분석 결과가 아직 실제로 로드되지 않으므로 시기상조입니다. Engine Bridge가 결정된 후 데이터 흐름이 구체화되면 도입해도 늦지 않습니다.
- **Pydantic 전면 전환:** TypedDict로 핵심 계약만 강화하면 현 단계에서는 충분합니다.
- **Chart template JSON화:** ECharts option builder가 이미 함수 단위로 분리되어 있으므로, Phase 2에서 Chart Studio를 구현할 때 자연스럽게 추출하면 됩니다.

---

## 3. 코드 품질 진단

### 3.1 Python 엔진

**잘 된 부분:**
- `frozen=True` dataclass로 불변성 보장
- encoding fallback chain (`file_utils.py`)이 한국어 환경(cp949)을 고려
- `percentile` 함수의 보간(interpolation) 구현이 정확
- CLI가 typer + rich로 사용자 경험 고려

**개선이 필요한 부분:**

| 파일 | 이슈 | 심각도 |
|---|---|---|
| `access_log_parser.py` | `parse_access_log`가 매칭 실패한 줄을 무시(silent skip). 스킵 카운트를 반환하거나 metadata에 기록해야 함 | 중 |
| `collapsed_parser.py:21` | `line.rsplit(maxsplit=1)`이 공백 없는 라인에서 `ValueError` 발생. 예: 빈 줄이 아닌 malformed 줄 | 중 |
| `profiler_analyzer.py:131` | `_classify_stack`의 분류 규칙이 하드코딩. 설정 파일이나 인자로 분리하면 JVM 외 런타임 확장 시 유리 | 낮 |
| `pyproject.toml` | `requires = ["setuptools<64"]` — setuptools 버전 상한이 낮음. 의도적이 아니라면 제거 권장 | 낮 |
| `setup.py` | pyproject.toml과 setup.py가 공존. PEP 621 기반 pyproject.toml 단일 관리 권장 | 낮 |

### 3.2 Desktop UI (TypeScript/React)

**잘 된 부분:**
- `messages.ts`에서 `as const` + `MessageKey` 타입으로 번역 키 누락 방지
- 컴포넌트가 단일 책임 원칙을 잘 따름 (Layout, Sidebar, ChartPanel, FileDropZone)
- ECharts option builder가 UI 컴포넌트와 분리되어 테스트 가능

**개선이 필요한 부분:**

| 파일 | 이슈 | 심각도 |
|---|---|---|
| `App.tsx` | 페이지 라우팅이 `activePage === "..."` 조건문 나열. React Router나 간단한 매핑 객체로 전환하면 페이지 추가 시 한 곳만 수정 | 낮 |
| `AccessLogAnalyzerPage.tsx` | "Analyze" 버튼에 onClick 핸들러 없음. Engine Bridge 결정 전이므로 이해하지만, placeholder라도 있으면 통합 작업이 수월 | 낮 |
| `analyzerClient.ts` | sample data를 직접 import — 실제 엔진 호출 인터페이스(타입 시그니처)를 먼저 정의하고 mock으로 대체하는 방향이 나음 | 중 |
| `package.json` | React 18.2, Electron 31 — 현 시점 기준 다소 오래됨. 보안 패치 측면에서 업데이트 권장 | 낮 |

---

## 4. 테스트 진단

**현재 테스트 커버리지:**

| 모듈 | 테스트 유무 | 비고 |
|---|---|---|
| Access Log Parser | O | 단일 라인 파싱 + 파일 분석 통합 테스트 |
| Collapsed Parser | O | 라인 파싱 + 파일 분석 통합 테스트 |
| GC Log Parser | O | placeholder NotImplementedError 확인만 |
| Thread Dump Parser | X | |
| Exception Parser | X | |
| Exporter (JSON) | X | |
| Statistics | X | percentile/average는 엣지 케이스 테스트 필요 |

**제안:**
- `statistics.py`의 엣지 케이스 테스트 추가 필요: 빈 리스트, 단일 요소, 동일 값 반복, 음수 값
- Access Log Parser의 malformed line 처리 테스트 추가 필요
- JSON Exporter의 라운드트립 테스트 (write → read → 비교) 추가 권장

---

## 5. 아키텍처 리스크 매트릭스

| 리스크 | 영향도 | 발생 가능성 | 완화 방안 |
|---|---|---|---|
| Engine-UI 통합 방식 미확정으로 Phase 2 진입 지연 | 높 | 높 | Phase 1 완료 전에 Bridge PoC 구현 |
| 대용량 로그에서 OOM | 높 | 높 | streaming aggregation 또는 sampling |
| GC/Thread Dump 파서 추가 시 AnalysisResult 구조 혼란 | 중 | 중 | 유형별 TypedDict 사전 정의 |
| Desktop 패키징(PyInstaller + electron-builder) 통합 난이도 | 중 | 중 | CI에서 조기 패키징 테스트 |
| 런타임 확장(Node.js, Python, Go) 시 분류 규칙 하드코딩 문제 | 낮 | 낮 | 설정 기반 분류 규칙 분리 |

---

## 6. 권장 우선순위 (Phase 1 완료 전)

1. **Engine-UI Bridge PoC** — Electron에서 Python CLI를 호출하고 JSON 결과를 받아 렌더링하는 최소 경로 구현
2. **Access Log Parser의 malformed line 처리** — 스킵 카운트를 metadata에 기록, 사용자에게 피드백
3. **AnalysisResult 유형별 TypedDict 정의** — Phase 3 진입 전 계약 강화
4. **statistics.py 엣지 케이스 테스트** — 빈 리스트, 단일 요소 등

---

**검토자:** Claude Opus 4.6 (Senior Software Architect)
**검토 상태:** 완료
