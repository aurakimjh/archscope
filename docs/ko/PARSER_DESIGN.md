# 파서 설계

## 로컬 진단 확장 절차 (2026-11-11, T-628 기반 구현)

[통합 설계 §16](./LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md#16-analysisresult와-확장-계약)의
descriptor/catalog 기반 경로를 도입할 때 아래 순서를 따른다. 이후 본문의
기존 `Analyze*`와 family별 CLI 설명은 현행 경로이며 새 구조의 완료 증거가 아니다.

1. producer/버전/측정 의미·한계를 정의하고 선언형 → 외부 변환 → compiled importer 순서로 검토한다.
2. 공유 자산에 비식별 fixture·기대 의미·manifest를 등록하고 `ASSET_INDEX.md`를 갱신한다.
3. descriptor에 detector 예산, schema, record family, capability, 변환 의존성, adapter/fixture 버전을 선언한다.
4. parser는 typed records·원본 위치·diagnostics, adapter는 단위·clock·identity, analyzer는 `AnalysisResult`를 만든다.
5. composition root에 등록하고 ResultDescriptor의 generic/timeline/SLI/compare/detail provider를 검증한다. 기존 `Analyze*`는 wrapper로 유지한다.
6. 감지 충돌·손실·미지 버전·취소·partial·metadata 왕복과 golden을 검증하고 지원표를 갱신한다.
7. 기존 의미 계열과 renderer를 재사용하는 입력은 package + fixture + 등록으로 끝나야 한다. service/CLI/UI switch가 늘면 이유와 제거 계획을 남긴다. 새 의미·시각화는 별도 계약 변경으로 명시한다.

### T-628 상세 설계 범위와 현재 기준

상세 설계는 `83b8ff9`의 코드 대조를 거쳐 `eb3e782`에 고정했다. 이후 T-628의
C1–C4 공통 기반과 한 importer를 구현했다. 아래는 논리 계약이며 실제 API·검증
범위·제한은 문서 끝의 구현 기록을 따른다. 전체 ProfileSet·renderer·수치 정정은 후속 작업이다.

| 현재 근거 | 확인한 동작 | 이행할 경계 |
|---|---|---|
| `internal/ingestion/registry.go` | 최대 64 KiB probe, confidence 최고값 하나 선택, 같은 점수는 등록 정렬 순서의 첫 항목 유지 | 기존 `Detect`는 호환 경로로 두고 새 다중 후보 선택 API 추가 |
| `internal/ingestion/ingestion_test.go` | confidence·미지 형식·HAR/NetLog 구분 테스트 | 새로운 충돌·예산·강제 형식 계약은 별도 적합성 사례로 추가 |
| `internal/threaddump/registry.go` | plugin 기반, 기본 mixed-format 거부와 명시적 override 경로 | wrapper가 현재 옵션 의미를 보존하고 지원 범위를 넓혔다고 선언하지 않음 |
| `internal/analyzers/profile/analyzer.go` | `profile_evidence` 0.1.0 출력과 기존 profiler 연산 사용 | T-628은 실행 접점, T-629는 ProfileSet 코어, T-634는 의미 정정 책임 |

### 등록과 형식 선택 계약

`internal/catalog`가 시작 시 built-in descriptor를 명시적으로 조립하고 검증 후
registry를 동결한다. parser package의 `init()` 부작용으로 순서를 정하지 않는다.
descriptor는 실행 함수와 외부 표시용 설명을 분리한다. 목록·UI에는 함수 핸들,
실행 파일 경로, 임의 코드가 노출되지 않는다.

| 계약 | 구현 시 고정할 규칙 |
|---|---|
| ID와 alias | canonical ID는 소문자 ASCII 영숫자·`.`·`_`·`-`의 비어 있지 않은 값. 명시된 legacy alias만 같은 정규화 규칙으로 lookup; 중복 canonical/alias와 reserved `auto` 등록은 실패 |
| 버전 | adapter version과 producer의 검증 범위를 분리. 미지 producer version을 최신 버전과 동일하게 간주하지 않음 |
| 출력 family | descriptor의 record family, normalizer 입력, analyzer 입력을 조립 시 대조. 다른 family의 batch를 받으면 결과 생성 전 실패 |
| schema 참조 | 앱에 포함된 ID/version/hash로 resolve. 옵션·record·결과 schema의 외부 URL을 자동 가져오지 않음 |
| 능력 | descriptor는 가능한 능력의 상한. 파일별 시간·심볼·비교 능력은 parse/normalize 후 근거로 축소 |
| 등록 증거 | fixture manifest ID/hash와 conformance version이 없는 importer는 release 지원 목록에 포함하지 않음 |

새 `DetectCandidates`는 모든 유효 후보와 근거를 반환한다. 구조·signature 확인은
`verified`, 확장자/파일명만 맞으면 `hint`로 구분한다. confidence는 화면 정렬용이며
확률이나 자동 선택의 단독 근거가 아니다. 비유한 값과 범위 밖 점수는 detector 오류다.
한 canonical importer만 verified이면 자동 선택하고, 둘 이상이면 `needs_selection`,
hint만 있으면 확인 필요, 후보가 없으면 unsupported로 반환한다. alias가 같은 canonical
ID를 가리키는 경우 먼저 합친다. 정렬은 근거 등급 → confidence → stable ID로 고정한다.

사용자가 importer를 지정해도 schema·지원 버전·구조 검증은 생략하지 않는다.
다른 형식의 확실한 signature와 충돌하면 `IMPORT_FORMAT_MISMATCH`로 종료한다.
실패 뒤 다른 parser로 조용히 재시도하지 않는다. 기존 CLI의 format alias와
thread override는 wrapper의 명시적 compatibility option으로 다루고 fixture로 고정한다.

### 실행 인터페이스와 소유권

다음은 Go 내부·Wails bridge가 공유할 논리 계약이다. HTTP endpoint나 외부 plugin ABI를
추가하는 설계가 아니다. 세부 DTO 이름은 바꿀 수 있지만 입력·소유권·결과 의미는 유지한다.

```text
Catalog.List() -> bounded DescriptorView[]
Catalog.DetectCandidates(ctx, Probe) -> DetectionSet
Catalog.Resolve(importerID, options, policyRevision) -> ResolvedImporter
ImportEvidence(ctx, sourceRef, importerSelection, options) -> ImportJobRef
SelectImporter(jobID, generation, importerID) -> JobSnapshot
Parser.Parse(ctx, SourceLease, RecordSink) -> ParseReport
Normalizer.Normalize(ctx, RecordBatch, NormalizedSink) -> error
Analyzer.Analyze(ctx, NormalizedRef, AnalysisOptions) -> AnalysisResult
GetImportJob(jobID, generation) -> JobSnapshot
CancelImport(jobID, generation) -> CancelOutcome
```

- `SourceLease`는 선택된 source ID, 표시용 이름, 크기·fingerprint와 bounded read/reopen 기능을 제공한다. service가 열고 닫으며 parser는 별도 파일·네트워크·converter를 열지 않는다. seek가 필요한 parser는 descriptor에서 선언하고 bounded spool 또는 미지원 응답을 받는다.
- `RecordBatch`는 family별 typed record, 원본 byte/line/event 위치, sequence를 가진다. parser는 batch를 sink에 넘긴 뒤 재사용·변경하지 않는다. sink는 동기 backpressure를 제공하며 UI가 소비할 때까지 무한 queue에 쌓지 않는다.
- source를 연 뒤 probe와 실제 parse의 대상이 바뀌지 않게 한다. 같은 file handle만으로 불변성을 보장하지 않는다. 수명·fingerprint·읽은 내용과 완료 hash로 일관성을 신뢰성 있게 검증할 수 없으면 먼저 앱 소유 immutable snapshot을 만든다. snapshot 예산 부족이나 변경 감지는 실패이며 `SOURCE_CHANGED` 등을 반환한다. hash가 같다는 사실만으로 수집 provenance를 합치지 않는다.
- options는 schema 검증 후 immutable snapshot으로 고정한다. key 정렬과 버전 있는 canonicalization으로 option hash를 만들고, secret/원본 경로를 job progress나 hash 설명에 노출하지 않는다.
- 최대 입력/해제 bytes, record 크기·수, stack 깊이, unique key 수, 시간·임시 디스크 예산을 job 시작 때 기록한다. 기본 probe는 현행 64 KiB를 유지하며 압축 입력의 해제 probe에도 별도 상한을 적용한다. 상위 policy보다 큰 importer/user 예산은 거부한다.
- converter 실행은 parser 밖의 별도 승인된 local-conversion 단계다. collection kit와 설치된 executable의 허용 옵션을 사용하고, stdout/stderr·시간·임시 파일을 제한한다.

### Job 상태와 실패 계약

```text
queued -> probing -> needs_selection -> validating -> parsing -> normalizing
       -> analyzing -> validating_result -> committing -> completed | partial
```

이 흐름은 논리 단계이며 스트리밍 parse/normalize가 겹칠 수 있다. 자동 선택이면
`needs_selection`을 건너뛰고, selection 전에는 원본 전체 파싱을 시작하지 않는다.
`SelectImporter`는 해당 대기 job/generation에만 적용한다. 같은 선택의 재전송은 멱등이고
파싱 시작 후 다른 선택으로 바꾸려면 기존 job을 취소하고 새 job을 만든다.
진행 중 단계에서 `failed` 또는 `cancelled`로 끝날 수 있다. `completed`·`partial`·
`failed`·`cancelled`는 terminal이다. manifest publish의 atomic 시점이 완료의 기준이며,
그 뒤의 취소는 `already_terminal`을 반환한다. 같은 job ID/generation의 취소는 멱등이다.

결과는 staging에서 schema·측정 의미·참조를 검증한 뒤 확정한다. 실패·취소는 staging과
owned lease를 정리하고 이전 결과 revision을 유지한다. 부분 결과는 **분석 가능한 완결
record가 있고 importer가 partial 의미를 정의했으며 호출자가 허용한 경우에만** 저장한다.
source 변경, schema/family 오류, 필수 측정값의 단위 불명, 핵심 구조 손상은 partial로 성공 처리하지
않는다. 예산 도달도 importer가 정의한 안전한 record 경계에서만 partial이 가능하다.
누락 event의 종료 시각·stack weight를 임의 보충하지 않는다.

| 제안 코드 | 반환 위치와 의미 | 소비자 동작 |
|---|---|---|
| `IMPORT_REGISTRY_INVALID` | 중복 ID/alias·미존재 schema·family 불일치 | 새 catalog 활성화 실패; 검증되지 않은 일부 등록으로 계속 진행하지 않음 |
| `IMPORT_SELECTION_REQUIRED` | verified 충돌 또는 hint만 있음 | 후보와 근거 표시, 선택 전 재파싱 금지 |
| `IMPORT_FORMAT_UNSUPPORTED` / `IMPORT_VERSION_UNSUPPORTED` | 형식 또는 producer 지원 범위 불충족 | 변환/제한 안내; 성공 결과 생성 금지 |
| `IMPORT_FORMAT_MISMATCH` / `IMPORT_OPTIONS_INVALID` | 명시 선택 또는 옵션 검증 실패 | 해당 입력 정정, 자동 다른 parser fallback 금지 |
| `IMPORT_BUDGET_EXCEEDED` / `SOURCE_CHANGED` | 자원 또는 동일 source 보장 실패 | 원인·실제 제한·부분 자료 가용성 표시 |
| `IMPORT_RESULT_INVALID` | 정규화/결과 schema·단위·참조 위반 | commit 거부, 기존 snapshot 보존 |

취소는 오류 finding을 만드는 대신 job의 별도 terminal 상태다. `JobProblem`은 code,
stage, retryability, source ID, sanitized message와 bounded diagnostic references를 가진다.
실패를 빈 정상 `AnalysisResult`로 바꾸지 않는다. partial의 coverage는 result metadata에도
남겨 CLI 저장·재열기·export에서 job history 없이 확인할 수 있게 한다.

### T-628 구현 순서와 검증 인계

| 단계 | 변경 위치 | 완료 증거 |
|---|---|---|
| C1 | ingestion descriptor/catalog와 compatibility wrapper | 기존 confidence/alias/thread fixture 유지, 새 충돌·등록 실패 검증 |
| C2 | service orchestration과 bounded sink | 선택/취소/부분 성공/동시 source 변경/cleanup의 결정적 회귀 |
| C3 | models·profiler metadata 및 공통 decoder | [데이터 모델 상세 계약](./DATA_MODEL.md#t-628-측정과-호환-상세-계약)의 정밀도·unknown-field·버전 사례 통과 |
| C4 | 기존 importer 하나의 전체 경로 연결 | CLI/Wails load-save-export에서 contract 유지, 새 형식별 switch 증가 여부 기록 |

T-628은 최소 하나의 기존 importer를 새 등록 경로로 연결해 C1–C4를 검증하는 범위다.
모든 형식의 코어 이행은 T-629, 공통 renderer는 T-631, 수치 정정은 T-634에 남긴다.
기존 프로파일 goldens, 모델 roundtrip, thread mixed-format 테스트를 기준선으로 보존한다.
새 적합성에는 duplicate/alias 충돌, hint-only, 두 verified 후보, 잘못된 detector 점수,
cancel-before/after-commit, unsafe partial, schema mismatch, stale generation,
큰 정수/소수·unknown block 왕복을 포함한다. fixture 실행을 생략한 통과는 release 근거가 아니다.
재사용 fixture와 실제 JSON Schema는 구현 단계에 공유 자산으로 등록하고 인덱스를 갱신한다.
이 절은 ArchScope 전용 실행 계약이며 새로운 범용 schema 파일을 생성하지 않는다.

Parser는 raw file을 typed record로 변환한다. Chart rendering이나 report formatting 책임은 갖지 않는다.

## 책임

- encoding fallback을 사용해 파일 읽기
- line-oriented 또는 block-oriented evidence parsing
- traceability를 위해 raw input fragment 보존
- typed record 또는 parser diagnostic 반환
- 향후 대용량 파일 처리를 위한 streaming pattern 유지

## 공통 Ingestion 기반

Mid-Term Plus importer 작업은 parser별 record를 만들기 전에
`apps/engine-native/internal/ingestion` 아래의 공통 Go package를 사용한다.
경계는 다음과 같다.

- Parser package는 `internal/parsers/<family>` 아래에 두고 file decode, format
  분류, typed record와 diagnostics 생성만 담당한다.
- Analyzer package는 `internal/analyzers/<family>` 아래에 두고
  `AnalysisResult` envelope를 만든다.
- CLI group은 `server-log analyze`, `database-log analyze`, `profile import`
  같은 kebab-case family name을 사용한다.
- Wails binding은 `AnalyzeServerLog`, `AnalyzeDatabaseLog`처럼
  `Analyze<Family>` 형태를 유지한다.
- Result type은 `server_log`, `database_slow_query`, `broker_log`,
  `kubernetes_evidence`, `profile_evidence`처럼 snake_case를 사용한다.

같은 package는 다음 contract도 제공한다.

- Auto-detect dispatch를 위한 reusable source-format registry와 bounded probe
  contract.
- Valid, partial, malformed, unknown-format, large-file importer sample을 위한
  golden fixture diagnostic harness.
- Source kind, format, product/version, host, service, environment, sanitized
  file identity를 담는 normalized source metadata.
- Trace/span/request ID, tenant/customer ID, container/pod/host/PID identity,
  timestamp window를 담는 cross-source correlation-key model.

## Evidence Stitching

`stitch analyze`는 기존 `AnalysisResult` JSON 파일을 대상으로 동작하는
second-pass analyzer다. 원본 log를 다시 parsing하지 않고 table에서 bounded
evidence node를 추출한 뒤 trace/span/request/tenant/container/host/PID key를
기록하고 match group을 만든다. 또한 missing trace ID, dropped parent span,
unmatched request log, unmatched database call, unmatched broker event를
correlation-gap finding으로 낸다. 고도화된 stitching은 timestamp-window
matching, workload/service/database/broker naming variant를 위한 service alias
normalization, profile label 기반 trace-profile linkage, raw source 확인용
`match_drilldowns`를 추가한다.

## API 및 Event Contract Parser

`api-contract analyze`는 OpenAPI/AsyncAPI JSON과 보수적인 YAML subset을
받는다. YAML subset은 `paths`, HTTP method object, `operationId`, `summary`,
`tags`, `channels`, `publish`, `subscribe`, message name을 다룬다. Analyzer는
OpenAPI operation을 access-log result table과, AsyncAPI channel을 broker result
table과 비교한 뒤 undocumented, unused, slow, high-error API/event surface를
deterministic finding으로 낸다.

## Architecture Documentation Package

`architecture-docs draft`는 기존 `AnalysisResult` JSON 파일을 대상으로 하는
second-pass analyzer다. arc42 Context, Runtime View, Deployment View, Quality
Requirements, Risks, ADR draft에 들어갈 architecture-review row를 추출한다.
원본 evidence parsing 책임은 각 importer에 남기고, 이 package는 이미 정규화된
evidence와 finding을 문서 입력으로 변환한다.

## Access Log Parser

Access-log parsing은 기존 NGINX/common/combined path의 호환성을 유지하면서
명시적 selector와 `auto` detection 뒤에 source-specific access 및 edge format을
추가한다. 지원 selector는 NGINX, Apache/common/combined, OHS, WebLogic,
Tomcat, Jetty, HAProxy HTTP, Envoy/Istio text 및 JSON, AWS ELB/ALB/CloudFront,
GCP HTTP(S) Load Balancer JSON, Azure App Service/Front Door JSON, IIS W3C,
Caddy JSON, Traefik JSON, Kong JSON, Tyk JSON, AWS API Gateway JSON이다.

기존 combined-like path는 response time field를 포함한 NGINX row를 대상으로
하며, response time이 없는 common/combined access-log row도 tolerant하게
처리한다.

```text
127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] "GET /api/orders/1001 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123
```

Parser는 공통 access-log field를 추출한다.

- timestamp
- method
- uri
- status
- bytes sent
- referer
- user agent
- response time in milliseconds
- raw line

Common/combined row에 response time field가 없으면 source format 자체에 latency
정보가 없으므로 request/status evidence는 보존하고 `response_time_ms=0.0`으로
기록한다. Apache, OHS, WebLogic, Tomcat, Jetty format selector는 row shape이
맞는 경우 이 combined/common parsing path를 공유한다.

Source-specific parser는 source format, host, protocol/scheme, upstream service
또는 cluster, route, backend, gateway/upstream latency, TLS field, response
flag, termination state, retry count, trace ID, request ID, consumer, cloud
provider, edge location 같은 additive edge metadata도 채울 수 있다. 이 field는
analyzer summary와 downstream projection에 사용하지만 필수 access-log contract를
대체하지 않는다.

## Server Log Parser

Server-log parser는 `auto`, Tomcat, Jetty, JBoss/WildFly, WebLogic, WebSphere,
GlassFish/Payara, nginx error, Apache error selector를 받는다. Parser 책임은
row를 normalized event record로 decode하는 데서 끝난다.

- timestamp, 존재하는 경우
- severity
- product
- component
- thread / host / service, 존재하는 경우
- event type
- message
- trace ID 및 request ID, 존재하는 경우
- raw line

Startup, deployment, datasource pool, stuck/hung thread, thread-pool pressure,
managed-server health, severe error, worker/upstream error taxonomy와 finding은
analyzer가 소유한다.

## Collapsed Profiler Parser

초기 지원 대상은 async-profiler collapsed output이다.

```text
frame1;frame2;frame3 123
```

규칙:

- 마지막 필드는 sample count로 읽는다.
- stack string은 마지막 sample count 앞의 전체 문자열이다.
- `1000`, `1e3`처럼 정수로 환산 가능한 numeric sample count는 허용한다.
  음수, non-finite, fractional sample count는 거부한다.
- 동일 stack은 sample count를 합산한다.
- total samples를 계산한다.
- estimated seconds는 `samples * interval_ms / 1000`으로 계산한다.
- Top N stack은 sample count 기준 내림차순으로 정렬한다.

Collapsed stack은 공통 `FlameNode` tree contract로도 변환한다. 이를 통해 drill-down과 execution breakdown이 flat top-stack row가 아니라 동일한 tree model을 기준으로 동작한다.

## 통합 Profile Evidence Parser

`profile import` 경로는 여러 profiler family를 공통 sample schema로 정규화한 뒤
분석한다. 각 sample은 language, runtime, `native`, `managed`, `async` marker가
붙은 frame stack을 가진다. Analyzer는 이 sample들을 collapsed stack으로
roll up한 뒤 기존 flamegraph, top-stack, execution-breakdown, drill-down 로직을
재사용한다.

지원 import contract:

- `github.com/google/pprof/profile` 기반 pprof binary/gzip profile (`.pb.gz`)
- 기존 profiler parser를 재사용하는 async-profiler collapsed 및 HTML flamegraph
- py-spy, rbspy raw text stack dump
- dotnet-trace speedscope export를 포함한 speedscope JSON
- Rust/native evidence용 perf collapsed stack
- JFR JSON stack sample event
- Ruby StackProf JSON
- PHP Excimer/Tideways 계열 JSON 및 Xdebug cachegrind text
- Swift backtrace와 generic async stack text
- Pyroscope/Phlare 및 Parca 계열 continuous-profiling snapshot

## Jennifer APM Flamegraph CSV Parser

Jennifer APM flamegraph CSV import는 다음 canonical column을 기대한다.

```text
key,parent_key,method_name,ratio,sample_count,color_category
```

실무에서 자주 쓰는 `id`, `parent_id`, `method`, `name`, `samples`, `count`, `category` 같은 alias는 가능한 범위에서 허용한다. Parser는 `key`와 `parent_key`로 tree를 재구성하고 각 row를 공통 `FlameNode` model로 매핑한다.

```text
id
parentId
name
samples
ratio
category
color
children
path
```

Reader는 `utf-8-sig`, `utf-8`, `cp949`, platform preferred encoding 순서로
decode를 시도한다. CSV에 root가 여러 개 있으면 ArchScope는 virtual `All`
root를 만든다. Missing parent는 tolerant mode에서 warning으로 보고하고 root로
취급한다. Duplicate key와 parent cycle은 diagnostics에 보고하고 안전하게 skip
또는 detach한다.

Jennifer `sample_count`는 inclusive node sample로 취급한다. Tree rendering은
inclusive 값을 보존하고, top-stack 및 execution breakdown 계산은 parent/child
sample을 중복 집계하지 않도록 non-overlapping exclusive path를 사용한다.

## JVM Parsers

GC log, thread dump, exception stack trace parser는 access log 및 profiler input과 동일하게 parser/analyzer 책임 분리를 따른다.

- **GC parser** (`gc_log_parser.py`)는 HotSpot unified GC pause line을 지원하며 timestamp, GC type, cause, pause, Eden/Survivor/Old/Metaspace 영역의 heap before/after/committed 값을 추출한다.
- **GC log header parser** (`gc_log_header.py`)는 `OpenJDK ... VM` 배너와 `CommandLine flags:` 라인, 그리고 `[gc,init]` 블록(CPU 수, 메모리, heap min/initial/max/region size, parallel/concurrent worker 수, NUMA, pre-touch)을 읽어들인다. 결과는 GC log 분석기 상단의 **JVM Info** 카드를 채운다.
- **JFR recording parser** (`jfr_recording.py`)는 바이너리 `FLR\0` 매직을 감지하고 JDK `jfr` CLI를 호출(절대 경로 `ARCHSCOPE_JFR_CLI` 명시; PATH/JAVA_HOME 탐색만으로 실행하지 않음)하여 `print --json`을 수행한다. 기존 JSON 경로(`jfr_parser.py`)는 그대로 유지된다. 집계 전에 모드/시간 범위/상태 필터도 받는다.
  JDK `jfr` CLI는 바이너리 `.jfr`를 JSON으로 변환하는 경계까지만 담당한다.
  ArchScope는 JDK `jfr view` 또는 `jfr summary` UI 전체를 복제하지 않고,
  JSON 출력의 `stackTrace.frames`를 기반으로 보고서용 이벤트 요약,
  async-profiler 스타일 스택 집계, recording hint, Evidence Board row를 만든다.
- **Thread dump parser registry** (`thread_dump/registry.py`)는 plugin contract `(format_id, can_parse(head: str) -> bool, parse(path) -> ThreadDumpBundle)`을 노출한다. 레지스트리는 모든 입력의 첫 4 KB를 검사해 매칭 플러그인으로 dispatch하고, UTF-16 / BOM 인코딩 입력을 자동 디코딩한다. 단일 멀티 덤프 요청 안에서 포맷이 섞이면 `format_override`가 없는 한 `MixedFormatError`로 즉시 거부한다.
- **Exception parser**는 optional ISO timestamp, nested `Caused by` root cause, stack frame, stable stack signature가 있는 Java exception stack block을 지원한다.

Record-level malformed input은 skip하고 `metadata.diagnostics` 아래에 보고한다.

## Multi-runtime Thread-dump 플러그인

Thread-dump 레지스트리는 현재 **9개 플러그인 변형**을 제공한다.

| Format ID                       | 런타임           | 자동 감지 시그니처 |
| ------------------------------- | ---------------- | ------------------ |
| `java_jstack`                   | JVM              | `Full thread dump …` 헤더 **또는** quoted-name + `nid=0x…` **또는** quoted-name + state 라인 (JDK 21+ no-`nid`) |
| `java_jcmd_json`                | JVM              | `"threadDump"`를 포함한 JSON 객체 |
| `go_goroutine`                  | Go               | `^goroutine \d+ \[\w` (`runtime.Stack`, panic, debug.Stack) |
| `python_pyspy`                  | Python (py-spy)  | `Process N:` + `Python vX.Y` |
| `python_faulthandler`           | Python (stdlib)  | `Thread 0xHEX (most recent call first):` |
| `python_traceback`              | Python (custom)  | `Thread ID: <n>` + `File "...", line N, in func` |
| `nodejs_diagnostic_report`      | Node.js (12+)    | `"header"` + `"javascriptStack"`을 가진 JSON 객체 |
| `nodejs_sample_trace`           | Node.js          | `Sample #N` + `Error` + `at fn(file:line:col)` |
| `dotnet_clrstack`               | .NET             | `OS Thread Id: 0xHEX` + `Child SP / IP / Call Site` |
| `dotnet_environment_stacktrace` | .NET             | `at Type.Method() in path:line N` |

각 플러그인은 자기 언어 한정 후처리에서 `RUNNABLE` / `UNKNOWN` 상태를
보다 구체적인 `NETWORK_WAIT` / `IO_WAIT` / `LOCK_WAIT` /
`CHANNEL_WAIT`로 승격해 멀티 덤프 상관 분석기가 언어 비의존 finding을
만들 수 있게 한다. 전체 매트릭스는
[`MULTI_LANGUAGE_THREADS.md`](MULTI_LANGUAGE_THREADS.md) 참고.

## Multi-runtime Single-stack Parsers

단일 예외/패닉 artifact용 — 위의 thread-dump 파이프라인과 별개의
**single-stack** 분석기 페이지에서 사용된다.

- Node.js parser는 `Error`, `TypeError`, custom `*Error` block과 `at ...` stack frame을 지원한다.
- Python parser는 표준 `Traceback (most recent call last):` block과 마지막 exception line을 지원한다.
- Go parser는 `panic:` header와 `goroutine N [state]:` block 및 function frame을 지원한다.
- .NET parser는 `*Exception` stack block과 `#Fields:` metadata가 있는 IIS W3C access line을 지원한다.

이 parser들은 작은 diagnostic artifact와 demo scenario용 MVP다. Full structured log ingestion이나 OpenTelemetry correlation을 대체하지 않는다.

## OpenTelemetry JSONL Parser

OpenTelemetry MVP는 line-delimited JSON log record를 입력으로 받는다. `trace_id`/`traceId`, `span_id`/`spanId`, `parent_span_id`/`parentSpanId`, `severity_text`/`severityText`, `service_name`/`service.name`, `body` 같은 일반적인 field alias를 지원한다.

Analyzer는 trace, service, severity 기준으로 record를 그룹화하고, 둘 이상의 service에 나타나는 trace를 lightweight correlation signal로 보고한다. Parent span ID가 있으면 span tree로 service path를 재구성하고, 없으면 timestamp 순서로 fallback한다. Failed trace는 첫 실패 service, 실패 이후 downstream service, failure propagation finding을 보고한다. Full OTel resource/logs envelope ingestion과 span timing correlation은 향후 작업이다.

## Error Handling

Parser error handling은 다음 원칙으로 확정한다.

```text
File-level 또는 configuration-level 실패는 fatal로 처리한다.
Record-level malformed input은 기본적으로 non-fatal로 처리하고 diagnostics에 보고한다.
```

### 기본 Mode

기본 parser mode는 tolerant로 둔다. 운영 로그에서는 일부 line이 깨졌다는 이유로 전체 대용량 파일 분석을 중단하면 실무 가치가 떨어지기 때문이다.

- Blank line은 무시하며 diagnostics에 포함하지 않는다.
- Malformed record는 skip한다.
- Skipped record 수를 집계한다.
- Skipped record sample은 제한된 개수만 `metadata.diagnostics`에 기록한다.
- 0건 이상 파싱 가능한 경우 analyzer output은 유효한 결과로 반환한다.
- `strict=True`를 노출하는 parser는 malformed record에서 fail-fast로 중단할 수
  있다. 기본값은 계속 tolerant/non-strict다.

### Fatal Errors

분석을 신뢰성 있게 진행할 수 없는 조건은 run 실패로 처리한다.

| Condition | Policy | Example error code |
|---|---|---|
| Input file이 없거나 읽을 수 없음 | Fatal | `FILE_NOT_READABLE` |
| 지원하지 않는 parser format | Fatal | `UNSUPPORTED_FORMAT` |
| Encoding fallback chain으로 decode 불가 | Fatal | `ENCODING_ERROR` |
| Required analyzer option이 잘못됨 | Fatal | `INVALID_OPTION` |
| Output path에 쓸 수 없음 | Exporter/CLI layer에서 fatal | `OUTPUT_WRITE_ERROR` |
| 예상하지 못한 internal exception | Fatal | `INTERNAL_ERROR` |

### Non-Fatal Record Errors

Record-level error는 skip하고 diagnostics에 보고한다.

| Parser | Malformed condition | Policy | Reason code |
|---|---|---|---|
| Access Log | 선택한 log format과 line이 맞지 않음 | Skip | `NO_FORMAT_MATCH` |
| Access Log | Timestamp parse 실패 | Skip | `INVALID_TIMESTAMP` |
| Access Log | Numeric field 변환 실패 | Skip | `INVALID_NUMBER` |
| Access Log | JSON access-log payload decode 실패 | Skip | `INVALID_JSON` |
| Access Log | IIS W3C row가 `#Fields:` header보다 먼저 나오거나 필수 column이 없음 | Skip | `MISSING_W3C_FIELDS` |
| Collapsed Profiler | Trailing sample count가 없음 | Skip | `MISSING_SAMPLE_COUNT` |
| Collapsed Profiler | Sample count가 integer가 아님 | Skip | `INVALID_SAMPLE_COUNT` |
| Collapsed Profiler | Sample count가 음수 | Skip | `NEGATIVE_SAMPLE_COUNT` |

### Diagnostics Shape

Parser diagnostics는 `AnalysisResult.metadata.diagnostics` 아래에 기록한다.

초기 shape:

```text
metadata.diagnostics = {
  source_file: string | null,
  format: string | null,
  total_lines: number,
  parsed_records: number,
  skipped_lines: number,
  skipped_by_reason: Record<string, number>,
  warning_count: number,
  error_count: number,
  samples: [
    {
      line_number: number,
      reason: string,
      message: string,
      raw_preview: string
    }
  ],
  warnings: DiagnosticSample[],
  errors: DiagnosticSample[]
}
```

규칙:

- `samples`, `warnings`, `errors`는 제한한다. 현재 한도는 각각 100건이다.
- `raw_preview`는 잘라서 기록한다. 초기값은 200자이다.
- 짧은 preview로 충분한 경우 대용량 log record 전문을 diagnostics에 넣지 않는다.
- Parser 내부에서는 더 풍부한 diagnostics를 보유할 수 있지만, 외부 contract는 `metadata.diagnostics`를 기준으로 한다.

### Portable Parser Debug Logs

Parser diagnostics는 `AnalysisResult` 안에 들어가므로 의도적으로 작게 유지한다. 현장에서 parser 수정을 위한 증거가 필요하면 engine은 ArchScope 실행 위치 하위 `archscope-debug/`에 별도 debug JSON 파일을 쓸 수 있다.

규칙:

- skipped record 또는 parser exception이 수집되면 debug log를 자동 생성하며, `--debug-log`로 강제 생성할 수 있다.
- `--debug-log-dir`로 기본 portable 출력 위치를 바꿀 수 있다.
- raw context는 기본 redaction을 적용한다. token, cookie, query value, email, 긴 식별자, IP/user/host 식별자, absolute path는 마스킹한다.
- Redaction은 parser evidence를 보존해야 한다. delimiter, quote, bracket, timestamp shape, numeric shape, field count, failed pattern name, partial match data, `field_shapes`는 남긴다.
- Debug log는 개발자용 artifact이며, UI/result의 안정 contract는 계속 `metadata.diagnostics`다.

### Access Log Policy

- Empty 또는 whitespace-only line은 무시한다.
- 설정된 format과 맞지 않는 line은 `NO_FORMAT_MATCH`로 skip한다.
- Timestamp, status, bytes, response-time 값이 잘못된 line은 구체적인 reason code로 skip한다.
- Response time field가 없는 common/combined row는 `response_time_ms=0.0`으로
  파싱한다.
- `metadata.diagnostics.parsed_records`는 analyzer aggregation에 포함된 record 수와 같아야 한다.
- `metadata.diagnostics.skipped_lines`에는 blank ignored line을 포함하지 않는다.

### Collapsed Profiler Policy

- Empty 또는 whitespace-only line은 무시한다.
- Stack과 trailing whole-number sample count가 없는 line은 skip한다.
- Negative sample count는 fatal이 아니라 skip으로 처리한다.
- Zero sample count는 parsed record로 계산하지만 flamegraph sample은 만들지
  않는다.
- 유효한 duplicate stack은 기존처럼 sample count를 합산한다.
- `metadata.diagnostics.parsed_records`는 stack merge 전 유효 collapsed line 수를 의미한다.

### Encoding And Corrupt Input

`iter_text_lines`는 `utf-8`, `utf-8-sig`, `cp949`, `latin-1` 순서의 fallback chain을 사용한다. `latin-1`은 모든 byte sequence를 decode할 수 있으므로, 일부 corrupt byte sequence는 decode failure가 아니라 malformed record로 parser layer에 도달할 수 있다. 정책은 다음과 같다.

- 모든 configured encoding으로 decode에 실패하면 fatal이다.
- Decode는 되었지만 의미적으로 잘못된 record는 non-fatal로 skip하고 diagnostics에 보고한다.
- 향후 binary/corruption detector를 추가할 수 있지만, parser diagnostics를 대체하지 않는다.

## 새 Parser 구현 가이드

새 parser를 추가할 때는 다음 패턴을 따른다.

### 기본 구조 (Go 예시)

```go
func ForEachRecord(
    path string,
    format string,
    opts Options,
    fn func(MyRecord) error,
) (*diagnostics.ParserDiagnostics, error) {
    diags := diagnostics.New(format)
    diags.SetSourceFile(path)

    err := textio.ForEachTextLine(path, "", func(lineNumber int, line string) error {
        diags.TotalLines++
        record, parseErr := tryParseLine(line)
        if parseErr != nil {
            diags.AddSkipped(lineNumber, parseErr.Reason, parseErr.Message, line)
            return nil
        }
        diags.ParsedRecords++
        return fn(record)
    })
    return diags, err
}
```

### 핵심 원칙

1. **Callback streaming 패턴 사용**: 전체 record slice를 만들지 않고 한 건씩 analyzer로 전달
2. **ParserDiagnostics 사용**: skip된 레코드는 반드시 reason code와 함께 기록
3. **encoding fallback 위임**: `textio.ForEachTextLine`이 처리하므로 parser에서 직접 인코딩을 관리하지 않음
4. **raw_preview 보존**: 최대 200자까지의 원본 텍스트를 진단용으로 보존

### Sampling 알고리즘 (Reservoir Sampling)

대용량 파일에서 percentile 계산용 샘플을 bounded 크기로 유지하는 방식:

```python
import random

class BoundedReservoir:
    """Deterministic reservoir sampler for bounded percentile estimation."""

    def __init__(self, capacity: int, seed: int = 42):
        assert seed > 0, "seed=0 causes degenerate replacement stream"
        self._capacity = capacity
        self._rng = random.Random(seed)
        self._items: list[float] = []
        self._count = 0

    def add(self, value: float) -> None:
        self._count += 1
        if len(self._items) < self._capacity:
            self._items.append(value)
        else:
            # Vitter's Algorithm R
            j = self._rng.randint(0, self._count - 1)
            if j < self._capacity:
                self._items[j] = value

    def sorted_samples(self) -> list[float]:
        return sorted(self._items)
```

이 sampler는 `max_lines` 제한 없이도 메모리 사용량을 `capacity` 크기로 고정한다.

## Large File Baseline

Phase 1B baseline은 parser와 analyzer 책임을 분리하면서 access-log 분석에서 전체 record materialization을 피한다.

### Sampling Options

Access-log analysis는 다음 optional control을 받는다.

| Option | Policy |
|---|---|
| `max_lines` | 이 수만큼 physical source line을 읽은 뒤 중단한다. positive integer여야 한다. |
| `start_time` | parsed record timestamp가 이 ISO 8601 datetime 이상인 record만 포함한다. |
| `end_time` | parsed record timestamp가 이 ISO 8601 datetime 이하인 record만 포함한다. |

Rules:

- `max_lines`는 configured limit을 넘는 line을 parse하기 전에 적용한다.
- Time filter는 malformed line에 trustworthy timestamp가 없으므로 line parse 성공 후 적용한다.
- `metadata.diagnostics.total_lines`는 읽은 physical line 수를 나타내며, `max_lines`가 있으면 그 값으로 bounded된다.
- `metadata.diagnostics.parsed_records`는 time filtering 이후 analyzer aggregation에 포함된 valid record 수를 나타낸다.
- 적용된 option은 `metadata.analysis_options` 아래에 echo해야 한다.

### Streaming Aggregation

Access-log analyzer는 parser record iterator를 소비하며 aggregation state를 incremental하게 update해야 한다.

- total request count
- error count
- summary percentile 계산용 response-time sample
- per-minute request counter와 response-time sample
- status-family counter
- URL request counter
- average latency ranking용 URL response-time total 및 count
- table output용 bounded sample records

Analyzer의 main analysis path는 전체 `list[AccessLogRecord]`를 만들지 않아야 한다. Phase 1B에서는 exact percentile 계산을 위해 response-time sample array를 유지할 수 있으며, 이를 approximate sketch로 대체하는 일은 이후 large-file optimization으로 둔다.

### Go 엔진 Streaming Baseline

Go/Wails baseline 기준으로 line-oriented parser는
`internal/textio.ForEachTextLine` 기반 callback record iteration을 우선 사용한다.

- Access log, GC log, OTel JSONL, exception stack, simple runtime stack 파일은
  `ForEachTextLine` 경로를 기본으로 둔다.
- 테스트와 small caller 호환을 위해 slice 반환 helper는 남길 수 있지만,
  analyzer entrypoint는 가능한 callback iteration을 사용한다.
- `ForEachTextLineWithContext`는 debug context를 위해 previous/current/next
  line만 유지하며 파일 전체를 중복 보관하지 않는다.
- Parser diagnostics는 `max_lines`와 time filter 적용 이후에도 읽은 physical
  line, parsed record, skipped record 수를 계속 기록해야 한다.

큰 결과 payload는 parser memory와 별도로 제한한다. GC log series는
`MaxSeriesPoints`와 deterministic event-bucket downsampling을 사용해 차트
payload가 모든 이벤트 수에 비례해 증가하지 않게 한다. JFR JSON과 HTML
profiler 입력은 아직 구조화 payload materialization이 필요하므로 파일 크기
preflight를 적용한다.

### Percentile Sampling

Access-log summary와 per-minute percentile 값은 unbounded response-time array가 아니라 bounded deterministic sample을 사용한다. 이 방식은 대용량 파일에서도 percentile 계산용 메모리를 고정된 크기로 유지하고, 같은 입력에 대해 재현 가능한 결과를 만든다. 다만 sampler는 근사 방식이며 입력 순서의 영향을 받을 수 있으므로, 매우 정렬된 입력이나 의도적으로 편향된 입력에서는 percentile 값을 exact statistic이 아니라 운영 판단용 estimate로 해석해야 한다.

Sampler seed는 positive integer여야 한다. `seed=0`은 deterministic replacement stream이 퇴화하여 같은 reservoir slot을 반복적으로 선택할 수 있으므로 거부한다.

### Access Log Findings

Access-log finding은 `metadata.findings` 아래의 bounded structured observation이다. 초기 rule은 다음과 같다.

- error rate가 10% 이상이면 `HIGH_ERROR_RATE`.
- error rate가 5% 이상이면 `ELEVATED_ERROR_RATE`.
- 하나 이상의 `5xx` response가 있으면 `SERVER_ERRORS_PRESENT`.
- 가장 느린 URL 평균 응답 시간이 1000 ms 이상이면 `SLOW_URL_AVERAGE`.
- Gateway 또는 edge log가 retry attempt를 보고하면 `EDGE_RETRIES_PRESENT`.
- HAProxy termination state가 비정상 close 또는 server/client-side failure를
  나타내면 `HAPROXY_TERMINATION_ERRORS`.
- Gateway latency percentile이 edge tier의 의미 있는 지연을 나타내면
  `GATEWAY_LATENCY_ELEVATED`.
- 추론된 caller-to-upstream service edge의 HTTP error가 높으면
  `HIGH_ERROR_SERVICE_EDGE`.

Finding은 stable `code`, `severity`, 짧은 `message`, 작은 structured `evidence`를 포함해야 한다.

### 프로파일러 순서 지정 정규식 검증

순서 지정 정규식 필터는 스택 탐색 전에 전체 패턴의 제한을 검사하고 각 항을 한 번씩
컴파일합니다. 입력에서 도달할 수 없는 항도 검증합니다. 잘못되거나 제한을 넘는
포함·제외 필터는 `INVALID_REGEX` 또는 `UNSAFE_REGEX` 진단과 빈 단계를 반환합니다.
정상적인 0건 결과나 필터가 적용되지 않은 제외 결과로 오인하지 않도록 하며,
기존 순서·대소문자·루트 재설정 규칙은 유지합니다.


## T-628 구현 기록 (2026-11-11)

기준 커밋은 `eb3e782`다. C1–C4는 공통 기반과 기존 Tomcat 서버 로그 pilot에
연결했다. 최초 구현 시점에는 독립 리뷰가 없었으며, 이후 CONDITIONAL 판정과
후속 수정은 아래 리뷰 기록을 따른다. 테스트 통과가 독립 PASS를 뜻하지 않는다.

| 단계 | 구현 | 검증 |
|---|---|---|
| C1 | ingestion의 immutable descriptor/catalog, ID·alias·schema ID/version/hash·family 대조, verified/hint 후보 집합과 명시 선택, catalog의 built-in 조립·legacy confidence wrapper | importer_test: 중복·reserved alias·schema/family 오류·잘못된 점수·hint/충돌·옵션 hash·deadline. 기존 confidence 및 thread mixed/override 테스트 유지 |
| C2 | analysis.Service, owned SourceLease snapshot과 두 번째 hash/stat 확인, 동기 normalize/sink·seal, 자원 제한과 취소, 안전한 partial·검증·atomic publish | service/sink 테스트: 선택 전 파싱 금지, 동일 선택 멱등, stale generation, 강제 형식 충돌, source 변경, 취소 전/후 commit, budget/unsafe partial, 이전 리비전·cleanup 보존 |
| C3 | wirejson/resultcontract, common·legacy metadata, operation별 호환 gate, opaque JSON bridge | DATA_MODEL의 W01–W10 회귀와 별도 bridge·frontend 검사 |
| C4 | 기존 서버 로그 parser factory와 streaming analyzer 재사용, evidence CLI와 EngineService job/load/export 연결 | 실제 CLI 실행 및 Wails 메서드 통합 테스트, 같은 family의 합성 adapter를 등록만으로 추가하는 extension_test |

새 CLI는 `evidence formats`, `evidence import --in FILE --importer tomcat --out result.json`,
`evidence export --in result.json --export-format json|csv|html|report-pack --out FILE`이다.
`--options '{"top_n":25,"max_lines":0,"strict":false}'`와 `--allow-partial`을 지원한다.
hint/충돌은 후보를 출력하고 선택을 요구한다. 기존 family CLI alias와 thread override,
기존 AnalyzeServerLog 등은 유지한다. 새 catalog의 pilot은 `server.tomcat`/`tomcat`뿐이며
기존 parser 전체를 새 경로로 이행했다고 주장하지 않는다.

Wails는 ListEvidenceImporters, ImportEvidence, GetImportJob, SelectImporter,
CancelImport, LoadEvidenceResult, GetEvidenceResult, GetEvidenceResultJSON,
GetEvidenceScalar, CompareEvidenceScalar, ExportEvidenceResult를 제공한다.
options는 JSON 문자열, 결과는 immutable handle과 bounded view/JSON 문자열이다.
사용 가능한 형식·operation은 backend catalog가 결정하며 imported capability가 활성화하지 않는다.

운영 기본 상한은 입력/정규화 각 64 MiB, serialized record 1 MiB, record 100,000개,
stack 깊이 1,024, unique key 100,000개, 임시 디스크 160 MiB, 큐 대기·실행 각각 5분,
probe 64 KiB/검출기당 1초, options 64 KiB다. 요청은 이를 낮출 수 있으며 올릴 수 없다.
활성 job은 4개, process 내 job history는 128개로 제한한다. timeout은 협력적으로
전파하며 OS 파일 I/O나 신뢰된 compiled 함수의 강제 선점·sandbox를 제공하지 않는다.
lease는 parser가 반환하지 않은 reader도 종료 시 닫는다. 임시 파일 정리 실패는
cleanupPending으로 노출하고 shutdown 때 재시도한다.

이번 구현에서 atomic publish 단위는 provenance를 포함한 결과 JSON 리비전 파일이다.
별도 case/index manifest는 T-630 책임으로 남겨 두었으며, 동일 파일시스템 rename이
이 단계의 완료 marker 역할을 한다. 실패·취소는 기존 리비전을 덮어쓰지 않는다.
원본을 보존하는 read-only Load는 정상 분석 job commit과 구분하며 미지원 의미를 계산하지 않는다.

pilot은 비압축 UTF-8 및 probe 내 확인 가능한 서버 로그 record를 지원한다.
다른 encoding·빈/미확인 prefix·seek·승인된 converter가 필요한 입력은 기존 경로 또는
후속 adapter를 사용한다. producer 버전을 로그 형식만으로 추정하지 않는다.
공유 합성 manifest `diagnostics.ingestion.foundation.v1`와 측정 schema를 해시로 묶어
배포본에 포함한다. 후속 수정에서 고정 버전 실행기로 번들 JSON Schema의 조립 시 적합성을 검증하며
외부 schema 로딩은 차단한다.

실제 CLI와 Go bridge, TS 상태/opaque transport, 패키지 검증은 각각 기록한다.
새 generic job 화면·실제 WebView IPC 실행 증거는 이 단계에서 만들지 않았다(T-631).
Windows 네이티브 실행은 Mac에서 수행하지 않았으며 교차 빌드와 구분한다.
전체 ProfileSet(T-629), generic renderer(T-631), 프로파일 수치 정정(T-634),
case 수명·cache 정리(T-630)와 Q01–Q04는 이 구현으로 완료되지 않는다.

### T-628 독립 리뷰 후속 수정 (2026-11-11)

`a698d26`에 대한 Claude 리뷰 원 판정은 **CONDITIONAL**이다. 원문은 변경 없이
[보관](../review/done/2026-11-11_claude-code_T-628_implementation-review.ko.md)했다.
아래 내용은 앞의 최초 구현 시점 기록을 보완한다. 로컬 검증으로 독립 PASS를
선언하거나 T-628의 REVIEW 상태를 종료하지 않는다.

- **R-01:** 전체 결과 트리에는 `2 × stackDepth + 64` JSON 깊이(1,024프레임
  정책에서 2,112)를 허용하고 metadata/options/record는 깊이 64를 유지한다.
  import 서비스는 결과 바이트 정책 64 MiB를 유지한다. 기존 report reader는
  별도 512 MiB 상한을 적용하고 초과 시 바이트 예산 오류로 거절한다. 과거
  무제한 읽기에 비해 새 운영 상한이므로 명시한다. 유효한 65 MiB JSON과
  256프레임 실제 CLI 보고서 및 Wails 직접 메서드 load/export를 검증한다.
  트리·바이트 상한 초과와 과도하게 깊은 opaque metadata는 거절한다.
- **R-02/R-13:** 작업 스냅샷은 최대 128개다. 새 작업 접수 시 가장 오래된
  종료 이력부터 회수하며 실행 중 작업과 결과 revision은 보존한다. 회수된 ID는
  IMPORT_JOB_STALE을 반환한다. 큐 대기와 실행은 각각 5분 상한
  (`queueTimeoutMs`, `timeoutMs`)이며 독립적으로 낮출 수 있다. 실행 예산은
  동시 실행 슬롯을 얻은 뒤 시작한다.
- **R-03/R-11:** 신뢰된 schema 등록에 input/option/record/result 역할, 의미 계열,
  결과 타입을 선언한다. catalog는 각 슬롯과 normalizer/analyzer 연결을 확인하고
  schema 정의 및 컴파일된 validator 양쪽으로 적합성 사례를 실행한다. 서비스
  활성화 시 모든 analyzer 출력의 result reader 등록과 선언한 payload 버전의
  reader 호환 범위도 확인한다(schema 문서 revision과 분리). 고정 버전의
  JSON Schema draft 2020-12 실행기를 조립 시 적합성 검증에 사용하며 실행 중에는
  신뢰된 compiled adapter가 검증한다. 외부 파일·URL 로딩은 차단하고 schema
  상수·적합성 피연산자의 연산 예산을 opaque 보존 경로와 분리한다. 정의 hash는
  데이터 식별자이며 실행 함수의 hash가 아니다. fixture
  manifest 기대 카운터도 pilot 결과와 대조하여 실행한다.
- **R-07/R-09:** detector 오류는 importer별 정제된 진단으로 제한한다(최대
  catalog 256개). 무관한 detector 오류로 명시 선택한 정상 importer를 막지 않는다.
  detector가 실패하면 자동 선택은 보류하고 정상 후보의 명시 선택을 요구한다.
  선택한 detector 자체가 실패한 경우 거절한다. 부모 취소·detector 시간 초과·
  버퍼 초과·입력 I/O를 구분하며 JobProblem 재시도 여부는 오류 코드로 결정한다.
  원 예외·경로 문자열을 JobProblem에 노출하지 않는다. 신뢰된 실행 함수의
  시간 제한은 협력적 취소다.
- **R-05/R-10:** `max_lines`는 완료된 사용자 선택이며
  `import_coverage.selection=max_lines`로 범위를 기록한다. R3에서는 공백이 아닌 rune이 suffix에
  남을 때만 selection을 표시한다. 정확한 EOF 및 빈 줄/Unicode 공백만 남으면
  표지를 남기지 않는다. SourceLease 입력 바이트 예산과 job 취소/deadline 아래에서
  고정 메모리로 검사하며 첫 비공백에서 중단한다. 나머지 공백 바이트를 읽을 수는
  있지만 physical line을 추가 parse/count하거나 진단 수치를 바꾸지 않는다.
  잘못된 UTF-8은 비공백으로 보고 제외된 입력을 파싱하지 않는다. 읽기 오류는
  IMPORT_IO_FAILED다. 과거 저장 coverage는 보존하며 원본 재import로 수정된
  공백 suffix coverage를 얻어 다른 원본과 비교할 수 있다. allow-partial을 요구하는 자원 절단과 구분한다. 자원 예산
  절단은 기존의 importer 안전성 및 사용자 동의 조건을 유지한다. 기존 strict/
  top_n, summary/tables/correlation_keys와 마지막 빈 줄의 레코드 수를 검증한다.
- **R-08:** import와 Load는 동시 실행 4개 및 종료 취소·대기를 공유한다.
  OS 프로세스 잠금으로 저장소를 보호하며 독점 소유자만 시작 시 이전 실행의
  staging을 회수한다. 동일 스냅샷의 중복 로드는 handle을 재사용한다. 결과는
  1,024개 / 512 MiB로 제한하고 자동 삭제하지 않는다. R2 N-02에서
  ReleaseEvidenceResult를 공개 Wails API/binding에서 제거한다. T-630의 호출자/
  Workspace 소유권 설계 전까지 backend Release는 관리용 삭제이며 참조 해제가 아니다. 결과가 없으면 IMPORT_RESULT_STALE과 저장
  파일 재열기 안내를 반환한다. 사용자가 내보낸 파일은 이 backend 캐시와 별개다.

T-630에는 영속 case manifest, 저장된 workspace 참조의 이전·복구, 중복 handle
해제 전 참조 소유권, 사용자 보존·삭제·복원 및 index 수명주기가 남는다. 데스크탑
저장소는 OS 사용자 캐시 위치이므로 외부에서 지워질 수 있으며 handle만으로
영속 저장된 case라고 볼 수 없다. T-631에는 Load별 UI 취소와 job/fallback 화면
연결이 남는다. 이 조건들은 work_status.md에 기록하며 이번 공통 기반 수정의
완료 범위로 주장하지 않는다. Q01~Q04의 제품·입력 결정은 확정하지 않는다.

## T-629 구현과 이행 경계 (2026-11-11)

기준 경로는 `profile.set-json importer → ProfileSet → analyzers/profile.BuildSet
→ AnalysisResult(profile_evidence 0.2.0)`이다. 내부 JSON 교환 경로는 composition root에
schema 역할·fixture hash·compiled analyzer로 등록한다. `profile_ids`와 `metric_id`를
명시해야 한다. ProfilerService의 기존 Analyze/loadStacks는 `profilelegacy`의 명시적
호환 adapter catalog를 사용하며 형식별 service switch를 추가하지 않는다. 공통 generic
trie를 정밀 모델 연산과 기존 정수·float 표시 adapter가 함께 사용한다. 큰 기존 입력에
두 번째 전체 모델을 만들지 않는 경로를 유지하고 collapsed/Jennifer/SVG/HTML의 수치
의미·버전도 유지한다.

`parsers/profile.ParseLegacySet`은 SourceLease 스냅샷에서 기존 정규화 레코드를 감싸고
복원 가능한 frame identity·profile·weight·opaque 정규화 metadata를 보존한다. 품질
블록에는 기존 의미와 chronology 부재를 명시하고, 원래 timestamp 값은 label로 보존하며
측정 interval로 추정하지 않는다. 정규화 과정에서 절단된 결과는 거절한다. 이 wrapper로
기존 decoder가 이미 잃은 소수·0·event duration·다중 value를 복구할 수는 없다. 해당
producer decoder 정정·실제 버전 적합성·지원표 정정은 T-634이며 Q01~Q04나 새 제품 지원을
이번 작업에서 확정하지 않는다.

사용 예:

```sh
archscope-engine evidence import --in model.json --importer profile-set \
  --options '{"profile_ids":["worker-a","worker-b"],"metric_id":"duration"}' --out result.json
archscope-engine profile-set tree --in result.json
archscope-engine profile-set diff --in result.json --after result.json
archscope-engine profile-set drilldown --in result.json --filters '[]'
archscope-engine profile-set pprof --in result.json --tick-scale 0.01 --out selected.pprof
archscope-engine profile-set normalize-legacy --in samples.collapsed --format collapsed --out normalized.json
```

normalize는 제한이 명시된 legacy-normalized 모델을 생성한다. EngineService에는
NormalizeLegacyProfileSet/GetProfileSetTree/CompareProfileSets/DrilldownProfileSet/
ExportProfileSetPprof를 추가한다. 새 Wails 값은 취소 가능한 context와 JSON text/handle로
전달하며 모델 수치를 JavaScript number나 구형 정수 DTO로 왕복시키지 않는다. 분석·JSON
저장·재읽기·CSV/HTML·Report Pack·선택한 pprof가 보존된 selection을 사용한다. query는
서비스 동시 실행·수명 제한을 공유하고 immutable 모델을 읽는다.

기존 profile golden은 변경하지 않는다. 63d3b6a 기준 CLI와 collapsed/Jennifer/SVG/HTML/
drilldown을 비교하며 생성 시각·progress log 경로만 제외한다. 추가 fixture는 소수·0·큰 정수,
signed/재귀/동명 frame, 여러 profile/metric, 정확한 scale, 불가 projection 거절, 버전 번호
변경만으로 활성화하지 않는 조건, 원본·선택 호환을 검증한다. Wails 직접 메서드·생성
binding 검사는 실제 WebView UI 실행과 별개다. generic 화면은 T-631, case/baseline
수명주기는 T-630/T-635에 남는다.

## T-629 리뷰 후속: bounded 규모와 비교 (R4)

내부 profile adapter 1.0.1은 ProfileSet 입력 1.0.0과 AnalysisResult profile_evidence
0.2.0의 수치 의미를 유지한다. parser가 bounded 모델을 확인한 뒤 raw 256 KiB 이하
데이터와 정확한 byte offset의 profile-set.chunk.v1 record를 내보내고 analyzer가
32 MiB 안에서 재조립한 완성 모델을 검증한다. 공통 record 1 MiB, 정규화/결과 각
64 MiB, temp 160 MiB 정책은 올리지 않으며 base64 증폭도 spool/temp에 계산한다.
bounded 전송이며 producer decoder의 완전 streaming 이행은 아니다. 공통 source
lease 일관성·backpressure·취소·atomic commit·실패 cleanup을 유지한다. 실효 상한,
크기 증폭, 예산 세부 사유는 DATA_MODEL에 명시한다.

64 KiB detector는 probe 안에서 끝나는 완전한 모델만 검증한다. 더 큰 JSON은 hint로
명시 importer 및 profile/metric 선택과 전체 입력 검증을 요구한다. 잘린 JSON 접두부로
verified 지원을 부여하지 않는다. 공유 규모 사례는 3,000 stack 및 50,000개 이상 tree
node를 검증한다. 품질 비교·과거 저장 정책 호환·registry 검증 캐시는 DATA_MODEL을
따르며 metadata.quality 사본도 모델과 대조한다. 새로운 producer 지원이나 entity
매핑은 주장하지 않는다(T-634/T-635).

R5-01에서 wire 디코더의 자원 오류를 ProfileSet 입력 경계까지 보존한다. 개별
문자열/key 4 MiB 초과는 형식 불일치가 아닌 상세 예산 거절이고, 경계와 초과 import를
검증한다. 잘못된 JSON은 형식 오류를 유지한다. R5-02 capture key 정책은 DATA_MODEL의
T-634/T-635 인계 계약을 따르며 현재 불투명 옵션 전체 비교는 유지한다.

## T-630 안전한 intake·adapter 경계 (2026-11-11)

`internal/cases`는 기존 analysis 바깥의 context/저장/intake 책임이며 profile parsing,
weight 재해석, T-631 generic renderer를 구현하지 않는다. ZIP/TAR/tar.gz preview는 전체
항목을 검사하고 archive SHA256으로 실제 intake를 고정한다. 파일 128개, 경로 16요소,
파일당 64 MiB, 압축/해제 전체 256 MiB로 제한하며 ZIP central directory도 표준 라이브러리
할당 전에 검증한다. split/ZIP64·암호화·link/특수 파일·TAR extended/sparse, 절대/탈출/
Windows 예약 경로, Unicode·대소문자 정규화 중복, 파일-부모 충돌, 중첩 압축을 거부한다.
실제 크기/CRC도 확인한다. archive 권한·소유자·mtime은 적용하지 않는다. 선택 내용은
상한이 있는 메모리에 staging하고 구조 검사를 모두 통과한 뒤 object와 한 manifest HEAD를
확정한다. capture/명시 hash 실패는 failed/omitted 원본 기록으로 남기며 성공 object로
발행하지 않는다. 구조 오류·취소는 revision을 발행하지 않는다. basename/mtime이 아닌
명시 capture file/entity 관계로만 구분한다.

별도 decoder는 명시 encoding → BOM → 유효 UTF-8 → 명시 허용 legacy sniff 순이다.
CP949 확장은 strict EUC-KR에서 거부하며 모호하면 사용자가 선택해야 한다. 손실 대체는
명시 허용 시에만 수행하고 수를 기록한다. UTF-16 surrogate pair/BOM 위치/CRLF를 검증한다.
행별 원본 반개구간 byte와 디코딩 UTF-8 위치는 decimal string이다. source digest, decoder
revision, encoding, replacement 수를 index revision에 고정한다. encoding 수정은 재구축과
종속 artifact stale 처리를 동반한다. source page는 최대 200행·행당 4 KiB·전체 64 KiB다.
비식별 복사본은 parent digest·redaction profile을 가진 별도 UTF-8 source이며 좌표는 그
복사본 기준이다. 바뀐 길이를 원본 좌표라고 주장하지 않는다. 외부 원본은 읽기 전용 regular file이며 filesystem link는 명시적으로 해소해야 한다.
내용/hash·stat을 재대조해 읽는 중 변경을 거부한다. HEAD 확정 직전 취소를 확인하고
이미 확정한 revision은 되돌리지 않는다. 기존 textio 및 parser golden 동작은 유지한다.

`analysis.OpenDurable`은 양쪽 커널 lock 하에 기존 OS cache를 앱 데이터로 이관한다.
결과별 decode/publish 뒤 최종 버전 marker를 쓰며 중단된 복사는 content dedup으로 재시도한다.
원본 cache는 유지하고 case purge 제외 범위로 명시한다. 완료 marker는 이후 의도적으로 지운
cache를 재생하지 않는다. saved case는 결과 바이트를 직접 소유하고 `LoadSnapshot`으로
재개하므로 예전 handle/cache 부재와 독립적이다. 호출자별 opaque lease를 획득/해제하며
administrative Release는 owner가 남으면 거부한다. 위험한 공개 ReleaseEvidenceResult는
계속 노출하지 않는다. Workspace는 자기 lease만 관리하고 clear 이후 늦은 reopen을 거부하며
작은 saved-case 메뉴로 저장/열기를 제공한다. JS projection 대신 backend 원본을 저장하고
backend handle이 없는 legacy 항목은 메모리에만 둔다. 원본 연결·수명주기·portable 조작은
CLI/Wails에서 제공하며 generic 사건 화면·탐색·통합 검색은 T-631/T-633/T-635 범위다.

CLI는 `case create --title ...`, `case list`, `case show --id ...`,
`case edit --request edit.json`, `case save-result --in result.json --request refs.json`,
`case export-result --id ... --result ... --out ...`, `case preview --in bundle.zip`,
`case pack --id ... --request reviewed-selection.json --out case.zip`,
`case restore --in case.zip --sha256 <preview digest>`, `case delete-plan --id ...`,
`case purge --id ... --revision ... --token ...`, `case rebuild-index`, `case cleanup-cache`를
제공한다. `--store`는 격리 저장소를 지정한다. edit JSON은 case_id/expected_revision과
operation(details/retention/workspace/capture/source/replace-source/encoding/omit-source/
archive-intake/review-source/review-result/artifact/baseline/archive/restore/delete)을 사용한다.
SaveResult는 case_id/expected_revision/source_ids/capture_id/selection/options_hash/versions로
참조를 고정하며 원본 없는 snapshot 저장은 source_ids가 빈 배열이다. portable 선택은
manifest_reviewed/source_ids/result_ids/include_workspace이고 저장/복원이 공유를 자동 승인하지
않는다. Wails는 JSON 문자열과 문자열 lease를 사용한다. Go bridge 검증은 실제 WebView IPC/UI,
Windows runtime 증거가 아니며 작성자 검사로 독립 리뷰 PASS를 선언하지 않는다.

사건 export는 managed 저장소와 현재 외부 원본 참조/alias를 덮어쓰는 경로를 거부한다.

## T-631 공통 소비자 연결 (2026-11-11)

`AnalyzeEvidence`는 기존 `ImportEvidence` catalog/job 경로를 재사용하고 기존 `Analyze*`
계약을 보존한다. 같은 의미 family의 adapter는 패키지·스키마·fixture·등록을 추가하여
결과 descriptor와 공통 화면을 재사용한다. service/CLI/UI에 포맷 switch를 추가하지 않는다.
새 의미 provider는 컴파일된 선언과 payload 적합성 검증이 필요하며 metadata 주장으로
활성화되지 않는다. timeline·SLI는 제한된 행 투영으로 표시하며 새로운 overlay·SLO 정책·
출처 간 stitching 알고리즘을 추가한 것으로 해석하지 않는다.

CLI는 `evidence analyze`(import 별칭), `evidence describe --in result.json`,
`evidence query --in result.json --provider ID --offset 0 --limit 25`,
`evidence compare --in baseline.json --target target.json --provider ID`를 제공한다.
비교 거절은 구조화된 출력이며 0을 만들지 않는다. Wails에는 context 취소가 가능한
`OpenEvidenceResult`·`DescribeEvidenceResult`·`QueryEvidenceProvider`·
`CompareEvidenceProvider`를 추가하고 기존 load/export API를 보존한다.

Analysis Workspace에서 catalog 가져오기·옵션·명시적 부분 허용·모호한 형식 선택·작업
상태/취소·저장 결과 열기·기능 선택·문자 기반 요약/표/진단/상세·페이지 이동·순서가 명확한
비교·정밀 JSON 저장·명시적 Workspace 추가를 제공한다. 케이스에서 연 snapshot도 같은
화면을 사용한다. 화면은 자신의 lease만 소유하고 늦은 획득을 해제한다. Wails context로
개별 load/query를 취소하고 이전 응답을 거부하며 페이지 handle/digest/provider를 확인한다.
늦게 도착한 Start 응답도 CancelImport로 정리한다. 오래되거나 손상된 handle은 저장 파일
또는 케이스 snapshot으로 복구한다. 백엔드 handle 없는 기존 Workspace 결과의 화면은
유지하며 정밀 backend snapshot으로 직렬화하지 않는다. Windows 실기는 별도 Windows PC
작업으로 남는다. 단위·Go bridge·상태·빌드·CLI 검사만으로 실제 WebView 조작을 인증하지 않는다.

## T-630 R6 원본 페이지·복구 (2026-11-11)

`Store.SourcePage`는 리비전·원본 digest·옵션에 연결된 compact 색인 한 개를 사용한다.
디코딩 UTF-8 최대 64 MiB, 좌표 최대 1,000,000개(각 16바이트)다. 반복 페이지는 디코딩을
재사용하고 제한된 스트리밍 hash로 현재 원본을 확인하므로 mtime만 신뢰하거나 전체 입력을
다시 할당하지 않는다. 크기·mtime을 유지한 내용 변조도 거부한다. 사건 수정·삭제·정리·종료 시
파생 텍스트를 폐기한다. 기존 유효 UTF-8/UTF-16/CP949/EUC-KR 입력의 좌표와
`text-index-v1` 식별 의미를 유지한다.

색인 한도에 도달해도 제한된 입력 전체를 검증·계수한다. `total_lines`·`indexed_lines`·
`index_truncated`로 범위를 공개하고 파일 전체를 거부하지 않고 앞 100만 행을 읽는다.
색인 뒤를 요청하면 빈 행과 명시적 잘림 flag를 반환하며 완전한 EOF를 주장하지 않는다.
원본·내보내기는 완전하게 유지한다. T-635는 이 범위를 반영하거나 byte window 접근을 추가한
뒤 더 넓은 원본 링크를 제공해야 한다. 페이지 200행·행별 4 KiB·텍스트 합계 64 KiB 한도는
유지한다. 기존 전체 `ReadSource`는 Wails 페이지 경로가 아니며 더 큰 타입화된 색인을 할당할 수 있다.

새 정비 API는 `CleanupEvidenceCache`·`GetCorruptCaseDeletionPlan`·`PurgeCorruptCase`다.
손상 삭제는 `case corrupt-delete-plan --store PATH --id ID`로 최신 계획·범위를 확인한 뒤
그 token으로 `case purge-corrupt --store PATH --id ID --token TOKEN`을 명시적으로 실행한다.
일반 사건 수명주기와 별도이며 CLI cache 정비는 저장소를 소유한 앱을 먼저 닫도록 안내한다.
사용자 사건에 정리·purge를 자동 실행하지 않는다.

### T-631 R8 소비자 경계

참조 집합 준비와 비공개 표시/열 캐시는 파서/분석기가 아닌 result contract와
분석 서비스의 책임이다. 표시용 제어문자 처리는 정규화 레코드, 메트릭 이름,
출처 식별자나 원본 내보내기를 바꾸지 않는다. 캐시는 일시적이며 영구 원본/검색/
baseline 색인은 T-635 범위다. [데이터 모델](DATA_MODEL.md)을 참조한다.

## T-633 입력 안내 경계 (2026-11-12)

`inputassist`는 제한된 소유 스냅샷 기반 후보와 명시적 캡처 파일 결합을 관리한다.
결과를 파싱하거나 확장자/레시피로 지원을 부여하지 않는다. `localconvert`는 설치
도구 실행, 고정 인자, 스트리밍 한도, 취소, provenance를 관리하며 JFR 파서/분석기는
각각의 책임을 유지한다. 일반 카탈로그와 공통 AnalysisResult 계약은 바꾸지 않는다.
한도/거부 코드와 수동 키트·CLI·UI 사용은 [입력 안내](INPUT_ASSISTANCE.md)를 따른다.
런타임 자산은 내장하며 형제 체크아웃이 필요하지 않다.

### T-634 원본 의미 버전

정확한 원본 normalizer는 ProfileSet schema 1.0.0에
`org.archscope.profile_source.v1` revision 1.0.0을 추가하고 AnalysisResult
profile_evidence 0.3.0을 생성합니다. 기존 facade·결과·v2 hash 의미는 유지합니다.
새 공통 transport `profile.source-set-json` (별칭 profile-source-set)은 기존
profile.set-json / payload 0.2.0과 분리됩니다. 모든 profile과 값·단위를 보존하고
선택을 명시합니다. fixture로 검증한 컴파일 정책과 v3 canonical 비교는 양쪽 원래
옵션 및 incidental 변경을 공개합니다. 알 수 없는 binding은 strict이며 일반
scalar 비교도 T-635 연결 전까지 전체 capture 동등성을 유지합니다. 기존 R5의
“구현 대기” 설명은 이번에 검증한 canonical 원본 범위에서만 갱신됩니다.
형식 의미·한도·검증·정책 key·CLI/화면은
[정확한 원본 프로파일 가져오기](PROFILE_SOURCE_IMPORT.md)를 참고하세요.

### T-635 공통 조회·검증 baseline과 R11 의미 버전

[케이스 근거 조회](CASE_EVIDENCE_QUERIES.md)는 제한된 cursor 기반 profile 보기·근거,
색인 범위를 공개하는 원본/summary 검색, 원본 좌표, 선택한 불변 결과와 baseline
재분석·매핑 gate를 정의합니다. R11 수정은 normalizer 1.1.0 / 결과 0.4.0을 생성하고
원래 type/unit을 producer 정책 없이 metric identity에 포함합니다. Speedscope none은
일반 count 가중치이며 표본 수를 단정하지 않습니다. 기존 0.3.0은 바이트·hash·읽기·저장·
조회·내보내기를 유지하되 비교에는 새 원본 재분석이 필요합니다. 새 정책은 선언/
file-exporter basis와 경고를 공개합니다. 영속 baseline 검증은 매번 양쪽 원본을 다시
분석하고 identity_basis=declared를 유지합니다. 일반 scalar 비교는 계속 strict이며
새 canonical/baseline 화면이 검증 정책을 소비합니다. 시계 정렬·미검증 producer 지원을
추정하지 않습니다. Windows 네이티브/UI 및 독립 리뷰는 별도입니다.

### T-638 메트릭 이력

[정확한 메트릭 이력](METRIC_HISTORY.md)은 모델/normalizer 1.0.0,
`metrics.history-json` transport와 `metrics_timeseries` payload 0.2.0(기존 저장 0.1.0 reader 유지)을 정의합니다.
중립 snapshot sequence·명시적 query 출처의 Prometheus range matrix·OTLP JSON에서
정확한 값, resource/scope/schema/속성 식별자와 temporality를 유지합니다.
Delta/cumulative/reset/gap/overlap과 explicit/exponential bucket은 별도 의미입니다.
공통 envelope로 상세/timeline·저장 조회·export를 연결하고 기존 metrics_snapshot 의미는
유지합니다. SDK Python 1.38.0 합성 표본 범위만 검증하며 Q04/GenAI alias와 네이티브
수용 검증은 별도입니다. 자동 합산/quantile 계산은 없습니다. R12 baseline 캐시는 새 query
재분석 결과만 프로세스 내 제한 보관하고 페이지마다 원본/결과 hash와 case binding을
재검사합니다. 기존의 매번 전체 재분석 설명은 이 범위에서 대체합니다.

## T-637 시스템 근거 adapter

`system.evidence-json`은 SystemEvidence 1.1.0 JSON을 256 KiB 이하 chunk로 받고 compiled
형태/의미 검사와 digest 고정 fixture를 사용합니다. 원본은 capture 문맥을 갖춘 명시적
perf-script/events-jsonl/bpftrace-jsonl 정규화로 받으며 도구를 실행하지 않습니다.
`system_evidence` 0.2.0에 원본 관측·별도 scheduler 구간 분석·집계 bucket을 보존합니다.
[시스템 근거](SYSTEM_EVIDENCE.md)에 fields/버전 범위·수명/loss/clock 거부·한도·원본
drilldown을 정의합니다. 기존 profile/metric 저장 reader는 유지합니다.


## T-639 장치·서빙 근거

제한 지원 `inference.evidence-json`은 InferenceEvidence 1.0.0을 공통
`inference_serving` 0.1.0으로 처리한다. 원본 MetricHistory를 한 번만 저장하고
signal은 series/entity/context/unit/통계 형식/모집단을 명시적으로 연결한다.
누락 값은 unavailable이다. NVIDIA CSV는 고정 query 계약을 요구하고 fixture는
합성이며 Q03의 특정 vLLM 버전 alias gate는 유지한다. Parser·의미 analyzer·
generic/export 책임은 분리한다. 한도·정확한 요청 공식·원본 메트릭 상세와 미실행
native 수용 범위는 [오프라인 장치·서빙 계약](INFERENCE_EVIDENCE.md)을 참고한다.

SystemEvidence 1.0.0 / system_evidence 0.1.0 저장 reader는 유지하며 새 1.1.0은 frame/stack을 중복 제거하고 0.2.0 결과와 과거 snapshot을 분리한다.

## T-640 구현 경계 (2026-11-12)

제한 importer `trace.llm-json`은 LLMTrace 1.0.0을 기존 trace_import 0.3.0 / llm_core 1.0.0으로 분석한다. 기존 trace 0.2.0 reader는 유지한다. Parent/link·자식 합집합 경과 시간·명시적 논리/시도 accounting·token 경계·lifecycle 집단을 구분하고 본문 제외 정규화를 케이스 복사/색인보다 먼저 적용한다. 정확한 페이지 query/UI와 기존 provider/export를 재사용한다. Q01 제품 JSON adapter는 유지한다. [LLM trace 근거](LLM_TRACE_EVIDENCE.md) 참조. T-639 R16은 untyped range의 명시적 메타데이터와 입력 event ID 생성을 추가하며 1.0/1.1 모델·0.1/0.2 결과 reader를 유지한다. [추론 서빙 근거](INFERENCE_EVIDENCE.md) 참조.


## T-641 런타임 근거

런타임 근거는 AnalysisResult 안의 별도 정규 모델을 사용합니다. 고정 producer 형식·identity/window 확인·live allocation 프로파일 재사용·GC 합집합·실행계획 loop당 의미·저장 reader 경계는 [RUNTIME_EVIDENCE.md](RUNTIME_EVIDENCE.md)를 참고하세요.


## T-642 실행 근거

제한된 benchmark/crash 문맥은 공통 AnalysisResult의 execution_core 1.0.0 / execution_evidence 0.1.0을 사용합니다. 고정 k6/LLMPerf·합성 JTL/정제한 실제 HotSpot 입력을 정규화한 뒤 기존 provider/case/export를 사용합니다. 비교는 선언 조건 snapshot 검증이며 profile 원본 재수입 baseline과 구분합니다. [EXECUTION_EVIDENCE.md](EXECUTION_EVIDENCE.md)를 참고하세요.

T-643은 apple-ollama-capture.v1의 닫힌 제한된 입력을 검증하고 정확한 메트릭으로 정규화한 후 기존 generic import를 사용합니다. 가져오기에서 수동 수집을 실행하지 않습니다. [APPLE_SILICON_EVIDENCE.md](APPLE_SILICON_EVIDENCE.md).

R19-01은 큰 JTL의 정확한 분포용 execution_core 1.1.0 / execution_evidence 0.2.0을 추가하며 저장된 1.0.0/0.1.0은 유지합니다. [EXECUTION_EVIDENCE.md](EXECUTION_EVIDENCE.md).

T-644: trace.gpu-execution-json / trace_import0.4.0 / gpu_execution_core1.0.0은 정확한 CPU/GPU/rank 구간·명시적 연결·kernel 합계/합집합·집계 전용 근거를 분리합니다. CLI/Wails/Workspace·제한된 선택 SVG와 기존 소비자를 사용하며 저장된 trace/LLM reader는 유지합니다. [GPU_EXECUTION_EVIDENCE.md](GPU_EXECUTION_EVIDENCE.md).


T-645의 실제 DB·OS 첫 검증 배치: [수동 DB·소켓 스냅샷](SYSTEM_DB_SNAPSHOTS.md). 기존 MetricHistory·정확한 epoch/gauge·공통 소비자를 재사용하며 다른 어댑터는 실제 입력 검증 전까지 보류합니다.
