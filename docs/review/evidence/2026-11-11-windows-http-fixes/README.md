# Windows 발견 문제 3건 수정·재검증 — 2026-11-11

[이전 실행](../2026-11-11-windows-http-followup/README.md)에서 발견한 세 문제를 수정하고
현재 물리 Windows PC에서 다시 검증했다. **세 문제의 수정·재검증은 모두 통과했다.**
이 기록은 `82bc6a8c3fd86ebb99901892cdb41f56134d6800` 이후 커밋하지 않은 작업 트리의 결과다.
정확한 수정 소스·바이너리·로그 해시는 [validation.json](validation.json)에 기록했다.

## 수정 내용

| 문제 | 수정 |
|---|---|
| 진단 기능 테스트 9개의 fixture/schema 해시 불일치 | `.gitattributes`에서 내장 catalog/case 계약 파일의 LF checkout을 고정하고 현재 작업 트리도 LF로 갱신. manifest와 런타임 SHA-256 검증은 그대로 유지 |
| x86 바로가기 테스트의 간헐적 `invalid marker` | 자식이 같은 디렉터리의 `.pending` 파일에 JSON을 모두 쓴 뒤 최종 이름으로 rename. 부모가 쓰는 도중의 파일을 읽을 수 없게 함 |
| x86 앱 `go test`의 Wails callback 초기화 panic | 기존 production 빌드의 hash-guarded 임시 SDK/modfile 처리를 `go.mjs`로 공유하고 `test.mjs` 및 `task windows:test` 추가. CI에도 native 386 전체 테스트 job 추가 |

공유 runner는 Wails 3.1.0의 버전·원본 파일 해시를 확인한 뒤 임시 복사본만 패치한다.
모듈 캐시의 `idroptarget.go`, `user32.go` 해시가 검증 전후 동일함을 확인했다.
의존성 버전이나 사용자 시스템 설정은 바꾸지 않았다.

## Windows 재검증

| 검증 | 결과 |
|---|---|
| amd64 `go test -count=1 -p 1 -timeout 5m ./...` | PASS; 실패 0 |
| 386 shared runner로 동일 전체 Go 테스트 | PASS; 실패 0, 앱 초기화 panic 없음 |
| 386 catalog/cases/앱 집중 테스트 | PASS; 이전 진단 기능 실패 포함 |
| 바로가기 실제 자식 프로세스 반복 | amd64 100/100, 386 100/100 PASS |
| 양쪽 아키텍처 `go vet ./...` | PASS |
| 양쪽 아키텍처 production 빌드·앱 제어 harness | PASS; 각 16개 parity case 중 4개 PASS, 종합 UNVERIFIED |
| `task windows:test ARCH=386`로 실제 라이브 캡처 E2E | PASS |
| `core.autocrlf=true` checkout 변환 | 계약 파일 12개 모두 LF 및 동일 바이트 |
| JS 구문, workflow/Task YAML 구문, desktop-only/server-build 거부, diff 검사 | PASS |

현재 full suite에서 선택적으로 건너뛴 항목은 JSON에 명시했다. 테스트 helper 단독 실행,
비대상 OS 전용 테스트, 미설정 private/independent fixture, 기존 COM 등록 보존,
본문/강제 종료/투명 캡처의 opt-in gate 등을 전체 통과 범위에 포함시키지 않는다.

이번 변경에는 프런트엔드 소스 수정이 없어 이전 실행의 frontend production 산출물을 사용했다.
새 CI job은 설정·구문을 검증했으며 이 작업에서 push하거나 hosted CI를 실행하지 않았다.

## 재실행

`apps/engine-native`에서:

```powershell
$env:GOARCH = 'amd64'
go test -count=1 -p 1 -timeout 5m ./...
go vet ./...

$env:GOARCH = '386'
node cmd/archscope-app/build/windows/test.mjs -count=1 -p 1 -timeout 5m ./...
go vet ./...
```

앱 디렉터리에서는 다음 Task 진입점도 사용할 수 있다:

```powershell
task windows:test ARCH=386 -- -count=1 .
```

x86 앱의 직접 `go test`는 외부 Wails SDK의 기존 오류를 우회하지 않으므로 공통 runner를 사용한다.
이전 checkout은 계약 파일을 LF로 갱신해야 한다. 새 checkout에는 `.gitattributes`가 적용된다.

## 남은 인수 범위

이번 수정으로 T-601/T-611/T-613/T-627의 전체 인수 상태를 변경하지 않는다.
관리자·WinDivert·peer IPv4 HTTP 80 fixture, 덤프 정책 격리·복원,
원본 클라이언트 호환성/clean-host COM, 실제 UI·DPI 및 수 시간 GUI/드라이버 검증은
[이전 준비사항](../2026-11-11-windows-http-followup/README.md)에 따라 별도로 남아 있다.
로컬 race 검사는 C compiler가 없어 실행하지 않았다.
