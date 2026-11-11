# 로컬 장애·성능 분석 개선 설계 리뷰 — 2026-11-11

- 대상: `docs/ko/LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md` (902줄, 작성일 2026-11-11, 상태 "개선 설계 제안")
- 대상 고정: 리뷰 시점에 git 미추적 파일이다. sha256 `72bb45064bfe96fd933dc1deece372b9bd770d1a5e1a234592ca79c5e24793d9`
- 코드 대조 기준: 커밋 `bbbe1ae`와 작업 트리의 `apps/engine-native`
- 작성: claude-code (리뷰 작성 역할만 수행). **처리는 다른 AI가 수행한다.** 이 문서는 `work_status.md`를 바꾸지 않았고 `done/`으로 옮기지 않았다.
- 방법: 설계 문서 전문 정독과 소스 정적 대조. 빌드·테스트·앱 실행은 하지 않았다.

## 1 결론

**조건부 승인.** 측정 의미를 지키는 부분(단위·분모·누락과 0의 구분·상관과 인과의 구분)은 구현 기준으로 삼기에 충분하다. 반면 **"새 프로파일·로그 타입을 손쉽게 추가하는 구조"는 이 설계에 들어 있지 않다.** 무엇을 지원 단위로 볼지는 정의했지만(§4.1), 그 단위를 코드에서 선언·등록·검증하는 계약이 없고 물리 배치는 "첫 구현 시 결정"(§16.5)으로 미뤘다. 현재 코드는 스레드 덤프만 플러그인 구조이고 프로파일·로그·메트릭은 `switch` 분기와 분석기별 서비스 메서드로 연결되어 있어, 이 상태로 §4·§11·§12의 입력을 늘리면 추가 비용이 입력 수에 비례해 커진다.

P1 4건(F-01~F-04)은 첫 구현 작업 전에 설계에 반영해야 한다. 나머지는 해당 기능 착수 전까지 반영하면 된다.

| 검토 포인트 | 판정 | 관련 finding |
|---|---|---|
| 1. 설계의 완전성 | 분석 의미는 충실, 운영·수명주기 시나리오 누락 | F-07, F-09, F-10 |
| 2. 확장성(플러그인/모듈 구조) | **미흡** — 확장 계약이 설계에 없음 | F-01, F-02, F-05, F-08 |
| 3. 독립 업그레이드 | 버전 항목은 언급, 버전 축과 호환 정책 없음 | F-06, F-10 |
| 4. air-gapped 호환성 | 분석 경로는 양호, 배포·갱신 모델 없음 | F-04 |
| 5. AnalysisResult 정합성 | 방향(additive)은 맞음, 두 envelope와 버전 정책 미해결 | F-03, F-12 |

## 2 Finding 요약

| ID | 등급 | 제목 | 설계 위치 |
|---|---|---|---|
| F-01 | P1 | importer/adapter 등록 계약이 없어 포맷 추가가 `switch` 수정에 묶여 있음 | §4.1, §16.5, §18.1 |
| F-02 | P1 | 공통 프로파일 모델과 기존 `map[string]int` 코어 사이의 이행 경로 없음 | §4.3, §16.5, §19 |
| F-03 | P1 | `metadata.*` 확장이 profiler envelope에서는 additive가 아니며 버전 정책이 없음 | §16.1, §2.1, §3.3 |
| F-04 | P1 | air-gapped 배포·지식 갱신·외부 변환기 모델이 한 문장뿐임 | §17.2, §4.6, §16.4, §18 |
| F-05 | P2 | 새 결과 타입 추가 시 서비스·CLI·프런트엔드 타입 분기를 모두 고쳐야 함 | §16.2, §13.3 |
| F-06 | P2 | 구성요소별 버전 축과 저장물 호환 정책이 정의되지 않음 | §3.2, §16.4, §14.5 |
| F-07 | P2 | 누락 시나리오: 수집 manifest, 사건 수명주기, 일괄 반입, 인코딩 | §3.2, §4.2, §9.1, §17.1 |
| F-08 | P2 | 코드 없이 확장하는 선언형 경로가 계층으로 정리되지 않음 | §9.1, §12.3, §15.1 |
| F-09 | P2 | 우선순위 표에 선행 기반 항목이 빠져 있음 | §19 |
| F-10 | P2 | 시각 자료 렌더링 위치와 headless 재생성, 기존 exporter와의 관계 미정 | §14.3, §14.5, §16.3 |
| F-11 | P3 | 영문 설계 문서가 없음 | 문서 전체 |
| F-12 | P3 | `gpu_metrics`와 `metrics_timeseries`의 경계가 모호함 | §16.2 |

이와 별도로 6절에 기능 보완 제안 20건(G-01~G-21, G-11은 G-15로 통합)을 두었다. 결함이 아닌 범위 제안이므로 등급을 매기지 않았고, 채택 여부는 처리자가 다시 판단한다. 이 중 6.2의 8건(G-14~G-21)은 사용자가 직접 확인한 AI 서비스·다언어 수요이며, 사용자 답변(vLLM·NVIDIA 위주, MacBook GPU, Python·Rust·Node)을 6.2에 기록했다.

## 3 P1 상세

### F-01 importer/adapter 등록 계약 부재

**근거**

- 설계 §4.1은 지원 단위를 `producer + producer_version + format + measurement_kind + capabilities`로 정했고 §18.1은 지원 선언에 필요한 근거 6가지를 요구한다. 그러나 이 정보를 importer가 어디에 선언하는지, 누가 검증하는지는 없다. §16.5는 "새 패키지의 정확한 이름과 물리 배치는 첫 구현 시 결정한다"로 끝난다.
- 현재 프로파일 포맷 분기는 하드코딩이다. `internal/parsers/profile/parser.go:109-134`가 포맷별 `switch`, `:719-744`가 별칭 정규화 `switch`다. `cmd/archscope-app/profilerservice.go:302-329, 451-456, 498-505`에 같은 포맷 분기가 세 번 더 있고 `:299-301` 주석은 "Do not add new formats to the legacy profiler switch below"라고 적어 구조 부채를 스스로 인정한다.
- 포맷 ID 하나(`pyspy-raw`)가 파서, 파서 테스트, `cmd/archscope-engine/cmd_profile.go`에 흩어져 있다.
- 재사용할 수 있는 기반은 이미 있다. `internal/threaddump/registry.go:83-88`의 `Plugin` 인터페이스와 `init()` 자기 등록은 실제로 동작하는 플러그인 구조다. `internal/ingestion/registry.go`의 `FormatRegistry`는 `Detector`·`ResultType`·`Parser`·confidence 기반 감지를 갖췄지만 비-테스트 코드에서 `NewFormatRegistry`/`DetectFile` 호출을 찾지 못했다. 설계는 두 기반을 언급하지 않는다.

