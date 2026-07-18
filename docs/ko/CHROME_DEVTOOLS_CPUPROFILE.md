# Chrome DevTools 프로파일(.cpuprofile) 분석 (설계 노트)

## 1. 배경 및 동기

### 현재 상태

ArchScope의 runtime profile importer(`internal/parsers/profile`)는 22종 포맷을
지원하지만 전부 **서버 사이드** 런타임이다. JVM(pprof, JFR, async-profiler),
Python(py-spy), Ruby(rbspy, stackprof), PHP(Excimer, Tideways, Cachegrind),
.NET, Go, Rust가 대상이고 브라우저 JavaScript 프로파일은 진입점이 없다.

`.cpuprofile` 파일을 현재 엔진에 넣으면 다음과 같이 오분류된다.

- `detect()`(`parser.go:464`)는 `.cpuprofile` 확장자를 모르므로
  `parser.go:519`의 fallback으로 떨어져 `generic-async-stack`으로 판정한다.
- `.json`으로 확장자를 바꾸면 `parser.go:510-515`에서 `stackTrace` 키를 찾지 못해
  `generic-profile-json`이 되고, V8의 `nodes[]`/`samples[]` 구조를 해석하지
  못한 채 빈 결과 또는 `PROFILE_EMPTY` finding을 낸다.

즉 **현재는 지원하지 않는다**가 정확한 상태이며, 실패도 조용히 일어난다.

### 왜 필요한가

1. **프론트엔드 병목의 근거 확보.** ArchScope는 access log → APM → 서버 프로파일
   순으로 응답시간 근거를 좁혀왔으나, "서버는 80ms인데 체감은 3초"인 구간
   — 번들 파싱, 리렌더 폭주, 롱태스크 — 은 근거 없이 추정으로 남아 있다.
2. **수집 비용이 사실상 0.** DevTools Performance 패널은 모든 Chrome에 내장되어
   있고 에이전트 설치, 권한, 재기동이 필요 없다. 다른 importer 대비 도입 장벽이
   가장 낮다.
3. **파이프라인 재사용률이 높다.** 3절 이후에서 보듯 `[]Sample`만 만들면
   FlameGraph, Drilldown, Timeline, Classification, Diff, Report Export가
   그대로 동작한다. 신규 코드 대비 회수하는 기능 면적이 크다.
4. **Node.js와의 대칭.** `--cpu-prof`로 만든 Node 프로파일도 동일한 V8
   CpuProfile 스키마다. 브라우저 지원은 서버 사이드 Node 프로파일 지원을
   부산물로 얻는다.

### 비목표

- DevTools를 대체하는 UI를 만들지 않는다. ArchScope의 역할은 **보관, 비교,
  근거화, 리포트**이며 실시간 인터랙션 프로파일링은 DevTools가 계속 담당한다.
- Chrome 원격 디버깅 프로토콜(CDP) 직접 연결은 이번 범위가 아니다(10절).

## 2. 목표 — 수집에서 분석까지

```
[Chrome DevTools]                    [ArchScope]
Performance 패널 기록
  └ Save profile… ──> foo.cpuprofile ──> profile import --in foo.cpuprofile
                                            │
                                            ├─ detect() → v8-cpuprofile-json
                                            ├─ []Sample 변환
                                            └─ AnalysisResult (profile_evidence)
                                                 ├ FlameGraph / Drilldown
                                                 ├ Timeline
                                                 ├ Classification (브라우저 규칙)
                                                 └ Diff / Report Export
```

사용자 워크플로는 세 단계로 끝나야 한다.

1. DevTools Performance 패널에서 기록 후 저장(또는 `node --cpu-prof`).
2. `archscope-engine profile import --in ./trace.cpuprofile` 또는 데스크톱 앱에
   드래그 앤 드롭.
3. 기존 프로파일 화면에서 그대로 확인. **포맷 전용 화면을 새로 만들지 않는다.**

성공 기준은 "서버 프로파일과 똑같이 다뤄진다"이다. 배포 전후 두 개의
`.cpuprofile`을 `DiffPage`에서 비교할 수 있으면 목표 달성이다.

### 플레임차트가 아니라 플레임그래프다

같은 `.cpuprofile`을 DevTools와 ArchScope가 **서로 다른 그림**으로 그린다.
이름이 비슷해 혼동되지만 X축의 의미가 근본적으로 다르며, 이 차이를 이해하지
못하면 "DevTools에서 본 것과 그래프 모양이 다르다"는 오해가 생긴다.

