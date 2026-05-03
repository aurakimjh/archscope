# Phase 1 완료 검토 의견서

- **작성일**: 2026-04-29
- **작성자**: Claude Code (Opus 4.6)
- **대상 프로젝트**: ArchScope — Application Architecture Diagnostic & Reporting Toolkit
- **검토 범위**: main 브랜치 전체 (`f3b2ded` ~ `c64044c`, 24 commits)
- **검토자 관점**: 시니어 아키텍트 + 시니어 개발자

---

## 0. Executive Summary

- **종합 평가**: ArchScope Phase 1은 초기 프로젝트치고 놀라울 정도로 설계 규율이 잡혀 있다. Parser → Analyzer → AnalysisResult → Exporter → UI Renderer라는 데이터 흐름이 일관되고, AnalysisResult 공통 컨트랙트가 처음부터 확립되어 다중 분석기 확장의 기반이 마련되었다. Electron IPC Bridge PoC가 동작하고, 타입 컨트랙트가 Python/TypeScript 양쪽에서 정렬되며, 23개 테스트가 전수 통과한다. 다만 ParserDiagnostics 클래스 중복, 프로파일러 분류 규칙 하드코딩, Electron 31 보안 지원 종료, UI 테스트 부재 등은 Phase 2 확장 전에 정리가 필요한 기술 부채다.
- **Phase 2 진입 가능 여부**: ⚠️ 조건부 가능
- **Stop-the-line 항목 수**: 2건
- **권장 후속 조치 Top 3**:
  1. ParserDiagnostics 클래스를 단일 모듈로 통합하고 파서 간 중복을 제거할 것
  2. Electron을 지원 버전(v33+)으로 업그레이드하고 Chromium 보안 패치를 확보할 것
  3. Analyzer 단위 테스트를 파서 테스트와 분리하여 독립적으로 작성할 것

---

## 1. 프로젝트 컨텍스트

### 1.1 Phase 1 목표 및 달성도

Phase 1 목표는 `ROADMAP.md`와 `work_status.md`에 18개 태스크(T-001~T-017, T-030~T-032, T-037)로 정의되어 있었으며, **전수 완료 상태**이다.

| 목표 항목 | 달성 상태 | 비고 |
|---|---|---|
| Repository skeleton & 프로젝트 구조 | ✅ 완료 | `apps/desktop/`, `engines/python/`, `docs/`, `examples/`, `scripts/` |
| Desktop UI skeleton (Electron + React + TS) | ✅ 완료 | 9개 페이지, i18n, 차트 대시보드 |
| Python engine skeleton | ✅ 완료 | CLI, 5개 파서(2 구현 + 3 placeholder), 5개 분석기(2 구현 + 3 placeholder) |
| Access log parser MVP | ✅ 완료 | NGINX 포맷, diagnostics, malformed-line 처리 |
| Collapsed profiler parser MVP | ✅ 완료 | async-profiler, diagnostics, 스택 병합 |
| JSON result format & AnalysisResult 컨트랙트 | ✅ 완료 | Python TypedDict + TypeScript 타입 정렬 |
| Engine-UI Bridge PoC | ✅ 완료 | Electron IPC + `child_process.execFile` |
| Parser diagnostics & encoding fallback | ✅ 완료 | `iter_text_lines` fallback chain, 진단 메타데이터 |
| 대규모 파일 기본 대응 | ✅ 완료 | `max_lines`, 시간 범위 필터, 스트리밍 집계 |
| 테스트 | ✅ 완료 | 23개 테스트 전수 통과 (0.04s) |
| 영문/한국어 문서 및 UI i18n | ✅ 완료 | `docs/en/`, `docs/ko/`, `messages.ts` 이중 언어 |

### 1.2 기술 스택 인벤토리

| 계층 | 기술 | 버전 |
|---|---|---|
| Desktop Shell | Electron | 31.1.0 |
| UI Framework | React | 18.2.0 |
| UI 언어 | TypeScript | 5.5.3 |
| 차트 | Apache ECharts | 5.5.1 |
| 빌드 (UI) | Vite | 5.3.3 |
| 엔진 언어 | Python | >=3.9 (runtime: 3.9.6) |
| CLI | Typer | >=0.12,<1.0 |
| 엔진 패키징 | setuptools | <64 (ceiling 있음) |
| 테스트 | pytest | >=8,<9 |
| 프로세스 관리 | concurrently + wait-on | 8.x / 7.x |

**의존성 특기 사항**:
- Electron 31은 2025년 3월에 EOL됨 — 현재 보안 패치를 받지 못하는 상태
- `setuptools<64` ceiling은 `pyproject.toml`에 의도적으로 설정되어 있으나, 이유가 문서화되지 않음
- Python 의존성이 `typer`와 `rich` 단 2개로 매우 경량 — pandas는 optional으로 선언되었으나 미사용
- React 18.2는 아직 유효한 LTS지만, Electron 업그레이드와 맞물려 React 19 평가 시점이 올 수 있음

### 1.3 아키텍처 개요

