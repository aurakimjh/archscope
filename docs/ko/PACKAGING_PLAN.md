# 패키징 계획

## 폐쇄망 배포 개선안 (2026-11-11, 미구현)

[통합 설계 §17](./LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md#17-로컬-보안과-공유-자산)의
새 요구다. 아래 기존 빌드 기록은 해당 날짜의 증거이며 이 요구의 검증 결과가 아니다.

- 앱에 문서·schema·검증된 fixture·차트 자산·라이선스 확인한 한글 font를 포함한다. Windows는 지원되는 WebView2 offline/fixed runtime 경로를 검증한다.
- 지식 팩은 ID/version/호환 범위·파일 hash·license·fixture manifest와 신뢰 키의 분리 서명을 가진 data-only archive다. staging 검증·conformance 후 atomic 활성화하고 실행 중 작업은 기존 버전을 고정한다. rollback과 오프라인 키 폐기 목록을 관리한다.
- 수집 스크립트는 data-only 팩과 분리해 사용자가 수동 실행한다. perf/JDK/Nsight/.NET 변환은 수집 환경 우선이며 분석 PC의 명시적 기존 converter 사용도 허용한다. 자동 다운로드는 하지 않는다.
- SQLite는 후보 의존성이다. pure-Go prototype을 우선 비교하고 cgo 선택 시 OS/arch별 배포·라이선스·크기를 별도 검증한다.
- release CI는 버전/hash가 고정된 필수 fixture bundle 누락을 FAIL로 처리한다. sibling `projects-assets`가 없으면 skip하는 개발 편의 동작을 release 증거로 쓰지 않는다.
- 깨끗한 설치·네트워크 차단 환경에서 importer/export/도움말/pack import와 DNS/socket/updater/telemetry/font/symbol 외부 연결 **시도 0건**을 확인한다. 기존 성공 여부와 별도의 수용 기준이다.

## 기존 패키징 현황

ArchScope는 Go/Wails 데스크톱 애플리케이션으로 패키징합니다. 이전
Python wheel 및 FastAPI browser server 경로는 폐기되어 `archive/`에
보관되었습니다.

## 현재 모델

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build

cd ..
GOCACHE=/tmp/aiservice-go-cache task package
```

산출물은 `apps/engine-native/cmd/archscope-app/bin/`에서
생성됩니다.

최신 로컬 검증(2026-05-09):

- `task` 3.50.0과 `wails3` v3.0.0-alpha.87가 `/opt/homebrew/bin`에
  설치되었습니다.
- Vite는 8.0.11, `@vitejs/plugin-react`는 6.0.1로 올렸습니다.
- `npm audit` 결과 취약점 0건입니다.
- macOS package build 성공: raw binary 11 MB, `.app` bundle 13 MB.
- 남은 Vite 경고는 bundle-size 관련이며 보안 취약점이 아닙니다.

## CI와 릴리스

- `.github/workflows/ci.yml`은 Go 테스트와 Wails frontend build를 수행합니다.
- `.github/workflows/engine-native.yml`은 cross-platform Go/Wails 검증
  matrix입니다.
- `.github/workflows/release.yml`은
  `apps/engine-native/cmd/archscope-app`에서 Wails 앱을 패키징합니다.

## Historical Notes

Electron은 배포 크기 문제로 제거되었습니다. 이후 Go/Wails 바이너리가 목표
배포 모델에 충분히 작다는 것이 확인되어 Python wheel/FastAPI web 경로도
폐기되었습니다.
