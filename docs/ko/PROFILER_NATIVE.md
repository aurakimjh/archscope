# ArchScope Profiler — 네이티브 빌드 (Wails v3)

웹 빌드와 동일한 분석기를 실행하지만, 단일 ~10 MB 실행파일로 배포되는
독립형 데스크탑 프로파일러입니다. `apps/profiler-native/` 아래에 있습니다.

## 제공 기능

- **입력 형식 5종** — async-profiler `*.collapsed`, Jennifer APM 플레임그래프
  CSV, FlameGraph SVG (Brendan default + icicle), async-profiler 자체완결
  HTML, 인라인 SVG가 포함된 HTML.
- **드릴다운 엔진** — include / exclude / regex include / regex exclude ×
  anywhere / ordered (`a > b > c`) / subtree × preserve full path /
  reroot at match. ReDoS 안전 (RE2 + 500자 패턴 한도).
- **프로파일러 비교** — A/B 비교, 합계 정규화 옵션, 발산 팔레트 플레임그래프
  (빨강 = 악화, 초록 = 개선, 회색 = 변화없음), top-30 증가/감소 테이블.
- **pprof 내보내기** — `go tool pprof`와 호환되는 `.pb.gz`.
- **휴대 가능한 파서 디버그 로그** — 현장 사용자가 발송 가능한 단일 redacted
  JSON 산출물; Python 구현과 동일 shape이라 두 엔진 간 비교 가능.
- **라이트/다크/시스템 테마**, **en/ko 로케일**, **드래그 앤 드롭 파일 입력**,
  **최근 파일 (최근 5개, 영속화)**, **접을 수 있는 사이드바**, **Settings
  페이지** (기본값: sample interval / top-N / profile kind).
- **취소 가능한 비동기 분석** — 긴 분석은 spinner 오버레이에서 중단 가능.

## 설치 (개발자 모드)

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.84
cd apps/profiler-native/cmd/archscope-profiler-app
npm install --prefix frontend
wails3 task package
open bin/archscope-profiler.app   # macOS
# Linux: ./bin/archscope-profiler
# Windows: bin\archscope-profiler.exe
```

`.github/workflows/profiler-native.yml`의 CI 매트릭스가 push마다
플랫폼별 다운로드 가능한 아티팩트를 만듭니다 (`archscope-profiler-
darwin-arm64` / `darwin-amd64` / `windows-amd64` / `linux-amd64`).

## 시스템 사전 조건

| 플랫폼 | 필요 사항 |
|---|---|
| macOS 12+ | 없음 — WKWebView가 OS에 포함됨. |
| Windows 11 | 없음 — WebView2가 OS에 포함됨. |
| Windows 10 | WebView2 Evergreen 런타임 (Microsoft 설치 프로그램; NSIS 부트스트래퍼가 자동 처리). |
| Linux | `libgtk-3-dev` + `libwebkit2gtk-4.1-0` (Debian/Ubuntu) 또는 `webkit2gtk4.1` (Fedora). |

## 레이아웃

```
사이드바 (접기 가능)         탑바 (테마 + 로케일)
├ Profiler  ─┐
├ Compare   │  파일 스트립 (path + format + interval/top-N/elapsed/profile-kind)
└ Settings  │  탭: Summary · Flamegraph · Charts · Drill-down · Diagnostics
```

탭은 첫 분석 성공 전까지 자동 비활성화; 경고/에러가 있으면 Diagnostics
탭에 빨간 배지가 표시됩니다.

## Profiler 탭

1. 윈도우에 파일을 드롭하거나 **파일 선택**을 클릭 (또는 절대 경로 붙여넣기).
   확장자에서 형식 자동 감지; 드롭다운으로 재정의 가능.
2. 샘플 간격, 경과 시간(초), top-N, profile kind, 선택적 timeline base
   method를 조정.
3. **분석** 클릭. spinner 오버레이의 **취소** 버튼이 백엔드 goroutine
   작업을 중단합니다; 옵션 영역은 분석 중 잠기고 성공 시 자동으로 접힘
   (다시 펴려면 **옵션 편집** / **옵션 숨기기**).

### Summary

- 총 샘플 수, 추정 초, 간격(ms), 경과 시간(초), profile kind, 파서.
- Timeline scope 카드 — mode / base method / match mode / view mode /
  base samples / base ratio / 경고 (예: `TIMELINE_BASE_METHOD_NOT_FOUND`).
- 상위 자식 프레임 테이블.
- Top stacks (15개, 내부 스크롤).

### Flamegraph

Canvas 렌더링, HiDPI 대응, 클릭으로 줌 (**줌 초기화** 포함), hover 툴팁,
**Save PNG** (`canvas.toDataURL`).

### Charts

- 실행 분류 가로 바 (카테고리별 샘플 + 비율).
- 타임라인 분석 가로 바 (segment 비율).

### Drill-down

필터 추가 (패턴 + 필터 유형 + 매칭 모드 + 보기 모드 + 대소문자 구분).
스테이지에 breadcrumb + 필터 chip (제거 버튼 포함) + 스테이지 metrics +
스테이지별 Canvas flamegraph가 표시됩니다.

필터 유형: `include_text`, `exclude_text`, `regex_include`,
`regex_exclude`. 매칭 모드: `anywhere`, `ordered` (a > b > c),
`subtree`. 보기 모드: `preserve_full_path`, `reroot_at_match`.

### Diagnostics

- 누적 카운트 + skipped 사유 chip cloud.
- 심각도별 색상 sample 리스트 (에러 / 경고 / 샘플) + 원본 라인 미리보기.
- **디버그 로그 저장**은 `<cwd>/archscope-debug/` 아래에 휴대 가능한
  redacted JSON을 작성합니다; 판정 단계는
  `CLEAN → PARTIAL_SUCCESS → MAJORITY_FAILED → FATAL_ERROR`.

## Compare 탭

두 파일을 선택 (5종 형식 어떤 조합이든), 선택적으로 **합계 정규화**,
**비교 실행**. 출력: 발산 flamegraph + 가장 큰 악화/개선 summary 카드,
top-10 증가/감소 테이블.

## Settings 탭

- 언어 (English / 한국어).
- 테마 (Light / Dark / System) — `prefers-color-scheme` 청취.
- 기본 sample interval / top-N / profile kind (`localStorage`의
  `archscope.profiler.defaults`에 영속).
- 최근 파일 뷰어 + **초기화**.

## 내보내기 대상

- **PNG** — flamegraph (Canvas의 Save PNG 버튼).
- **pprof `.pb.gz`** — Profiler 탭 strip의 Export as pprof 버튼.
  `go tool pprof bin/archscope-profiler-export.pb.gz`로 열기.
- **휴대 가능한 디버그 로그** — Diagnostics 탭의 디버그 로그 저장 버튼.

## CLI

동일한 Go 코어가 스크립팅과 parity 테스트용으로 `cmd/archscope-profiler`
명령으로 노출됩니다:

```bash
go run ./cmd/archscope-profiler \
  --collapsed examples/profiler/sample-wall.collapsed \
  --interval-ms 100 --elapsed-sec 1336.559 \
  --timeline-base-method Job.execute \
  --top-n 20 \
  --debug-log --debug-log-dir /tmp/archscope-debug
