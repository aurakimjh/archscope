# WR-02 Windows 장시간·10분 Clear 검증

2026-11-14 사용자 요청에 따라 실제 Windows production x64/amd64와 x86/386
각각 7,200초 테스트를 실행했다. 두 앱은 별도 descriptor·캡처 저장소·WebView
프로필·proxy port와 별도 producer PID/start identity를 사용하며, 하나의 소유
Docker loopback HTTP origin만 공유했다. 본문 캡처는 꺼져 있다.
두 앱의 장시간 실행, 종료·저장·재열기 및 독립 데이터 비교가 모두 통과했다.

| 관측 | x64/amd64 | x86/386 |
|---|---:|---:|
| 실제 실행 시간(초) | 7201.013 | 7200.576 |
| 전체 생성 요청 수 | 24984 | 24972 |
| 체크포인트 수 | 241 | 241 |
| 일시정지·재개 횟수 | 119 | 119 |
| 10분 Clear 횟수 | 11 | 11 |
| 최종 세대 캡처 수 | 2080 | 2076 |
| 최종 세대 저장 수 | 2080 | 2076 |
| 최종 보관·export 행 수 | 128 | 128 |
| 복원한 선택 행 sequence | 24857 | 24845 |

전체 생성 요청 수는 Clear를 넘어 누적한 수이고, 최종 캡처·저장 수는 마지막
Clear 이후 세대의 수다. 둘을 동일한 값으로 해석하지 않는다. 각 10분 경계에서
Clear 전 running·캡처=저장·드롭0을 확인하고, Clear 직후 로그·카운터0과 세대+1,
이어 새 URL 4건의 정확한 저장·조회까지 검사했다. 별도 검증기는 모든 경계의
요청 수 차이와 Clear 전 카운터도 재계산했다. 주기 체크포인트마다 세대·요청 수,
전체 pagination·보관128행 이하, 드롭0과 프로세스 응답 조건을 만족했다.
이번에는600~6,600초의11개 경계를 통과했고7,200초에는 루프가 종료되어
12번째 Clear를 실행하지 않았다. 종료 전 별도4건을 추가한 뒤 저장했다.

기존 러너는 Clear 명령 성공과 횟수만 기록했으므로 이번에
`clearDesktopCapture()`와 위 검사를 추가했다. 초기화 누락·세대 불변·옛 URL
재등장·저장 재개 실패를 거절하는 회귀를 포함해 Windows predicate 테스트
41개가 통과했다(`predicates.tap`: 실패·건너뜀0). 이것은 통제된 판정 테스트이며
실제 네이티브 디스크 장애 주입의 증거가 아니다. 장시간 실행에서 추가 제품
오류는 발견되지 않았다.

Stop은 finalized를 반환했고, 이후 authoritative 카운터가 일치하며 드롭0이다.
최종 export는 비어 있지 않으며 선택 행을 복원했다. 별도 Python 검증기는
원격817ed244에서 추가된 maintained comparator의 명시적 정책1.0.0을 사용해
rolling store와 portable export의 고유 ID 및 전체 필드를 비교했다. source URL에서
유도한 host/query와 export 값을 대조하며 export 값을 덮어써 비교하지 않는다.
store annotations.json을 ID별 권위 값으로 사용하고 생략된 annotation은
false/빈 문자열로 처리한다. Windows에서 comparator 회귀23개도 통과했다
(`annotation-regressions.txt`). 새 보고서의 annotationsMatch 및
allTransactionFieldsMatchDeclaredPolicy는 이 명시적 정책에 대한 결과다.
이 장시간 fixture는 별표·주석을 편집하지 않는다. 실제 sidecar/기본값의 비교이며
새로운 비어 있지 않은 live 주석 편집 UI 수용을 주장하지 않는다.
최종 URL 연속 범위가 마지막 Clear 이후 요청으로만 구성되는지도 확인했다.
결과는 `readback-amd64.json`, `readback-386.json`이다.