```text
┌─────────────────────────────────────────────────┐
│                  Desktop UI                      │
│  React + TypeScript + ECharts                    │
│  ┌────────────┐  ┌───────────┐  ┌────────────┐  │
│  │ Pages      │  │ Charts    │  │ Components │  │
│  │ (9 pages)  │  │ Options   │  │ (Layout,   │  │
│  │            │  │ + Theme   │  │  Sidebar,  │  │
│  │            │  │           │  │  Feedback) │  │
│  └─────┬──────┘  └───────────┘  └────────────┘  │
│        │                                         │
│  ┌─────▼──────┐                                  │
│  │ Analyzer   │  analyzerContract.ts (types)     │
│  │ Client     │  analyzerClient.ts (mock + IPC)  │
│  └─────┬──────┘                                  │
│        │ ipcRenderer.invoke()                    │
├────────┼─────────────────────────────────────────┤
│  ┌─────▼──────┐  preload.ts                      │
│  │ Preload    │  contextBridge.exposeInMainWorld  │
│  └─────┬──────┘                                  │
│        │ ipcMain.handle()                        │
│  ┌─────▼──────┐  main.ts                         │
│  │ Electron   │  child_process.execFile           │
│  │ Main       │  → temp JSON → validate → return │
│  └─────┬──────┘                                  │
├────────┼─────────────────────────────────────────┤
│  ┌─────▼──────┐                                  │
│  │ Python CLI │  archscope_engine.cli             │
│  │ (Typer)    │                                   │
│  └─────┬──────┘                                  │
│        │                                         │
│  ┌─────▼──────┐  ┌───────────┐  ┌────────────┐  │
│  │ Analyzers  │→ │ Models    │  │ Exporters  │  │
│  │            │  │ (Result,  │  │ (JSON)     │  │
│  └─────┬──────┘  │  Record,  │  └────────────┘  │
│        │         │  Contract)│                   │
│  ┌─────▼──────┐  └───────────┘                   │
│  │ Parsers    │  + common/ (file, time, stats)   │
│  └────────────┘                                  │
│              Python Engine                        │
└─────────────────────────────────────────────────┘
```

---

## 2. 아키텍처 진단

| 평가 축 | 점수(1-5) | 근거 요약 |
|---|---|---|
| 계층 분리 | 4/5 | Parser → Analyzer → Exporter → UI 분리가 명확. 단, UI에서 Analyzer 직접 호출하는 사실상의 2-tier 구조 |
| 결합도/응집도 | 3/5 | 모듈 경계는 좋으나, ParserDiagnostics 클래스 중복(2군데)과 MetricCard 중복(3군데)이 응집도를 낮춤 |
| 확장성(OCP) | 4/5 | AnalysisResult 공통 컨트랙트, TypedDict 컨트랙트, Extension Model 문서화로 새 분석기 추가 경로가 명확 |
| 데이터 모델링 | 4/5 | Frozen dataclass + TypedDict 이중 계약, schema_version 예약, 단위 명시 규칙 등 우수 |
| API 설계 | 4/5 | IPC 채널 명명이 일관(`analyzer:{type}:{action}`), Request/Response 타입이 명시적. 버저닝은 미도입 |
| 트랜잭션/일관성 | 3/5 | 단일 파일 분석이므로 트랜잭션 이슈는 적으나, 임시 파일 정리가 `finally`로 처리되어 안전 |
| 장애 격리 | 4/5 | Python 엔진 크래시가 UI를 다운시키지 않음. timeout 60s, maxBuffer 4MB로 방어. BridgeError 구조 일관 |
| 관측성 | 2/5 | 구조화된 로깅 없음. Parser diagnostics는 우수하나 런타임 로깅/메트릭/트레이싱 전략 부재 |
| 보안 | 3/5 | contextIsolation + execFile(shell 회피) 우수. 단 Electron 31 EOL, CSP 미설정, 입력 경로 검증 제한적 |
| 확장성(Scalability) | 3/5 | max_lines + 시간 필터로 기초 대응. 스트리밍 집계 설계 있으나, percentile은 아직 전체 메모리 적재 |
| 배포/운영 | 2/5 | PyInstaller/electron-builder 패키징 미구현. CI/CD 파이프라인 없음. 환경 분리 없음 |

### 2.1 강점 (Architectural Strengths)

**S-1. AnalysisResult 공통 컨트랙트의 일관성**

`engines/python/archscope_engine/models/analysis_result.py:9-22`의 `AnalysisResult` dataclass와 `apps/desktop/src/api/analyzerContract.ts:9-25`의 제네릭 `AnalysisResult<T>` 타입이 정확히 동일한 필드 구조를 공유한다. `type`, `source_files`, `summary`, `series`, `tables`, `charts`, `metadata`, `created_at` — 이 공통 셸 위에 `TypedDict`/TypeScript 타입으로 분석기별 컨트랙트를 구체화한 설계는 교과서적으로 올바르다. Phase 2 이후 GC, Thread Dump 등 새 분석기를 추가할 때 UI 레이어 수정 없이 컨트랙트만 확장하면 되는 구조다.

**S-2. Electron Bridge의 보안 설계**

`electron/main.ts:44-47`에서 `contextIsolation: true`, `nodeIntegration: false`를 유지하고, `preload.ts`에서 `contextBridge.exposeInMainWorld`로 최소한의 API만 노출한다. Python 호출 시 `execFile`을 사용하여 shell interpolation을 원천 차단하며(`main.ts:236`), 임시 출력 파일을 UUID 경로에 쓰고 `finally`에서 삭제하는 패턴(`main.ts:192-228`)이 안전하다.

**S-3. Parser Diagnostics 설계**

