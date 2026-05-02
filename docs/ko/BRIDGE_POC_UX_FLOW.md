# Bridge PoC UX Flow

이 문서는 첫 Engine-UI Bridge PoC의 최소 사용자 경험을 고정한다.

목표는 다음 경로를 증명하는 것이다.

```text
local file 선택 -> analyzer 호출 -> AnalysisResult JSON 수신 -> summary/chart/table 렌더링
```

PoC는 최종 analyzer workspace가 아니라 실제로 동작하는 최소 진단 경로처럼 느껴져야 한다.

## 범위

첫 PoC에 포함하는 범위:

- `analyzeAccessLog({ filePath, format })` 기반 Access Log 분석
- `analyzeCollapsedProfile({ wallPath, wallIntervalMs, elapsedSec, topN })` 기반 collapsed profiler 분석
- loading, success, parser diagnostics, bridge error 상태

포함하지 않는 범위:

- 여러 파일 간 correlation
- chart editing
- report export
- persisted analysis history
- custom parser configuration UI

## Access Log Flow

1. 사용자가 access log 파일 하나를 선택하거나 drop한다.
2. 사용자가 log format을 선택한다. 기본값은 `nginx`다.
3. File path와 format이 모두 있을 때만 Analyze button이 활성화된다.
4. Analyze 실행 시 renderer는 `AnalyzerClient.analyzeAccessLog`를 호출한다.
5. 요청이 진행 중일 때도 file control과 format control은 보이며, Analyze button은 loading 상태를 표시한다.
6. 성공하면 반환된 `AnalysisResult`의 summary, chart-ready series, parser diagnostics를 렌더링한다.
7. 실패하면 선택된 file과 option은 유지하고 action 영역 근처에 bridge error message를 표시한다.

## Profiler Flow

1. 사용자가 wall-clock collapsed stack file 하나를 선택하거나 drop한다.
2. 사용자가 `wallIntervalMs`를 설정한다. 기본값은 `100`이다.
3. 사용자는 `elapsedSec`, `topN`을 선택적으로 설정할 수 있다. `topN` 기본값은 `20`이다.
4. Wall file path와 양수 interval이 있을 때만 Analyze button이 활성화된다.
5. Analyze 실행 시 renderer는 `AnalyzerClient.analyzeCollapsedProfile`을 호출한다.
6. 요청이 진행 중일 때도 control은 보이며, Analyze button은 loading 상태를 표시한다.
7. 성공하면 반환된 `AnalysisResult`에서 summary metric과 top stack table을 렌더링한다.
8. 실패하면 선택된 file과 option은 유지하고 action 영역 근처에 bridge error message를 표시한다.

## UI State Model

Analyzer page는 다음 상태를 사용한다.

| State | 의미 | 주요 UI |
|---|---|---|
| `idle` | 아직 분석을 시작하지 않음 | 빈 metric, 빈 chart/table 영역, 필수 입력 전 Analyze 비활성화 |
| `ready` | 필수 입력이 있음 | Analyze 활성화 |
| `running` | IPC 요청 진행 중 | Analyze 비활성화 및 loading text 표시, 이전 결과는 새 성공 결과가 오기 전까지 유지 |
| `success` | Analyzer가 `ok: true` 반환 | `result`에서 summary, series, tables, diagnostics 렌더링 |
| `error` | Analyzer가 `ok: false` 반환하거나 IPC 예외 발생 | 안정적인 code와 사용자용 message를 가진 error panel 표시 |

Renderer는 stdout을 파싱하거나 process exit text로 성공 여부를 추론하지 않는다. UI boundary는 `AnalyzerResponse` contract다.

## Success Rendering

Success rendering은 표준화된 `AnalysisResult` field를 우선 사용한다.

- `result.summary`에서 summary card 렌더링
- `result.series`에서 trend chart 렌더링
- `result.tables`에서 top-N 또는 detail table 렌더링
- `result.charts`에서 optional chart template 사용
- `result.metadata.diagnostics`에서 diagnostics 렌더링

첫 PoC result에서 특정 field가 빠져 있으면 전체 page를 실패시키지 않고 해당 panel만 empty state를 표시한다.

## Diagnostics Panel

`result.metadata.diagnostics`가 있으면 parser diagnostics panel을 표시한다.

최소 지원 field:

| Field | 표시 방식 |
|---|---|
| `parsed_records` | Aggregation에 포함된 record 수 |
| `skipped_lines` | Skip된 malformed non-blank record 수 |
| `encoding` | 사용한 source file encoding, 제공된 경우 |
| `samples` | 제한된 malformed-record 예시, 제공된 경우 |

Diagnostics는 성공한 run에서 정보성으로 표시한다. Skipped record가 있는 결과도 성공일 수 있다.

## Error Messages

Bridge error는 `BridgeError` contract를 따른다.

```text
{
  code,
  message,
  detail?
}
```

초기 error code category:

| Code | 사용 시점 |
|---|---|
| `ANALYZER_NOT_CONNECTED` | Mock client 또는 IPC bridge가 연결되지 않음 |
| `FILE_NOT_FOUND` | 선택된 file path가 더 이상 존재하지 않거나 읽을 수 없음 |
| `INVALID_OPTION` | 필수 analyzer option이 없거나 잘못됨 |
| `ENGINE_EXITED` | Python CLI가 non-zero exit code를 반환 |
| `ENGINE_OUTPUT_INVALID` | CLI가 종료됐지만 유효한 `AnalysisResult` JSON을 만들지 못함 |
| `IPC_FAILED` | 정규화된 bridge response가 반환되기 전 IPC invocation 실패 |

UI는 `message`를 primary text로 표시하고, support/debugging을 위해 `code`도 보이게 한다. `detail`은 해당 UI가 생기면 compact expandable 영역에 표시할 수 있다.

## 성능 기대치

| 분석 유형 | 입력 크�� | 기대 응답 시간 | 비고 |
|-----------|-----------|----------------|------|
| Access Log | 10,000 줄 | < 2초 | 파일 읽기 + 파싱 + 집계 + JSON 쓰기 + IPC 반환 |
| Access Log | 100,000 줄 | < 5초 | streaming aggregation |
| Collapsed Profiler | 10,000 스택 | < 2초 | flamegraph tree 구축 포함 |
| Collapsed Profiler | 100,000 스택 | < 5초 | drilldown + breakdown |

사용자 체감 기준:
- 2초 이내: 즉각적 응답으로 느껴짐
- 2~5초: "분석 중..." 상태 표시가 적절함
- 5초 초과: Cancel 버튼 활용 안내, max_lines 옵션 제안

## T-003 구현 참고

- File selection은 renderer에 두되, Python 실행은 Electron main process에서만 한다.
- Renderer에서 preload와 IPC로 typed request object를 전달한다.
- Main process는 Python CLI를 `execFile`로 호출하고, output JSON을 temporary path에 쓰게 한 뒤 JSON을 읽어 `AnalyzerResponse`로 반환한다.
- Renderer에 raw command string을 노출하지 않는다.
- Stdout parsing을 data contract로 삼지 않는다.
