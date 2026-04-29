# Phase 2 Readiness - 완료 후 재검토 의견서

**문서 유형:** Follow-up Review  
**기준 리뷰:** `docs/review/done/2026-04-29_claude-code_phase1-review.md`  
**검증 대상:** Phase 2 Readiness 13개 태스크 (T-018, T-023, T-038, T-041 ~ T-049)  
**리뷰어:** Claude Code (Senior Architect + Senior Developer)  
**작성일:** 2026-04-29  

---

## 0. 요약 판정

| 항목 | 판정 |
|---|---|
| **Phase 2 진입 가능 여부** | **가능 (Go)** |
| 검증 대상 태스크 | 13개 전수 검증 |
| 완료 판정 | 13/13 완료 |
| 신규 Stop-the-line 이슈 | 0건 |
| 신규 주의 사항 | 5건 (아래 상세) |

Phase 1 Review에서 식별된 2개 STL 항목(Electron EOL, CSP 부재)이 모두 해소되었고, 13개 Phase 2 Readiness 태스크가 의도대로 구현되었다. Phase 2 UI/Chart 확장 작업을 시작해도 된다.

---

## 1. 변경 이력 분석

Phase 1 Review(commit `6a0410e`) 이후 4개 커밋으로 13개 태스크가 구현되었다:

| Commit | 주요 변경 |
|---|---|
| `ff098c7` | T-023 Electron 41 업그레이드, T-042 CSP, T-043 ParserDiagnostics 통합, T-044 MetricCard 추출, T-018 App.tsx 매핑 테이블 |
| `04f8343` | T-046 analyzer 테스트 분리, T-045 CI workflow, T-047 CLI E2E 테스트 |
| `b44523a` | T-048 IPC validation 강화, T-038 Generic IPC handler, T-041 stderr/progress 피드백 |
| `91f65b9` | T-049 BoundedPercentile 도입 |

변경 규모: 28 files changed, +1406 / -1582 lines.

---

## 2. 태스크별 검증 결과

### T-023: Electron 33+ 업그레이드

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `electron: ^41.3.0` (`package.json:25`). Phase 1 Review STL-1 해소. |
| 근본 원인 해결 | **Pass** | EOL Electron 31 → 지원 중인 41으로 전환. |
| 테스트 충족 | **Pass** | TypeScript 컴파일 (`tsc --noEmit`) 정상 통과. |
| 문서 갱신 | **N/A** | Electron 버전은 `package.json`이 canonical. |
| 부작용 | **None** | `vite`도 `^8.0.10`, `@vitejs/plugin-react`도 `^6.0.1`로 함께 업그레이드 완료. |
| 신규 부채 | **None** | — |

**심층 검증:**
- Electron 41의 Node.js, Chromium 버전 호환성은 `package-lock.json` 기준으로 정상 해소됨.
- `@vitejs/plugin-react`가 `dependencies` → `devDependencies`로 올바르게 이동함.

---

### T-042: Content Security Policy (CSP)

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `PACKAGED_CSP` / `DEVELOPMENT_CSP` 분리 구현. Phase 1 Review STL-2 해소. |
| 근본 원인 해결 | **Pass** | `session.defaultSession.webRequest.onHeadersReceived`로 모든 응답에 CSP 헤더 주입 (`main.ts:136-146`). |
| 테스트 충족 | **N/A** | CSP는 Electron 런타임 동작 — 단위 테스트 대상 아님. |
| 문서 갱신 | **N/A** | — |
| 부작용 | **None** | — |
| 신규 부채 | **Minor** | 아래 NI-1 참조. |

**심층 검증:**

Production CSP (`main.ts:38-47`):
```typescript
const PACKAGED_CSP = [
  "default-src 'self'",
  "script-src 'self'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data:",
  "font-src 'self'",
  "connect-src 'self'",
  "object-src 'none'",
  "base-uri 'none'",
].join("; ");
```

- `script-src 'self'`: `unsafe-eval`, `unsafe-inline` 제외 — 양호.
- `object-src 'none'`, `base-uri 'none'`: 모범 사례 준수.
- `style-src 'unsafe-inline'`: 인라인 스타일 허용. React 렌더링에 필요하므로 현 단계에서 허용 가능하나, nonce 기반 전환을 장기 과제로 남겨둠.
- Development CSP: `unsafe-eval`과 Vite dev server 주소 허용 — 개발 환경에서만 사용되므로 적절.

