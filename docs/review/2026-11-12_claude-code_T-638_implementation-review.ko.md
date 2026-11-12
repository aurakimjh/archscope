# T-638 정확한 메트릭 이력(MetricHistory·OTLP histogram) 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-638 구현 커밋 `2454987`(feat(metrics): implement T-638 history and resolve T-635 review). 직전 `2e0e16c`(T-635 리뷰) 위에 놓이며, 이 커밋은 T-635 리뷰 R12-01~R12-04의 로컬 수정도 함께 포함한다.
- 계약 기준: `work_status.md`의 T-638 행과 완료 기록, `docs/ko/METRIC_HISTORY.md`, `docs/ko/DATA_MODEL.md`·`PARSER_DESIGN.md`·`IMPORTER_SUPPORT_MATRIX.md`·`CASE_EVIDENCE_QUERIES.md`의 T-638 절
- 증거: `docs/review/evidence/2026-11-12-t638/local.json`(`source_sha256` 80개와 `evidence_sha256` 23개가 `2454987` 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(HTTP 문서 변경, 진행 중인 시각화 작업 파일 등)은 수정하지 않았다.
- 방법: 소스 정독(`metricmodel/interval.go`·`model.go`·`query.go`, `parsers/metrichistory/otlp.go`·`source.go`·`json.go`, `analyzers/metrichistory/analyzer.go`, `analysis/metric_history.go`·`baseline_cache.go`, `catalog/metric_history.go`, `cmd_metric_history.go`, Wails `metricsourceservice.go`, `MetricSourceAssistance.tsx`), 저장소 밖 scratch worktree(`2454987`)에서 빌드한 실제 CLI로 OTLP JSON·snapshot·Prometheus range 입력을 직접 만들어 `metrics history-inspect/normalize/import/query/merge`, `evidence import`(JSON·CSV·HTML)를 끝까지 실행했다. 변조·경계·규모 시나리오와 throwaway Go 측정 테스트를 사용했고 측정 후 삭제했다.
- 범위: MetricHistory 모델 1.0.0 / `metrics_timeseries` 0.1.0, OTLP JSON의 gauge/sum/explicit/exponential histogram, 중립 snapshot, Prometheus range matrix, 구간 산술(리셋·겹침·간격·순서), 출처 중복 제거와 merge, 불변 질의·cursor·시간창, CLI·Wails·generic consumer. 그리고 같은 커밋에 들어 있는 T-635 R12-01~R12-04 수정의 확인. Q04(실제 사용자 export·계측 버전·GenAI alias)와 지표 비교/SLI 정책, 시계 정렬은 범위 밖이다.

## 1 결론

**판정: 조건부(P2 1건, P3 6건).** 계산 의미는 요구 계약대로 정확하게 동작한다. 리셋·수명 겹침·설명 없는 감소·시작 시각 누락은 각각 다른 상태로 표시되고 값을 지어내지 않으며, 정확한 분수 rate, 명시적/지수 histogram의 bucket별 차분과 schema 변경 거부, 종료 시각 `[from,to)` 시간창, 변조 4종 거부, 출처 기반 중복 제거가 모두 실제 실행에서 성립했다. 문제는 **저장 결과의 크기 설계**다. `metrics_timeseries` 결과가 정규화 모델 전체와 같은 구간 표 두 벌을 함께 담아, 문서가 말한 100,000 point 규모가 결과 단계에서는 도달할 수 없고 시리즈를 일부만 선택해도 모델 부분이 줄지 않는다(R13-01). 나머지는 성능·표시·오류 문구에 관한 P3이다. 앞선 T-635 리뷰의 R12-02~R12-04 수정은 실제로 동작했다.

- **T-638 `DONE` 판정:** R13-01(P2)을 처리한 뒤 `DONE` 복귀를 허용한다. R13-02~R13-07은 후속 큐에 넣으면 된다. 실제 WebView UI와 Windows 네이티브 검증은 별도 수용 항목이다.
- **후속 작업과의 관계:** `metrics_timeseries` payload는 T-639·T-640·T-642·T-643·T-644가 입력으로 쓴다. R13-01은 저장 payload 모양을 바꿀 수 있으므로 그 소비자들이 payload 0.1.0 위에 쌓이기 전에 정리하는 편이 좋다. 기존 0.1.0 저장 reader는 보존해야 한다.

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| 누적 counter 구간: 첫 관측·리셋·겹침·감소 | 충족 | 합성 OTLP cumulative sum(시작 1 s): 10 s `first_cumulative_point`(이전 값을 0으로 만들지 않음), 20 s `derived`(delta 50, rate 5), 30 s 값이 150→40으로 감소하나 시작이 같음 → `counter_reset_unannounced`(delta·rate 없음), 시작 25 s가 이전 종료 30 s보다 앞 → `lifetime_overlap`, 시작 41 s가 이전 종료 40 s 이후 → `reset`(delta = 새 수명 값 5, rate `5/9`, `gap` true). |
| delta 구간: 겹침·간격 | 충족 | 구간 `[25,30)`·`[28,40)`은 둘 다 `overlap`이고 delta·rate 없음, `[10,20)` 뒤 `[25,…)`는 `gap` true. 시작 시각이 없는 delta 첫 점은 `interval_unavailable`. |
| 정확한 산술 | 충족 | 실제 OpenTelemetry Python SDK 1.38.0 표본에서 rate가 `125000/27`, `125000/9`, `875000/27`처럼 정확한 분수. 값·delta·bucket count는 문자열 정수. |
| explicit histogram | 충족 | cumulative counts `[1,2,3]→[2,2,5]`가 delta `[1,0,2]`, count 3, sum 11. bounds가 `[1,5]→[1,6]`로 바뀌면 `histogram_schema_changed`로 계산 거부, 새 수명은 `reset`으로 전체 값 사용. |
| exponential histogram | 충족 | bucket을 offset 포함 인덱스로 맞춰 차분한다. `offset 0 [0,1,1]`→`offset 1 [2,2]`처럼 0 bucket이 잘린 경우에도 delta `[0,1,1]`·zero count·count 합이 일치. scale이 바뀌면 `histogram_schema_changed`, 인덱스가 사라지며 count가 줄면 `counter_decreased`. |
| 시간창·질의 | 충족 | 종료 시각 `[from,to)`로 선택하고 구간은 자르지 않음(`[20,40)`이 10→20 구간을 가진 20 s 점 포함). `from==to`는 `METRIC_WINDOW_INVALID`, 중복 series·limit 0/201은 거부, 다른 limit의 cursor는 `METRIC_CURSOR_STALE`. |
| 변조 거부 | 충족 | 표의 delta 변경, 모델 point 값 변경, series `monotonic` 변경, summary 수치 변경 4종이 각각 `METRIC_RESULT_MODEL_MISMATCH`/`METRIC_POINT_INVALID`/`METRIC_SERIES_INVALID`/`METRIC_RESULT_MODEL_MISMATCH`로 거부. |
| 출처·중복 제거 | 충족 | gzip 원본의 SHA-256은 압축 바이트 기준. 같은 `--export-id` 두 모델 merge는 5건 중복 제거(참조 2개씩 보존), 다른 export ID는 중복 제거 없이 10건 유지. 300 MB 0 채움 gzip은 `METRIC_BUDGET_EXCEEDED: decoded source exceeds 64 MiB`로 거부(최대 RSS 189 MB). |
| 입력 범위 경계 | 충족 | Prometheus range는 query 출처 없으면 거부, native histogram 거부, Summary 거부, OTLP oneof 충돌·중복 속성 거부. Q04/GenAI alias는 구현하지 않았고 선언 identity 경고가 모델에 남는다. |
| generic consumer | 충족 | HTML 내보내기에 구간 표가 들어가고 `<img onerror>`·`<b>` 같은 이름/속성이 `&#39;`·엔티티로 이스케이프된다. |
| 증거 무결성 | 충족 | `source_sha256` 80개, `evidence_sha256` 23개가 커밋 내용과 모두 일치. |
| T-635 R12 수정 확인 | 충족 | R12-02: 실제 Go heap의 `top`이 `sort:"self"`에서 `StartCPUProfile`(1,179,824)·`main.main`·`compress/flate.NewWriter` 순, 생략/`inclusive`는 `runtime.main` 순으로 이전 동작 보존. R12-03: `password=`·`secret:`·`api_key=`·`token=`·JWT·Bearer가 `<REDACTED…>`로 마스킹되고 검색 불가, 색인 파일에서 원문 문자열 검색 0건, 일반 단서(`unique-marker-xyz2`)는 계속 검색됨. R12-04: `CASE_BUDGET_EXCEEDED: result is 76839180 bytes; case result limit is 67108864 bytes (64 MiB) … Select fewer profiles/metrics …`로 실제 크기·상한·조치를 안내. R12-01은 아래 5절 참고. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R13-01 | P2 | 결과가 정규화 모델 전체와 구간 표 두 벌을 담아 실질 용량이 약 3만 4천 point이고, 일부 series만 선택해도 모델 부분이 줄지 않음 | `analyzers/metrichistory/analyzer.go:46,51,72` |
| R13-02 | P3 | 질의 쪽마다 결과 읽기·디코드·검증·구간 재계산(9만 point에서 쪽당 약 2.7 s) | `analysis/metric_history.go:19-55`, `metricmodel/query.go` |
| R13-03 | P3 | Prometheus `step_ns`가 식별자에만 쓰이고 누락 구간 표시를 만들지 않음 | `metricmodel/interval.go:195,245`, `parsers/metrichistory/json.go:151` |
| R13-04 | P3 | 숫자 한도·형식 오류가 파일 전체를 중단시키고 좌표 없이 날 오류 문구를 노출 | `parsers/metrichistory/source.go:254,281`, `json.go` |
| R13-05 | P3 | 동일 payload의 다른 export 중복이 모두 `conflicting_timestamp`로 표시되어 원인을 구분하지 못함 | `metricmodel/interval.go:346-362` |
| R13-06 | P3 | CSV 내보내기에 구간 행이 없고 요약 5개와 가용성만 담김(문서는 CSV를 소비자로 나열) | `analyzers/metrichistory/analyzer.go:47`, `exporters/csv` |
| R13-07 | P3 | series 선택 화면이 전체 목록을 한 번에 그리고 검색·전체 선택·쪽 나눔이 없음 | `MetricSourceAssistance.tsx:45` |

## 4 Finding 상세

### R13-01 결과 크기 설계와 실질 용량 (P2)

**근거**: `Build`는 선택된 구간 행을 `tables.points`와 `tables.timed_points`에 각각 넣고(`analyzer.go:46`), 정규화 모델 **전체**를 `metadata.metrics_core.model`에 넣는다(`analyzer.go:51`, `Core{Version, Model, Selection}`). 결과가 64 MiB를 넘으면 `METRIC_RESULT_BUDGET: result exceeds 64 MiB`로 끝난다(`analyzer.go:72`). 문서는 모델 32 MiB·100,000 point를 한도로 말하고 "큰 선택은 series 수를 줄여야 할 수 있다"고만 적는다.

**재현**(counter 시리즈를 4개 또는 10개 가진 합성 OTLP, 모두 `history-import`):

| 입력 | 결과(compact) | 결과 |
|---|---|---|
| 20,000 point / 4 series, 전체 선택 | 39.4 MB | 성공. point당 약 1.97 KB(모델 약 350 B + 구간 행 약 780 B × 표 2벌) |
| 24,000 | 47.3 MB | 성공 |
| 28,000 | 55.2 MB | 성공 |
| 40,000 | — | `METRIC_RESULT_BUDGET: result exceeds 64 MiB` (크기·한도·조치 없음) |
| 90,000 point / 10 series (원본 8.4 MB), series **1개** 선택 | 45.8 MB(모델 31.2 MB + 구간 표 2 × 7.0 MB) | 성공 |
| 같은 원본, series **3개** 선택 | — | `METRIC_RESULT_BUDGET` |
| 같은 원본, 전체 선택 | — | `METRIC_RESULT_BUDGET` |

- 모델 부분은 선택과 무관하게 31.2 MB가 그대로 들어가므로 선택을 줄여도 한계가 약 1/3 지점에서 오며, 문서의 100,000 point는 결과 단계에서 도달할 수 없다. 전체 선택이 가능한 실질 상한은 컴팩트 기준 약 34,000 point다.
- 같은 원본에서 시리즈별로 결과를 여러 개 만들면(`Add selection as new result`, `MaterializeMetricSelection`) 결과마다 31 MB 모델이 다시 저장된다. case에 N개를 저장하면 모델이 N번 중복된다.
- CLI `history-import`는 결과를 들여쓰기 출력해 위 90,000/1-series 결과가 68.9 MB가 된다. 이 파일은 같은 CLI의 `history-query`가 `METRIC_DOCUMENT_BUDGET`(64 MiB 문서 한도)으로 읽지 못하고, `case save-result`도 `CASE_BUDGET_EXCEEDED`가 된다. 컴팩트 45.8 MB는 읽힌다.
- 오류 문구는 실제 크기·한도·point 수·조치를 담지 않는다. R12-04에서 profile 쪽에 넣은 안내 방식이 metric 결과에는 적용되지 않았다.

**영향**: 시간당 수십 series 규모의 일상적인 export(예: 10 series × 하루 1분 간격 약 14,400 × 6 = 86,400 point)가 시리즈 두세 개만 고르면 실패하고, 실패 원인이 보이지 않는다. 이 payload를 T-639(GPU/serving)와 T-640/T-642/T-644가 이어받는다.

**수정 제안**: (1) 결과에 저장하는 모델을 선택된 series와 그 point·출처 참조로 한정하고, 원본 전체가 필요한 경우는 digest로 참조한다(전체 모델은 원본 재분석으로 복구 가능). (2) `tables.points`와 `tables.timed_points`의 중복을 없애고 timeline은 같은 행을 가리키거나 필요한 열만 갖게 한다. (3) 한도 오류에 실제 바이트·한도·선택 point/series 수·"선택 series를 줄이세요"를 담고 문서에 실질 용량을 적는다. (4) CLI 출력이 CLI 입력 한도를 넘지 않게 컴팩트로 쓰거나 한도를 맞춘다. 저장 포맷이 바뀌면 payload 버전을 올리고 0.1.0 reader를 유지한다.

### R13-02 질의 쪽마다 전체 재처리 (P3)

**근거**: Wails 질의 경로 `withMetric`은 쪽마다 `s.Read`(파일 읽기+SHA-256+`resultcontract.DecodeWithLimits`), `a.ReadCore`(`Core` 재디코드와 `Model.Validate`)를 실행하고(`metric_history.go:19-55`, `service.go:687-717`), `QuerySet`이 다시 `ValidateSelection`→`SelectRows`→`Intervals`→`Validate`로 전체 모델의 point ID를 재해시한다. 25행만 반환해도 모델 전체를 처리한다.

**재현**(위 90,000 point/1-series 결과, 컴팩트 45.8 MB, Mac arm64): throwaway 측정에서 결과 디코드 1.81 s, `ReadCore` 0.64 s, `QuerySet` 한 쪽 0.28~0.30 s(쪽 0·1·2 모두 같음) → 쪽당 약 2.7 s. 상태 없는 CLI `history-query`는 쪽당 4.4~4.6 s. 9,000행을 25행씩 넘기면 360쪽이다. T-635 R12-01·T-631 R8-02와 같은 부류다.

**수정 제안**: `(handle, result digest)`를 키로 검증된 모델과 구간 계산 결과를 수명이 짧은 캐시에 두고, 쪽마다 다시 계산하지 않게 한다. 바이트 해시와 검증 의미는 첫 쪽에서 한 번 확정하고 cursor가 결과 digest·질의 해시에 묶여 있다는 현재 성질은 유지한다.

### R13-03 range `step_ns`가 누락 표시를 만들지 않음 (P3)

**근거**: Prometheus range는 `ExpectedStep`을 series에 담지만(`json.go:151`) 모든 range series가 gauge라 `derive`가 `observed_gauge`로 먼저 반환한다(`interval.go:195`). `ExpectedStep`을 쓰는 곳은 cumulative 분기(`interval.go:245`)뿐이고, OTLP·snapshot 입력은 `ExpectedStep`을 채우지 않는다. 따라서 `gap_average` 경로는 원본 importer에서 도달하지 않는다. 작업 기록은 "gaps"를 명시적으로 다룬다고 말하고, 문서는 "명시적 cadence보다 긴 구간"을 설명한다.

**재현**: `{"query":"up","step_ns":"1000000000"}`로 range matrix를 만들고 값이 1 s, 2 s, 5 s, 6 s에만 있게 했다. 모든 점이 `observed_gauge`, `gap` false이고 3~4 s의 누락은 어디에도 나타나지 않는다.

**수정 제안**: gauge 점에서도 이전 점과의 종료 시각 차이가 `step`보다 크면 `gap` 표시(값 보간 없음)를 하거나, range에서는 누락 구간이 표시되지 않는다고 문서에 명시한다. 어느 쪽이든 `step_ns`가 식별자에만 쓰인다는 사실을 소비자가 알 수 있어야 한다.

### R13-04 숫자·형식 오류의 좌표 부재와 날 오류 노출 (P3)

**근거/재현**(모두 `metrics history-import`):
- 유한 double이지만 지수가 ±128을 벗어나는 값(`1e-200`, `1.7976931348623157e308`)은 `METRIC_NUMBER_EXPONENT_BUDGET`으로 **파일 전체**를 중단한다(`source.go:254`). 1,000개 점 중 777번째만 `1e-200`이어도 가져오기가 실패하고, 어느 점인지 표시가 없다. 한도 자체는 문서에 명시되어 있다.
- `asInt` 범위 초과는 Go 원문 `strconv.ParseInt: parsing "9223372036854775808": value out of range`가 그대로 나온다(`source.go:281`). snapshot의 `time_unix_nano`가 숫자이면 `json: cannot unmarshal number into Go struct field .snapshots.time_unix_nano of type string`이 나온다. snake_case 키(`resource_metrics`) OTLP는 `METRIC_FORMAT_REQUIRED`만 나오고 lowerCamelCase가 필요하다는 설명이 없다.

**수정 제안**: 오류에 형식별 좌표(`resourceMetrics/0/…/dataPoints/777`)와 규칙을 담고 안정 코드로 감싼다. 범위 밖 유한 double은 파일 전체 중단 대신 해당 점의 명시적 상태(예: `value_out_of_exact_range`)로 보존할지 검토한다.

### R13-05 동일 payload 중복이 `conflicting_timestamp`로 표시됨 (P3)

**근거/재현**: 같은 원본을 `--export-id E1`, `--export-id E2`로 각각 정규화해 merge하면 점 10건이 모두 남고(설계상 다른 export는 합치지 않음) 시리즈 전체 10건의 상태가 `conflicting_timestamp`가 되어 delta·rate가 사라진다(`interval.go:346-362`). 값이 모두 동일한데도 "충돌"이라는 이름이라 사용자가 서로 다른 값이 충돌한다고 오해하거나 원인을 찾기 어렵다. 자동 병합을 하지 않는 결정 자체는 문서와 일치한다.

**수정 제안**: 같은 시각·같은 payload에 출처만 다른 경우는 별도 상태(예: `duplicate_across_exports`)로 구분해 어떤 export끼리인지 참조를 보이고, 값이 다른 경우만 `conflicting_timestamp`로 둔다.

### R13-06 CSV에 구간 행이 없음 (P3)

**근거/재현**: `Build`는 `result.Series = map[string]any{}`로 두고(`analyzer.go:47`) 구간 행은 `tables`에만 둔다. generic CSV 내보내기는 summary와 series만 쓰므로, 구간 1건짜리 결과의 CSV가 요약 5개 값과 `measurement_availability` 5행(총 544 B)뿐이고 구간 표는 없다. HTML은 같은 결과의 표를 포함한다. 문서는 "generic 상세/timeline, CSV/HTML/report-pack을 사용합니다"라고 쓴다.

**수정 제안**: CSV에 구간 행을 넣거나(예: series 영역에 필요한 열만) 문서에 CSV는 요약만 담는다고 명시한다.

### R13-07 series 선택 화면 규모 (P3, 코드 정독 + 페이로드 측정)

**근거**: `MetricSourceAssistance`는 inventory의 모든 series를 체크박스와 `identity_json` 접이식으로 한 번에 렌더링하고(`MetricSourceAssistance.tsx:45`) 검색·전체 선택·쪽 나눔이 없다. 8,000 series(분당 5점) 합성 원본의 inspect 페이로드는 3.4 MB이고 10,000 series는 모델 한도 때문에 일부 형태에서만 열린다. 작성 측 증거는 25행 쪽 나눔과 명시적 선택을 renderer로 보았다고 하나 수천 series 목록은 다루지 않았다. 실제 WebView에서는 렌더링하지 않았다.

**수정 제안**: 이름·속성 필터와 현재 필터 결과 전체 선택·해제, 쪽 나눔 또는 가상 목록을 추가한다. 선택 개수와 R13-01의 용량 예상을 함께 보여주면 좋다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `2454987`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 104개 패키지 모두 `ok`(테스트 파일 없는 6개 제외)
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON 해시 103개 대조
- 실제 CLI 시나리오: 누적/delta/range/histogram 구간 5종 입력, SDK 1.38.0 표본, 시간창 4종·cursor·인자 거부, 변조 4종, gzip 해시·폭탄, merge(동일/다른 export), 규모 측정(20k·24k·28k·40k·60k/6,000 series·90k/10 series·10k series), 질의 쪽 시간 측정, JSON·CSV·HTML 내보내기와 이스케이프, T-635 R12-02·R12-03·R12-04 수정 확인
- throwaway 스크립트·생성 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 WebView UI(`MetricHistoryExplorer`, `MetricSourceAssistance`; 작성 측도 화면 잠금으로 네이티브 관찰을 하지 못했다고 기록), Windows 네이티브 실행
- 실제 OpenTelemetry SDK 재생성(이 환경에 패키지가 없고 내려받기는 하지 않았다. 번들된 SDK 1.38.0 표본의 값만 검증), 실제 Collector/Prometheus export
- R12-01(비교 캐시)은 독립 시간 측정을 하지 않았다. 캐시는 Wails 서비스 프로세스 안에서만 동작하고 CLI는 상태가 없어, CLI 기준 compare 쪽 시간은 앞선 3.4 s 그대로다. 단위 테스트 통과만 확인했다.
- 오프라인 syscall 관측은 재실행하지 않았다(증거 정독). hosted CI.
- Wails `QueryMetricHistory`/`MaterializeMetricSelection`을 앱 안에서 호출하지 않았다(분석 서비스 단위의 비용은 측정).

## 6 권고 처리 순서

1. R13-01 — 결과 payload에서 모델 전체·중복 표 제거와 실질 용량·오류 문구. T-639/T-640 소비자 전.
2. R13-02 — 질의 쪽 사이 검증 결과 캐시(R13-01과 함께 하면 효율적).
3. R13-04 — 오류 좌표·안정 코드, 범위 밖 double 처리 결정.
4. R13-03 — range 누락 구간 표시 또는 문서화.
5. R13-05/R13-06/R13-07 — 상태 구분, CSV 행, series 선택 화면.
