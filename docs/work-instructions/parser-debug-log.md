# 작업 지시서: 파서 디버그 로그 파일 생성

## 배경

ArchScope가 실전에 나가면 다양한 형태의 로그를 만나게 된다. nginx 버전별 포맷 차이, 커스텀 로그 포맷, 예상 못한 인코딩, 비정상 JFR 출력 등 — 파서가 처리하지 못하는 케이스는 반드시 발생한다.

현재 파싱 오류 정보는 `AnalysisResult.metadata.diagnostics`에 포함되어 UI까지 전달되지만, **개발자가 코드를 수정하기 위한 용도로는 부족하다**:

- 결과 JSON 안에 묻혀 있어 추출이 번거로움
- 샘플 20건 제한, raw_preview 200자 제한으로 정보 손실
- 예외 스택트레이스가 누락됨
- JFR 파서는 진단 자체가 없음
- 어떤 정규식/포맷이 실패했는지 알 수 없음
- 파서 버전, 설정, 환경 정보 없음

**요구사항**: 파싱 오류 발생 시 개발자가 **이 파일 하나만 받으면 바로 코드 수정에 착수**할 수 있는 디버그 로그를 남긴다.

---

## 목표

파서 실행 중 발생한 모든 오류를 **단일 파일**에 구조화하여 기록한다. 원본 로그 파일 전체는 필요 없다. 오류 종류별로 구분되고, 재현과 수정에 필요한 최소 컨텍스트를 포함한다.

---

## 현재 상태 분석

### 기존 진단 시스템

```
ParserDiagnostics (common/diagnostics.py)
├── total_lines: int
├── parsed_records: int
├── skipped_lines: int
├── skipped_by_reason: {reason_code: count}
└── samples: [{line_number, reason, message, raw_preview(200자)}]  ← 최대 20건
```

### 파서별 에러 코드 현황

| 파서 | 에러 코드 | 진단 통합 |
|---|---|---|
| access_log | `NO_FORMAT_MATCH`, `INVALID_TIMESTAMP`, `INVALID_NUMBER` | ✅ |
| collapsed | `MISSING_SAMPLE_COUNT`, `INVALID_SAMPLE_COUNT`, `NEGATIVE_SAMPLE_COUNT` | ✅ |
| exception | `NO_EXCEPTION_HEADER`, `INVALID_EXCEPTION_BLOCK` | ✅ |
| gc_log | `NO_GC_FORMAT_MATCH` | ✅ |
| thread_dump | `OUTSIDE_THREAD_BLOCK`, `INVALID_THREAD_BLOCK` | ✅ |
| jennifer_csv | `INVALID_JENNIFER_ROW` | ✅ |
| jfr | ValueError 예외만 발생 | ❌ — 진단 미통합 |

### 갭 요약

| 현재 상태 | 개발자에게 필요한 것 |
|---|---|
| 샘플 20건 제한 | 오류 유형별 대표 샘플 (유형당 충분한 양) |
| raw_preview 200자 | 정규식 디버깅에 충분한 원본 컨텍스트 (전후 라인 포함) |
| reason + message만 존재 | 실패한 정규식 이름, 부분 매치 정보, 예외 스택트레이스 |
| 결과 JSON 안에 매장 | 별도 독립 파일로 즉시 접근 가능 |
| 환경 정보 없음 | Python 버전, ArchScope 버전, OS, 파서 설정 |
| JFR 진단 없음 | 모든 파서에서 일관된 진단 |

---

## 설계

### 디버그 로그 파일 형식

**JSON** — 사람이 읽을 수 있고, 도구로 파싱도 가능.

### 파일명 규칙

```
archscope-debug-{analyzer_type}-{timestamp}.json
```

예시: `archscope-debug-access_log-20260430T143022.json`

### 생성 조건

다음 중 하나라도 해당하면 생성:

1. `skipped_lines > 0` — 파싱 실패 라인이 있음
2. 분석 중 예외(exception)가 발생했으나 부분 결과는 반환 가능
3. CLI 플래그 `--debug-log` 명시 시 항상 생성 (오류 없어도)

