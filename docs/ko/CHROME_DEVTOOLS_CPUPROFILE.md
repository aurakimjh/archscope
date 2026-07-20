# Chrome DevTools / V8 CPU 프로파일 분석 (설계 노트)

## 0. 설계 상태 — 구현 착수 게이트

2026-07-18 Codex 설계 검토(`docs/review/2026-07-18_codex_chrome_devtools_cpuprofile_design_review.md`)
결과 본 문서는 **조건부 승인** 상태다. 아래 네 계약이 확정되기 전에는 구현에
착수하지 않는다. 각 항목은 `work_status.md`의 TO-DO에 대응한다.

| 게이트 | 미결 쟁점 | TO-DO |
|---|---|---|
| G1 수집 포맷 | 현재 Chrome Performance 패널의 저장 산출물은 `.cpuprofile`이 아니라 **trace(`.json`/`.json.gz`)**다. 1차 대상을 trace로 올릴지, 범위를 V8 direct profile로 좁힐지 미결(3.0) | T-558 |
| G2 측정 단위 | `Sample.Value` 단위와 `IntervalMS` 이중 환산 문제. 시간 귀속 규칙과 골든 불변식 미확정(4.3) | T-559 |
| G3 데스크톱 경로 | 신규 importer와 현재 `ProfilerService`/`DiffPage`가 서로 다른 서비스 경로를 사용(5.0, 7.6) | T-560 |
| G4 시간축 의미 | CPU 샘플만으로는 브라우저 Long Task 경계를 복원할 수 없음(9 Phase 3) | T-561 |

게이트가 열리기 전까지 **전용 메뉴 문구와 수집 안내를 확정하지 않는다.** 7절의
메뉴 설계는 G1 결정에 종속된다. 지원 매트릭스에는 `planned` 상태로만 표기한다.

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
[수집원]                              [ArchScope]
Chrome Performance 패널
  └ Save trace ──> foo.json(.gz) ─┐
                                  │  (G1 결정 시 trace 어댑터)
Node.js --cpu-prof               │
CDP Profiler.stop ──> foo.cpuprofile ──> profile import --in <file>
                                            │
                                            ├─ detect() → v8-cpuprofile-json
                                            ├─ V8 CpuProfile 정규화 → []Sample
                                            └─ AnalysisResult (profile_evidence)
                                                 ├ FlameGraph / Drilldown
                                                 ├ Timeline (Phase 3, 9절)
                                                 ├ Classification (브라우저 규칙)
                                                 └ Diff / Report Export
