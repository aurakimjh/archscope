# H-RG1 — 오프라인 HAR 분석 엔진 구현 리뷰 (T-579)

- 검토 ID: H-RG1 (엔진 구현 게이트, work_status.md §"Browser Profile / HTTP Capture Review Groups")
- 대상 작업: T-579 offline HAR analysis completion — **엔진(Go) 슬라이스 한정**
- UI 슬라이스(Wails/프런트엔드)는 별도로 Codex가 검토하므로 본 문서 범위 제외.
- 검토자: Claude Code (독립 구현 검증)
- 검토일: 2026-07-21
- 선행 게이트: H-SEC1(보안 독립 검증, `CONDITIONAL`) — 보안 계약 반영 여부를 본 리뷰에서 교차 확인.

## 검토 대상 코드

| 영역 | 파일 |
|---|---|
| HAR 파서 | `apps/engine-native/internal/parsers/httpcapture/parser.go` |
| 다이얼렉트/타이밍 정규화 | `apps/engine-native/internal/parsers/httpcapture/dialect.go` |
| 자원 상한/가드 | `apps/engine-native/internal/parsers/httpcapture/guard.go` |
| 리댁션 | `apps/engine-native/internal/capture/redact/redact.go` |
| 집계 코어 | `apps/engine-native/internal/capture/aggregate/aggregate.go` |
| 분석 파이프라인 | `apps/engine-native/internal/analyzers/httpcapture/analyzer.go` |
| 데이터 모델 | `apps/engine-native/internal/models/http_capture.go` |
| 테스트 | 위 각 패키지의 `*_test.go` + `manifest_test.go` |

## 검토 방법

각 방어선(자원 상한 → 구조 검증 → 정규화 → 리댁션 → 집계/분석)을 계약별로 재구성하고,
**신뢰할 수 없는 입력**(임의 HAR)을 전제로 파서를 적대적으로 대조했다. 의심 지점은 임시
프로브 테스트로 실제 재현하여 검증했다(임시 테스트는 커밋하지 않음). H-SEC1이 지적한
리댁션·자원 계약의 반영 여부를 함께 확인했다.

---

## 종합 판정: `CONDITIONAL`

엔진의 아키텍처 골격은 견고하다. 파서·리댁션·집계·분석의 책임이 깨끗하게 분리되어 있고,
공통 `models.CaptureTransaction`/`AnalysisResult` 계약이 보존된다. 자원 상한(가드)은 파싱
**이전**에 압축폭탄·엔트리 수·깊이·문자열·바디 크기를 모두 거부하며(SEC-4), 구조 손상은
부분 성공 없이 fatal 중단한다(SEC-5). 리댁션은 파서에 무조건 배선되어 비활성 경로가 없고,
H-SEC1 P2-5(자유텍스트 denylist 불일치)는 이미 반영되었다(작업 트리 기준).

**그러나 P1(중대) 2건이 열려 있다.** 두 건 모두 명시된 계약을 신뢰 불가 입력에서 실제로
위반하며, 프로브 테스트로 재현했다.

1. **P1-1** — 단일 손상 URL 하나로 엔진 전체가 nil 포인터 패닉(가용성 계약 위반).
2. **P1-2** — 1 MiB 초과 JSON 바디에서 구조적 리댁션이 정규식 폴백으로 조용히 강등되어
   재사용 가능한 자격증명이 살아남음(SEC-1 위반).

이 2건이 닫히면 게이트는 `PASS` 전환 가능하다. P0(치명·데이터 파괴·RCE급)은 없다.

### 범위별 요약 판정

| # | 항목 | 판정 | 핵심 |
|---|---|---|---|
| 1 | HAR 파서 정확성 | **결함 있음** | 다이얼렉트/타이밍/구조 검증 우수. **P1-1 손상 URL 패닉** |
| 2 | 리댁션 | **결함 있음** | 필드-인지 리댁션·기본 켜짐 우수. **P1-2 대용량 JSON 폴백 누출**, P2-1 과다 리댁션 |
| 3 | 자원 상한/가드 | 양호 | 파싱 전 전량 거부. 반면 일부 상한 테스트 미커버(P2-3) |
| 4 | 데이터 모델/계약 | 양호 | 공통 계약 보존, 스키마 버저닝. P3-2 |
| 5 | 분석 파이프라인 | 양호 | 결정적·유계 집계, 라이브 패리티 테스트 우수 |
| 6 | 보안 계약 준수 | 대체로 반영 | P2-5 반영. SEC-1은 P1-2로 부분 미달 |
| 7 | 테스트 커버리지 | 보완 필요 | 핵심 케이스 강함. **손상 URL·대용량 바디·MaxBytes/MaxFields 미커버(P2-3)** |

