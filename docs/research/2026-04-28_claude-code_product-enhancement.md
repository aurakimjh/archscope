# 제품 강화 리서치 보고서

- **작성일**: 2026-04-28
- **작성자**: Claude Code
- **대상 프로젝트**: ArchScope — Application Architecture Diagnostic & Reporting Toolkit
- **분석 범위**: `main` 브랜치, 커밋 `2a3d457`

---

## 1. Executive Summary

ArchScope는 미들웨어 로그, GC 로그, 프로파일러 출력, 스레드 덤프, 예외 스택 트레이스를 파싱하여 아키텍처 진단 보고서를 생성하는 데스크톱 도구이다. 현재 Phase 1 Foundation 단계로, Access Log 파서와 Collapsed Profiler 파서만 동작하며 Engine-UI 브리지는 미구현 상태이다.

핵심 발견사항:

1. **Electron 31은 지원 종료 버전**이다. 최신 안정 버전(41.x)과 10개 메이저 버전 차이가 있으며, 알려진 보안 패치가 적용되지 않은 상태이다.
2. **Engine-UI 브리지 미구현**이 프로젝트 진행의 최대 병목이다. 모든 UI는 현재 하드코딩된 샘플 데이터를 렌더링하며, 실제 Python 엔진과 연동되지 않는다.
3. **테스트 커버리지가 최소 수준**이다. 파서 2개에 대한 기본 테스트 3개만 존재하며, 분석기·내보내기·유틸리티 테스트가 부재하다.
4. **Python 패키징이 레거시 방식**(setuptools < 64 + setup.py)을 사용하고 있어 현대적 도구 체인으로 마이그레이션이 필요하다.
5. **ECharts 6.0의 다크 모드, 신규 차트 타입, SVG SSR 기능**은 보고서 품질을 즉시 향상시킬 수 있는 저비용 개선점이다.

---

## 2. 프로젝트 현황

### 2.1 도메인 및 목적

ArchScope는 애플리케이션 아키텍트를 대상으로, 운영/성능 원시 데이터를 아키텍처 진단 증거(Architecture Evidence)로 변환하는 도구이다. 단순한 로그 뷰어가 아니라 **Architecture Evidence Builder**를 지향한다.

진단 흐름:
```
Raw Data → Parsing → Analysis / Aggregation → Visualization → Report-ready Export
```

초기 모듈: Access Log Analyzer, GC Log Analyzer, Profiler Analyzer, Thread Dump Analyzer, Exception Analyzer, Chart Studio, Export Center

### 2.2 기술 스택 및 버전

| 계층 | 기술 | 버전 | 비고 |
|------|------|------|------|
| Desktop Shell | Electron | 31.1.0 | **지원 종료 버전** |
| UI Framework | React | 18.2.0 | 안정적이나 19.x 출시됨 |
| Language | TypeScript | 5.5.3 | 현재 안정 |
| Build Tool | Vite | 5.3.3 | 현재 안정 |
| Chart Library | Apache ECharts | 5.5.1 | 6.0 출시됨 (2025-07) |
| Analysis Engine | Python | 3.x (미명시) | typer + rich CLI |
| Build Backend | setuptools | < 64 (제약됨) | 레거시 방식 |
| Packaging (계획) | electron-builder + PyInstaller | 미구현 | |

**Desktop 의존성** (`apps/desktop/package.json`):
- `@vitejs/plugin-react: ^4.2.1`
- `echarts: ^5.5.1`
- `react: ^18.2.0`, `react-dom: ^18.2.0`
- `electron: ^31.1.0`
- `concurrently: ^8.2.2`, `wait-on: ^7.2.0`

**Python 의존성** (`engines/python/pyproject.toml`):
- `setuptools < 64` (빌드 시스템 제약)
- 런타임 의존성은 setup.py에 별도 정의 없음 (typer, rich는 코드에서 import하나 명시적 선언 미확인)

### 2.3 아키텍처 개요

```
archscope/
├── apps/desktop/          # Electron + React + ECharts (UI skeleton)
│   ├── electron/          # main.ts, preload.ts (최소 구현)
│   └── src/               # React 페이지, 차트, i18n, API 클라이언트
├── engines/python/        # Python 파서 + 분석 엔진
│   └── archscope_engine/
│       ├── parsers/       # access_log, collapsed, gc_log(stub), thread_dump(stub), exception(stub)
│       ├── analyzers/     # access_log_analyzer, profiler_analyzer, gc/thread/exception(stub)
│       ├── models/        # AnalysisResult, AccessLogRecord, GcEvent, ProfileStack, ThreadDumpRecord
│       ├── exporters/     # json, csv(stub), html(stub)
│       └── common/        # file_utils, time_utils, statistics
├── docs/                  # en/ko 이중 언어 설계 문서
├── examples/              # 샘플 입력 파일 및 출력 JSON
└── scripts/               # 개발 도우미 스크립트
```

