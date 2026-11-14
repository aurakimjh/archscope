# P2 Windows 검증 — WR-01 / WR-02

2026-11-14 `git pull --ff-only`로 `ab42eeb8`에서 `324a8d97`까지 갱신했다.
두 P2 수정이 포함된 소스로 production Windows x64/amd64 및 x86/386 앱을
각각 빌드하고 실제 네이티브 앱과 Docker loopback origin으로 검증했다.
이번 검증에서 추가 제품 오류는 발견되지 않았다. 본문 캡처는 계속 꺼져 있다.

| 항목 | x64/amd64 | x86/386 |
|---|---|---|
| WR-01 실제 live 중첩 Query | 4건, Path 필터·정렬·Query·Raw 일치 | 4건, 동일 검증 및 반복 `?`·인코딩·중복 키 추가 확인 |
| WR-02 schema4 실행 시간 | 130.3817031초 | 130.5664087초 |
| 캡처 / 저장 / 드롭 | 468 / 468 / 0 | 464 / 464 / 0 |
| 체크포인트 / 일시정지·재개 | 6 / 2 | 6 / 2 |
| 최종 상태 / 보관 / 순환 제외 | finalized / 128 / 340 | finalized / 128 / 336 |
| 저장·재열기 독립 비교 | 128행·선택 sequence341 일치 | 128행·선택 sequence337 일치 |
| 네이티브 최종 화면 | 종료됨·카운터·선택 행·Raw 200 확인 | 종료됨·카운터·선택 행·Raw 200 확인 |

WR-01의 15개 네이티브 관측과 4행 export 검증은
[별도 기록](README-query.md)에 있다. WR-02의 두 최종 화면은
[관측 기록](native-soak-cases.json)과 해당 JPEG에 있다. 전체 17개 관측은
`native-cases.json`이다. query 관측 중 x86 오름차순 스크린샷 한 장은 다른
사용자 창에 가려 제외했으며, 해당 native accessibility 순서와 이후 가림 없는
내림차순 화면을 기록했다. 전체 레이아웃·DPI 판정은 하지 않았다.

Windows에서 공통 soak 성공·실패 조건 테스트 28개가 모두 통과했다
(`soak-predicates.tap`: 실패·건너뜀 0). running 상태, 음수·비정수 카운터,
드롭, 저장 지연 deadline, finalized 종료, 최종 authoritative 카운터 및
비어 있지 않은 저장·선택 복원 조건의 거절 경계를 포함한다. 이것은 통제된
predicate 테스트이며 실제 네이티브 디스크 장애 주입을 수행한 결과는 아니다.

실제 두 실행 모두 schema4 러너가 6개 체크포인트에서 running·드롭0·캡처와
저장 일치를 확인했다. Stop 후 finalized 상태 및 authoritative 카운터 일치를
확인하고 저장·열기를 완료했다. 별도 Python 검증기는 private rolling store와
portable export의 128개 고유 ID 집합 및 나머지 필드를 비교했다. portable
URL의 host/query 정규화만 명시적으로 반영하며 선택 행과 파일 해시도 검증했다
([x64](readback-soak-amd64.json), [x86](readback-soak-386.json)).
최종 네이티브 화면은 이 데이터 검증과 별도로 실제 앱에서 관측했다.

프런트엔드 state 회귀·production build, 전체 production 앱 테스트
(amd64 38.091초 / 386 42.259초), capture 24개 패키지씩도 모두 통과했다.
환경은 Windows, Go1.26.10, Node22.15.1, CGO_ENABLED=0이다.
이 결과로 Windows CGO race 테스트의 통과를 주장하지 않는다.

`soak-{amd64,386}.json`은 실행 당시 러너 결과 원본이다. 관측을 위해 앱을
살려 두므로 `desktopAliveAtEnd`와 `cleanupRequired`가 남아 있다. 이후
private descriptor PID와 정확한 exe 경로를 대조해 소유 앱만 종료했고,
소유권 label을 대조해 전용 Docker 컨테이너만 정리했다. 별도
[정리 확인](cleanup.json)에서 소유 앱·producer·창 0개 및 기존 Docker
컨테이너 7개 유지가 확인된다. raw 세션·토큰·프로세스 목록·실행 파일은
공개하지 않는다. 화면에는 테스트 producer의 PID/start label이 보일 수 있다.

재현 명령은 저장소 루트에서 아래와 같다. 지정 경로는 이번 private 작업
디렉터리 예시이며 기존 사용자 앱을 재사용하지 않는다.

```powershell
node --test --test-reporter=tap scripts/test-desktop-soak-checks.mjs
node scripts/verify-windows-desktop-soak.mjs --exe .tmp/p2-windows-20261114/archscope-p2-amd64.exe --out .tmp/p2-windows-20261114/soak-amd64 --origin http://127.0.0.1:11510/ --duration 130
node scripts/verify-windows-desktop-soak.mjs --exe .tmp/p2-windows-20261114/archscope-p2-386.exe --out .tmp/p2-windows-20261114/soak-386 --origin http://127.0.0.1:11510/ --duration 130
```

`soak-runner-used.mjs`, `soak-checks-used.mjs`, `soak-tests-used.mjs`,
`soak-origin-used.py`는 실제 사용 파일의 byte snapshot이다. 공통 module까지
해시를 고정했다. snapshot은 평평한 증거 폴더에 있으므로 실행은 원 저장소
상대 import 구조를 사용한다. `soak-source-hashes.json`은 Windows worktree
바이트와 `324a8d97` Git blob 바이트의 SHA-256을 구분한다.
`soak-validation.json`은 검사 요약이고 `artifact-sha256.json`은 자기 자신을
제외한 전체 공개 파일의 byte 길이·해시다. 첫 WR-01 커밋의
`artifact-query-sha256.json`과 이전 Mac remediation 10개 해시는 변경 없이
다시 검증했다.

이번 130초 실행은 변경된 성공 판정과 종료·저장 경로를 확인하는 짧은 통합
검증이다. 새 2시간 실행이나 메모리 누수 없음의 증거가 아니며, 10분 Clear
분기는 실행되지 않았다. 기존 2시간·Mac·Linux 증거는 그대로 보존한다.
원 독립 리뷰는 CONDITIONAL이고 다른 AI의 재검토·처리는 대기 중이다.
T-613 PENDING / T-627 REVIEW와 전체 reference·locale·DPI, 장시간 메모리,
본문·vendor·driver·service 인수 조건은 별도로 남는다.