오류가 전혀 없고 `--debug-log`가 없으면 파일을 생성하지 않는다.

### 저장 위치

1. 기본값: ArchScope 실행 위치 하위 `archscope-debug/` 디렉터리에 저장
2. CLI 옵션 `--debug-log-dir <path>`로 변경 가능
3. Electron portable 앱에서는 실행 파일 위치 기준 `archscope-debug/`
4. 개발 모드에서는 repository root 또는 current working directory 기준 `archscope-debug/`
5. 입력 파일 디렉터리에는 기본 저장하지 않음

Portable 실행 파일을 현장에서 그대로 전달/회수하는 운영 방식을 우선한다. 디버그 로그도 실행 파일 옆에 남아야 사용자가 원본 운영 로그 위치를 건드리지 않고 `archscope-debug/` 폴더만 압축해 전달할 수 있다.

### 민감정보 마스킹 원칙

디버그 로그는 현장 밖으로 전달될 가능성이 높으므로 **redaction은 기본 활성화**한다. 단, 파서 수정에 필요한 구조 정보는 보존해야 한다.

보존해야 하는 정보:

- 라인/row/event 번호
- 오류 reason code와 message
- parser/analyzer type, parser option, failed pattern 이름
- 구분자, quote, bracket, field 순서, field 개수
- timestamp 형식, HTTP method, status, byte/latency 숫자 형식
- captured group 이름과 성공/실패 위치
- 값의 type, 길이, shape 정보

마스킹해야 하는 정보:

- Authorization, Cookie, Set-Cookie, API key, token, secret, password
- URL query string 값
- 이메일, 전화번호, 주민/계정/고객 식별자로 보이는 긴 숫자열
- 내부 호스트명, absolute source path의 사용자/조직 디렉터리
- exception message 안의 credential-like value

권장 redaction 방식:

```text
원본 값 자체는 숨기되, 파서가 실패한 이유를 알 수 있도록 token shape를 남긴다.
예: Authorization: Bearer abc... -> Authorization: Bearer <TOKEN len=43>
예: /api/orders?customerId=12345&token=abc -> /api/orders?customerId=<NUMBER len=5>&token=<TOKEN len=3>
예: user@example.com -> <EMAIL>
예: /Users/acme/prod/access.log -> <PATH>/access.log
```

마스킹된 raw context는 실제 원문과 byte-for-byte 동일하지 않다. 따라서 디버그 로그에는 `redacted: true`, `redaction_version`, `redaction_summary`, `field_shapes`를 함께 기록한다. 정규식 재현이 필요한 경우에는 원문 대신 **구조화된 field shape**를 기준으로 테스트 케이스를 만든다.

---

## 파일 구조

