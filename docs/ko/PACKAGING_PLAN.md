# 패키징 계획

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
