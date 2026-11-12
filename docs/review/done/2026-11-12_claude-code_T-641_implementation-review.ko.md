# T-641 Python·Node·Rust·기존 런타임/로그 진단 심화(RuntimeEvidence) 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-641 구현 커밋 `5484b3b`(feat(runtime): add scoped runtime evidence and address T-640 review). 직전 `8f277fc`(T-640 리뷰) 위에 놓이며, 이 커밋은 T-640 리뷰 R17-01~R17-03의 로컬 수정(inference payload 0.3.0, raw carrier 표지 확장, 의미 모델 cache)도 함께 포함한다.
- 계약 기준: `work_status.md`의 T-641 행과 완료 기록, `docs/ko/RUNTIME_EVIDENCE.md`, `docs/ko/LLM_TRACE_EVIDENCE.md`·`INFERENCE_EVIDENCE.md`의 R17 수정 절, `docs/ko/DATA_MODEL.md`·`PARSER_DESIGN.md`·`IMPORTER_SUPPORT_MATRIX.md`의 T-641 절
- 증거: `docs/review/evidence/2026-11-12-t641/local.json`(`source_index_files` 118개와 `evidence_files` 23개의 SHA-256이 `5484b3b` 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(HTTP 문서 변경 등)은 수정하지 않았다.
- 방법: 소스 정독(`runtimemodel/{model,correlation,merge}.go`, `parsers/runtimeevidence/*`, `analyzers/runtimeevidence/*`, `analyzers/multithread/{identity,correlator}.go`, `symbolinfo/symbol.go`, `analysis/semantic_cache.go`, `cmd_runtime_evidence.go`), 저장소 밖 scratch worktree(`5484b3b`)에서 빌드한 실제 CLI. **이 Mac에는 문서가 고정한 것과 같은 CPython 3.14.4·Node v25.9.0·rustc 1.94.1과 OpenJDK 21(21.0.10)이 있어** 번들 fixture 대신 직접 만든 워크로드로 실제 실행을 만들어 독립 대조했다: Python asyncio/tracemalloc/gc 콜백, Node report·perf_hooks event-loop, Rust panic backtrace(v0 mangling, `RUST_BACKTRACE=1`/`full`), 실제 `jstack` 덤프와 `faulthandler.dump_traceback`. 번들 CPU/PostgreSQL 합성 fixture와 직접 만든 변형으로 CPU 연결·실행 계획을 확인했다.
- 범위: RuntimeEvidence 1.0.0 / `runtime_evidence` 0.1.0(Python·Node·Rust·thread-dump·ps CSV·PostgreSQL plan·ProfileSet 참조), CPU/thread 맥락 연결, allocation profile 재사용, GC 구간 합집합, symbol 해석, 기존 multi-dump·thread-dump payload 0.2.0 변경, 의미 모델 cache, CLI·Wails 브리지. 실제 PostgreSQL·py-spy·Memray, 네이티브 WebView·Windows, hosted CI는 범위 밖이다.

## 1 결론

**판정: 조건부(P2 2건, P3 2건).** 실제 런타임 출력으로 확인한 수치는 정확했다. 직접 만든 tracemalloc 할당 3,099,483 bytes·1,124 blocks가 allocation 프로파일의 `total_weight`와 정확히 일치하고, GC 콜백 3구간 합 1,799,000 ns가 `pause_union_ns`와 같으며(겹치는 구간 `[100,200]·[150,250]·[300,310]`은 160 ns로 합집합, concurrent 구간은 분리), Node `monitorEventLoopDelay`/`eventLoopUtilization` 값이 정확한 십진 문자열로 보존되고, Rust v0 backtrace가 14/14·41/41 frame을 복원하며 panic 본문을 담지 않는다. CPU↔thread 연결은 문서의 조건표대로 `qualified_context`/`candidate`/`incompatible`을 정확히 구분한다. 문제는 **기존 분석기 두 곳**이다. 문서가 지원한다고 한 `python_faulthandler` 형식은 이 Mac의 실제 Python 3.14.4 출력(스레드 이름 `[w2]` 포함)에서 **오류 없이 스레드 0개**를 가져오고(R18-01), 기존 `thread-dump analyze-multi`는 상관 맥락을 넣을 방법이 없는 채로 맥락 없는 덤프를 모두 분리해 실제 jstack 2개에서 persistent blocked 1건이 0건으로 사라진다(R18-02). 나머지는 P3 두 건이다. T-640 리뷰 R17-01·R17-02 수정은 실제로 동작한다.

- **T-641 `DONE` 판정:** R18-01(P2)과 R18-02(P2)를 처리한 뒤 `DONE` 복귀를 허용한다. R18-03·R18-04는 후속 큐에 넣으면 된다. 실제 PostgreSQL·py-spy·Memray·네이티브 Windows/WebView는 작성 측이 밝힌 대로 별도 항목이다.
- **방법상 참고:** 이번 리뷰는 처음으로 문서가 고정한 런타임 버전을 직접 실행해 독립 입력을 만들었다. 번들 fixture만으로는 R18-01·R18-02가 드러나지 않는다.

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| Python allocation | 충족 | 내가 만든 `bytearray(1_000_000)`×3 + 소형 500개의 tracemalloc 통계 42건: 정규화 모델의 관측 84건(bytes/count), `runtime allocation-profile --metric live_bytes`의 `total_weight` **3099483** = 원본 합, `live_blocks` **1124** = 원본 합. 가장 큰 호출 경로 3,089,708 bytes. |
| Python GC 구간 | 충족 | 실제 `gc.callbacks` 3구간 합 1,799,000 ns = `pause_union_ns`. 겹치는 pause를 합집합 160 ns, concurrent 280 ns 분리, 역전 구간(`300>200`)·알 수 없는 phase는 `RUNTIME_INTERVAL_INVALID`/`RUNTIME_GC_PHASE_REQUIRED`로 거부, 시계 `unknown`이면 구간 4건이 `unqualified`로 남고 합계는 null. |
| Python task | 충족 | 실제 asyncio task 4개(대기 task 3 + main)가 `pending`, thread 1개. 끝난 task(`failer`)는 snapshot에 없음(원본 API 동작). |
| Node | 충족 | 실제 Node 25.9.0 report + 60 ms 블로킹: `delay_mean_ns 11488256`, `delay_p95_ns 62226431`, `delay_max_ns 62226431`, `utilization_ratio 0.4981686777983156`이 정확한 십진 문자열로 보존. libuv handle 12건이 `active`/`inactive`로 분리(대기 상태 아님). |
| Rust | 충족 | 실제 rustc 1.94.1 `-C symbol-mangling-version=v0` panic: 14개 frame과 `full` 41개 frame이 모두 복원, 재귀 frame은 같은 frame ID를 반복, panic 메시지 본문(`no entry found for key`)은 결과에 0건. 레거시 mangling 컴파일 옵션은 내 환경에서 실행 파일을 만들지 못해 미확인. |
| 실제 JDK jstack | 충족 | 실제 JDK 21.0.10 jstack(스레드 23개, BLOCKED 1·RUNNABLE 9·TIMED_WAITING 3·WAITING 3 + 상태 줄 없는 내부 스레드 7)을 선언된 맥락으로 가져오기: 관측 23건. 상태 줄이 없는 VM/GC 스레드는 헤더 단어(`runnable`/`waiting on condition`)를 상태로 쓴다. 상태가 있는 16개는 jstack과 일치. |
| CPU↔thread 연결 | 충족 | 번들 CPU CSV(tid 500)와 JVM nid `0x1f4`(=500) 쌍: 기본 `qualified_context`, tid 불일치 `native_tid_mismatch`, 창 밖 snapshot `snapshot_outside_cpu_window`, 생애 평균 `lifetime_average_is_not_interval_cpu`(candidate), 시계 불일치 `clock_alignment_unavailable`(candidate), thread 시작 불일치 `thread_lifetime_mismatch`, process 네임스페이스 변경은 병합 단계에서 `RUNTIME_MERGE_ID_CONFLICT`. |
| PostgreSQL 실행 계획 | 충족(합성) | 직접 만든 PG 형태 JSON: 소수 `Actual Rows 2.5/1.25`, `Actual Total Time 12.3456`, `Actual Loops 3`이 정확, 실행되지 않은 노드(loops 0)는 0, `Relation Name` 같은 식별자·SQL은 결과에 0건, 부모/자식 시간은 더하지 않음. 실제 PostgreSQL은 없다. |
| symbol 해석 | 충족(코드) | 유효한 Rust v0/legacy(`17h<16hex>E`) mangling만 `validated_rust_mangling`, `::`만으로 분류하지 않음, 입력 1024 byte 한도. |
| T-640 R17-01 수정 확인 | 충족 | 15초 간격 +100 카운터의 inference 관측이 `interval_rate 20/3`·`rate_unit tokens/second`로 나온다(이전에는 null). |
| T-640 R17-02 수정 확인 | 충족 | Traceloop/Langfuse 형식(`llm.prompts.0.content`, `langfuse.observation.input`) 원본이 `trace import`와 `case edit source`에서 `LLMTRACE_SEMANTIC_REQUIRED`/`CASE_LLM_SANITIZED_SOURCE_REQUIRED`로 거부된다. |
| 증거 무결성 | 충족 | 해시 141개(소스 118·증거 23) 일치. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R18-01 | P2 | 실제 Python 3.14.4 faulthandler 덤프(`Thread 0x… [name]`)가 오류 없이 스레드 0개로 가져와짐 | thread-dump `python_faulthandler` 파서 |
| R18-02 | P2 | `thread-dump analyze-multi`가 상관 맥락 입력 경로 없이 모든 덤프를 분리해 persistent/long-running 탐지가 사라짐 | `analyzers/multithread/identity.go:21` |
| R18-03 | P3 | 시간 구간 하나가 잘못돼도(역전 GC 구간) 파일 전체가 좌표 없이 거부됨 | `parsers/runtimeevidence/python.go` 구간 검증 |
| R18-04 | P3 | 실제 `jstack`의 `nid`가 macOS에서는 10진수인데 파서가 항상 `jvm_nid_hex`로 표기함 | `threaddump/plugins/javajstack/parser.go:251-253` |

## 4 Finding 상세

### R18-01 Python 3.14 faulthandler 덤프 소실 (P2)

**근거**: 문서의 thread-dump 행은 `python_faulthandler`를 지원 형식으로 나열한다. Python 3.14의 `faulthandler.dump_traceback`은 헤더에 스레드 이름을 붙인다(`Thread 0x000000016e023000 [w2] (most recent call first):`). 이 문서가 모든 fixture의 기준 버전으로 고정한 바로 그 Python 3.14.4다.

**재현**: 이 Mac의 Python 3.14.4로 스레드 3개를 만들고 `faulthandler.dump_traceback(all_threads=True)`로 덤프한 뒤 `runtime import-source`(`thread_format: python_faulthandler`):
- 원본 그대로: **`thread_snapshot_observations 0`, `observation_count 0`**, source 1개·entity 0개로 **성공 종료**(오류·경고 없음).
- 같은 파일에서 헤더의 `[w2]`만 제거: 스레드 3개가 정상 가져와진다.

따라서 새 형식이 비어 있는 가져오기가 정상 결과처럼 보인다. "스레드 없음"과 "형식 미인식"이 구분되지 않는다.

**수정 제안**: 헤더 정규식에 선택적 `[name]`을 받아 스레드 이름으로 보존하고, 입력에 `Thread 0x`/`Current thread` 같은 표지가 있는데 스레드가 0개이면 `RUNTIME_THREAD_FORMAT_UNRECOGNIZED`로 거부한다. 실제 3.14 출력 fixture를 추가한다.

### R18-02 multi-dump 상관 맥락의 입력 경로 부재 (P2)

**근거**: payload 0.2.0 multi-dump는 `correlation_context`(host/boot/PID namespace/PID/process start + runtime thread ID + thread start)가 모두 있을 때만 덤프 간 스레드를 연결한다(`identity.go:21`). 이 메타데이터를 설정하는 코드는 저장소 어디에도 없다(읽기만 있음). 기존 CLI `thread-dump analyze-multi --in a --in b`와 화면에는 맥락을 줄 수단이 없다. 문서는 "나머지는 dump별로 분리한다"고 설명하지만 정상 입력에서 항상 그렇게 된다.

**재현**: 같은 JVM(실제 JDK 21)에서 1초 간격으로 뜬 `jstack` 두 개(스레드 각 23개)를 `analyze-multi --threshold 2`로 분석:

| 빌드 | payload | unique_threads | persistent_blocked_threads | long_running_threads |
|---|---|---|---|---|
| 직전 `eadc040` | 0.1.0 | 23 | **1** | 15 |
| `5484b3b` | 0.2.0 | **46** | **0** | **0** |

결과의 `diagnostics.warning_count`는 0이고 `identity_policy`가 메타데이터에 문장으로만 있다. 즉 두 덤프에서 계속 BLOCKED였던 실제 스레드가 지속 차단으로 보고되지 않는다. 이름·stack 일치만으로 지속성을 판정하지 않는다는 설계 의도는 이해되나, 대체 경로가 없고 경고도 없다.

**수정 제안**: CLI/화면에 `--correlation-context`(또는 `runtime`의 options)를 받아 덤프에 붙이는 경로를 제공하고, 맥락이 없으면 결과에 "N개 덤프가 맥락 없이 분리되어 지속성/장기 실행 탐지가 비활성" 경고를 진단으로 남긴다. 이전 이름 기반 연결을 `declared_name_correlation` 같은 명시적 낮은 신뢰 모드로 선택할 수 있게 하는 방법도 있다.

### R18-03 구간 오류가 파일 전체를 좌표 없이 거부 (P3)

**근거/재현**: GC 이벤트 하나의 `from_ns > to_ns`(`300→200`)가 `RUNTIME_INTERVAL_INVALID`로 파일 전체를 거부한다. 오류에는 몇 번째 이벤트인지 좌표가 없다. 알 수 없는 phase(`RUNTIME_GC_PHASE_REQUIRED`)도 같다. 문서의 "조용한 절삭 없이 거부"와는 일치하지만 수백 개 이벤트 중 하나일 때 원인을 찾기 어렵다.

**수정 제안**: 오류에 이벤트 경로(`/gc_events/1`)와 값을 담는다.

### R18-04 JVM `nid` 인코딩 표기 (P3)

**근거/재현**: 파서는 `nid`가 있으면 항상 `native_tid_encoding = "jvm_nid_hex"`로 기록한다(`javajstack/parser.go:251-253`). Linux jstack은 `nid=0x1f4`(16진)이지만 이 Mac의 JDK 21.0.10은 `nid=5891`(10진, 헤더에 `[5891]`)을 출력한다. CPU 연결에서 `jvm_nid_hex`는 16진으로 환산하므로 10진 `nid`를 선언 entity에 그대로 넣으면 TID가 잘못 계산된다. 이번 실행은 entity를 선언하지 않아 연결 오류로 이어지지는 않았다.

**수정 제안**: `nid` 접두(`0x` 유무)로 인코딩을 구분하거나 플랫폼을 선언하게 하고, 문서에 macOS/Windows 차이를 적는다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `5484b3b`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 114개 패키지 모두 `ok`(테스트 파일 없는 9개 제외)
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON 해시 141개 대조
- 실제 런타임 실행으로 만든 독립 입력: CPython 3.14.4(asyncio·tracemalloc·gc 콜백, faulthandler), Node v25.9.0(report·perf_hooks), rustc 1.94.1(v0 panic backtrace 2종), OpenJDK 21.0.10(jstack 2개), 번들 CPU/PostgreSQL fixture 변형 7종, GC 구간 변형 5종
- T-640 R17-01·R17-02 수정 확인
- throwaway 스크립트·생성 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 PostgreSQL EXPLAIN(PostgreSQL 미설치), py-spy 실제 producer, Memray, 실제 `ps`/`top` 수집
- Wails 브리지·Workspace 화면(`RuntimeEvidence.tsx`), 네이티브 Windows, 오프라인 syscall 관측 재실행, hosted CI
- 의미 모델 cache(`semantic_cache.go`)는 코드를 정독했고 설계(digest·버전 키, 2개·2분·96 MiB, 매 요청 소유권·fingerprint 확인)는 타당해 보이나 서비스 프로세스 안의 동작과 쪽당 시간 효과를 실행하지 못했다(CLI에는 프로세스 간 cache가 없다). 레거시 Rust mangling 실행 파일(내 환경에서 컴파일 실패)
- T-640 R17-03~R17-05의 후속 큐 항목은 다시 평가하지 않았다.

## 6 권고 처리 순서

1. R18-01 — faulthandler 3.14 헤더 지원과 "표지가 있는데 0개" 거부.
2. R18-02 — 상관 맥락 입력 경로와 "맥락 없음" 경고(또는 명시적 이름 기반 모드).
3. R18-03·R18-04 — 오류 좌표, `nid` 인코딩 구분.