---

## Findings

### P0

없음.

---

### P1-1 — 손상된 요청 URL 하나로 파서가 nil 포인터 패닉 (가용성 계약 위반)

- **근거:** `mapEntry`는 리댁션된 URL을 `url.Parse`로 재파싱하고 결과를 **에러 무시**로
  받는다 — `parsedURL, _ := url.Parse(redactedURL)` (parser.go:333). 이후
  `parsedURL.Scheme`/`.Host`/`.EscapedPath()`/`.RawQuery`를 무조건 역참조한다
  (parser.go:367–370). `url.Parse`는 잘못된 퍼센트 이스케이프(`%GG`), 미완결 IPv6
  호스트(`http://[::1`), 제어 문자 등에서 **`(nil, err)`를 반환**하므로, 그런 URL이 하나라도
  있으면 `parsedURL`은 nil이고 첫 역참조에서 SIGSEGV가 발생한다.
- **재현:** `request.url`을 `"http://ex/%GG"`로 둔 최소 HAR 1엔트리를 `ParseFile`에 넣으면
  즉시 패닉:
  ```
  panic: runtime error: invalid memory address or nil pointer dereference
  net/url.(*URL).EscapedPath(...) url.go:687
  httpcapture.mapEntry(...) parser.go:369
  httpcapture.ParseFile(...) parser.go:265
  ```
  `RedactURL`도 내부 `url.Parse` 실패 시 원문에 가까운 문자열(`applyText(raw)`)을
  그대로 돌려주므로(redact.go:122–125), 손상 URL은 리댁션을 통과해 `mapEntry`에서 다시
  파싱되어 nil이 된다.
- **영향:** 파서의 존재 이유가 **신뢰 불가 HAR을 유계·비파괴로 처리**하는 것이다. `ParseFile`
  전 경로에 `recover`가 없어(parser.go 전체) 단일 악성/손상 엔트리가 **엔진 프로세스 전체를
  강제 종료**한다. SEC-4/SEC-5가 약속한 "부분 성공 없이 안전 거부(진단 반환)"가 이 입력군에서
  깨진다 — 거부가 아니라 크래시다. 사용자가 브라우저에서 뽑은 HAR에 인코딩 깨진 URL이
  섞이는 것은 흔한 일이므로 악의 없는 입력으로도 재발 가능.
- **권고:**
  1. `mapEntry`에서 `url.Parse` 에러를 처리한다 — 실패 시 `parsedURL`을 빈 `&url.URL{}`로
     대체하고 Scheme/Host/Path를 빈 값 또는 `"(unparsable)"`로 채운 뒤 진단
     (`HAR_URL_UNPARSABLE` 등)을 추가. 손상 URL은 리댁션된 원문 문자열만 `URL` 필드에 남긴다.
  2. 방어심층으로 `ParseFile` 진입부에 `defer recover()`를 두어, 예기치 못한 패닉을 fatal
     진단(`ReasonStructural`)으로 강등하고 프로세스 생존을 보장한다.
  3. 회귀 테스트에 손상 URL(`%GG`, `[::1`, 제어 문자) 케이스를 추가(SEC-5의 하위 케이스).

### P1-2 — `maxScanBytes`(1 MiB) 초과 JSON 바디에서 리댁션이 정규식 폴백으로 강등되어 자격증명 누출 (SEC-1 위반)

- **근거:** `RedactBody`는 바디를 `maxScanBytes`(기본 1 MiB, redact.go:24)로 **먼저 절단**한
  뒤(redact.go:187–190), 절단된 문자열을 `json.Unmarshal`로 파싱해 구조적 리댁션을 시도한다
  (redact.go:192–200). 그런데 바디가 1 MiB를 넘으면 절단본은 **불완전 JSON**이라 Unmarshal이
  실패하고, 폼 검사도 실패해 최종적으로 `p.applyText(limited)` **정규식 폴백**으로 떨어진다
  (redact.go:214). 정규식 `assignmentPattern`의 값 그룹은 `[^&\s,;"'}]+`(redact.go:65)라
  **여는 따옴표에서 즉시 멈춘다** → `"token":"opaque"` 형태의 따옴표로 감싼 JSON 키/값을
  잡지 못한다. 한편 가드는 `MaxBodyBytes`를 10 MiB(guard.go:15)까지 허용하므로, **1–10 MiB JSON
  바디 대역**이 정확히 이 폴백에 걸린다.