파서가 맬포밍된 레코드를 만났을 때 전체 분석을 중단하지 않고 건너뛰면서, `skipped_by_reason` 맵과 `samples` 배열(상한 20건, preview 200자)로 진단 정보를 수집하는 정책이 운영 로그 분석 도구의 실전 요구사항에 정확히 부합한다. reason code(`NO_FORMAT_MATCH`, `INVALID_TIMESTAMP` 등)가 안정적으로 정의되어 있어 UI나 자동화 파이프라인에서 프로그래밍적으로 분기할 수 있다.

**S-4. i18n이 처음부터 내장**

`messages.ts`에서 영문/한국어 97개 키가 동시에 관리되고, `I18nProvider` 컨텍스트를 통해 모든 UI 레이블이 locale-aware다. 차트 축 레이블까지 i18n 대상에 포함시킨 것은 보고서 용도의 전문 도구에서 중요한 결정이다.

**S-5. 스트리밍 집계로의 점진적 전환**

`access_log_analyzer.py:50-64`에서 `iter_access_log_records_with_diagnostics`를 이터레이터로 소비하며 `Counter`, `defaultdict`로 증분 집계하는 구조는 메모리 제한 환경에서도 동작 가능한 올바른 방향이다.

### 2.2 우려 사항 (Architectural Concerns)

**C-1. ParserDiagnostics 클래스 중복** [Severity: High]

- **현상**: `access_log_parser.py:28-65`와 `collapsed_parser.py:16-53`에 동일한 `ParserDiagnostics` 클래스가 독립적으로 정의되어 있다. 필드 이름, `add_skipped` 메서드, `to_dict` 메서드, 상수(`MAX_DIAGNOSTIC_SAMPLES=20`, `RAW_PREVIEW_LIMIT=200`)까지 문자 단위로 동일하다.
- **영향**: Phase 3에서 GC, Thread Dump, Exception 파서를 구현할 때 동일한 클래스를 또 복사하거나, 한쪽만 수정하고 다른 쪽은 놓쳐서 진단 동작이 불일치하는 리스크가 크다.
- **권고안**: `common/diagnostics.py` 모듈로 통합하라.

```python
# archscope_engine/common/diagnostics.py
from __future__ import annotations
from dataclasses import dataclass, field
from typing import Any

MAX_DIAGNOSTIC_SAMPLES = 20
RAW_PREVIEW_LIMIT = 200

@dataclass
class ParserDiagnostics:
    total_lines: int = 0
    parsed_records: int = 0
    skipped_lines: int = 0
    skipped_by_reason: dict[str, int] = field(default_factory=dict)
    samples: list[dict[str, int | str]] = field(default_factory=list)

    def add_skipped(self, *, line_number: int, reason: str, message: str, raw_line: str) -> None:
        self.skipped_lines += 1
        self.skipped_by_reason[reason] = self.skipped_by_reason.get(reason, 0) + 1
        if len(self.samples) < MAX_DIAGNOSTIC_SAMPLES:
            self.samples.append({
                "line_number": line_number,
                "reason": reason,
                "message": message,
                "raw_preview": raw_line[:RAW_PREVIEW_LIMIT],
            })

    def to_dict(self) -> dict[str, Any]:
        return {
            "total_lines": self.total_lines,
            "parsed_records": self.parsed_records,
            "skipped_lines": self.skipped_lines,
            "skipped_by_reason": dict(self.skipped_by_reason),
            "samples": list(self.samples),
        }
```

- **예상 공수**: 0.5일

**C-2. Electron 31 EOL** [Severity: Critical]

- **현상**: `package.json`에서 `"electron": "^31.1.0"`. Electron 31은 2025-03-18에 지원 종료되었다.
- **영향**: Chromium 보안 패치가 중단된 상태에서 로컬 파일을 열고 분석하는 데스크탑 앱을 배포하는 것은 보안 관점에서 용납되기 어렵다. 특히 `file://` 프로토콜 기반 데스크탑 앱에서 Chromium 취약점은 로컬 파일 시스템 접근 권한 상승으로 이어질 수 있다.
- **권고안**: Electron 33 이상으로 업그레이드. Electron 33은 Chromium 130 기반으로 2026년 4월 현재 지원 중이다. `work_status.md` T-023에 이미 계획이 있으나 Phase 2 진입 전으로 앞당겨야 한다.
- **예상 공수**: 1~2일 (주로 호환성 확인)

**C-3. MetricCard 컴포넌트 3중 정의** [Severity: Medium]

- **현상**: `MetricCard` 함수 컴포넌트가 `DashboardPage.tsx:81-94`, `AccessLogAnalyzerPage.tsx:257-270`, `ProfilerAnalyzerPage.tsx:234-247`에 각각 독립 정의되어 있다. 세 곳의 구현이 거의 동일하지만, `DashboardPage`의 `value` prop은 `string | number`이고 나머지는 `string`이다.
- **영향**: Phase 2에서 차트 스튜디오나 새 분석기 페이지를 추가할 때마다 또 복사하게 되고, 스타일 변경 시 모든 곳을 수정해야 한다.
- **권고안**: `components/MetricCard.tsx`로 추출. `value` prop은 `string | number`로 통일.
- **예상 공수**: 0.5일

**C-4. Analyzer 테스트가 파서 테스트에 결합** [Severity: Medium]