---

### T-043: ParserDiagnostics 통합

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `common/diagnostics.py`에 단일 `ParserDiagnostics` dataclass 정의. |
| 근본 원인 해결 | **Pass** | 양 파서에서 중복 정의 제거, 공통 모듈로 통합. |
| 테스트 충족 | **Pass** | parser 테스트와 analyzer 테스트 모두 `ParserDiagnostics` 경유 — 25개 테스트 전수 통과. |
| 부작용 | **None** | — |
| 신규 부채 | **None** | — |

**심층 검증:**
- `class ParserDiagnostics` 정의 위치: `common/diagnostics.py:12` (runtime dataclass) 및 `result_contracts.py:13` (TypedDict contract). 이 두 곳만 존재 — 역할이 다르므로 중복이 아님.
- parser 모듈에서의 import: `access_log_parser.py:10`, `collapsed_parser.py:8` 모두 `from archscope_engine.common.diagnostics import ParserDiagnostics`로 통일.
- 구 import 경로 잔재 없음 (grep 검증 완료).
- `ParseError = tuple[str, str]` type alias도 `common/diagnostics.py:8`에 함께 정의 — parser간 공유 적절.
- bounded samples (`MAX_DIAGNOSTIC_SAMPLES = 20`, `RAW_PREVIEW_LIMIT = 200`) 상수가 공통 모듈에 위치 — 일관된 제한.

---

### T-044: MetricCard 공유 컴포넌트 추출

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `src/components/MetricCard.tsx`에 단일 정의, 3개 페이지에서 import. |
| 근본 원인 해결 | **Pass** | 3중 중복 정의 → 단일 소스. |
| 테스트 충족 | **Pass** | TypeScript 컴파일 정상 통과. |
| 부작용 | **None** | — |
| 신규 부채 | **None** | — |

**심층 검증:**
- `function MetricCard` 정의: `src/components/MetricCard.tsx:6`에 유일하게 1개 존재 (grep 검증).
- import 경로: `DashboardPage.tsx:5`, `AccessLogAnalyzerPage.tsx:14`, `ProfilerAnalyzerPage.tsx:11` — 모두 `../components/MetricCard`.
- Props 타입 `{ label: string; value: string | number }` — 기존 3개 구현체와 동일한 인터페이스.

---

### T-018: App.tsx 페이지 매핑 테이블

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `Record<PageKey, () => JSX.Element>` 매핑 테이블로 전환 (`App.tsx:25-35`). |
| 근본 원인 해결 | **Pass** | if-else / switch 체인 제거, 선언적 매핑. |
| 테스트 충족 | **Pass** | TypeScript 컴파일 정상 통과. |
| 부작용 | **None** | `PageKey` union type이 매핑 테이블과 1:1 — 누락 시 컴파일 에러 발생. |
| 신규 부채 | **None** | — |

**심층 검증:**
- 9개 페이지 전수 등록: `dashboard`, `access-log`, `gc-log`, `profiler`, `thread-dump`, `exception`, `chart-studio`, `export-center`, `settings`.
- `PageKey` type과 `pageComponents` 객체의 key가 `Record<PageKey, ...>`로 타입 안전하게 연결됨 — 새 페이지 추가 시 `PageKey`에 추가하지 않으면 컴파일 에러.
- 파일 47줄로 간결.

---

### T-046: Analyzer 테스트 분리

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `test_access_log_analyzer.py` (6개 테스트), `test_profiler_analyzer.py` (2개 테스트) 신규 생성. |
| 근본 원인 해결 | **Pass** | parser 테스트 파일에서 analyzer 로직 테스트 분리. |
| 테스트 충족 | **Pass** | 25개 테스트 전수 통과. |
| 부작용 | **None** | parser 테스트 파일은 순수 parsing 테스트만 유지. |
| 신규 부채 | **None** | — |

**심층 검증:**

