# 케이스 근거 조회와 재사용 baseline

T-635는 조회 계약 1.0.0을 추가합니다. 공통 AnalysisResult envelope, 기존 profile
payload 의미, case schema와 저장 바이트는 유지합니다. 선택 변경은 명시적인 새
결과로 만들며 baseline은 새 결과를 자동으로 따라가지 않습니다.

## 화면 사용 순서

1. T-634 원본 패널에서 프로파일을 가져와 Workspace에 추가합니다.
2. **공통 프로파일 drilldown과 근거**에서 profile/metric과 순서 있는 frame-ID
   결합 필터를 선택합니다. Top, tree/flamegraph, callers, callees, 관측을
   조회합니다. 근거 경로를 눌러 focus하고 **선택을 새 결과로 추가**하면
   백엔드의 불변 결과가 Workspace에 추가됩니다.
3. **케이스 검색과 재사용 baseline**에서 케이스를 생성·선택한 뒤 현재 리비전을
   조회합니다. 원본을 읽기 전용 외부 참조로 추가하고 원본을 선택해 현재 정밀
   결과와 연결 저장합니다. 기존 snapshot-only 저장은 보존하며 재분석 검증을
   주장하지 못합니다.
4. 케이스를 검색하고 범위 및 digest에 연결된 원본 좌표를 확인합니다. 저장 결과와
   원본을 골라 **원본 재분석 후 새 baseline 고정**을 실행합니다. 고정 baseline과
   저장된 대상·원본을 선택해 비교합니다. 외부 변경이나 이미 commit됐을 수 있는
   변경 요청 취소 후에는 케이스를 다시 조회하세요. 기존 결과·원본은 덮어쓰지 않습니다.

## 공통 프로파일 조회

`profile-query --in result.json --query JSON`과 QueryProfileEvidence는 기존
ProfileSet tree/diff/drilldown/export와 같은 계산을 사용합니다. 결과 digest,
선택, 보기, frame/문자열/시간창 필터, 한도와 cursor를 고정하며 결과·질의 변경은
cursor를 무효화합니다. 응답은 population/selection, 정확한 합계, metric/unit/scale,
availability/quality와 형식별 원본 참조를 제공합니다. 분모는 선택된 전체 관측이며
문자열·frame·보기 필터는 표시 행만 줄입니다. Caller/callee 및 재귀 frame 집계는
관측마다 서로 다른 frame/이웃을 한 번씩 계산합니다. Tree는 경로별 inclusive/self
가중치를 유지합니다.

시간창은 evidence 시작 timestamp에만 적용하며 정확한 선언 clock scale과
반개구간 `[from,to)`를 사용합니다. interval을 자르거나 선택 분모를 바꾸지 않습니다.
Clock kind/unit/scale이 섞이면 거부합니다. 시각 없는 관측 수를 공개하고 sampled
시각을 만들지 않습니다. Offset·상관·시계 정렬을 추론하지 않습니다.

페이지 단위 flamegraph는 전체 선택 tree로 위치를 계산하므로 숨긴 노드의 공간을
유지합니다. 좌표만 제한된 표시용 근삿값이며 가중치·query/export 값은 정확한 소수
문자열입니다. 합계 0이나 signed metric에 양수 면적을 만들지 않습니다. 응답은 최대
200행·1 MiB입니다. 표시 label은 UTF-8 512-byte prefix, 근거 stack 문자열은 약
4 KiB이며 label 검색도 해당 prefix 범위입니다. Canonical ID·전체 모델·합계는
유지합니다. 기존 모델·tree·참조 한도도 적용합니다.

```sh
archscope-engine profile-query --in result.json --query '{"selection":{"profile_ids":["profile-0"],"metric_id":"sampled-seconds"},"view":"evidence","limit":25}'
archscope-engine case-query search --store /path/to/cases --request search.json
archscope-engine case-query pin-baseline --store /path/to/cases --request pin.json
archscope-engine case-query compare-baseline --store /path/to/cases --request compare.json
```

## 검색 색인과 원본 링크

