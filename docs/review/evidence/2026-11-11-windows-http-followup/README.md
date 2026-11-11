# Windows HTTP 캡처 재실행 — 2026-11-11

이 실행에서 발견한 세 문제는 [후속 수정·재검증](../2026-11-11-windows-http-fixes/README.md)에서
해결했다. 아래 최초 실패 기록은 그대로 보존한다.

대상 커밋: `82bc6a8c3fd86ebb99901892cdb41f56134d6800`.
현재 물리 Windows PC에서 자동화 테스트를 실행했다. **일부 통과이며 전체 통과가 아니다.**
제품 코드와 기존 작업 상태는 변경하지 않았다. 세부 결과·바이너리 해시·실패 목록은
[validation.json](validation.json)에 기록했다. 실행 로그와 바이너리는 로컬 `.tmp`에 있다.

## 실행 결과

| 검증 | 결과 |
|---|---|
| amd64 캡처 내부 패키지, HTTP 파서·분석기 | 26개 패키지 모두 PASS |
| amd64 앱 전체 테스트 | 최상위 테스트 50 PASS / 9 FAIL / 3 SKIP; 아래 줄바꿈 문제로 패키지는 FAIL |
| 실제 Windows 라이브 프록시·두 프로세스 선택 | PASS; PID와 시작 시각, 저장·종료 확인 |
| amd64/386 COM ABI·ITypeInfo·별도 프로세스 RPC·이벤트·사전 저장 취소 | PASS; 기존 등록을 보존하므로 clean-host 공개 등록 테스트는 SKIP |
| 원본 V7.6.4.508 설치 디렉터리의 필터 | 두 아키텍처 PASS; 16개 지원, 캐시 출처가 필요한 1개 명시적 거부 |
| 386 캡처·HTTP 파서·분석기 | 25개 패키지 PASS / processlaunch 1개 패키지 FAIL |
| 386 앱 직접 `go test` | 테스트 실행 전 Wails 초기화 panic |
| amd64/386 production 빌드·실제 앱 제어 harness | 각 빌드 PASS; 각 16개 parity case 중 4개 PASS, 종합 UNVERIFIED |
| 2분 서비스·프록시 반복 검증 | PASS; 120.305초, 39회, 10,140건, 실제 보존 행 최대 64 |
| TCP 소유자 측정 | 각 아키텍처 50/50 confirmed; amd64 p95 6.253ms / 386 p95 22.740ms; 성능 합격 기준은 설정하지 않음 |
| 기존 원본 해시와 일치하는 HALOG 복사본 | PASS; 675건·35,463,916바이트 decode/byte-exact reencode |
| 프런트엔드 상태 테스트·production 빌드, Go vet | PASS |
| parity report/합성 fixture 계약, desktop-only/server-build 거부 | PASS |

앱 harness의 네 가지 통과 항목은 CONTROL-01, PROCESS-01, EXCHANGE-01,
SESSION-01이다. 캡처 시작·일시정지·재개·종료, PID/start-time 귀속,
103/302/final 응답, 저장 ID와 선택 항목의 다시 열기를 확인한다.
앱은 native 초기화 후 15초 이상 생존했지만 시각적인 GUI 판정은 하지 않았다.
현재 실행에서 본문·CA·전역 프록시·덤프 정책을 활성화하거나 바꾸지 않았다.
2분 검증은 실제 서비스·프록시를 사용하며 장시간 GUI/드라이버 검증을 대신하지 않는다.

## 발견한 실패와 필요한 조치

1. **진단 기능 테스트 9개: 내장 파일의 CRLF 변환.**
   `internal/catalog/contracts`의 `wire-cases.json`, `server.log`,
   `measurement-descriptor.schema.json`이 Windows 작업 트리에서 CRLF다.
   각 파일을 메모리에서 LF로 바꿔 계산한 SHA-256은 manifest의 기대 해시와 모두 일치한다.
   현재 `.gitattributes`는 이 디렉터리의 LF를 고정하지 않는다.
   필요한 조치: 해시 검증 대상 파일의 LF checkout을 보장한 뒤 앱 재빌드와 전체 테스트.
   이번 실행에서는 원본 파일이나 검증 규칙을 수정하지 않았다.

2. **386 바로가기 테스트: 한 번 `invalid marker`.**
   `TestNativeShortcutLaunchPreservesUnicodeArgumentsAndDirectory`가 최초 실패했다.
   동일 테스트를 10회 다시 실행하니 모두 통과했다. 자식이 `os.WriteFile`로 생성한
   JSON을 부모가 파일 생성 직후 읽는 코드라 부분 읽기 경쟁과 부합하지만 원인을 확정하지는 않았다.
   필요한 조치: marker를 원자적으로 게시하거나 완성된 JSON을 기다리도록 테스트를 보완하고
   양쪽 아키텍처에서 재검증. 최초 suite 결과는 FAIL로 유지한다.

