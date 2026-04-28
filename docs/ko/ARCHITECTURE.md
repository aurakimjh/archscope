# ArchScope 아키텍처

ArchScope는 애플리케이션 아키텍처 진단 및 보고서 작성을 위한 toolkit이다. 핵심 책임은 운영 환경에서 수집한 원천 데이터를 표준화된 분석 결과와 보고서용 시각화 자료로 변환하는 것이다.

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

## 확장 모델

새 diagnostic data type은 다음 순서로 추가한다.

1. `models`에 record model 추가
2. `parsers`에 streaming parser 추가
3. `analyzers`에 aggregation logic 추가
4. 공통 JSON exporter로 result 생성
5. desktop chart catalog에 chart template 추가
6. analyzer page 또는 기존 UI 확장

## Runtime Scope

초기에는 JVM 진단에 집중하지만, ArchScope는 Java 전용 도구가 아니다. Node.js, Python, Go, .NET, middleware log까지 확장 가능한 runtime-neutral 구조를 유지한다.

## Packaging 방향

향후 packaging은 다음 도구를 기준으로 한다.

- Desktop application: `electron-builder`
- Python engine: `PyInstaller`

초기 skeleton에서는 packaging 구현을 포함하지 않는다.