**영향**: §4.1 표의 13개 계열, §5의 perf/BPF, §11의 GPU 자료 6종, §12의 vLLM adapter를 현 구조에 추가하면 분기 수정 지점이 계속 늘고, "파싱은 되지만 capability는 제한" 같은 상태를 코드에서 일관되게 표현할 수 없다. 사용자가 물은 "손쉬운 업그레이드 구조인가"의 답은 현재 **아니오**다.

**권장 조치**

1. 설계에 `ImporterDescriptor` 계약을 추가한다. 최소 필드: 포맷 ID와 별칭, producer와 검증된 버전 범위, bounded probe detector, 제공 measurement kind, capability 집합(§4.1의 stack·self/inclusive·시간 선택·process/thread 분리·원본 위치·비교), adapter 버전, fixture manifest 참조, 지원 상태(지원/제한 지원/외부 변환 필요/미지원).
2. 등록은 단일 레지스트리로 통일한다. `ingestion.FormatRegistry`를 확장해 실제 dispatch에 쓰고, 스레드 덤프 `Plugin`도 같은 descriptor를 내도록 맞춘다. `switch` 분기는 레지스트리 조회로 대체한다.
3. `IMPORTER_SUPPORT_MATRIX`와 §15.1 입력 탐색 안내, UI의 기능 활성화는 레지스트리에서 생성한다. 문서와 코드가 따로 관리되면 §2.1의 마지막 행(문서가 실제 지원보다 넓게 약속)과 같은 불일치가 반복된다.
4. §18.1의 근거 6가지를 **등록된 모든 importer에 자동 실행되는 적합성 테스트**로 만든다. fixture가 없는 importer는 `지원`으로 표시할 수 없게 한다.

**완료 기준**: 새 프로파일 포맷 추가가 "패키지 1개 + fixture + 등록 1줄"이고, 기존 파서·서비스·CLI·프런트엔드 파일을 수정하지 않는다. 이 기준을 설계의 수용 기준(§18)에 넣는다.

### F-02 공통 프로파일 모델로의 이행 경로 없음

**근거**

- 설계 §4.3은 `ProfileSet/Profile/Frame/Sample/EventSpan/Quality`를 제안한다. Frame은 module·build ID·inline 구분을, Sample은 정수가 아닌 weight·metric ID·선택적 timestamp를 갖는다.
- 현재 profiler core의 중심 자료구조는 접힌 문자열을 키로 하는 `map[string]int`다. `profilerservice.go:298` `loadStacks(...) (map[string]int, error)`, `:293` `AnalyzeProfilerDiff(baselineStacks, targetStacks, ...)`, `internal/profiler/pprof.go:48` `ExportToPprof(stacks map[string]int, ...)`가 모두 이 형태다. 문자열 키는 같은 이름의 다른 module을 구분하지 못하고(§4.5·§6.2 요구), `int`는 소수 weight를 담지 못하며, 한 map은 여러 sample type·thread·시간을 담지 못한다.
- `internal/parsers/profile/parser.go:38`의 `Sample.Value`는 `int64`이고 `:751-754` `normalizeSample`은 음수와 0을 1로 바꾼다. 설계 §2.1의 첫 항목이 지적한 동작이 코드에서 확인된다.
- 프로파일 경로가 둘이다. `internal/profiler`는 자체 typed `AnalysisResult`로 `profiler_collapsed`/`profiler_diff`/`profiler_jennifer`를 내고, `internal/parsers/profile` + `internal/analyzers/profile`은 공통 envelope로 `profile_evidence`를 낸다. 설계 §16.5는 두 경로를 한 행에 묶어 "공통화"라고만 적었고, §16.2는 "가능한 한 기존 `profile_evidence`와 profiler 계열을 사용"이라고 해 어느 쪽이 기준인지 정하지 않았다.

**영향**: §19가 P0로 둔 "측정 단위·가중치·프로파일 품질 계약"은 사실상 core 자료구조 교체다. 경로를 정하지 않고 시작하면 세 번째 모델이 생기거나, 드릴다운·diff·flamegraph가 경로마다 다른 수치를 낼 수 있다(§6.2가 금지한 상황).

**권장 조치**

1. 기준 파이프라인을 하나로 정하고, 새 importer가 내는 결과 타입을 명시한다.
2. 새 모델 → 기존 `map[string]int`로 가는 **명시적 projection**(선택한 metric 하나, 정수화 규칙, 손실 표시)을 정의해 기존 드릴다운·diff·export를 단계적으로 옮긴다.
3. 기존 golden 테스트(`internal/profiler/golden_test.go`)를 이행 중 불변 조건으로 지정하고, 수치가 의도적으로 바뀌는 경우(F-03의 의미 변경)는 목록으로 관리한다.
4. §19의 P0 행에 "core 자료구조 이행"을 선행 조건으로 적는다.

### F-03 AnalysisResult 정합성: 두 envelope와 버전 정책

**근거**

- §16.1은 `metadata.measurement/capture/quality/provenance/capabilities`를 additive로 추가하고 "구형 consumer는 모르는 항목 무시"라고 한다. 공통 envelope(`internal/models/analysis_result.go:88-99`)는 `Extra`를 같은 레벨로 펼치므로 가능하다. 그러나 profiler envelope의 `Metadata`(`internal/profiler/types.go:137-150`)는 `Extra`가 없는 고정 struct다. 여기서는 필드 추가가 struct 변경이고, Go에서 이 타입으로 읽었다 다시 쓰는 경로가 있으면 모르는 키가 사라진다. §16.1이 "round-trip도 함께 확인"이라고 한 줄 적었지만 해결 방식은 없다.
- `schema_version`이 분석기마다 다르다. 공통 `New()`는 `"0.1.0"`(`analysis_result.go:121`), `gclog`·`traceimport`·`profiler`는 `"0.2.0"`, `httpcapture`는 `"1.0.0"`, `metrics`·`lockcontention`·`brokerlog`는 `"0.1.0"`이다. `docs/ko/DATA_MODEL.md:1370-1374`는 0.2.0으로 올랐다고 적는다. 설계 §16.1은 "필수 필드 변경·의미 변경은 schema version과 migration 필요"라고 하지만 어느 버전을 누가 올리는지, 확장 블록 자체의 버전은 무엇인지 정하지 않았다.
- §2.1의 정확성 수정은 **같은 키의 값이 달라지는 의미 변경**이다. Speedscope weight 정수화 제거와 evented duration 계산은 `total_samples`·stack value의 의미를 바꾼다. 설계 자신의 규칙으로는 버전 상승과 저장 결과 처리가 필요한데, 각 수정이 additive인지 의미 변경인지 분류하지 않았다.
- §3.3은 모든 수치에 `value/unit/measurement_kind/scope/aggregation/availability/evidence_ref`를 요구한다. 기존 `summary`는 단위 없는 스칼라 map이다. summary 키와 측정 의미를 잇는 규칙, 미수집을 `null`로 낼 때 숫자를 기대하는 기존 consumer의 동작이 정의되지 않았다.