```

**주의(G1).** 위 그림의 두 수집원은 산출 포맷이 다르다. Chrome Performance
패널의 저장 기능은 현재 **Save trace**이고 결과물은 `.json` 또는 `.json.gz`이며
Chrome 142부터 gzip이 기본이다. `.cpuprofile`은 Node.js `--cpu-prof`와 CDP
`Profiler.stop`의 산출물이다. 따라서 "Chrome DevTools에서 저장한 파일을
연다"는 사용자 서사는 **trace 어댑터가 있어야 성립한다**(3.0).

사용자 워크플로는 세 단계로 끝나야 한다.

1. `node --cpu-prof` 또는 CDP로 `.cpuprofile` 확보. Chrome Performance 패널
   저장 파일은 G1 결정 이후 지원한다.
2. `archscope-engine profile import --in ./profile.cpuprofile` 또는 데스크톱
   앱에 드래그 앤 드롭(진입점은 G1/G3 결정에 종속, 7절).
3. 브라우저 맥락에 맞춘 화면에서 확인. 화면은 분리하되 **분석 엔진은 서버
   프로파일과 100% 공유한다**(7절).

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

### 3.0 1차 대상 결정 (G1 / T-558) — 미결

설계 초안은 `.cpuprofile`을 1차, Chrome trace를 Phase 4로 두었다. 그러나 현재
공식 DevTools 문서는 Performance 패널의 다운로드를 **Save trace**로 정의하고,
저장 파일은 `.json` / `.json.gz`(Chrome 142+ gzip 기본)이며 top-level 배열과
`{traceEvents, metadata}` 객체를 모두 허용한다. DevTools 자체 테스트 fixture도
`*.json.gz`로 관리된다. 즉 **현재 Phase 순서로는 Chrome 사용자가 저장한 파일을
열 수 없다.**

선택지는 둘이다.

| 안 | 내용 | 대가 |
|---|---|---|
| **A. trace 우선** | Chrome trace JSON/JSON.GZ 어댑터를 Phase 1로 올리고, 내부 `Profile`/`ProfileChunk`를 V8 CpuProfile 정규화 함수로 전달 | gzip 스트리밍과 대형 파일 처리를 Phase 1에서 떠안음(8절) |
| **B. 범위 축소** | 1차 범위를 `.cpuprofile`로 유지하되 문서 제목·수집 안내를 **V8 CPU Profile(Node.js/CDP) 지원**으로 좁히고, Chrome Performance 사용자는 아직 미지원임을 명시 | "브라우저 성능 분석" 메뉴의 근거가 약해짐 |

제품 동기(프론트엔드 병목 근거 확보)와 신규 메뉴 구상을 고려하면 **A가 더
일관적이다.** 다만 A는 Phase 1 비용을 크게 올리므로, B로 시작해 trace 어댑터를
Phase 2로 앞당기는 절충도 유효하다. 어느 쪽이든 **결정 전에는 메뉴 문구와 수집
안내를 확정하지 않는다.**

근거:

- [Chrome DevTools: Save and share performance traces](https://developer.chrome.com/docs/devtools/performance/save-trace)
- [DevTools trace fixtures README](https://chromium.googlesource.com/devtools/devtools-frontend/+/refs/heads/main/front_end/panels/timeline/fixtures/traces/README.md)
- [Node.js `--cpu-prof` CLI](https://nodejs.org/download/release/v22.17.0/docs/api/cli.html#--cpu-prof)

### 3.1 V8 CpuProfile JSON (`.cpuprofile`)

Node.js `--cpu-prof`, CDP `Profiler.stop`, `v8.Profiler.stop()`의 산출물이
공유하는 스키마다. Chrome trace 내부의 `ProfileChunk`도 병합하면 동일 구조가
되므로, 어느 안을 택하든 **이 스키마의 정규화 함수가 공통 코어**다.

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

#### optional 필드 정책 (T-563)

CDP 계약상 `samples`, `timeDeltas`, `children`, `hitCount`는 **모두
optional**이다. 반면 4.2의 detect는 `nodes`+`samples` 조합을 요구하고 4.3의
알고리즘은 `samples`를 정본으로 삼는다. 즉 위 서술("`nodes[]`만으로
플레임그래프가 가능하다")과 구현 계획이 어긋난다. 다음 중 하나를 명시한다.

- **지원**: `samples`가 없으면 `hitCount` 기반 집계 전용 경로로 떨어지고,
  시간축 기능(Phase 3)과 `duration_ms`는 비활성 + diagnostic으로 표시한다.
- **미지원**: detect 조건을 유지하고 aggregated-only profile은 명시적 오류로
  거부한다.

기본 권장은 **지원**이다. 거부하면 CDP 계약상 유효한 입력이 조용히 실패한다.
어느 쪽이든 `hitCount`-only fixture로 테스트를 고정한다(9절).

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
- **음수 delta**: V8은 클럭 보정으로 음수 delta를 낼 수 있다. `delta < 0`이면
  0으로 클램프하고 diagnostic에 카운트를 기록한다.
- **길이 불일치**: `len(timeDeltas) != len(samples)`는 **조용히 자르지 않는다.**
  짧은 쪽에 맞추면 sample과 timestamp의 대응이 통째로 밀려 시간 귀속이 틀어진다.
  DevTools 자체도 길이가 다르면 profile parse를 실패시킨다. 기본 정책은 **오류**,
  옵트인 옵션으로만 `--tolerate-delta-mismatch` 복구를 허용하고 이때 diagnostic을
  경고 이상으로 올린다.
- **`(idle)` / `(program)`**: 표시에서는 기본 **제외한다**. `(idle)`을 포함하면
  FlameGraph 폭의 대부분이 유휴가 되어 병목이 보이지 않는다. `--include-idle`
  플래그로 옵트인한다. `(garbage collector)`는 **포함한다** — GC는 실제 비용이다.
  단 4.3.1의 회계 규칙에 따라 **제외는 표시 옵션이고 총시간 회계 옵션이 아니다.**

#### 4.3.1 측정 단위·시간 귀속 계약 (G2 / T-559) — 미결

초안은 `Sample.Value`에 마이크로초를 담고 동시에 `IntervalMS`를 평균 샘플
간격으로 역산해 채우라고 했다. **이 조합은 모든 시간 지표를 이중 환산한다.**
현재 공통 분석기는 다음과 같이 계산한다
(`internal/profiler/analyzer.go:313-314`).

```go
intervalSeconds := options.IntervalMS / 1000
estimatedSeconds := round(float64(totalSamples)*intervalSeconds, 3)
```

`Value`가 이미 시간인데 여기에 평균 간격을 다시 곱하면 결과가 무의미해진다.
현재 코드 경로에는 더 직접적인 문제도 있다.

- `internal/analyzers/profile.Build`는 옵션이 없으면 `IntervalMS=100`을 무조건
  적용하고(`analyzer.go:36-39`), parser metadata에서 간격을 읽지 않는다. 파서
  어느 포맷도 샘플링 간격을 채우지 않으므로 이 기본값은 **전 포맷 공통**이다.
- CLI `--interval-ms` 기본값도 100이라 자동 역산 경로가 호출되지 않는다.
- `unified_profile_schema.sample_unit`은 `"samples"` 리터럴로 고정되어 있다
  (`analyzer.go:78`). 파서가 `value_unit=microseconds`를 남겨도 중첩된
  `parser_metadata`에만 들어가 공통 지표의 의미를 바꾸지 못한다.

따라서 구현 전에 계약을 **하나로** 고정해야 한다.

| 안 | 내용 | 대가 |
|---|---|---|
| **최소 변경** | V8 경로에서 `Value=귀속 마이크로초`, `IntervalMS=0.001` | `total_samples`, `samples`, `sample_ratio` 필드명이 사실상 마이크로초를 뜻하게 되는 계약 부채 |
| **권장** | 분석기 내부에 `value_unit`/`count_unit`을 명시하고 시간 환산을 **단일 함수**로 모음. `sample_unit`을 하드코딩에서 파생값으로 전환 | 공통 분석기 수정 범위가 넓음 |

**시간 귀속 규칙**도 함께 정해야 한다. `timeDeltas[0]`은 공식 명세상
`startTime`에서 첫 샘플까지의 간격이다. 초안의 "첫 샘플에 두 번째 delta나
중앙값을 쓴다"는 규칙은 **근거가 없고 총시간을 왜곡한다.** 대신 timestamp 복원과
비용 귀속을 분리한다.

- timestamp 복원: `ts[i] = startTime + Σ(timeDeltas[0..i])`
- 비용 귀속: 샘플 `i`의 비용 = `ts[i+1] - ts[i]`, 마지막 샘플은 `endTime - ts[i]`

또한 **recording duration과 active CPU duration을 분리 기록**한다. idle을
제외하더라도 `idle_ratio`, `active_ratio`, 총 기록 시간은 원본 값으로 계산한다.

어느 안을 택하든 다음 불변식을 **골든 테스트로 먼저 고정한 뒤** 구현한다.

1. `sum(timeDeltas)`와 결과의 busy/estimated duration이 허용 오차 내에서 일치.
2. CLI 기본 옵션과 Wails 기본 옵션이 같은 결과를 산출.
3. Diff 정규화 여부와 무관하게 baseline/target 단위가 동일.
4. idle 제외 전후에도 recording duration과 active CPU duration이 구분됨.

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
- **표시 이름과 정체성을 분리한다(T-563).** 초안대로 URL을 basename으로 줄여
  `renderList (app.js:120)`만 남기면 **서로 다른 origin/path의 같은 파일명·함수·
  라인이 하나로 합쳐진다.** 현재 공통 analyzer의 collapse 키는 `Frame.File`이
  아니라 `Frame.Name`만 이어 붙이므로(`analyzers/profile/analyzer.go:312-323`)
  충돌이 그대로 결과에 남는다. 같은 파일의 `frameRows`(`:183`)는
  `{Name, File, Runtime, Language}`로 키를 만들어 **flamegraph와 frames 테이블이
  서로 다른 정체성을 쓰는 기존 불일치**도 함께 드러난다.
  - `Frame.File`에는 query/fragment를 제거한 **canonical URL**을 보존한다.
  - collapse 키에는 충돌 방지용 script identity(`scriptId` 또는 canonical URL)를
    포함하고, **짧은 이름은 UI 표시 계층에서만** 만든다.
  - `Sample.Labels`는 sample 단위 map이라 한 스택에 포함된 여러 프레임의 원본
    URL을 각각 보존할 수 없다. 원본 URL 보존처는 `Labels`가 아니라 `Frame.File`이다.
- **redaction 정책(T-563).** trace에는 resource content와 source map이 포함될 수
  있다. `scripts` metadata 상한만으로는 부족하므로 URL, 소스 내용, 확장 프로그램
  ID, 사내 호스트명에 대한 마스킹 규칙과 export 정책을 함께 정의한다.
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
| `scripts` | 등장한 스크립트 URL 목록(상한 있음, redaction 적용 — 4.4) |

### 4.6 그래프 검증 (T-563)

파서는 다음 비정상 입력을 **명시적으로 검증하고 deterministic diagnostic을**
내야 한다. 조용한 복구는 잘못된 결과를 정상처럼 보이게 한다.

| 입력 | 정책 |
|---|---|
| duplicate node ID, zero/음수 ID | 오류 |
| 존재하지 않는 sample ID / child ID | 오류 |
| 한 child에 복수 parent | 오류 (트리 가정 위반) |
| children cycle | 오류 (스택 복원이 무한 루프) |
| root가 아닌 단절 노드 | 경고 + 해당 노드 스택은 고아 루트로 처리 |
| `len(samples) != len(timeDeltas)` | 오류 (4.3, 옵트인 시에만 복구) |
| `endTime < startTime` | 오류 |
| `Σ timeDeltas`와 `endTime-startTime`의 과도한 불일치 | 경고 + metadata에 편차 기록 |

근거: [CDP Profiler.Profile 타입](https://chromedevtools.github.io/devtools-protocol/tot/Profiler/#type-Profile),
[DevTools SamplesHandler](https://chromium.googlesource.com/devtools/devtools-frontend.git/+/refs/heads/main/front_end/models/trace/handlers/SamplesHandler.ts)

### 4.7 옵션 소유권 (T-563)

본 설계는 `--include-idle`, `MaxRSSMB`, `MaxStackDepth`, sample cap을 전제하지만
**현재 이 필드들은 어디에도 없다.**

- `internal/parsers/profile.Options`는 `struct{}`이고 `ParseFile`은 이를 `_`로
  무시한다(`parser.go:51,58`).
- `ProfileEvidenceOptions`는 `{Format, TopN, IntervalMS, ProfileKind}`뿐이며
  Wails `ProfileEvidenceRequest`도 동일하다(`engineservice.go:193-199`).
- `MaxRSSMB`/`MaxStackDepth`는 legacy `internal/profiler.Options`에만 존재하고
  (`types.go:57,79`) 신규 경로와 연결되어 있지 않다. `include-idle`은 저장소
  전체에 존재하지 않는다.

따라서 구현 파일 목록에 다음을 포함해야 한다: 각 옵션의 **소유 계층**(parser /
analyzer / UI), 기본값, `Metadata.Extra` 기록 여부, **Diff 양쪽에 동일 옵션을
강제하는 규칙**. 특히 idle 제거는 4.3.1대로 표시 옵션과 회계 옵션을 분리한다.

## 5. 기존 파이프라인 연동

`collapsedStacks`(`analyzer.go:97`) 이후의 **분석 로직**은 포맷 비의존이다.
그러나 초안이 "변경 없음"으로 적었던 Diff와 Report Export는 실제로는 **다른
서비스 경로**를 타므로 성립하지 않는다(5.0).

| 단계 | 구현 | 필요 작업 |
|---|---|---|
| FlameGraph 구조 | `buildFlameTree`(`flamegraph.go:86`) | 없음 |
| FlameGraph category/color | `freezeNode`(`flamegraph.go:157-177`) | **변경 필요** — 현재 `Category`/`Color`를 채우지 않아 항상 `null`(6.3) |
| Drilldown | `BuildDrilldownStages`(`drilldown.go:175`) | 없음 |
| Top stacks / child frames | `topStacksFromTree`(362), `topChildFrames`(380) | 없음 |
| Execution breakdown | `buildExecutionBreakdown`(`breakdown.go:35`) | 없음 |
| Component breakdown | `componentBreakdown`(`breakdown.go:80`) | 분류 규칙 추가(6절) |
| Collapse 키 정체성 | `stackKey`/`frameNames`(`analyzer.go:312-323`) | **변경 필요** — script identity 반영(4.4) |
| Timeline | `buildTimeline`(`timeline.go:100`) | **변경 필요**(Phase 3) — 아래 |
| Runtime/Language 분포 | `normalizeSample` 경유 | 파서가 값을 채우면 자동 |
| Diff | `ProfilerService.Diff` | **변경 필요** — 4개 포맷만 지원(5.0) |
| Report Export | HTML/PPTX 파이프라인 | **검증 필요** — Workspace 등록 경유 확인(5.0) |

### 5.0 데스크톱 단일 분석 경로 (G3 / T-560) — 미결

초안은 FlameGraph·Drilldown·Diff·Report Export가 변경 없이 동작하고 기존
`profiler` 메뉴도 `.cpuprofile`을 계속 받는다고 했다. **현재 코드에서는 성립하지
않는다.** 신규 importer와 기존 데스크톱 화면이 서로 다른 서비스를 호출한다.

- 신규 공통 importer 경로: `EngineService.AnalyzeProfileEvidence` →
  `internal/analyzers/profile`(`engineservice.go:518-527`).
- `ProfilerAnalyzerPage`는 이를 호출하지 않고 `ProfilerService.AnalyzeAsync`를
  쓴다(`ProfilerAnalyzerPage.tsx:774`). 이 서비스는
  collapsed / Jennifer / SVG / HTML **4종만** 허용하고
  (`profilerservice.go:450-461`) 파일 필터와 `detectFormat`에 `.cpuprofile`이
  없다.
- `DiffPage`도 같은 legacy `ProfilerService.Diff`를 쓰며 타입이 4종으로 고정되어
  있다(`DiffPage.tsx:33`). Go `loadStacks`도 동일한 4종만 지원한다
  (`profilerservice.go:295-322`).

즉 **"CLI import 성공"과 "데스크톱/Diff 성공"은 서로 다른 구현 과제다.** 다음 중
하나를 명시적으로 선택한다.

| 안 | 내용 | 대가 |
|---|---|---|
| **A. 경로 통합(권장)** | 데스크톱 프로파일 경로를 `AnalyzeProfileEvidence` 중심으로 통합하고 legacy 서비스는 Jennifer/SVG/HTML 어댑터로 축소 | 기존 프로파일러 페이지 회귀 위험 |
| **B. 병행** | 신규 Browser 페이지만 `EngineService`를 쓰고, Diff에는 `profile.Parsed → collapsed stack` 공통 로더를 별도 연결 | 경로 이원화가 고착 |

완료 기준에는 **Browser 페이지 결과가 Analysis Workspace에 등록되고 Export
Center가 이를 소비하는 흐름**까지 포함해야 "Report Export 변경 없음"을 검증할 수
있다.

### Timeline

`timeDeltas`는 샘플별 실제 타임스탬프를 복원할 수 있는 정보다. 기존 collapsed
경로는 타임스탬프가 없어 균등 분포를 가정하지만, cpuprofile은 **실제 시간축**을
가진다. 이 정보를 버리면 "3초 지점에 무엇이 CPU를 점유했나" 같은 가장 유용한
관찰을 잃는다.

`Sample.Labels`에 `ts_us`(프로파일 시작 기준 오프셋)를 기록한다. `Labels`는 이미
`map[string]string`으로 존재하므로 `Sample` 구조체 변경은 필요 없고,
**`AnalysisResult` 공통 contract를 건드리지 않는다**는 프로젝트 가드레일을
지킬 수 있다.

**다만 "타임라인 빌더가 라벨이 있으면 실제 타임스탬프를 쓴다"는 초안의 서술은
현재 구조에서 성립하지 않는다.** `collapsedStacks`(`analyzer.go:97-110`)가
`stackKey` 기준으로 합산하면서 `Labels`, `Thread`, `Process`, `Runtime`을 모두
버리고, `buildTimeline`(`timeline.go:100`)은 `[]Sample`이 아니라 이미 합산된
`FlameNode`를 받는다. `ts_us`는 타임라인 빌더에 **도달하지 못한다**.

따라서 시간축은 합산 경로의 분기가 아니라 **collapse 이전에 분리되는 별도
경로**여야 한다(9절 Phase 3 항목 10, T-561).

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

### 6.0 분류기가 둘로 갈라져 있다 (T-562)

**전제 정정.** `runtime_classification_rules.json`을 고쳐도 현재 profile
evidence의 component/execution breakdown은 **바뀌지 않는다.** 저장소에는 서로
연결되지 않은 분류기가 둘 있다.

| 분류기 | 위치 | 실제 소비처 |
|---|---|---|
| JSON 규칙 기반 `ClassifyStack` | `internal/analyzers/profileclassification` | `engineservice.go:606-607`의 독립 Wails 메서드 **하나뿐**. 분석 결과에 관여하지 않음 |
| 하드코딩 `classifyFrames` | `internal/profiler/classify.go:203` | `breakdown.go:42`, `timeline.go:246` — **실제 분석 경로** |

따라서 6.1의 규칙 추가는 그 자체로는 효과가 없다. T-562는 **두 분류기를
source format / runtime context를 받는 하나의 인터페이스 뒤로 통합**하는 것을
선행 조건으로 한다. 통합 없이 6.1만 적용하면 사용자가 편집 가능한 규칙 파일이
리포트에 반영되지 않는 현재의 혼란이 유지된다.

### 6.1 런타임 계열 규칙

(6.0의 통합을 전제로) 규칙은
`internal/analyzers/profileclassification/config/runtime_classification_rules.json`
(go:embed)에 추가한다. 구조는 `{Label, Contains[]}`이며 **첫 매칭 우선,
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

`"Node.js"` 규칙이 `v8::`를 이미 포함하므로 브라우저 GC 프레임이 Node.js로
분류되는 문제가 있다. **다만 전역 순서에서 `V8 Internal`을 `Node.js`보다 앞으로
옮기는 해법은 틀렸다** — 같은 `v8::` 프레임을 가진 Node 프로파일까지 브라우저로
재분류되어 기존 서버 분석이 회귀한다. `v8::`는 두 런타임에 공통이므로 **순서로는
분리할 수 없다.**

해법은 6.0의 통합 인터페이스가 **source format을 분류 입력으로 받는 것**이다.
`v8-cpuprofile-json` + 브라우저 컨텍스트에서만 `V8 Internal`을 우선하고, Node
컨텍스트에서는 기존 순서를 유지한다. 어느 경우든 Node `--cpu-prof` fixture로
회귀를 고정한다.

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

**`EXTERNAL_API_HTTP` 해석 주의.** `fetch`/`xmlhttprequest` 표본은 요청을
**개시하는 CPU 실행 비용**이지 네트워크 대기시간의 근거가 아니다. CPU 프로파일은
대기 중인 시간을 샘플링하지 않는다. UI 문구와 finding 서술에서 이를 분명히 하지
않으면 "외부 API가 느리다"는 잘못된 결론을 유도한다.

레이아웃/페인트/스타일 계산은 CPU 프로파일 프레임이 아니라 트레이스 이벤트라
3.2(Phase 4) 이후에나 분류 대상이 된다.

### 6.3 category/color 주입 지점 (T-562)

7.4-b는 "엔진에서 색을 채우면 프론트엔드 렌더러 변경이 0건"이라고 한다. 방향은
맞지만 **주입 지점이 설계에 빠져 있다.** 현재 `freezeNode`
(`flamegraph.go:157-177`)는 `ID, ParentID, Name, Samples, Ratio, Children`만
채우고 `Category`/`Color`는 `null`로 직렬화된다. 색을 채우는 곳은 Jennifer
경로(`jennifer.go:488`)뿐이라, 분류 결과는 breakdown 테이블에는 있어도
**flamegraph 노드에는 없다.**

따라서 구현 목록에 다음을 명시한다.

- 어느 분석 단계가 flame tree 전체 노드를 순회하며 분류기를 호출하는가
- 분류 입력이 노드 이름뿐인가, 루트까지의 스택 경로인가 (부분 문자열 매칭은
  경로 없이는 오분류가 잦다)
- category → color 매핑의 소유 계층 (엔진 상수 vs 프론트 팔레트)

## 7. 프론트엔드 구성 — 브라우저 성능 분석 메뉴

> **G1/G3 종속.** 이 절의 메뉴 이름, 수집 안내 문구, 파일 필터는 3.0의 1차 포맷
> 결정에 종속된다. 결정 전에는 **확정하지 않는다.** 또한 7.6의 "엔진 공유" 서술은
> 5.0의 데스크톱 경로 통합이 끝나야 성립한다.

### 7.0 방침 전환 기록

본 문서 초안은 "신규 페이지를 만들지 않고 기존 `ProfilerAnalyzerPage`에
`.cpuprofile` 확장자만 추가한다"는 최소 변경안이었다. 이를 **독립 메뉴 신설로
변경한다.** 근거는 7.1이며, 이에 따라 2절의 "포맷 전용 화면을 새로 만들지
않는다"는 문장은 **철회한다**. 다만 2절의 비목표("DevTools를 대체하는 UI를
만들지 않는다")는 유효하다 — 새 메뉴는 진입점과 표시의 분리이지 DevTools급
인터랙션 도구를 만드는 것이 아니다.

전환 비용은 정직하게 적어 둔다. 최소 변경안의 프론트엔드 작업은 4건이었고,
독립 메뉴안은 7.7 기준 **최소 5개 파일 + 신규 페이지 컴포넌트**다. 기능이
아니라 정보 구조에 쓰는 비용이므로, 7.1의 근거를 납득하지 못한다면 최소
변경안으로 되돌리는 편이 낫다.

### 7.1 왜 서버 프로파일과 분리하는가

`.cpuprofile`을 기존 `프로파일러` 메뉴에 확장자 하나로 끼워 넣지 않는 이유는
네 가지다.

1. **사용자와 상황이 다르다.** JFR/pprof/py-spy를 여는 사람은 서버 장애나 부하
   테스트를 보고 있고, `.cpuprofile`을 여는 사람은 화면이 느리다는 제보를 받은
   프론트엔드 담당이다. 같은 메뉴에 두면 양쪽 모두 자기와 무관한 포맷 목록을
   지나쳐야 한다.
2. **수집 방법 안내의 위치.** 서버 프로파일은 에이전트·플래그·권한 설정이
   필요하고 브라우저는 DevTools 조작이다. 안내가 완전히 달라 한 페이지의 도움말
   에 공존하기 어렵다. `.cpuprofile`의 최대 장점이 "수집이 쉽다"인데 그 안내가
   묻히면 장점이 전달되지 않는다.
3. **해석 어휘가 다르다.** 서버 쪽은 스레드·GC·커넥션풀·락이고 브라우저 쪽은
   롱태스크·리렌더·번들·이벤트 루프다. 6.2절에서 `GC_JVM_RUNTIME` 카테고리를
   브라우저에 재사용하기로 한 타협이 UI 라벨에서 그대로 드러나는데, 메뉴가
   분리되어 있으면 표시 계층에서 브라우저 어휘로 바꿔 줄 수 있다.
4. **확장 방향이 다르다.** 서버 프로파일의 다음 단계는 OTLP Profiles와 JFR
   심화지만, 브라우저의 다음 단계는 Lighthouse·Web Vitals다(7.5). 이들은
   플레임그래프가 아니라 **점수와 지표**라서 프로파일러 페이지에 넣을 자리가
   없다. 지금 독립 메뉴를 만들어 두면 그 확장이 자연스럽게 들어간다.

분리하지 **않는** 것도 명확히 한다. 파서, 분석 엔진, `AnalysisResult` 계약,
FlameGraph 빌더는 전부 공유한다(7.6). 분리 대상은 **진입점과 표시**뿐이다.

### 7.2 현재 내비게이션 구조

새 메뉴를 설계하려면 기존 구조의 제약을 먼저 봐야 한다.

`components/Sidebar.tsx`에는 라우트 테이블이 없다. 컴포넌트 본문에 손으로 쓴
배열 세 개가 있다.

- `NavKey` union (41-59행) — 모든 화면 키의 원천
- `NAV_ICONS: Record<NavKey, NavIcon>` (63-82행) — `lucide-react` 아이콘 매핑
- `analysisItems` (141-151행) — 헤더 `t("navAnalysisTools")`("분석 도구")
- `workspaceItems` (152-161행) — 헤더 `t("navWorkspaceTools")`
- `topLevelItems` (162-164행) — 설정

메뉴 항목의 실제 형태는 **키와 라벨뿐**이고 아이콘은 별도 조회다.

```ts
const analysisItems: { key: NavKey; label: string }[] = [
  { key: "profiler", label: t("navProfiler") },
  …
];
```

라우터 라이브러리도 없다. `App.tsx`는 `useState<NavKey>("profiler")`(132행)와
`{active === "profiler" && <ProfilerAnalyzerPage />}`(196행) 형태의 조건부
렌더링 체인이며, 페이지는 `React.lazy`로 로드한다(37-41행).

제약 두 가지가 설계를 좌우한다.

- **그룹 개념이 배열 + 접이식 헤더가 전부다.** `category`나 `order` 필드가 없고
  순서는 배열 순서, 그룹 순서는 JSX 순서다. 그룹마다 localStorage 키와 상태와
  effect를 손으로 만든다(`TOOLS_STORAGE_KEY`, `WORKSPACE_STORAGE_KEY`, 37-39행).
- **현재 프로파일 관련 4개 페이지는 모두 같은 그룹에 있다.** `profiler`,
  `diff`, `jfr`, `msa_profile`이 전부 `analysisItems` 안에 평면적으로 나열되어
  있고 하위 그룹이 없다.

### 7.3 메뉴 구조

사이드바에 **분석 도구 다음, 워크스페이스 앞**에 새 그룹을 넣는다. 서버
프로파일 도구와 인접해야 두 영역의 대응 관계가 드러나고, 워크스페이스(증거
보드·리포트)는 성격이 달라 뒤에 두는 편이 맞다.

```
사이드바
├─ 분석 도구                      ← 기존, 변경 없음
│   ├─ 프로파일러                  (profiler)   서버 프로파일 22종
│   ├─ JFR                        (jfr)
│   ├─ MSA 프로파일               (msa_profile)
│   ├─ 비교                        (diff)
│   └─ …
├─ 브라우저 성능 분석              ← 신규 그룹
│   ├─ CPU 프로파일                (browser_cpu)      Phase 1~3
│   ├─ Lighthouse 보고서           (browser_lighthouse)  향후, 7.5
│   └─ Web Vitals                  (browser_vitals)      향후, 7.5
└─ 워크스페이스                    ← 기존, 변경 없음
```

Phase 4 시점에 실제로 만드는 항목은 **`browser_cpu` 하나뿐이다.** 나머지 둘은
구조상의 자리만 예약하며 메뉴에 노출하지 않는다. 항목 하나짜리 그룹은 UI
낭비처럼 보이지만, 7.5의 확장이 예정되어 있고 7.1의 분리 근거가 항목 수와
무관하므로 그대로 진행한다. 다만 **Lighthouse 착수 계획이 무산되면 이 그룹은
재검토 대상**이다.

식별자 규칙:

| 대상 | 값 |
|---|---|
| `NavKey` 추가 | `"browser_cpu"` (향후 `"browser_lighthouse"`, `"browser_vitals"`) |
| 그룹 헤더 i18n 키 | `navBrowserTools` — ko "브라우저 성능 분석" / en "Browser performance" |
| 항목 i18n 키 | `navBrowserCpu` — ko "CPU 프로파일" / en "CPU profile" |
| 페이지 컴포넌트 | `pages/BrowserCpuProfilePage.tsx` |
| localStorage 키 | `archscope.profiler.sidebar.browser.expanded` |
| 아이콘 | `Gauge` 또는 `MonitorSmartphone` (`lucide-react`) |
| 도움말 키 | `pageBrowserCpu`, `sidebarBrowser` |

i18n 키는 기존 `nav*` camelCase 관례를 따른다(`messages.ts`의 `en` 200-212행 /
`ko` 981-993행). 도움말 키는 `page<PascalNavKey>` / `sidebar<Group>` 관례를
따른다(`helpCatalog.ts`). `messages.ts`는 `MessageKey = keyof typeof messages.en`
이라 ko 누락 시 타입 에러가 나므로 양쪽을 함께 추가해야 한다.

### 7.4 브라우저 특화 뷰

`BrowserCpuProfilePage`는 `ProfilerAnalyzerPage`의 복사본이 아니라 **공유
컴포넌트를 재조합한 페이지**다. `CanvasFlameGraph`는 이미
`ProfilerAnalyzerPage`, `JfrAnalyzerPage`, `DiffPage`가 함께 쓰고 있어 재사용
선례가 확립되어 있다.

**(a) 브라우저 전용 수집 안내.** 페이지 상단에 DevTools 기록 방법을 3단계로
고정 노출한다. 서버 프로파일 페이지에는 없어야 할 요소이고, 이것이 메뉴를
분리하는 가장 즉각적인 실익이다.

**(b) 프레임워크별 색상 구분.** 현재 `CanvasFlameGraph`의 `colorFor`(106-111행)
우선순위는 `node.color` → `hashColor(node.category)` → `hashColor(node.name)`
이고, `FALLBACK_PALETTE`(62-79행)는 16색 해시 배정이라 **의미 없는 색**이다.

여기서 중요한 점은 `node.color`가 **엔진 payload에서 그대로 전달된다**는 것이다
(`ProfilerAnalyzerPage.tsx:298`의 `color: node.color ?? null`). 따라서 분류
카테고리 → 색상 매핑을 **엔진 측에서 채우면 프론트엔드 렌더러 변경이 0건**이다.
`DiffPage`가 이미 색을 명시적으로 덮어쓰는 선례(43-66행)가 있다.

| 분류 | 색상 | 의도 |
|---|---|---|
| React / Vue | 파랑 계열 | 프레임워크 렌더 |
| Bundler Runtime | 회색 | 배경 소음, 시선 분산 억제 |
| V8 Internal / GC | 주황 | 런타임 비용 |
| Browser API | 청록 | I/O 및 브라우저 경계 |
| Application | 초록 | 사용자 코드 — 최적화 대상 |

색맹 접근성을 고려해 색만으로 구분하지 않고 범례와 툴팁 텍스트를 함께 제공한다.

**(c) CPU 점유 구간 하이라이트.** Phase 3의 항목 12가 만드는 **sample run**
구간을 타임라인에 표시하고 클릭 시 해당 구간의 스택으로 이동한다. UI 문구는
"롱태스크"가 아니라 **"연속 CPU 점유 구간"**을 쓴다 — 9절 Phase 3의 G4 근거대로
CPU 샘플만으로는 task 경계를 알 수 없고, 잘못된 용어는 사용자가 Web Vitals의
Long Task 지표와 동일시하게 만든다. **Phase 3 의존 기능이므로 메뉴 신설 시점에는
비활성 상태로 둔다.**

**(d) 브라우저 어휘 라벨.** 6.2절에서 `GC_JVM_RUNTIME` 상수를 재사용하기로 한
타협을 이 페이지의 표시 계층에서 흡수한다. 카테고리 상수는 그대로 두고 라벨만
"가비지 컬렉션"으로 바꾼다. 상수를 건드리면 JVM 경로와 i18n이 함께 깨지므로
**표시 계층에서만** 처리한다.

**(e) 재사용/신규 구분.**

| 컴포넌트 | 처리 |
|---|---|
| `WailsFileDock` | 재사용 — accept에 `.cpuprofile` |
| `CanvasFlameGraph` | 재사용 — 색상은 엔진이 `node.color`로 공급 |
| `DrilldownPanel` | 재사용 (현재 `ProfilerAnalyzerPage` 전용이나 결합 없음) |
| `MetricCard`, `DiagnosticsPanel`, `AnalyzerOptionsDock` | 재사용 |
| `adaptFlameNode` | **중복 주의** — 페이지마다 각자 구현 중. `ProfilerAnalyzerPage`의 것을 공용으로 추출한 뒤 공유 |
| 수집 안내 배너, CPU 점유 구간 뷰, 색상 범례 | 신규 |

`adaptFlameNode`류 어댑터는 이미 세 페이지에 중복되어 있다. 네 번째 복사본을
만들지 말고 이번에 공용 함수로 추출한다.

### 7.5 확장성 — Lighthouse / Web Vitals

새 메뉴는 `.cpuprofile` 전용이 아니라 **브라우저 성능 근거 전반의 컨테이너**로
설계한다.

| 항목 | 데이터 성격 | 결과 타입 | 재사용 가능 여부 |
|---|---|---|---|
| CPU 프로파일 | 스택 샘플 | `profile_evidence` | 기존 파이프라인 100% |
| Lighthouse 보고서 | 감사 점수 + 기회 목록 | 신규 `browser_audit_evidence` | 낮음 — 플레임그래프 아님 |
| Web Vitals | 시계열 메트릭(LCP/INP/CLS) | 신규 `browser_vitals_evidence` | 낮음 — 차트 계열 |
| Performance Insights | DevTools 인사이트 | 미정 | Chrome 포맷 확정 후 판단 |

**이 표가 7.1의 4번 근거를 뒷받침한다.** Lighthouse와 Web Vitals는
`AnalysisResult`는 공유하되 `[]Sample` 모델에는 맞지 않는다. 플레임그래프
중심인 `ProfilerAnalyzerPage`에 이들을 넣을 방법이 없고, 브라우저 그룹이 있으면
형제 항목으로 자연스럽게 들어간다.

각 항목은 `internal/ingestion/boundary.go`의 `EvidenceFamilySpec`에 **별도
family로 등록**한다. CPU 프로파일은 기존 `runtime_profiles` family에 남고, 이는
"메뉴 그룹 ≠ evidence family"임을 뜻한다 — 메뉴는 사용자 관점의 묶음이고
family는 엔진 관점의 계약이다. 둘을 억지로 일치시키지 않는다.

### 7.6 기존 프로파일 메뉴와의 관계

**공유하는 것 — 엔진 전체:**

| 계층 | 공유 대상 |
|---|---|
| 파서 | `internal/parsers/profile` — 포맷만 추가 |
| 분석기 | `internal/analyzers/profile`, `internal/profiler` |
| 결과 계약 | `AnalysisResult` / `profile_evidence` — **변경 없음** |
| FlameGraph/Drilldown 빌더 | `buildFlameTree`, `BuildDrilldownStages` |
| CLI | `profile import` — 그룹 신설 없음 |
| Wails 바인딩 | `AnalyzeProfileEvidence` / `ProfilerService` |
| 프론트 공용 컴포넌트 | `CanvasFlameGraph`, `WailsFileDock` 등 |

**분리하는 것 — 진입점과 표시뿐:**

| 계층 | 분리 대상 |
|---|---|
| 사이드바 그룹 | 신규 `navBrowserTools` |
| 페이지 컴포넌트 | `BrowserCpuProfilePage` |
| 수집 안내 / 도움말 | 브라우저 전용 문구 |
| 색상·라벨 | 브라우저 어휘 및 프레임워크 색상 |

즉 **엔진은 하나, 창구는 둘**이다. 서버 프로파일 사용자는 변화를 느끼지 않고,
브라우저 사용자는 자기 맥락에 맞는 화면으로 진입한다.

`profiler` 메뉴가 `.cpuprofile`을 받을지는 **5.0의 G3 결정에 따른다.** 초안은
"auto-detect가 포맷 무관이라 계속 받는다"고 했으나, 현재 `ProfilerService`는
4종 포맷만 허용하고 파일 필터와 `detectFormat`에 `.cpuprofile`이 없다. 즉
**지금은 받지 않으며, 받게 하려면 별도 구현이 필요하다.** A안(경로 통합)을
택하면 자연히 받게 되고, B안(병행)을 택하면 기존 메뉴는 서버 포맷 전용으로
남는다. 어느 쪽이든 브라우저 메뉴는 대체가 아니라 **전용 창구**다.

### 7.7 구현 시 파일 목록

내비게이션에 항목 하나를 추가하려면 **최소 5곳**을 고쳐야 한다(라우터가 없어
자동 등록 지점이 없다).

| 파일 | 변경 |
|---|---|
| `components/Sidebar.tsx` | `NavKey`에 `"browser_cpu"`, `NAV_ICONS` 항목, `browserItems` 배열, 그룹 접기 상태 + localStorage + effect + 토글 블록 |
| `App.tsx` | `lazy()` import, `{active === "browser_cpu" && …}` 렌더 |
| `pages/BrowserCpuProfilePage.tsx` | 신규 |
| `i18n/messages.ts` | `navBrowserTools`, `navBrowserCpu` — **en/ko 양쪽 필수** |
| `help/helpCatalog.ts` | `pageBrowserCpu`, `sidebarBrowser` |

**리팩터링 제안(선택).** 그룹 신설은 localStorage 키 + 상태 + effect + JSX
블록을 통째로 복제하는 작업이다. 이번이 **세 번째 그룹**이므로 `NAV_GROUPS`
서술자 배열로 추출할 시점이다.

```ts
const NAV_GROUPS = [
  { key: "analysis", labelKey: "navAnalysisTools", storageKey: "…tools.expanded",   items: […] },
  { key: "browser",  labelKey: "navBrowserTools",  storageKey: "…browser.expanded", items: […] },
  { key: "workspace", labelKey: "navWorkspaceTools", storageKey: "…workspace.expanded", items: […] },
];
```

다만 이 리팩터링은 사이드바 전체의 회귀 위험을 지므로 **본 작업과 분리해
선행 또는 후행 커밋**으로 처리한다. 함께 커밋하면 브라우저 메뉴 추가로 인한
회귀와 리팩터링 회귀를 구분할 수 없다.

### 7.8 UI 설계 — 화면 레이아웃

7.4가 **컴포넌트 재사용 전략**(무엇을 다시 쓰고 무엇이 신규인가)을 다뤘다면,
이 절은 **화면 배치**(선택된 컴포넌트가 어떻게 놓이는가)를 다룬다. 두 절은 같은
페이지의 서로 다른 단면이며, 색상·드릴다운·CPU 점유 구간의 세부는 7.4를
정본으로 삼고 여기서는 위치와 상호작용만 기술한다.

#### 7.8.1 전체 레이아웃

`BrowserCpuProfilePage`는 기존 `ProfilerAnalyzerPage`의 3분할 골격(좌측 옵션/
드릴다운 · 중앙 플레임그래프 · 우측 상세)을 따르되, **상단에 브라우저 전용
수집 안내 배너**(7.4-a)와 **분류 요약 스트립**을 얹는다. 서버 프로파일 화면과
같은 근골격을 유지해 두 화면을 오갈 때 사용자가 재학습하지 않게 한다.

```
┌────────────────────────────────────────────────────────────────────────┐
│ 툴바: [📂 .cpuprofile 열기] [🔍 검색…] [필터 ▾] [⇄ 비교] [내보내기 ▾]     │
├────────────────────────────────────────────────────────────────────────┤
│ ⓘ DevTools 기록법: ① Performance 패널 ② ⏺ 기록 ③ 우클릭 → Save profile  │  ← 7.4-a
├────────────────────────────────────────────────────────────────────────┤
│ 분류 요약:  Framework 34% ▐ V8 Internal 21% ▐ User 28% ▐ GC 9% ▐ Net 8% │  ← 7.8.4
├──────────────┬──────────────────────────────────────────┬──────────────┤
│ 옵션·드릴다운 │  플레임그래프 (CanvasFlameGraph 재사용)    │ 상세          │
│              │                                          │              │
│ ▸ include    │  ┌──────────────────────────────────┐    │ 프레임        │
│ ▸ exclude    │  │ ███ App.render      (초록)        │    │ renderRoot   │
│ ▸ reroot     │  │  ██ React.reconcile (파랑)        │    │ react-dom.js │
│              │  │  █ V8 GC            (주황)         │    │  :1204       │
│ 브레드크럼:   │  │ ██ node_modules/…   (노랑?)       │    │ self  128ms  │
│ (root) ›     │  │  █ Browser API      (청록)        │    │ total 512ms  │
│  render ›    │  └──────────────────────────────────┘    │ calls 47     │
│  reconcile   │  범례: ■초록 사용자 ■파랑 프레임워크 …    │ [소스 열기]  │
├──────────────┴──────────────────────────────────────────┴──────────────┤
│ 타임라인 (Phase 3):  CPU% ▁▃█▅▂▁▇█▄▂  ▮연속 CPU 점유 구간  └─브러시─┘    │  ← 7.8.5
└────────────────────────────────────────────────────────────────────────┘
```

배치도의 `?` 표기(node_modules 노랑)와 타임라인 행은 **미확정·조건부 요소**다.
아래 각 절에서 근거와 함께 다룬다.

#### 7.8.2 플레임그래프 뷰

`CanvasFlameGraph`를 **그대로 재사용**한다(7.4-e). 색상은 프론트가 아니라
엔진이 `node.color`로 공급하므로(7.4-b) 이 페이지에서 렌더러를 건드릴 필요가
없다. 프레임워크별 색상 매핑의 **정본은 7.4-b 표**이며 여기서 재정의하지
않는다.

> **요청과 정본의 불일치 (확인 필요).** 이 절 추가 요청은 `React=파랑`,
> `V8 Internal=회색`, `node modules=노랑`을 제시했으나, 7.4-b의 확정 표는
> `V8 Internal/GC=주황`, `Bundler Runtime=회색`, `Application(사용자 코드)=초록`
> 이다. 즉 **회색의 대상**(요청=V8 Internal / 정본=Bundler)과 **V8 Internal의
> 색**(요청=회색 / 정본=주황)이 어긋난다. 문서 일관성을 위해 **7.4-b를
> 유지**했고, 새로 등장한 `node_modules`(서드파티 라이브러리)만 배치도에
> `노랑?`으로 잠정 표기했다. 어느 쪽을 정본으로 할지는 정해지면 7.4-b **한
> 곳만** 고친다 — 색 매핑은 문서 내 단일 출처를 유지한다.

`node_modules` 계열을 별도 색으로 분리할지 자체가 미결이다. 6.1절 런타임 계열
규칙에 `node_modules` 경로 매칭을 추가하면 새 카테고리가 되지만, 애플리케이션
코드와 서드파티를 시각적으로 가를 실익이 검증되지 않았다. **7.4-b 표에 정식
행으로 넣기 전에는 배치도의 `노랑?`을 확정으로 읽지 않는다.**

#### 7.8.3 드릴다운 패널 — include / exclude / reroot

좌측의 `DrilldownPanel`을 재사용한다(7.4-e). 이 패널은 현재 `ProfilerAnalyzerPage`
전용이나 결합이 없어 그대로 가져온다. 세 가지 조작을 제공한다.

| 조작 | 의미 | 결과 |
|---|---|---|
| **include** | 특정 프레임을 포함하는 스택만 남김 | 관심 경로에 집중 |
| **exclude** | 특정 프레임을 포함하는 스택을 제거 | 소음(런타임·번들) 제거 |
| **reroot** | 특정 프레임을 새 루트로 승격 | 하위 트리만 재집계 |

`BuildDrilldownStages`(7.6절, 엔진 공유)가 각 조작을 스테이지로 누적하고,
**브레드크럼**이 그 스택을 보여준다 — `(root) › render › reconcile`. 브레드크럼
항목을 클릭하면 그 지점까지 되돌린다(그 이후 스테이지 취소). reroot가 활성이면
브레드크럼 시작점이 `(root)`가 아니라 승격된 프레임임을 명시한다. 드릴다운은
플레임그래프·상세·분류 요약을 **동시에** 갱신한다 — 세 뷰가 같은 스테이지
결과를 소비한다.

#### 7.8.4 분류 요약

6절 분류 규칙과 `componentBreakdown`(5절, `breakdown.go:80`)의 결과를 상단
스트립 + 확장 차트로 보여준다. 카테고리는 **Framework / V8 Internal / User Code
/ Network / GC**이며, 라벨은 7.4-d의 브라우저 어휘를 표시 계층에서 적용한다
(카테고리 상수 자체는 JVM 경로와 공유하므로 건드리지 않는다).

- **상단 스트립**: 가로 100% 누적 바. 항상 보이며 클릭하면 해당 카테고리로
  플레임그래프를 include 필터.
- **확장 차트**: 파이 또는 세로 바(`MetricCard`/차트 컴포넌트 재사용). 비율과
  절대 self 시간을 병기한다 — 비율만으로는 "전체가 느린데 골고루"와 "특정
  카테고리가 지배"가 구분되지 않는다.

색상은 플레임그래프와 **같은 매핑**(7.4-b)을 써서 스트립·차트·그래프가 한
색어휘로 읽히게 한다. 카테고리별 색이 화면마다 다르면 요약의 의미가 사라진다.

#### 7.8.5 타임라인 뷰 (Phase 3)

**Phase 3 의존 기능이므로 메뉴 신설(Phase 4) 시점에는 비활성**으로 둔다
(7.4-c). 9절 Phase 3의 G4 근거상 CPU 샘플만으로는 브라우저 Long Task 경계를
복원할 수 없으므로, UI 문구는 **"롱태스크"가 아니라 "연속 CPU 점유 구간"**을
쓴다. 사용자가 이를 Web Vitals의 Long Task(50ms+) 지표와 동일시하지 않도록
하는 것이 핵심이며, 이 구분은 7.4-c에서 이미 확정했다.

| 요소 | 내용 |
|---|---|
| 시간축 CPU% | `ts_us` 라벨(4.3절·5절)로 만든 버킷별 CPU 점유율 |
| 연속 CPU 점유 구간 | Phase 3 항목 12의 sample run. 강조 표시, 클릭 시 그 구간 스택으로 이동 |
| 브러시 선택 | 가로 드래그로 구간 선택 → 플레임그래프·상세·요약이 그 구간으로 재집계 |

임계값 라벨은 조건부로 둔다. 50ms 등 절대 임계를 표기하되 **"CPU 점유 기준,
브라우저 Long Task와 다름"**을 툴팁에 명시한다. 렌더링 자체는 9절 Phase 3이
보류 상태이므로(항목 12까지만, 렌더는 DevTools 대비 검토 후), 이 절의 타임라인
UI도 그 결정에 종속된다.

#### 7.8.6 상세 패널

우측 패널은 선택된 프레임의 귀속 정보를 보여준다. `Frame` 매핑(4.4절)이 공급하는
필드를 그대로 표시한다.

| 항목 | 출처 |
|---|---|
| 소스 위치 `파일:줄` | `url` + `lineNumber`(4.4절). redaction 규칙(4.4) 적용 |
| self 시간 | 이 프레임 자체의 샘플 |
| total 시간 | 하위 트리 포함 |
| 호출 횟수 | `hitCount` 집계 |
| 카테고리 배지 | 7.4-b 색 + 7.4-d 라벨 |

**소스 위치는 클릭 대상이 아니라 표시 전용이다.** 데스크톱 앱이 임의 경로의
파일을 여는 것은 보안·이식성 문제가 있고, `.cpuprofile`의 `url`은 원격
`http(s)://` 또는 `webpack://` 가상 경로가 대부분이라 로컬에서 열리지 않는다.
경로 복사 버튼만 제공한다. redaction된 경로는 가려진 사실과 규칙을 함께 보여준다
(4.4절).

