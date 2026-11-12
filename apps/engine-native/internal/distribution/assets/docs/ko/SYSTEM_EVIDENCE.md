# 파일 기반 perf와 제한된 BPF 근거

T-637은 SystemEvidence 1.1.0 모델 위에 `system_evidence` 0.2.0을 추가합니다.
공통 AnalysisResult를 유지하며 parser·analyzer·generic 소비자를 분리합니다.
기존 folded stack·ProfileSet·case/index·Visual Artifact 계약도 유지합니다.
BPF attach, 권한 변경, perf 실행, symbol 수신이나 converter 다운로드를 하지 않습니다.

## 지원 범위와 검증한 표본

| 입력 | 명시적 계약과 검증 범위 |
|---|---|
| `perf-script` sample | `perf script --ns -F comm,pid,tid,cpu,time,period,event,ip,sym,dso`. 자체 생성한 작은 perf.data를 Linux perf **6.1.190**으로 실제 변환했습니다. 호스트 이벤트는 수집하지 않았습니다. PID/TID·CPU·정확한 ns 시각·event·period·root→leaf stack·module/address·미해석 symbol을 보존합니다. |
| `perf-script` scheduler | 별도 fields `comm,pid,tid,cpu,time,period,event,trace`, 명시적 sched_switch/wakeup 정의와 entity map이 필요합니다. 제한된 Linux tracepoint text 문법을 합성 테스트했으며 sample fixture가 실제 scheduler 수집/export까지 검증한 것은 아닙니다. 다른 layout/header는 거부합니다. |
| `events-jsonl` | 명시적 `system-evidence.v1` collector 계약입니다. Sample·sched_out/in·wakeup 또는 block-I/O/syscall/network event와 phase를 선언한 완결 duration을 받습니다. 임의 BPF JSON의 자동 지원을 뜻하지 않습니다. |
| `bpftrace-jsonl` | bpftrace **0.22.0**의 flat/단일 keyed map `hist` message만 받습니다. 고정 upstream hist/lhist 계약과 형태를 대조한 합성 fixture이며 실제 BPF attach는 하지 않았습니다. 정수 min/max는 양끝 포함, 생략한 끝은 열린 tail이며 zero bucket·정확한 count·불투명 key 문자열을 보존합니다. 다른 map/printf/stats/syscall/diagnostic은 거부합니다. |

Sidecar는 schema `system-evidence.v1`, capture ID, tool/version, clock·boot ID·PID namespace,
PID/TID와 nullable process-start를 가진 고유 entity 수명, loss/unwind count(null은 모름),
kernel-symbol 품질, 선택 frequency, event/histogram 정의를 선언합니다. BPF는 script SHA-256도
필요합니다. Duration event는 queue/service/total/observed_local/unclassified 중 phase를 고정하고
각 record와 대조하므로 queue와 service를 몰래 섞지 않습니다. Histogram은 단위·모집단·
cumulative/delta·reset 식별자와 선택 `[from,to)` 구간을 선언합니다. 이 metadata는 선언된 정보이며
생성자 인증이나 수집 완전성의 증명이 아닙니다.

번들의 `system-evidence-*capture.json`과 schema를 참고하세요. 원본 hash는 gzip이면 압축 바이트를
대상으로 하고 line/byte 좌표는 해제한 text 기준이며 CRLF의 byte offset도 보존합니다.
Binary perf.data·folded stack의 잃은 시간 순서·미선언 event·모호한 수명 매핑·다른 producer layout은
현지 변환/검증이 필요합니다. 입력 속 실행 경로나 인자를 실행하지 않고 export options는 출처로만 보관합니다.

## 측정·scheduler 의미

샘플은 record count 1과 선택적인 event별 period를 **별도 metric**으로 보존합니다.
Cycles·event count·명시적 CPU-clock ns는 다른 측정입니다. Frequency를 시간 배수로 쓰거나
multiplex 보정·IPC 비율·서로 다른 event 합계를 만들지 않습니다. 미해석 frame은 주소를 남기고
user/kernel/unknown 문맥을 구분합니다.

