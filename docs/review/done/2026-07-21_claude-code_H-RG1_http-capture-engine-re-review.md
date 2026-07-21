# H-RG1 — 오프라인 HAR 분석 엔진 재리뷰 (T-579)

- 검토 ID: H-RG1 엔진 재리뷰 (1차 `CONDITIONAL` → 개선분 독립 재검증)
- 대상 작업: T-579 offline HAR analysis — **엔진(Go) 슬라이스 한정** (UI는 Codex 별도 리뷰)
- 1차 리뷰: `docs/review/done/2026-07-21_claude-code_H-RG1_http-capture-engine-implementation-review.md` (`CONDITIONAL`, P1 2건)
- 개선 커밋: `7e02142 fix(http-capture): remediate H-RG1 engine review`
- 재검토 기준 HEAD: `4c814bb`
- 검토자: Claude Code (독립 재검증)
- 검토일: 2026-07-21

## 종합 판정: `PASS`

1차 리뷰의 **P1 2건·P2 3건·P3 3건이 전부 해소됨을 독립 재현으로 확인**했다. 수정은
근본 원인을 정확히 겨냥했고(에러 무시 → 명시 처리 + 방어심층 recover, 정규식 폴백 →
fail-closed), 각 finding마다 회귀 테스트가 추가되어 재발을 막는다. 신규 P0/P1/P2 결함은
발견되지 않았다.

P2-1(범용 키 과다 리댁션) 수정 과정에서 **좁은 잔여 항목 1건**(하드-시크릿 키 아래의
**숫자** 값이 리댁션에서 제외됨)이 파생되었으나, 이는 1차 리뷰가 권고한 "숫자·불리언은
자격증명이 아니므로 제외" 방향을 그대로 따른 결과이며, 문자열 비밀·유효 카드번호는 여전히
차단된다. 게이트 차단 사유가 아닌 **P3(비차단) 정제 권고**로 기록한다.

검증 근거: 대상 4개 패키지 `go test` 통과, 엔진 모듈 `go test ./...`·`go build ./...`·
`go vet` 통과. 아래 각 finding은 1차 패닉/누출을 재현하던 입력을 그대로 다시 넣어
독립 프로브로 확인했다(프로브는 커밋하지 않음).

---

## 1차 Findings 해소 검증

### P1-1 — 손상 요청 URL nil 포인터 패닉 → **RESOLVED**

- **수정:** `mapEntry`가 `url.Parse` 에러를 명시 처리 — 실패 시 `parsedURL = &url.URL{}`로
  대체(parser.go:398–401)해 nil 역참조를 제거. 추가로 `ParseFile` 진입부에
  `defer recoverParsePanic(...)` 방어심층 경계(parser.go:170, 621–629)를 두어 예기치 못한
  패닉을 **비누출** 구조 에러로 강등한다. 손상 URL은 `HAR_URL_UNPARSABLE` 진단과 함께
  리댁션된 원문만 보존(parser.go:33, 300–302, 409).
- **부가 강화(권고 반영 이상):** `RedactURL` 에러 경로에 `urlUserInfoPattern`을 추가해
  파싱 불가 URL의 `user:secret@` 자격증명까지 리댁션(redact.go:68, 126–131). 1차에는
  이 경로에서 userinfo가 새어나갈 여지가 있었는데 함께 닫혔다.
- **독립 재현:** 1차 패닉 입력 `http://ex/%GG`와 변형 4종(`user:SECRET@[::1`, 제어문자,
  `%ZZ`+userinfo+query secret)을 `ParseFile`에 투입 →
  - 패닉 없음, `err == nil`, 엔트리 정상 1건 반환.
  - 최종 URL: `http://[REDACTED]@[::1`, `.../%ZZ?api_key=[REDACTED]` 등 — `URL-USERINFO-SECRET`,
    `CTRL-SECRET`, `PW-SECRET`, `Q-SECRET` 전부 미유출.