**Engine-UI 브리지 설계** (문서화 완료, 미구현):
```
React Renderer → preload API → Electron IPC → Main Process → child_process.execFile → Python CLI → AnalysisResult JSON → ECharts
```

**핵심 데이터 계약**: `AnalysisResult` dataclass — `type`, `source_files`, `created_at`, `summary`, `series`, `tables`, `charts`, `metadata` 필드로 구성. `access_log`과 `profiler_collapsed` 두 타입에 대해 필수 키가 문서화되어 있다.

---

## 3. 내부 분석 결과

### 3.1 강점

1. **명확한 관심사 분리**: 파서 → 분석기 → 내보내기 → UI의 파이프라인이 잘 설계되어 있다. 각 모듈은 독립적으로 확장 가능한 구조이다.
2. **공통 결과 계약(AnalysisResult)**: 모든 분석기가 동일한 JSON 구조를 출력하도록 설계되어, UI와 내보내기가 파서 세부사항에 의존하지 않는다.
3. **이중 언어 문서화**: 영문/한국어 문서가 체계적으로 분리 관리되며, UI i18n 기반이 갖추어져 있다.
4. **방어적 파서 정책**: 파일/설정 오류는 치명적(fatal), 레코드 수준 오류는 건너뛰기(skip) + 진단(diagnostics) 보고라는 명확한 정책이 문서화되어 있다.
5. **확장 모델 문서화**: 새 진단 데이터 타입 추가를 위한 6단계 절차가 명시되어 있다.

### 3.2 약점 및 개선 후보

#### W-001: Engine-UI 브리지 미구현 (Critical)
- **위치**: `apps/desktop/electron/main.ts` (IPC 핸들러 없음), `apps/desktop/electron/preload.ts` (platform 속성만 노출)
- **영향**: UI는 `sampleCharts.ts`의 하드코딩 데이터만 렌더링. 실제 분석 불가.
- **근거**: `analyzerClient.ts:5-7` — `loadSampleAnalysisResult()`가 정적 샘플만 반환

#### W-002: Electron 31.x 지원 종료
- **위치**: `apps/desktop/package.json:16` — `"electron": "^31.1.0"`
- **영향**: Electron은 최근 3개 메이저 버전만 보안 패치 지원. v31은 2024년 중반 릴리스로, 2026년 현재 지원 범위 밖이다.
- **보안 위험**: Chromium/Node.js 보안 패치 미적용

#### W-003: 최소 테스트 커버리지
- **위치**: `engines/python/tests/` — 테스트 파일 3개, 테스트 케이스 5개
- **영향**: `test_access_log_parser.py` (2개 테스트), `test_collapsed_parser.py` (2개 테스트), `test_gc_log_parser.py` (1개 placeholder)
- **부재 영역**: 분석기 엣지 케이스, statistics.py, json_exporter 왕복(round-trip), malformed 입력 처리, CSV/HTML exporter

#### W-004: 파서 오류 진단(diagnostics) 미구현
- **위치**: `engines/python/archscope_engine/parsers/access_log_parser.py:27-29` — malformed 라인을 무시하되 진단 정보 미수집
- **영향**: `metadata.diagnostics` 스키마가 설계 문서에 정의되어 있으나(`PARSER_DESIGN.md:102-130`), 코드에 미구현

#### W-005: Python 의존성 미선언
- **위치**: `engines/python/pyproject.toml` — 런타임 의존성(typer, rich) 미선언
- **영향**: `pip install -e .` 시 typer, rich가 자동 설치되지 않아 수동 설치 필요

#### W-006: Placeholder 파서/분석기 (3개)
- **위치**: `gc_log_parser.py`, `thread_dump_parser.py`, `exception_parser.py` — 모두 `NotImplementedError`
- **영향**: Phase 3 계획 항목이나, UI에는 이미 해당 페이지가 존재하여 사용자 혼란 가능

#### W-007: `iter_text_lines` 인코딩 폴백 재시도 문제
- **위치**: `engines/python/archscope_engine/common/file_utils.py:7-22`
- **영향**: 제너레이터 기반인데, 인코딩 오류 시 이미 yield된 라인을 다시 읽을 수 없음. 첫 번째 인코딩(utf-8)으로 일부 라인을 yield한 후 에러가 발생하면 다음 인코딩으로 전환 시 파일을 처음부터 다시 읽게 되어, 호출자는 중복 라인을 받을 수 있다.

