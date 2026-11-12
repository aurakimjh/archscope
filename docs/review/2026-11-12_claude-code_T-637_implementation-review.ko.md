# T-637 파일 기반 perf·제한된 BPF 근거(SystemEvidence) 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-637 구현 커밋 `662b56d`(feat(diagnostics): implement T-637 perf and BPF evidence). 직전 `6033d5b`(T-636 리뷰) 위에 놓이며, 이 커밋은 T-636 리뷰 R14-01의 로컬 수정(표 보호 규칙)도 함께 포함한다.
- 계약 기준: `work_status.md`의 T-637 행과 완료 기록, `docs/ko/SYSTEM_EVIDENCE.md`, `docs/ko/DATA_MODEL.md`·`PARSER_DESIGN.md`·`IMPORTER_SUPPORT_MATRIX.md`의 T-637 절, `docs/ko/VISUAL_ARTIFACTS.md`의 R14-01 절
- 증거: `docs/review/evidence/2026-11-12-t637/local.json`(`source_index_files` 95개와 `evidence_files` 16개의 SHA-256·크기가 `662b56d` 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(HTTP 문서 변경 등)은 수정하지 않았다.
- 방법: 소스 정독(`systemmodel/model.go`, `parsers/systemtrace/source.go`, `analyzers/systemtrace/{analyzer,intervals,project,histogram}.go`, `analysis/system_evidence.go`, `cmd_system.go`, Wails `systemsourceservice.go`, `SystemEvidence.tsx`, `visuals/render.go`), 저장소 밖 scratch worktree(`662b56d`)에서 빌드한 실제 CLI로 격리 상태 루트에서 번들 합성 fixture 3종과 직접 만든 perf script·scheduler·bpftrace 입력(정상·경계·거부·규모)을 `system inspect-source/import-source/project-profile`, `profile-set tree`로 끝까지 실행했다. throwaway Go 테스트로 표 변환을 측정하고 삭제했다.
- 범위: SystemEvidence 1.0.0 / `system_evidence` 0.1.0(perf script sample·scheduler, events-jsonl, bpftrace 0.22.0 histogram JSONL), 구간 짝짓기와 null 상태, ProfileSet 투영, histogram 쪽 질의, 케이스 원본 열기(코드), CLI·Wails 브리지·화면 컴포넌트, T-636 R14-01 수정 확인. 실제 perf/BPF 수집, 네이티브 WebView, Windows, hosted CI는 범위 밖(작성 측도 미관찰)이다.

## 1 결론

**판정: 조건부(P2 2건, P3 4건).** 의미 계약의 핵심은 실제 실행에서 성립했다. 합성 scheduler 추적에서 on-CPU·off-CPU(sleeping/runnable)·run-queue 구간이 정확한 ns로 짝지어지고, 짝 없는 구간·같은 시각 중복·다른 CPU의 겹침은 값을 지어내지 않고 null 상태 행으로 남으며, 바이너리 `perf.data`·`--header`·다른 필드 레이아웃·미지원 BPF 메시지·소유 위치 위조는 모두 명시적 오류로 거부된다. 다만 **문서가 지원한다고 쓴 keyed-map bpftrace histogram이 항상 실패하고**(R15-01), **현실적인 stack 깊이에서는 약 4천 sample 규모에서 모델 한도에 걸려** 문서의 5만 record·1만/2만 sample 표가 얕은 합성 stack에만 해당한다(R15-02). 그 외 P3 4건이 남았다. T-636 R14-01 수정은 의도대로 동작하지만 소수 표기 일부가 새로 영향을 받는다(R15-06).

- **T-637 `DONE` 판정:** R15-01(P2)과 R15-02(P2)를 처리한 뒤 `DONE` 복귀를 허용한다. R15-03~R15-06은 후속 큐에 넣으면 된다. 실제 Linux perf/BPF 수집, 네이티브 Wails/Windows/Office 수용은 작성 측이 밝힌 대로 별도 항목이다.
- **후속 작업과의 관계:** T-639(GPU/serving)와 T-640(LLM trace)이 같은 "모델 전체를 결과에 포함" 패턴을 쓰면 R13-01·R15-02와 같은 용량 문제가 반복된다. 모델 안에서 frame/stack을 중복 제거하는 방식은 새 producer 전에 정하는 편이 좋다.

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| 번들 합성 fixture 3종 | 충족 | perf script(3 sample, entity 2개, ns 시각), events-jsonl(11 record → 구간 12개 중 10개 qualified), bpftrace hist(1 record → bucket 10개). 증거 해시 95+16개 모두 일치. |
| scheduler 구간 짝짓기 | 충족 | 직접 만든 perf scheduler 7줄(A·B·idle): A on-CPU 1.0→1.5 s = `500000000`, sleeping off-CPU 1.5→2.1 s = `600000000`, wakeup→sched_in run-queue 2.0→2.1 s = `100000000`; B가 preempt(`R+`)되면 off-CPU(runnable)와 run-queue가 둘 다 `200000000`. 첫/끝의 짝 없는 쪽은 `unpaired_end`/`unpaired_start`로 null. |
| 불완전 구간은 값을 만들지 않음 | 충족 | 같은 시각 다중 이벤트 `ambiguous_timestamp`, 이전 상태가 열린 채 다음 이벤트가 오면 `ambiguous_sequence`, 손실이 0이 아니거나 모름이면 `loss_unknown`/`loss_affected`, process start 없으면 `entity_lifetime_unqualified`(코드와 fixture). |
| 투영 | 충족 | `off_cpu`/`on_cpu`/`runqueue` 투영이 `profile_evidence` 결과가 되어 `profile-set tree`가 열리고 합계가 구간 합과 일치(off-CPU 800,000,000 = 600 ms + 200 ms). 이전 T-635 경로를 그대로 쓴다. |
| 단위·정확한 정수 | 충족 | 시각은 `1.000000001` s → `1000000001` ns 문자열, period `9007199254740993` 보존, 20자리 정수는 `SYSTEM_BUCKET_NUMBER_INVALID`로 거부. |
| 명시적 거부 | 충족 | 바이너리 `perf.data` → `SYSTEM_PERF_LAYOUT_UNSUPPORTED`, `# header` 줄 → `SYSTEM_PERF_HEADER_UNSUPPORTED`, perf 텍스트를 `events-jsonl`로 읽기 → 형식 오류, capture 없음 → `SYSTEM_CAPTURE_INVALID`, 겹치는 bucket(`[1,3]`,`[3,7]`) → `SYSTEM_BUCKET_OVERLAP`, bpftrace `stats` 메시지 → `SYSTEM_BPFTRACE_MESSAGE_UNSUPPORTED`. |
| 열린 tail histogram | 충족 | `{"max":-1}`, `{"min":0,"max":9}`, `{"min":20}`가 lower/upper null 열린 끝과 정확한 count로 보존된다. histogram은 timeline·분위수·구간 차분에 쓰지 않는다. |
| 실행·네트워크 경계 | 충족(코드) | perf/bpftrace 실행, 권한 변경, 도구 다운로드, symbol 수신 코드가 없다. 작성 측 network-none CLI 관측은 정독했다. |
| T-636 R14-01 수정 확인 | 충족 | `runtime/pprof.StartCPUProfile`, `https://x/y`, `/usr/lib/x.so`, 머리글 `Frame / evidence`가 TSV·HTML 모두 변경 없이 나온다. 수식형(`=1+1`, `@SUM(1)`, `+1`, 단독 `-`)은 TSV·HTML 모두 보호, 분수 `1/3`·`3/4`·날짜 `2024-01-02`·16자리 이상 정수는 TSV만 보호하고 HTML은 텍스트 서식 요청에 맡긴다. |
| 증거 무결성 | 충족 | 위 해시 일치. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R15-01 | P2 | keyed-map bpftrace histogram(문서상 지원)이 항상 raw Go 오류로 거부됨 | `parsers/systemtrace/source.go:272-274` |
| R15-02 | P2 | 현실적 stack 깊이에서 약 4천 sample에 모델 한도 도달, 오류는 규모·한도·조치를 말하지 않음 | `systemmodel/model.go:20-21,353`, `source.go:114` |
| R15-03 | P3 | sidecar의 `original_sha256`이 실제 원본과 대조되지 않고 틀려도 그대로 저장됨 | `systemmodel/model.go:48,138` |
| R15-04 | P3 | perf scheduler 입력은 waker·idle까지 모든 PID/TID 선언이 필요하고 idle(TID 0)이 구간 잡음을 만듦 | `source.go:352`, `intervals.go` |
| R15-05 | P3 | scheduler 기반 투영이 `[unattributed]` 한 프레임이고 off-CPU가 sleeping·preempted를 합침 | `analyzers/systemtrace/project.go:111` |
| R15-06 | P3 | R14-01 수정이 `0.1`·`1.5`·`3.14`·`12.5` 같은 소수를 날짜로 보고 TSV에서 텍스트로 만듦 | `visuals/render.go:54,77` |

## 4 Finding 상세

### R15-01 keyed-map bpftrace histogram 거부 (P2)

**근거**: 문서는 "bpftrace 0.22.0의 flat/단일 keyed map `hist` message만 받는다"고 쓰고 불투명 key 문자열 보존을 약속한다. 구현은 `rows := map[string]json.RawMessage{"": raw}`로 시작한 뒤 값이 `{`이면 같은 map에 `Strict`로 디코드한다(`source.go:272-274`). Go JSON은 기존 map에 키를 **추가**하므로 `""` 항목(객체 전체)이 남고, 이어지는 반복에서 이 객체를 bucket 배열로 해석하려다 실패한다.

**재현**: `{"type":"hist","data":{"@latency":{"nginx":[{"min":1,"max":1,"count":3},{"min":2,"max":3,"count":4}]}}}`를 번들 bpftrace capture로 `system import-source`:
`SYSTEM_SOURCE_INVALID at line 1 byte 0: json: cannot unmarshal object into Go value of type []struct { Min json.RawMessage … }`.
같은 map의 flat 형식은 성공한다. `@x[comm] = hist(...)`처럼 key별 histogram은 bpftrace에서 가장 흔한 형태다. 번들 테스트(`source_test.go`)와 fixture에는 keyed 사례가 없다.

**수정 제안**: 배열이면 key `""`, 객체이면 key별 행으로 분기하고(새 map에 디코드), keyed fixture·테스트를 추가한다. 실제 bpftrace 0.22 keyed 출력 모양을 고정 fixture로 대조하고, 변환 오류는 안정 코드로 감싼다.

### R15-02 stack 깊이와 모델 한도 (P2)

**근거**: record는 stack의 frame을 모두 값으로 들고 있다(address·symbol·module·context, frame당 약 150 B). 모델 한도는 16 MiB(`model.go:20`)이고, 결과는 모델에 더해 events·timeline 표를 담는다. 문서의 "합성 1만/2만 sample 결과는 약 10.5/21.1 MB"는 stack이 2~3 frame인 fixture 기준이며 "깊은 stack은 더 일찍 한도에 닿을 수 있다"고만 적는다.

**재현**(perf script, stack 깊이 25 frame, 8 CPU, 모두 `import-source`):

| 입력 | 결과 |
|---|---|
| 3,000 sample (원본 7.7 MB) | 성공, 결과 13.9 MB |
| 6,000 sample (15.5 MB) | `SYSTEM_MODEL_BUDGET: split original; no records dropped` (line 112,563 ≈ 4,300번째 sample에서 중단) |
| 20,000 sample (51.6 MB) | 같은 오류 |
| 20,000 sample, 깊이 3 | 성공, 26.8 MB |

현실적인 `perf record -F 99 -g` 한 번(8 CPU 30초 ≈ 24,000 sample, 깊이 20~40)은 거부된다. 오류는 sample 수·바이트·한도를 말하지 않으며 "split original"만 안내한다(`source.go:114`). 같은 설계(모델 전체를 결과에 포함)에서 T-638 R13-01을 겪었다.

**수정 제안**: 모델에 frame 표와 stack 표를 두고 record는 stack ID만 갖게 해 중복을 제거한다(ProfileSet이 이미 쓰는 방식). 한도 오류에 실제 바이트·한도·처리한 record 수·"`perf script --time`으로 구간을 나누세요"를 담고, 문서의 용량 표를 stack 깊이별로 적는다. 저장 형식이 바뀌면 payload 버전을 올리고 0.1.0 reader를 유지한다.

### R15-03 `original_sha256` 미대조 (P3)

**근거/재현**: `Capture.original_sha256`은 형식만 검사한다(`model.go:138`, 비교하는 코드 없음). 번들 bpftrace capture에 `"original_sha256":"000…0"`을 넣고 실제 원본으로 가져오면 성공하고 결과의 `quality.original_sha256`에 틀린 값이 그대로 저장되며, 정상 값을 넣으면 그대로 저장된다. 이 필드가 "이 capture가 설명하는 원본"이라는 연결을 뜻한다면 사용자가 불일치를 알 방법이 없다.

**수정 제안**: 값이 있으면 실제 원본의 SHA-256과 대조해 다르면 `SYSTEM_CAPTURE_ORIGINAL_MISMATCH`로 거부하거나 경고한다.

### R15-04 scheduler 입력의 entity 선언 부담과 idle 잡음 (P3)

**근거/재현**: perf script 줄의 header PID/TID와 sched payload의 `prev_pid`/`next_pid`/`pid`는 모두 capture에 선언된 entity여야 하며 하나라도 없으면 가져오기가 중단된다(`source.go:352`, `SYSTEM_ENTITY_REQUIRED`; waker `1/1`을 선언하지 않은 첫 시도가 line 6에서 실패, idle `0/0`도 같은 규칙). 2 CPU 합성 추적에서 idle(`0/0`) entity는 CPU마다 있는 swapper가 같은 TID라 하나로 합쳐져 `ambiguous_sequence`·`unpaired_*` 행이 전체 21개 구간 중 9개를 차지하고, `unavailable_intervals`가 15/21이 된다. 해당 행을 제외하거나 idle을 무시하는 선택이 없다. 문서는 entity 선언 필요성은 설명하지만 idle 처리는 설명하지 않는다.

**수정 제안**: idle/무시 목록을 capture에 선언해(`ignored_tids`) 구간·요약에서 제외하거나, idle을 CPU별 entity로 다루는 규칙을 정한다. 문서에 시스템 전체 추적에서의 선언 방법과 한도를 적는다.

### R15-05 scheduler 투영의 정보량 (P3)

**근거/재현**: perf scheduler 레이아웃(`…,event,trace`)에는 stack이 없어 `on_cpu`·`off_cpu`·`runqueue` 투영의 모든 sample이 `[unattributed]` 한 프레임이다(`project.go:111`). 합성 추적에서 `profile-set tree`의 off-CPU 트리는 루트 아래 `[unattributed]` 노드 하나(800,000,000)다. 문서는 "기존 Top/flamegraph·보고서 이미지를 사용한다"고만 말한다. 또한 `off_cpu` 투영은 sleeping(600 ms)과 preempted-runnable(200 ms)을 합치고 `state`는 label로만 남아 투영 단계에서 선택할 수 없다. run-queue 구간은 off-CPU와 시간이 겹친다(문서도 겹침을 언급).

**수정 제안**: 문서에 "stack을 가진 입력(events-jsonl)에서만 off-CPU 호출 구조가 있다"고 명시하고, 투영에 state 선택(`sleeping`/`runnable`)을 추가하는 후속을 정한다.

### R15-06 R14-01 수정의 소수 표기 영향 (P3)

**근거/재현**: `spreadsheetDate`의 `[0-9]{1,2}[-/.][0-9]{1,2}` 분기가 `숫자.숫자`를 날짜로 본다(`render.go:54,77`). `tableText` 결과: `0.1`→`'0.1`, `1.5`→`'1.5`, `3.14`→`'3.14`, `12.5`→`'12.5`, 반면 `100.25`, `0.001`, `1000000.5`, `-0.5`, `33.4%`는 그대로다. TSV에서 비율·소수 열 일부만 텍스트가 되어 합계·정렬이 열마다 달라진다(HTML은 텍스트 서식만 요청하므로 영향 없음). 이전 규칙(R14-01 이전)은 이 값을 건드리지 않았다.

**수정 제안**: `.`를 날짜 구분자에서 빼고(`-`·`/`만) 소수점은 숫자로 두거나, 날짜 보호를 4자리 연도가 있는 형태로 한정한다. 비율 같은 소수 샘플을 테스트에 추가한다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `662b56d`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 109개 패키지 모두 `ok`(테스트 파일 없는 6개 제외)
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON 해시 111개(소스 95·증거 16) 대조
- 실제 CLI 시나리오: 번들 fixture 3종, 직접 만든 perf script(깊이 25, 3천~2만 sample)·scheduler 7줄·bpftrace(flat·열린 tail·keyed·겹침·미지원 메시지·20자리 정수), capture 오류·`original_sha256` 불일치, 바이너리/헤더/형식 불일치 거부, `project-profile`(off_cpu/on_cpu/runqueue)과 `profile-set tree`, T-636 R14-01 표 변환 35종
- throwaway 테스트·생성 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 Linux perf record/script 수집, scheduler tracepoint 실출력, BPF attach, bpftrace 0.22 실출력(번들 fixture는 합성). 이 환경에서 perf/bpftrace를 실행하지 않았다. 따라서 keyed-map 출력 모양(R15-01)은 구현의 의도와 문서 문구 기준이며 실제 bpftrace 출력과의 대조는 하지 않았다.
- 작성 측 perf 6.1.190 변환 증거와 CC0 `synthetic-perf.data` 재변환은 하지 않았다(증거 정독).
- 케이스 원본 열기(`OpenSystemEvidence`)는 코드 정독만 했고 Wails 앱·실제 case에서 실행하지 않았다.
- 실제 Wails/WebView 화면(`SystemEvidence.tsx`), Windows 실행, 오프라인 syscall 관측 재실행, hosted CI.
- T-636 R14-02~R14-07 후속 큐는 이 리뷰에서 다시 평가하지 않았다.

## 6 권고 처리 순서

1. R15-01 — keyed-map 분기 수정과 fixture. 작은 수정이며 문서와 어긋난 지원 범위를 바로잡는다.
2. R15-02 — frame/stack 중복 제거와 한도 오류 개선. T-639/T-640 전에.
3. R15-03 — `original_sha256` 대조.
4. R15-04·R15-05 — idle/무시 선언, state 선택과 stack 없는 투영 문서화.
5. R15-06 — 날짜 보호 규칙에서 소수점 제외.
