# Chrome DevTools CPU 프로파일 분석 설계 검토

- 작성일: 2026-07-18
- 검토자: Codex
- 대상: `docs/ko/CHROME_DEVTOOLS_CPUPROFILE.md` (commit `1f36a39` 기준)
- 범위: 사용자 수집 흐름, V8 CpuProfile 파싱, 시간/단위 계약, 기존 분석기 재사용,
  Wails UI·Diff 연동, 분류, 대형 파일, 테스트 전략
- 검토 방식: 설계서와 현재 Go/Wails 코드 정적 대조, Chrome DevTools/CDP 및
  Node.js 공식 문서 교차 확인. 구현 변경 없음.

## 총평

방향 자체는 타당하다. V8 CpuProfile을 공통 `profile_evidence`로 정규화하고
플레임그래프·비교·리포트 자산을 재사용하려는 선택, 파서와 UI를 분리하려는
원칙, 시간 정보를 초기 파싱 단계부터 보존하려는 판단은 좋다.

다만 현재 설계대로 구현하면 **CLI에서 파일 하나를 읽는 최소 데모는 가능해도,
문서가 제시한 Chrome 사용자 흐름과 완료 기준은 충족되지 않는다.** 특히 다음
네 항목은 구현 착수 전 반드시 설계를 고쳐야 한다.

1. 현재 Chrome Performance 패널의 저장 산출물은 `.cpuprofile`이 아니라
   Performance trace이며, 최신 Chrome은 gzip 저장이 기본이다.
2. `Sample.Value=microseconds`와 기존 `IntervalMS` 계산을 함께 쓰면 모든 시간
   지표가 이중 환산되어 틀어진다.
3. 공통 profile importer와 현재 `ProfilerAnalyzerPage`/`DiffPage`가 서로 다른
   Wails 서비스 경로를 사용한다.
4. CPU 샘플의 인접 스택만으로 브라우저 Long Task 경계를 복원할 수 없다.

따라서 판정은 **“조건부 승인 — P0 설계 수정 후 구현”**이다.

---

## P0 — 구현 전에 해소해야 할 블로커

### 1. 설계의 1차 포맷이 실제 Chrome Performance 저장 흐름과 다르다

설계서는 `Performance 패널 기록 → Save profile → foo.cpuprofile`을 대표 사용자
흐름으로 두고(`CHROME_DEVTOOLS_CPUPROFILE.md:48,106`), Chrome Trace Event는
Phase 4로 미룬다. 그러나 현재 공식 DevTools 문서는 Performance 패널의 다운로드
기능을 **Save trace**로 정의한다. 최신 저장 파일은 `.json` 또는 `.json.gz`이고,
Chrome 142부터 gzip이 기본이다. DevTools 자체 테스트 자료도 Performance 패널
저장 파일을 `*.json.gz`로 관리하며, top-level event 배열과
`{traceEvents, metadata}` 객체를 모두 허용한다.

반면 `.cpuprofile`은 Node.js `--cpu-prof` 또는 CDP `Profiler.stop`의 직접
산출물로는 명확하지만, 현재 Performance 패널의 일반 저장 산출물이라고 볼 수
없다. 즉 지금의 Phase 순서는 “Chrome DevTools에서 생성한 프로파일을 분석한다”는
제품 목표와 어긋난다.

권장 결정은 둘 중 하나다.

- 제품 목표를 **Chrome Performance 저장 파일 지원**으로 유지한다면 Chrome
  trace JSON/JSON.GZ 어댑터를 Phase 1로 올리고, 그 내부의
  `Profile`/`ProfileChunk`를 V8 CpuProfile 정규화 함수로 전달한다.
- 1차 범위를 `.cpuprofile`로 유지한다면 문서 제목과 수집 안내를
  **V8 CPU Profile(Node.js/CDP) 지원**으로 좁히고, Chrome Performance 사용자는
  아직 지원하지 않는다고 명시한다.

현재 제품 동기와 신규 “브라우저 성능 분석” 메뉴를 고려하면 첫 번째가 더
일관적이다. 이 결정 전에는 전용 메뉴와 수집 안내 문구를 확정하면 안 된다.

공식 근거:

