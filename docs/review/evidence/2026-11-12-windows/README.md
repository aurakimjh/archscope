# Windows 미완료 검증 후속 실행 — 2026-11-12

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

T-601/T-611/T-613/T-627의 상태와 전체 7개 미완료 task 수는 유지한다.

- T-601: 비관리자 호스트이며 WinDivert와 외부 IPv4 port-80 fixture가 없다.
  실제 드라이버 주입·서비스·프록시 무시 클라이언트 범위는 미검증이다.
- T-611: 원본 등록을 보존했다. clean-host 공개 등록, 실제 vendor early-bound
  client, 추가 cache-origin 근거와 타 프로세스의 메모리 session-cookie
  상호운용이 남는다.
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
