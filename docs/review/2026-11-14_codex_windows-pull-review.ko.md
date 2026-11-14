# Windows 테스트 변경 검토

- 검토일: 2026-11-14
- 검토자: Codex
- 범위: `f25ab80a..ab42eeb8`, 49개 커밋, 514개 파일
- 판정: **CONDITIONAL — P2 2건, 확인된 P1 없음**
- 기존 상태: T-613 `PENDING`, T-627 `REVIEW` 유지. 다른 수용 게이트도 변경하지 않음.

## WR-01 · P2 · Path 쿼리 복원 시 두 번째 물음표 뒤의 값 누락

위치: `apps/engine-native/cmd/archscope-app/frontend/src/state/httpColumns.ts:61`

`4db0294c`에서 Path 표시·필터·정렬이 `requestQuery()`를 사용하도록
변경되었다. 이 헬퍼는 `url.split("?")[1]`로 쿼리를 추출하므로 쿼리 값에
물음표가 있으면 그 뒤를 모두 버린다. live capture는 별도 `query`를 비우고
마스킹된 URL을 보관하므로 이 경로가 실제 live 행에 적용된다.

재현 입력은 다음과 같다.

```text
url   = http://example.test/lookup?next=/detail?id=7&keep=2
path  = /lookup
query = ""
```

기대 Path는 `/lookup?next=/detail?id=7&keep=2`이지만 실제 표시값은
`/lookup?next=/detail`이다. Path contains `id=7` 필터도 false를 반환한다.
Raw 재구성은 전체 쿼리를 유지하므로 같은 요청의 두 화면이 다르다.
보관 URL/요청 원본을 변경하는 문제는 아니지만 요청 구분과 검색·정렬을
잘못하게 만든다. 기존 Query 탭에도 같은 헬퍼 문제가 있으며, 이번 변경은
그 영향을 기본 그리드까지 확대한다.

수정 조건: 첫 `?` 뒤에서 fragment 전까지의 문자열을 추가 `?`와 인코딩을
보존하여 추출한다. 중첩 URL 값, 반복 `?`, fragment, 빈 별도 query를 포함한
표시·필터·정렬·Query/Raw 일관성 회귀를 확인한다.

## WR-02 · P2 · 장시간 테스트 러너가 저장 불일치와 recoverable 종료를 허용

위치: `scripts/verify-windows-desktop-soak.mjs:66–67,137–147`

체크포인트는 `dropped`만 실패 조건으로 사용하며 `captured/persisted`와
세션 상태는 기록만 한다. 종료 단계도 `stop`의 반환 상태를 무시하고,
현재 저장된 일부 행의 save/open이 성공하면 `report.passed = true`로 만든다.
`session.Manager.Stop()`은 저장 수 불일치 시 `recoverable` 상태를 반환하면서
오류가 nil일 수 있고, 이미 recoverable인 세션의 stop도 성공 반환한다.
따라서 HTTP 응답 이후 저장 실패 또는 마지막 체크포인트 이후 불완전 종료가
이 러너의 PASS를 막는다는 보장이 없다.

실제 러너 소스의 체크포인트/종료 본문을 통제된 포트로 실행했다.

- `state=recoverable, captured=2, persisted=1, dropped=0`을 체크포인트가 거절하지 않았다.
- `stop → recoverable` 이후 저장된 1행만 save/open한 종료 본문이 `passed=true`를 설정했다.

이는 JavaScript 판정 로직의 재현이며 새 Windows 장애 관측은 아니다.
이미 제출된 고정 빌드 두 시간 실행은 별도로 재계산한 482개 체크포인트에서
모두 `captured=persisted`, `dropped=0`, 보관 행/페이지 수 일치를 충족한다.
기존 두 시간 실행 결과를 실패로 변경하지 않는다.

수정 조건: 체크포인트에서 예상 running 상태를 확인하고, in-flight 쓰기는
제한된 대기/배출 후 비교한다. stop 이후 finalized 상태와 최종 authoritative
카운터 일치를 요구하고 recoverable/failed·저장 불일치·저장 실패를 주입한
러너 판정 회귀를 추가한다. 이전 실행 증거는 원문 그대로 보존한다.

## 검증과 증거 범위

- `git pull --ff-only origin main` 완료: HEAD와 origin/main은 `ab42eeb8`.
  기존 HTTP 문서 2개·작업표·미추적 verifier를 stash 후 충돌 없이 복원했다.
- macOS에서 `go test ./internal/capture/... ./internal/analyzers/multithread ./internal/visuals ./cmd/archscope-app` 통과.
- 같은 범위 `go vet` 통과.
- 프런트엔드 `npm run test:state`, `npm run build` 통과.
- `python3 scripts/build-offline-assets.py --check`,
  `python3 scripts/build-input-assistance-assets.py --check` 통과.
- 신규 네이티브 증거 7개 해시 목록의 404개 파일은 SHA-256이 모두 일치한다.
- amd64/386 고정 두 시간 실행 JSON의 241개씩, 총 482개 체크포인트 조건을
  독립 재계산했다. 실패한 기존 386 실행과 INCONCLUSIVE 기록은 유지되어 있다.
- Go app 링크에는 기존 macOS SDK deployment-target 경고가 있었지만 테스트는 통과했다.
- 이번 검토는 Windows UI/드라이버/Office를 재실행하지 않았다. 원래 실행의
  전체 reference·locale·DPI·body·vendor·service·장기 메모리 수용 한계는 남는다.

재현: 프런트엔드 상태 테스트 빌드 후 저장소 루트에서
`node docs/review/evidence/2026-11-14-windows-pull-review/reproduce.mjs`.
출력은 [reproduction.json](evidence/2026-11-14-windows-pull-review/reproduction.json),
검증 요약은 [local.json](evidence/2026-11-14-windows-pull-review/local.json)에 기록했다.

후속 순서는 WR-01 → WR-02이며 두 작업 간 구현 의존성은 없다.
작업표에 기록했으며, 이 문서는 작성자와 다른 처리자가 후속 처리하도록
`docs/review/`에 미처리 상태로 둔다. 제품 수정·커밋·푸시는 수행하지 않았다.