- **회귀 테스트:** `TestParseHARHandlesUnparseableURLsWithoutPanic`(3케이스),
  `TestParseFilePanicRecoveryReturnsStructuralErrorWithoutPanicValue`(패닉 값 미누출까지 단언).

### P1-2 — 대용량 JSON 바디 리댁션 정규식 폴백 누출 → **RESOLVED**

- **수정:** `RedactBody`가 JSON 바디 && `len(text) > maxScanBytes`이면 절단·정규식 폴백
  대신 **fail-closed** — `"[REDACTED_OVERSIZED_JSON]"` 반환 + `HAR_REDACTION_DEGRADED`
  경고 + `oversized_json_body` 카운트(redact.go:191–197). 정규식이 따옴표 감싼 JSON
  키/값을 못 잡던 강등 경로 자체가 제거됨.
- **독립 재현:** 1차 누출 입력(민감 키를 스캔창 앞쪽에 두고 1 MiB 초과 패딩) 재투입 →
  결과 `[REDACTED_OVERSIZED_JSON]`, `OPAQUE-SECRET-TOKEN` 완전 소거.
- **회귀 테스트:** `TestOversizedJSONBodyFailsClosed`(억제·카운트·경고 전부 단언).

### P2-1 — `code`/`auth`/`session` 등 범용 키 과다 리댁션 → **RESOLVED (좁은 잔여 → P3-R1)**

- **수정:** `redactJSON`이 `sensitiveKey(k) && redactWholeJSONValue(child)`일 때만 통째
  치환(redact.go:257). `redactWholeJSONValue`는 `bool`/`float64`/`json.Number`에 대해
  false를 반환(redact.go:427–434)하므로 숫자·불리언 분석 필드는 타입 보존된다.
- **독립 재현:** `{"code":200,"auth":false,"session":123,"token":"S1","nested":{"code":"S2"}}`
  → `{"auth":false,"code":200,"session":123,"nested":{"code":"[REDACTED]"},"token":"[REDACTED]"}`.
  숫자/불리언 보존, **문자열 비밀은 중첩 포함 전부 리댁션**. 숫자→문자열 강제 변환도 사라짐.
- **회귀 테스트:** `TestStructuredJSONPreservesNonStringCodeAuthAndSessionValues`.

### P2-2 — 파서 패닉 복구 경계 부재 → **RESOLVED**

- **수정:** `recoverParsePanic`(parser.go:621–629)가 어떤 패닉도 `ErrStructuralHAR` 기반
  구조 에러로 강등하고, **패닉 값을 에러 문자열에 싣지 않는다**(정보 누출 방지). P1-1의
  구조적 수정과 독립적으로 동작하는 이중 방어.
- **재현:** 위 P1-1 프로브에서 recover 경로 정상 동작 확인, 회귀 테스트가 패닉 값 미누출 단언.

### P2-3 — 자원 상한 테스트 공백(MaxBytes 평문·MaxFields·손상 입력) → **RESOLVED**

- **수정:** `guard_test.go`에 `plain-byte-cap`(MaxBytes:32), `field-cap`(MaxFields:2) 케이스
  추가(guard_test.go:20–21). 손상 URL/음수 timings는 `parser_test.go`의
  `TestParseHARHandlesUnparseableURLs...` / `TestParseHARReportsMalformedNegativeTimings`로
  커버.
- **확인:** 두 신규 가드 케이스 모두 "부분 성공 없이 거부" 단언을 통과.

### P3-1 — `bodyStorage` 도달 불가 분기 → **RESOLVED**

- **수정:** `bodyStorage(preview, redacted)`로 시그니처 단순화, 중복 `return "omitted"`
  죽은 분기 제거(parser.go:546–557). 호출부도 정리(parser.go:423, 452).

### P3-2 — 요청 `BodyDecoded` 의미 비대칭 → **RESOLVED**

- **수정:** 요청 `BodyDecoded`를 `unknownSize()`(-1)로 변경(parser.go:424) — HAR postData에
  디코딩 크기 소스가 없으므로 "미상"이 정직. 회귀 테스트가 `BodyDecoded == -1` 단언
  (parser_test.go).