| | 플레임차트 (Flame Chart) | 플레임그래프 (Flame Graph) |
|---|---|---|
| **X축** | **시간** — 왼쪽에서 오른쪽으로 흐르는 실제 타임라인 | **샘플 수(비중)** — 시간 순서 없음 |
| **X축 정렬** | 발생 시각 순 | 함수명 알파벳순(비교 안정성 확보) |
| **같은 함수** | 호출될 때마다 **별도 블록**으로 반복 등장 | 동일 스택이면 **하나로 합산** |
| **블록 폭** | 그 호출 1회의 소요 시간 | 그 스택의 총 누적 비중 |
| **답하는 질문** | "**언제** 무엇이 실행됐나" | "**무엇이** 가장 비싼가" |
| **강점** | 롱태스크 위치, 이벤트 순서, 특정 구간 원인 추적 | 전체 병목 순위, 두 프로파일 비교, 회귀 탐지 |
| **약점** | 폭이 넓어 전체 비중 파악이 어렵고 비교가 사실상 불가 | 시간 정보 소실 — "3초 지점"을 표현할 수 없음 |
| **대표 도구** | Chrome DevTools Performance 패널 | Brendan Gregg FlameGraph, pprof, async-profiler |

**Chrome DevTools Performance 패널이 그리는 것은 플레임차트다.** 기록 구간 전체를
시간축에 펼치고 각 호출을 개별 블록으로 보여준다. 반면 **ArchScope의
`buildFlameTree`(`flamegraph.go:86`)가 만드는 것은 플레임그래프다.** 동일 스택을
`collapsedStacks`(`analyzer.go:97`)에서 `"a;b;c" → count`로 합산하는 순간 시간
순서는 사라지고 누적 비중만 남는다.

이는 결함이 아니라 **의도된 설계**다. ArchScope의 목적은 실시간 인터랙션
디버깅(DevTools의 영역)이 아니라 보관·비교·근거화이며, 두 프로파일을 겹쳐
회귀를 찾으려면 시간축이 제거된 합산 표현이 필수다. 플레임차트는 같은 코드라도
실행 순서가 조금만 달라지면 그림이 완전히 달라져 Diff 대상이 되지 못한다.

따라서 두 표현은 대체 관계가 아니라 **보완 관계**이고, 본 문서의 로드맵은
플레임그래프(합산)를 먼저 완성한 뒤 플레임차트(시간축)를 덧붙이는 순서를 따른다
(9절).

## 3. 지원 포맷

### 3.1 V8 CpuProfile JSON (`.cpuprofile`) — 1차 대상

DevTools Performance 패널의 "Save profile", Node.js `--cpu-prof`,
`v8.Profiler.stop()`의 산출물이 공유하는 스키마다.

```jsonc
{
  "nodes": [
    {
      "id": 1,
      "callFrame": {
        "functionName": "(root)",
        "scriptId": "0",
        "url": "",
        "lineNumber": -1,
        "columnNumber": -1
      },
      "hitCount": 0,
      "children": [2, 5]
    }
  ],
  "startTime": 1234567890123,   // microseconds
  "endTime":   1234567899999,
  "samples":   [2, 2, 5, 5, 5], // node id 시퀀스
  "timeDeltas": [0, 125, 118, 130, 121]  // 직전 sample 대비 microseconds
}
```

구조적 특징과 그로 인한 설계 제약:

| 특징 | 설계상 의미 |
|---|---|
| `nodes[]`는 **부모→자식 단방향** 트리. `parent` 필드 없음 | 역인덱스(child id → parent id)를 1회 구축해야 스택 복원 가능 |
| `id`는 1-based가 관례지만 **연속 보장 없음** | 배열 인덱스가 아닌 `map[int]*node`로 다뤄야 안전 |
| `samples[i]`는 **리프 노드 id** | 루트까지 부모를 거슬러 올라가야 전체 스택이 나옴 |
| `timeDeltas[i]`는 마이크로초, **음수 가능** | 클램프 필요(4.3) |
| `hitCount`는 있으나 `samples[]`와 **중복 정보** | `samples[]`를 정본으로 삼고 `hitCount`는 무시 또는 검증용 |
| `(root)`, `(program)`, `(idle)`, `(garbage collector)` 합성 프레임 | 분류 규칙 필요(6절) |
| `url`은 `https://…/main.a1b2c3.js` 형태 | 프레임 이름 생성 시 정규화 필요(4.4) |

#### 어떤 필드가 어떤 표현을 가능하게 하는가

이 스키마에서 주목할 점은 **세 배열의 역할이 다르다**는 것이다. 2절의 구분이
여기서 기술적 근거를 얻는다.

- **`nodes[]`만으로는 플레임그래프까지만 가능하다.** `nodes[]`는 호출 트리와
  각 노드의 `hitCount`를 담는다. 즉 "어떤 스택이 몇 번 관측됐는가"라는 **집계
  결과**다. 여기에는 각 호출이 *언제* 일어났는지에 대한 정보가 전혀 없다.
  `hitCount`를 그대로 누적하면 정확히 플레임그래프가 나오지만, 그 이상은
  복원할 수 없다. 정보가 이미 소실되었기 때문이다.