- **현상**: `test_access_log_parser.py`에서 `analyze_access_log`(분석기)를 직접 호출하여 분석 결과를 검증한다. 파서 테스트 파일이 분석기의 정확성도 함께 검증하는 구조다. 순수한 파서 단위 테스트는 `test_parse_nginx_access_line_with_response_time`과 `test_parse_access_log_reports_malformed_line_diagnostics` 2건뿐이다.
- **영향**: 분석기 로직(percentile 계산, finding 생성, 스트리밍 집계 등)에 버그가 생겨도 어느 계층이 원인인지 분리하기 어렵다. Phase 2에서 분석 로직이 복잡해지면 이 문제가 커진다.
- **권고안**: `tests/test_access_log_analyzer.py`를 별도로 만들고, 분석기 테스트에서는 파서를 목(mock)하거나 인메모리 레코드 리스트를 직접 주입하라. `build_access_log_result`가 이미 `Iterable[AccessLogRecord]`를 받으므로 이 분리는 자연스럽다.
- **예상 공수**: 1일

**C-5. 프로파일러 스택 분류 하드코딩** [Severity: Low]

- **현상**: `profiler_analyzer.py:149-159`의 `_classify_stack` 함수가 `"oracle.jdbc"`, `"socket"`, `"http"`, `"springframework"` 등 문자열 리터럴로 분류한다.
- **영향**: Node.js, Python, Go 등 다른 런타임 스택을 분석할 때 이 함수를 확장하기 어렵고, 사용자별 커스텀 분류가 불가능하다.
- **권고안**: `work_status.md` T-026/T-027에 이미 계획되어 있으며, Phase 3에서 구성 파일 기반 분류로 전환하는 것이 적절하다. Phase 2 진입 전에 처리할 필요는 없다.
- **예상 공수**: 2~3일

---

## 3. 코드 리뷰 결과

### 3.1 잘 작성된 부분 (Exemplary Code)

**E-1. `_parse_nginx_access_line`의 에러 분류 패턴** (`access_log_parser.py:158-201`)

파서 함수가 `(record | None, error | None)` 튜플을 반환하는 Go-style 에러 패턴은 이 컨텍스트에서 적절하다. 정규표현식 매치 실패, 타임스탬프 파싱 실패, 숫자 변환 실패, 값 범위 검증 실패 각각에 대해 명확한 reason code를 부여하며, 호출자가 diagnostics 수집과 제어 흐름을 분리할 수 있다. `isfinite` 검사까지 포함한 점이 실전적이다.

**E-2. Bridge의 `runAnalyzer<T>` 제네릭 패턴** (`main.ts:189-229`)

임시 디렉터리 생성 → 엔진 실행 → JSON 읽기 → 구조 검증 → 타입 안전 반환 → finally 정리라는 흐름이 한 함수에 깔끔하게 들어있다. 제네릭 `T extends AnalysisResult`로 호출 시점에 타입을 좁히면서도 런타임 `isAnalysisResult` 가드를 통과시킨다. 구조 검증이 아직 얕지만(`type`, `source_files`, `summary`, `series` 존재 여부만 확인), 초기 PoC 단계에서는 충분하다.

**E-3. `iter_text_lines`의 인코딩 감지 전략** (`file_utils.py:7-32`)

파일을 먼저 한 번 전체 읽어서 인코딩을 확정한 뒤, 확정된 인코딩으로 다시 읽는 2-pass 접근은 중간에 인코딩이 틀어져서 라인이 중복되는 문제(`3aefe6e` 커밋에서 수정)를 근본적으로 방지한다. `latin-1`이 fallback chain 끝에 있어 모든 바이트 시퀀스를 수용하면서도, 그 결과로 의미적 파싱이 실패하면 파서 diagnostics에서 잡히는 방어적 설계가 좋다.

**E-4. Analyzer의 finding 생성 로직** (`access_log_analyzer.py:226-284`)

`_build_access_log_findings`가 summary와 series 데이터로부터 `HIGH_ERROR_RATE`, `ELEVATED_ERROR_RATE`, `SERVER_ERRORS_PRESENT`, `SLOW_URL_AVERAGE` 등 구조화된 finding을 생성한다. 각 finding에 `severity`, `code`, `message`, `evidence`(수치 증거)가 포함되어, UI나 보고서 자동 생성에서 프로그래밍적으로 활용 가능하다. 단순한 로그 뷰어가 아닌 "Architecture Evidence Builder"라는 제품 정체성에 부합하는 기능이다.

**E-5. `analyzerContract.ts`의 타입 설계** (`analyzerContract.ts:1-242`)

Python `TypedDict`와 1:1로 대응하는 TypeScript 타입들이 한 파일에 체계적으로 정리되어 있다. `AnalysisResult` 제네릭의 6개 타입 파라미터(`TType`, `TSummary`, `TSeries`, `TTables`, `TCharts`, `TMetadata`)는 각 분석기의 구체 타입을 정밀하게 표현하면서도 공통 결과 구조를 강제한다. `AnalyzerResponse<T>`의 태그드 유니언(`ok: true | ok: false`)은 UI에서 분기 처리를 타입 안전하게 만든다.

### 3.2 개선 필요 항목

#### Issue #1: ParserDiagnostics 중복 정의 [Severity: High]

- **위치**: `engines/python/archscope_engine/parsers/access_log_parser.py:23-65`, `engines/python/archscope_engine/parsers/collapsed_parser.py:11-53`
- **현상**: 두 파서 모듈에 완전히 동일한 `ParserDiagnostics` 클래스, `MAX_DIAGNOSTIC_SAMPLES`, `RAW_PREVIEW_LIMIT`, `ParseError` type alias가 각각 정의되어 있다.
- **문제점**: DRY 원칙 위반. GC, Thread Dump, Exception 파서 구현 시 복사 횟수가 늘어나고, 한쪽 수정이 다른 쪽에 반영되지 않는 일관성 리스크가 발생한다. `access_log_analyzer.py:22`에서 `from archscope_engine.parsers.access_log_parser import ParserDiagnostics`로 임포트하는데, 이 임포트 경로는 collapsed 파서에서 같은 클래스를 사용하려 할 때 혼란을 야기한다.
- **권고 개선안**: C-1 항목 참조 — `common/diagnostics.py`로 통합
- **예상 공수**: 0.5일