#### W-008: App.tsx 조건 분기 중복
- **위치**: `apps/desktop/src/App.tsx:33-39` — 9개 페이지에 대한 반복적 `&&` 조건 렌더링
- **영향**: 페이지 추가 시마다 컴포넌트 import + 조건 추가 필요. `review_decisions.md`의 RD-013에서 매핑 테이블 전환 승인됨.

### 3.3 기술 부채 인벤토리

| ID | 카테고리 | 위치 | 내용 | 우선순위 |
|---|---|---|---|---|
| TD-001 | 보안 | `package.json:16` | Electron 31.x → 41.x 업그레이드 필요 | P0 |
| TD-002 | 인프라 | `pyproject.toml:2` | `setuptools < 64` 제약 — 현대적 빌드 백엔드 전환 필요 | P1 |
| TD-003 | 인프라 | `pyproject.toml` + `setup.py` | 이중 패키징 설정 파일 — 통합 필요 | P2 |
| TD-004 | 신뢰성 | `file_utils.py:7-22` | 인코딩 폴백 체인의 제너레이터 재시도 버그 | P1 |
| TD-005 | 품질 | `tests/` | 분석기, 유틸리티, 내보내기 테스트 부재 | P1 |
| TD-006 | 기능 | `parsers/access_log_parser.py` | diagnostics 메타데이터 미구현 | P1 |
| TD-007 | 구조 | `App.tsx` | 페이지 렌더링 조건 분기 중복 | P2 |
| TD-008 | 인프라 | `pyproject.toml` | Python 런타임 의존성(typer, rich) 미선언 | P0 |

---

## 4. 외부 리서치 결과

### 4.1 업계 동향

**데스크톱 앱 기술 스택 (2025-2026)**:
- Electron은 여전히 크로스플랫폼 데스크톱 앱의 지배적 프레임워크이다. Tauri(Rust 기반)가 대안으로 부상했으나, Python 사이드카 통합에서는 Electron이 더 성숙한 생태계를 보유한다.
- Electron 41.x(2026년 기준 최신)는 Chromium 146, Node 24.14를 내장하며, ASAR 무결성 검증, MSIX 자동 업데이트 등을 지원한다.
  - 출처: https://www.electronjs.org/blog/electron-41-0

**Python 패키징 생태계 변화**:
- `uv`(Astral 사)가 2025-2026년 Python 패키징의 새 표준으로 부상했다. pip 대비 10-100배 빠른 의존성 해석 속도를 제공한다.
- `setuptools`는 여전히 작동하지만, 신규 프로젝트에는 권장되지 않는다. `hatchling` 또는 `uv_build`가 현대적 대안이다.
  - 출처: https://medium.com/@dynamicy/python-build-backends-in-2025-what-to-use-and-why-uv-build-vs-hatchling-vs-poetry-core-94dd6b92248f

**로그 분석 도구 시장**:
- 클라우드 기반: ELK Stack, Grafana Loki, SigNoz, Graylog가 주요 플레이어이다.
- ArchScope의 오프라인/데스크톱 포지셔닝은 클라우드 도구와 차별화된다. 데이터가 로컬에 머무르므로, 민감한 운영 환경(금융, 공공기관, 보안 중시 기업)에 특히 적합하다.
  - 출처: https://signoz.io/blog/open-source-log-management/

**JVM 진단 도구 트렌드**:
- JDK 25에 JFR CPU-time 프로파일링(JEP 509)이 추가되어, async-profiler와의 역할 분담이 변화하고 있다.
- JFR 녹화 파일(.jfr) 파싱은 새로운 파서 타겟으로서 시장 적합성이 높다.
  - 출처: https://www.javacodegeeks.com/2026/03/jfr-in-2026-is-not-theblack-box-you-remember-jep-509-and-continuous-profiling.html

### 4.2 경쟁/유사 솔루션 비교

