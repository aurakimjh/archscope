# T-628~T-645 로컬 진단 확장 구현 종합 리뷰

- 작성: claude-code (독립 리뷰어; 구현·처리와 다른 AI). 이 리뷰는 구현 코드를 수정하지 않았다.
- 대상: `f69daa2` (T-628~T-645 전체와 2026-11-13 리뷰 후속 수정 배치 포함). T-628 첫 구현 커밋(`a698d26`) 직전 대비 `apps/engine-native` 860개 파일, +111,223/−1,352줄.
- 성격: 개별 작업 리뷰(R-01~R22)의 반복이 아니라 **전체를 관통하는 구조·일관성·잔여 범위** 리뷰다. 개별 리뷰에서 이미 확인한 항목은 결과만 인용한다.
- 방법: 대상 커밋 scratch worktree에서 CLI를 새로 빌드해 격리 `ARCHSCOPE_STATE_ROOT`로 실행했다. 의존성은 `go list -deps`, 중복·한도·버전 고정은 소스 전수 검색, 커버리지는 `go test -cover`, 수정 배치는 이전 리뷰에서 만든 재현 입력(실제 hs_err, 실제 Ollama/PostgreSQL/`ss`/PyTorch 캡처, 변형 SQLite)을 다시 실행해 확인했다.

## 1. 결론

**CONDITIONAL — P2 3건, P3 7건, P1 없음.**

- 계산·계약 수준의 결함은 찾지 못했다. 새 스택은 측정값 연산에 `float64`를 쓰지 않고(레이아웃 3곳뿐), 리뷰마다 독립 계산과 정확히 일치했으며, 2026-11-13 수정 배치 중 재현 입력이 남아 있는 R19~R22 항목은 모두 고쳐졌다.
- 남은 문제는 **범위 추적, 현장 적용성, 보장의 회귀 보호**다. (1) 미완 범위가 열린 작업 ID 없이 `DONE` 뒤에 있고, (2) 어댑터가 패치 버전까지 정확히 고정되어 실제 현장 입력 대부분이 거부되며, (3) air-gapped 보장이 CI가 아니라 수동 증거에만 기대고 있다.
- P2 3건은 코드 결함이 아니라 관리·정책 항목이므로 기존 작업의 `DONE`을 되돌릴 필요는 없다. 다만 후속 작업으로 **등록**되어야 한다(RS-01이 바로 그 등록 요구다).

## 2. 검토 포인트별 요약

| # | 검토 포인트 | 판단 | 관련 finding |
|---|---|---|---|
| 1 | 추가 작업·미구현 시나리오 | 미완 범위가 있으나 추적 ID가 없음 | RS-01, RS-02 |
| 2 | 잘못 구현된 것 | 계약 위반 없음. 숨은 한도 결합 1건, 문서 모순 | RS-04, RS-09 |
| 3 | 알고리즘 오류 | 신규 스택에서 발견 없음. 레거시 백분위 정의 불일치 | RS-06 |
| 4 | 아키텍처 정합성 | 분리 원칙 준수. 강제 장치 없음, 예외 2곳 | RS-10, RS-05 |
| 5 | 확장성 | 열려 있으나 비용이 큼(계열 추가 커밋당 약 70~110개 파일) | RS-08, RS-02 |
| 6 | 테스트 커버리지 | 핵심 경로 양호. 모델 패키지·fuzz·UI 공백 | RS-07, RS-05 |
| 7 | air-gapped 호환성 | 현재 코드 기준 충족. 회귀 보호 없음 | RS-03 |

## 3. 전체적으로 확인된 것

