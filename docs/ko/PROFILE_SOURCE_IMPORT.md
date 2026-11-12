# 정확한 원본 프로파일 가져오기

T-634는 원본 전용 경로를 추가합니다. `profile import`, legacy facade, 저장된
결과, 내부 ProfileSet 0.2.0 결과의 의미는 유지합니다. 원본을 재분석해야
`profile_evidence` payload 0.4.0을 얻으며 저장된 근거의 버전만 바꾸면 안 됩니다.
공통 AnalysisResult envelope와 ProfileSet schema 1.0.0은 유지합니다.
예약 extension `org.archscope.profile_source.v1`은 normalizer 1.1.0, 형식 버전,
원래 metric/profile 선언과 원본 메타데이터를 고정합니다.

## 화면과 CLI

Analysis Workspace에서 원본 파일을 선택하고 **원본 py-spy / Speedscope / pprof
프로파일**을 펼칩니다. 확인 후 metric과 프로파일을 명시적으로 선택하고 새
정규화 모델을 저장합니다. 화면이 `profile.source-set-json` (별칭
`profile-source-set`)과 선택 옵션을 적용합니다. 분석을 누르면 공통 job, 화면,
case 저장과 내보내기 경로를 사용합니다. 빈 프로파일도 목록에 남으며 CPU metric,
첫 worker나 첫 값 column을 자동으로 선택하지 않습니다.

```sh
archscope-engine profile inspect-source --in input.json --format speedscope-json
archscope-engine profile normalize-source --in input.json --format speedscope-json --out new-model.json
archscope-engine evidence import --in new-model.json --importer profile.source-set-json --options '{"profile_ids":["profile-0"],"metric_id":"sampled-seconds"}'
archscope-engine profile import-source --in input.json --format speedscope-json --profile-id profile-0 --metric-id sampled-seconds --out result.json
archscope-engine profile inspect-source --in input.raw --format pyspy-raw --producer py-spy --producer-version 0.4.1
archscope-engine profile inspect-source --in heap.pb.gz --format pprof --producer go.runtime/pprof --producer-version go1.26.10
```

`--capture-options`는 민감 정보를 제거한 전체 JSON object입니다. 인증 정보,
환경 변수와 명령 값은 먼저 제거해야 합니다. 일반적인 민감 key를 거부하지만
포괄적인 비밀 탐지기는 아니므로 임의 필드에도 운영 비밀을 넣으면 안 됩니다.
알 수 없는 안전한 key는 보존하고 엄격하게 비교합니다. `name@version` exporter가
있으면 명시적 선언과 일치해야 합니다. raw/pprof는 형식을 명시해야 하며 auto는
Speedscope JSON 경로만 인식합니다. 기존 `profile.set-json`은 새 source extension을,
새 transport는 기존 모델을 거부해 결과 reader를 분리합니다.

확인은 제한된 inventory만 반환하며 모델을 JavaScript로 전달하지 않습니다.
정규화는 원본 SHA-256을 재확인하고 새 파일을 원자적으로 저장합니다. 원본 변경과
기존 출력은 거부합니다. 원본·옵션 변경은 오래된 UI 요청을 취소합니다. 취소나
종료 시 부분 모델을 성공 결과로 발행하지 않습니다.

## 측정 의미와 한도