| 도구 | 유형 | 강점 | ArchScope 대비 |
|------|------|------|----------------|
| **ELK Stack** | 서버 기반 | 대규모 실시간 검색 | ArchScope는 오프라인, 보고서 중심 |
| **Grafana + Loki** | 클라우드/온프레미스 | 비용 효율적 라벨 인덱싱 | ArchScope는 설치 없이 독립 실행 |
| **SigNoz** | SaaS/셀프호스트 | 통합 관측성 | ArchScope는 진단 증거 생성에 특화 |
| **GCViewer** | 데스크톱(Java) | GC 로그 전문 | ArchScope는 다중 소스 통합 분석 |
| **JDK Mission Control** | 데스크톱(Java) | JFR 분석 | ArchScope는 보고서 자동화 + 다중 런타임 |
| **Salesforce LogAI** | Python 라이브러리 | AI 로그 분석 | ArchScope는 데스크톱 GUI + 보고서 생성 |

**ArchScope의 차별점**: 다중 진단 소스(로그, GC, 프로파일러, 스레드 덤프, 예외)를 하나의 데스크톱 도구에서 통합하여 **보고서 수준 증거(report-ready evidence)**를 생성하는 점. 기존 도구들은 단일 소스에 특화되거나, 서버 설치를 요구한다.

### 4.3 적용 가능한 신기술 및 도구

#### ECharts 6.0 (2025-07 출시)
- 디자인 토큰 기반 새 기본 테마, 동적 테마 전환, 다크 모드 지원
- 신규 차트 타입: 바이올린 차트, 비스웜(beeswarm), 등고선, 범위 막대/꺾은선
- 끊어진 축(Broken Axis): 큰 규모 편차 데이터 시각화에 유용 — GC pause time 시각화에 직접 활용 가능
- SVG SSR: 의존성 없이 서버 사이드에서 SVG 문자열 생성 → 보고서 내보내기에 활용
  - 출처: https://echarts.apache.org/handbook/en/basics/release-note/v6-feature/

#### uv (Python 패키지 관리)
- Astral 사가 개발한 차세대 Python 패키지 매니저. Rust 기반으로 pip 대비 10-100배 빠른 속도.
- `uv_build` 빌드 백엔드로 pyproject.toml 단일 파일 관리 가능.
  - 출처: https://dasroot.net/posts/2026/01/python-packaging-best-practices-setuptools-poetry-hatch/

#### OpenTelemetry 로그 포맷
- 업계 표준으로 자리잡은 OTel 로그 포맷을 입력 소스로 지원하면 시장 적합성이 높아진다.
- trace_id/span_id 기반 로그-트레이스 상관관계 분석이 가능해진다.
  - 출처: https://www.dash0.com/knowledge/opentelemetry-logging-explained

#### Ollama (로컬 LLM)
- 로컬 머신에서 LLM 실행. 운영 데이터를 외부로 전송하지 않는 프라이버시 우선 AI 해석 가능.
- 구조화된 출력(Structured Output) 지원으로 프로그래밍 방식의 로그 분석 결과 생성 가능.
  - 출처: https://dev.to/devopsstart/local-llm-for-log-analysis-privacy-first-debugging-with-ollama-361o

---

## 5. 강화 제안 (우선순위순)

### 제안 #1: Electron 31 → 41 업그레이드 [Priority: High]

**As-Is**: `apps/desktop/package.json:16`에서 Electron 31.1.0을 사용 중. v31은 Electron 지원 정책(최근 3개 메이저 버전)에서 벗어난 지 오래이며, Chromium/Node.js 보안 패치가 적용되지 않는다.

**To-Be**: Electron 41.x로 업그레이드. 주요 마이그레이션 항목:
- v32: `File.path` → `webUtils.getPathForFile()` (파일 드롭 관련)
- v33: 네이티브 모듈 C++20 요구
- v38: macOS 11 미만 지원 종료
- v40: Renderer에서 Clipboard API 직접 접근 제거

**근거**: Electron 보안 권고 — https://www.electronjs.org/docs/latest/tutorial/security ; 브레이킹 체인지 목록 — https://www.electronjs.org/docs/latest/breaking-changes

**기대 효과**:
- Chromium 146 + Node 24.14의 보안 패치 적용
- ASAR 무결성 검증으로 배포 보안 강화
- MSIX 자동 업데이터 지원 (Windows)

**리스크**:
- 10개 메이저 버전 점프로 인한 호환성 문제 가능
- `FileDropZone.tsx` 등 파일 처리 코드 수정 필요
- 현재 UI가 단순한 skeleton이므로 실제 브레이킹 영향은 제한적

**예상 공수**: 2-3일 (현재 코드가 단순하므로 리스크 낮음)

---

### 제안 #2: Engine-UI 브리지 PoC 구현 [Priority: High]

**As-Is**: `apps/desktop/electron/preload.ts:3-5`는 `platform` 속성만 노출. `main.ts`에 IPC 핸들러 없음. `analyzerClient.ts`는 하드코딩 샘플 반환.