같은 capture/clock/entity 수명 안에서 시각으로 정렬하며 원래 좌표를 유지합니다.
Sched-in→sched-out은 실행 endpoint 구간, sched-out→sched-in은 off-CPU,
wakeup→sched-in(또는 runnable sched-out→sched-in)은 run-queue 근거입니다.
중복/같은 시각·비일관/짝 없는 이벤트는 명시적 null 구간입니다. 계산한 scheduler duration에는
선언된 loss 0, 알려진 clock/boot/namespace/수명이 필요하며 on-CPU는 CPU endpoint도 같아야 합니다.
Loss가 모름/0이 아니면 계산을 거부합니다. 한 record에서 완결한 duration은 clock이 알려져 있으면
자체 start/end 차이를 유지하며 다른 관측의 손실과 구분합니다. Gap을 보간하지 않습니다.

Off-CPU는 lock 경합의 증명이 아닙니다. Runnable·sleeping·unknown 상태를 따로 둡니다.
Block-I/O queue/service·syscall 대기·관측된 로컬 network duration을 원격 서비스 latency로 부르지
않습니다. 병렬 entity/겹치는 관측의 비용 합계는 wall time보다 클 수 있습니다. 수치 프로파일에서
제외한 불완전 구간도 부모 결과의 상태 행과 원본 관측에 남습니다.

## 화면 흐름

1. Workspace에서 원본 파일을 고른 뒤 **perf / 제한된 BPF 근거**를 엽니다. 형식과 민감 정보를
   제거한 capture JSON을 입력하고 확인·새 모델 저장 후 `system.evidence-json`, `{}`로 분석합니다.
2. Generic provider에서 events/intervals/histograms 및 event 전용 timeline을 고릅니다.
   Duration에는 ns 단위와 행별 availability/reason을 제공합니다. Histogram은 timeline에 넣지 않습니다.
   Histogram provider/event의 **원래 bucket 분포 보기**는 정확한 count·경계·구간·reset과 전체 보존
   bucket의 최대 count를 기준으로 한 페이지형 막대를 제공합니다. 보존 count를 전체 요청 모집단으로
   해석하지 않으며 생략된 map key는 복구할 수 없습니다.
3. Sample/period/계산 가능한 duration metric과 전체 또는 한 entity 수명을 명시적으로 골라
   **Workspace에 프로파일 생성**을 누릅니다. 소유권이 있는 새 `profile_evidence` 0.2.0 결과에서
   기존 Top/flamegraph·caller/callee·근거/시간 query·보고서 이미지/표를 사용합니다. Clock을 모르는
   sample 투영은 aggregate-only이며 원래 시각은 부모에 남습니다. 비교는 기존 엄격한 capture
   동등성을 유지하고 새 alias/policy 승인을 부여하지 않습니다.
4. 원본과 연결된 case를 저장하고 현재 provider 페이지의 event를 골라 **검증된 케이스 원본 열기**를
   누릅니다. Case/result/source hash와 revision을 대조하고 원본을 재파싱해 event ID/line을 검증한 뒤
   제한된 text 페이지를 엽니다. 조작한 좌표·오래된 참조는 거부합니다. 투영 프로파일은 부모 시스템
   결과에서 검증된 원본을 여세요. Gzip 원본은 해제한 좌표를 보존하지만 raw-text 열기는 거부하므로
   먼저 명시적 archive intake를 사용합니다. 이 재검증은 파일 연결을 검증하며 수집 권한·script 동작·
   물리 clock 정확도를 증명하지 않습니다.

JSON은 저장 바이트를 유지합니다. Generic CSV는 summary/availability이고 HTML/Report Pack은
보존 event/interval/bucket 투영을 사용합니다. Visual Artifact는 현재 선택 generic 표 또는 기존
profile template을 내보내며 새 histogram 이미지 renderer의 검증을 주장하지 않습니다.
Native Wails/Windows/Office와 실제 producer 수집은 별도 수용 검증입니다.

