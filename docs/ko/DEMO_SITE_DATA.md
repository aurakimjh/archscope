# Demo-site 데이터 실행 흐름

ArchScope는 `../projects-assets/test-data/demo-site`에 있는 공유 demo-site
manifest를 읽어 실행할 수 있습니다. 데이터 원본은 이 저장소에 복사하지 않고,
ArchScope는 manifest를 읽어 로컬 출력 디렉터리에 report bundle을 생성합니다.

## CLI

전체 demo-site 시나리오 실행:

```bash
./scripts/run-demo-site-data.sh
```

특정 시나리오 실행:

```bash
cd apps/engine-native
go run ./cmd/archscope-engine demo-site run \
  --manifest ../../../projects-assets/test-data/demo-site/synthetic/gc-pressure/manifest.json \
  --out /tmp/archscope-demo-bundles
```

출력 구조:

```text
<out>/
  index.html
  synthetic/<scenario>/
    index.html
    run-summary.json
    *-<analyzer_type>.json
    *-<analyzer_type>.html
    *-<analyzer_type>.pptx
    normal-baseline-vs-<analyzer_type>.json
    normal-baseline-vs-<analyzer_type>.html
  real/<scenario>/
    ...
```

`run-summary.json`에는 analyzer 출력, 실패 analyzer, skipped line 수,
reference-only correlation 파일, 핵심 지표, finding, comparison report 목록이
기록됩니다. 시나리오 `index.html`은 같은 정보를 portable report로 보여줍니다.

## Analyzer Type Mapping

demo-site manifest 매핑의 canonical source는 다음 파일입니다.

```text
../projects-assets/test-data/demo-site/analyzer_type_mapping.json
```

ArchScope는 demo-site manifest 실행 시 이 JSON을 읽습니다. Command rendering은
Go demo-site runner가 처리하며, 필요한 경우 `--mapping <path>`로 override할 수
있습니다. 현재 감지되는 manifest 목록은 다음 명령으로 확인합니다.

```bash
cd apps/engine-native
go run ./cmd/archscope-engine demo-site list \
  --manifests ../../../projects-assets/test-data/demo-site
```

예시:

| manifest `analyzer_type` | CLI command |
|---|---|
| `access_log` | `access-log analyze` |
| `profiler_collapsed` | `profiler analyze-collapsed` |
| `profiler_collapsed` + `format: jennifer_csv` | `profiler analyze-jennifer-csv` |
| `jfr_recording` | `jfr analyze-json` |
| `otel_logs` | `otel analyze` |

## Desktop UI

Demo Data Center는 다음 흐름을 지원합니다.

- demo manifest root 선택
- `real`, `synthetic`, 전체 데이터 필터
- 단일 시나리오 또는 표시되는 전체 시나리오 실행
- 생성된 JSON/HTML/PPTX/index 파일 열기
- JSON 출력을 Export Center로 전달
- 실패 analyzer, skipped line, finding, reference-only context 요약 표시

현재 제품 라인은 폐기된 FastAPI handler가 아니라 Go CLI/Wails engine 경계로
demo scenario를 실행합니다. UI는 engine task가 끝날 때까지 running 상태를
유지하며, streaming progress event는 별도 후속 작업입니다.

레거시 Electron 전용 Playwright smoke test는 Phase 1에서 desktop shell이
폐기될 때 함께 제거되었습니다.

## OpenTelemetry 시나리오 검증

OTel 분석은 `parent_span_id` 또는 호환 alias가 있으면 parent span 관계를
우선 사용해 service path를 재구성합니다. parent span 정보가 없으면 timestamp
순서로 fallback합니다. Demo manifest의 기대 service와 trace 수는 result
metadata에 기록되며, 분석 결과와 다르면 finding으로 표시됩니다.