검색은 `case_id`, 현재 `revision`, 문자열 `text`, `limit`(1–200), 선택적
`source_id`·`result_id`·`field`·cursor를 받습니다. 원본 `text`, 결과
`/summary/<key>` 및 `profile.observation`에서 ID/digest와 마스킹 문자열을 찾습니다.
Typed profile 관측은 공통 evidence visitor의 frame/profile ID, 원본 참조와
scale을 적용한 timestamp를 재사용합니다. 검색 색인/조회 1.1.0은 time_from,
time_to, clock, time_unit을 선택적으로 받으며 시간창에는 result_id 1개를 명시해
서로 다른 수집의 상대 시계를 정렬하지 않습니다. 시각 없는 행 수를 공개하고
시각을 만들지 않습니다. 미파싱 로그에서 임의 필드·시각을 추정하지 않습니다.

Case store의 `indexes`에 저장하는 파생 색인은 indexer·마스킹 정책·manifest
리비전·원본 가용성/digest·결과 digest를 고정합니다. Cache hit에서도 원본 바이트와
결과 object 무결성을 검사합니다. 누락·손상 색인은 재생성하며 정리·재시작이 저장
결과나 pin을 지우지 않습니다. 파생 본문은 별도 index-mask 0.2.0을 적용한 뒤
최대 2 KiB snippet으로 저장합니다. 이 정책은 알려진 패턴만 처리하므로 다른 민감
값은 입력 전에 제거해야 합니다. 구조화한 ID/digest/좌표는 마스킹 본문과 분리하며
새 producer 파서를 추가하는 기능이 아닙니다.

케이스 색인은 최대 100,000행·16 MiB이며 원본 색인은 기존 100만 행 prefix를
재사용합니다. 원본·결과별 indexed/total과 누락·decoder·prefix·budget 이유를
공개하고 snippet 잘림도 표시합니다. 색인 밖 텍스트를 검색 가능하다고 주장하지
않습니다. 전역 한도로 아직 읽지 않은 원본은 `index_budget`, 알 수 없는 total(0)을
표시하며 빈 전체 자료로 취급하지 않습니다. 여기서 원본 좌표를 열면 최대 10행과
원래 byte/decoded offset을 반환합니다. Prefix 밖 링크는 거부합니다. pprof/Speedscope의
형식별 구조 좌표를 임의 텍스트 행으로 바꾸지 않습니다. Case/result/source/query/index
변경은 검색 cursor를 무효화합니다.

## Baseline 검증과 매핑

고정 요청은 `{case_id,revision,result_id,source_id}`입니다. 원본이 저장 ResultRef에
연결돼 있고 hash가 내장 모델의 원본 hash와 같아야 합니다. 현재 재생 가능한 경로는
원본 profile payload 0.4.0입니다. 기존 결과는 읽을 수 있으며 검증된 새 pin에는
명시적 원본 재분석이 필요합니다. 백엔드가 결과를 검증하고 제한된 원본 바이트 및
복구한 수집 옵션으로 고정 normalizer를 다시 실행해 전체 정확한 모델을 대조합니다.
Source bounds/period는 원본에서 재계산합니다. 저장 selection/options-hash 선언이
있으면 복구한 값과 일치해야 합니다. 옵션 digest는 canonical SourceOptions JSON이며
기존 job의 selection hash와 다릅니다.

기존 Baseline.conditions JSON에 `profile-baseline` 1.0.0 binding을 저장합니다.
불변 result/source/model/options hash, selection, comparison identity, capture/clock
hash와 semantic/incidental 원래 옵션을 포함한 컴파일 정책을 담으며 64 KiB 한도입니다.
가져오거나 편집한 “verified” 선언에는 권한이 없습니다. 재사용할 때마다 기준·대상
원본을 다시 분석하고 전체 pin을 대조하며 원본·리비전을 재확인한 뒤에만
`source_verification=verified_by_reimport`를 반환합니다. 이는 해당 바이트·옵션과 모델의
일치 검증이며 배포자·수집 진위·프로세스 생존 기간의 인증이 아닙니다. 비교의
`identity_basis`는 `declared`로 유지합니다.