```jsonc
{
  // ── 1. 환경 정보 ──
  "environment": {
    "archscope_version": "0.5.0",
    "python_version": "3.12.3",
    "os": "Darwin 25.4.0",
    "timestamp": "2026-04-30T14:30:22Z"
  },

  // ── 2. 실행 컨텍스트 ──
  "context": {
    "analyzer_type": "access_log",
    "source_file": "<PATH>/access.log",
    "source_file_name": "access.log",
    "file_size_bytes": 1048576,
    "encoding_detected": "utf-8",
    "parser": "nginx_access_log",
    "parser_options": {
      "format": "nginx",
      "max_lines": 100000,
      "start_time": null,
      "end_time": null
    },
    "debug_log_dir": "./archscope-debug"
  },

  // ── 2-1. 마스킹 정보 ──
  "redaction": {
    "enabled": true,
    "redaction_version": "0.1.0",
    "raw_context_redacted": true,
    "summary": {
      "URL_QUERY_VALUE": 12,
      "TOKEN": 3,
      "EMAIL": 1,
      "ABSOLUTE_PATH": 1
    }
  },

  // ── 3. 요약 ──
  "summary": {
    "total_lines": 50000,
    "parsed_ok": 49823,
    "skipped": 177,
    "skip_rate_percent": 0.35,
    "error_types": {
      "NO_FORMAT_MATCH": 150,
      "INVALID_TIMESTAMP": 20,
      "INVALID_NUMBER": 7
    },
    "exceptions": 0,
    "verdict": "PARTIAL_SUCCESS"
    // verdict: "CLEAN" | "PARTIAL_SUCCESS" | "MAJORITY_FAILED" | "FATAL_ERROR"
  },

  // ── 4. 오류 유형별 상세 ──
  "errors_by_type": {
    "NO_FORMAT_MATCH": {
      "count": 150,
      "description": "Line does not match nginx access log format.",
      "failed_pattern": "NGINX_WITH_RESPONSE_TIME",
      "samples": [
        {
          "line_number": 42,
          "raw_context": {
            "before": "<IPV4> - - [30/Apr/2026:14:30:00 +0900] \"GET /api/health HTTP/1.1\" 200 15 0.001",
            "target": "이 라인은 완전히 다른 포맷입니다 — Apache combined 형식",
            "after": "<IPV4> - - [30/Apr/2026:14:30:01 +0900] \"POST /api/data HTTP/1.1\" 201 42 0.023"
          },
          "field_shapes": {
            "target_token_count": 8,
            "quote_count": 0,
            "bracket_count": 0,
            "looks_like": "plain_text_or_unexpected_format"
          },
          "partial_match": null,
          "message": "Line does not match nginx access log format."
        },
        {
          "line_number": 1337,
          "raw_context": {
            "before": "...",
            "target": "<IPV4> - <USER> [30/Apr/2026:14:30:05 +0900] \"GET /status HTTP/2\" 200 512 \"-\" \"curl/8.1\" rt=0.002",
            "after": "..."
          },
          "field_shapes": {
            "target_token_count": 11,
            "quote_count": 6,
            "bracket_count": 2,
            "suffix_shape": "key=value"
          },
          "partial_match": {
            "matched_up_to": "status",
            "remaining": "rt=0.002"
          },
          "message": "Line does not match nginx access log format."
        }
      ]
    },
    "INVALID_TIMESTAMP": {
      "count": 20,
      "description": "Timestamp does not match nginx format.",
      "samples": [
        {
          "line_number": 88,
          "raw_context": {
            "before": "...",
            "target": "<IPV4> - - [2026-04-30 14:30:00] \"GET / HTTP/1.1\" 200 1024 0.005",
            "after": "..."
          },
          "field_shapes": {
            "timestamp_shape": "yyyy-MM-dd HH:mm:ss",
            "request_shape": "METHOD PATH PROTOCOL"
          },
          "partial_match": {
            "matched_up_to": "timestamp_raw",
            "captured_value": "2026-04-30 14:30:00"
          },
          "message": "Timestamp does not match nginx format. Expected: dd/Mon/YYYY:HH:MM:SS +ZZZZ"
        }
      ]
    }
  },

  // ── 5. 예외 (비정상 종료/크래시) ──
  "exceptions": [
    {
      "phase": "parsing",
      "line_number": 25000,
      "exception_type": "UnicodeDecodeError",
      "message": "'utf-8' codec can't decode byte 0x80 in position 12: invalid start byte",
      "traceback": "Traceback (most recent call last):\n  File \"access_log_parser.py\", line 45, in ...\n  ...",
      "raw_context": {
        "before": "...",
        "target": "(binary content, hex: 48 54 54 50 2F 31 2E 31 80 ...)",
        "after": "..."
      }
    }
  ],

  // ── 6. 개발자 힌트 ──
  "hints": [
    "NO_FORMAT_MATCH가 150건으로 전체 오류의 85%. 주요 패턴: Apache combined 형식 라인 혼재 → 멀티포맷 감지 또는 포맷 옵션 확인 필요",
    "INVALID_TIMESTAMP 20건: ISO 8601 형식 타임스탬프 발견. nginx 기본 포맷과 다른 log_format 설정 추정"
  ]
}
```