**To-Be**:
1. `preload.ts`에 `analyzeAccessLog({ filePath, format })`, `analyzeCollapsedProfile(...)` API 노출
2. `main.ts`에 `ipcMain.handle()` 핸들러 추가 — `child_process.execFile`로 Python CLI 호출
3. `analyzerClient.ts`에 mock/real 클라이언트 인터페이스 정의 + 전환 메커니즘

**근거**: `work_status.md`의 T-002, T-003 — 프로젝트 로드맵상 P0 최우선 과제. 아키텍처 문서(`ARCHITECTURE.md:59-97`)에 설계가 완료되어 있음.

**기대 효과**:
- 최초의 end-to-end 진단 경로 확보
- UI 개발을 실데이터 기반으로 전환 가능
- 이후 모든 파서/분석기 추가의 기반

**리스크**:
- Python 실행 환경 탐색(venv, 시스템 Python, PyInstaller 바이너리) 로직 필요
- 프로세스 타임아웃 처리 미설계

**예상 공수**: 3-5일

---

### 제안 #3: Python 패키징 현대화 [Priority: High]

**As-Is**:
- `pyproject.toml:2` — `setuptools < 64` 제약
- `setup.py` — 빈 `setup()` 호출만 존재
- 런타임 의존성(typer, rich) 미선언

**To-Be**:
1. `setup.py` 삭제
2. `pyproject.toml`에 빌드 백엔드를 `hatchling` 또는 `uv_build`로 변경
3. `[project]` 섹션에 의존성 명시: `typer`, `rich`, 및 `[project.optional-dependencies]`에 테스트 의존성 추가
4. `[project.scripts]`에 `archscope-engine` 콘솔 엔트리포인트 명시

**근거**:
- Python Packaging Authority 권장 사항 — https://www.pyopensci.org/python-package-guide/package-structure-code/python-package-build-tools.html
- `review_decisions.md`의 RD-020, RD-021에서 관련 작업이 승인됨 (Deferred → P3이나, 의존성 미선언은 P0 버그)

**기대 효과**:
- `pip install -e .` 시 의존성 자동 설치
- PyInstaller 패키징 시 의존성 자동 감지
- 신규 기여자의 환경 설정 시간 단축

**리스크**: 최소. 현재 코드 변경 없이 설정 파일만 변경.

**예상 공수**: 0.5일

---

### 제안 #4: 파서 진단(diagnostics) 메타데이터 구현 [Priority: High]

**As-Is**: `access_log_parser.py:27-29` — malformed 라인을 `None` 반환으로 건너뛰지만, 건너뛴 라인 수, 이유, 샘플을 수집하지 않음. `collapsed_parser.py:20-22` — 파싱 실패 시 예외를 발생시키며 건너뛰기 처리 없음.

**To-Be**: `PARSER_DESIGN.md:102-130`에 정의된 diagnostics 스키마 구현:
```python
metadata.diagnostics = {
    "total_lines": int,
    "parsed_records": int,
    "skipped_lines": int,
    "skipped_by_reason": {"NO_FORMAT_MATCH": int, ...},
    "samples": [{"line_number": int, "reason": str, "message": str, "raw_preview": str}]
}
```

**근거**: `work_status.md`의 T-004, T-005 (P1). 설계 문서에 정책이 확정되어 있으므로 구현만 필요.

**기대 효과**:
- 사용자가 파싱 실패 원인을 파악 가능
- 대용량 로그 분석 시 데이터 품질 가시성 확보
- 결과 계약(AnalysisResult)의 완결성 향상

**리스크**: 최소. 기존 파서 인터페이스를 유지하면서 metadata 필드만 추가.

**예상 공수**: 2일

---

### 제안 #5: `iter_text_lines` 인코딩 폴백 버그 수정 [Priority: High]

**As-Is**: `file_utils.py:7-22` — 제너레이터가 utf-8로 일부 라인을 yield한 후 `UnicodeDecodeError` 발생 시, 다음 인코딩(utf-8-sig)으로 파일 전체를 다시 읽음. 호출자는 중복 라인을 수신할 수 있다.

**To-Be**: 인코딩 감지를 제너레이터 시작 전에 수행하는 방식으로 변경:
```python
def iter_text_lines(path: Path) -> Iterable[str]:
    encoding = detect_encoding(path)  # 파일 앞부분만 읽어 인코딩 판별
    with path.open("r", encoding=encoding) as handle:
        for line in handle:
            yield line.rstrip("\n")
```

