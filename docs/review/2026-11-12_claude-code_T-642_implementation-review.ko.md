# T-642 부하·LLM 벤치마크 결과와 JVM 크래시 텍스트 비교(ExecutionEvidence) 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-642 구현 커밋 `efb9c7c`(feat(execution): implement T-642 and resolve T-641 review). 직전 `e80fb62`(T-641 리뷰) 위에 놓이며, 이 커밋은 T-641 리뷰 R18-01~R18-04의 로컬 수정(faulthandler 3.14 헤더, multi-dump 상관 맥락 입력, 구간 오류 좌표, JVM `nid` 인코딩)도 함께 포함한다.
- 계약 기준: `work_status.md`의 T-642 행과 완료 기록, `docs/ko/EXECUTION_EVIDENCE.md`, `docs/ko/RUNTIME_EVIDENCE.md`의 R18 수정 절, `docs/ko/DATA_MODEL.md`·`PARSER_DESIGN.md`·`IMPORTER_SUPPORT_MATRIX.md`의 T-642 절
- 증거: `docs/review/evidence/2026-11-12-t642/local.json`(`source_index_files` 88개와 `evidence_files` 27개의 SHA-256이 `efb9c7c` 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(HTTP 문서 변경 등)은 수정하지 않았다.
- 방법: 소스 정독(`executionmodel/{model,compare}.go`, `parsers/execution/{jtl,benchmarks,crash,source}.go`, `analyzers/execution/*`, `cmd_execution.go`, `cmd_threaddump.go`의 맥락 입력, `threaddump/correlation_context.go`), 저장소 밖 scratch worktree(`efb9c7c`)에서 빌드한 실제 CLI로 격리 상태 루트에서 실행했다. **JMeter JTL 통계는 독립 계산으로 대조했다**(합성 JTL 3만 행을 직접 만들고 Python으로 label별 nearest-rank 백분위·평균·오류 수를 계산해 비교). **실제 HotSpot 크래시 보고서는 이 Mac의 OpenJDK 21.0.10에서 직접 JVM을 비정상 종료시켜 만들었다**(`Unsafe.putAddress(0)`로 SIGSEGV, 1,129줄 hs_err). 비교 조건 변형 8종, 변조 3종, 규모 입력, k6·LLMPerf 번들 fixture를 실행했고 T-641 R18 수정을 실제 jstack·faulthandler로 확인했다.
- 범위: ExecutionEvidence 1.0.0 / `execution_evidence` 0.1.0(k6 handleSummary, LLMPerf summary, JMeter JTL CSV, HotSpot `hs_err` 텍스트), 수집 조건 sidecar, baseline 비교(`execution compare`), 개인정보 제외, CLI·Wails 브리지, T-641 R18 수정 확인. k6·JMeter·LLMPerf 실제 실행(이 환경에 없음), 네이티브 WebView·Windows는 범위 밖이다.

## 1 결론

**판정: 조건부(P2 1건, P3 3건).** 계산·비교 계약은 독립 계산과 실제 크래시 출력에서 정확했다. 3만 행 JTL의 label 3개 × 통계 9종(count·error count·min·max·mean·p50·p90·p95·p99), 총 27개 값이 Python 독립 계산과 **모두 일치**했다(쉼표가 든 label과 줄바꿈이 든 failureMessage 포함). 실제 hs_err에서 호스트명·사용자 이름·경로·환경 변수·명령줄이 결과에 **0건**이었다. 비교는 도착 모델 미상·동시성·워밍업·단계·대상 환경이 다르면 수치 차이를 내지 않고 `incompatible`로 막으며, 같은 조건이면 `-500`, `-100/3`처럼 정확한 값을 내고 baseline 0의 변화율은 null이다. 문제는 **JTL 규모**다. 표본을 모두 개별 레코드로 보관(표본당 약 1 KB)하고 한도가 5만 행이라 일반적인 부하 시험 한 번(예: 10분 × 100 rps = 6만 건)이 `split original JTL`로 거부되는데, 문서 정책상 나눈 파일의 백분위는 합칠 수 없어 올바른 경로가 없다(R19-01). 나머지는 실제 hs_err 파서의 누락·절단과 JTL 타당성 검사 부재 같은 P3이다. T-641 R18-01~R18-04 수정은 모두 실제로 동작한다.

- **T-642 `DONE` 판정:** R19-01(P2)을 처리한 뒤 `DONE` 복귀를 허용한다. R19-02~R19-04는 후속 큐에 넣으면 된다. 실제 k6·JMeter·LLMPerf 실행과 네이티브 Windows/WebView는 작성 측이 밝힌 대로 별도 항목이다.
- **방법상 참고:** 크래시 보고서와 JTL을 직접 만들어 입력하는 방식이 번들 fixture의 정제된 발췌에서 가려진 파서 문제(R19-02)를 드러냈다.

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| JTL 통계 정확성 | 충족 | 독립 계산 대조: label 3개 × 9통계 = 27개 값 전부 일치(count 10,000, error count, min/max, 평균은 정확한 유리수, nearest-rank `ceil(n·p/100)−1` 백분위 p50/p90/p95/p99). label `POST /b, with comma`와 줄바꿈 포함 failureMessage는 CSV로 정상 처리. |
| JTL 개인정보 | 충족 | URL(`http://x/…`)·thread 이름(`Thread Group 1-…`)·label 원문·실패 메시지가 결과에 0건, label은 SHA-256만 남긴다. |
| 실제 hs_err 개인정보 | 충족 | 실제 JDK 21.0.10 크래시(1,129줄): 호스트명(`Mac17,6`)·사용자(`aura`)·`/Users/…`·scratch 경로·`JAVA_HOME`·`PATH=` 모두 0건. 신호 `SIGSEGV`, PID, native TID `6147`(10진), 런타임 `HotSpot`, JRE 버전 `21.0.10+7`, `end_marker_present`가 정확. 끝부분을 자른 파일은 `partial_or_unknown`. |
| 비교 호환성 | 충족 | 같은 조건: `compatible_declarations`, 행 18. 동시성 2→4, 도착 모델 `unknown`, 단계 `mixed`, 워밍업 0→1 s, 대상 환경 hash 변경은 모두 `incompatible`·행 0(수치 차이 없음). run ID만 다르면 호환. |
| 정확한 차이 | 충족 | 표본 1,500→1,000 → `difference -500`, `percent_change -100/3`; baseline 0(오류 0→73) → `difference 73`, `percent_change null`; 한쪽에만 있는 label 지표는 `missing_or_unavailable`(9행). 모든 행에 "observed_difference; no significance claim". |
| 변조 거부 | 충족 | 결과 표의 값 변경, 모델의 context 변경, 모델의 metric 변경 3종이 모두 `EXECUTION_RESULT_CORE_MISMATCH`. |
| k6·LLMPerf fixture | 충족 | k6 1.3.0 handleSummary: `1.449 ms`→`0.001449 s`, `1.1066999999999998 ms`→`0.0011066999999999998 s`(원본 십진 문자열 기반 정확 환산), `error_ratio`는 비율, `requests/second`와 구분, 실행 시간 `1049.628 ms`→`1.049628 s`. LLMPerf: ITL을 TPOT로 바꾸지 않고 successful_requests population으로 분리. |
| T-641 R18-01 수정 확인 | 충족 | 이전 Python 3.14.4 faulthandler 덤프(`[w2]` 포함)가 스레드 4개로 가져와지고, 헤더 없는 텍스트는 `RUNTIME_THREAD_FORMAT_UNRECOGNIZED: no faulthandler thread headers`로 거부. |
| T-641 R18-02 수정 확인 | 충족 | `thread-dump analyze-multi`에 `--correlation-context(-file)` 추가. 맥락 없이 실행하면 `THREAD_CORRELATION_CONTEXT_MISSING` 경고("46 snapshots in 2 dumps isolated … Supply explicit correlation context; names alone never join"). 실제 jstack 2개에 스레드 23개의 `tid`로 만든 맥락을 주면 `unique_threads 23`, `persistent_blocked 1`, `long_running 15`로 이전(0.1.0)과 같은 결과. |
| T-641 R18-03·R18-04 수정 확인 | 충족 | 역전된 GC 구간 오류가 `RUNTIME_INTERVAL_INVALID at /gc_events/0: from_ns=300 to_ns=200`로 좌표를 말하고, jstack `nid`의 인코딩이 `0x` 접두 유무로 `jvm_nid_hex`/`os_tid_decimal`로 구분된다(코드). |
| 증거 무결성 | 충족 | 해시 115개(소스 88·증거 27) 일치. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R19-01 | P2 | JTL 표본을 모두 개별 레코드로 보관(약 1 KB/표본)하고 5만 행 한도라 일반적인 부하 시험이 가져와지지 않으며 나눈 파일의 백분위는 합칠 수 없음 | `executionmodel/model.go:15`, `parsers/execution/jtl.go:84` |
| R19-02 | P3 | 실제 hs_err에서 elapsed가 null이고 native 심볼이 첫 공백에서 잘리며 같은 Java frame이 구역 표시 없이 중복됨 | `parsers/execution/crash.go:57,82-96` |
| R19-03 | P3 | JTL 값의 타당성 검사 부재(epoch 초 단위가 ms로 수용, Latency > elapsed 수용) | `parsers/execution/jtl.go:97-118` |
| R19-04 | P3 | JMeter 5.6.3 외 producer 버전은 우회 경로 없이 거부 | `parsers/execution/source.go` 버전 검증 |

## 4 Finding 상세

### R19-01 JTL 규모와 백분위 합성 불가 (P2)

**근거**: `parseJTL`은 모든 행을 `Sample` 레코드로 모델에 보관한다(`MaxSamples = 50000`, `model.go:15`; `jtl.go:84`에서 초과 시 `EXECUTION_SAMPLE_BUDGET`). label별 통계는 이 레코드에서 계산한다. 문서는 "원본을 분할하세요"라고 안내하지만 같은 문서가 "이미 집계한 percentile을 평균내거나 label을 합치지 않는다"고 하므로 나눈 파일의 지연 백분위는 합칠 수 없다.

**재현**(합성 JTL, label 3개, 오류 약 3 %):

| 입력 | 결과 |
|---|---|
| 30,000 행 | 성공, 결과 30.4 MB(표본당 약 1 KB: 표본 레코드 + timeline 30,000행 + 모델) |
| 60,000 행 | `EXECUTION_SAMPLE_BUDGET: split original JTL` — 행 수·한도·조치 없음 |

JMeter 부하 시험 한 번(10분 × 100 rps = 60,000건)이 이미 한도를 넘는다. 5만 행이라는 한도는 비정상적으로 작지 않지만 이 설계에서는 넘는 순간 **정확한 백분위를 얻는 올바른 경로가 없다**.

**수정 제안**: 표본 레코드를 그대로 두지 말고 label별로 정확한 지연 값 분포(값→횟수의 정렬 표 또는 압축 배열)만 보관해 nearest-rank 백분위를 정확히 유지하면서 표본 수 상한을 크게 올린다. 한도에 닿으면 행 수·한도·"시간 구간별로 나누면 백분위를 합칠 수 없음"을 오류와 문서에 적는다. timeline에서 표본 전체를 반복 저장하지 않게 한다.

### R19-02 실제 hs_err 파서 (P3)

**근거/재현**(실제 JDK 21.0.10 크래시 보고서):
- `elapsed_seconds`가 `null`이다. 파서는 줄이 `elapsed time:`으로 시작할 때만 읽는데(`crash.go:57`) 실제 보고서는 `Time: Thu Nov 12 18:01:22 2026 KST elapsed time: 0.627140 seconds (…)`로 한 줄에 이어져 있다. 별도 줄로 바꾼 파일에서는 `0.62714`가 나온다.
- native frame `V  [libjvm.dylib+0xa2cb34]  Unsafe_PutLong(JNIEnv_*, _jobject*, _jobject*, long, long)+0x124`의 심볼이 `Unsafe_PutLong(JNIEnv_*,`로 잘린다(공백에서 끊음).
- 결과의 frame 18개 중 Java frame 8개가 "Native frames" 구역과 "Java frames" 구역에서 두 번 나오고(예: `C.level3`, line 35와 line 51), 문제 frame(line 9)도 line 30과 중복된다. `Frame`에 구역 정보가 없어 같은 호출이 두 번 있었던 것처럼 보인다.

번들 fixture는 정제한 발췌라 이 형태가 없다.

**수정 제안**: `elapsed time:`을 줄 어디서든 읽고, native 심볼은 공백을 포함한 전체(`+0x…` 앞까지)를 보존하며, `Frame`에 `section`(problematic/native/java)을 추가하거나 중복을 제거한다.

### R19-03 JTL 타당성 검사 (P3)

**근거/재현**: `timeStamp`가 epoch 초(`1700000000`)여도 epoch 밀리초로 수용되어 시작 시각이 `1700000000000000 ns`(1970년)가 된다. `Latency`(first byte) 500 ms가 `elapsed` 100 ms보다 커도 `first_byte 1/2`로 그대로 저장되고 상태가 없다. 음수 `elapsed`는 거부된다. 문서는 "epoch ms를 명시"한다고 하나 값이 그럴듯한지 확인하지 않는다.

**수정 제안**: 시작 시각이 2000년 이전이거나 먼 미래이면 경고, `Latency > elapsed`·`Connect > Latency`이면 해당 표본의 값을 null과 상태로 둔다.

### R19-04 producer 버전 고정 (P3)

**근거/재현**: JMeter 옵션의 `producer_version`을 `5.6.2`로 바꾸면 `EXECUTION_JTL_VERSION_UNQUALIFIED: pinned synthetic 5.6.3 layout`으로 거부된다. 실제 JTL CSV 열 구성은 버전 간에 거의 같으므로 다른 버전의 정상 파일을 받으려면 버전을 `5.6.3`으로 허위 선언해야 한다. 문서는 "다른 producer 버전은 입력/fixture 대기"라고 밝히지만 사용자가 위험을 감수하고 가져올 방법이 없다.

**수정 제안**: 미검증 버전은 `qualification: unverified_version` 경고를 남기고 받아들이는 옵션을 두거나, 열 구성이 필수 열을 만족하면 버전과 무관하게 허용하고 버전 검증 상태를 결과에 표시한다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `efb9c7c`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 116개 패키지 모두 `ok`(테스트 파일 없는 10개 제외)
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON 해시 115개 대조
- 독립 입력·대조: 합성 JTL 3만/6만 행(Python 독립 통계 27개 값 일치), 실제 OpenJDK 21.0.10 크래시 보고서(1,129줄)와 변형 2종, 비교 조건 변형 8종과 비교 결과 3종, 변조 3종, 오류 JTL 2종, k6·LLMPerf 번들 fixture
- T-641 R18-01~R18-04 수정 확인(실제 faulthandler·jstack)
- throwaway 스크립트·생성 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 k6 실행(이 환경에 없음. k6 1.3.0 handleSummary는 번들 fixture 변환만 확인), 실제 JMeter·LLMPerf 실행과 GenAI-Perf·nGrinder·Gatling
- Wails 브리지·Workspace 화면(`ExecutionEvidence.tsx`)의 앱 내 실행, 케이스 snapshot 재열기 비교 경로, 네이티브 Windows, 오프라인 syscall 관측 재실행, hosted CI
- Windows·Linux hs_err 형태(macOS arm64 한 건만 대조)
- T-641 R18의 다른 P3 후속 큐는 다시 평가하지 않았다.

## 6 권고 처리 순서

1. R19-01 — 표본 레코드 대신 label별 정확한 지연 분포 보관과 오류 문구. 모델 크기 한도(16 MiB)도 함께 정리.
2. R19-02 — 실제 hs_err 형태에 맞춘 elapsed·심볼·구역 처리(실제 보고서 fixture 추가).
3. R19-03 — JTL 값 타당성 검사.
4. R19-04 — 미검증 버전 처리 방식.