- **재현:** 민감 키를 앞쪽(1 MiB 스캔창 내부)에 두고 뒤를 패딩으로 1 MiB 초과시킨
  `application/json` 바디:
  ```
  {"token":"OPAQUE-SECRET-TOKEN","pad":"xxxx...(1MiB+)..."}
  → RedactBody 결과: {"token":"OPAQUE-SECRET-TOKEN",...  (앞 1 MiB 그대로, 토큰 미리댁션)
  ```
  프로브 테스트에서 `OPAQUE-SECRET-TOKEN`이 출력에 생존함을 확인. (값이 JWT면
  `jwtPattern`이 잡지만, **비-JWT opaque 토큰**이 누출된다 — H-SEC1 P2-5와 같은 부류의 결함이
  크기 축에서 재발.)
- **영향:** §11.3 "기본 켜짐·저장 시점 리댁션"과 SEC-1 "재사용 가능한 자격증명 0건"이
  1 MiB 초과 JSON 바디에서 **조용히 무력화**된다. 대형 API 응답(토큰·세션·PII를 담은 수 MB JSON)은
  드물지 않으며, 강등이 **경고 없이** 일어나 사용자는 리댁션이 걸린 것으로 오인한다.
- **권고:** 세 방식 중 하나를 계약으로 고정한다.
  1. 구조적 파싱용 스캔 한도를 `MaxBodyBytes`까지 올려 JSON을 온전히 파싱·리댁션한다(가장
     단순, 바디는 이미 `MaxBodyBytes`로 유계).
  2. 절단된 JSON이 파싱 불가면 정규식 폴백으로 **원문을 내보내지 말고** 바디를
     `bodyStorage="redacted"`로 전량 억제(`[REDACTED_OVERSIZED_JSON]`)한다.
  3. 최소한 강등이 일어날 때 `HAR_REDACTION_DEGRADED` 진단을 발생시켜 사용자에게 고지한다.
  회귀 테스트에 "민감 키가 스캔창 내부에 있는 1 MiB 초과 JSON 바디" 케이스 추가.

---

### P2-1 — 범용 키 `code`/`auth`/`session`의 무조건 리댁션이 분석 가치를 훼손

- **근거:** `sensitiveNormalizedKey`의 정확 매칭 집합에 `code`, `auth`, `session`이 포함된다
  (redact.go:408). `redactJSON`은 이 키를 값 타입과 무관하게 `"[REDACTED]"` 문자열로 치환한다
  (redact.go:245–248).
- **재현:** `{"code":200,"country":"US","statusCode":404}` →
  `{"code":"[REDACTED]","country":"US","statusCode":404}`. 즉 **숫자 `code`가 문자열로
  강제 변환**되고, 흔한 API 엔벌로프 필드(`{"code":0,"message":...}`, `?code=US`)가 통째로
  삭제된다. (다행히 `statusCode`는 정확 매칭이 아니라 보존됨 — 접미사 목록에 `code` 없음.)
- **영향:** 이 제품의 핵심 가치인 **분석 가능성**이 광범위한 정상 필드에서 손상된다. `code`는
  OAuth 인가 코드 외에도 상태/에러/국가/우편 코드 등으로 편재하므로, OAuth 문맥이 아닌
  대다수 케이스에서 **과다 리댁션**이다. 타입 강제 변환(number→string)은 다운스트림 집계·차트를
  깨뜨릴 수 있다.
- **권고:** `code`/`auth`를 무조건 키 리댁션에서 제외하고, OAuth 문맥(예: 경로가 `/token`,
  `/authorize`이거나 값이 인가-코드 형태)에서만 리댁션하거나, 최소한 값이 **문자열일 때만**
  리댁션한다(숫자·불리언은 자격증명이 아님). `session`(bare)은 유지 검토.

### P2-2 — 파서에 패닉 복구 경계(defense-in-depth)가 없다

- **근거:** `ParseFile`은 신뢰 불가 입력을 다루면서도 `defer recover()`가 없다(parser.go
  전체). P1-1이 이를 직접 증명하지만, 그와 별개로 **원칙 차원의 결함**이다 — 향후 정규화
  단계(타임스탬프, base64 디코딩, 커스텀 리댁션 정규식 등)에서 새 패닉 경로가 추가되면
  동일하게 엔진 전체가 죽는다.