## CLI

```sh
archscope-engine system inspect-source --in perf.txt --format perf-script --capture-file perf-capture.json
archscope-engine system normalize-source --in perf.txt --format perf-script --capture-file perf-capture.json --out model.json
archscope-engine evidence import --in model.json --importer system.evidence-json --options '{}' --out result.json
archscope-engine system project-profile --in result.json --metric-id 'period/cpu-clock:HG' --out profile.json
archscope-engine profile-set tree --in profile.json
archscope-engine evidence export --in result.json --export-format report-pack --out report.zip
```

`system import-source`는 원본 정규화와 결과 생성을 묶습니다. `--entity-id`로 투영 수명을 고릅니다.
새 파일은 기존/관리 경로를 덮어쓰지 않습니다. 기존 snapshot은 바꾸지 않으며 미지원 입력은 명시적으로
거부하고 자동 이관·잘라내기를 하지 않습니다.

원본/해제64 MiB·line1 MiB·모델16 MiB·record50,000·entity1,024·stack depth1,024·참조100만·
record당 bucket4,096/전체10만·result/case64 MiB 한도입니다. 원본 정규화는 동시2건/5분이며
service의 공통 queue/취소 정책도 적용합니다. Canonical 모델은 256 KiB 이하 chunk로 운반합니다.
합성 1만/2만 sample 결과는 약10.5/21.1 MB였고 깊은 stack/label은 더 일찍 byte 한도에 닿을 수 있습니다.
Record 수만으로 모든 입력의 수용을 보장하지 않습니다. 검증 근거는
`docs/review/evidence/2026-11-12-t637/`입니다.

기준: [Linux perf fields](https://raw.githubusercontent.com/torvalds/linux/v6.12/tools/perf/Documentation/perf-script.txt),
[bpftrace 0.22 JSON](https://bpftrace.org/docs/0.22),
[고정 histogram fixture](https://github.com/bpftrace/bpftrace/blob/v0.22.0/tests/runtime/outputs/hist.json).


## R15 저장 형식과 scheduler 한정

SystemEvidence 1.1.0은 frame/stack 공유 표와 record의 제한된 정수 `stack_id` 참조로
중복을 제거한다. 결과 payload는 0.2.0이며 기존 1.0.0 모델과 0.1.0 저장 reader를
유지한다. 과거 byte/hash를 재작성하지 않는다. 로컬 합성 25-frame 20k/24k sample은
모델 5,913,811/7,101,811 bytes, 결과 17,933,051/21,533,051 bytes로 저장했다.
고유 stack·더 깊은 frame·label은 여전히 한도를 소모한다. 한도 오류는 charged bytes·
limit·처리 record 수와 `perf script --time` 구간 분할을 안내한다. Keyed histogram은
새 map으로 분기하고 shape 오류 코드를 고정했으며 0.22.0 formatter source와 합성
형태를 대조했다.

Capture `original_sha256`은 변환 전 provenance 선언(예: binary perf.data hash)이다.
실제 가져온 text/gzip의 `source_sha256`과 다를 수 있고 강제 text hash assertion이
아니다. 현재 waker/idle도 entity 선언이 필요하며 시스템 전체 idle TID 0은 CPU 공유로
모호한 행을 만들 수 있다. 명시적 idle/ignore 정책은 후속 큐다. Scheduler trace
레이아웃에는 stack이 없어 profile은 `[unattributed]` 비용만 보인다. Stack을 포함한
event 입력만 off-CPU 호출 구조를 갖는다. 현재 off_cpu는 sleeping/runnable 비용을
함께 보존하고 state label을 남기며 runqueue와 겹칠 수 있다. State별 선택은 후속이다.

고정 keyed formatter: [bpftrace 0.22 output.cpp](https://github.com/bpftrace/bpftrace/blob/v0.22.0/src/output.cpp).
