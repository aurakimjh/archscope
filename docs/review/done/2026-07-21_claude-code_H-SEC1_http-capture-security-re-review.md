# H-SEC1 — 시스템 HTTP 캡처 보안 독립 재검증 (재리뷰)

- 검토 ID: H-SEC1-R1 (개별 보안 게이트, work_status.md §"Browser Profile / HTTP Capture Review Groups")
- 대상 그룹: H-RG1 offline HAR analysis completion (T-579)
- 원 리뷰: `docs/review/done/2026-07-21_claude-code_H-SEC1_http-capture-security-review.md` (판정 `CONDITIONAL`)
- 검토 기준 HEAD: `1a1b409`
- 검토 대상 문서: `docs/ko/SYSTEM_HTTP_CAPTURE.md` (권위 사본), `docs/en/SYSTEM_HTTP_CAPTURE.md`
- 교차 검증 대상 코드:
  - `apps/engine-native/internal/capture/redact/redact.go`
  - `apps/engine-native/internal/capture/redact/redact_test.go`
  - `apps/engine-native/internal/parsers/httpcapture/parser.go`
  - `apps/engine-native/internal/parsers/httpcapture/parser_test.go`
- 검토자: Claude (독립 재검증, findings-only)
- 검토일: 2026-07-21
- 검토 방법: 원 리뷰의 P1-1, P2-1~6, P3-1~3 각 finding을 현재 HEAD에서 **문서·코드에
  대해 독립적으로 재확인**했다. 원 리뷰와 동일하게, 이미 구현된 HAR 가져오기
  보안 표면(리댁션·파서)은 코드+회귀 테스트로 검증하고, 아직 미구현(Phase 2+)인
  캡처·CA·헬퍼 표면은 설계 계약의 완결성 관점에서 검증한다.

---

## 종합 재판정: `PASS`

원 리뷰의 8개 finding(P1×1, P2×6, P3×3)이 **전부 닫혔다.** 두 건의 구현 코드
결함(P2-5, P3-2)은 코드에서 수정되고 전용 회귀 테스트로 잠겼으며, 나머지 여섯 건의
설계 계약 결함은 `docs/ko`/`docs/en` 두 언어 사본에 대칭적으로 반영됐다. P0는 원
리뷰와 동일하게 부재하고, 게이트 차단 사유였던 P1·P2가 모두 해소됐다.

`go test ./internal/capture/redact/ ./internal/parsers/httpcapture/`와
`go vet` 대상 두 패키지 모두 통과한다.

### 범위 한정 (원 리뷰와 동일)

P1-1, P2-1, P2-3, P2-4, P2-6, P3-1, P3-3은 아직 구현되지 않은 캡처/CA/헬퍼
표면의 **설계 계약**이다. 본 재리뷰는 원 리뷰가 `CONDITIONAL`을 낸 것과 동일한
기준선에서 이 계약들이 명세로 완결됐음을 확인한 것이며, 구현 시점의 실측은
`SEC-8/10/16/17` 적대적 테스트 항과 H-SEC2(CA/TLS/권한 구현 리뷰)에서
별도로 강제된다. 코드로 실측 가능한 P2-5·P3-2는 실제 실행·테스트로 검증했다.

---

## Finding별 재검증 결과

| ID | 원 판정 | 재판정 | 근거 |
|---|---|---|---|
| P1-1 | 미봉 | **닫힘** | §11.1 "라이브 캡처 CLI 미제공"·§14 Q7 종료·`SEC-16` 추가 |
| P2-1 | 보장 불가 | **닫힘** | §11.2 zeroize 주장 철회 + 덤프 비활성·키스토어 서명 handle 메커니즘화 |
| P2-2 | 모순 | **닫힘** | §6.5.6 표가 §11.3.1(JWT/쿠키 전면 삭제) 기본값으로 정렬 |
| P2-3 | 미완 | **닫힘** | §11.2 플랫폼별 소유자 전용 ACL 계약(Windows DACL / POSIX 0600+0700) |
| P2-4 | 미명세 | **닫힘** | §11.2.1 "발견된 전체 저장소" 열거 + manifest 기록 + 재열거 검증 |
| P2-5 | 결함(코드) | **닫힘** | `assignmentPattern` 정렬 + 3채널 회귀 테스트 |
| P2-6 | 누락 | **닫힘** | §11.1.1 캡처 시점 스코프 최소화 + `SEC-17` |
| P3-1 | 미명시 | **닫힘** | §11.2 리프 개인키 메모리 상주·디스크 미기록 명문화 |
| P3-2 | 회귀 위험 | **닫힘** | 파서 WS/SSE 드롭 유지 + 불변식 회귀 테스트 |
| P3-3 | 정직성 | **닫힘** | §11.5 논리적 삭제·매체 secure erase 비보장 명시, 리댁션 off 보존 축소 |