- **영향:** 단일 신뢰 불가 파일이 다중 파일 배치 분석 전체나 장기 실행 엔진을 종료시킬 수 있다.
- **권고:** `ParseFile`(및 `analyzers/httpcapture.Analyze`) 진입부에 `recover`를 두어 어떤
  패닉도 fatal 진단으로 강등한다. P1-1의 구조적 수정과 병행하되 서로 대체하지 않는다.

### P2-3 — 자원 상한 테스트 커버리지 공백 (MaxBytes 평문·MaxFields·손상 URL)

- **근거:** `guard_test.go`는 entry-cap/depth-cap/string-cap/body-cap과 gzip 팽창비를 커버한다
  (guard_test.go:11–64). 그러나:
  - **비압축 `MaxBytes`**(guard.go:48, 74) 경로 미테스트 — 평문 파일 크기 상한 회귀 미보호.
  - **`MaxFields`**(guard.go:124) 미테스트 — `:` 카운트 상한이 회귀해도 감지 불가.
  - **손상 URL / 손상 timings** 등 엔트리 매핑 단계의 비정상 입력 미테스트(P1-1이 이 공백으로
    빠져나갔다).
- **영향:** 가드는 이 제품의 1차 방어선인데, 회귀가 조용히 통과할 표면이 남아 있다.
- **권고:** 위 3종 테스트를 추가. 특히 손상 URL/timings는 P1-1·P1-2 수정과 한 세트로 회귀
  테스트화. 가능하면 파서·리댁션에 `go test -fuzz` 하네스를 도입해 신뢰 불가 입력군을 넓게 커버.

---

### P3-1 — `bodyStorage`의 도달 불가 분기(dead code)

- **근거:** `bodyStorage`는 `preview == ""`일 때 `if mimeType != "" && !isTextMIME(mimeType)`
  분기와 그 외 분기가 **둘 다 `return "omitted"`**를 반환한다(parser.go:534–540). 비-텍스트를
  구분하려던 의도로 보이나 현재는 무의미한 죽은 분기다.
- **영향:** 기능 결함은 아니나, 의도(비-텍스트 바디를 별도 상태로 표기)가 코드에 반영되지
  않아 오해를 유발한다.
- **권고:** 비-텍스트 omit을 별도 상태(예: `"omitted_binary"`)로 표기하거나 분기를 제거해
  의도를 명확히 한다.

### P3-2 — 요청 `BodyDecoded` 의미가 응답과 비대칭

- **근거:** 응답은 `BodyDecoded: sizeValue(source.Content.Size)`(디코딩 크기, parser.go:437)를
  쓰는 반면 요청은 `BodyDecoded: sizeValue(source.BodySize)`(전송 바이트, parser.go:412)를
  쓴다. HAR `postData`에 디코딩 크기 필드가 없어 불가피한 면이 있으나, 같은 필드명이 요청/응답에서
  다른 의미를 갖는다.
- **권고:** 요청 `BodyDecoded`를 실제 디코딩 값이 없을 때 `unknownSize()`(-1)로 두거나,
  모델/문서에 비대칭을 명시.

### P3-3 — 인라인 프리뷰 절단이 UTF-8 룬 경계를 무시

- **근거:** `inlinePreview`는 `value[:inlineBodyPreviewBytes]`로 **바이트 단위 절단**한다
  (parser.go:552–557). 멀티바이트 UTF-8 문자 중간에서 잘리면 불완전 룬이 남는다.
- **영향:** `json.Marshal`이 U+FFFD로 치환하므로 크래시는 없으나, 프리뷰 말미에 깨진 문자가
  생길 수 있다(경미).
- **권고:** 룬 경계까지 뒤로 물러나 절단(`utf8.DecodeLastRuneInString` 기반)하면 깔끔하다.

---

## H-SEC1 Findings 반영 여부 (엔진 관련분)

