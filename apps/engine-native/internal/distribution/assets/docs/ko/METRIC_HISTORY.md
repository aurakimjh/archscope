# 정확한 메트릭 이력

T-638은 `metrics_timeseries` payload 0.2.0(기존 저장 0.1.0 reader 유지)과 MetricHistory 모델/normalizer 1.0.0을
추가합니다. 기존 `metrics_snapshot` 가져오기와 저장 reader의 의미는 유지합니다.
공통 AnalysisResult envelope와 parser/analyzer/exporter 책임 분리도 유지합니다.

## 입력과 검증 범위

- 중립 snapshot sequence는 type, unit, monotonic, temporality, 속성과 시작 시각을
  명시합니다. 이름으로 counter 의미를 추정하지 않습니다.
- Prometheus range **matrix** JSON에는 `{"query":"...","step_ns":"1000000000"}`
  같은 query 출처가 필요합니다. 결과는 물리 단위를 알 수 없는 query-result gauge로
  취급하고 query/step도 식별자에 넣습니다. Native histogram matrix는 거부합니다.
- OTLP lowerCamelCase JSON의 gauge/sum, explicit/exponential histogram을 지원합니다.
  정수 enum, 정확한 정수 문자열/숫자, resource/scope 속성·버전과 두 schema URL을
  보존합니다. 알 수 없는 message 필드는 무시하고 oneof 충돌·중복 속성·잘못된
  bucket·summary는 거부합니다. Binary OTLP는 범위 밖입니다. Histogram sum/min/max의
  비유한 값은 명시적으로 거부하며 scalar NaN/±Inf·no-recorded-value는 0으로 바꾸지 않습니다.

합성 표본은 실제 OpenTelemetry Python SDK 1.38.0에서 counter, up/down counter,
explicit/exponential histogram을 두 번 수집하고 공식 OTLP encoder/protobuf JSON으로
직렬화했습니다. 큰 정수와 resource/scope를 포함하는 이 표본만 검증합니다. 사용자의
실제 export 경로·instrumentation 버전·GenAI alias 승인은 여전히 Q04에 남습니다.