#### 7.8.7 비교 뷰

두 `.cpuprofile`의 **delta 플레임그래프**를 제공한다. 기존 `DiffPage`가 색을
명시적으로 덮어써 증감을 표현하는 선례(7.4-b가 인용한 `DiffPage` 43-66행)가
있으므로, 이 델타 색 규칙을 재사용한다.

| 델타 | 색 | 의미 |
|---|---|---|
| self 시간 증가 | 적색 계열, 증가폭에 비례한 명도 | 회귀 후보 |
| self 시간 감소 | 청색 계열 | 개선 |
| 변화 없음/신규·소멸 | 중립/점선 테두리 | 구조 변화 |

비교는 **프레임 정체성**(4.4절: `url`+`함수명`+`lineNumber`)으로 두 프로파일의
프레임을 대응시킨다. 익명 함수나 정체성이 불안정한 프레임은 대응 실패로
표시하고 억지로 매칭하지 않는다 — 잘못된 매칭이 없는 회귀를 만들어낸다.
비교 진입점은 툴바의 `⇄ 비교`이며, 두 번째 파일을 요구한다.

#### 7.8.8 툴바

| 항목 | 동작 | 비고 |
|---|---|---|
| `📂 열기` | `.cpuprofile` 파일 열기 | `WailsFileDock` 재사용, accept에 `.cpuprofile`(7.4-e). 드래그 앤 드롭도 동일 진입 |
| `🔍 검색` | 프레임명·파일 경로 검색 | 일치 프레임을 플레임그래프에서 강조 |
| `필터 ▾` | 카테고리·최소 self 시간 필터 | 드릴다운의 include/exclude와 별개인 표시 필터 |
| `⇄ 비교` | 두 번째 프로파일 로드 → 7.8.7 | |
| `내보내기 ▾` | `AnalysisResult` 기반 리포트/차트 내보내기 | 기존 내보내기 경로 공유 |