| ID | H-SEC1 지적 | 현재 상태 | 근거 |
|---|---|---|---|
| P2-5 | 자유텍스트 denylist가 구조화 필드보다 좁음 | **반영됨** | `assignmentPattern`/`cliArgumentPattern`에 `token`/`auth`/`session(id)`/`code`/`cookie` 추가(redact.go:65–66), `sessionid` 정확 매칭 추가(redact.go:418). 3채널(평문 바디·프래그먼트·비표준 헤더) 회귀 테스트 추가(redact_test.go:46–60) |
| — | URL 프래그먼트 미리댁션 | **반영됨** | `RedactURL`이 Path·Fragment에 `applyText` 적용(redact.go:144–147) |
| — | 결제 카드 번호 | **추가됨** | Luhn 검증 기반 `redactPaymentCards`(redact.go:330–355), 회귀 테스트(redact_test.go:62–71) |
| SEC-1 | 재사용 자격증명 0건 | **부분 미달** | 구조화 필드·소형 바디 OK. **1 MiB 초과 JSON 바디에서 미달(본 리뷰 P1-2)** |
| SEC-3 | export 후 `commandLine`/`user` 미포함 | **반영됨** | 파서가 공통 모델 리댁션(redact.go:218–239) + 분석기가 export에서 완전 제외(analyzer.go:326–342), 회귀 테스트(analyzer_test.go:35–40) |
| SEC-4 | 압축폭탄·엔트리·깊이·크기 파싱 전 거부 | **반영됨** | guard.go 전량. 단 P2-3 테스트 공백 |
| SEC-5 | 구조 손상 fatal 중단 | **반영됨(단, P1-1 예외)** | `ErrStructuralHAR` 경로(parser.go:206–216). **손상 URL은 fatal이 아니라 패닉으로 빠짐(P1-1)** |
| SEC-6 | 가져오기 경로 리댁션 무조건 배선 | **반영됨** | 파서가 정책을 무조건 통과(parser.go:225, 347–356). WS/SSE는 파싱 자체를 안 함(안전) |
| SEC-7 | catastrophic backtracking 불가 | **반영됨** | Go `regexp`=RE2 + 커스텀 규칙 시간예산 후 비활성화(redact.go:312–315) |

> H-SEC1의 CA·덤프·IPC·피닝 등 캡처 표면 findings(P1-1, P2-1~4, P2-6)는 미구현 Phase 2+
> 영역이므로 본 엔진 리뷰 범위 밖이다.

---

## 테스트 커버리지 평가

**강점**
- 다이얼렉트 정규화·URL 리댁션·타이밍·상태를 한 번에 검증(parser_test.go:13–36).
- 구조 손상 시 부분 성공 금지(parser_test.go:38–60), WS/SSE 페이로드 미유출(parser_test.go:62–79).
- 자원 가드가 파싱 전 거부(guard_test.go), 공유 픽스처 매니페스트 기반 다이얼렉트/악성 코퍼스
  회귀 + 픽스처 비밀 미유출 단언(manifest_test.go:106–135).
- 리댁션의 전 필드 클래스·커스텀 규칙 유계·무효 규칙 비활성·3채널 자유텍스트·Luhn 검증
  (redact_test.go 전량).
- 집계의 순서 독립성·스냅샷 복구·장기 세션 라이브/오프라인 패리티(aggregate_test.go) — 특히
  5만 엔트리 패리티는 우수.

**공백 (권고: 게이트 전 보강)**
1. 손상 URL(`%GG`/`[::1`/제어문자) → P1-1 미탐지의 직접 원인.
2. 1 MiB 초과 JSON 바디 리댁션 → P1-2 미탐지의 직접 원인.
3. 비압축 `MaxBytes`, `MaxFields` 상한(P2-3).
4. 손상 timings(음수/NaN), 손상 timestamp의 정렬/타임라인 영향.
5. 파서·리댁션 fuzz 하네스 부재.

---

## 게이트 `PASS` 조건 (권고)

1. **P1-1 종료:** `mapEntry`의 `url.Parse` 에러 처리 + `ParseFile` `recover` 경계 + 손상 URL
   회귀 테스트.
2. **P1-2 종료:** 대용량 JSON 바디의 구조적 리댁션 보장(스캔 한도 상향 또는 폴백 억제) +
   고지 진단 + 회귀 테스트.
3. **P2-1~3 반영:** `code`/`auth` 과다 리댁션 축소, 패닉 복구 경계, 상한 테스트 보강.
4. P3 3건은 코드 명료성·경미 정정 차원 — 게이트 차단 사유 아님.

P0 부재 및 아키텍처 완성도를 고려할 때, 위 P1 2건이 닫히면 H-RG1 엔진 슬라이스는 `PASS`로
전환 가능하다. 현 독립 판정은 `CONDITIONAL`이다.
