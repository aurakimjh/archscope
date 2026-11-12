# Windows 미완료 검증 후속 실행 — 2026-11-12

이 Windows 작업의 최종 검증 대상은 `97501f86a3b101cc4dfd9d03d1fa06e57aab5719`다.
x64/x86 각각 전체 118개 패키지, 테스트 완료 이벤트 1,689 PASS / 0 FAIL /
15 SKIP를 확인했다. 양쪽 vet·production 빌드·15초 기동/제어, 프런트엔드
상태/빌드와 11개 importer fixture도 통과했다.
[통합 검증 결과](merged-validation.json), [x64 앱 제어](merged-control-amd64.json),
[x86 앱 제어](merged-control-386.json)를 참조한다. 아래 초기 검증과 발견한
실패 기록은 각 당시의 commit 범위로 보존한다.

같은 commit의 최종 [Engine Native](https://github.com/aurakimjh/archscope/actions/runs/43217091988)
8/8 job과 [General CI](https://github.com/aurakimjh/archscope/actions/runs/43217092334)
2/2 job이 모두 통과했다. Windows x64 전체 race, native x86 전체 테스트,
Windows 라이브/본문 캡처, SEC-10 강제 종료 덤프 제외·정책 복원을 포함한다.
[ci.json](ci.json)에 정확한 job·step 판정과 이전 실패 실행을 보존했다.
결과 기록 직전에 원격에 별도 T-644 리뷰·T-645 구현이 추가돼 문서만 그 위로
통합했다. 이 기록의 검사 범위는 `97501f86`이며 후속 T-645 Windows 인수로
확대하지 않는다.

Pull 대상은 `d9e62b1ddc22e5cc2150d981e0909a23c210d7c9`, 수정 후 검증 코드는
`3b968ea97cfb81b3ebdb0454549d685dd7e649c0`이다. 물리 Windows 11 Pro
10.0.26300에서 실행했다. 원본 로그·토큰·세션 파일·테스트 도구는 로컬
`.tmp/windows-20261112`에 보존하고, 공개 결과와 해시는
[validation.json](validation.json)에 기록했다.

## 재현 및 수정

- T-636/T-637/T-638/T-639 CLI 테스트가 확장자 없는 `engine`을 빌드해
  Windows에서 실행하지 못했다. Windows `.exe`를 처리하는 기존
  `buildBinary(t)`를 재사용했다. 네 테스트와 전체 CLI 패키지를 양쪽
  아키텍처에서 재실행했다.
- 실제 producer 입력 세 개의 CRLF 변환으로 고정 SHA-256 검사가 실패했다.
  해당 fixture 디렉터리를 LF로 고정했다. 내장 오프라인 문서와 원본 문서도
  서로 다른 줄바꿈으로 리소스 일치 검사에 실패해, 복사 대상 EN/KO 문서를
  LF로 고정했다. 기존 LF 속성이 있는 npm lockfile도 작업 트리를 재정규화했다.
  정규화한 모든 리소스는 기존 Git HEAD 바이트와 일치하며, fixture·내장
  ZIP·해시·의존성 notice를 다시 생성하거나 검증 기준을 완화하지 않았다.
- 변환기 종료 테스트는 1초 안에 도구 검증과 프로세스 시작까지 요구해,
  부하 중 자식 PID 파일이 만들어지기 전에 만료됐다. 이제 자식 준비를
  확인한 뒤 취소한다. 취소 후 5초 이내 반환, 결과 미공개, 실제 자식 종료
  검증은 유지했다. 양쪽 아키텍처에서 각 10회 추가 반복도 통과했다.

제품 Go/renderer 코드와 의존성은 변경하지 않았다.

## 실행 결과

| 검증 | 결과 |
|---|---|
| 수정 후 native amd64 전체 Go 테스트 | 116 패키지 PASS, 테스트 완료 이벤트 1,672 PASS / 0 FAIL / 15 SKIP |
| 수정 후 native 386 전체 Go 테스트 | 동일; 검증된 임시 Wails SDK 패치를 적용하는 `test.mjs` 사용 |
| 양쪽 전체 Go vet | PASS |
| 프런트엔드 `npm ci`, 상태 회귀, production 빌드 | PASS; npm audit 0 vulnerabilities |
| 양쪽 production GUI 빌드, 15초 기동 | PASS; 별도 GUI smoke는 graceful timeout 후 강제 종료 |
| 양쪽 인증된 실제 앱 캡처·저장·재열기 | 각 CONTROL/PROCESS/EXCHANGE/SESSION 4개 PASS; 전체 16개 parity 항목의 종합 판정은 UNVERIFIED |
| native COM ABI/typeinfo/events/RPC/admission 및 Unicode 바로가기 | 전체 Go suite에 포함해 PASS; clean-host 공개 등록은 기존 등록 보존으로 SKIP |
| 설치된 원본 프로그램의 프리셋 필터 17개 | 양쪽 PASS: 지원 16개, 캐시 출처가 필요한 1개는 명시적 거부 |
| 독립 XLS 읽기 | 양쪽 PASS; 격리 venv의 xlrd 2.0.2로 1,001행·한글·수식처럼 보이는 리터럴 검증 |
| 오프라인 자산 일치·10개 등록 importer fixture 실행 | PASS |
| desktop-only/server 실제 빌드 거부, parity/합성 fixture 계약 | PASS |
| 보호된 registry rename 회귀 | PASS; 테스트 소유 키의 ACL·ExpandString 보존 |
| 10분 실제 로컬 서비스/프록시 반복 | PASS: 603.731초, 172회, 요청 44,720건, 실제 보존 행 최대 64 |
| TCP 소유자 귀속 비용 측정 | 양쪽 50/50 confirmed; 부하가 있는 동시 실행 측정이며 성능 합격 기준은 설정하지 않음 |
| 로컬 race detector | 미실행: C compiler 없음, CGO_ENABLED=0. 호스티드 검증과 구분 |

테스트 완료 이벤트에는 하위 테스트가 포함된다. 두 suite의 SKIP 목록은
validation.json에 그대로 열거했다. 선택 실행으로 resolver, 원본 필터,
독립 XLS 검사를 추가 수행했다. helper·다른 플랫폼 전용·외부 fixture·UI 전용
검사는 통과로 계산하지 않았다.

최초 앱 하니스는 수정 커밋 전 빌드 메타데이터가 들어간 바이너리와 새 HEAD를
비교해 `binary/source commit is unverified`로 FAIL했다. 네 기능 검사는
통과했지만 이 최초 판정은 보존한다. 이후 현재 코드 커밋을 명시해 양쪽
앱을 재빌드하고 하니스를 다시 실행해 메타데이터 조건까지 통과했다.
최종 보고서는 [x64](control-amd64.json), [x86](control-386.json)이다.

## 실제 production Wails/WebView2 화면

[native-ui.json](native-ui.json)에 관찰 범위를 분리했다.

- 영어 모드에서 네이티브 Analyze/Workspace/Capture/View 메뉴로 21개
  화면의 기본·빈 상태 또는 폼 진입을 확인했다. 감사 앱·브라우저·mock IPC를
  사용하지 않았다. 이 초기 탐색 바이너리는 pull 커밋 빌드이며, 이후 수정은
  테스트/체크아웃 속성뿐이다.
- 한국어로 전환하면 페이지와 네이티브 메뉴가 함께 바뀌고 영어 복귀도
  확인했다. [한국어 HTTP 화면](native-ko-http.png)은 아래 취소 오류도 보존한다.
- HAR 선택창은 열렸으나 Computer Use의 `set_value`가 두 번
  `element ... is not available in cached app state for archscope.exe`를
  반환했다. 파일 선택·Analyze 완료는 미검증이다. Escape로 선택창을
  닫고 HTTP 화면 복귀는 확인했다.
- 최신 커밋 앱을 지원되는 `--open ABS_SESSION`으로 실행해, 이번 하니스가
  저장한 합성 2행을 실제 화면에서 다시 열었다. 저장된 GET 302 선택 복원,
  POST 201 선택과 중복 redacted Set-Cookie/Content-Length 34 등
  [Headers](native-reopened-headers.png)를 확인했다.
- [Body](native-reopened-body.png)는 요청/응답 모두 기록 꺼짐을 명시하고,
  전송 크기 34 B/77 B를 유지했다. 새 본문 캡처 인수로 해석하지 않는다.
- Analyze → Profiler → Capture → HTTP Capture 이후 두 행이 복원됐다.
  화면 내부 POST/Body 선택은 저장 세션의 첫 GET/Raw 선택으로 돌아왔다.

관찰한 두 UI 후속 항목은 남긴다: 영어 MSA Timeline의 일부 탭·빈 상태가
한국어이며, 파일 선택 취소가 원시 내부 오류 문자열로 표시된다.
두 행은 sparse fixture이므로 70% 밀도 통과를 주장하지 않는다. Computer Use
스크린샷은 1266×793으로 보고됐지만 정확한 CSS viewport·OS 배율 측정은 아니다.

## 남은 인수 조건

본 Windows 검증으로 T-601/T-611/T-613/T-627을 닫지는 않았다. 초기 기준은
225/232 DONE(7개 미완료)이었다. 이후 별도 T-643 리뷰·T-644 원격 변경을
통합하면서 전체 기준은 226/232 DONE(6개 미완료)이 됐으며, 위 네 Windows
인수 상태는 유지한다.

- T-601: 비관리자 호스트이며 WinDivert와 외부 IPv4 port-80 fixture가 없다.
  실제 드라이버 주입·서비스·프록시 무시 클라이언트 범위는 미검증이다.
- T-611: 이 PC의 원본 등록은 보존했다. clean-host 공개 등록은 호스티드 COM
  실행에서 양쪽 PASS를 다시 확인했다. 실제 vendor early-bound client,
  추가 cache-origin 근거와 타 프로세스의 메모리 session-cookie 상호운용이 남는다.
- T-613: 이번 10분 검증은 서비스/프록시 시험이다. 여러 시간 연속 데스크톱
  조작·전체 원본 기능 비교를 대신하지 않는다.
- T-627: 21개 메뉴 진입이 전체 populated/conditional/error·좁은 창·125% OS
  DPI 행렬을 완료하지는 않는다. 정확한 viewport/비율과 위 UI 후속 항목도 남는다.
- 본문 E2E/SEC-10 forced crash: 비관리자이고 기존 HKLM LocalDumps가 있어
  정책 격리 없이 실행하지 않았다. CA·전역 프록시·덤프 정책을 변경하지 않았다.

## 재실행

`apps/engine-native`에서 다음을 실행한다.

```powershell
go test -json -count=1 -p 2 -timeout 10m ./...
go vet ./...
$env:GOARCH = '386'
node cmd/archscope-app/build/windows/test.mjs -json -count=1 -p 2 -timeout 10m ./...
go vet ./...
```

저장소 루트에서 `node scripts/capture-soak.mjs --duration 10m --output ABS_JSON`,
`python scripts/build-offline-assets.py --check`를 실행하고, engine 디렉터리에서
`go run ./cmd/archscope-engine offline fixtures`를 확인한다.
앱은 `build/windows/build.mjs`로 `production`과 실제 HEAD의
`-X main.desktopBuildCommit=COMMIT`을 지정해 빌드한다.
그 바이너리에 `verify-http-standalone.mjs --exe ABS_EXE --out PRIVATE_DIR`를
실행한다. 전체 parity UNVERIFIED는 하니스 실패를 뜻하지 않는다.

## 푸시 후 CI와 빈 캐시 추가 수정

첫 푸시 `b85d43cb`의 [General CI](https://github.com/aurakimjh/archscope/actions/runs/43210742956),
[Windows COM](https://github.com/aurakimjh/archscope/actions/runs/43210844096),
[Windows GUI Smoke](https://github.com/aurakimjh/archscope/actions/runs/43210844069)는
통과했다. COM은 amd64 race와 native 386, GUI는 양쪽 production 실행을 포함한다.

첫 [Engine Native 실행](https://github.com/aurakimjh/archscope/actions/runs/43210742986)의
Linux-hosted x86 GUI 교차 빌드는 빈 모듈 캐시에서 `module.Dir`가 없어 실패했다.
`go list -m -json`에 소스 경로가 없는 경우 선택된 Wails 버전을 먼저 다운로드하고,
응답의 모듈 경로·버전·디렉터리를 확인하도록
`9486ab0d656f60f3bf8b7f4459f5cb59cf44b77a`에서 수정했다. 기존 v3.1.0 고정,
정확한 소스 해시 검사와 임시 SDK 복사본 패치는 유지한다.

동일 오류를 로컬의 비어 있는 별도 GOMODCACHE에서 재현했다. 다른 빈 캐시로
수정된 도우미를 실행해 실제 최소 Go 프로그램 컴파일을 통과했고, 일반 캐시의
x86 production 앱 빌드 및 native live-capture E2E도 다시 통과했다.
최소 프로그램 검사는 전체 cold-cache 제품 빌드로 과장하지 않는다.
[cold-cache.json](cold-cache.json)에 범위와 종료 코드를 기록했다.

이 수정 후 [x86/x64 GUI 교차 빌드](https://github.com/aurakimjh/archscope/actions/runs/43211422903),
[Windows GUI Smoke](https://github.com/aurakimjh/archscope/actions/runs/43211422838),
[General CI](https://github.com/aurakimjh/archscope/actions/runs/43211422704)는 해당
검증을 통과했다. Engine Native 전체 워크플로의 통과를 뜻하지는 않는다.

첫 실행의 Linux/macOS race에서는 배포 정합성 검사 두 개의 20초 테스트
context가 만료됐다. 10개 importer를 실행하는 기능 검사이며 시간 SLO가 아니므로,
`a4cff300`에서 테스트 context를 3분으로 조정했다. 개수·해시·위변조·누락·
symlink 거부 assertion과 제품 제한은 유지한다. 양쪽 Windows의 distribution
패키지 테스트와 vet를 다시 통과했다.

macOS analysis 패키지는 누적 scale/import/export 테스트가 Go의 기본 패키지
전체 10분 제한에 도달했다. 당시 마지막 실행 테스트는 시작한 지 5초였으며
JSON 처리 중이었다. `3fa51b1f`에서 CI race 명령에 패키지 한도 `-timeout 20m`을
명시했다. 개별 테스트와 검증 입력을 줄이거나 건너뛰지 않는다.
이미 대체된 중간 Engine Native 실행은 중복 부하를 줄이기 위해 취소했다.
그 실행에서 완료된 job의 실패·성공은 각각 보존한다.

이후 [Engine Native](https://github.com/aurakimjh/archscope/actions/runs/43212869637)에서
Linux race가 통과했다. analysis 505.955초, distribution 46.759초였다.
[General CI](https://github.com/aurakimjh/archscope/actions/runs/43212869574)도 통과했다.
하지만 macOS·Windows race의 최종 집계에서는 아래 테스트용 context 만료가
추가로 확인되어 이 Engine Native 실행의 종합 결과는 FAIL이다.

- 54,001노드 import 대기 3분 초과: 입력·정확한 노드/가중치·저장 바이트 동일성과
  128바이트 제한의 거부 assertion은 유지하고, import 대기만 5분으로 조정했다.
  saved-result load와 후속 거부 검사에는 기존 독립 3분 context를 유지한다.
- 렌더링 재생성·명시적 upgrade가 같은 120초 context를 재사용했다. 각 렌더링에
  별도의 120초 context를 주어 원래 세션당 한도를 유지했다. 취소, 호출자 격리,
  과거 PNG 불변과 암묵적 선택 변경 거부 검사는 그대로다.
- catalog 초기화·선택·deactivate·import·reopen이 같은 20초를 공유했다.
  각 작업에 별도의 20초 context를 주고, pack/provenance 고정·별칭 격리·
  소유자의 비활성화 상태 보존 assertion은 유지했다.

대형 fixture들이 서로 CPU/메모리를 과도하게 경쟁하지 않도록 CI race 명령의
패키지 동시 실행 수를 `-p 2`로 고정했다. 어떤 테스트도 제외하지 않는다.
기존 전체 로컬 suite는 `3b968ea9`의 Go 코드 검증이다. 후속 수정 뒤 영향받은
app/analysis/catalog 전체 패키지와 vet를 양쪽 Windows에서 다시 검증한다.
이 시점까지의 추가 변경은 빌드 도우미, 테스트 context, CI 실행 범위의 시간/
동시성 설정과 검증 문서다. 입력 크기·정합성 assertion·제품 시간 제한은 유지한다.
최종 job·step 판정은 [ci.json](ci.json)에 기록한다.

## 동시 원격 변경 통합과 T-644 Windows 수정

푸시 직전 원격에 별도 T-643 리뷰 및 T-644 구현 `09413bfe`가 추가돼
non-fast-forward 거부를 받았다. 아직 공개하지 않은 context 수정 한 커밋만
원격 위에 rebase했고, 검증 문서는 명시한 두 경로만 잠시 stash한 뒤 복원했다.
기존 원격 이력과 로컬 미추적 파일은 보존했다. context 수정의 실제 통합
커밋은 `7a1c1dfe`다.

새 `TestT644ActualCLIAndCommonConsumers`에도 확장자 없는 실행 파일 경로가
있어 Windows 실패를 재현했다. 같은 `buildBinary(t)`를 재사용했다.
그 다음 실제 Nsight SQLite 정규화가 `GPU_TRACE_SQLITE_READONLY_SETUP`으로
실패했다. 드라이브 경로가 `file://C:/...` 호스트 부분으로 해석되는 문제였다.
`97501f86`에서 URI path에 선행 `/`를 넣어 `file:///C:/...`로 전달한다.
Unix 절대 경로는 그대로다. `mode=ro`, `immutable=1`, query_only,
trusted_schema 차단과 고정 schema/column 검사도 유지한다.

이 수정 후 실제 정규화 CLI와 SQLite schema-spoof/whole-interval 회귀가
Windows에서 통과했다. 통합 초기 실행의 app/CLI/parser 실패는 별도 로그로
보존했다. 최종 통합 코드는 양쪽 전체 Go suite/vet, 새 프런트엔드 상태/빌드,
11개 오프라인 importer fixture와 양쪽 production 기동/제어 하니스로 재검증했다.
기존 21-route/Headers/Body 이미지의 빌드 commit은 native-ui.json에 명시했으며
새 GPU 화면의 시각 인수로 확대하지 않는다. 원본 SQLite fixture는 오프라인
import 근거이며 Windows CUDA/NCCL producer 수집 인수를 뜻하지 않는다.