**권장 조치**

1. `metadata.measurement`를 **키 주소 방식의 descriptor map**으로 규정한다. 예: `summary.<key>`·`series.<key>` → `{unit, kind, scope, aggregation, availability}`. 기존 스칼라는 그대로 두고 의미만 옆에 붙이면 additive가 유지된다.
2. 확장 블록마다 자체 버전을 둔다(예: `metadata.extensions = {"measurement": "1", "quality": "1"}`). envelope `schema_version`과 분석기별 버전의 역할을 `DATA_MODEL.md`에 한 표로 정리한다.
3. 두 envelope 모두에 확장 필드를 넣고 unmarshal→marshal round-trip 테스트를 계약으로 둔다. profiler `Metadata`에 미지 키 보존 수단을 추가하거나 공통 envelope로 수렴하는 방침을 정한다(F-02와 함께 결정).
4. §2.1의 6개 항목을 additive/의미 변경으로 분류하고, 의미 변경 항목에는 버전 상승, 저장된 결과의 표시 방식(과거 스냅샷 표시 또는 재계산), 비교 결과에서 신·구 버전 혼용 차단을 적는다.
5. `null` 도입 대상 키 목록과 프런트엔드·exporter·diff의 처리 규칙을 계약에 포함한다.

### F-04 air-gapped 배포·갱신 모델 부재

**근거**

- 분석 경로의 오프라인 원칙은 충분하다(§1.2, §4.5 symbol server 미사용, §15.2 AI 선택 사항, §17.1). 프런트엔드 소스에서도 외부 URL 로드는 찾지 못했다(테스트용 `external.invalid`만 존재).
- 그러나 배포와 갱신은 §17.2 마지막 문장 "배포 시 검증된 자산 버전/hash를 앱에 포함해 인터넷이나 워크스페이스 상대 경로 없이 동작시킨다" 하나뿐이다. 설계가 가장 자주 바뀐다고 스스로 밝힌 것이 데이터다: vLLM metric 이름(§12.3), DCGM 이름 alias(§11.2), Nsight SQLite schema(§11.4), OTel GenAI 속성(§12.6), 진단 임계값(§12.5). 이들이 바이너리에만 들어가면 폐쇄망 현장에서는 mapping 한 줄을 고치려 해도 앱 전체를 재배포해야 한다.
- 진단 규칙과 mapping의 원본은 워크스페이스 형제 저장소 `../projects-assets`다(§17.2 표의 링크가 모두 `../../../projects-assets/...`). 현재 코드에서 이 경로는 테스트와 스크립트에서만 쓰이고, `internal/analyzers/profile/fixture_manifest_test.go:147-150`처럼 저장소가 옆에 없으면 테스트가 **skip**된다. 폐쇄망 빌드나 단독 checkout에서는 §18.1의 지원 근거가 조용히 실행되지 않는다.
- §4.6·§5.1·§11.4는 `perf`, `nsys`, `dotnet-trace`, JDK 도구 등 외부 변환을 전제로 한다. 변환을 수집 서버에서 하는지 분석 PC에서 하는지, 분석 PC에 도구가 없을 때의 안내가 없다.
- 새 의존성의 배포 영향이 빠져 있다. §16.4는 SQLite를 인덱스 후보로 두지만 `go.mod`에는 SQLite 드라이버가 없다. cgo 여부가 Windows/macOS 교차 빌드와 서명에 영향을 준다. §14.4의 "한국어 글꼴 fallback"은 폰트 번들 여부에 따라 PC마다 그림이 달라진다. Windows는 WebView2 런타임이 필요하다(`docs/ko/NATIVE_APP.md:19`).
- §18의 수용 기준에 "네트워크 차단 상태에서 전체 회귀"가 없다.

**권장 조치**

1. **오프라인 지식 팩**을 설계한다. metric mapping·alias·규칙·임계값·수집 안내를 버전·호환 범위·hash가 있는 데이터 전용 묶음으로 만들고, 파일로 반입해 교체할 수 있게 한다. 기본값은 앱에 내장하고, 팩은 실행 코드를 포함하지 않으며, 결과의 `metadata.provenance`에 팩 버전을 남긴다. 무결성 검증 방식(서명 또는 hash 대조)을 정한다.
2. 외부 변환 계약(§4.6)에 **변환 위치**를 추가한다. 기본은 "수집 서버에서 변환까지 마치고 반입"이며, 도구별 오프라인 수집·변환 절차를 앱 내장 도움말로 제공한다. 외부 문서 링크만으로는 폐쇄망에서 읽을 수 없다.
3. 패키징 항목을 `PACKAGING_PLAN`과 연결해 명시한다: SQLite 드라이버 선택 기준, 폰트 번들과 라이선스, WebView2 오프라인 설치본, 설치 파일 무결성 확인 절차, 자동 업데이트 확인 없음.
4. 공유 fixture가 없을 때 테스트가 skip되는 경로를 릴리스 게이트에서는 실패로 바꾸거나, 릴리스에 필요한 fixture를 vendoring한다.
5. §18에 수용 기준을 추가한다: 네트워크를 차단한 상태에서 반입·분석·드릴다운·이미지 복사·AI 비활성 경로가 전부 통과하고, 외부 연결 시도가 0건이다.

## 4 P2 상세

### F-05 새 결과 타입 추가의 접점이 많음

**근거**: `cmd/archscope-app/engineservice.go`에는 분석기마다 `Analyze*` 메서드와 요청 타입이 따로 있다(`:423-719`에 24개). 프런트엔드에는 결과 타입 문자열로 분기하는 소비자가 있다: `state/sloGoldenSignals.ts:742-775`, `state/incidentTimeline.ts:266-269, 604-618`. 설계 §16.2는 결과 타입 4개를 새로 제안하고 `stitched_evidence`·`incident_timeline`을 "이 타입들을 입력으로 받는 방향으로 확장"한다고 했다. 타입마다 이 분기를 전부 고쳐야 한다. §16.1이 요구한 generic renderer fallback은 프런트엔드 소스에서 찾지 못했다.