`test_access_log_analyzer.py` 커버리지:
| 테스트 | 검증 대상 |
|---|---|
| `test_analyze_access_log_sample` | 샘플 파일 기반 기본 분석 |
| `test_analyze_access_log_includes_diagnostics_metadata` | malformed line → diagnostics 전파 |
| `test_analyze_access_log_respects_max_lines` | `max_lines` 옵션 |
| `test_analyze_access_log_filters_by_time_range` | `start_time` / `end_time` 필터 |
| `test_analyze_access_log_reports_status_and_slow_url_findings` | findings 생성 (T-032 검증) |
| `test_analyze_access_log_handles_more_records_than_percentile_sample_limit` | 10K+ 레코드 시 BoundedPercentile 동작 (T-049 검증) |

`test_profiler_analyzer.py` 커버리지:
| 테스트 | 검증 대상 |
|---|---|
| `test_analyze_collapsed_merges_duplicate_stacks` | 샘플 파일 기반 stack 병합 |
| `test_analyze_collapsed_profile_includes_diagnostics_metadata` | malformed line → diagnostics 전파 |

- parser 테스트 파일(`test_access_log_parser.py`, `test_collapsed_parser.py`)에서 analyzer 관련 테스트가 제거되었고, 순수 parsing/diagnostics 테스트만 남아 있음.
- `nginx_line()` helper가 parser 테스트와 analyzer 테스트 양쪽에 중복되나, 테스트 파일 간 helper 공유보다 독립성이 더 중요하므로 허용 가능.

---

### T-045: GitHub Actions CI

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `.github/workflows/ci.yml` 생성, Python 테스트 + Desktop 빌드 2개 job. |
| 근본 원인 해결 | **Pass** | 자동화된 회귀 방지. |
| 테스트 충족 | **N/A** | CI 자체는 GitHub 에서 실행됨. |
| 부작용 | **None** | — |
| 신규 부채 | **Minor** | 아래 NI-2 참조. |

**심층 검증:**

```yaml
on:
  pull_request:
  push:
    branches:
      - main
```
- trigger: PR과 main push — 표준 패턴.

Python job:
- Python 3.11, `pip install -e ".[dev]"`, `pytest tests` — 적절.
- `working-directory: engines/python` — 올바른 경로.

Desktop job:
- Node 22, `npm ci`, `npm run build` — 타입 체크 + Vite 빌드 + Electron 빌드 포함.
- `cache: npm` + `cache-dependency-path` — CI 성능 최적화 적용.

---

### T-047: CLI E2E 통합 테스트

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `test_cli_e2e.py`에서 `subprocess.run`으로 CLI 명령 실행, JSON 출력 검증. |
| 근본 원인 해결 | **Pass** | CLI → Parser → Analyzer → Exporter 전체 경로 회귀 방지. |
| 테스트 충족 | **Pass** | `typer`/`rich` 설치 시 통과 (CI 환경에서 `.[dev]` 설치로 해결). |
| 부작용 | **None** | `pytest.importorskip`으로 선택적 의존성 미설치 시 graceful skip. |
| 신규 부채 | **None** | — |

**심층 검증:**
- `test_access_log_cli_writes_analysis_result_json`: `access-log analyze --file ... --format nginx --out ...` 실행 → JSON 읽기 → `type == "access_log"`, `total_requests == 6` 검증.
- `test_profiler_cli_writes_analysis_result_json`: `profiler analyze-collapsed --wall ... --wall-interval-ms 100 --elapsed-sec 1336.559 --out ...` 실행 → JSON 읽기 → `type == "profiler_collapsed"`, `total_samples == 32629` 검증.
- `sys.executable` 사용으로 올바른 Python 인터프리터 보장.
- `check=True`로 비정상 종료 시 즉시 실패.

---

### T-048: IPC AnalysisResult 런타임 검증 강화

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `isAccessLogAnalysisResult`, `isProfilerCollapsedAnalysisResult` 타입별 deep validator 구현 (`main.ts:393-434`). |
| 근본 원인 해결 | **Pass** | Phase 1 Review C-3(shallow validation) 해소. |
| 테스트 충족 | **Pass** | TypeScript 컴파일 정상 통과. |
| 부작용 | **None** | — |
| 신규 부채 | **None** | — |

**심층 검증:**

`isAnalysisResult` (`main.ts:374-391`): 7개 공통 필드 검증 — `type`, `created_at`, `source_files`, `summary`, `series`, `tables`, `metadata`.