### P1-1 — 은닉 캡처 금지의 CLI 채널 미봉 → 닫힘

`docs/ko` §11.1이 "**라이브 캡처 CLI를 제공하지 않는다** — `archscope-engine`은 파일
가져오기·분석만 제공하고, 캡처 시작은 지속 인디케이터·중지 제어가 있는 Wails UI로
제한하며 detached/headless 시작 경로를 등록하지 않는다"를 계약으로 고정했다.
§14 열린 질문 7이 "해결됨 (2026-07-21)"로 종료됐고, 적대적 테스트 행렬에 `SEC-16`
(CLI/비대화형/detached 시작 시도 → 거부)이 추가됐다. `docs/en`에도 대칭 반영됨
(`SEC-16`, §14 재검증 문구). 원 권고안 1번(CLI 캡처 시작 미제공)을 정확히 채택했다.

### P2-1 — Go 런타임 메모리 위생 주장 → 닫힘

§11.2가 "Go GC 메모리의 완전 zeroize를 주장하지 않는다"로 문구를 교체하고, 구체
메커니즘으로 재정의했다: 민감 자료 로드 **전** Linux `RLIMIT_CORE=0`/`PR_SET_DUMPABLE=0`,
Windows WER/local-dump 제외, macOS core-dump 비활성을 적용하고 **실패 시 HTTPS 평문
캡처를 시작하지 않는다**(fail-closed). 가능한 플랫폼은 CA 서명을 OS 키스토어의
non-exportable handle로 수행하고, 평문 바디는 bounded buffer 한정, 동일 사용자
권한 활성 디버거로부터의 힙 보호는 주장하지 않는다(정직한 한계). 위협 모델(§11.0)
"크래시 덤프" 행 1차 방어도 동일하게 갱신됨. 원 권고 (a)(b)(c)를 모두 반영.

### P2-2 — §6.5.6 ↔ §11.3.1 모순 → 닫힘

§6.5.6 "분석 가치를 보존하는 리댁션 기법" 표가 "아래 표의 기본값은 11.3.1의 안전성
재검토를 반영한다. 상관관계를 보존하는 약한 형태는 명시적 옵트인일 때만 허용한다"로
개정됐고, **JWT payload 전체 삭제**·**쿠키 값 완전 삭제**가 기본값으로 명시됐다.
상반된 "서명만 제거"/"해시 접두 치환"은 옵트인 예외로 강등. §11.3.1과 일치.

### P2-3 — CA at-rest 폴백 권한 크로스 플랫폼 → 닫힘

§11.2가 "파일 폴백은 권한을 강제하지 못하면 생성 자체를 실패시킨다"를 두고,
플랫폼별 계약을 명시: Windows는 ACL 상속을 끄고 현재 사용자+`SYSTEM`만 허용,
macOS/Linux는 키 `0600`과 상위 디렉터리 `0700`을 함께 검증, 기본 상속·world/group
가독 폴백 금지. `SEC-8` 통과 기준도 "Windows owner-only DACL 또는 POSIX 0600+0700"로
갱신. 원 권고를 그대로 채택.

### P2-4 — NSS/다중 프로필 열거 완전성 → 닫힘

§11.2.1이 "모든 저장소"를 **플랫폼별 고정 집합이 아니라 발견된 전체 집합**으로
정의: Windows/macOS 대상 저장소 + 발견된 모든 Firefox NSS 프로필, Linux는 시스템
trust anchor·`~/.pki/nssdb`·발견된 Firefox/Chromium의 모든 사용자·프로필별 NSS DB.
설치 시 저장소 identity와 CA fingerprint를 manifest에 기록하고, 제거 시 기록 집합과
재열거 집합의 합집합 처리 후 전수 재확인하며, 열거 불가·잠김·권한 실패 저장소는
성공으로 간주하지 않고 상태를 `trusted`로 유지한다. §11.2.1의 "제거했다고
생각했지만 남아 있는 CA" 최악 시나리오를 다중 프로필에서 실제로 차단.

### P2-5 — (구현) 자유텍스트 denylist 불일치 → 닫힘

`assignmentPattern`(redact.go:65)이 `sensitiveNormalizedKey`와 정렬됐다.
`[a-z0-9_.-]*(?:password|passwd|token|secret|signature|credential)` 접두 교대로
bare `token`과 일반 `*token` 접미사를 포괄하고, `auth|authorization|session(?:id)?|
code|cookie|set-cookie` 등이 추가됐다. 원 리뷰가 지적한 3개 채널(평문 바디, URL
프래그먼트, 비표준 헤더 값)은 각각 `RedactBody` 비-JSON 경로, `RedactURL`의
`parsed.Fragment = applyText(...)`(redact.go:151), `RedactHeaders` 비민감 헤더
else 분기(redact.go:165)를 통해 `applyText`에 도달한다. 회귀 테스트
`TestFreeTextChannelsUseStructuredSensitiveKeyCoverage`(redact_test.go:47)가
세 채널에서 `token`/`sessionId`/`code`/`authToken`과 Luhn 카드가 살아남지 않음을
검증한다. **독립 실행 결과 통과.**