| 항목 | 근거 |
|---|---|
| 파서/분석기/익스포터 분리 | `internal/parsers` → analyzers/exporters import 0건. T-628+ 분석기(execution, gputrace, metrichistory, inference, runtimeevidence, systemtrace)는 parsers를 import하지 않는다. 모델 패키지(`*model`, `statistics`, `resultcontract`) → parsers/analyzers import 0건. |
| AnalysisResult 계약 | 결과 타입 descriptor가 `internal/catalog/builtin.go:130` 한 곳에서 등록되고(레거시·compact·distribution 변형 포함), 저장본 구버전 reader가 유지된다. 개별 리뷰에서 변조 입력(표·summary·core·버전) 거부를 계열마다 확인했다. |
| 정확 수치 | T-628+ 모델·파서·분석기 20개 패키지에서 `float64`/`ParseFloat` 사용은 3곳(프로파일 레이아웃 좌표 2, 히스토그램 폭 1)뿐이며 측정값은 정수 문자열·유리수다. |
| 알고리즘 독립 대조(개별 리뷰 누적) | JTL nearest-rank 통계 27+27개, CPU self 216/216, GPU kernel 합/합집합(겹침 변형 포함), PostgreSQL delta 14개와 유리수 rate, `ss -s` 84/84, 메트릭 이력 rate, 선택 창 이벤트 수가 Python/`sqlite3` 독립 계산과 모두 일치. |
| 백분위 정책 | T-628+에서 백분위를 계산하는 곳은 JTL 한 곳이며 `jtl-nearest-rank.v1` 정책명이 결과에 남는다. 다른 계열은 백분위·보간을 의도적으로 하지 않는다(`window_policy` 명시). |
| 수정 배치 재검증 | 실제 hs_err: `elapsed_seconds 0.62714`, 공백 포함 심볼 보존, section 구분(problematic 1/native 9/java 8) — R19-02. epoch 초 타임스탬프·Latency>elapsed 경고 — R19-03. `allow_unverified_version` 명시 선택과 `qualification: unverified_version` — R19-04. Apple `size_vram>size`·`vm_free>capacity`·경로형 태그·대문자 digest 거부 — R20-02. 바이트 한도 안내 — R20-03. Nsight VIEW 거부와 누락 테이블 고지 — R21-02. `epoch_transition_from_unknown` 상태, `numbackends≥1`·reset 역행 거부 — R22-01/02. |
| 네트워크 의존 | `go list -deps`로 parsers/analyzers 전 패키지를 확인: `net/http`를 끌어오는 것은 레거시 `httpcapture`·`httpcorrelation`뿐이고 T-628+ 패키지는 0건, exporters는 0건. 프런트엔드 소스의 외부 URL은 저장소 링크 3개와 테스트용 `external.invalid`뿐이다. |
| 문서 미러 | `docs/{en,ko}`와 배포 번들 미러 12개 문서가 바이트 단위로 동일. |
| 전체 테스트 | 대상 커밋에서 `go test ./... -count=1` 전체 실행: 120개 패키지 `ok`, 실패 0. |

## 4. Finding 요약

| ID | 심각도 | 요약 |
|---|---|---|
| RS-01 | P2 | 미완 범위가 열린 작업 ID 없이 `DONE` 뒤에 있음(TODO 0건) |
| RS-02 | P2 | 9곳의 패치 버전 정확 고정, 탈출구는 JMeter 한 곳뿐 — 현장 입력 대부분 거부 |
| RS-03 | P2 | air-gapped 보장이 회귀로 보호되지 않음(협조적 정책, import 경계 테스트·CI 오프라인 검증 없음) |
| RS-04 | P3 | 모든 모델의 `Strict`가 `metricmodel.Strict`에 위임되어 문서의 64 MiB가 32 MiB에서 다른 도메인 오류로 거부됨; 헬퍼 9벌 중복 |
| RS-05 | P3 | 렌더러가 결과 타입·payload 버전을 하드코딩(Go 레지스트리와 이중 관리); 프런트엔드 테스트 1개 |
| RS-06 | P3 | 레거시 분석기의 백분위 정의 4종이 정책 표기 없이 공존 |
| RS-07 | P3 | 테스트 공백: 모델 패키지 6개·SVG 익스포터 무테스트, 신규 파서 fuzz 없음, 일부 수정분 독립 재검증 없음 |
| RS-08 | P3 | 새 계열 추가 비용이 큼(계열 추가 커밋당 약 70~110개 파일, 계열별 cursor/CLI/서비스/컴포넌트 복제) |
| RS-09 | P3 | 사용자 문서에 작업·리뷰 이력이 누적되어 현재 동작과 모순되는 문장이 남음 |
| RS-10 | P3 | 계층 예외 2곳과 계층 규칙 강제 테스트 부재 |