**근거**: 현재 구현의 논리적 결함. `PARSER_DESIGN.md:148-154`에서 인코딩 동작이 정의되어 있으나 구현이 설계와 불일치.

**기대 효과**: 다국어/다인코딩 로그 파일 처리 시 데이터 무결성 보장.

**리스크**: 인코딩 감지 로직 변경으로 기존 테스트 영향 가능. 단, 현재 인코딩 관련 테스트 부재.

**예상 공수**: 0.5일

---

### 제안 #6: 테스트 커버리지 확충 [Priority: High]

**As-Is**: 테스트 3개 파일, 5개 케이스만 존재. `work_status.md`의 T-006~T-009에 구체적 테스트 항목이 정의됨.

**To-Be**:
1. `statistics.py` 엣지 케이스: 빈 리스트, 단일 값, 반복 값, 음수, 백분위 보간 (T-008)
2. `access_log_parser` malformed 입력: 포맷 불일치, 잘못된 타임스탬프, 잘못된 숫자 (T-006)
3. `collapsed_parser` malformed 입력: 샘플 카운트 누락, 비정수, 음수 (T-007)
4. `json_exporter` 왕복 테스트: 쓰기 → 읽기 → 구조 비교 (T-009)

**근거**: `review_decisions.md`의 RD-016, RD-017, RD-018에서 모두 Accepted.

**기대 효과**: 파서 동작 변경 시 회귀 방지, 계약 안정성 검증.

**리스크**: 없음. 기존 코드 변경 불요.

**예상 공수**: 2일

---

### 제안 #7: ECharts 5.5 → 6.0 업그레이드 [Priority: Medium]

**As-Is**: `package.json:15` — `echarts: ^5.5.1`

**To-Be**: ECharts 6.0으로 업그레이드. 활용 가능한 신기능:
- 디자인 토큰 기반 동적 테마 전환 (라이트/다크)
- Broken Axis: GC pause 같은 큰 편차 데이터에 유용
- SVG SSR: Node.js 환경에서 의존성 없이 SVG 차트 생성 → 보고서 내보내기
- 바이올린/비스웜 차트: 응답 시간 분포 시각화

**근거**: https://echarts.apache.org/handbook/en/basics/release-note/v6-feature/

**기대 효과**:
- 다크 모드 즉시 지원 (사용자 요청 빈도 높은 기능)
- 보고서 품질 향상 (더 다양한 차트 옵션)
- 헤드리스 SVG 내보내기로 Phase 2 Report Export 가속

**리스크**: v5 → v6 마이그레이션 시 기본 테마 변경으로 기존 차트 외관 변화. v5 호환 테마 파일 제공됨.

**예상 공수**: 1일

---

### 제안 #8: TypedDict/TypeScript 결과 계약 강화 [Priority: Medium]

**As-Is**: `analysis_result.py:9-21` — `AnalysisResult`의 `summary`, `series`, `tables`, `metadata`가 모두 `dict[str, Any]`. TypeScript 측에도 타입 정의 없음.

**To-Be**:
1. Python `TypedDict`로 Access Log / Profiler 결과 섹션의 필수 키 정의
2. TypeScript `interface`로 동일 계약 정의
3. `DATA_MODEL.md`에 문서화된 필수 키 테이블과 코드 동기화

**근거**: `review_decisions.md`의 RD-006 (Accepted, P1). `DATA_MODEL.md:62-135`에 이미 필수 필드가 상세 정의됨.

**기대 효과**:
- 타입 안전성으로 브리지 JSON 교환 시 런타임 오류 조기 발견
- 자동 완성으로 개발 생산성 향상
- 문서-코드 일관성 확보

**리스크**: 기존 분석기 코드가 이미 해당 키를 출력하므로 호환성 문제 없음.

**예상 공수**: 2일

---

### 제안 #9: JFR 녹화 파일 파서 추가 [Priority: Medium]

**As-Is**: 현재 JVM 진단은 GC 로그, 스레드 덤프, 예외만 계획. JFR은 로드맵에 없음.

**To-Be**: Phase 3에 JFR 녹화 파일(.jfr) 파서를 추가하여, JFR 이벤트(CPU 사용량, GC 이벤트, 락 경합, I/O 대기)를 AnalysisResult로 변환.

**근거**:
- JDK 25의 JEP 509로 JFR의 CPU-time 프로파일링 기능이 강화됨
- JFR은 JVM 생태계의 표준 진단 포맷으로 자리잡았으며, async-profiler도 JFR 출력을 지원
- 출처: https://www.javacodegeeks.com/2026/03/jfr-in-2026-is-not-theblack-box-you-remember-jep-509-and-continuous-profiling.html