**권장 조치**: (1) 결과가 스스로 제공 기능을 선언하게 한다. 예: `metadata.capabilities`에 timeline lane·SLI·비교 가능 여부를 넣고, 타임라인·SLO·상관 분석은 타입 이름이 아닌 capability로 입력을 찾는다. (2) generic renderer fallback을 새 타입보다 먼저 구현하도록 §19에 선행 조건으로 적는다. (3) "새 분석 타입 추가 절차"를 `PARSER_DESIGN.md`에 체크리스트로 두고 접점 수를 수용 기준으로 관리한다.

### F-06 버전 축과 호환 정책

**근거**: 설계는 parser/adapter/rule 버전(§3.2, §16.4), chart template 버전(§14.5), fixture hash(§21)를 언급한다. 그러나 각 구성요소가 버전을 어디에 선언하는지, 버전이 다를 때 무엇을 하는지가 없다. 예: adapter v1 결과를 v2 앱에서 열 때, 구버전 앱이 신버전 사건 묶음을 열 때, SQLite 인덱스 schema가 바뀔 때.

**권장 조치**: 버전 축 표를 추가한다. 대상: envelope schema, 확장 블록, importer/adapter, 지식 팩, 로컬 인덱스 schema, 사건 manifest, 휴대용 묶음 포맷, `VisualArtifactSpec`, chart template. 축마다 올리는 주체, 불일치 시 동작(stale 표시·재계산·읽기 전용 스냅샷·거부), 상위 버전 파일을 만났을 때의 동작을 적는다. 인덱스는 파생물이므로 migration 없이 재생성하는 것으로 정하면 단순해진다.

### F-07 누락된 시나리오

| 누락 | 근거 | 권장 |
|---|---|---|
| 수집 manifest 포맷 | §4.2·§5.1·§10.1·§11.1이 모두 "manifest에 수집 조건을 남긴다"고 하지만 파일에 없으면 사용자 입력이다. 수집 측이 만드는 표준 형식이 없다 | 수집 스크립트가 함께 내는 sidecar manifest(host, timezone, 도구 버전, 명령 옵션, 수집 시각) 형식을 정의한다. 폐쇄망에서는 수집자와 분석자가 다른 경우가 많다 |
| 사건 작업 공간 수명주기 | §3.2는 생성과 stale만 다룬다. 삭제, 보존 기간, 디스크 정리, cache에 남은 민감 자료 파기, 사건 보관·복원이 없다 | 수명주기와 저장 위치, 정리 명령, 파기 범위를 추가한다 |
| 일괄 반입 | §9.1은 access log의 회전·gzip만 언급한다. 여러 호스트의 자료가 든 tar/zip을 사건에 한 번에 넣는 흐름이 없다 | 아카이브 반입, 호스트·프로세스 자동 그룹화, §17.1의 path traversal 검사를 반입 단계 계약으로 통합한다 |
| 레거시 인코딩 | §9.1은 "인코딩"이라고만 적었다. 스레드 덤프는 UTF-16/BOM을 처리한다 | 국내 현장에서 흔한 CP949/EUC-KR 로그를 fixture와 회귀 표(§18.2)에 명시한다 |
| baseline 관리 | §4.5는 두 프로파일 비교를 정의하지만 정상 시점 기준 자료를 보관·재사용하는 방법이 없다 | 사건 간 재사용 가능한 baseline 등록을 검토한다(우선순위 낮음) |

### F-08 선언형 확장 경로

**근거**: 설계에는 코드 없이 확장하는 요소가 흩어져 있다: access 필드 매핑 preset(§9.1), 알 수 없는 로그의 매핑 도우미(§15.1), metric alias와 canonical 매핑(§11.2, §12.3). 하나의 확장 계층으로 정리되지 않았고 preset의 파일 반출·반입도 없다.

**권장 조치**: 확장 수단을 3계층으로 명시한다. ① 선언형(매핑·alias·규칙 데이터 파일, 사용자와 지식 팩이 추가, 재빌드 불필요) ② 컴파일형 importer(F-01의 레지스트리) ③ 외부 변환(§4.6). 새 입력 요청이 오면 ①→③→② 순으로 검토한다는 기준을 둔다. 지속적으로 늘어날 로그·메트릭 종류의 대부분은 ①로 흡수할 수 있고, 폐쇄망에서도 파일 하나로 전달된다. 선언형 정의도 §15.1의 제한된 문법·실행 예산과 §18.1의 fixture 검증을 그대로 적용한다.

### F-09 우선순위 표의 누락

**근거**: §19 표에 행이 없는 기반 항목: 사건 작업 공간(§3.2), clock·identity 계약(§13.2, §11.3 — 여러 행의 선행 조건으로만 등장), 입력 탐색 도우미(§15.1), 비식별·보안 경로(§17.1), TA 자료(§9.3). P1이 9행인데 서로의 순서는 본문 한 단락에만 있다.

**권장 조치**: 위 항목과 F-01의 레지스트리 계약, F-02의 core 이행, F-05의 generic renderer를 행으로 추가하고 P0에 레지스트리 계약과 envelope 확장을 넣는다. 또한 이 표의 P0/P1/P2는 리뷰 등급과 표기가 같아 `work_status.md` 반영 시 혼동될 수 있으니 구분되는 표기를 쓴다.

### F-10 시각 자료 렌더링과 exporter 경계

**근거**: §14.3은 "화면 캡처가 아닌 별도 렌더링"이라고 하지만 렌더링이 어디서 실행되는지 없다. 현재 `cmd/archscope-app/chart_export.go`의 `SaveChartArtifact`는 WebView가 이미 그린 결과를 받아 저장만 한다(`ChartArtifactRequest` 주석: "already-rendered chart, not an AnalysisResult"). 이 구조에서는 §16.3의 `RenderVisual`이 CLI에서 동작할 수 없고, §14.5의 "사건을 다시 열어 동일 그림 재생성"도 GUI가 있어야만 가능하다. 또한 기존 exporter(`ExportJSON/HTML/PPTX/CSV`, `engineservice.go:889-922`)는 `AnalysisResult`를 소비하는데 새 경로는 `AnalysisView`를 소비한다. HTML 보고서와 Report Pack이 `VisualArtifact`를 포함하는지 언급이 없다.