## 5. Finding 상세

### RS-01 (P2) 미완 범위가 열린 작업으로 추적되지 않는다

- 근거: `work_status.md` 상태표는 TODO 0 / IN_PROGRESS 0 / REVIEW 3 / PENDING 1이고 열린 4건은 모두 Windows 승인(T-601, T-611, T-613, T-627)이다. 그런데 `DONE`인 작업의 제목 범위와 납품 범위가 다르다.
  - T-645 "Qualify additional **profile, serving, DB, OS and APM** adapters": 납품은 PostgreSQL 16.14 `pg_stat_database`와 `ss -s` 두 가지. profile·serving·APM, AWR/ASH/Tibero 등은 없음.
  - T-644 "PyTorch, Nsight and **distributed-inference** detail": PyTorch는 CPU/Gloo 합성 2-rank만 실제 검증, CUDA/NCCL·분산 추론은 구조 테스트만.
  - T-640: Q01(Langfuse/LangSmith) 어댑터 미착수. Q04(OTel/GenAI 별칭) 미결.
  - R20-04는 `EVIDENCE_PARTIAL`(swap/pressure 상승 실제 증거 없음).
- 영향: 이 범위들은 본문 서술("individually gated future adapters remain")에만 남아 있고 작업 ID·우선순위·활성화 조건이 표에 없다. "구현 완료, TODO 0"으로 읽히면 잔여 범위가 유실된다.
- 권고: 잔여 범위를 `PENDING` 작업으로 분리 등록한다(예: D16 CUDA/NCCL 실제 입력, D17 잔여 어댑터군별, Q01, Q04, R20-04). 각 항목에 활성화 조건(실제 입력·버전 선택)을 적고, T-644/T-645 제목 또는 완료 정의에 "bounded first batch"임을 표기한다.

### RS-02 (P2) 패치 버전 정확 고정과 불균일한 탈출구

- 근거(소스 전수 검색): `parsers/gputrace/sqlite.go:144`(Nsight 2024.2.3.38 / schema 3.11.1), `pytorch.go:40`(2.15.0), `summary.go:13`, `parsers/metrichistory/systemdb.go:106`(PostgreSQL 16.14 / 160014), `systemdb.go:214`(iproute2 6.1.0), `apple.go:75`(Ollama 0.40.16), `parsers/execution/source.go:130`(k6 1.3.0), `:142`(JMeter 5.6.3). `allow_unverified_version` 탈출구는 `parsers/execution/source.go` 한 곳(JMeter)에만 있다.
- 재현(개별 리뷰): PostgreSQL 16.13, iproute2 6.2.0, Ollama 0.40.17, Nsight 2025.1.1이 모두 버전 오류로 거부된다.
- 영향: air-gapped 현장은 버전을 고를 수 없다. 패치 릴리스 하나만 달라도 가져오기가 불가능하므로, 정확성은 높지만 실제 적용 범위가 "작성자 장비의 버전"으로 좁아진다. R19-04에서 JMeter에 도입한 정책(실제 버전 선언 + 명시적 `allow_unverified_version` + 결과에 `qualification: unverified_version`)이 다른 8곳에는 적용되지 않아 정책이 계열마다 다르다.
- 권고: 미검증 버전 정책을 공통 규칙으로 올린다. 구조 검증(필수 열·필드·값 형태)을 통과하면 명시적 선택 하에 가져오고, 결과·내보내기·비교에 `unverified_version`을 전파한다. 호환 범위가 확인된 계열은 버전 범위(예: PostgreSQL 16.x의 `pg_stat_database` 열 집합)로 선언한다.

