# 범위가 있는 런타임 근거

T-641은 공통 `AnalysisResult`, `runtime_evidence` payload 0.1.0,
`runtime_core` 모델 1.0.0을 사용합니다. 제한적 `runtime.evidence-json` importer는
정규 JSON을 받습니다. 원본 지원 UI와 `runtime` CLI는 오프라인 파일을 먼저
정규화합니다. collector, Python pickle/marshal 실행, 힙 객체 그래프, 온라인
SQL 실행은 제공하지 않습니다. 기존 결과 reader는 버전별 의미를 보존합니다.

## 검증한 입력

| format | 고정 근거 | 의미·제약 |
|---|---|---|
| python-runtime-json | CPython 3.14.4 표준 API export, python-runtime-export.v1 | asyncio task snapshot, 명시적 native worker ID, tracemalloc live bytes/blocks, 관측 GC 구간. 지속 대기·GIL 시간·누적 allocation rate가 아닙니다. |
| node-report-json / node-runtime-json | Node v25.9.0 reportVersion 5 / node-runtime-export.v1 | reporting thread의 JS/native stack, 명시적 worker report, libuv handle 목록, perf_hooks event-loop 지연·이용률. 열린 handle은 대기 상태가 아니며 worker ID는 native TID가 아닙니다. |
| rust-backtrace | rustc 1.94.1 실제 panic/backtrace 예제 | 번호가 있는 frame·파일 위치. panic 본문은 제외하며 CPU 측정이 아닙니다. |
| thread-dump | java_jstack (합성 JDK 21.0.8 형식), python_pyspy, python_faulthandler | 기존 안전한 텍스트 parser를 명시적으로 선택합니다. JVM nid는 16진 native TID이며 tid/runtime ID와 구분합니다. Python 휴리스틱은 관측 상태와 구분하고 virtual task는 OS thread로 취급하지 않습니다. |
| ps-l-csv | 합성 pid,tid,cpu_percent CSV | 프로세스 맥락·측정 기준 필수. ps 생애 평균은 후보이며 interval 판정은 실제 구간 평균 자료가 필요합니다. ps 명령을 실행하지 않습니다. |
| postgres-explain-json | 합성 PostgreSQL 18.0 plain/ANALYZE JSON | 추정 row와 실제 row·loops·loop당 총 ms를 구분합니다. 미측정은 null, 관측 0은 0입니다. SQL·predicate·부모/자식 시간 합산은 제외합니다. |
| profile-source-set-json | 검증된 기존 ProfileSet | 명시적 entity/profile 대응과 원본·모델 hash·시계 참조만 유지합니다. CPU 비용 복제나 시계 정렬 추론은 없습니다. |
| runtime-evidence-json | 공유 모델·schema fixture | 엄격한 정규 전송. 최대 16개 모델의 동일 ID 충돌은 거부합니다. |

Python/Node 예제는 합성 작업에서 실제 표준 API를 실행해 생성했습니다. PID·경로·시계
기점은 비식별화·이동했습니다. PostgreSQL/JVM/CPU 예제는 구조·의미 검증이며 실제
DB·OS 수집 증거가 아닙니다. 다른 Node/report 버전, py-spy/faulthandler 실제 producer
버전, Memray·vendor adapter는 별도 capture가 필요합니다. 기술 이름·경로도 가져오기
전에 정제하세요. 원본에는 비밀값이 있을 수 있으므로 case/index에는 정규 모델을 넣습니다.

## 수집 맥락과 CPU 연결

번들의 `runtime-evidence-*-options.json`에 완전한 예제가 있습니다. options는 최대
128 KiB의 엄격한 JSON입니다. format·producer version·clock_id·process identity를
선언합니다. thread-dump는 thread_format도 필요합니다. 선택적 entities sidecar로
runtime/native ID·프로세스/스레드 수명·mapping basis를 선언할 수 있습니다.
없는 맥락은 unavailable로 유지합니다.

CPU와 thread observation 한 쌍을 명시적으로 선택합니다. host·boot·PID namespace·PID·
프로세스 시작, 인코딩을 환산한 native TID, 알려진 동일 thread 시작, 정렬된 알려진
시계, snapshot을 포함하는 CPU 구간, 구간 전체의 thread 수명, observed/declared
mapping이 모두 일치해야 qualified_context입니다. 누락은 candidate, 모순은 incompatible입니다.
이는 선언 맥락 확인이며 출처 인증·인과·GIL 경합·async task CPU 귀속 판정이 아닙니다.

기존 multi-dump payload 0.2.0은 correlation_context의 host_id·boot_id·pid_namespace·
pid·process_start_ns와 runtime/format·runtime thread ID·명시적 thread_start_ns가
모두 있을 때만 연결합니다. 나머지는 dump별로 분리하며 이름·stack 일치만으로 지속성을
판정하지 않습니다. thread payload 0.2.0은 observed state·inferred category·basis를
구분합니다. 과거 저장 payload 해석은 유지합니다.

## 조회와 공통 소비 경로

Evidence에서 원본·정제한 맥락을 확인하고 새 정규 모델로 저장한 뒤 가져옵니다.
추가 자료도 별도로 정규화한 다음 모델을 합칩니다. observation/entity에서 stack/frame
ID를 따라 raw symbol 근거를 확인합니다. 큰 수와 정확한 소수는 Go/JS 사이에서 문자열입니다.

