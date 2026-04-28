# ArchScope 아키텍처

ArchScope는 애플리케이션 아키텍처 진단 및 보고서 작성을 위한 toolkit이다. 핵심 책임은 운영 환경에서 수집한 원천 데이터를 표준화된 분석 결과와 보고서용 시각화 자료로 변환하는 것이다.

## 제품 포지셔닝

ArchScope는 **privacy-first local professional diagnostic workbench**로 포지셔닝한다.

제품 방향은 다음 요소를 결합하는 것이다.

- SaaS JVM 진단 도구의 편의성
- 전통적인 desktop analyzer의 local/offline 보안성
- 현대적인 report-ready visualization
- 여러 runtime을 수용할 수 있는 표준 evidence contract

ArchScope는 범용 log viewer나 full observability backend가 되는 것을 목표로 하지 않는다. Offline operational evidence를 architecture diagnosis와 report artifact로 변환하는 데 집중한다.

## 시스템 흐름

```text
원천 데이터
  -> 파싱
  -> 분석 / 집계
  -> 시각화
  -> 보고서용 Export
```

## 구성 요소

### Desktop UI

Desktop application은 Electron, React, TypeScript, Apache ECharts를 기반으로 한다.

- Analyzer navigation
- File selection workflow
- Chart dashboard
- Chart Studio placeholder
- Export Center placeholder
- English/Korean UI locale switching

UI는 raw log가 아니라 표준화된 analysis result JSON을 읽는다. 이 구조는 parser 구현과 화면 렌더링을 분리한다.

### Python Analysis Engine

Python engine은 parsing, normalization, aggregation, export를 담당한다.

- `parsers`: source file을 typed record로 변환
- `analyzers`: summary, series, table 구조로 집계
- `models`: 표준 데이터 모델 dataclass
- `exporters`: JSON, CSV, HTML 및 향후 export 형식 처리
- `common`: 파일, 시간, 통계 유틸리티

### Result Contract

모든 analyzer는 AnalysisResult 형태의 JSON을 생성한다.

```text
type
source_files
created_at
summary
series
tables
charts
metadata
```

차트는 raw log line이 아니라 `series`, `tables`, `summary`를 기반으로 렌더링한다.

## Engine-UI Bridge

초기 Engine-UI bridge는 다음 방식으로 확정한다.

```text
React Renderer
  -> preload 노출 API
  -> Electron IPC
  -> Electron Main Process
  -> child_process.execFile
  -> Python Engine CLI
  -> AnalysisResult JSON
  -> ECharts renderer
```

### 결정 사항

ArchScope는 desktop 통합 경로로 **Electron IPC + Python CLI child process** 방식을 사용한다.

Renderer process는 Python을 직접 실행하지 않는다. Renderer는 `preload.ts`가 제한적으로 노출한 API만 호출한다. Preload 계층은 요청을 `ipcRenderer.invoke`로 main process에 전달한다. Electron main process가 process 실행을 책임지고, Python engine은 `child_process.execFile`로 호출한다. Shell execution은 사용하지 않는다.

### 개발 Runtime

개발 환경에서는 main process가 Python engine을 다음 중 하나로 호출할 수 있다.

```text
python -m archscope_engine.cli ...
```

또는 설치된 console script:

```text
archscope-engine ...
```

CLI는 임시 output path에 `AnalysisResult` JSON을 쓴다. Main process는 JSON 파일을 읽고 기본 shape를 검증한 뒤 renderer로 반환한다.

### 패키징 Runtime

패키징된 desktop build에서는 Python engine을 PyInstaller sidecar binary로 번들링하고 application resources directory에서 찾는다. 이 packaging 작업은 Bridge PoC 이후로 연기한다.

### IPC Contract

Renderer에 노출하는 API는 raw command line이 아니라 analyzer request 중심으로 타입을 정의한다. 초기 request shape는 다음과 같다.

```text
analyzeAccessLog({ filePath, format })
analyzeCollapsedProfile({ wallPath, wallIntervalMs, elapsedSec, topN })
```

초기 response shape:

```text
{
  ok: true,
  result: AnalysisResult
}
```

오류 response shape:

```text
{
  ok: false,
  error: {
    code,
    message,
    detail?
  }
}
```

### Bridge 규칙

- Shell interpolation을 피하기 위해 `exec`가 아니라 `execFile`을 사용한다.
- 허용된 analyzer command는 Electron main process에 명시적으로 둔다.
- File path와 analyzer option은 argument array로 전달한다.
- 임시 JSON output은 project tree 밖에 저장한다.
- Renderer에는 표준화된 JSON만 반환하며, stdout parsing을 data contract로 삼지 않는다.
- `contextIsolation: true`, `nodeIntegration: false`를 유지한다.
- Local HTTP/FastAPI 방식은 web delivery가 근시일 내 제품 목표가 될 때만 재검토한다.

## 확장 모델

새 diagnostic data type은 다음 순서로 추가한다.

1. `models`에 record model 추가
2. `parsers`에 streaming parser 추가
3. `analyzers`에 aggregation logic 추가
4. 공통 JSON exporter로 result 생성
5. desktop chart catalog에 chart template 추가
6. analyzer page 또는 기존 UI 확장

이 확장 모델은 SDK-like boundary를 유지해야 한다. Parser와 analyzer가 추가되어도 UI foundation을 다시 작성하거나 외부 `AnalysisResult` transport shape를 바꾸지 않아야 한다.

## Runtime Scope

초기에는 JVM 진단에 집중하지만, ArchScope는 Java 전용 도구가 아니다. Node.js, Python, Go, .NET, middleware log까지 확장 가능한 runtime-neutral 구조를 유지한다.

향후 JVM 및 observability input에는 JFR recording과 OpenTelemetry log record가 포함될 수 있다. 이들 input도 `AnalysisResult`로 정규화하고, 추적 가능한 raw evidence 또는 event reference를 보존해야 한다.

## Packaging 방향

향후 packaging은 다음 도구를 기준으로 한다.

- Desktop application: `electron-builder`
- Python engine: `PyInstaller`

초기 skeleton에서는 packaging 구현을 포함하지 않는다.