---

## 구현 범위

### 변경 대상

| 파일 | 변경 내용 |
|---|---|
| `common/diagnostics.py` | 디버그 로그 수집기 확장: 전후 라인 컨텍스트, 부분 매치 정보, 예외 수집 |
| `common/redaction.py` | 디버그 로그용 민감정보 마스킹: token, cookie, query value, email, absolute path, long identifier |
| 모든 파서 (`parsers/*.py`) | 실패 시 부분 매치 정보 반환 (어디까지 매치되었는가) |
| `jfr_parser.py` | ParserDiagnostics 통합 (현재 미통합) |
| `analyzers/*.py` | 디버그 로그 생성 트리거, 예외 캐치 및 수집 |
| `cli.py` | `--debug-log`, `--debug-log-dir` 옵션 추가 |
| `main.ts` | Electron 앱에서 디버그 로그 저장 경로 관리 |

### 변경하지 않는 것

- 기존 `AnalysisResult.metadata.diagnostics` 구조는 유지 (하위 호환)
- 기존 파서 동작(skip 정책)은 변경하지 않음
- UI 변경 없음 (이 작업은 엔진 레이어)
- 원본 로그 파일 전체 복사 없음
- 마스킹되지 않은 원문 raw context 저장 없음

---

## 핵심 설계 결정

### D-1: 전후 라인 컨텍스트

오류 라인만으로는 부족하다. **앞뒤 1줄**을 함께 기록하면:
- 정상 라인과 비정상 라인의 차이를 즉시 비교 가능
- 포맷 전환점(로그 로테이션, 포맷 변경) 감지 가능
- 멀티라인 로그(스택트레이스)의 컨텍스트 유지

```python
raw_context = {
    "before": lines[i-1] if i > 0 else None,   # 앞 1줄
    "target": lines[i],                          # 오류 라인
    "after": lines[i+1] if i < len(lines)-1 else None  # 뒤 1줄
}
```

각 라인은 **500자**까지 기록. raw_preview(200자)보다 넉넉하게.

raw context는 저장 직전에 redaction을 통과한다. Redaction은 quote, bracket, whitespace, delimiter를 최대한 보존해야 하며, 민감값만 `<TOKEN len=...>` 같은 placeholder로 대체한다. 이 방식이면 원문을 보지 않아도 “필드가 하나 더 있다”, “query suffix가 있다”, “timestamp shape가 다르다” 같은 parser 수정 단서를 유지할 수 있다.

### D-2: 오류 유형별 샘플 수 제한

유형별로 **최대 5건**의 대표 샘플을 기록한다.

- 전체 제한(현재 20건)이 아니라 유형별 제한으로 변경
- 다양한 오류 유형이 있을 때 특정 유형에 샘플이 편중되지 않음
- 유형당 5건이면 패턴 파악에 충분

### D-3: 부분 매치 정보 (선택적)

정규식 매치 실패 시 **어디까지 매치되었는가**를 기록할 수 있으면 기록한다.

```python
partial_match = {
    "matched_up_to": "status",         # 마지막으로 성공한 캡처 그룹
    "captured_value": "abc"            # 실패한 필드의 원본 값 (있다면)
}
```

이것이 있으면 개발자가 "정규식의 어느 부분이 실패했는지"를 즉시 파악할 수 있다. 정규식 전체를 단계별로 매치하는 것은 비용이 크므로, **이미 캡처된 그룹 정보를 활용**하는 수준에서 구현한다.

### D-4: verdict (종합 판정)

개발자가 파일을 열자마자 심각도를 파악할 수 있도록:

| verdict | 조건 | 의미 |
|---|---|---|
| `CLEAN` | skipped = 0, exceptions = 0 | 오류 없음 (--debug-log 플래그로만 생성) |
| `PARTIAL_SUCCESS` | skip_rate < 50% | 일부 라인 실패, 대부분 정상 |
| `MAJORITY_FAILED` | skip_rate >= 50% | 대부분 실패 — 포맷 불일치 가능성 |
| `FATAL_ERROR` | 분석 자체가 예외로 중단 | 크래시 |