**권장 조치**: 렌더러 위치를 결정하고(WebView 전용인지, headless 렌더 경로를 둘지) CLI에서 지원하는 범위를 명시한다. 기존 exporter와 `VisualArtifact` 경로의 관계, chart template 버전이 올라갔을 때 과거 spec의 재생성 규칙(F-06)을 `REPORT_EXPORT_DESIGN`에 반영한다.

## 5 P3

- **F-11 영문 문서 부재**: `docs/en/LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md`가 없다. 프로젝트 지침은 영문·국문 문서를 언어별 파일로 유지하도록 한다. 설계 확정 후 영문본을 추가하고 양쪽 README 색인에 등록한다.
- **F-12 결과 타입 경계**: GPU telemetry는 장치 identity가 붙은 metric 시계열이다. `gpu_metrics`를 독립 저장 타입으로 두면 §16.2가 스스로 경계한 중복 저장이 된다. `metrics_timeseries` 위의 semantic projection으로 두고, "새 타입은 선택 의미나 전용 화면이 다를 때만"이라는 판단 기준을 적는다.

## 6 기능 보완 제안

이 절은 결함 지적이 아니라 **범위에 대한 제안**이다. 3~5절의 finding과 달리 채택 여부는 처리자와 사용자가 다시 판단한다. 판단에 쓸 전제를 먼저 밝힌다.

- 설계는 이미 넓다(프로파일 13개 계열, GPU, vLLM, RAG). 누락보다 범위 과다가 더 큰 위험이므로 추가는 최소로 제안한다.
- 6.1과 6.3의 우선순위는 국내 폐쇄망·JVM 중심 현장을 가정하고 썼다. 이후 사용자가 **JVM 외 언어와 AI 서비스 수요가 늘고 있다**고 밝혀 이 가정은 일부 수정되었다. 그 수요에 해당하는 항목은 6.2에 따로 두었고, 6.1·6.3의 순위는 6.2를 반영해 다시 매겨야 한다.
- "코드 확인"은 `apps/engine-native`의 비-테스트 Go 소스에 대한 키워드 `grep` 결과다. 다른 이름으로 구현된 기능은 놓쳤을 수 있다.
- 채택하더라도 F-01(등록 계약)과 F-08(선언형 계층)이 먼저 정리되어야 추가 비용이 낮다. 구조 보완 전에 입력만 늘리지 않는다.

### 6.1 추가를 권장하는 항목

| ID | 항목 | 설계의 현재 상태 | 코드 확인 | 제안 범위 |
|---|---|---|---|---|
| G-01 | 부하 테스트 결과 입력 | 비교마다 "동일 workload 조건"을 요구(§4.2-6, §4.5, §9.1, §12.3)하지만 그 조건을 증명할 입력이 없음 | JMeter·nGrinder·k6·Gatling 관련 코드 없음 | JMeter JTL, nGrinder, k6, Gatling 결과와 LLM용 GenAI-Perf·llmperf 출력을 workload 기준 자료로 반입. 부하 구간·동시 사용자·요청 구성·클라이언트 측 응답시간과 TTFT를 비교 조건으로 사용 |
| G-02 | `top -H`/`ps -L`과 스레드 덤프 결합 | §7.1 JVM 행의 필요한 자료에 "process CPU"만 있음 | jstack 파서가 `nid=0x…`를 형식 감지에 사용(`javajstack/parser.go:92`). `top -H`·`ps -L` 입력은 찾지 못함 | thread별 CPU 스냅샷을 반입해 nid ↔ TID로 연결하고 "CPU를 쓰는 스레드의 stack"을 제시. RUNNABLE을 CPU 사용으로 단정하지 않는다는 §7.1 원칙의 실제 검증 수단 |
| G-03 | 오프라인 수집 키트 | 수집은 안내만 제공(§3.1, §5.3)하고 수집 조건이 없으면 사용자 입력(§4.2-2) | 해당 없음 | 사용자가 직접 실행하는 수집 스크립트와 sidecar manifest(F-07 참조) 생성. 자동 attach·원격 실행 금지 원칙(§1.2, §5.3)을 유지. 폐쇄망은 재수집이 어려워 첫 수집의 완결성이 중요 |
| G-04 | Oracle AWR/ASH, Tibero TPR 리포트 | §9.2가 "사용자 수요와 파일 제공 조건을 확인한 뒤"로 미룸 | 관련 코드 없음 | DBA가 건네는 텍스트·HTML 리포트를 파일로 반입. Top SQL·wait event·load profile을 SQL fingerprint와 사건 시간축에 연결. DB 접속은 하지 않음. 라이선스·권한 가정은 §9.2 원칙을 따름 |
| G-05 | 사건 전체 횡단 검색 | 드릴다운은 자료별로 정의(§6.1). 여러 자료에서 같은 식별자를 찾는 기능은 없음 | 해당 없음 | trace ID·request ID·시간 범위·문자열로 사건의 모든 자료를 검색하고 결과를 원본 위치와 연결. §13.1의 명시적 연결을 사용자가 직접 확인하는 수단. §16.4의 로컬 인덱스를 재사용 |
| G-06 | JVM `hs_err_pid` 크래시 로그 | 설계에 없음 | 관련 코드 없음 | 크래시 원인 프레임, 신호, VM 상태, 메모리 맵 요약, 당시 thread와 GC 상태를 구조화. native OOM·컨테이너 OOMKill 분류(§8.2)와 연결 |
| G-07 | 바이너리 `.jfr` 직접 읽기 | §4.1은 "JFR 및 event JSON"으로 표기 | 바이너리는 JDK `jfr print --json`을 호출해 변환(`parsers/jfr/recording.go`). 분석 PC에 JDK가 없으면 열 수 없음 | 순수 Go decoder 도입을 검토하거나, 최소한 §4.6 외부 변환 계약에 JFR을 명시하고 JDK 부재 시 안내를 제공. F-04의 변환 위치 결정과 함께 판단 |

### 6.2 사용자가 확인한 수요: AI 서비스와 다언어

리뷰 도중 사용자가 직접 밝힌 요구다. ① JVM 외 언어를 쓰는 곳이 늘고 있다. ② AI 서비스가 늘면서 LangSmith·Langfuse 등에서 나오는 로그와 trace를 지원해야 한다. ③ GPU와 LLM 서빙 솔루션 분석이 필요하다. 이 항목들은 "수요 확인" 조건이 이미 충족되었으므로 6.3의 보류 항목과 다르게 다룬다.