- **`samples[] + timeDeltas[]`가 시간축을 복원한다.** `samples[i]`는 i번째 관측
  시점의 리프 노드이고 `timeDeltas[i]`는 직전 관측과의 간격(마이크로초)이다.
  따라서 `timeDeltas`를 누적하면 각 샘플의 **절대 타임스탬프**가 나온다.

  ```
  ts[i] = startTime + Σ(timeDeltas[0..i])
  ```

  샘플을 시간순으로 늘어놓고 인접한 동일 스택을 병합하면 "이 함수가 t=3.12s부터
  t=3.29s까지 실행됐다"는 구간이 만들어지고, 이것이 곧 **플레임차트**다.

정리하면 `.cpuprofile`은 두 표현에 필요한 정보를 **모두** 담고 있으며, 어느
쪽을 그릴지는 순전히 소비 측의 선택이다. `hitCount`를 "중복 정보"로 표시한 위
표는 플레임그래프 관점에서만 참이다 — 시간축을 쓰려면 `samples[]`와
`timeDeltas[]`가 유일한 원천이므로 파서는 **두 배열을 반드시 보존해야 한다**.
`hitCount`만 읽고 `timeDeltas`를 버리는 구현은 Phase 3(플레임차트)의 길을
스스로 막는다. 4.3절이 `timeDeltas`를 클램프해서라도 살려 두고, 5절이 `ts_us`
라벨로 이를 전달하는 이유가 여기에 있다.

### 3.2 Chrome Trace Event JSON (`.json`) — 2차 대상

DevTools "Save as…" 전체 트레이스는 Trace Event Format 배열이며, CPU 프로파일은
그 안에 `Profile` / `ProfileChunk` 이벤트로 **조각나서** 들어 있다.

```jsonc
[
  {"ph":"P","name":"Profile","id":"0x1","args":{"data":{"startTime":…}}},
  {"ph":"P","name":"ProfileChunk","id":"0x1",
   "args":{"data":{"cpuProfile":{"nodes":[…],"samples":[…]},"timeDeltas":[…]}}},
  {"ph":"X","name":"FunctionCall","dur":1234, …}
]
```

`ProfileChunk`의 `nodes`/`samples`/`timeDeltas`를 id별로 누적 병합하면 3.1과
동일한 구조가 된다. 따라서 **3.1 파서를 내부 함수로 두고 트레이스 파서는
전처리 어댑터**로 구현한다.

Phase 4로 미루는 이유: 파일이 통상 10~100배 크고(전체 트레이스), 스트리밍
파싱이 사실상 필수이며, `ph:"X"` 지속 이벤트(롱태스크, 레이아웃, 페인트)는
샘플이 아니라 **구간**이라 `[]Sample` 모델에 그대로 담기지 않는다.

### 3.3 범위 밖

| 포맷 | 판단 |
|---|---|
| `.heapprofile` / `.heapsnapshot` | 메모리 프로파일. `[]Sample` 모델과 축이 다름. 별도 family |
| Firefox Profiler JSON | 스키마 상이. 수요 확인 후 별도 검토 |
| Safari Web Inspector | 비공개 포맷. 대상 아님 |
| Speedscope로 변환된 `.cpuprofile` | **이미 지원**(`speedscope-json`). 사용자가 변환하면 오늘도 동작하나, 변환 단계가 도입 장벽이라 네이티브 지원의 근거가 약해지지 않음 |

## 4. 파서 설계

### 4.1 배치

기존 `parser.go`가 이미 1000줄을 넘으므로 신규 파일로 분리한다.

```
internal/parsers/profile/
├── parser.go              # detect(), canonical(), dispatch (수정)
├── cpuprofile.go          # 신규 — parseV8CpuProfile()
└── cpuprofile_test.go     # 신규
```

### 4.2 detect() 확장

`detect()`(`parser.go:464`)에 **`.json` 확장자 분기(`parser.go:510`)보다 앞에**
다음을 추가한다. 확장자보다 내용을 우선해야 `.json`으로 저장된 cpuprofile도
잡힌다.

```go
// nodes[] + samples[] 조합은 V8 CpuProfile 고유 시그니처다.
if jsonHas(data, "nodes", "samples") {
    return "v8-cpuprofile-json"
}
```

`.cpuprofile` 확장자 분기도 함께 추가하되, 위 내용 기반 판정이 우선한다.
`canonical()`(`parser.go:522`)에는 alias를 등록한다.

```go
case "cpuprofile", "v8-cpuprofile", "chrome-cpuprofile", "devtools-cpuprofile":
    return "v8-cpuprofile-json"
```

`jsonHas`는 top-level 키만 검사하므로 3.2의 트레이스 배열(top-level이 array)은
걸리지 않는다. Phase 4에서 `traceEvents` 또는 `ph`/`ProfileChunk` 시그니처로
별도 분기한다.

**회귀 위험**: `nodes`+`samples`를 top-level에 갖는 다른 포맷이 있으면 가로챈다.
현재 지원 포맷 중 해당 조합은 없으나(speedscope는 `shared.frames`+`profiles`,
stackprof는 `frames`+`raw`), 판정 순서를 바꾸는 변경이므로 기존 fixture 3종에
대한 detect 회귀 테스트를 함께 넣는다.

