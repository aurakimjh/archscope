# 데이터 모델

ArchScope는 모든 parser output을 공통 analysis structure로 표준화한다. 이를 통해 UI, chart template, exporter가 안정적인 contract를 기준으로 동작한다.

## 공통 결과 모델

```text
AnalysisResult
  type
  source_files
  created_at
  summary
  series
  tables
  charts
  metadata
```

### 필드 목적

- `type`: `access_log`, `profiler_collapsed` 같은 진단 결과 유형
- `source_files`: 결과 생성에 사용한 source file 목록
- `created_at`: ISO 8601 생성 시각
- `summary`: card와 executive summary에 사용할 핵심 metric
- `series`: chart-ready time series 또는 distribution
- `tables`: 보고서용 table data
- `charts`: chart template reference와 rendering metadata
- `metadata`: parser format, runtime, time zone, interval 등 부가 정보

## 계약 강화 범위

1차 계약 강화는 이미 코드에 존재하는 analyzer result type으로 제한한다.

- `access_log`
- `profiler_collapsed`

공통 `AnalysisResult` dataclass는 당분간 외부 transport model로 유지한다. 계약 강화 계층은 `summary`, `series`, `tables`, `metadata` 내부에 들어가는 type별 필수 key를 정의하는 방식으로 적용한다.

### 이번 범위에 포함

- Access Log와 Profiler result section에 대한 Python `TypedDict` 정의
- Renderer와 chart code에서 사용할 대응 TypeScript interface
- 필수 key, 값 type, unit 문서화
- 향후 migration을 위한 `metadata.schema_version` 유지

### 이번 범위에서 제외

- Pydantic model 전면 전환
- 모든 nested field에 대한 runtime validation
- GC log, thread dump, exception analyzer 구현 전 해당 result contract 확정
- Chart Studio template schema
- Dashboard sample data를 canonical contract로 취급하는 것. `dashboard_sample`은 UI fixture data로만 본다.

### Versioning 규칙

- Optional field 추가는 동일 `schema_version`에서 허용한다.
- Required field 제거 또는 rename은 `schema_version` bump가 필요하다.
- Numeric field는 unit이 명확하지 않으면 key에 `_ms`, `_sec`, `_percent` 같은 unit suffix를 둔다.
- Malformed-input 처리를 지원하는 parser의 diagnostics는 `metadata.diagnostics` 아래에 둔다.

## 필수 Result Contract

### Access Log Result

`type`: `access_log`

필수 `summary` fields:

| Field | Type | Unit / 의미 |
|---|---|---|
| `total_requests` | integer | 파싱된 request 수 |
| `avg_response_ms` | number | 평균 응답 시간, milliseconds |
| `p95_response_ms` | number | 95 percentile 응답 시간, milliseconds |
| `p99_response_ms` | number | 99 percentile 응답 시간, milliseconds |
| `error_rate` | number | HTTP status `>= 400` 요청 비율, percent |

필수 `series` fields:

| Field | Row shape |
|---|---|
| `requests_per_minute` | `{ time: string, value: number }` |
| `avg_response_time_per_minute` | `{ time: string, value: number }` |
| `p95_response_time_per_minute` | `{ time: string, value: number }` |
| `status_code_distribution` | `{ status: string, count: integer }` |
| `top_urls_by_count` | `{ uri: string, count: integer }` |
| `top_urls_by_avg_response_time` | `{ uri: string, avg_response_ms: number, count: integer }` |

필수 `tables` fields:

| Field | Row shape |
|---|---|
| `sample_records` | `{ timestamp: string, method: string, uri: string, status: integer, response_time_ms: number }` |

필수 `metadata` fields:

| Field | Type | 의미 |
|---|---|---|
| `format` | string | Access log format selector |
| `parser` | string | Parser implementation identifier |
| `schema_version` | string | Result schema version |
| `diagnostics` | `ParserDiagnostics` | Parser line count와 skipped record sample |
| `analysis_options` | `AccessLogAnalysisOptions` | 적용된 sampling 및 time-range option |
| `findings` | array of `AccessLogFinding` | 보고서 지향 access-log observation |

`AccessLogAnalysisOptions` 필수 fields:

| Field | Type | 의미 |
|---|---|---|
| `max_lines` | integer or null | source file에서 읽을 최대 physical line 수 |
| `start_time` | string or null | inclusive ISO 8601 lower timestamp bound |
| `end_time` | string or null | inclusive ISO 8601 upper timestamp bound |

`AccessLogFinding` 필수 fields:

| Field | Type | 의미 |
|---|---|---|
| `severity` | string | `warning`, `critical` 같은 finding severity |
| `code` | string | stable finding code |
| `message` | string | 사람이 읽을 수 있는 finding summary |
| `evidence` | object | finding을 뒷받침하는 작은 structured value |

### Profiler Collapsed Result

`type`: `profiler_collapsed`

필수 `summary` fields:

| Field | Type | Unit / 의미 |
|---|---|---|
| `profile_kind` | string | Profile type, 초기값은 `wall` |
| `total_samples` | integer | Collapsed stack sample 총수 |
| `interval_ms` | number | Sampling interval, milliseconds |
| `estimated_seconds` | number | 추정 sampled time, seconds |
| `elapsed_seconds` | number or null | Optional observed elapsed time, seconds |

필수 `series` fields:

| Field | Row shape |
|---|---|
| `top_stacks` | `{ stack: string, samples: integer, estimated_seconds: number, sample_ratio: number, elapsed_ratio: number | null }` |
| `component_breakdown` | `{ component: string, samples: integer }` |

필수 `tables` fields:

| Field | Row shape |
|---|---|
| `top_stacks` | `{ stack: string, samples: integer, estimated_seconds: number, sample_ratio: number, elapsed_ratio: number | null, frames: string[] }` |

필수 `metadata` fields:

| Field | Type | 의미 |
|---|---|---|
| `parser` | string | Parser implementation identifier |
| `schema_version` | string | Result schema version |
| `diagnostics` | `ParserDiagnostics` | Parser line count와 skipped record sample |

### ParserDiagnostics

필수 fields:

| Field | Type | 의미 |
|---|---|---|
| `total_lines` | integer | source file에서 읽은 physical line 수 |
| `parsed_records` | integer | parser가 수용한 valid non-blank record 수 |
| `skipped_lines` | integer | parser가 skip한 malformed non-blank record 수 |
| `skipped_by_reason` | object mapping string to integer | reason code별 skipped record count |
| `samples` | array of `DiagnosticSample` | skipped record의 bounded example |

`DiagnosticSample` 필수 fields:

| Field | Type | 의미 |
|---|---|---|
| `line_number` | integer | 1-based source line number |
| `reason` | string | `NO_FORMAT_MATCH` 같은 stable reason code |
| `message` | string | 사람이 읽을 수 있는 parser message |
| `raw_preview` | string | truncated raw input preview, 현재 200 characters로 제한 |

## AccessLogRecord

```text
timestamp
method
uri
status
response_time_ms
bytes_sent
client_ip
user_agent
referer
raw_line
```

## GcEvent

```text
timestamp
uptime_sec
gc_type
cause
pause_ms
heap_before_mb
heap_after_mb
heap_committed_mb
young_before_mb
young_after_mb
old_before_mb
old_after_mb
metaspace_before_mb
metaspace_after_mb
raw_line
```

## ProfileStack

```text
stack
frames
samples
estimated_seconds
sample_ratio
elapsed_ratio
category
```

## ThreadDumpRecord

```text
thread_name
thread_id
state
stack
lock_info
category
raw_block
```

## ExceptionRecord

```text
timestamp
language
exception_type
message
root_cause
stack
signature
raw_block
```

## 설계 원칙

- Parser는 가능한 경우 `raw_line` 또는 `raw_block`으로 원본 근거를 보존한다.
- Analyzer는 숫자 필드에 명확한 unit을 사용한다.
- Chart input은 parser-specific object가 아니라 `series`와 `tables`를 사용한다.
- Runtime-specific field는 범용성이 낮으면 `metadata`에 둔다.
- Analyzer sampling 및 filter setting은 `metadata.analysis_options` 아래에 echo한다.
- 보고서용 interpretation은 prose-only blob이 아니라 bounded structured finding으로 표현한다.