설계의 현재 범위를 먼저 구분한다. GPU는 §11에서 다루지만 NVIDIA 자료에 한정한다(§11.5). LLM 서빙은 §12에서 다루지만 metric adapter가 vLLM 이름 기준이다(§12.3). LLM 애플리케이션 계층(체인·agent·RAG trace)은 §12.6의 세 단락뿐이고 §19에서 P2("분산 추론·RAG·Agent 심화")다. 따라서 부족한 부분은 **애플리케이션 계층 trace, vLLM 이외의 서빙 엔진, 일부 운영 시나리오**다.

| ID | 항목 | 설계의 현재 상태 | 코드 확인 | 제안 범위 |
|---|---|---|---|---|
| G-14 | LLM 애플리케이션 trace 반입 (Langfuse, LangSmith, OpenTelemetry GenAI, OpenInference 계열) | §12.6에 "GenAI trace adapter는 버전과 schema URL을 보존한다"는 한 단락. 우선순위 P2 | Langfuse·LangSmith·GenAI 속성 관련 코드 없음. trace import는 OTLP JSON·Zipkin·Jaeger·Elastic APM·SkyWalking을 지원(`parsers/traceimport/parser.go:24-29`) | 제품별 분석기를 따로 만들지 않고 **공통 LLM 실행 trace 의미 계층**을 둔다. 단계 종류(LLM 호출, retriever, tool, chain/agent, embedding, reranker), 모델, 입력·출력 token, 시작·첫 토큰·종료 시각, 오류, 재시도를 정규화하고 제품별 adapter가 이 계층으로 변환. OTLP로 나온 자료는 기존 OTLP 경로에 semantic adapter만 추가. 결과는 기존 trace·timeline 화면과 §12의 서빙 분석에 연결 |
| G-15 | 서빙 엔진 adapter 계약의 일반화 (G-11을 대체) | §12.3이 vLLM metric 이름으로 작성됨. §16.2의 `inference_serving`은 "vLLM 등"이라고만 표기 | vLLM을 포함해 서빙 엔진 metric 관련 코드 없음 | §12를 엔진 중립 구조로 재구성한다. 공통 신호(대기·실행 중 요청, TTFT, 토큰 간격, 요청 지연, 단계별 시간, token 처리량, cache)마다 엔진이 제공하는 **통계 형태**를 capability로 선언: histogram, 누적 counter, 요청별 로그 필드, 미제공. 첫 adapter는 vLLM(사용자 확인). 이후 후보는 TGI, Triton(TensorRT-LLM 포함), NIM, SGLang, llama.cpp, Ollama와 그 앞단의 KServe·Ray Serve이며 실제 자료가 들어올 때 추가. 엔진별 이름·단위 mapping은 F-04의 지식 팩 데이터로 관리 |
| G-16 | 모델 수명주기와 cold start 시나리오 | §12.5 조사 시나리오 표에 없음. §12.2에 startup log 입력만 있음 | 해당 없음 | 모델 로드 시간, 첫 요청 warmup, 모델 교체·메모리 회수, scale-from-zero, 재시작 후 복구 구간을 정상 구간과 분리. 이 구간의 요청을 일반 지연 분포에 섞지 않는다(§12.4의 집단 분리 원칙 확장) |
| G-17 | LLM gateway 자료 | 설계에 없음. §12.6이 비용을 "사용자 단가 × token"으로만 언급 | LiteLLM 등 관련 코드 없음 | proxy·gateway 로그에서 모델 라우팅, rate limit 응답, 재시도, fallback 모델 전환, 요청별 token을 반입. 재시도 attempt와 논리 요청의 분리(§9.1)를 그대로 적용 |
| G-18 | Python AI 서비스 런타임의 깊이와 다언어 대응 | §7.1 Python 행이 py-spy dump·record 연결까지. §4.1의 Python 기타 도구는 "후속 adapter" | Python 스레드 덤프 plugin 있음(`threaddump/plugins/pythondump`). memray·tracemalloc 없음 | Python을 JVM과 같은 깊이의 런타임으로 올린다: asyncio task 상태 자료, ASGI/WSGI worker 로그, 작업 큐 worker, 메모리 프로파일(memray, tracemalloc). Node/TypeScript·Rust·Go 기반 AI 구성요소는 언어별 분석기를 늘리지 않고 V8·pprof·perf 공통 형식으로 흡수(F-01 레지스트리 전제) |
| G-19 | GPU 입력 범위 확장 | §11.5가 NVIDIA 한정. 타 vendor는 "fixture와 수요가 확보되면" | DCGM·NVIDIA 관련 코드 없음. §11 전체가 미구현 상태 | NVIDIA 범위 안에서 process 단위 관측(`nvidia-smi pmon`/`dmon`), CUDA OOM 메시지와 프레임워크 메모리 요약, Kubernetes GPU 할당 자료를 입력 후보에 추가. Apple Silicon GPU는 G-20에서 별도로 다룬다 |

**G-14에서 지켜야 할 해석 규칙.** 설계의 기존 원칙을 이 자료에 적용한 것이다.

- prompt·completion·tool 인자 본문은 가장 민감한 자료다. 기본 반입 대상에서 제외하고 시각·token·모델·오류·구조만 가져온다(§15.2, §17.1). 본문 반입은 사용자의 명시적 선택과 비식별 정책을 거친다.
- chain·agent 단계의 시간은 하위 단계를 포함한다. 중첩된 시간을 더해 전체 시간으로 표시하지 않는다(§12.6, 2026-11-10 점검의 critical path 문제와 같은 유형).
- 애플리케이션이 관측한 첫 토큰 시각은 client 관점이다. 서빙 엔진의 TTFT와 같은 값으로 합치지 않고 §12.4의 client/server 구분에 대응시킨다.
- token 수와 비용은 관측 플랫폼이 계산한 값일 수 있다. 출처를 보존하고 ArchScope가 다시 계산한 값과 구분한다.
- 평가 점수와 사용자 피드백은 품질 자료다. label로만 보존하고 성능 원인 판정에 섞지 않는다(§12.6).
- SDK는 보통 비동기로 묶어서 전송한다. 시각은 애플리케이션 clock 기준이며 sampling·전송 누락을 coverage에 반영한다(§13.2).
- 서빙 metric·GPU 자료와의 연결은 전파된 request/trace ID가 있을 때만 명시적 연결로 표시한다(§13.1).
- 폐쇄망 원칙상 관측 플랫폼의 API를 호출하지 않고 **export 파일**을 반입한다. 자체 호스팅 여부와 export 형식은 제품마다 다르므로 수집 측 절차를 G-03의 수집 키트와 F-04의 오프라인 안내에 포함한다.

**기존 finding에 미치는 영향**