| 형식 | 보존하는 해석 | 한도와 명시적 거부 |
|---|---|---|
| py-spy raw / collapsed | 모든 행, 0을 포함한 정확한 소수·지수 가중치, aggregate sample count, root-first stack | 수집 시간·CPU 시간·worker 경계를 추론하지 않습니다. raw의 process/thread 표시는 stack frame으로 유지합니다. dump text 형식과 다릅니다. |
| Speedscope sampled | 모든 profile, frame의 파일·행·열, 원래 단위, 0을 포함한 정확한 가중치 | samples와 weights가 모두 있고 길이가 같아야 합니다. 관측별 시각이나 CPU 의미를 만들지 않습니다. |
| Speedscope evented | 균형 잡힌 시간순 O/C frame, 현재 stack의 정확한 exclusive 구간, 선언된 start/end | 잘못된 순서·범위·frame 참조·frame 누락·불균형·비시간 단위를 거부합니다. stack이 없는 경과 구간은 `[unattributed]`로 기록해 전체 합계가 이벤트 수가 아닌 선언된 시간폭이 됩니다. |
| pprof | 모든 값 column과 원래 type/unit, 0·signed difference를 포함한 정확한 int64, inline/root 순서, label·숫자 단위, period/type/duration | 이미 가중된 값에 period를 다시 곱하지 않습니다. 알려진 process/thread/worker/rank label tuple은 별도 opaque profile이며 label이 없으면 aggregate입니다. 알 수 없는 단위는 거부합니다. |

pprof location/mapping/function 메타데이터도 주소·inline 근거와 함께 extension에
보존합니다. 공통 frame 집계는 function/file/line 기준입니다. 지원 단위는
count/samples/objects, bytes, seconds/milliseconds/microseconds/nanoseconds,
cycles입니다. 음수 column은 명시적 signed-difference metric이 됩니다.
Speedscope `none`/bytes는 count/bytes이며 시간 가중치를 자동으로 CPU 시간으로
해석하지 않습니다.

원본 및 압축 해제 입력 각각 64 MiB, 직렬화 모델 32 MiB, 관측 100,000개
(pprof 값마다 별도 관측), frame 100,000개, 깊이 1,024, profile 1,024개,
서로 다른 stack의 frame 참조 100만 개 한도입니다. collapsed 한 행은 1 MiB로
제한하며 wire string/token/depth 한도도 적용합니다. 파일 정규화는 동시 2개,
context 제한 5분입니다. 초과 시 전체 작업을 거부하며 잘린 성공 결과를 만들지
않습니다. 공통 transport의 chunk/record/result/temp 한도도 별도로 적용합니다.

기존 정확한 tree/diff/drilldown/export가 선택한 모델을 소비합니다. pprof
내보내기는 tick scale과 집계한 차원을 공개하는 선택 aggregate projection이며
원본 바이트 재구성이 아닙니다. 소수는 정확한 tick scale이 필요하며 반올림이나
overflow가 필요한 출력은 거부합니다. `ticks` 단위 pprof 출력은 차원 해석에
projection 메타데이터가 필요하므로 원본 reader로 다시 가져오지 않습니다.
손실 없는 모델 재사용에는 저장한 AnalysisResult를 사용하세요.

## 검증된 수집 옵션 정책 (R5-02)

컴파일된 binding만 정책을 결정합니다. snapshot에는 정책 ID·revision 1.0.0·hash,
producer/version/format/normalizer/metric, 민감 정보를 제거한 전체 원래 옵션,
semantic/incidental projection과 strict unknown key를 담습니다. 입력의
whitelist/ignore/policy 선언에는 권한이 없습니다. 누락·null·0은 구분하며
canonical 비교 참조에 양쪽 원래 옵션과 incidental 변경을 공개합니다. entity,
clock, metric, selection, 의미 버전, quality category gate는 계속 적용합니다.
null이 아닌 선언된 capture PID는 profile process 조건에도 연결하므로 PID 변경은
명시적 entity mapping이 필요합니다. 이번 작업이 mapping을 만들어 주지 않습니다.

| 검증한 binding | Semantic key | Incidental key |
|---|---|---|
| py-spy 0.4.1 raw count / sampled Speedscope seconds | rate, gil, native, idle, subprocesses, duration, sampling_mode | pid, output_path |
| Go runtime/pprof go1.26.10 heap alloc_objects/count, alloc_space/bytes | sample_rate, period, period_type, profile_type, source_period, source_period_type, duration, source_duration | pid, output_path |
| 같은 Go heap inuse_objects/count, inuse_space/bytes | sample_rate, period, period_type, profile_type, source_period, source_period_type | pid, output_path, duration, source_duration |