### 4.3 nodes[]/samples[]/timeDeltas[] → []Sample

핵심은 **id 기반 트리를 역방향으로 걸어 루트-우선 스택을 만드는 것**이다.
`parseStackprof`(`parser.go:299`)의 id→frame 맵 패턴과 `parseSpeedscope`
(`parser.go:212`)의 공유 프레임 테이블 패턴을 합친 형태가 된다.

```go
func parseV8CpuProfile(data []byte, opts Options) (Parsed, error) {
    var doc v8CpuProfile
    if err := json.Unmarshal(data, &doc); err != nil { … }

    // 1) id → node 인덱스. id 연속성을 가정하지 않는다.
    byID := make(map[int]*v8Node, len(doc.Nodes))
    for i := range doc.Nodes { byID[doc.Nodes[i].ID] = &doc.Nodes[i] }

    // 2) 자식 → 부모 역인덱스. children이 유일한 간선 정보다.
    parent := make(map[int]int, len(doc.Nodes))
    for _, n := range doc.Nodes {
        for _, c := range n.Children { parent[c] = n.ID }
    }

    // 3) node id → 루트-우선 스택. 메모이즈로 O(N·D) → 사실상 O(N).
    stackOf := memoizedStackBuilder(byID, parent)

    // 4) samples[i] + timeDeltas[i] → Sample
    samples := make([]Sample, 0, len(doc.Samples))
    for i, nodeID := range doc.Samples {
        delta := deltaAt(doc.TimeDeltas, i)   // 음수/결측 클램프
        ...
    }
}
```

세부 결정 사항:

- **스택 방향**: `Sample.Stack`은 루트-우선(`parser.go:20` 주석)이다. 부모를
  거슬러 올라가면 리프-우선이 되므로 `reverseFrames`(`parser.go:798`)를 적용한다.
- **메모이제이션**: 깊이 D의 스택을 샘플마다 재구성하면 O(S·D)이고 S가 수십만이
  되면 체감 지연이 생긴다. node id → `[]Frame` 캐시를 두면 노드 수만큼만
  구성한다. 캐시는 슬라이스를 **공유**하되, 하위에서 변형하지 않도록 읽기 전용
  규약을 지킨다.
- **Value 단위**: `Sample.Value`는 `int`다. `timeDeltas`는 마이크로초이므로
  그대로 담으면 단위가 기존 포맷(샘플 수 또는 ms)과 어긋난다. **마이크로초를
  정본으로 담고**, `Metadata.Extra`에 `"value_unit": "microseconds"`를 기록한다.
  분석기의 `IntervalMS`는 `samples.length`와 총 경과시간으로 역산해 채운다.
- **음수/결측 delta**: V8은 클럭 보정으로 음수 delta를 낼 수 있다. `delta < 0`이면
  0으로 클램프하고 diagnostic에 카운트를 기록한다. `len(timeDeltas) !=
  len(samples)`인 경우도 실제로 발생하므로(±1) 짧은 쪽에 맞추고 경고한다.
- **`(idle)` / `(program)`**: 기본적으로 **제외한다**. `(idle)`을 포함하면
  FlameGraph 폭의 대부분이 유휴가 되어 병목이 보이지 않는다. `--include-idle`
  플래그로 옵트인한다. `(garbage collector)`는 **포함한다** — GC는 실제 비용이다.
- **첫 샘플**: `timeDeltas[0]`은 `startTime` 기준 오프셋이라 샘플 비용이 아니다.
  관례대로 첫 샘플에는 두 번째 delta 또는 중앙값을 쓴다.

### 4.4 Frame 매핑

```go
Frame{
    Name:     "renderList (app.js:120)",   // 표시용 합성 이름
    Function: cf.FunctionName,             // 빈 문자열이면 "(anonymous)"
    File:     shortenURL(cf.URL),          // https://host/a/b/main.a1b2.js → main.a1b2.js
    Line:     cf.LineNumber + 1,           // V8는 0-based, ArchScope는 1-based
    Language: "JavaScript",
    Runtime:  "V8",
    Native:   isV8Synthetic(cf),           // (program), (garbage collector) 등
}
```

- `functionName`이 빈 문자열인 익명 함수는 `(anonymous)`로 채운다. 그대로 두면
  `collapsedStacks`(`analyzer.go:97`)의 `;` 구분 키에서 빈 세그먼트가 되어
  서로 다른 프레임이 병합된다.
- URL은 origin과 경로를 잘라 파일명 + 해시만 남긴다. 전체 URL을 남기면 프레임
  이름이 100자를 넘어 FlameGraph 라벨이 무의미해진다. 원본 URL은 `Labels`에
  보존해 drilldown에서 조회 가능하게 한다.
