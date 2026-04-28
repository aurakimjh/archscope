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