#### Issue #2: Analyzer 테스트와 Parser 테스트의 비분리 [Severity: Medium]

- **위치**: `engines/python/tests/test_access_log_parser.py:36-43`, `test_access_log_parser.py:106-119`, `test_access_log_parser.py:122-139` 등
- **현상**: 파서 테스트 파일(`test_access_log_parser.py`)에서 `analyze_access_log` 함수를 직접 호출하여 `result.summary`, `result.metadata.diagnostics`, `result.metadata.findings` 등 분석기 출력을 검증한다.
- **문제점**: 테스트 대상이 섞여 있어, 파서 변경인지 분석기 변경인지에 따른 테스트 실패 원인 추적이 어렵다. 분석기의 스트리밍 집계 로직, percentile 계산, finding 규칙 등은 파서 정확성과 독립적으로 검증되어야 한다.
- **권고 개선안**: `tests/test_access_log_analyzer.py`를 신설하고, `build_access_log_result`에 인메모리 `AccessLogRecord` 리스트를 주입하는 순수 분석기 테스트를 작성하라.

```python
# tests/test_access_log_analyzer.py
from datetime import datetime, timezone
from archscope_engine.models.access_log import AccessLogRecord
from archscope_engine.analyzers.access_log_analyzer import build_access_log_result
from pathlib import Path

def make_record(uri="/api", status=200, response_time_ms=100.0) -> AccessLogRecord:
    return AccessLogRecord(
        timestamp=datetime(2026, 4, 27, 10, 0, 0, tzinfo=timezone.utc),
        method="GET", uri=uri, status=status, response_time_ms=response_time_ms,
        bytes_sent=1234, client_ip="127.0.0.1", user_agent="test", referer="-", raw_line="",
    )

def test_build_result_calculates_error_rate():
    records = [make_record(status=200), make_record(status=500)]
    result = build_access_log_result(records, source_file=Path("test.log"), log_format="nginx")
    assert result.summary["error_rate"] == 50.0
```

- **예상 공수**: 1일

#### Issue #3: `response_times` 리스트의 메모리 무제한 증가 [Severity: Medium]

- **위치**: `engines/python/archscope_engine/analyzers/access_log_analyzer.py:74`, `87`
- **현상**: `response_times: list[float] = []`에 모든 레코드의 응답 시간을 append하며, 이 리스트는 `percentile()` 계산에 사용된다. 분당 응답 시간 리스트(`response_times_by_minute`)도 동일하게 전체 보존된다.
- **문제점**: `max_lines` 없이 100만 건 이상의 로그를 분석하면, `response_times` 리스트만으로 ~8MB(float 8byte × 100만)를 차지한다. `PARSER_DESIGN.md:191`에서 "Exact percentile calculation may still keep response-time sample arrays in Phase 1B"라고 명시적으로 인지하고 있으나, Phase 2에서 대형 파일 분석이 일상화되면 이슈가 된다.
- **권고 개선안**: Phase 2 중기에 t-digest 또는 DDSketch 기반 approximate percentile로 전환. 당장은 `max_lines` 기본값 설정(예: 500,000)으로 방어 가능하다.
- **예상 공수**: 2~3일 (근사 percentile 라이브러리 도입 시)

#### Issue #4: `_classify_stack`에 `lower()` 반복 호출 [Severity: Low]

- **위치**: `engines/python/archscope_engine/analyzers/profiler_analyzer.py:149-159`
- **현상**: `_classify_stack` 함수가 호출될 때마다 `stack.lower()`를 실행한다. 이 함수는 `_component_breakdown`에서 전체 스택에 대해 호출되고(`profiler_analyzer.py:141`), `_to_profile_stack`에서 top_n 스택에 대해 또 호출된다(`profiler_analyzer.py:136`). 즉, 같은 스택에 대해 `lower()`가 2회씩 호출된다.
- **현상**: 성능상 의미 있는 영향은 아니지만, 분류 로직이 확장되면 비효율이 커진다.
- **권고 개선안**: top_n 스택의 category를 `_to_profile_stack`에서 할당할 때 `_component_breakdown` 결과를 재사용하거나, category를 `ProfileStack` 생성 시 한 번만 계산하라. 다만 Phase 2에서 T-026(분류 외부화)이 구현되면 자연스럽게 해결되므로, 단독 수정의 우선순위는 낮다.
- **예상 공수**: 0.5일

#### Issue #5: CSP(Content Security Policy) 미설정 [Severity: High]

- **위치**: `electron/main.ts:34-47`의 `createWindow`
- **현상**: BrowserWindow 생성 시 CSP 헤더가 설정되지 않았다. Electron 앱에서 CSP가 없으면, 렌더러 프로세스에서 inline script injection이나 외부 스크립트 로딩이 가능하다.
- **문제점**: `contextIsolation: true`로 Node API 직접 접근은 차단되어 있으나, XSS 공격 벡터(예: 분석 결과의 `raw_line`이 HTML로 렌더링될 경우)를 통해 렌더러 프로세스 내에서 악의적 코드가 실행될 수 있다.
- **권고 개선안**: `session.defaultSession.webRequest.onHeadersReceived`로 CSP를 주입하거나, `index.html`의 `<meta>` 태그에 CSP를 설정하라.