- **`inferRuntime` 회귀 주의**: `inferRuntime`(`parser.go:732`)에는 JavaScript
  분기가 없고 `::` 포함 여부로 Rust를 판정한다. `v8::internal::…` 프레임이
  Rust로 오분류되므로, cpuprofile 경로는 파서가 `Runtime`/`Language`를 **명시적으로
  채워** `inferRuntime`이 개입하지 않도록 한다(`normalizeSample`은 값이 이미
  있으면 덮어쓰지 않는다).

### 4.5 Metadata

`Parsed.Metadata`에 담아 `result.Metadata.Extra`로 전달한다.

| 키 | 값 |
|---|---|
| `value_unit` | `"microseconds"` |
| `start_time_us` / `end_time_us` | `doc.StartTime` / `doc.EndTime` |
| `duration_ms` | `(end - start) / 1000` |
| `sample_count` | `len(doc.Samples)` |
| `node_count` | `len(doc.Nodes)` |
| `avg_sample_interval_us` | 실측 평균 — 샘플링 레이트 신뢰도 판단용 |
| `idle_ratio` | `(idle)` 비중. 제외했더라도 기록 |
| `scripts` | 등장한 스크립트 URL 목록(상한 있음) |

## 5. 기존 파이프라인 연동

`collapsedStacks`(`analyzer.go:97`) 이후는 전부 포맷 비의존이다. 따라서 연동
작업의 대부분은 "아무것도 하지 않는 것"이며, 아래 표에서 **변경 필요**로 표시된
항목만 손댄다.

| 단계 | 구현 | 필요 작업 |
|---|---|---|
| FlameGraph | `buildFlameTree`(`flamegraph.go:86`) | 없음 |
| Drilldown | `BuildDrilldownStages`(`drilldown.go:175`) | 없음 |
| Top stacks / child frames | `topStacksFromTree`(362), `topChildFrames`(380) | 없음 |
| Execution breakdown | `buildExecutionBreakdown`(`breakdown.go:35`) | 없음 |
| Component breakdown | `componentBreakdown`(`breakdown.go:80`) | 분류 규칙 추가(6절) |
| Timeline | `buildTimeline`(`timeline.go:100`) | **변경 필요**(Phase 3) — 아래 |
| Runtime/Language 분포 | `normalizeSample` 경유 | 파서가 값을 채우면 자동 |
| Diff | `ProfilerService.Diff` | 없음 |
| Report Export | HTML/PPTX 파이프라인 | 없음 |

### Timeline

`timeDeltas`는 샘플별 실제 타임스탬프를 복원할 수 있는 정보다. 기존 collapsed
경로는 타임스탬프가 없어 균등 분포를 가정하지만, cpuprofile은 **실제 시간축**을
가진다. 이 정보를 버리면 "3초 지점에 롱태스크" 같은 가장 유용한 관찰을 잃는다.

`Sample.Labels`에 `ts_us`(프로파일 시작 기준 오프셋)를 기록하고, 타임라인
빌더가 해당 라벨이 있으면 실제 타임스탬프를, 없으면 기존 균등 분포를 쓰도록
분기한다. `Labels`는 이미 `map[string]string`으로 존재하므로 `Sample` 구조체
변경은 필요 없다 — **`AnalysisResult` 공통 contract를 건드리지 않는다**는
프로젝트 가드레일을 지키는 선택이다.