#### 7.8.9 실시간 모드 없음 — 사후 분석 전용

**이 화면에는 실시간 모드가 없다. `.cpuprofile`을 여는 사후 분석 전용이다.**
HTTP 캡처(SYSTEM_HTTP_CAPTURE.md §8.6.4)가 실시간/사후 두 모드를 오가는 것과
대조된다. 근거:

- 수집은 **Chrome DevTools가 담당**한다. ArchScope는 기록을 붙이는(attach) 도구가
  아니라 저장된 산출물을 분석하는 도구다(1절 비목표·7.4-a 수집 안내).
- CDP 직접 연결은 명시적 범위 밖이다(10절 제약, `.cpuprofile` 라인 57).
- 따라서 녹화 인디케이터·자동 스크롤·유입률 같은 실시간 UI 요소는 **없다.**
  입력은 항상 완결된 파일이고, 화면은 항상 고정된 구간을 보여준다.

이 명시는 사용자 기대를 맞추는 장치다. "브라우저 성능 분석"이라는 메뉴명이
실시간 모니터링을 연상시킬 수 있으므로, 수집 안내 배너(7.4-a)와 이 원칙이
함께 "여기는 기록을 여는 곳"임을 분명히 한다.

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

**(a-1) 크기 가드만으로는 부족하다 (T-564).** (b)의 RSS 체크와 (c)의
다운샘플링은 **이미 `os.ReadFile` + 전체 `json.Unmarshal`이 끝난 뒤** 실행되므로
**파싱 시점 OOM을 막지 못한다.** 게다가 현재 `detect()`의 `jsonHas`는 호출마다
`json.Valid`로 전체를 한 번 훑고 다시 전체를 `map[string]any`로 unmarshal한다
(`parser.go:939-953`). 즉 본 파싱 전에 큰 객체를 한 번 더 만든다. 256MB 일괄
거부는 안전장치일 뿐 "대형 프로파일 지원"이 아니다.

