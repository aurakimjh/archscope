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

Parser는 설정이 있을 때만 malformed line을 skip해야 한다. Validation 구현 후에는 skipped line count를 `metadata`에 기록한다.