### D-5: 개발자 힌트 자동 생성

단순 규칙 기반으로 힌트를 자동 생성한다:

- `NO_FORMAT_MATCH`가 전체 오류의 80% 이상 → "입력 파일이 설정된 포맷과 다를 수 있음"
- `INVALID_TIMESTAMP` 다수 → "타임스탬프 형식이 파서 기대와 다름"
- skip_rate > 50% → "파일 포맷 자체가 지원 대상이 아닐 가능성"
- 예외 발생 → "비정상 종료 — traceback 확인 필요"

### D-6: 파일 크기 제한

디버그 로그 파일 자체가 과도하게 커지지 않도록:
- 유형당 샘플 5건 × 오류 유형 수(보통 3-5개) = 15-25건 정도
- 각 샘플의 raw_context 500자 × 3줄 = 1,500자
- 전체 파일 크기 상한: **1MB** — 초과 시 오래된 샘플부터 제거

### D-7: 마스킹과 분석 가능성의 균형

마스킹은 값의 의미를 지우되, 파싱 오류 분석에 필요한 구조를 지우면 안 된다.

| 항목 | 저장 방식 | 이유 |
|---|---|---|
| HTTP method/status/bytes/latency | 원문 유지 | 포맷 판별과 numeric parse 실패 분석에 필요 |
| timestamp | 원문 유지 | timestamp parser 수정에 필요 |
| URL path | path segment는 유지하되 식별자 segment는 shape로 치환 | route shape 분석에 필요 |
| URL query value | value만 `<QUERY_VALUE type=... len=...>`로 치환 | query delimiter/parameter 존재 여부는 필요 |
| Authorization/Cookie/token | 전체 secret value 치환 | 보안 위험이 큼 |
| IP/host/user/email | 기본 치환 | 현장 식별자 노출 방지 |
| exception type/stack frame | 원문 유지 | parser/analyzer 수정에 필요 |
| exception message | credential-like token만 치환 | 원인 분석 문맥 보존 |

예시:

```text
원본:
10.0.0.1 - admin [30/Apr/2026:14:30:05 +0900] "GET /api/order/12345?token=abcd HTTP/2" 200 512 "-" "curl/8.1" rt=0.002

저장:
<IPV4> - <USER> [30/Apr/2026:14:30:05 +0900] "GET /api/order/<NUMBER len=5>?token=<TOKEN len=4> HTTP/2" 200 512 "-" "curl/8.1" rt=0.002

field_shapes:
{
  "path_shape": "/api/order/<NUMBER>",
  "query_keys": ["token"],
  "request_shape": "METHOD PATH_WITH_QUERY PROTOCOL",
  "suffix_shape": "key=value"
}
```

---

## 구현 순서

### Step 1: 디버그 로그 수집기 구현

`common/debug_log.py` 신규 생성.

```python
@dataclass
class DebugLogCollector:
    """분석 실행 중 파싱 오류를 수집하여 디버그 로그 파일로 내보낸다."""

    def add_parse_error(self, *, line_number, reason, message, raw_context, partial_match=None): ...
    def add_exception(self, *, phase, line_number, exception, raw_context): ...
    def set_context(self, *, analyzer_type, source_file, parser, parser_options): ...
    def set_redaction(self, *, enabled=True, redaction_version="0.1.0"): ...
    def write(self, path: Path) -> None: ...
    def should_write(self) -> bool: ...
```

기존 `ParserDiagnostics`와 **독립적으로 동작**한다. ParserDiagnostics는 결과 JSON에 들어가는 요약용이고, DebugLogCollector는 개발자용 상세 파일이다. 둘 다 파서에서 동시에 호출된다.

### Step 2: 파서 통합

모든 파서의 `add_skipped()` 호출 지점에서 `DebugLogCollector.add_parse_error()`도 함께 호출.