### RS-03 (P2) air-gapped 보장의 회귀 보호가 없다

- 근거:
  - `internal/netpolicy/policy.go`는 `--offline-root`로 켜는 **협조적** 정책이다. 네트워크를 쓰는 코드가 스스로 `RequireOnline`을 호출해야 하며(현재 호출처 10개 파일), 호출하지 않는 새 코드를 막는 장치는 없다.
  - parsers/analyzers/exporters가 `net`·`net/http`·`os/exec`를 import하지 못하게 하는 테스트가 없다(`go/parser`·`go list` 기반 테스트 검색 결과 0건).
  - `scripts/verify-offline-syscalls.py`(network-none syscall 관측)와 브라우저 QA는 `.github/workflows/` 어디에서도 실행되지 않는다. 작업별 `docs/review/evidence/*/offline*.json`은 수동 1회 실행 기록이다.
- 현재 상태: 이 리뷰의 `go list -deps` 확인으로는 T-628+ 패키지에 네트워크 의존이 없다. 즉 지금은 충족하지만, 다음 변경에서 깨져도 CI가 잡지 못한다.
- 권고: (a) 허용 목록 밖 패키지가 `net`, `net/http`, `os/exec`를 직접 import하면 실패하는 아키텍처 테스트 추가, (b) 오프라인 syscall 검증을 Linux CI job으로 편입, (c) 외부 프로세스를 쓰는 변환기(`localconvert`)는 정책 통과를 테스트로 고정.

### RS-04 (P3) `Strict` 위임이 만든 숨은 한도와 헬퍼 중복

- 근거: `executionmodel`, `gputracemodel`, `llmmodel`, `runtimemodel`, `inferencemodel`의 `Strict`는 자체 한도 검사 뒤 `metricmodel.Strict`를 호출하고, 후자는 `len(raw) > MaxBytes(32 MiB)`를 다시 검사한다. `parsers/execution/source.go:215`는 `e.Strict(raw, &v, 64<<20)`로 64 MiB를 넘긴다.
- 재현: k6 fixture에 사용자 정의 metric을 추가해 42.0 MiB로 만든 summary를 가져오면 문서상 한도(64 MiB) 안인데 `METRIC_BUDGET_EXCEEDED: json_bytes=44002728 (limit 33554432); reduce export scope or select fewer series`로 거부된다. 도메인이 다른 코드와 맞지 않는 조치 안내가 나온다(21 MiB 변형은 `JSON key budget exceeded`).
- 구조: `Strict` 9벌, `Hash` 8벌, `Digest` 8벌, `Text` 5벌, `Uint` 3벌이 패키지마다 복제되어 있고 세부가 다르다(`systemmodel.Strict`만 `UseNumber` 없음, 한도 인자 형태 3종).
- 권고: 엄격 JSON 디코딩·정규 정수·해시·텍스트 검증을 공용 패키지(예: `wirejson` 확장)로 올리고 한도는 호출자가 단일 지점에서 준다. 위임 경로의 실효 한도를 문서와 맞춘다.

### RS-05 (P3) 렌더러의 타입·버전 하드코딩

- 근거: `cmd/archscope-app/frontend/src/components/EvidenceResultPanel.tsx:211-218`이 `caps.type === "execution_evidence" && ["0.1.0","0.2.0"].includes(...)` 형태로 8개 계열의 payload 버전을 직접 나열한다. Go 쪽 `resultcontract` 레지스트리와 별도 관리다.
- 이미 발생한 결함: `execution_evidence 0.2.0` 추가 때 이 목록이 빠져 상세 패널이 뜨지 않았고 T-644 처리 중에야 수정됐다(`work_status.md` "desktop action guard now also accepts execution_evidence0.2").
- 테스트: 프런트엔드 테스트 파일은 상태 회귀 1개(`test:state`)뿐이고 컴포넌트 55개에 대한 자동 테스트가 없다. 화면 검증은 작업별 수동 Playwright/mock IPC 증거에 의존한다.
- 권고: 백엔드 capability(지원 동작·패널 종류)를 내려 주고 렌더러는 그것으로 분기한다. 최소한 "레지스트리의 모든 type/payload가 렌더러 분기에 존재"를 확인하는 계약 테스트를 추가한다.