**단계 분리에 주의한다.** `ts_us` 라벨을 *채우는* 것은 Phase 1의 파서 작업이고,
그 라벨을 *소비해* 시간축을 그리는 것은 Phase 3다. 파서가 Phase 1에서 라벨을
남겨 두지 않으면 Phase 3에서 원본 파일을 다시 파싱해야 하므로, 표현이
플레임그래프뿐인 시점에도 라벨은 기록해 둔다(3.1의 "두 배열을 반드시
보존해야 한다"와 같은 이유).

### 결과 타입

`ResultType`은 기존 `"profile_evidence"`를 그대로 쓴다.
`internal/ingestion/boundary.go:132`의 `EvidenceFamilySpec`은 family 단위
등록이고 이번 작업은 family 내부의 포맷 추가이므로 **변경 없다**.

### CLI

`cmd_profile.go:41`의 `--format` 도움말 문자열에 포맷명을 추가한다. 이 문자열은
사용자가 지원 포맷을 확인하는 유일한 창구이므로 누락 시 기능이 있어도 발견되지
않는다.

## 6. 브라우저 프레임 분류 규칙

### 6.1 런타임 계열 규칙

`internal/analyzers/profileclassification/config/runtime_classification_rules.json`
(go:embed)에 규칙을 추가한다. 구조는 `{Label, Contains[]}`이며 **첫 매칭 우선,
대소문자 무시 부분 문자열** 방식이다(`profileclassification.go`).

기존 `"Node.js"` 규칙은 `node:`, `node_modules`, `v8::`, `uv_`를 잡는다.
브라우저 프레임(`(garbage collector)`, `https://…/bundle.js`)은 아무것도 매칭되지
않아 `FallbackLabel = "Application"`으로 떨어진다.

추가할 규칙(순서 중요 — 좁은 규칙이 먼저):

| Label | Contains 예시 |
|---|---|
| `React` | `react-dom`, `commitwork`, `beginwork`, `performconcurrentwork`, `renderwithhooks`, `flushsync` |
| `Vue` | `vue.runtime`, `patchelement`, `reactiveeffect`, `flushjobs` |
| `Bundler Runtime` | `webpack_require`, `webpack/bootstrap`, `__vite__`, `parcelrequire`, `systemjs` |
| `V8 Internal` | `(garbage collector)`, `(program)`, `(root)`, `(idle)`, `v8::`, `builtin:` |
| `Browser API` | `blink::`, `settimeout`, `requestanimationframe`, `xmlhttprequest`, `fetch` |

`"Node.js"` 규칙이 `v8::`를 이미 포함하므로 **`V8 Internal`을 `Node.js`보다
앞에 두어야** 브라우저 GC 프레임이 Node.js로 분류되지 않는다. 규칙 순서 변경은
Node 프로파일 분류에 영향을 주므로 기존 Node fixture로 회귀 확인이 필요하다.

### 6.2 의미론적 프레임 분류

`internal/profiler/classify.go`의 `classifyFrames`(203행)는 JVM 중심 토큰
목록이 하드코딩되어 있다. 브라우저 프레임에 대응하는 카테고리 매핑:

| 브라우저 관심사 | 기존 카테고리 | 토큰 |
|---|---|---|
| GC | `GC_JVM_RUNTIME` | `(garbage collector)` |
| fetch/XHR | `EXTERNAL_API_HTTP` | `fetch`, `xmlhttprequest`, `send` |
| 프레임워크 렌더 | `FRAMEWORK_MIDDLEWARE` | `react-dom`, `commitwork`, `vue.runtime` |
| 앱 코드 | `APPLICATION_LOGIC` | (fallback) |

`GC_JVM_RUNTIME`이라는 이름이 브라우저 맥락에서 어색하나, **카테고리 상수를
변경하면 JVM 경로와 프론트엔드 라벨, i18n 메시지가 함께 깨진다**. 이번 범위에서는
기존 상수를 재사용하고, 라벨 중립화(`GC_RUNTIME`)는 별도 리팩터링으로 분리한다.
설계 부채로 명시해 둔다.

레이아웃/페인트/스타일 계산은 CPU 프로파일 프레임이 아니라 트레이스 이벤트라
3.2(Phase 4) 이후에나 분류 대상이 된다.

## 7. 프론트엔드 변경 범위

**원칙: 최소 변경. 신규 페이지를 만들지 않는다.**

조사 결과 프론트엔드에는 포맷 문자열이 하드코딩된 곳이 없다
(`speedscope-json`, `pyspy-raw` 등으로 grep 시 결과 없음). 포맷 선택은 generic
경로이며, 기존 `ProfilerAnalyzerPage.tsx`가 FlameGraph/Drilldown을 그대로
렌더한다.

필요한 변경:

| 항목 | 파일 | 내용 |
|---|---|---|
| 파일 선택 필터 | `ProfilerAnalyzerPage.tsx` | accept 확장자에 `.cpuprofile` 추가 |
| 포맷 드롭다운 | `ProfilerAnalyzerPage.tsx` | 목록에 항목 추가(auto 감지가 기본이므로 선택적) |
| i18n | `i18n/messages.ts` | 신규 분류 라벨(React, V8 Internal 등) 번역 키 |
| 도움말 | `help/helpCatalog.ts` | 수집 방법 안내 1개 항목 |

CLI 경로만 쓴다면 프론트엔드 변경은 **0건**이다. 위 항목은 데스크톱 앱에서
드래그 앤 드롭을 지원하기 위한 최소 집합이다.

## 8. 대형 프로파일 처리

### 현재 격차

메모리 가드는 전부 `internal/profiler`(collapsed 파일 경로)에 있다.

- `defaultMaxUniqueStacks = 100_000`, `defaultMaxStackDepth = 512`,
  `defaultMaxRSSMB = 4096`, `defaultMaxFlamegraphNodes = 100_000`
  (`collapsed.go:81-95`)

반면 `internal/parsers/profile.ParseFile`은 `os.ReadFile`(`parser.go:63`) 후
전체를 `json.Unmarshal` 한다. **크기 상한도, 샘플 상한도, 스트리밍도 없다.**
기존 포맷은 실무상 파일이 작아 문제가 드러나지 않았을 뿐이다.

30분 기록한 `.cpuprofile`은 수백 MB가 나온다. 이때 원본 바이트 + Go 구조체가
동시에 상주해 실측 3~5배로 부풀고, JSON 파싱 중 OOM이 발생하면 Wails 앱 전체가
죽는다(엔진 프로세스 분리가 없는 구간).

### 대응

**(a) 파싱 전 크기 가드.** `ParseFile` 진입 시 `os.Stat`으로 크기를 확인하고
임계치(기본 256MB) 초과 시 파싱을 시도하지 않고 명확한 오류를 낸다.
`profiler/analyzer.go:65-80`의 RSS 체크가 이미 "OOM 대신 친절한 오류" 패턴을
쓰고 있으므로 그 선례를 따른다. 이 가드는 cpuprofile 전용이 아니라
**모든 포맷에 적용**되는 개선이다.

**(b) 파싱 중 RSS 체크.** 샘플 루프에서 N개마다 RSS를 확인해 `MaxRSSMB` 초과 시
중단하고 부분 결과 + diagnostic을 반환한다. 부분 결과라도 상위 병목은 대개
드러나므로 전량 실패보다 낫다.

**(c) 샘플 다운샘플링.** `samples`가 상한(기본 500,000)을 넘으면 균등 간격으로
추출하고 각 샘플의 `Value`를 비율만큼 보정한다. 통계적 프로파일에서 균등
다운샘플링은 상대 비중을 보존한다. 적용 시 `Metadata.Extra`에 `downsampled:
true`와 비율을 기록하고 **UI에 반드시 노출**한다 — 조용한 절삭은
"전부 분석했다"는 오해를 낳는다.

**(d) 깊이 절삭.** 재귀 스택은 수천 프레임까지 간다. 기존
`truncateStackDepth`(`collapsed.go:176`)와 동일하게 `MaxStackDepth = 512`를
적용하고 `OverDepthRecords` diagnostic을 쓴다.

**(e) FlameGraph pruning.** `pruneFlamegraph`(`analyzer.go:227`)가
`MaxFlamegraphNodes` 기준으로 레벨별 top-K + `"...other"` 처리를 이미 한다.
추가 작업 없음.

**(f) 스트리밍(Phase 4).** 3.2 트레이스 포맷은 `json.Decoder`로 `traceEvents`
배열을 토큰 단위 순회하며 `ProfileChunk`만 뽑는 방식이 필요하다. 전체 문서를
메모리에 올리지 않는 유일한 방법이다.

## 9. 구현 단계

### Phase 1 — 최소 동작 (`.cpuprofile` 파싱)

1. `cpuprofile.go`에 `parseV8CpuProfile` 구현 (4.3)
2. `detect()` / `canonical()` 확장 + 기존 3개 fixture detect 회귀 테스트 (4.2)
3. `examples/profile/sample-chrome.cpuprofile` fixture 추가
4. `cpuprofile_test.go` — 스택 복원, 음수 delta, id 비연속, `(idle)` 제외,
   빈 `functionName`
5. `cmd_profile.go:41` `--format` 도움말 갱신

**완료 기준**: `profile import --in sample-chrome.cpuprofile`이
**플레임그래프**와 top stacks를 담은 `profile_evidence`를 산출한다. 이 시점의
표현은 합산 기반이며 시간축은 `ts_us` 라벨로 보존만 된다(5절).

### Phase 2 — 분류 (플레임그래프)

6. `runtime_classification_rules.json`에 브라우저 규칙 추가 + 순서 검증 (6.1)
7. `classify.go` 브라우저 토큰 매핑 (6.2)
8. Node.js `--cpu-prof` fixture로 회귀 확인
9. 배포 전후 `.cpuprofile` Diff 검증 — 합산 표현의 핵심 가치

**완료 기준**: React 앱 프로파일에서 컴포넌트 렌더와 GC가 분리되어 보이고,
두 프로파일의 병목 순위 변화를 Diff로 비교할 수 있다.

> Phase 1~2는 **플레임그래프(합산 분석)** 범위다. 시간 순서를 버리고 누적
> 비중만 다루므로 `timeDeltas`는 `Sample.Value` 산출과 `ts_us` 라벨 **보존**에만
> 쓰이고 표현에는 관여하지 않는다. 이 두 Phase만으로 "무엇이 가장 비싼가",
> "배포 후 무엇이 나빠졌는가"에 답할 수 있으며, 이것이 ArchScope의 주된 용도다.

### Phase 3 — 시간축 (플레임차트)

10. `ts_us` 라벨 기반 실제 타임스탬프 타임라인 (5절)
11. 인접 동일 스택 병합 → 시간 구간(span) 생성 (3.1)
12. 롱태스크 탐지 — 임계치(기본 50ms) 초과 연속 구간을 finding으로
13. 시간 구간 렌더링 방식 결정 — 기존 `CanvasFlameGraph` 확장 vs 신규 컴포넌트

**완료 기준**: "기록 3.1초 지점에 210ms 롱태스크, 원인은 `renderList`"를
프로파일만으로 지목할 수 있다.

> Phase 3이 **플레임차트(시간축 분석)** 범위다. Phase 1~2와 달리 X축이 시간이
> 되므로 합산 파이프라인(`collapsedStacks` 이후)을 우회하는 **별도 경로**가
> 필요하다. 기존 플레임그래프를 대체하는 것이 아니라 나란히 추가하는 것이며,
> 두 표현은 같은 `[]Sample`에서 파생된다.
>
> 항목 13은 UI 비용이 크다. `CanvasFlameGraph`는 합산 트리를 전제로 하므로
> 시간축 렌더링에 그대로 쓰기 어렵다. Phase 3 착수 전 **DevTools를 그대로 쓰는
> 편이 낫지 않은가**를 먼저 검토한다 — ArchScope가 플레임차트를 제공할 실익은
> "저장된 과거 프로파일을 다시 볼 때"에 한정되며, 그 수요가 확인되지 않으면
> Phase 3은 타임라인 finding(항목 12)까지만 하고 렌더링은 보류한다.

### Phase 4 — 견고성 및 데스크톱 UI

14. `ParseFile` 크기 가드 (8-a) — 전 포맷 공통
15. 다운샘플링 + diagnostic + UI 노출 (8-c)
16. 깊이 절삭 (8-d)
17. 대형 fixture(100MB+) 벤치마크를 `PERFORMANCE.md` 기준에 추가
18. `.cpuprofile` 드래그 앤 드롭, i18n, 도움말 (7절)
19. Chrome Trace Event 스트리밍 파서 (3.2, 8-f)

**완료 기준**: 256MB 초과 파일이 OOM 없이 명확한 오류 또는 다운샘플 결과를 낸다.

**우선순위 주의.** 위 순서는 논리적 의존성 기준이고 실제 착수 순서는 다를 수
있다. Phase 1 완료 후 실제 파일 크기를 측정해 항목 14~15가 Phase 2보다 시급하면
앞당긴다 — 파싱 단계에서 앱이 죽으면 분류 규칙은 의미가 없다. 반대로
다운샘플링(항목 15)은 **Phase 3보다 반드시 먼저** 와야 한다. 균등 다운샘플링은
합산 비중은 보존하지만 시간 구간은 왜곡하므로, 두 기능이 함께 켜지면 잘못된
롱태스크 위치를 보고하게 된다. 이 상호작용은 항목 12 구현 시 다운샘플 여부를
확인해 시간축 finding을 억제하는 방식으로 처리한다.

## 10. 제약 사항 및 미래 확장

### 제약

- **샘플링 프로파일러의 한계.** V8 기본 샘플 간격은 약 1ms다. 그보다 짧은 함수는
  통계적으로만 나타나며 짧은 기록에서는 아예 누락될 수 있다. "이 함수가 안
  보인다"가 "실행되지 않았다"를 뜻하지 않는다.
- **인라이닝.** V8이 인라인한 함수는 별도 프레임으로 나타나지 않고 호출자에
  흡수된다. self time 해석 시 주의가 필요하다.
- **소스맵 미적용.** 프로덕션 번들은 `main.a1b2c3.js:1:48213` 형태다. 소스맵
  적용은 별도 작업이며 이번 범위가 아니다. 원본 함수명이 필요하면 개발 빌드를
  프로파일링해야 한다.
- **CPU만.** 네트워크 대기, 레이아웃, 페인트, 컴포지팅은 CPU 프로파일에
  나타나지 않는다. `.cpuprofile`만으로는 프론트엔드 성능의 일부만 설명된다 —
  이 한계는 사용자에게 문서로 명시해야 한다.
- **단일 스레드.** 워커와 메인 스레드는 별도 파일로 저장된다. 통합 뷰 없음.

### 미래 확장

| 방향 | 내용 | 선행 조건 |
|---|---|---|
| Chrome Trace 전체 | 롱태스크, 레이아웃, 페인트를 타임라인에 병치 | Phase 4, 스트리밍 파서 |
| 소스맵 | `.map` 파일로 번들 프레임을 원본 위치로 복원 | 소스맵 라이브러리 도입 |
| Lighthouse JSON | LCP/TBT/CLS 등 실사용자 지표를 프로파일과 연결 | 별도 evidence family |
| Performance Insights | DevTools Insights 내보내기 포맷 안정화 시 | Chrome 측 포맷 확정 |
| CDP 직접 수집 | `Profiler.start/stop`으로 ArchScope가 직접 수집 | 원격 디버깅 포트 접근 정책 |
| RUM 연동 | 실사용자 프로파일 샘플 수집·집계 | 수집 인프라. 범위 큼 |
| 서버-클라이언트 상관 | trace ID로 백엔드 프로파일과 프론트 프로파일 연결 | `stitch analyze` 확장 |

마지막 항목이 ArchScope 고유의 차별점이다. DevTools는 브라우저만, APM은 서버만
본다. 하나의 요청에 대해 **브라우저 프레임과 서버 프레임을 같은 시간축에**
놓을 수 있으면 기존 도구가 답하지 못하는 질문에 답할 수 있다. 다만 이는
`.cpuprofile` 지원이 아니라 상관관계 인프라의 문제이며, 본 문서의 Phase 1~4가
그 전제 조건을 만든다.