**기대 효과**:
- JVM 진단 시장에서의 차별화 강화
- GC 로그 + 스레드 덤프 + JFR을 통합한 타임라인 상관분석(T-028) 기반 마련
- 하나의 .jfr 파일로 다수 진단 관점 확보

**리스크**: JFR 바이너리 포맷 파싱 복잡도가 높음. Python에 `jfr-parser` 같은 라이브러리 존재 여부 확인 필요.

**예상 공수**: 5-7일 (파서 + 분석기 + 테스트)

---

### 제안 #10: OpenTelemetry 로그 포맷 입력 지원 [Priority: Medium]

**As-Is**: 입력 소스는 NGINX 액세스 로그와 async-profiler collapsed 포맷만 지원.

**To-Be**: OpenTelemetry Log Data Model(OTLP JSON) 형식의 로그를 입력으로 받는 파서 추가.

**근거**:
- OpenTelemetry가 관측성 업계 표준으로 정착하고 있으며, 많은 기업이 로그를 OTel 포맷으로 전환 중
- OTel 로그에는 trace_id, span_id가 포함되어 로그-트레이스 상관분석 가능
- 출처: https://www.dash0.com/knowledge/opentelemetry-logging-explained

**기대 효과**:
- 모던 관측성 파이프라인과의 호환성 확보
- 기존 OTel Collector로 수집된 로그를 오프라인 진단에 활용 가능
- 시장 적합성 향상

**리스크**: OTel 로그 스키마가 범용적이어서 분석기 설계에 도메인 지식이 필요.

**예상 공수**: 3-4일

---

### 제안 #11: electron-builder 배포 파이프라인 구축 [Priority: Medium]

**As-Is**: 패키징 미구현. `ARCHITECTURE.md:156-162`에서 electron-builder + PyInstaller 방향만 문서화.

**To-Be**:
1. `electron-builder` 설정 추가 (macOS DMG, Windows NSIS/MSIX)
2. PyInstaller로 Python 엔진을 단일 바이너리로 빌드
3. electron-builder의 `extraResources`로 Python 바이너리 번들링
4. macOS 코드 서명 + 공증(notarization) 설정

**근거**:
- electron-builder가 주간 다운로드 160만으로 업계 표준
- 패키징을 지연하면 경로/환경 문제가 후반에 발견되는 리스크
- `work_status.md` T-022 (P3)에 이미 계획됨
- 출처: https://www.electron.build/auto-update.html

**기대 효과**: 사용자 배포 가능한 설치 파일 생성, 릴리스 파이프라인 확보.

**리스크**: macOS 공증에 Apple Developer 계정($99/년) 필요, Windows 코드 서명에 인증서 필요.

**예상 공수**: 3-5일 (기본 설정) + 1-2일 (코드 서명)

---

### 제안 #12: 로컬 LLM 기반 AI 해석 레이어 (Ollama 통합) [Priority: Low]

**As-Is**: Phase 5 로드맵에 "AI-assisted interpretation, optional"로 계획. RD-024에서 원시 증거 참조 필수 가드레일 승인.

**To-Be**:
1. Ollama를 선택적 의존성으로 통합
2. 파서 결과(구조화된 findings)를 LLM 프롬프트 컨텍스트로 전달
3. AI 해석 결과에 반드시 소스 증거(타임스탬프, 메트릭, 스택 프레임) 참조 포함
4. UI에서 AI 해석을 별도 섹션으로 표시, "AI-generated" 라벨 명시

**근거**:
- 로컬 LLM으로 운영 데이터가 외부로 전송되지 않음 — 엔터프라이즈 고객의 핵심 요구사항
- Structured Output 지원으로 프로그래밍 방식의 결과 생성 가능
- 출처: https://dev.to/devopsstart/local-llm-for-log-analysis-privacy-first-debugging-with-ollama-361o

**기대 효과**:
- 아키텍트가 수동으로 수행하던 해석 작업 보조
- 프라이버시 우선 AI로 엔터프라이즈 시장 차별화

**리스크**:
- LLM 환각(hallucination) — 증거 기반 가드레일로 완화
- Ollama 설치 요구로 사용자 진입 장벽 증가
- Phase 1-4 완료 후에만 의미 있음

**예상 공수**: 5-7일 (통합 + 프롬프트 설계 + UI)

---

## 6. 단기/중기/장기 로드맵 제안

### 단기 (0~3개월): Foundation 완결