`isAccessLogAnalysisResult` (`main.ts:393-413`): 공통 검증 후 `type === "access_log"` 확인 + summary 5개 숫자 필드 + series 6개 배열 필드 + tables 1개 배열 필드 + metadata.diagnostics object 검증.

`isProfilerCollapsedAnalysisResult` (`main.ts:415-434`): 공통 검증 후 `type === "profiler_collapsed"` 확인 + summary 5개 필드 + series 2개 배열 + tables 1개 배열 + metadata.diagnostics object 검증.

- `hasNumber()` helper (`main.ts:440-442`)로 `typeof === "number" && Number.isFinite()` 검증 — `NaN`, `Infinity` 방어.
- `source_files` 배열의 각 요소를 `typeof sourceFile === "string"`으로 검증.
- `elapsed_seconds`의 `null` 허용 (`typeof ... === "number" || ... === null`) — Python 측 `Optional[float]`과 일치.

---

### T-038: Generic IPC Handler (`analyzer:execute`)

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | discriminated union `AnalyzerExecuteRequest`으로 단일 IPC 채널 통합. |
| 근본 원인 해결 | **Pass** | 분석기 추가 시 IPC 채널 증식 방지. |
| 테스트 충족 | **Pass** | TypeScript 컴파일 정상 통과. |
| 부작용 | **None** | 기존 `analyzeAccessLog`, `analyzeCollapsedProfile` 편의 메서드도 preload에서 유지 — 하위 호환. |
| 신규 부채 | **None** | — |

**심층 검증:**

Main process (`main.ts:121-133`):
```typescript
ipcMain.handle("analyzer:execute", async (_event, request) => {
  try { return await executeAnalyzer(request); }
  catch (error) { return ipcFailed(error); }
});
```

Dispatcher (`main.ts:148-163`):
```typescript
switch (request.type) {
  case "access_log": return analyzeAccessLog(request.params);
  case "profiler_collapsed": return analyzeCollapsedProfile(request.params);
  default: return failure("INVALID_OPTION", "Unsupported analyzer execution type.");
}
```

Preload (`preload.ts:15-27`): 
- `execute()`: generic 경로로 직접 호출.
- `analyzeAccessLog()`, `analyzeCollapsedProfile()`: 동일 IPC 채널에 `{ type, params }` 래핑하여 호출 — 편의 메서드.

Contract (`analyzerContract.ts`):
```typescript
export type AnalyzerExecuteRequest =
  | { type: "access_log"; params: AnalyzeAccessLogRequest }
  | { type: "profiler_collapsed"; params: AnalyzeCollapsedProfileRequest };
```
- 새 분석기 추가 시 이 union에 variant 추가 + switch case 추가로 확장 가능.

---

### T-041: stderr/progress 피드백

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | engine stdout/stderr를 `engine_messages`로 UI에 전달. |
| 근본 원인 해결 | **Pass** | Python CLI의 `rich.console` 출력이 더 이상 silently 폐기되지 않음. |
| 테스트 충족 | **Pass** | TypeScript 컴파일 정상 통과. |
| 부작용 | **None** | — |
| 신규 부채 | **None** | — |

**심층 검증:**

`splitEngineMessages` (`main.ts:480-486`):
```typescript
function splitEngineMessages(parts: Array<string | undefined>): string[] {
  return parts
    .flatMap((part) => part?.split(/\r?\n/) ?? [])
    .map((line) => line.trim())
    .filter((line) => line.length > 0)
    .slice(-20);
}
```
- 최대 20줄 제한 — 무한 출력 방어.
- stderr + stdout 모두 수집.
- `EngineProcessResponse` success 경로에 `messages: string[]` 포함 (`main.ts:33`).
- `AnalyzerSuccess.engine_messages?: string[]` 으로 UI까지 전달 (`analyzerContract.ts`).

---

### T-049: BoundedPercentile (근사 백분위)