- [Chrome DevTools: Save and share performance traces](https://developer.chrome.com/docs/devtools/performance/save-trace)
- [DevTools trace fixtures README](https://chromium.googlesource.com/devtools/devtools-frontend/+/refs/heads/main/front_end/panels/timeline/fixtures/traces/README.md)
- [Node.js `--cpu-prof` CLI](https://nodejs.org/download/release/v22.17.0/docs/api/cli.html#--cpu-prof)

### 2. `Sample.Value`와 `IntervalMS` 설계가 기존 분석기에서 시간을 이중 계산한다

설계서는 `Sample.Value`에 `timeDeltas`의 마이크로초를 넣고, 동시에 평균 샘플
간격을 역산해 `IntervalMS`에 넣으라고 한다(`:288-291`). 현재 공통 분석기는
`estimated_seconds = sum(Value) * IntervalMS / 1000`으로 계산한다
(`internal/profiler/analyzer.go:310-321`). 따라서 `Value`가 이미 시간인데 평균
간격을 다시 곱하면 시간 지표가 잘못된다.

현재 코드 경로에는 더 직접적인 문제가 있다.

- `internal/analyzers/profile.Build`는 호출 옵션이 없으면 `IntervalMS=100`을
  적용하고(`analyzer.go:31-40`), parser metadata에서 간격을 읽지 않는다.
- CLI의 `--interval-ms` 기본값도 100이므로 자동 역산 경로가 호출되지 않는다.
- 결과 metadata의 `unified_profile_schema.sample_unit`은 항상 `"samples"`로
  고정되어 있다(`analyzer.go:71-86`). parser가 `value_unit=microseconds`를
  기록해도 중첩된 `parser_metadata`에만 들어가 공통 지표 의미를 바꾸지 못한다.

구현 전에 측정 계약을 하나로 고정해야 한다. 가장 작은 변경은 V8 경로에서
`Value=attributed_microseconds`, `IntervalMS=0.001`로 두는 것이지만, 이 경우
`total_samples`, `samples`, `sample_ratio`라는 기존 필드명이 사실상 마이크로초를
뜻하게 되는 계약 부채가 생긴다. 더 안전한 설계는 분석기 내부에
`value_unit/count_unit`을 명시하고 시간 환산을 단일 함수로 모으는 것이다.

어느 방식을 택하든 다음 불변식을 골든 테스트로 먼저 고정해야 한다.

- `sum(timeDeltas)`와 결과의 busy/estimated duration이 허용 오차 내에서 일치한다.
- CLI 기본 옵션과 Wails 기본 옵션이 같은 결과를 낸다.
- Diff 정규화 여부와 무관하게 baseline/target 단위가 동일하다.
- idle 제외 전후에도 recording duration과 active CPU duration이 구분된다.

### 3. 데스크톱 프로파일러와 Diff는 신규 importer를 사용하지 않는다

설계서는 FlameGraph, Drilldown, Diff, Report Export가 변경 없이 동작하고
(`:346-359`), 기존 `profiler` 메뉴도 `.cpuprofile`을 계속 받는다고 한다(`:658`).
현재 코드에서는 성립하지 않는다.

- 신규 공통 importer는 `EngineService.AnalyzeProfileEvidence` →
  `internal/analyzers/profile` 경로다(`engineservice.go:518-527`).
- 현재 `ProfilerAnalyzerPage`는 `ProfilerService.AnalyzeAsync`를 호출하며
  (`ProfilerAnalyzerPage.tsx:774`), 이 서비스는 collapsed/Jennifer/SVG/HTML만
  허용한다(`profilerservice.go:450-461`). 파일 필터와 `detectFormat`에도
  `.cpuprofile`이 없다.
- `DiffPage`도 같은 legacy `ProfilerService.Diff`를 사용하고, 타입과 자동 감지는
  네 포맷만 허용한다(`DiffPage.tsx:33-40`). Go `loadStacks` 역시 같은 네 포맷만
  지원한다(`profilerservice.go:295-322`).

따라서 “CLI import 성공”과 “기존 프로파일러/전용 브라우저 페이지/Diff 성공”은
서로 다른 구현 과제다. 다음 중 하나를 명시적으로 선택해야 한다.

- 데스크톱 프로파일 경로를 `AnalyzeProfileEvidence` 중심으로 통합하고 legacy
  서비스는 Jennifer/SVG/HTML 어댑터로 축소한다.
- 신규 Browser 페이지는 `EngineService`를 사용하되, Diff에도
  `profile.Parsed → collapsed stack` 공통 로더를 별도로 연결한다.

또한 Browser 페이지가 결과를 Analysis Workspace에 등록하고 Export Center가
이를 소비하는 흐름까지 완료 기준에 포함해야 “Report Export 변경 없음”을 검증할
수 있다.

### 4. CPU 샘플만으로 Long Task를 판정하는 Phase 3은 의미론적으로 틀리다

설계서는 인접한 동일 스택을 병합해 시간 구간을 만들고 50ms 초과 구간을 Long
Task finding으로 내려고 한다(`:771-774`). 하지만 V8 CpuProfile의
`samples/timeDeltas`는 **샘플 시점의 top node와 시각**만 제공하고, 브라우저의
task 시작/종료 경계를 제공하지 않는다. 한 task 안에서도 스택은 계속 바뀌며,
같은 스택이 서로 다른 task에서 연속 관측될 수도 있다. idle 샘플까지 기본
제외하면 경계 추정은 더 불가능해진다.

또한 현재 `collapsedStacks`는 `Sample.Labels.ts_us`를 버리고 stack→count로
합산한다(`internal/analyzers/profile/analyzer.go:40,97-110`). 기존
`buildTimeline`은 `[]Sample`이나 labels가 아니라 이미 합산된 `FlameNode`를 받기
때문에, 설계서의 “timeline builder가 `ts_us`를 읽는다”는 변경만으로는 구현할 수
없다.

권장 수정:

- `.cpuprofile`만 있을 때는 **연속 sampled CPU hotspot** 또는 **sample run**으로
  부르고 Long Task라는 용어와 finding code를 사용하지 않는다.
- 정확한 Long Task는 Chrome trace의 `RunTask`/task event 경계를 정본으로 삼고,
  그 구간에 CPU samples를 귀속한다.
- `AnalysisResult`를 유지하더라도 시간 순서 데이터는 합산 flamegraph와 별도의
  series/table 계약으로 정의한다. `Sample.Labels`를 내부 전달 수단으로 쓰더라도
  collapse 이전에 소비해야 한다.

[CDP Profiler.Profile 명세](https://chromedevtools.github.io/devtools-protocol/tot/Profiler/#type-Profile)는
`samples`를 top-node ID, `timeDeltas`를 인접 샘플 간 마이크로초 간격으로만
정의한다. task 경계는 이 계약에 없다.

---

## P1 — 정확성·아키텍처·견고성 보완

### 5. 분류 규칙과 FlameGraph 색상 연결 경로가 설계와 다르다

`runtime_classification_rules.json`을 수정해도 현재 profile evidence의
component/execution breakdown은 바뀌지 않는다. 이 JSON 기반
`profileclassification.ClassifyStack`은 API의 독립 분류 호출에서만 사용되고,
실제 공통 profiler 분석은 `internal/profiler/classify.go:203`의 하드코딩
`classifyFrames`/`classifyStack`을 호출한다.

또한 일반 `buildFlameTree`의 `freezeNode`는 `Category`와 `Color`를 채우지 않는다
(`flamegraph.go:157-177`). 따라서 “엔진에서 색을 채우면 렌더러 변경 0건”이라는
방향은 가능하지만, 어느 분석 단계가 어떤 분류기로 모든 노드를 순회하는지가
구현 목록에 빠져 있다.

분류기는 source format/runtime context를 받아 하나로 통합하는 편이 안전하다.
전역 규칙 순서에서 `V8 Internal`을 `Node.js`보다 앞으로 옮기면 동일한 `v8::`
프레임을 가진 Node 프로파일까지 브라우저로 재분류한다. `fetch`/XHR 표본 역시
CPU 실행 근거이지 네트워크 대기시간 근거가 아니므로 UI 문구와
`EXTERNAL_API_HTTP` 해석에서 이를 분명히 해야 한다.

### 6. 포맷 검증과 시간 귀속 규칙이 부족하다

CDP 계약상 `samples`, `timeDeltas`, `children`, `hitCount`는 optional이다. 설계서도
`nodes[]`만으로 hitCount 기반 flamegraph가 가능하다고 설명하지만(`:149-153`),
detect는 `nodes+samples`를 요구하고 실제 알고리즘은 `samples`만 정본으로 삼는다.
valid-but-aggregated profile을 지원할지, “개별 samples 필수”로 범위를 좁힐지
결정해야 한다.

파서는 다음 비정상 입력을 명시적으로 검증해야 한다.

- duplicate/zero node ID, 존재하지 않는 sample/child ID
- 한 child의 복수 parent, cycle, root가 아닌 단절 노드
- `len(samples) != len(timeDeltas)` 및 `endTime < startTime`
- 누적 delta와 `endTime-startTime`의 과도한 불일치

DevTools 자체는 samples/timeDeltas 길이가 다르면 profile parse를 실패시킨다.
짧은 쪽으로 조용히 자르면 sample과 timestamp 대응이 바뀌므로 기본 정책으로
적절하지 않다. 첫 delta는 공식 명세상 startTime에서 첫 sample까지의 간격이다.
이를 두 번째 delta나 중앙값으로 교체하는 규칙(`:298-299`)은 근거가 없고 총시간도
왜곡한다. timestamp 복원과 sample duration 귀속을 분리해, 예를 들어 sample i의
비용은 다음 timestamp 또는 endTime까지의 구간으로 정의하는 식의 명시적 규칙과
테스트가 필요하다.

공식/구현 근거:

- [CDP Profiler.Profile 타입](https://chromedevtools.github.io/devtools-protocol/tot/Profiler/#type-Profile)
- [DevTools SamplesHandler](https://chromium.googlesource.com/devtools/devtools-frontend.git/+/refs/heads/main/front_end/models/trace/handlers/SamplesHandler.ts)

### 7. 표시용 URL 단축이 프레임 정체성과 원본 증거를 잃는다

현재 공통 analyzer는 `Frame.File`이 아니라 `Frame.Name`만 이어 붙여 collapse
키를 만든다. 설계처럼 URL을 basename으로 줄여 `renderList (app.js:120)`만
남기면 서로 다른 origin/path의 같은 파일명·함수·라인이 합쳐진다.
`Sample.Labels`는 sample-level map이므로 한 스택에 포함된 여러 프레임의 원본
URL을 각각 보존할 수도 없다.

표시 이름과 정체성을 분리해야 한다. `Frame.File`에는 query/fragment와 민감한
정보를 제거한 canonical URL을 보존하고, collapse key에는 충돌 방지용 script
identity를 포함하되 UI에서만 짧게 보여주는 방식이 필요하다. trace에 resource
content/source map이 포함될 수 있으므로 scripts metadata 상한뿐 아니라 URL,
소스 내용, 확장 프로그램/사내 주소에 대한 redaction 및 export 정책도 추가해야
한다.

### 8. 대형 파일 대책이 JSON 파싱 시점의 OOM을 막지 못한다

샘플 루프의 RSS 체크와 파싱 후 downsampling은 이미 `os.ReadFile`과 전체
`json.Unmarshal`이 끝난 뒤 실행되므로 파싱 시점 OOM을 막지 못한다. 현재
`detect()`의 `jsonHas`도 매 호출마다 전체 JSON을 `map[string]any`로 unmarshal해
본 파싱 전에 큰 객체를 한 번 더 만든다(`parser.go:939-953`).

크기 가드는 필요하지만 256MB 일괄 거부만으로 “대형 프로파일 지원”이라고 하기는
어렵다. 최소한 다음을 구분해야 한다.

- `.cpuprofile`: stat/gzip sniff → 제한된 top-level decode 또는 streaming/token
  decode → nodes 구축 → samples를 읽으며 온라인 집계/선택
- Performance trace: gzip 스트리밍 해제, top-level array와
  `{traceEvents:[]}` 모두 지원, `ProfileChunk`만 선택
- downsampling: 단순 sample 개수 배율 보정보다 건너뛴 delta를 보존하는
  bucket/weighted aggregation 사용

부분 결과를 반환하려면 현재 `ParseFile`의 `(Parsed, diagnostics, error)`와
`Analyze`의 error 처리 계약도 바꿔야 한다. 지금은 error가 있으면 결과가
폐기된다. “부분 결과 + warning”과 “중단 error”를 명확히 분리해야 한다.

### 9. 옵션과 외부 API 변경 목록이 빠져 있다

설계는 `--include-idle`, `MaxRSSMB`, `MaxStackDepth`, sample cap을 사용하지만
`internal/parsers/profile.Options`는 현재 빈 구조체이고, CLI/Wails의
`ProfileEvidenceOptions`와 request에도 이 필드가 없다. parser/analyzer/UI 간 옵션
소유권, 기본값, metadata 기록, Diff 양쪽 적용 규칙까지 파일 목록에 포함해야 한다.

특히 idle 제거는 단순 표시 옵션과 총시간 회계 옵션을 분리해야 한다. active
flamegraph에서는 숨겨도 recording duration, idle ratio, active ratio 계산에는
원본 값을 유지해야 한다.

---

## P2 — 테스트·문서 품질

### 10. 완료 기준을 end-to-end fixture로 바꿔야 한다

현재 Phase 1 테스트 목록에 다음 케이스를 추가하는 것이 필요하다.

- 공식 Chrome Performance trace JSON, JSON.GZ, object/array wrapper
- Node.js `--cpu-prof` fixture와 CDP direct profile fixture
- duplicate/missing/cyclic node, samples/timeDeltas mismatch
- 동일 basename을 가진 서로 다른 script URL
- idle 100%, GC 포함, samples 없는 hitCount-only profile
- CLI와 Wails 결과의 단위/summary/flamegraph parity
- 두 cpuprofile의 실제 `DiffPage` end-to-end
- 256MB 경계 아래/위 및 gzip bomb 방어

실제 브라우저 산출 fixture는 생성 Chrome/Node 버전과 생성 명령을 함께 기록하고,
URL·소스·사내 식별자를 sanitize해야 한다.

### 11. 영문 설계 문서와 지원 매트릭스 반영이 필요하다

신규 설계는 `docs/ko`에만 있고 `docs/en` 대응 문서와 README 링크가 없다. 구현
확정 후 영문 문서를 짝지어 추가하고, `IMPORTER_SUPPORT_MATRIX`, `DATA_MODEL`,
`USER_GUIDE`, `PERFORMANCE`에 실제 지원 포맷과 제한을 반영해야 한다. 구현 전에는
지원 매트릭스에 “planned” 상태로만 표시해야 한다.

---

## 권장 재구성 순서

| 순서 | 설계/구현 게이트 | 완료 조건 |
|---|---|---|
| 1 | 실제 수집 포맷 결정 | Performance trace 우선인지 V8 direct profile 우선인지 문서와 메뉴 문구가 일치 |
| 2 | 측정 단위·시간 귀속 계약 | duration/idle/estimated seconds 골든 테스트 통과 |
| 3 | 파서 검증·프레임 identity | malformed 입력이 deterministic diagnostic을 내고 script 충돌 없음 |
| 4 | CLI `profile import` | `.cpuprofile` 또는 결정된 1차 trace fixture가 `profile_evidence` 산출 |
| 5 | 데스크톱 단일 분석 경로 | Browser 페이지와 기존 profiler 정책이 명확하고 workspace 등록 가능 |
| 6 | 공통 Diff/Export | 실제 두 파일 비교와 report export end-to-end 통과 |
| 7 | context-aware 분류·색상 | Node/Browser 회귀 없이 flame node category/color가 채워짐 |
| 8 | 정확한 시간축 | trace task 경계 기반 Long Task 또는 명확히 이름 붙인 sample-run 분석 |
| 9 | 대형 파일/trace 확대 | JSON.GZ streaming, cap, benchmark, partial-result 정책 검증 |

## 한 문장 요약

**V8 CpuProfile importer라는 핵심 아이디어는 유효하지만, 현재 문서는 실제 Chrome
저장 포맷, 시간 단위, Wails/Diff 경로, Long Task 의미를 잘못 연결하고 있으므로
이 네 계약을 먼저 바로잡은 뒤 구현해야 한다.**