### P2-6 — 캡처 시점 스코프 필터 부재 → 닫힘

§11.1.1 "캡처 범위 최소화"가 추가됐다: 라이브 캡처 시작 시 host 패턴 allowlist·확인된
process identity allowlist·"모든 로컬 프로세스" 중 명시 선택, 마지막 선택은 세션별
확인 없이는 비활성. scope는 세션 manifest 기록·캡처 중 UI 상시 표시, **scope 밖
트랜잭션은 헤더·바디를 저장 전에 드롭**. `SEC-17`(scope 밖·process `unknown` 트래픽
미저장)이 행렬에 추가됨. Privacy-by-design 수집 최소화 계약이 리댁션 앞단에 확보됨.

### P3-1 — 리프 개인키 비영속성 → 닫힘

§11.2가 "호스트별 리프 개인키는 메모리 캐시에만 존재하고 디스크에 기록하지 않는다.
세션 종료 시 캐시 참조를 폐기한다"를 명문화. CA와 동일한 Go 런타임 한계 적용을 명시.

### P3-2 — WS/SSE 리댁션 회귀 위험 → 닫힘

파서는 여전히 `_webSocketMessages`/`_eventSourceMessages`를 역직렬화하지 않아 출력에
도달하지 않는다(누출 0). 원 권고대로 불변식을 회귀 테스트로 잠갔다:
`TestParseHARDropsWebSocketAndSSEPayloadsFromNormalizedOutput`(parser_test.go:127)이
WS/SSE 필드에 원문 비밀을 넣은 HAR을 파싱해 정규화 출력에 부재함을 확인한다.
**독립 실행 결과 통과.** 향후 WS 분석 파싱을 추가할 때 이 테스트가 리댁션 배선
누락을 즉시 검출한다.

### P3-3 — 세션 보존·삭제 의미 → 닫힘

§11.5가 "세션 삭제는 인덱스·manifest·blob 참조와 파일을 모두 unlink하는 논리적
삭제이며 SSD/파일시스템 매체 수준 secure erase를 보장하지 않는다"를 UI·문서에
정직하게 한정하고, "향후 리댁션 off 세션은 기본 자동 보존 대상에서 제외"를 두었다.
원 리뷰가 요구한 정직성·보존 축소 두 축을 모두 반영.

---

## §11.7 적대적 테스트 행렬 재점검

| ID | 재판정 | 비고 |
|---|---|---|
| SEC-1 | **통과** | P2-5 정렬로 평문/프래그먼트/비표준 헤더 bare-token 채널 폐쇄. 회귀 테스트 존재 |
| SEC-2 | 통과 | 원 리뷰와 동일 |
| SEC-4~7 | 통과 | 원 리뷰와 동일 (가드/구조손상/리댁션 배선/RE2). SEC-6 WS 하위 케이스는 P3-2로 잠김 |
| SEC-8 | 설계(강화) | Windows owner-only DACL / POSIX 0600+0700 폴백 계약 명시 |
| SEC-10 | 설계(재정의) | zeroize 주장 → 덤프 비활성·키스토어 서명 handle 메커니즘으로 통과 기준 재정의 (P2-1) |
| SEC-12 | 설계(강화) | 발견된 전체 NSS 저장소 열거로 완전성 계약화 (P2-4) |
| SEC-16 | 설계(신규) | CLI/비대화형/detached 시작 거부 (P1-1) |
| SEC-17 | 설계(신규) | scope 밖·`unknown` 트래픽 미저장 (P2-6) |

미구현 항(SEC-8/10/16/17)은 구현 시점에 H-SEC2에서 실측되어야 한다. 본 재리뷰의
"설계" 판정은 계약 완결성 확인이며 구현 통과 확인이 아니다.

---

## 게이트 결론

원 리뷰의 `PASS` 전환 조건(P1-1·P2-5 종료, P2-1/2/3/4/6 반영, P3 3건 반영)이 모두
충족됐다. 구현 코드 결함 2건은 실행·테스트로 검증됐고, 설계 계약 6건은 두 언어
사본에 대칭 반영됐다. **H-SEC1 독립 재판정: `PASS`.**

후속 주의: SEC-8/10/16/17로 계약된 캡처·CA·헬퍼 표면은 Phase 2+ 구현 시 H-SEC2에서
실측 재검증이 필수다. 본 게이트는 그 계약이 명세로 완결됐음을 확인한 것이다.
