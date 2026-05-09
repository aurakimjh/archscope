# 성능 측정

성능 측정 대상은 이제 Go 엔진과 Wails 데스크톱 빌드입니다.

## 기준 명령

```bash
cd apps/engine-native
go test ./...
go test -bench=. -run=^$ ./internal/profiler
go build -trimpath -ldflags="-s -w" ./cmd/archscope-engine ./cmd/archscope-profiler
```

프론트엔드 빌드 크기:

```bash
cd apps/engine-native/cmd/archscope-profiler-app/frontend
npm ci
npm run build
```

## 예산

- 데스크톱 바이너리는 현장 직접 배포가 가능한 크기를 유지합니다.
- Electron 또는 HTTP 서버를 릴리스 바이너리에 다시 넣지 않습니다.
- 대용량 profiler, GC, access-log, thread-dump 입력에는 streaming parser와
  bounded diagnostics를 우선합니다.