```typescript
// main.ts — createWindow 내부에 추가
mainWindow.webContents.session.webRequest.onHeadersReceived((details, callback) => {
  callback({
    responseHeaders: {
      ...details.responseHeaders,
      "Content-Security-Policy": [
        "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;"
      ],
    },
  });
});
```

- **예상 공수**: 0.5일

#### Issue #6: `isAnalysisResult` 검증이 얕음 [Severity: Low]

- **위치**: `electron/main.ts:303-315`
- **현상**: `isAnalysisResult`가 `type`, `source_files`, `summary`, `series` 4개 필드의 존재 여부와 대략적 타입만 확인한다. `tables`, `metadata`, `created_at`은 검증하지 않는다.
- **문제점**: Python 엔진이 불완전한 JSON을 출력했을 때(예: `metadata` 누락) UI에서 런타임 에러가 발생할 수 있다. Phase 1 PoC에서는 허용 범위지만, Phase 2에서 더 많은 분석기가 추가되면 방어가 필요하다.
- **권고 개선안**: Phase 2에서 Zod 등 런타임 스키마 검증 라이브러리 도입을 고려. 당장은 `metadata`와 `tables` 존재 확인을 추가하는 것으로 충분하다.
- **예상 공수**: 0.5일 (수동 가드 보강) / 1~2일 (Zod 도입)

#### Issue #7: `App.tsx`의 조건부 렌더링 체인 [Severity: Low]

- **위치**: `apps/desktop/src/App.tsx:28-39`
- **현상**: 9개 페이지를 `{activePage === "xxx" && <XxxPage />}` 패턴으로 나열한다. `work_status.md` T-018에서 이미 매핑 테이블 전환이 계획되어 있다.
- **문제점**: 페이지가 추가될 때마다 `App.tsx`와 `PageKey` 타입을 모두 수정해야 한다. 실수로 매핑을 빠뜨리면 빈 화면이 되고 에러는 발생하지 않는다.
- **권고 개선안**: T-018 계획대로 `Record<PageKey, React.ComponentType>` 맵으로 전환하라.

```tsx
const pageComponents: Record<PageKey, React.ComponentType> = {
  dashboard: DashboardPage,
  "access-log": AccessLogAnalyzerPage,
  // ...
};

export function App(): JSX.Element {
  const [activePage, setActivePage] = useState<PageKey>("dashboard");
  const PageComponent = pageComponents[activePage];
  return (
    <Layout activePage={activePage} onNavigate={setActivePage}>
      <PageComponent />
    </Layout>
  );
}
```

- **예상 공수**: 0.5일

### 3.3 테스트 코드 품질 평가

**테스트 현황**: 23개 테스트, 7개 테스트 파일, 전수 통과 (0.04s)

| 영역 | 테스트 수 | 커버리지 평가 |
|---|---|---|
| Access log parser | 3 | 정상 파싱, malformed 진단, 분석기 연동 — 양호 |
| Access log analyzer | 4 | max_lines, time filter, findings — 파서 테스트에 혼재 |
| Collapsed parser | 2 | 정상 파싱, malformed 진단 — 양호 |
| Collapsed analyzer | 2 | 기본 분석, 진단 메타데이터 — 최소 |
| File utils | 2 | encoding fallback, utf-8 — 핵심 경로 커버 |
| Statistics | 5 | empty, single, repeated, negative, interpolation — 우수 |
| JSON exporter | 2 | round-trip, dict — 양호 |
| GC log parser | 1 | placeholder NotImplementedError — 최소 |
| Packaging metadata | 2 | dependencies, entry point — 독창적이고 유용 |

**강점**:
- `test_statistics.py`의 엣지 케이스 테스트(empty, single, repeated, negative, percentile interpolation)는 통계 유틸리티의 정확성을 잘 검증한다.
- `test_packaging_metadata.py`에서 `setup.cfg`의 `install_requires`와 `console_scripts`를 프로그래밍적으로 검증하는 것은 패키징 회귀를 방지하는 영리한 접근이다.
- `test_access_log_parser.py`의 `nginx_line()` 헬퍼는 테스트 데이터 생성을 깔끔하게 캡슐화한다.

**개선 필요**:
- **분석기 독립 테스트 부재**: Issue #2에서 상술. `build_access_log_result`와 `build_collapsed_result`를 직접 테스트하는 케이스가 없다.
- **UI/프론트엔드 테스트 전무**: React 컴포넌트, i18n, 차트 옵션 빌더에 대한 테스트가 없다. Phase 2에서 UI가 확장되면 회귀 리스크가 커진다.
- **CLI 통합 테스트 부재**: `archscope-engine access-log analyze --file ... --out ...` 명령의 end-to-end 테스트가 없다. Bridge PoC가 이 경로에 의존하므로 보호가 필요하다.
- **TODO/FIXME**: 코드 전체에서 TODO/FIXME 주석이 0건 — 좋은 신호이나, Phase 1에서 인식된 한계(approximate percentile, 패키징 등)가 코드가 아닌 문서에서만 추적되고 있다.

---

## 4. 보안 및 운영 관점 점검

### 4.1 보안 취약 가능 지점