실제 네이티브 초기·Clear 이후·1시간 이후·최종 재열기 화면을 별도로 관측했다
(`native-cases.json`). 두 앱이 같은 위치에 겹쳐 열려 요청한 창의 metadata와
화면에 그려진 앱이 다를 수 있었다. 초기 관측의 아키텍처 표기는 화면의 proxy를
각 private session과 대조해 보정했고 원래 JPEG 바이트를 유지했다. x64는
127.0.0.1:3011, x86은127.0.0.1:3008이다. 이후에는 전면 활성화와 별도 새
스냅샷으로 대조했다. 다른 사용자 창에 가린 화면은 공개 증거에서 제외했다.
실시간 화면의 순간 captured/persisted 차이는 그 프레임에 명시하며, quiescent
체크포인트의 일치 판정과 구분한다. 전체 layout·locale·DPI 판정은 하지 않았다.
native-cases의 recordedAt은 관측 기록을 파일에 저장한 시각이다.

| 아키텍처 | 루트 private 최대(MiB) | 루트+현재 자손 private 최대(MiB) | 트리 private 최초→최종(MiB) |
|---|---:|---:|---|
| amd64 | 98.60 | 496.32 | 349.57 → 473.47 |
| 386 | 88.31 | 496.41 | 347.19 → 477.40 |

메모리는 30초 주기의 프로세스 표본이다. 루트 private1GiB guard는 통과했다.
트리 수치는 매번 현재 살아 있는 자손을 포함한 합이며 종료된 프로세스·모든
시간의 피크를 추적하지 않는다. 두 앱이 같은 호스트에서 동시에 실행됐고
워밍업·GC·브라우저 캐시 영향도 있으므로 누수 없음이나 보편적 메모리 수용을
증명하지 않는다. 원시 표본과 handle/process 수는 `soak-*.json`에 보존했다.

앱은 이전 Windows 검증과 같은 source324a8d97의 production 바이너리다.
SHA-256은 원본 short-run receipt와 일치한다. 이 후속 작업은 앱 소스를 바꾸지
않고 JavaScript 검증 러너·공통 모듈·회귀만 변경했다. 사용 소스 snapshot과
worktree/Git blob 해시는 `source-hashes.json`에 고정했다. 실행은 원 저장소의
상대 import 구조를 사용하며 평평한 증거 폴더의 snapshot을 직접 실행하지 않는다.

실행 명령(저장소 루트):

```powershell
node --test --test-reporter=tap scripts/test-desktop-soak-checks.mjs
node scripts/verify-windows-desktop-soak.mjs --exe .tmp/p2-windows-20261114/archscope-p2-amd64.exe --out .tmp/wr02-long-20261114/amd64 --origin http://127.0.0.1:11511/ --duration 7200
node scripts/verify-windows-desktop-soak.mjs --exe .tmp/p2-windows-20261114/archscope-p2-386.exe --out .tmp/wr02-long-20261114/386 --origin http://127.0.0.1:11511/ --duration 7200
```

러너 결과는 최종 네이티브 관측을 위해 앱이 살아 있는 시점의 원본이다.
그 뒤 private descriptor PID와 exe 경로를 대조해 소유 앱만 종료했고, label을
확인해 전용 Docker fixture만 정리했다(`cleanup.json`). 기존 컨테이너는 유지했다.
raw 세션·토큰·프로세스 목록·실행 파일은 공개하지 않으며, 실제 controller
credential 값이 공개 파일에 없는지 검사했다. 화면에는 테스트 producer의
PID/start label이 보일 수 있다. `artifact-sha256.json`은 자기 자신을 제외한
모든 공개 파일의 정확한 바이트 길이·SHA-256을 기록한다.

이전 short-run·Mac·Linux·2시간 증거와 원 독립 리뷰는 변경하지 않았다.
WR-02의 이 Windows 장시간·Clear 범위는 검증됐지만 원 독립 판정 CONDITIONAL과
T-613 PENDING / T-627 REVIEW는 유지한다. 독립 재검토 및 전체 reference·locale·DPI,
본문·vendor·driver·service와 별도 장기 메모리 수용 조건은 남아 있다.