3. **386 앱 직접 테스트: Wails callback panic.**
   `GOARCH=386 go test ./cmd/archscope-app`는
   `compileCallback: argument size is larger than uintptr`로 테스트 시작 전에 종료했다.
   `build/windows/build.mjs`의 검증된 임시 SDK/modfile 패치가 직접 `go test`에는 적용되지 않는다.
   필요한 조치: 동일 패치를 테스트 빌드에 적용하는 runner.
   이 패치를 사용하는 현재 386 production 빌드와 실제 앱 harness는 통과했다.

추가 HALOG 시도에서 오래된 중간 출력물은 `HALOG_INVALID`로 거부됐다.
이 파일의 해시·크기는 기존 승인 원본과 달랐다. 승인 원본 해시로 파일을 다시 찾아
675건 decode/byte-exact reencode를 통과했다. 새 원본 프로그램 COM 상호운용은 실행하지 않았다.

## 남은 인수 테스트 준비사항

| 항목 | 현재 막힌 이유 / 준비사항 |
|---|---|
| T-601 투명 캡처·서비스 범위 | 현재 비관리자이며 WinDivert/peer fixture 미설정. 공식 SHA-256 검증을 통과하는 WinDivert 2.2.2-A DLL/SYS, 관리자 실행, loopback 밖 IPv4 HTTP 80 서버, 프록시 무시 앱·서비스 fixture와 드라이버 정책 조합이 필요 |
| T-611 clean-host 공개 등록 | 기존 원본/제품 COM 등록을 보존하므로 SKIP. 충돌하는 제품·인터페이스 등록이 없는 Windows 사용자/VM에서 등록·교체·해제 검증 필요 |
| T-611 원본 클라이언트 호환성 | 실제 원본 early-bound client와 기대 동작, 관측된 추가 필터·캐시 출처 근거, 다른 프로세스 메모리의 WinINet session-cookie fixture 필요 |
| 본문 E2E·SEC-10 강제 종료 | HKLM LocalDumps가 존재하고 현재 비관리자. 같은 사용자로 실행하는 관리자 worker와 일시 정책 격리·복원 검증 필요; `scripts/with-windows-body-capture-policy.ps1` 사용 |
| amd64 race detector | 현재 CGO_ENABLED=0이고 C compiler 없음. Go 호환 C compiler를 PATH에 준비하고 CGO_ENABLED=1로 실행 |
| 선택적 독립 XLS 검증 | Python은 있으나 xlrd 없음. xlrd가 있는 Python 경로를 ARCHSCOPE_XLS_READER로 지정 |
| T-613 전체 동등성·장시간 GUI | 전체 원본 비교 fixture, 수 시간 연속 GUI 조작·캡처·재진입·재시작·복구 및 드라이버 시나리오 필요 |
| T-627 전체 화면·OS DPI | 원본 기준 화면과 populated/empty/error fixture, 실제 네이티브 메뉴·OS DPI별 관찰 기록 필요. WebView zoom과 OS DPI는 구분 |

## 재실행 진입점

저장소 루트에서 production frontend를 빌드한 뒤 앱 디렉터리에서
`node build/windows/build.mjs`로 아키텍처별 production 바이너리를 만든다.
빌드 옵션은 `-tags production -trimpath -ldflags "-H windowsgui -X main.desktopBuildCommit=82bc6a8c3fd86ebb99901892cdb41f56134d6800"`이며,
바이너리 해시는 JSON에 기록했다.

```powershell
# apps/engine-native에서; amd64 전체 앱 패키지는 현재 CRLF 문제로 실패한다.
go test -json -count=1 -p 1 -timeout 5m ./internal/capture/... ./internal/parsers/httpcapture ./internal/analyzers/httpcapture ./cmd/archscope-app
go vet ./internal/capture/... ./internal/parsers/httpcapture ./internal/analyzers/httpcapture ./cmd/archscope-app

# 저장소 루트에서
node scripts/verify-http-standalone.mjs --exe ABS_EXE --out PRIVATE_OUTPUT
node scripts/capture-soak.mjs --duration 120s --output PRIVATE_REPORT_JSON

# 투명 캡처 준비 완료 후 apps/engine-native에서
$env:ARCHSCOPE_WINDIVERT_DLL = 'ABS_OFFICIAL_DLL'
$env:ARCHSCOPE_TRANSPARENT_FIXTURE_URL = 'http://PEER_IPV4/fixture'
$env:ARCHSCOPE_RUN_TRANSPARENT_E2E = '1'
go test -count=1 -v ./cmd/archscope-app -run '^TestWindowsTransparentDirectClientE2E$'
```

T-601/T-611/T-627은 REVIEW, T-613은 PENDING을 유지한다.
이번 자동화 결과로 남은 인수 항목을 DONE으로 바꾸지 않는다.