참고: [OTel metric model](https://opentelemetry.io/docs/specs/otel/metrics/data-model/),
[OTLP JSON](https://opentelemetry.io/docs/specs/otlp/#json-protobuf-encoding),
[Prometheus range API](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).

## 구간 계산

값·bucket count·나노초 시각·delta는 문자열, rate는 정확한 초당 분수로 유지합니다.
Cumulative는 같은 start-time lifetime 안에서만 차분합니다. 첫 관측의 이전 값을 0으로
만들지 않습니다. 이전 종료 이후의 명시적 새 lifetime은 reset 구간으로 계산할 수 있습니다.
설명 없는 monotonic 감소·lifetime overlap·충돌 시각·시작 시각 누락·모호한 이전 관측에는
rate를 만들지 않습니다. Non-monotonic sum은 음수 변화를 보존하고 delta는 자체 구간에서
입력 값을 한 번만 사용합니다. 겹친 delta 구간은 모두 계산 불가로 표시합니다.

종료 시각으로 정렬해 계산하면서 원래 좌표와 순서 변경 여부를 유지합니다. 명시적 cadence보다
긴 sum/histogram 구간은 전체 구간 평균이며 보간하지 않습니다. Delta 사이의 누락 구간은 gap으로 표시합니다.
Range-query gauge는 현재 `step_ns`를 식별자에만 반영하며 빠진 cadence slot은 표시하지 않습니다(R13-03 후속).
Cadence가 없는 cumulative에 임의 주기를 만들지 않습니다. OTLP의 0/누락 시각은 알 수 없음이며,
중립 snapshot/range의 Unix epoch 0은 보존합니다. 시각 없는 관측은 상세 표에 남기고 timeline에서
제외합니다.

Explicit bounds/counts와 exponential scale·양/음 bucket offset/count·zero 영역을 구분합니다.
Cumulative histogram 차분은 동일 schema와 bucket별 비감소를 요구합니다. Scale/zero threshold
변경은 재배치하지 않고 계산 불가로 표시합니다. Min/max는 차분하지 않습니다. 원본 histogram과
구간 bucket을 따로 제공합니다. Percentile 평균이나 quantile 추정은 수행하지 않습니다.

시간창은 종료 시각 `[from_ns,to_ns)`를 선택하며 구간 전체를 유지합니다. 시작이 창 밖이어도
잘라내거나 값을 재배분하지 않습니다. Unit, resource/scope, 속성, type/temporality, origin과 query가
다르면 별도 시계열입니다. Client/server와 OTLP/Prometheus 복사본을 자동 합산하지 않습니다.
Exemplar ID는 명시적 근거로만 보존하고 trace 연관을 추정하지 않습니다.

## 출처와 한도

원본 SHA-256은 gzip이면 압축 바이트를 대상으로 합니다. 형식별 좌표와 모든 원본 참조를
유지합니다. Merge는 동일 원본 hash/좌표/payload 또는 format·producer/version·좌표와 묶인
명시적 export ID 및 동일 payload일 때만 중복 제거합니다. 충돌 payload는 남기고 시각이 같다는
이유만으로 다른 export를 제거하지 않습니다. Reimport로 승인된 baseline을 주장하지 않습니다.

원본·gzip 해제는 64 MiB이고 중립 snapshot JSON 검증에는 32 MiB JSON 한도도 적용됩니다.
모델은 32 MiB, source 128, series 10,000, point 100,000, side별 bucket 4,096,
전체 bucket 500,000입니다. Explicit는 bounds 4,096/counts 4,097까지입니다. Merge의 펼친
출처 참조도 100,000으로 제한합니다. 결과와 case object는 64 MiB를 유지합니다. 원본 import는
동시 2건·5분 제한이며 공통 job의 별도 하향 예산도 적용됩니다. Transport는 256 KiB 이하
chunk입니다. 전용 query는 1–200행·1 MiB, generic provider는 100행·256 KiB입니다.
요청·옵션 한도 때문에 큰 선택은 series 수를 줄여야 할 수 있습니다. 한도를 맞추려고 관측을
몰래 삭제하지 않습니다.

## CLI와 화면

```
archscope-engine metrics history-inspect --in export.json --producer opentelemetry-python --producer-version 1.38.0
archscope-engine metrics history-normalize --in export.json --out model.json
archscope-engine evidence import --in model.json --importer metrics.history-json --options '{"series_ids":["metric-..."]}' --out result.json
archscope-engine metrics history-query --in result.json --query '{"selection":{"series_ids":["metric-..."]},"limit":25}'
archscope-engine metrics history-merge --inputs model-a.json,model-b.json --out merged.json
```

`metrics history-import`는 원본에서 결과를 직접 만들며 `--selection`이 없으면 보존한 전체 series를
선택합니다. 원본 range의 `--query`에는 query 출처를 전달합니다. Normalize/merge는 새 파일만
만들며 기존 목적지·관리 저장소 경로를 거부합니다. 네트워크 수신기나 원격 fetch는 없습니다.

Workspace에서 **메트릭 이력**을 열어 원본 확인 → series 명시 선택 → 새 모델 저장 → 분석을
진행합니다. Importer는 `metrics.history-json`(alias `metric-history`)입니다. **정확한 메트릭
구간과 출처**에서 식별자·값/rate·원본/구간 bucket·상태·좌표·시간창과 페이지를 조회합니다.
선택을 새 결과로 추가하면 새 소유 결과를 만들고 늦게 취소된 결과 lease를 해제합니다. Cursor는
결과 digest와 선택/시간창/페이지 조건을 고정합니다. Case 저장과 JSON export는 바이트를
보존하며 generic 상세/timeline, CSV/HTML/report-pack을 사용합니다. 메트릭 비교/SLI 정책,
자동 시계 정렬, 네이티브 UI/Windows 수용 검증은 별도입니다.

정확한 decimal literal은 1,024자, 지수 ±128, 소수점 아래 128자리 한도이며
범위를 벗어나면 명시적으로 거부합니다.

화면의 series 식별자는 opaque `identity_json` 문자열로 전달해 큰 숫자 속성이
JavaScript 표시에서 반올림되지 않게 하며 선택은 backend ID를 사용합니다.

## R13 저장 보완과 남은 한계

0.2.0은 명시적으로 선택한 series와 그 series의 모든 point만 저장합니다. 시간창 밖의
이전 관측·겹침 문맥은 계산에 필요하므로 유지합니다. 저장 결과의 series 목록도 이 부분집합으로
제한되며 제외한 series를 선택하려면 원본/모델을 다시 열어야 합니다. 상세 구간·bucket을 표에
중복 저장하는 대신 point/status와 time/clock/reference만 투영하고 전용 query가 고정 모델에서
상세 값을 계산합니다. 0.1.0 저장 reader와 원본 JSON 바이트는 그대로 유지합니다.

CLI normalize/merge와 `history-import`는 compact JSON을 써서 자체 입력 한도 안에서 다시
읽도록 합니다. 결과 한도 초과는 실제 바이트·point/series 수와 series 축소/원본 분할 안내를
제공합니다. 시간창만 좁혀도 이전 관측 문맥 때문에 모델 크기가 줄지 않을 수 있습니다.
합성 timed point 100,000개/10 series는 모델 27,670,574바이트, 결과 58,573,608바이트입니다.
40,000개/4 series 결과는 23,424,243바이트이며 1 series만 고르면 5,857,892바이트입니다.
Bucket/속성 크기에 따라 한도가 먼저 찰 수 있어 모든 10만 point 입력의 수용을 보장하지 않습니다.

Generic CSV는 현재 summary/availability만 내보내며 **구간 행은 포함하지 않습니다**.
정확한 구간/bucket은 `history-query` JSON을 사용하세요(R13-06 후속).
기존 본문의 CSV/HTML/report-pack 지원은 공통 exporter 사용을 뜻하며 전용 구간 CSV를 뜻하지 않습니다.

독립 CONDITIONAL 리뷰는 P2 저장 문제 해소 후 나머지 P3를 후속 큐로 두고 종결하도록 허용했습니다.
R13-02는 매 페이지 retained model 재해석·검증·계산 비용(메트릭 전용 warm cache 없음),
R13-04는 좌표가 부족한 수치/형식 오류와 exact 범위 밖 값의 파일 전체 거부,
R13-05는 다른 export의 동일 값도 별도 보존하며 `conflicting_timestamp`가 될 수 있는 상태,
R13-07은 검색/페이지 없는 bounded 전체 series 목록입니다. 위 R13-03/06과 함께 후속 작업이며
완료 기능으로 주장하지 않습니다. 실제 사용자 export와 native WebView/Windows 검증은 별도입니다.

R14-07 정정: 0.2.0 generic 상세/HTML/Report Pack 표는 point ID·status와 time/clock/reference
투영만 담으며 메트릭 이름·단위·값·delta·rate 행은 담지 않습니다. 정확한 값/bucket은 전용
history query/탐색기로 제공합니다. 더 풍부한 generic export는 R13-06과 함께 후속 처리합니다.