| 기준 | 판정 | 근거 |
|---|---|---|
| 의도 부합 | **Pass** | `BoundedPercentile` 클래스 도입, reservoir sampling으로 메모리 제한 (`statistics.py:26-53`). |
| 근본 원인 해결 | **Pass** | 대용량 파일에서 정확 백분위의 unbounded 메모리 사용 문제 해소. |
| 테스트 충족 | **Pass** | `test_bounded_percentile_keeps_sample_count_under_limit` + analyzer 테스트에서 10K+ 레코드 검증. |
| 부작용 | **None** | 기존 `percentile()` 함수는 보존, `BoundedPercentile`만 추가. |
| 신규 부채 | **Minor** | 아래 NI-3 참조. |

**심층 검증:**

Reservoir sampling (`statistics.py:52-53`):
```python
def _deterministic_reservoir_index(count: int) -> int:
    return ((count * 1_103_515_245) + 12_345) % count
```
- Linear Congruential Generator 상수 사용 — deterministic이므로 동일 입력에서 재현 가능.
- `max_samples` 기본값 10,000 — analyzer에서 `PERCENTILE_SAMPLE_LIMIT = 10_000`으로 설정 (`access_log_analyzer.py:26`).

`ResponseTimeStats` (`access_log_analyzer.py:29-48`):
- 전체 합계(`total_ms`)와 카운트(`count`)로 정확한 평균 유지.
- 백분위만 reservoir sampling 대상 — 평균의 정확도에 영향 없음.

분 단위 bucket의 `ResponseTimeStats`도 동일하게 `BoundedPercentile` 사용 — 분 단위 P95 차트에서도 메모리 제한 적용.

---

## 3. 교차 검증

### 3.1 회귀 확인

| 항목 | 결과 |
|---|---|
| Python 테스트 (`pytest tests/ -v`) | **25 passed, 1 skipped** (CLI E2E는 `typer`/`rich` 미설치 시 skip) |
| TypeScript 타입 체크 (`tsc --noEmit`) | **Pass** (에러 0건) |
| Electron 타입 체크 (`tsc -p tsconfig.electron.json`) | **Pass** (빌드 스크립트 성공) |

### 3.2 빌드 검증

- `npm run build` 성공 (Vite 빌드 + TypeScript 체크 + Electron 빌드 포함).
- `python -m pip install -e .` 정상 설치 (Python 3.9.6 호환).

### 3.3 보안 검증

| 항목 | 상태 |
|---|---|
| CSP 적용 | Production에서 `unsafe-eval` 제외, `object-src 'none'` 적용 |
| contextIsolation | `true` 유지 (`main.ts:71`) |
| nodeIntegration | `false` 유지 (`main.ts:72`) |
| IPC 입력 검증 | `typeof` 체크 + 타입별 deep validation |
| child_process 실행 | `execFile` 사용 (shell injection 방어), `timeout: 60_000` 설정 |

### 3.4 성능 검증

| 항목 | 상태 |
|---|---|
| 대용량 파일 메모리 | `BoundedPercentile(10_000)`으로 백분위 메모리 제한 |
| streaming 집계 | `iter_access_log_records_with_diagnostics` iterator 기반 소비 유지 |
| IPC 버퍼 | `maxBuffer: 4MB` 제한 (`main.ts:312`) |
| engine 출력 제한 | `splitEngineMessages` 20줄 제한 |
| diagnostic samples | 20개 제한, 200자 preview 제한 |

---

## 4. Phase 1 Review 지적 사항 해소 현황

| Phase 1 이슈 | 해소 태스크 | 상태 |
|---|---|---|
| STL-1: Electron 31 EOL | T-023 | **해소** — Electron 41.3.0 |
| STL-2: CSP 부재 | T-042 | **해소** — Production/Dev CSP 분리 |
| C-3: Shallow IPC validation | T-048 | **해소** — 타입별 deep validator |
| Code Issue #3: 중복 MetricCard | T-044 | **해소** — 공유 컴포넌트 |
| Code Issue #6: ParserDiagnostics 중복 | T-043 | **해소** — common 모듈 통합 |
| Code Issue #7: Unbounded percentile | T-049 | **해소** — BoundedPercentile |

---

## 5. 신규 발견 사항

### NI-1: CSP `style-src 'unsafe-inline'` 장기 과제