### RS-06 (P3) 백분위 정의가 계열마다 다르고 표기가 없다(레거시)

- 근거: 선형 보간(`statistics.Percentile`, `gclog/analyzer.go:579`, `httpcapture/analyzer.go:584`, `traceimport/analyzer.go:1005`), 내림 인덱스(`databaselog/analyzer.go:260` — `int((pct/100)*(n-1))`, 호출자 슬라이스를 제자리 정렬), nearest-rank(`jenniferprofile/msa_stats.go:349`, JTL), 저수지 표본(`accesslog`).
- 영향: 같은 표본에서 p95가 계열마다 다르다(1..10에서 내림 인덱스 9, 선형 보간 9.55, nearest-rank 10). T-642 비교나 보고서에서 서로 다른 계열의 "p95"가 나란히 놓여도 정책이 드러나지 않는다. T-628~T-645 범위 밖의 기존 코드이나, 새 스택이 정책명을 남기는 것과 대비된다.
- 권고: 레거시 결과에 `percentile_method`를 기록하고, `databaselog`는 공용 구현으로 교체하거나 정책을 명시한다.

### RS-07 (P3) 테스트 공백

- 커버리지(대상 커밋, 문장 기준): `parsers/traceimport` 53.7%, `analyzers/execution` 62.6%, `parsers/profile` 66.4%, `parsers/inference` 67.6%, `netpolicy` 60.0%. 나머지 신규 패키지는 70~88%.
- 테스트 파일 없음: `executionmodel`, `gputracemodel`, `inferencemodel`, `llmmodel`, `runtimemodel`, `systemmodel`(검증 규칙의 본체. 상위 패키지 테스트로만 간접 실행), `exporters/gputrace`(SVG).
- fuzz: `Fuzz*` 테스트는 `capture/halog`, `capture/legacy`, `capture/transparent` 3개뿐이다. 신뢰할 수 없는 입력을 직접 읽는 신규 파서(hs_err 텍스트, JTL CSV, PyTorch JSON, Nsight SQLite, `ss` 요약, faulthandler/jstack)는 fuzz가 없다.
- 독립 재검증: 2026-11-13 배치 중 이 리뷰가 다시 확인한 것은 R19~R22 항목이다. R13-02~07, R14-02~07, R15-03~05, R17-05는 재현 입력이 남아 있지 않아 다시 확인하지 않았다.
- 권고: 모델 패키지에 `Validate` 경계값 표 테스트, 신규 텍스트/CSV/JSON 파서에 fuzz(패닉·한도 초과 없음 불변식), SVG 익스포터에 well-formed·이스케이프 테스트를 추가한다.

### RS-08 (P3) 새 계열 추가 비용

- 근거: T-644 커밋(`09413bf`)은 112개, T-645 커밋(`b36d83e`)은 69개 파일을 건드렸다(직전 리뷰 수정·증거 파일 포함). 계열마다 모델 패키지(+헬퍼 복제), 파서 `source.go`(64 MiB 읽기·gzip 확장·lease 검증이 execution/gputrace/inference/runtimeevidence/systemtrace/traceimport에 각각 구현), 분석기, 페이지 질의·cursor(4개 구현 합계 714줄), CLI 명령 파일(41개), Wails 서비스(19개), TS 브리지, React 컴포넌트, 계약 fixture, en/ko 문서와 번들 미러가 따로 생긴다.
- 좋은 점: 메트릭 이력 계열은 형식 분기(`parsers/metrichistory/source.go:162-172`)만 추가하면 기존 결과·질의·내보내기를 재사용한다(Apple, PostgreSQL, `ss`가 이 경로). 선언형 매핑(`catalog/declarative.go`)도 있다.
- 권고: 원본 획득(크기·압축·고정·lease)과 digest-bound cursor/페이지 질의를 공용 구성요소로 추출하고, 새 계열은 "모델 + 파서 + descriptor"만으로 CLI·Wails·렌더러 기본 표시가 따라오게 한다. 새 로그/프로파일 타입은 우선 기존 결과 타입 재사용 가능성을 검토하는 절차를 문서화한다.