| 순서 | 항목 | 제안 ID | 의존성 |
|------|------|---------|--------|
| 1 | Python 패키징 현대화 + 의존성 선언 | #3 | 없음 |
| 2 | `iter_text_lines` 인코딩 버그 수정 | #5 | 없음 |
| 3 | 파서 diagnostics 구현 | #4 | #5 |
| 4 | 테스트 커버리지 확충 | #6 | #4 |
| 5 | TypedDict/TypeScript 계약 강화 | #8 | #4 |
| 6 | Engine-UI 브리지 PoC | #2 | #3, #8 |
| 7 | Electron 41 업그레이드 | #1 | #2 (PoC 검증 후) |

### 중기 (3~6개월): Report-Ready & Distribution

| 순서 | 항목 | 제안 ID | 의존성 |
|------|------|---------|--------|
| 8 | ECharts 6.0 업그레이드 + 다크 모드 | #7 | #1 |
| 9 | SVG/PNG 보고서 내보내기 | — | #7 |
| 10 | GC Log / Thread Dump 파서 구현 | — | #4, #6 |
| 11 | electron-builder 배포 파이프라인 | #11 | #1 |
| 12 | OTel 로그 포맷 파서 | #10 | #4 |

### 장기 (6개월 이후): 고급 진단 & AI

| 순서 | 항목 | 제안 ID | 의존성 |
|------|------|---------|--------|
| 13 | JFR 파서 | #9 | GC/Thread 파서 |
| 14 | 타임라인 상관분석 | — | #9, GC/Thread 파서 |
| 15 | 다중 런타임(Node.js, Python, Go, .NET) | — | #14 |
| 16 | 로컬 LLM AI 해석 | #12 | #14 |
| 17 | PowerPoint/Executive Summary 자동 생성 | — | #12 |

---

## 7. 참고 자료

| 주제 | URL | 접근일 |
|------|-----|--------|
| Electron 41 릴리스 | https://www.electronjs.org/blog/electron-41-0 | 2026-04-28 |
| Electron 브레이킹 체인지 | https://www.electronjs.org/docs/latest/breaking-changes | 2026-04-28 |
| Electron 보안 가이드 | https://www.electronjs.org/docs/latest/tutorial/security | 2026-04-28 |
| React 19 릴리스 | https://react.dev/blog/2024/12/05/react-19 | 2026-04-28 |
| React 19 업그레이드 가이드 | https://react.dev/blog/2024/04/25/react-19-upgrade-guide | 2026-04-28 |
| Python 빌드 백엔드 비교 2025 | https://medium.com/@dynamicy/python-build-backends-in-2025-what-to-use-and-why-uv-build-vs-hatchling-vs-poetry-core-94dd6b92248f | 2026-04-28 |
| Python 패키징 베스트 프랙티스 | https://www.pyopensci.org/python-package-guide/package-structure-code/python-package-build-tools.html | 2026-04-28 |
| PyInstaller vs Nuitka vs cx_Freeze | https://ahmedsyntax.com/2026-comparison-pyinstaller-vs-cx-freeze-vs-nui/ | 2026-04-28 |
| ECharts 6.0 기능 | https://echarts.apache.org/handbook/en/basics/release-note/v6-feature/ | 2026-04-28 |
| ECharts SSR 가이드 | https://apache.github.io/echarts-handbook/en/how-to/cross-platform/server/ | 2026-04-28 |
| 오픈소스 로그 관리 비교 | https://signoz.io/blog/open-source-log-management/ | 2026-04-28 |
| OpenTelemetry 로깅 | https://www.dash0.com/knowledge/opentelemetry-logging-explained | 2026-04-28 |
| JFR in 2026 (JEP 509) | https://www.javacodegeeks.com/2026/03/jfr-in-2026-is-not-theblack-box-you-remember-jep-509-and-continuous-profiling.html | 2026-04-28 |
| Electron + Python 통합 예시 | https://github.com/fyears/electron-python-example | 2026-04-28 |
| electron-builder 자동 업데이트 | https://www.electron.build/auto-update.html | 2026-04-28 |
| electron-builder vs electron-forge | https://npmtrends.com/electron-builder-vs-electron-forge | 2026-04-28 |
| 로컬 LLM 로그 분석 (Ollama) | https://dev.to/devopsstart/local-llm-for-log-analysis-privacy-first-debugging-with-ollama-361o | 2026-04-28 |
| Salesforce LogAI | https://github.com/salesforce/logai | 2026-04-28 |
| RAG 환각 방지 가이드 | https://www.mdpi.com/2227-7390/13/5/856 | 2026-04-28 |