포맷별로 다음을 구분해 설계한다.

| 대상 | 전략 |
|---|---|
| `.cpuprofile` | `os.Stat` / gzip sniff → 제한된 top-level decode 또는 `json.Decoder` 토큰 순회 → nodes 구축 → samples를 읽으며 **온라인 집계** |
| Performance trace | gzip 스트리밍 해제, top-level array와 `{traceEvents:[]}` 양쪽 지원, `Profile`/`ProfileChunk`만 선택 추출 |
| detect | 전체 unmarshal 대신 앞부분 토큰 스캔으로 시그니처 판정 |

**(b) 파싱 중 RSS 체크.** 샘플 루프에서 N개마다 RSS를 확인해 `MaxRSSMB` 초과 시
중단하고 부분 결과 + diagnostic을 반환한다. 부분 결과라도 상위 병목은 대개
드러나므로 전량 실패보다 낫다.

**(b-1) 부분 결과 계약 변경 (T-564).** 현재 `ParseFile`은
`(Parsed, *ParserDiagnostics, error)`를 반환하고(`parser.go:58`) 상위
`Analyze`는 **error가 있으면 결과를 폐기한다.** 따라서 "부분 결과 + warning"을
반환하려면 계약을 먼저 바꿔야 한다. **중단 error**(결과 폐기)와 **부분 완료**
(결과 유지 + 경고)를 명시적으로 분리하고, UI가 부분 결과임을 반드시 표시한다.