Speedscope의 원본 profile bounds는 파생된 strict 조건이며 예약 source 필드를
사용자가 덮어쓰면 거부합니다. 다른 버전·producer·metric(Go CPU와 evented
Speedscope 포함)은 구조가 지원되면 해석하지만 완화 정책을 받지 않습니다.
분류하지 않은 옵션은 모두 strict입니다. 전역 PID/duration 무시 규칙은 없습니다.
원본 정체성은 선언 또는 파일 exporter 근거이며 원본 인증으로 표시하지 않습니다.

새 canonical 비교 identity는 `profile-conditions-v4`이며 기존 v2 hash는 유지합니다.
저장된 0.3.0 메타데이터는 내장 모델·정책을 재계산해 검사합니다. 일반 scalar
비교는 전체 capture의 엄격한 동등성을 유지하고 그 범위를 공개합니다. 검증된
incidental 차이에는 canonical ProfileSet diff를 사용하세요. 영속 baseline binding과 선언된 entity mapping은
[케이스 근거 조회](CASE_EVIDENCE_QUERIES.md)에 구현했으며 기존 일반 scalar 경로는 strict를 유지합니다.

## 검증 근거

Digest로 고정한 재사용 corpus `diagnostics.profile-producers.v1`은 실제 py-spy
0.4.1 합성 subprocess 수집(Speedscope/raw), 실제 Go go1.26.10 합성 heap 수집과
경로·시각 정제 범위를 포함합니다. 파서 testdata에 바이트 동일 사본을 보관해
테스트와 런타임은 형제 checkout에 의존하지 않습니다. 해당 fixture 범위를
검증하며 모든 도구 버전·플랫폼·옵션을 전면 보증하지 않습니다. 별도 합성 회귀는
소수·0, 다중 프로파일, evented gap/중첩/거부, pprof 다중 column·단위·음수·큰 정수·label,
정책 안전성, 저장·Wails 왕복을 검증합니다. Windows 네이티브 UI 실행과 독립 리뷰는
별도입니다.

형식 근거: [Speedscope 명세](https://github.com/jlfwong/speedscope/blob/main/src/lib/file-format-spec.ts),
[py-spy 0.4.1 exporter](https://github.com/benfred/py-spy/blob/v0.4.1/src/speedscope.rs),
[pprof proto](https://github.com/google/pprof/blob/main/proto/profile.proto).

## R11 의미 버전 경계 (2026-11-12)

Normalizer 1.1.0 / payload 0.4.0은 모든 원본 metric, inventory, 화면, 결과 summary에
원래 type/unit을 드러냅니다. Column ID는 파일 안에서 유지하되 비교 identity가
정책 유무와 무관하게 원래 type/unit·형식 버전을 포함합니다. Producer를 선언하지
않거나 검증되지 않은 버전이어도 alloc_space와 inuse_space를 비교하지 못합니다.
기존 0.3.0 스냅샷은 원래 v2/v3 hash와 읽기·저장·조회·내보내기를 유지합니다.
비교하려면 원본을 명시적으로 재분석해 새 결과를 만들어야 하며 버전만 바꾸지 않습니다.

Speedscope none은 일반 count 가중치(kind=count)로 표시하고 source_unit=none을
보존하며 total_samples를 주장하지 않습니다. 새 정책은 basis=declared/file_exporter를
노출하고 비교는 capture_policy_uses_declared_producer 경고를 남깁니다. 재분석은
바이트·옵션·모델 검증이며 producer 인증이 아닙니다. 100k 관측 한도는 관측/frame/depth의
실제 수와 한도를 표시합니다. 100 Hz에서 100k sampled 관측은 단일 worker 약
1,000초, 같은 빈도의 4개 worker 약 250초이며 pprof는 column마다 관측을 소비합니다.
관측 원본 참조를 잃는 자동 병합은 하지 않습니다. 검증된 baseline 재사용은
[케이스 근거 조회](CASE_EVIDENCE_QUERIES.md)를 참고하세요.
