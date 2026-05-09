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

## 대용량 파일 정책

현재 Go 엔진은 대용량 입력을 브라우저 업로드가 아니라 현장 오프라인 분석
워크로드로 취급합니다.

- 텍스트 로그 파서는 `internal/textio.ForEachTextLine`을 사용해 파일을
  `ReadAll`하지 않고 라인 단위로 디코딩해야 합니다.
- GC 로그 차트 series는 `MaxSeriesPoints`로 제한하고 deterministic
  downsampling을 적용합니다. summary metric과 finding은 전체 이벤트를
  기준으로 계산합니다.
- Access-log와 OTel analyzer 진입점은 parser callback에서 바로 집계합니다.
  OTel은 summary counter는 전체 기준으로 유지하되 trace별 상세 row 보관 수를
  제한합니다.
- JFR JSON 직접 로딩에는 파일 크기 preflight를 둡니다. 큰 recording은
  `jfr print --events`, 시간 구간, stack-depth 필터로 줄인 뒤 분석합니다.
- Jennifer profile export는 TXID block 단위로 streaming segmentation하여
  하나의 transaction block을 파싱한 뒤 해제할 수 있게 합니다.
- Java jstack section parsing은 line streaming을 사용합니다. jcmd JSON,
  Node diagnostic report, .NET clrstack 같은 구조화 thread-dump 포맷은
  multi-GB 사용 전에 size preflight 또는 포맷별 streaming 처리를 유지해야 합니다.
- HTML profiler 입력은 직접 파싱 전에 크기를 확인합니다. SVG 파서는 추가
  전체 문자열 복사를 피하기 위해 byte reader를 사용합니다.

권장 UI/CLI 경고 기준:

| 입력 크기 | 정책 |
|---:|---|
| 100 MB+ | 대용량 안내와 사용 가능한 필터를 표시합니다. |
| 500 MB+ | 포맷이 지원하면 `max_lines`, event filter, time window를 우선 사용합니다. |
| 1 GB+ | stream-only 경로를 사용하고, 직접 JSON/HTML ingest는 명시적 필터가 있을 때만 허용합니다. |
