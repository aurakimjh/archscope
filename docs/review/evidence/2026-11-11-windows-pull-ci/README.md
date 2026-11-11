# Pull 변경사항 Windows 검증 및 CI 수정 — 2026-11-11

대상 pull: `e2c1c7d9ef7d0f1e3dafed00a03ee4543ebfaef0`.
수정 커밋: `5b5837454226515be34891e563210d9b6e2c128b`.
이전에 푸시한 `97e518dd`와 pull 대상의 CI를 함께 확인했다.
최종 로컬 결과·로그/바이너리 해시·호스티드 job 결과는 [validation.json](validation.json)에 기록한다.

## 재현한 실패와 수정

- **Windows CLI 테스트 2개:** 임시 실행 파일을 확장자 없는 `engine`으로 빌드해
  `os/exec`에서 찾지 못했다. T-630 R6 실제 case/cache CLI와 T-631 provider CLI 테스트가
  모두 실패했다. Windows `.exe`를 처리하는 기존 `buildBinary(t)`를 재사용했다.
- **대형 프로파일 테스트:** 54,001노드 검증에서 동일한 3분 context를 import 대기,
  저장 결과 다시 열기, 후속 record-limit 거부 검증에 재사용했다. CI race 계측으로
  앞 단계가 오래 걸리면 macOS에서는 load, Linux/Windows에서는 후속 Wait가 만료됐다.
  각 작업에 독립된 3분 context를 주고 import 대기가 끝나면 그 context를 즉시 취소한다.
  실제 데이터 크기·바이트 보존·128바이트 제한의 엄격한 거부 assertion은 유지했다.

제품의 timeout/budget, 실행 기능, 파일 형식, 의존성은 바꾸지 않았다.
수정은 테스트 코드 세 파일이다.

## 로컬 Windows 결과

| 범위 | 결과 |
|---|---|
| 최초 amd64 전체 Go 테스트 | CLI 실행 테스트 2개 실패 재현 |
| 수정 후 amd64 전체 Go 테스트 | 1,224 PASS / 0 FAIL / 14 SKIP |
| 수정 후 x86 app/CLI/analysis/cases/catalog/resultcontract | 151 PASS / 0 FAIL / 5 SKIP |
| 최종 context 분리 후 실패 회귀 3개 | amd64 3/3, x86 3/3 PASS |
| 양쪽 아키텍처 전체 Go vet | PASS |
| 변경된 프런트엔드 상태 회귀·production 빌드 | PASS |
| production 앱 빌드·실제 캡처 제어 harness | 각 아키텍처 PASS; 각 4/16 parity case PASS, 전체 UNVERIFIED |

전체 amd64 및 x86 변경 패키지 검증 뒤 최신 CI의 추가 load-context 만료도 확인해
load에 독립 context를 적용했다. 이 최종 변경은 실패했던 세 회귀를 양쪽 아키텍처에서
다시 실행해 검증했다. 최종 커밋 전체 코드의 race/전체 테스트는 아래 호스티드 CI로 확인한다.
프런트엔드와 production 바이너리는 pull 대상 제품 코드로 빌드했으며 이후 수정은 테스트 전용이다.

## 호스티드 CI

- [이전 Windows 수정 커밋 Engine Native](https://github.com/aurakimjh/archscope/actions/runs/43027509375): macOS 대형 프로파일의 후속 Wait 만료.
- [Pull 대상 Engine Native](https://github.com/aurakimjh/archscope/actions/runs/43028924763): Windows CLI 2개 및 세 OS의 대형 프로파일 context 만료.
- [수정 커밋 Engine Native](https://github.com/aurakimjh/archscope/actions/runs/43030540562).
- [수정 커밋 General CI](https://github.com/aurakimjh/archscope/actions/runs/43030540850).

수정 커밋의 Engine Native 8개 job과 General CI가 모두 성공했다.
Windows amd64 race/full/live/body/SEC-10 정책 복원, native 386 전체 테스트,
Linux/macOS race/full, 양쪽 GUI 크로스빌드, frontend 및 macOS 패키징을 포함한다.

## 재실행

`apps/engine-native`에서:

```powershell
go test -count=1 -p 1 -timeout 5m ./...
go vet ./...

$env:GOARCH = '386'
node cmd/archscope-app/build/windows/test.mjs -count=1 -p 1 -timeout 5m ./cmd/archscope-app ./cmd/archscope-engine ./internal/analysis ./internal/cases ./internal/catalog ./internal/resultcontract
go vet ./...
```

네이티브 Go 서비스/CLI 검증과 앱 기동·캡처 제어를 실제 새 generic UI/DPI 전체 인수로
확대하지 않는다. 로컬 race 검사는 C compiler가 없어 실행하지 않았으며,
호스티드 amd64 Windows/Linux/macOS race 결과와 구분한다. 관리자·드라이버·본문 덤프 정책이
필요한 인수 항목과 독립 리뷰 상태는 이번 작업으로 변경하지 않는다.