| # | 항목 | 현상 | 심각도 | 권고 |
|---|---|---|---|---|
| SEC-1 | Electron 31 EOL | Chromium 보안 패치 미수신 | Critical | v33+ 업그레이드 (Issue #2) |
| SEC-2 | CSP 미설정 | 렌더러에서 inline script 실행 가능 | High | CSP 헤더 주입 (Issue #5) |
| SEC-3 | `raw_preview` 렌더링 | `AnalyzerFeedback.tsx:71`에서 `sample.raw_preview`를 JSX에 직접 삽입 — React가 자동 이스케이프하므로 현재는 안전하지만, `dangerouslySetInnerHTML`로 전환하면 XSS 벡터가 됨 | Low | React 기본 이스케이프를 유지. 절대 `dangerouslySetInnerHTML` 사용 금지를 코딩 가이드에 명시 |
| SEC-4 | `execFile` 인자 검증 | `main.ts:142-154`에서 `request.filePath`를 인자 배열에 직접 전달. `execFile`이므로 shell injection은 불가하나, 심볼릭 링크나 `../` traversal로 의도치 않은 파일 접근 가능 | Low | 파일 경로를 `path.resolve` 후 허용 디렉터리 내 존재 여부 검증 추가 고려 |

### 4.2 관측성 (로깅/메트릭/트레이싱) 준비도

- **구조화된 로깅**: 없음. Python 엔진에서 `rich.console.Console`을 stdout에 사용하고 있으나, 이는 사용자 대면 메시지용이지 운영 로깅이 아니다. Electron main 프로세스에도 구조화된 로그가 없다.
- **메트릭**: 없음. 분석 소요 시간, 파일 크기, 레코드 수 등의 성능 메트릭이 수집/기록되지 않는다.
- **트레이싱**: 없음. Bridge 호출의 요청-응답 추적이 불가하다.
- **Parser diagnostics**: 우수. 이것이 현재 유일한 관측성 채널이다.
- **권고**: Phase 2에서 Python `logging` 모듈을 JSON 포맷으로 구성하고, Electron main 프로세스에 `electron-log` 도입을 고려하라. 데스크탑 앱이므로 외부 메트릭 시스템은 불필요하지만, 로컬 로그 파일은 디버깅에 필수적이다.

### 4.3 배포 및 운영 자동화 수준

- **CI/CD**: 없음. GitHub Actions 등의 CI가 설정되지 않아 PR마다 테스트/빌드가 자동으로 실행되지 않는다.
- **패키징**: 미구현. Electron은 `vite build` + `tsc` 빌드까지, Python은 `pip install -e .` 개발 설치까지만 지원. `electron-builder`, `PyInstaller` 모두 미도입.
- **환경 분리**: `ARCHSCOPE_ENGINE_COMMAND` 환경 변수로 엔진 경로 오버라이드가 가능하나, 체계적인 환경 설정(dev/staging/prod) 분리는 없다.
- **권고**: Phase 2 초기에 GitHub Actions CI를 추가하여 `pytest` 실행, TypeScript 타입 체크, 린팅을 자동화하라. 패키징은 T-022(Phase 3) 계획대로 유지하되, CI는 앞당길 가치가 있다.

---

## 5. Phase 2 진입 적합성 평가

### 5.1 ✅ 그대로 가도 되는 영역

| 영역 | 근거 |
|---|---|
| AnalysisResult 공통 컨트랙트 | Python/TypeScript 양쪽에서 정렬 완료. 새 분석기 추가 시 extension model이 동작함 |
| Electron IPC Bridge 아키텍처 | `execFile` + temp JSON + 구조 검증 패턴이 안정적. Phase 2 분석기에 동일 패턴 적용 가능 |
| Parser → Analyzer → Exporter 계층 분리 | 모듈 경계가 명확하고 Extension Model 문서화 됨 |
| i18n 시스템 | 영문/한국어 97개 키가 동시 관리되고, 차트 축까지 locale-aware |
| Python 의존성 경량성 | typer + rich 2개만 — 새 분석기 추가 시 의존성 증가가 최소화됨 |
| 설계 문서 품질 | ARCHITECTURE, DATA_MODEL, PARSER_DESIGN, CHART_DESIGN, ROADMAP, BRIDGE_POC_UX_FLOW — 각각이 결정 사항과 근거를 명시적으로 기록하고 있어 Phase 2 개발자 온보딩에 유용 |
| 스트리밍 집계 기반 | `iter_access_log_records_with_diagnostics` → `Counter`/`defaultdict` 증분 집계 패턴이 새 분석기에도 적용 가능 |

### 5.2 ⚠️ Phase 2 시작 전 반드시 처리할 항목 (Stop-the-line)

| # | 항목 | 근거 | 예상 공수 |
|---|---|---|---|
| STL-1 | Electron 33+ 업그레이드 | EOL 플랫폼 위에서 개발을 진행하면 Phase 2 중반에 업그레이드 시 호환성 충돌 범위가 커진다. 지금 올려야 Phase 2 코드가 지원 버전 위에서 검증된다. | 1~2일 |
| STL-2 | CSP 설정 | Phase 2에서 동적 분석 결과를 더 많이 렌더링하게 되면 XSS 공격 표면이 넓어진다. 보안 기본 설정은 기능 개발 전에 확보해야 한다. | 0.5일 |

### 5.3 🔄 병행 개선 가능 항목

| # | 항목 | 시점 | 예상 공수 |
|---|---|---|---|
| PAR-1 | ParserDiagnostics 통합 | Phase 2 초기, GC 파서 구현 전에 | 0.5일 |
| PAR-2 | MetricCard 컴포넌트 추출 | Phase 2 초기, 새 페이지 추가 전에 | 0.5일 |
| PAR-3 | App.tsx 페이지 매핑 테이블 전환 (T-018) | Phase 2 초기 | 0.5일 |
| PAR-4 | Analyzer 독립 테스트 신설 | Phase 2 초기, 새 분석 로직 추가 전에 | 1일 |
| PAR-5 | GitHub Actions CI 추가 | Phase 2 초기 | 0.5~1일 |
| PAR-6 | Python 구조화 로깅 도입 | Phase 2 중기, 디버깅 필요 시점에 | 1일 |

---

## 6. 권고 로드맵

| 우선순위 | 항목 | 예상 공수 | 시점 |
|---|---|---|---|
| P0 | Electron 33+ 업그레이드 (STL-1) | 1~2일 | Phase 2 진입 전 |
| P0 | CSP 설정 (STL-2) | 0.5일 | Phase 2 진입 전 |
| P1 | ParserDiagnostics 통합 (PAR-1) | 0.5일 | Phase 2 첫 주 |
| P1 | MetricCard 추출 (PAR-2) | 0.5일 | Phase 2 첫 주 |
| P1 | App.tsx 매핑 테이블 (PAR-3, T-018) | 0.5일 | Phase 2 첫 주 |
| P1 | Analyzer 독립 테스트 (PAR-4) | 1일 | Phase 2 첫 주 |
| P1 | GitHub Actions CI (PAR-5) | 0.5~1일 | Phase 2 첫 주 |
| P2 | 구조화 로깅 도입 (PAR-6) | 1일 | Phase 2 중기 |
| P2 | `isAnalysisResult` 검증 강화 또는 Zod 도입 | 0.5~2일 | Phase 2 중기 |
| P2 | Approximate percentile 전환 | 2~3일 | Phase 2 중기 |
| P3 | 프로파일러 분류 외부화 (T-026/T-027) | 2~3일 | Phase 3 |

---

## 7. 참고: 외부 리서치 및 레퍼런스

| 항목 | 자료 |
|---|---|
| Electron 릴리즈 타임라인 | Electron Releases (https://www.electronjs.org/docs/latest/tutorial/electron-timelines) — Electron 31 EOL: 2025-03-18 |
| CSP in Electron | Electron Security Checklist (https://www.electronjs.org/docs/latest/tutorial/security#6-define-a-content-security-policy) |
| t-digest approximate percentile | Dunning & Ertl, "Computing Extremely Accurate Quantiles Using t-Digests" (https://github.com/tdunning/t-digest) |
| Python TypedDict vs Pydantic | PEP 589 (https://peps.python.org/pep-0589/) — TypedDict as lightweight structural contract |
| Electron contextIsolation 보안 | Electron Security Best Practices (https://www.electronjs.org/docs/latest/tutorial/context-isolation) |

---

## 8. 검토자 코멘트 (자유 서술)

이 프로젝트는 Phase 1 단계임에도 불구하고, 아키텍처 결정의 근거를 문서로 남기고(`review_decisions.md`, `research_decisions.md`), 결정에 기반하여 실행 백로그를 구성하는 규율이 인상적이다. 많은 프로젝트가 "일단 만들고 나중에 문서화하자"로 시작하여 결국 문서 없이 성장하는 것과 대조된다.

특히 두 가지가 기억에 남는다.

첫째, **AnalysisResult 컨트랙트를 "데이터가 한 방향으로만 흐르는 파이프라인의 중간 표현(IR)"으로 설계한 결정**이다. UI가 파서를 모르고, 파서가 차트를 모르며, 모든 것이 `{summary, series, tables, charts, metadata}` 구조를 통해서만 교류한다. 이 설계는 앞으로 분석기가 5개, 10개로 늘어나도 각 모듈을 독립적으로 개발·테스트할 수 있는 핵심 기반이다.

둘째, **Parser Diagnostics 정책**이다. "한 줄이 깨졌다고 100만 줄 분석을 포기하지 않는다"는 정책은 운영 데이터를 다루는 도구에서 가장 중요한 결정 중 하나다. 이 정책이 문서화되고(`PARSER_DESIGN.md`), reason code가 안정적으로 정의되고, 진단 샘플이 bounded(20건, 200자)로 유지되는 것까지 — 실전 운영 경험이 반영된 설계다.

한 가지 당부: Phase 2로 가면서 가장 경계해야 할 것은 **컨트랙트의 침식**이다. 지금은 `access_log`과 `profiler_collapsed` 2개의 분석기만 있어서 TypedDict ↔ TypeScript 타입 정렬이 수작업으로 가능하다. 분석기가 5개를 넘어가면 Python과 TypeScript 간 컨트랙트 드리프트가 발생하기 시작한다. Phase 2 중반 이전에 JSON Schema 또는 코드 생성 기반의 단일 소스(single source of truth) 컨트랙트 관리 방안을 검토할 것을 권한다.

전체적으로, Phase 2로 진행할 수 있는 기반은 충분히 갖추어져 있다. Stop-the-line 2건(Electron 업그레이드, CSP)을 먼저 처리하고, 첫 주에 코드 위생(ParserDiagnostics 통합, MetricCard 추출, CI 추가)을 정리한 뒤 본격적인 기능 개발에 들어가기를 권한다.