**(c) 샘플 다운샘플링.** `samples`가 상한(기본 500,000)을 넘으면 축약한다. 단
**단순 균등 추출 + 개수 배율 보정은 쓰지 않는다** — 건너뛴 구간의 `timeDeltas`가
버려져 총시간과 시간 구간이 왜곡된다. 대신 **건너뛴 delta를 보존하는
bucket/weighted aggregation**을 사용한다. 적용 시 `Metadata.Extra`에
`downsampled: true`와 비율을 기록하고 **UI에 반드시 노출**한다 — 조용한 절삭은
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
3. fixture 추가 (아래 목록)
4. `cpuprofile_test.go` — 스택 복원, 음수 delta, id 비연속, `(idle)` 제외,
   빈 `functionName`, 4.6의 그래프 검증 케이스 전부
5. `cmd_profile.go:41` `--format` 도움말 갱신

#### 완료 기준을 end-to-end fixture로 고정한다 (T-565)

단위 테스트만으로는 G1~G3의 계약 위반을 잡지 못한다. Phase 1 완료 기준에 다음
fixture와 케이스를 포함한다.

- 공식 Chrome Performance trace JSON, JSON.GZ, object/array wrapper (G1 A안 시)
- Node.js `--cpu-prof` fixture, CDP direct profile fixture
- duplicate / missing / cyclic node, `samples`↔`timeDeltas` 길이 불일치
- **동일 basename을 가진 서로 다른 script URL** (4.4 정체성 회귀)
- idle 100%, GC 포함, `samples` 없는 `hitCount`-only profile
- **CLI와 Wails 결과의 단위 / summary / flamegraph parity** (4.3.1 불변식 2)
- **두 `.cpuprofile`의 실제 `DiffPage` end-to-end** (5.0 검증)
- 256MB 경계 아래/위, gzip bomb 방어