### P3-3 — 인라인 프리뷰 UTF-8 룬 경계 무시 → **RESOLVED**

- **수정:** `inlinePreview`가 절단 후 `utf8.ValidString`이 참이 될 때까지 뒤로 물러남
  (parser.go:561–570). 회귀 테스트 `TestInlinePreviewPreservesUTF8Boundary`가 이모지
  경계에서 유효 UTF-8 유지 확인.

---

## 잔여 항목 (비차단)

### P3-R1 — 하드-시크릿 키 아래 **숫자** 값이 리댁션에서 제외됨 (P2-1 수정의 파생)

- **근거:** `redactWholeJSONValue`가 모든 민감 키에 일괄 적용되어, `password`/`ssn`/`token`
  등 강한 시크릿 키의 값이 JSON **숫자**면 리댁션되지 않는다(redact.go:257, 427–434).
- **독립 재현:** `{"ssn":123456789,"password":98765,"creditCard":4111111111111111,"apiKey":42}`
  → `{"apiKey":42,"creditCard":[REDACTED_CARD],"password":98765,"ssn":123456789}`.
  즉 **숫자 SSN·숫자 password는 생존**한다. (단, 유효 카드번호는 리댁션 후 텍스트 패스의
  Luhn 카드 규칙이 여전히 `[REDACTED_CARD]`로 차단한다.)
- **평가:** 이 동작은 1차 리뷰 P2-1 권고("숫자·불리언은 자격증명이 아님 → 문자열일 때만
  리댁션")를 충실히 따른 결과다. 실무에서 토큰·비밀번호·SSN은 대개 문자열(선행 0 보존
  때문)이며 문자열 경로는 완전히 차단되므로 **일반 케이스는 안전**하다. 다만 숫자 PIN·
  숫자 SSN 같은 경계 사례가 남는다.
- **권고(비차단):** 숫자 제외를 **모호 분석 키(`code`/`auth`/`session`)에만** 한정하고,
  하드-시크릿 키(`password`/`passwd`/`pwd`/`ssn`/`secret`/`token`류/`apikey`/`signature`/
  `privatekey`)는 값 타입과 무관하게 통째 리댁션하도록 `redactWholeJSONValue`를 키 분류와
  결합하면 잔여 표면이 닫힌다. 회귀 테스트에 숫자 SSN/PIN 케이스 추가 권장. 게이트
  `PASS`를 막지 않는다.

---

## 재검증 실행 근거

- `go test ./internal/parsers/httpcapture/... ./internal/capture/... ./internal/analyzers/httpcapture/...` → **통과**
- 엔진 모듈 전체 `go test ./...` → **통과** (macOS SDK 링커 버전 경고만 존재, 테스트 실패 없음)
- `go build ./...`, `go vet ./...` (대상 패키지) → **통과**
- 독립 프로브 3종(손상 URL 무패닉·무누출 / 대용량 JSON fail-closed / 숫자·문자열 혼합 및
  하드키 숫자) → 위 판정대로 확인 (임시 테스트, 미커밋)

---

## 게이트 결론

1차 리뷰의 **모든 findings(P1×2, P2×3, P3×3)가 독립적으로 해소 확인**되었고 신규 차단
결함이 없으므로, **H-RG1 엔진 슬라이스는 `PASS`**로 전환한다.

- 남은 P3-R1은 문서화된 비차단 정제 항목이며 후속 커밋에서 처리 가능하다.
- H-RG1 그룹 전체 `PASS`는 본 엔진 `PASS` 외에 **Codex UI 재리뷰 `PASS`**와
  **H-SEC1 재리뷰 `PASS`**, 그리고 전체 수직 슬라이스 그룹 리뷰가 모두 충족되어야 성립한다
  (work_status.md 기준). 본 문서는 그중 **엔진 축의 `PASS`**를 확정한다.
