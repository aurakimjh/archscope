# 파서 설계

Parser는 raw file을 typed record로 변환한다. Chart rendering이나 report formatting 책임은 갖지 않는다.

## 책임

- encoding fallback을 사용해 파일 읽기
- line-oriented 또는 block-oriented evidence parsing
- traceability를 위해 raw input fragment 보존
- typed record 또는 parser diagnostic 반환
- 향후 대용량 파일 처리를 위한 streaming pattern 유지

## Access Log Parser

초기 지원 대상은 response time field를 포함한 NGINX combined 유사 format이다.

```text
127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] "GET /api/orders/1001 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123
```

Parser는 다음 필드를 추출한다.

- timestamp
- method
- uri
- status
- bytes sent
- referer
- user agent
- response time in milliseconds
- raw line

향후 Apache, OHS, WebLogic, Tomcat, custom regex pattern을 지원한다.

## Collapsed Profiler Parser

초기 지원 대상은 async-profiler collapsed output이다.

```text
frame1;frame2;frame3 123
```

규칙:

- 마지막 필드는 sample count로 읽는다.
- stack string은 마지막 sample count 앞의 전체 문자열이다.
- 동일 stack은 sample count를 합산한다.
- total samples를 계산한다.
- estimated seconds는 `samples * interval_ms / 1000`으로 계산한다.
- Top N stack은 sample count 기준 내림차순으로 정렬한다.

## Placeholder Parsers

GC log, thread dump, exception stack trace parser는 초기 skeleton에서 placeholder로 둔다. 이후에도 parser와 analyzer 책임 분리를 유지한다.

## Error Handling

Parser error handling은 다음 원칙으로 확정한다.

```text
File-level 또는 configuration-level 실패는 fatal로 처리한다.
Record-level malformed input은 기본적으로 non-fatal로 처리하고 diagnostics에 보고한다.
```

### 기본 Mode

기본 parser mode는 tolerant로 둔다. 운영 로그에서는 일부 line이 깨졌다는 이유로 전체 대용량 파일 분석을 중단하면 실무 가치가 떨어지기 때문이다.

- Blank line은 무시하며 diagnostics에 포함하지 않는다.
- Malformed record는 skip한다.
- Skipped record 수를 집계한다.
- Skipped record sample은 제한된 개수만 `metadata.diagnostics`에 기록한다.
- 0건 이상 파싱 가능한 경우 analyzer output은 유효한 결과로 반환한다.
- Strict fail-fast behavior는 이후 명시적 option으로 추가할 수 있지만 Phase 1 기본값은 아니다.

### Fatal Errors

분석을 신뢰성 있게 진행할 수 없는 조건은 run 실패로 처리한다.

| Condition | Policy | Example error code |
|---|---|---|
| Input file이 없거나 읽을 수 없음 | Fatal | `FILE_NOT_READABLE` |
| 지원하지 않는 parser format | Fatal | `UNSUPPORTED_FORMAT` |
| Encoding fallback chain으로 decode 불가 | Fatal | `ENCODING_ERROR` |
| Required analyzer option이 잘못됨 | Fatal | `INVALID_OPTION` |
| Output path에 쓸 수 없음 | Exporter/CLI layer에서 fatal | `OUTPUT_WRITE_ERROR` |
| 예상하지 못한 internal exception | Fatal | `INTERNAL_ERROR` |

### Non-Fatal Record Errors

Record-level error는 skip하고 diagnostics에 보고한다.

| Parser | Malformed condition | Policy | Reason code |
|---|---|---|---|
| Access Log | 선택한 log format과 line이 맞지 않음 | Skip | `NO_FORMAT_MATCH` |
| Access Log | Timestamp parse 실패 | Skip | `INVALID_TIMESTAMP` |
| Access Log | Numeric field 변환 실패 | Skip | `INVALID_NUMBER` |
| Collapsed Profiler | Trailing sample count가 없음 | Skip | `MISSING_SAMPLE_COUNT` |
| Collapsed Profiler | Sample count가 integer가 아님 | Skip | `INVALID_SAMPLE_COUNT` |
| Collapsed Profiler | Sample count가 음수 | Skip | `NEGATIVE_SAMPLE_COUNT` |

### Diagnostics Shape

Parser diagnostics는 `AnalysisResult.metadata.diagnostics` 아래에 기록한다.

초기 shape:

```text
metadata.diagnostics = {
  total_lines: number,
  parsed_records: number,
  skipped_lines: number,
  skipped_by_reason: Record<string, number>,
  samples: [
    {
      line_number: number,
      reason: string,
      message: string,
      raw_preview: string
    }
  ]
}
```

규칙:

- `samples`는 제한한다. 초기값은 20건이다.
- `raw_preview`는 잘라서 기록한다. 초기값은 200자이다.
- 짧은 preview로 충분한 경우 대용량 log record 전문을 diagnostics에 넣지 않는다.
- Parser 내부에서는 더 풍부한 diagnostics를 보유할 수 있지만, 외부 contract는 `metadata.diagnostics`를 기준으로 한다.

### Access Log Policy

- Empty 또는 whitespace-only line은 무시한다.
- 설정된 format과 맞지 않는 line은 `NO_FORMAT_MATCH`로 skip한다.
- Timestamp, status, bytes, response-time 값이 잘못된 line은 구체적인 reason code로 skip한다.
- `metadata.diagnostics.parsed_records`는 analyzer aggregation에 포함된 record 수와 같아야 한다.
- `metadata.diagnostics.skipped_lines`에는 blank ignored line을 포함하지 않는다.

### Collapsed Profiler Policy

- Empty 또는 whitespace-only line은 무시한다.
- Stack과 trailing integer sample count가 없는 line은 skip한다.
- Negative sample count는 fatal이 아니라 skip으로 처리한다.
- 유효한 duplicate stack은 기존처럼 sample count를 합산한다.
- `metadata.diagnostics.parsed_records`는 stack merge 전 유효 collapsed line 수를 의미한다.

### Encoding And Corrupt Input

`iter_text_lines`는 `utf-8`, `utf-8-sig`, `cp949`, `latin-1` 순서의 fallback chain을 사용한다. `latin-1`은 모든 byte sequence를 decode할 수 있으므로, 일부 corrupt byte sequence는 decode failure가 아니라 malformed record로 parser layer에 도달할 수 있다. 정책은 다음과 같다.

- 모든 configured encoding으로 decode에 실패하면 fatal이다.
- Decode는 되었지만 의미적으로 잘못된 record는 non-fatal로 skip하고 diagnostics에 보고한다.
- 향후 binary/corruption detector를 추가할 수 있지만, parser diagnostics를 대체하지 않는다.