- 전후 라인 컨텍스트를 전달하기 위해 파서가 이전/다음 라인에 접근할 수 있어야 함
- `iter_text_lines` 또는 파서 루프에서 sliding window(크기 3) 유지
- raw context 저장 전 `common/redaction.py`를 반드시 통과
- parser별로 가능한 경우 `field_shapes`와 `partial_match`를 함께 기록

### Step 3: JFR 파서 진단 통합

`jfr_parser.py`에 ParserDiagnostics + DebugLogCollector 통합. 현재 ValueError로만 실패하는 부분을 진단 샘플로 변환.

### Step 4: 예외 수집

분석기(analyzer) 레벨에서 try-except로 파서 예외를 잡고, DebugLogCollector에 기록한 후 적절히 re-raise 또는 부분 결과 반환.

### Step 5: CLI 연동

```bash
# 오류 시 자동 생성 (기본 동작)
archscope-engine access-log analyze --file /var/log/access.log --out result.json

# 디버그 로그 강제 생성
archscope-engine access-log analyze --file /var/log/access.log --out result.json --debug-log

# 저장 위치 지정
archscope-engine access-log analyze --file /var/log/access.log --out result.json --debug-log-dir /tmp/debug/
```

### Step 6: Electron 연동

`main.ts`에서 엔진 실행 후 디버그 로그 파일 경로를 결과와 함께 반환. UI에서 "디버그 로그 저장됨" 알림 (향후).

---

## 테스트

| 테스트 | 검증 내용 |
|---|---|
| 오류 없는 파일 분석 시 디버그 로그 미생성 | `should_write()` = False |
| `--debug-log` 플래그 시 항상 생성 | verdict = "CLEAN" |
| 다양한 오류 유형이 각각 구분되어 기록 | `errors_by_type` 키 확인 |
| 유형당 샘플 5건 제한 | 6건째부터 무시 |
| 전후 라인 컨텍스트 정확성 | before/target/after 라인 번호 매칭 |
| 민감정보 기본 마스킹 | token, cookie, query value, email, absolute path가 원문으로 남지 않음 |
| 분석 가능 구조 보존 | delimiter, quote, timestamp, status, numeric shape, field count가 유지됨 |
| portable 저장 위치 | 기본 debug log path가 실행 위치 하위 `archscope-debug/`인지 확인 |
| 예외 발생 시 traceback 기록 | `exceptions` 배열 확인 |
| 1MB 크기 제한 | 대용량 오류 파일에서 파일 크기 검증 |
| JFR 파서 진단 통합 | JFR 분석 시 디버그 로그 생성 확인 |
| 파일명에 타임스탬프와 분석기 타입 포함 | 파일명 패턴 검증 |

---

## 범위 외 (이번 작업에 포함하지 않음)

- UI에서 디버그 로그를 열거나 시각화하는 기능
- 디버그 로그를 원격으로 전송하는 기능
- 원본 로그 파일의 복사/첨부
- 사용자가 명시적으로 요청하지 않은 unredacted debug log 생성
- 기존 `ParserDiagnostics` 구조 변경 (하위 호환 유지)
- AI 해석 관련 디버그 로그 (Phase 5 별도)

---

## 성공 기준

1. **개발자가 디버그 로그 파일 하나만 받으면**, 어떤 파서에서, 어떤 오류 유형이, 몇 건 발생했고, 대표적으로 어떤 라인이 실패했는지 30초 안에 파악할 수 있다
2. 디버그 로그는 portable 실행 위치 하위 `archscope-debug/`에 남아 현장에서 폴더째 회수할 수 있다
3. 디버그 로그에는 민감 원문이 남지 않지만, 파서 수정에 필요한 구조 정보와 실패 위치는 보존된다
4. 정상 분석 시 **성능 영향 최소화** — collector가 필요할 때만 context capture와 JSON 직렬화를 수행한다
5. 오류 발생 시 **성능 영향 미미** — 오류당 전후 라인 캡처와 JSON 직렬화 비용만 추가
6. 기존 `AnalysisResult.metadata.diagnostics`와 **하위 호환** 유지