```

`--collapsed`, `--jennifer-csv`, `--flamegraph-svg`, `--flamegraph-html`은
상호 배타적입니다.

## 문제 해결

| 증상 | 해결 |
|---|---|
| macOS 실행 시 "실행파일이 없습니다" 오류 | `.app` 번들의 `CFBundleExecutable`이 `Contents/MacOS/` 안의 바이너리와 일치해야 합니다. `build/config.yml` 수정 후 `wails3 task package` 재실행 (또는 `wails3 task common:update:build-assets`로 `Info.plist` 재생성). |
| Windows 10 실행 시 WebView2 관련 에러 | WebView2 Evergreen 런타임 설치 (또는 부트스트래퍼가 포함된 NSIS 설치 프로그램 사용). |
| Linux 실행 시 `libwebkit2gtk-4.1.so.0 not found` | `apt install libwebkit2gtk-4.1-0` (Debian/Ubuntu) 또는 `dnf install webkit2gtk4.1` (Fedora). |
| 드래그 앤 드롭이 경로를 채우지 않음 | 일부 webview는 `File.path`를 노출하지 않습니다; 드롭은 파일명으로 fallback. **파일 선택**을 클릭하거나 경로를 붙여넣으세요. |
| macOS 격리 속성 차단 (`com.apple.quarantine`) | `xattr -dr com.apple.quarantine /path/to/archscope-profiler.app`. |

## 교차 참조

- 멀티 플랫폼 패키징 — `apps/profiler-native/cmd/archscope-profiler-app/PACKAGING.md`.
- CI 매트릭스 — `.github/workflows/profiler-native.yml`.
- 공유 분석기 (Python parity 참조) —
  `engines/python/archscope_engine/analyzers/profiler_*.py`.
- 로드맵 — `work_status.md`의 "Go/Native Profiler Follow-up" 섹션.