- F-04·F-08: 서빙 엔진과 관측 플랫폼의 이름·단위·필드 mapping은 가장 빨리 바뀌는 데이터다. 지식 팩과 선언형 계층의 첫 적용 대상으로 삼으면 엔진 추가에 재빌드가 필요 없다.
- F-09: §19의 "분산 추론·RAG·Agent 심화"(P2) 행에서 LLM 애플리케이션 trace 반입을 분리해 상향하고, "vLLM 기본 성능 분석" 행을 "서빙 엔진 공통 계약 + 첫 엔진"으로 바꾼다.
- F-12: `inference_serving` 타입은 엔진 중립으로 정의한다. 애플리케이션 trace는 새 타입보다 기존 trace 결과의 semantic 확장이 우선이다.

**사용자 답변 (2026-11-11)**

| 질문 | 답변 | 범위에 미치는 영향 |
|---|---|---|
| 서빙 엔진 | vLLM 위주 | G-15의 첫 adapter는 vLLM으로 확정. 다른 엔진 adapter는 지금 만들지 않는다. 단 계약은 엔진 중립으로 두어야 한다(아래 MacBook 항목 참조) |
| GPU | NVIDIA 위주. **MacBook의 GPU 분석도 필요** | G-19는 NVIDIA 범위로 유지. Apple Silicon GPU를 G-20으로 추가. §11.5의 "다른 accelerator는 수요가 확보되면"에서 Apple GPU는 수요가 확인된 것으로 바뀐다 |
| 언어 | Python, Rust, Node 등 | G-18은 Python을 1순위로 확정. Rust를 G-21로 추가. Node는 설계가 이미 다루는 범위(§4.1, §7.1, §8.1)의 구현 순서를 앞당기는 것으로 충분 |
| LangSmith·Langfuse 자료 형태 | 답변 없음 | G-14의 adapter 순서를 정할 수 없다. 아래 남은 질문 1 |

| ID | 항목 | 설계의 현재 상태 | 코드 확인 | 제안 범위 |
|---|---|---|---|---|
| G-20 | Apple Silicon(MacBook) GPU 분석 | 설계에 없음. macOS는 분석 PC로만 언급(§1.2, §18.3) | powermetrics·Metal·MLX 관련 코드 없음 | NVIDIA와 **별도의 vendor adapter**로 둔다. 입력 후보는 macOS 기본 도구의 전력·GPU 활동 출력, 프레임워크(Metal 백엔드)의 메모리 관측, 엔진 로그다. 장치 모델에 반영할 차이: ① 통합 메모리라 전용 GPU 메모리가 없고 시스템 메모리 압력·swap이 함께 봐야 할 지표다 ② GPU UUID·MIG·Xid·DCGM에 해당하는 개념이 없어 §11.3의 identity 규칙이 그대로 적용되지 않는다 ③ 노트북은 전원 상태·저전력 모드·발열 제한이 성능을 직접 바꾸므로 수집 조건(§3.3 capture)에 포함한다. NVIDIA metric을 이름만 바꿔 적용하지 않는다는 §11.5 원칙을 유지 |
| G-21 | Rust 런타임 자료 | §4.1 Native 행에 "C/C++/Rust, 공통 형식 우선"으로만 표기 | 프레임 이름에 `rust` 또는 `::`가 있으면 Rust로 분류(`parsers/profile/parser.go:947-948`). `::`는 C++ 이름에도 있어 오분류 가능. symbol demangle 코드는 찾지 못함 | ① symbol demangle과 빌드별 hash 접미사 정규화. 이것이 없으면 §4.5의 버전 간 함수 비교 키가 빌드마다 달라진다 ② 런타임 분류를 이름 추측이 아닌 producer·binary 정보 기준으로 변경 ③ async 실행은 stack에 poll 호출로만 나타나므로, 논리적 task 관계는 별도 자료가 있을 때만 표시(§7.1의 "async task와 OS thread 구분" 원칙) ④ panic·backtrace 로그 구조화. 프로파일 자체는 perf·pprof·collapsed 공통 경로로 받는다 |

**답변에서 생기는 설계 쟁점**

- **MacBook에서 도는 서빙 엔진이 vLLM이 아닐 수 있다.** vLLM의 Apple GPU 지원 여부는 이 리뷰에서 확인하지 않았다. 지원하지 않는 버전이라면 MacBook의 추론은 다른 엔진(Metal 백엔드를 쓰는 로컬 엔진)으로 돌고, 그 엔진의 로그·metric이 입력이 된다. "vLLM 위주"와 "MacBook GPU"를 함께 만족하려면 G-15의 엔진 중립 계약이 선택이 아니라 필수다.
- **vendor 중립 장치 모델을 두 번째 vendor로 조기에 검증할 수 있다.** §11.5는 내부 모델을 vendor 중립으로 둔다고 했지만 검증 대상이 NVIDIA뿐이었다. Apple GPU를 초기 fixture에 넣으면 identity·메모리·오류 모델이 NVIDIA 전용으로 굳는 것을 막는다.
- **MacBook은 수집 대상이면서 분석 PC다.** 설계는 수집 환경과 분석 PC를 구분하고(§3.2) 실시간 수집기를 만들지 않는다(§19). 같은 기기에서 수집할 때도 이 원칙을 유지할지 정해야 한다. 권장은 유지다: 관리자 권한이 필요한 수집은 사용자가 직접 실행하는 스크립트(G-03)로 하고 ArchScope는 결과 파일만 읽는다.
- **macOS native 프로파일 입력.** MacBook 분석이 범위에 들어오면 §4.1의 "Instruments export 후보"를 다시 평가한다. macOS 기본 도구의 텍스트 call tree 출력은 관련 코드를 찾지 못했고, 바이너리 trace보다 반입 비용이 낮은 후보다.

**남은 질문**

1. LangSmith·Langfuse 자료를 어떤 형태(UI export, 일괄 export, OTLP)로 받을 수 있는가. 자체 호스팅인가.
2. MacBook에서 실제로 돌리는 추론 엔진과 프레임워크는 무엇인가. G-20의 입력과 G-15의 두 번째 adapter가 여기서 정해진다.
3. vLLM의 대상 버전 범위는 무엇인가. metric 이름과 노출 조건이 버전마다 다르다(§12.3).

### 6.3 수요가 확인되면 추가할 항목

G-11(다른 추론 엔진)은 사용자 확인에 따라 6.2의 G-15로 옮겼다.