Allocation은 entity·source 하나를 선택해 live_bytes 또는 live_blocks를 소유권이 있는
프로파일로 엽니다. 기존 tree·callers/callees·path filter·시각화·export를 재사용합니다.
block은 count, byte는 bytes입니다. 여러 capture 시점 합산은 거부합니다. 제공한 live
traced allocation이며 전체 프로세스 retained heap이 아닙니다. 취소·reset 뒤 늦게
완료한 결과 lease는 해제합니다. profile 참조는 원본 identity만 유지하고 비용을 더하지 않습니다.

GC pause는 entity/source/clock/population별 구간 합집합이며 concurrent 구간은 따로
표시합니다. 시계·구간 누락은 unavailable/incomplete로 남기고 pause ratio를 만들지 않습니다.
기존 access/TA/DB analyzer는 분리합니다. 정규 모델은 명시적 latency/pool/lock/database
관측을 담을 수 있으나 요청 본문 복제나 stack에서 DB wait 추론은 하지 않습니다.

```sh
archscope-engine runtime inspect-source --in python.json --options-file python-options.json
archscope-engine runtime normalize-source --in python.json --options-file python-options.json --out model.json
archscope-engine runtime import-source --in model.json --out result.json
archscope-engine runtime query --in result.json --query '{"kind":"observations","limit":25}'
archscope-engine runtime merge-models --inputs model.json,other-model.json --out merged.json
archscope-engine runtime allocation-profile --in result.json --entity-id ENTITY --source-id SOURCE --metric live_bytes --out allocation-result.json
```

조회는 observations·entities/processes/sources·frames/stacks·symbol basis·relations·
profile refs·plan nodes를 지원합니다. CPU link는 cpu_observation_id와 stack_observation_id
둘 다 필요합니다. cursor는 전체 selector와 결과 digest에 결합하며 오래된 결과는 거부합니다.
기존 provider·unix 시계 timeline·CSV/JSON/HTML/report pack·case 저장 경로를 재사용합니다.
공통 provider pointer는 기존 tables 계약만 사용합니다.

## 한도·호환성

원본/해제 파일 64 MiB, 정규 모델 32 MiB, 저장 결과 64 MiB입니다. observation 50k,
entity 4096, frame/stack 50k, 전체 frame 참조 100만, stack 깊이 1024, plan node 20k,
source 128, process 1024이며 byte 한도에 먼저 도달할 수 있습니다. 페이지 최대 100행,
UI 25행입니다. 초과는 조용한 절삭 없이 거부합니다. 수집 슬롯 2개·취소 점검·5분 예산을
적용합니다. 부분 자료로 판정하지 말고 원본을 분할하세요.

서비스는 검증한 불변 model/projection을 digest/version별로 재사용합니다. 2개 항목·2분·
보수적 JSON charge 96 MiB 한도이며 매 요청 소유권과 저장 파일 fingerprint/digest를
확인합니다. 반환 DTO는 복사본입니다. CLI 프로세스 간 cache는 없습니다. RSS 메모리 상한이나
native Windows/WebView 실행 수용을 build/static 검사로 주장하지 않습니다.

Rust resolver는 ianlancetaylor/demangle v0.0.0-20261012173340-cbb8aaea4290으로 고정하고
입출력 1024 byte를 제한합니다. 유효한 Rust v0/legacy mangling은 encoded 근거이며
.rs/명시적 언어는 declared 근거입니다. ::나 rust 부분 문자열만으로 분류하지 않습니다.
Display는 raw symbol·source/build identity·frame ID를 대체하지 않으며 해석 실패도
보여줍니다. 의존 코드·license는 오프라인 번들에 포함하고 symbol server를 사용하지 않습니다.


## R18 스레드 입력·명시적 multi-dump 맥락

실제 CPython3.14.4의 Thread/Current thread 헤더와 선택적 [name]을 인식하고 이름을
보존합니다. 인식할 수 없거나 비어 있는 thread 입력은 스레드0개 성공 대신
RUNTIME_THREAD_FORMAT_UNRECOGNIZED로 거부합니다. JVM nid는 0x 접두면 hex,
없으면 decimal OS TID입니다. Python hex runtime ID·JVM tid pointer는 native TID가
아닙니다. Python GC 오류에는 /gc_events/N과 유효한 역전 시간 값을 표시합니다.

CLI thread-dump analyze-multi의 --correlation-context JSON 또는 최대64 KiB의
--correlation-context-file, 데스크톱 multi-dump 옵션의 같은 JSON 필드를 사용하세요.
번들 runtime-evidence-jdk21-correlation-context.json이 구조 예제입니다. schema_version
1.0.0·source_format·host_id·boot_id·pid_namespace·pid·process_start_ns와
threads [{runtime_id,thread_start_ns}]를 선언합니다. 시작 시간은 동일 선언 시계의
canonical uint64 ns이며 thread가 process보다 먼저 시작할 수 없습니다. 미관측/중복 ID·
format 불일치는 거부합니다. 이는 수명/identity 선언이지 producer 인증이 아닙니다.
미지정 snapshot은 분리합니다. 진단에는 맥락 없는 snapshot/dump 수와 해당 관측의
지속성 탐지 제한을 경고하고 multi-dump 화면에서도 표시합니다. 이름으로 대신 연결하지
않습니다. 실제 JDK21 연속 BLOCKED·CPython3.14 fixture를 기존 형식 예제에 추가했습니다.