### RS-09 (P3) 사용자 문서의 이력 누적과 모순

- 근거: `docs/en/EXECUTION_EVIDENCE.md:164-166` "R19-02 … R19-03 … and R19-04 … remain explicitly queued"가 바로 아래 문단(수정 완료 설명, `:172`)과 모순된다. `docs/ko/EXECUTION_EVIDENCE.md:142`도 같다. 여러 문서가 "T-644 adds…", "R21-01 is locally fixed…" 같은 작업·리뷰 이력을 사용 설명과 섞어 누적한다. `docs/ko/MSA_SERVLET_DISPATCH.md`는 en 대응 문서가 없다.
- 영향: 번들에 그대로 들어가는 사용자 문서가 현재 동작과 다른 말을 한다.
- 권고: 사용자 문서는 현재 동작만 서술하고 이력은 `work_status.md`·리뷰 문서에 둔다. 모순 문장 2곳을 정리한다.

### RS-10 (P3) 계층 예외와 강제 부재

- 근거: `internal/exporters/gputrace/svg.go:8`이 `analyzers/gputrace`를, `internal/exporters/html/jennifer.go:5`가 `analyzers/jenniferprofile`을 import한다. 익스포터가 공통 `AnalysisResult`가 아니라 분석기 내부 타입에 의존한다. 그 밖의 계층 규칙은 지켜지고 있으나 규율에 의존한다.
- 권고: RS-03의 아키텍처 테스트에 계층 규칙(parsers ↛ analyzers/exporters, exporters ↛ parsers, 예외 허용 목록)을 함께 넣는다.

## 6. 실행한 검증과 미검증 항목

**실행함**
- 대상 커밋 scratch worktree 빌드, 전 패키지 `go test ./... -count=1`(결과: 120개 패키지 `ok`, 테스트 파일 없는 패키지 11개, 실패 0, 종료 코드 0).
- 34개 패키지 `go test -cover`, `go list -deps` 네트워크 의존 확인, 계층 import 전수 검색, 헬퍼·한도·버전 고정 전수 검색, 문서 미러 바이트 비교.
- 수정 배치 재검증: 실제 hs_err, JTL 3종, Apple 변조 6종, 45만 행 JTL, Nsight VIEW/누락 테이블, PostgreSQL 캡처와 변조 3종. 숨은 한도 재현(k6 42 MiB).

**실행하지 않음**
- `-race` 전체 스위트, Wails WebView·Playwright 화면, 네이티브 Windows, hosted CI, 오프라인 syscall 재관측, Mac 패키징·서명.
- R13~R17 후속 수정분의 독립 재검증, 다른 producer 버전·실제 운영 입력, CUDA/NCCL·실제 `nsys`·k6·JMeter·LLMPerf 실행.
- 레거시 분석기(RS-06)의 수치 영향 범위는 코드 정의만 확인했고 결과 차이를 계열별로 실측하지 않았다.

## 7. 권고 처리 순서

1. RS-01 — 잔여 범위를 작업 ID로 등록(문서 작업, 즉시 가능).
2. RS-03 + RS-10 — 아키텍처·네트워크 import 테스트와 CI 오프라인 검증.
3. RS-02 — 미검증 버전 공통 정책.
4. RS-04 — `Strict` 위임 한도 정리와 공용 헬퍼.
5. RS-05, RS-07 — 렌더러 계약 테스트, 모델·fuzz·SVG 테스트.
6. RS-09 — 문서 모순 정리.
7. RS-08, RS-06 — 공용 구성요소 추출과 백분위 정책 표기(중기).