| ID | 항목 | 비고 |
|---|---|---|
| G-08 | DB 통계 뷰 스냅샷 비교 | `pg_stat_statements`, `performance_schema`, InnoDB status의 deadlock 정보. 코드에서 찾지 못함. §10의 counter 증가분 계산을 재사용할 수 있음 |
| G-09 | Windows 서버 자료 | IIS access log는 이미 지원(`cmd_accesslog.go`). PerfMon CSV와 이벤트 로그는 없음. §9.3이 Linux 도구만 나열 |
| G-10 | 네트워크 카운터 스냅샷 | `netstat -s`, `ss` 전후 비교로 재전송·reset 확인. pcap 전체 분석은 권하지 않음 |
| G-12 | 국내 APM export | Scouter, Pinpoint, WhaTap. 코드에서 찾지 못함. 기존 Jennifer·trace import 경로와 같은 방식 |
| G-13 | 개인 지식 축적 | 과거 사건의 stack 시그니처 검색, 알려진 이슈 패턴 라이브러리. 사건 작업 공간(§3.2)이 선행. 패턴 데이터는 F-04의 지식 팩과 같은 형식으로 관리 가능 |

### 6.4 추가하지 않기를 권하는 항목

- 힙덤프 엔진, PPTX 고도화, 실시간 수집기, 자동 튜닝: 설계의 제외 결정(§1.2, §8.3, §19)을 유지한다.
- 용량 산정·what-if 시뮬레이션, 통계적 유의성 자동 판정: 근거 없는 결론을 만들기 쉽다. §4.5의 "자동으로 통계적 유의성을 주장하지 않는다"와 같은 방향이다.
- 추가 프로파일 계열: §4.1에서 "후속 adapter"로 둔 Austin·Instruments 등은 실제 자료가 들어올 때까지 더 늦춰도 된다. Python 계열은 G-18에 따라 다시 평가한다.
- 모델 응답 품질 평가, 프롬프트 관리, 평가 데이터셋 관리: LangSmith·Langfuse의 기능이지만 ArchScope의 범위(성능·장애 분석)가 아니다. G-14는 실행 trace의 시간·token·오류만 다룬다.

### 6.5 처리자가 판단할 때 볼 기준

1. 사용자가 실제로 받는 자료인가. fixture로 쓸 비식별 실제 파일을 확보할 수 있는가(§18.1).
2. F-08의 선언형 계층으로 흡수되는가. 그렇다면 코드 추가 없이 데이터 파일로 처리한다(G-08, G-10, G-12 일부가 후보).
3. 기존 계약을 재사용하는가. G-01은 비교 조건, G-02는 스레드 분석, G-05는 로컬 인덱스를 재사용하므로 새 결과 타입이 필요 없을 수 있다.
4. 채택 항목은 §19 우선순위 표에 행으로 추가하고 선행 조건을 적는다.

## 7 유지해야 할 설계 결정

구현 중 범위 압력으로 약해지지 않도록 기록한다.

- 미수집·0·잘림·추정의 구분과 "합계는 전체 입력 기준, top-N은 별도 변환"(§3.3, §6.2).
- 파싱 성공과 capability 활성화의 분리, 파싱 성공률과 의미 보존율의 분리(§4.1, §18.1).
- 상관 연결 강도 4단계와 clock 불확실성 표시(§13.1, §13.2).
- p95 합산 금지, quantile 평균 금지, counter reset 선처리(§10.2, §12.4).
- 범위 제외의 명시: PPTX, 힙덤프 엔진, 자동 튜닝, 실시간 수집기(§19).
- §2.1처럼 확장 전에 기존 정확성 문제를 회귀 fixture로 먼저 고정하는 순서.

## 8 처리자를 위한 권장 순서

1. F-01·F-02·F-03을 하나의 설계 보완 작업으로 묶는다. 레지스트리 계약, core 이행, envelope 확장은 서로의 전제다.
2. F-04의 지식 팩 형식과 F-08의 선언형 계층을 함께 정한다. 같은 데이터 파일 형식을 공유할 수 있다.
3. F-06 버전 축 표를 위 결과로 채운다.
4. F-05·F-09·F-10은 §19 개정에 반영한다.
5. F-07·F-11·F-12는 해당 절 보완으로 처리한다.
6. 6절의 기능 제안(G-01~G-21)은 6.5의 기준으로 채택 여부를 다시 판단한다. 채택하더라도 1~2번의 구조 보완 뒤에 착수한다.
7. 6.2(G-14~G-21)는 사용자가 직접 확인한 수요다. 채택 여부보다 범위와 순서를 정하는 문제다. 사용자 답변은 6.2에 기록했고, 6.2 끝의 남은 질문 3개는 해당 항목 착수 전에 확인한다.

설계 보완이 끝나기 전에는 §19의 "가장 먼저 완결할 작은 구현 범위" 중 **정확성 회귀 fixture 확정**(§2.1)만 선행할 수 있다. 이 작업은 어떤 구조 결정과도 충돌하지 않는다.

## 9 검증하지 않은 것

- 6절의 "코드 확인"은 키워드 검색이다. 6.1·6.3 항목의 현장 수요와 우선순위는 리뷰어의 가정이다. 6.2는 사용자가 방향을 확인했지만 구체적인 엔진·버전·자료 형태는 확인하지 않았다.
- LangSmith·Langfuse의 export 형식과 필드, 서빙 엔진별 metric 이름과 통계 형태는 실제 파일이나 공식 문서로 대조하지 않았다. 6.2는 필요한 의미 계층과 해석 규칙만 제안하며 구체적인 필드 mapping은 fixture 확보 후 정한다(§18.1).
- vLLM의 Apple GPU 지원 여부, macOS 도구의 출력 형식과 필요한 권한, Rust symbol 표기 규칙은 실제 환경에서 확인하지 않았다. G-20·G-21의 입력 후보는 fixture로 확인한 뒤 확정한다.
- 빌드, 테스트, 앱 실행, 성능 측정은 하지 않았다. 코드 근거는 파일과 줄 번호의 정적 확인이다.
- "호출처를 찾지 못했다"는 판단(`FormatRegistry` 사용처, generic renderer, 프런트엔드 외부 URL)은 `grep` 결과이며 동적 등록이나 다른 명명은 배제하지 못한다.
- 설계가 인용한 외부 사양(py-spy, pprof, Speedscope, DCGM, vLLM, Nsight, OTel) 내용의 정확성은 대조하지 않았다.
- §2.1의 6개 항목 중 코드에서 직접 확인한 것은 첫 항목(`normalizeSample`)뿐이다.
- `projects-assets` 저장소의 자산 내용과 `docs/en` 문서는 검토하지 않았다.