실제 브라우저 산출 fixture는 **생성 Chrome/Node 버전과 생성 명령을 함께
기록**하고, URL·소스 내용·사내 식별자를 sanitize한다(4.4 redaction).

**완료 기준**: `profile import`가 **플레임그래프**와 top stacks를 담은
`profile_evidence`를 산출하고, 위 parity/diff 케이스가 통과한다. 이 시점의
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

> **G4 / T-561 — 재설계 필요.** 초안은 인접한 동일 스택을 병합해 50ms 초과
> 구간을 **롱태스크 finding**으로 내려 했다. 이는 의미론적으로 틀렸다. V8
> CpuProfile의 `samples`/`timeDeltas`는 **샘플 시점의 top node와 시각만** 제공하고
> 브라우저 task의 시작/종료 경계는 제공하지 않는다
> ([CDP Profiler.Profile](https://chromedevtools.github.io/devtools-protocol/tot/Profiler/#type-Profile)).
> 한 task 안에서도 스택은 계속 바뀌고, 같은 스택이 서로 다른 task에서 연속
> 관측될 수도 있다. idle 샘플을 기본 제외하면 경계 추정은 더 불가능해진다.
>
> 또한 현재 `collapsedStacks`는 `Sample.Labels`를 **버리고** stack→count로
> 합산하며(`analyzers/profile/analyzer.go:97-110`), `buildTimeline`은
> `[]Sample`이 아니라 이미 합산된 `FlameNode`를 받는다. 따라서 "timeline
> builder가 `ts_us`를 읽는다"는 변경만으로는 구현 자체가 불가능하다.

수정된 항목:

10. **pre-collapse 순서 보존 계약 정의** — 시간 순서 데이터는 합산 flamegraph와
    **별도의 series/table 계약**으로 정의한다. `Sample.Labels`를 내부 전달
    수단으로 쓰더라도 **collapse 이전에 소비**해야 한다. `AnalysisResult` 공통
    contract는 유지한다.
11. 인접 동일 스택 병합 → **sample run**(연속 sampled CPU hotspot) 구간 생성
12. **sampled CPU hotspot finding** — 임계치 초과 연속 구간. **`Long Task`라는
    용어와 finding code를 쓰지 않는다.** 정확한 Long Task는 Chrome trace의
    `RunTask` 이벤트 경계를 정본으로 삼고 그 구간에 CPU 샘플을 귀속하는 방식으로,
    trace 지원 이후에 별도 finding으로 만든다.
13. 시간 구간 렌더링 방식 결정 — 기존 `CanvasFlameGraph` 확장 vs 신규 컴포넌트

**완료 기준**: "기록 3.1초 지점에 `renderList`가 210ms 연속 점유"를 프로파일만으로
지목할 수 있고, 그 표현이 **task 경계가 아니라 샘플 관측 구간**임이 UI와 finding
문구에 드러난다.

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

### Phase 4 — 견고성 및 브라우저 성능 분석 메뉴

14. `ParseFile` 크기 가드 (8-a) — 전 포맷 공통
15. 다운샘플링 + diagnostic + UI 노출 (8-c)
16. 깊이 절삭 (8-d)
17. 대형 fixture(100MB+) 벤치마크를 `PERFORMANCE.md` 기준에 추가
18. **브라우저 성능 분석** 사이드바 그룹 + `BrowserCpuProfilePage` 신설 (7.3, 7.7)
19. 수집 안내 배너, 프레임워크 색상, 브라우저 어휘 라벨 (7.4)
20. `adaptFlameNode` 공용 추출 — 네 번째 중복 방지 (7.4-e)
21. Chrome Trace Event 스트리밍 파서 (3.2, 8-f)

**완료 기준**: 256MB 초과 파일이 OOM 없이 명확한 오류 또는 다운샘플 결과를 내고,
브라우저 담당자가 전용 메뉴에서 수집 안내를 보며 `.cpuprofile`을 열 수 있다.

> 메뉴 신설이 Phase 4인 이유는 **엔진이 먼저이기 때문**이다. Phase 1~3 동안에는
> 기존 `프로파일러` 메뉴와 CLI로 `.cpuprofile`을 분석할 수 있고, 전용 메뉴는
> 표시 계층의 개선이다. 다만 항목 18~19를 Phase 3의 롱태스크 뷰(항목 13)보다
> 먼저 하면 그 뷰가 놓일 자리가 준비되므로, 두 Phase를 병행한다면 **메뉴를
> 먼저** 만드는 편이 낫다. 7.4-c의 롱태스크 하이라이트는 Phase 3 의존이라 메뉴
> 신설 시점에는 비활성으로 둔다.

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
| Lighthouse JSON | 감사 점수·기회 목록을 프로파일과 연결 | 별도 evidence family. 메뉴 자리는 7.5에 예약됨 |
| Web Vitals | LCP/INP/CLS 시계열 메트릭 | 별도 evidence family. 메뉴 자리는 7.5에 예약됨 |
| Performance Insights | DevTools Insights 내보내기 포맷 안정화 시 | Chrome 측 포맷 확정 |
| CDP 직접 수집 | `Profiler.start/stop`으로 ArchScope가 직접 수집 | 원격 디버깅 포트 접근 정책 |
| RUM 연동 | 실사용자 프로파일 샘플 수집·집계 | 수집 인프라. 범위 큼 |
| 서버-클라이언트 상관 | trace ID로 백엔드 프로파일과 프론트 프로파일 연결 | `stitch analyze` 확장 |

마지막 항목이 ArchScope 고유의 차별점이다. DevTools는 브라우저만, APM은 서버만
본다. 하나의 요청에 대해 **브라우저 프레임과 서버 프레임을 같은 시간축에**
놓을 수 있으면 기존 도구가 답하지 못하는 질문에 답할 수 있다. 다만 이는
`.cpuprofile` 지원이 아니라 상관관계 인프라의 문제이며, 본 문서의 Phase 1~4가
그 전제 조건을 만든다.

## 11. 문서화 계획 (T-565)

본 문서는 현재 `docs/ko`에만 있고 `docs/en` 대응 문서와 README 링크가 없다.
`IMPORTER_SUPPORT_MATRIX.md`는 en/ko 양쪽에 존재하지만 `cpuprofile` 언급이
**전혀 없다.** 프로젝트 가드레일상 en/ko 문서는 짝을 이뤄야 하므로 다음 순서를
따른다.

1. **구현 전(현재)**: 지원 매트릭스에 `planned` 상태로만 표기한다. 아직 지원하지
   않는 포맷을 지원 목록에 올리지 않는다.
2. **G1 결정 후**: 확정된 1차 포맷명으로 본 문서 제목과 수집 안내를 정리하고,
   `docs/en/CHROME_DEVTOOLS_CPUPROFILE.md`를 짝으로 추가한다.
3. **구현 완료 후**: `IMPORTER_SUPPORT_MATRIX`, `DATA_MODEL`, `USER_GUIDE`,
   `PERFORMANCE`에 실제 지원 포맷·옵션·크기 제한·다운샘플 정책을 반영한다.

10절의 "CPU만" 제약(네트워크 대기·레이아웃·페인트 미포함)은 사용자 문서에
반드시 노출한다. 6.2의 `EXTERNAL_API_HTTP` 해석 주의도 함께 적는다.

## 12. 개정 이력

| 일자 | 변경 |
|---|---|
| 2026-07-18 | 2026-07-18 Codex 설계 검토 반영. 0절 게이트(G1~G4) 신설, Chrome 저장 포맷 정정(3.0), 측정 단위·시간 귀속 계약 재정의(4.3.1), 프레임 정체성/redaction(4.4), 그래프 검증(4.6), 옵션 소유권(4.7), 데스크톱 단일 경로(5.0), 분류기 이원화 정정(6.0/6.1/6.3), 대형 파일 전략(8), Phase 3 Long Task 의미 정정(9), fixture·문서 계획(9/11) |