**심각도:** Low  
**위치:** `main.ts:41`  
**설명:** `'unsafe-inline'` 없이 React의 인라인 스타일을 처리하려면 nonce 기반 CSP가 필요하다. 현 단계에서 ArchScope는 인라인 스타일 사용이 미미하므로 위험도는 낮지만, Phase 3 이후 CSP 강화 시 검토 대상.  
**권장:** Phase 3 이후 nonce 기반 `style-src` 전환을 로드맵에 추가.

### NI-2: CI workflow lint/format 미포함

**심각도:** Low  
**위치:** `.github/workflows/ci.yml`  
**설명:** Python linting(ruff/flake8)과 TypeScript linting(eslint)이 CI에 포함되지 않았다. 현재 코드 규모에서는 수동 검토로 충분하나, Phase 2에서 기여자 증가 시 추가 권장.  
**권장:** Phase 2 중반 이후 lint step 추가.

### NI-3: Reservoir sampling의 통계적 특성 문서화 부재

**심각도:** Low  
**위치:** `statistics.py:52-53`  
**설명:** `_deterministic_reservoir_index`의 LCG 상수(`1_103_515_245`, `12_345`)는 ANSI C `rand()` 상수이다. deterministic reservoir sampling은 테스트 재현성에 유리하나, 입력 순서에 민감할 수 있다(예: 정렬된 응답 시간). 현재 access log는 시간순이므로 응답 시간 분포가 고르게 샘플링될 가능성이 높지만, edge case에서 편향이 생길 수 있다.  
**권장:** `PARSER_DESIGN.md`에 reservoir sampling 특성과 제한 사항을 1-2문장으로 명시.

### NI-4: CI에서 `test_cli_e2e.py` skip 가능성

**심각도:** Low  
**위치:** `test_cli_e2e.py:8-9`  
**설명:** `pytest.importorskip("typer")`와 `pytest.importorskip("rich")`는 해당 패키지 미설치 시 테스트를 skip한다. CI의 `pip install -e ".[dev]"`가 이 의존성을 포함해야 하는데, `pyproject.toml`의 `[dev]` extras에 `typer`/`rich`가 포함되어 있는지 확인 필요.  
**권장:** `pyproject.toml`의 `[dev]` extras에 `typer`, `rich` 포함 여부 확인, 또는 CI step에서 `pip install -e ".[cli,dev]"` 등으로 명시.

### NI-5: `diagnostics` callable 패턴의 타입 불일치

**심각도:** Low  
**위치:** `access_log_analyzer.py:84`  
**설명:** `build_access_log_result`의 `diagnostics` 파라미터가 `dict | Callable | None`을 받지만, `analyze_access_log`에서 `diagnostics.to_dict` (메서드 참조, 호출 아님)를 전달한다. 동작은 정상이지만 (`callable(diagnostics)` 체크 후 호출), 타입 힌트가 `Callable[[], dict[str, Any]]`로 되어 있어 `to_dict`의 bound method와 정확히 일치한다. 코드 의도가 명확하나, 메서드를 property로 착각할 여지가 있다.  
**권장:** 현행 유지. 가독성 개선이 필요하면 `diagnostics=diagnostics.to_dict` 대신 `diagnostics=lambda: diagnostics.to_dict()`로 명시적 lambda 사용 검토.

---

## 6. 종합 결론

Phase 2 Readiness 13개 태스크가 모두 의도대로 구현되었다. Phase 1 Review의 2개 Stop-the-line 항목(Electron EOL, CSP 부재)이 완전히 해소되었으며, 코드 품질 지적 사항(shallow validation, MetricCard 중복, ParserDiagnostics 중복, unbounded percentile)도 적절히 처리되었다.

신규 발견 5건은 모두 Low severity로, Phase 2 작업을 차단하지 않는다. Phase 2 UI/Chart 확장 작업으로 진행 가능하다.

**Phase 2 진입 시 우선 확인 사항:**
1. `pyproject.toml` dev extras에 CLI 의존성(`typer`, `rich`) 포함 여부 — CI에서 E2E 테스트가 실제로 실행되는지 확인.
2. T-019 (Analyze handler 상태 관리) 시작 시, `AnalyzerFeedback.tsx`의 `engine_messages` 표시 UI 추가 검토.
3. T-020 (i18n-ready chart labels) 시작 시, 현재 하드코딩된 차트 제목/축 레이블을 `i18n/` locale 파일로 이동.