비교 요청은 `{case_id,revision,baseline_id,target_result_id,target_source_id,
mapping?,context_mapping?,limit?,cursor?}`입니다. 기본 50행·최대 200행·1 MiB로
페이지를 나누며 정확한 delta/share/percentage 의미를 유지합니다. T-634의
producer/format/version/metric 정책이 incidental 차이를 처리하고 양쪽 원래 옵션을
공개합니다. 기존 일반 scalar 비교는 strict이며 새 canonical/baseline 화면이 검증된
정책을 소비합니다.

Entity 별칭은 선택한 현재 tuple 전체의 1:1 매핑을 명시해야 합니다:
`{baseline:{id,process,thread,worker},target:{id,process,thread,worker},reason}`.
비교용 사본만 변경하며 응답 참조는 원래 모델·옵션을 가리킵니다. 오래된 tuple,
충돌, 부분 매핑은 거부합니다. Capture 맥락이 다르면 양쪽 hash에 연결한
`{baseline_sha256,target_sha256,reason}`도 필요합니다. 화면 매핑 입력은 entity 배열
또는 `{entities:[],capture:{...}}`를 받습니다. 전체 capture-clock hash는 계속 같아야
하며 명시적 clock 정렬은 구현하지 않습니다. 매핑으로 metric/selection/quality/semantic
capture 조건이나 unknown/no-policy strict 동등성을 우회할 수 없습니다.

원본 누락·변경, stale source/encoding/result, 모델·옵션 선언 변경, 정책·normalizer·binding
변경은 검증을 거부합니다. 과거 pin이나 hash를 고치지 않습니다. Portable restore는
pin을 보존하되 검증 권한을 부여하지 않으며 현재 원본이 다시 일치해야 합니다.
Windows 네이티브/UI 수용 및 독립 리뷰는 로컬 backend/state/build/cross-build 근거와
구분합니다.

## R12 조회 보완

Top query는 `sort: "self" | "inclusive"`를 받고 query hash/cursor에 고정합니다. 화면의
Top 기본값은 self이며 API에서 생략하면 기존 inclusive 정렬입니다. 검색은 선택 분모를
변경하지 않습니다.

Index-mask 0.2.0은 password/secret/token/api-key 대입, JWT, AWS access-key와
반복 적용 가능한 Authorization 마스킹을 추가합니다. 정책 버전과 내용 digest가 기존
파생 색인을 무효화합니다. 기존 저장 결과/수집의 text-mask 0.1.0은 유지합니다. 알려진
패턴만 처리하므로 공유 전에 입력과 본문을 확인해야 합니다. 원본/저장 결과를 수정하지 않습니다.

Baseline은 새 query에서 두 원본을 재분석합니다. 현재 서비스 프로세스에만 존재하는 캐시
(2항목·합계 128 MiB·2분 만료)가 검증된 전체 차이를 재사용합니다. 페이지마다 원본/결과
바이트 hash와 전체 case/query/policy binding을 다시 검사합니다. Case receipt나 disk에서
검증 권한을 읽지 않고 종료 때 캐시를 해제합니다. 만료/miss는 재분석하며 새 CLI 명령은
별도 프로세스라 매번 재분석합니다. 응답은 cache_hit·verification_scope를 공개하며
identity_basis는 declared입니다. 정규화/전체 diff를 반복하지 않지만 제한된 diff 복사와
바이트 해시는 페이지마다 남습니다.

Case 결과 한도는 67,108,864바이트(64 MiB)입니다. 고유 경로가 많은 프로파일은 원본보다
훨씬 큰 결과를 만들 수 있어 저장/pin이 불가합니다. 오류는 실제 크기·한도·알려진 sample
수를 표시하며 bounded decode 이전이면 sample 수 미상을 명시합니다. CLI는 읽기 전에
파일 크기를 검사합니다. Profile/metric 선택이나 수집 시간을 줄여 새 결과를 만드세요.
관측을 몰래 합치거나 한도를 높이지 않습니다. 네이티브 WebView/Windows와 독립 검토는 별도입니다.
