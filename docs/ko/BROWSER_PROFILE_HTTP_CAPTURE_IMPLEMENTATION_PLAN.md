# 브라우저 프로파일·HTTP 캡처 구현 및 리뷰 게이트 계획

- 작성일: 2026-07-21
- 기준 브랜치: `main`
- 관련 설계:
  - [Chrome DevTools CPU 프로파일 분석](./CHROME_DEVTOOLS_CPUPROFILE.md)
  - [시스템 전체 HTTP 캡처](./SYSTEM_HTTP_CAPTURE.md)
- 영문 짝: [Browser Profile and HTTP Capture Implementation Plan](../en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md)

## 1. 목적과 현재 판정

이 문서는 두 설계의 내용을 다시 정의하지 않는다. 확정된 설계를 실제 구현 순서로
바꾸고, **여러 개의 밀접한 작업을 하나의 리뷰 그룹으로 묶어 승인받은 뒤에만 다음
그룹으로 이동**하기 위한 실행 원장이다.

현재 판정은 다음과 같다.

- **Part A — Chrome/V8 프로파일 분석:** `C-RG1`은 **완료 — PASS**다.
- **Part B — HTTP 캡처 및 분석:** `H-RG1` 오프라인 HAR 분석은 엔진/H-SEC1
  재리뷰, bounded import UI, shared fixture, 전체 엔진/프런트 검증을 포함해
  **완료 — 통합 PASS (2026-07-21)**다. T-571/H-RG2는 2026-07-27 독립
  `H-COV1 PASS`로 닫혔고, T-580/H-RG3도 2026-07-28 독립 `H-SEC2 PASS`로
  닫혔다. T-581/H-RG4 Windows 실시간 UI와 E2E는 **완료 — 그룹 PASS
  (2026-07-30)**다: V1–V3에 한정한 네 번째 독립 재리뷰가 fixture-only 교체
  artifact에 대해 조건 해소를 검증했고, privacy 선언은 이제 공개된 출력에서
  계산되고 contradiction 검사로 강제된다. `H-RG5` HTTP 세션 Diff가
  해제됐다.

## 2. 역할과 소유권

| 영역 | 주 구현자 | 소유 범위 |
|---|---|---|
| 엔진 | **Codex** | Go 모델·파서·분석기·캡처기·세션 store·CLI·Wails API/이벤트·생성 binding·엔진 테스트 |
| UI | **Claude** | React 페이지·상태·상호작용·문구·접근성·시각/상태 회귀 테스트 |
| 통합 | Codex + Claude | 고정된 API 계약, fixture ID, 진단 코드, acceptance scenario를 기준으로 연결 |
| 리뷰 | 구현자와 다른 독립 리뷰어 | 그룹 전체의 정확성·보안·UX·회귀 판정. 본인 구현의 자기 승인 금지 |

Codex가 Wails request/result/event 계약과 생성 binding을 커밋한 뒤 Claude에게
handoff한다. Claude는 생성 binding을 수동 수정하지 않는다. Claude가 UI 구현 중
계약 변경이 필요하다고 판단하면 UI에서 우회하지 않고 Codex에게 계약 변경을
되돌려 보낸다.
남은 그룹에서는 이 경계를 필수로 적용한다. Codex는 백엔드/엔진 계약, binding,
fixture, 엔진 검증까지 끝나면 작업을 멈추고 Claude에게 handoff한다. 사용자가 UI
작업을 명시적으로 재배정하지 않는 한 React/UI/state/i18n/시각 회귀 변경은 모두
Claude가 담당한다.

## 3. 리뷰 게이트 운영 규칙

1. 각 그룹 안에서는 개별 작업을 자유롭게 커밋하되, **그룹 전체 acceptance가
   준비됐을 때 한 번 리뷰**한다.
2. 그룹 순서는 `Codex 엔진 계약/구현 → binding과 fixture handoff → Claude UI →
   공동 검증 → 독립 리뷰`다. UI가 없는 그룹은 엔진 구현 뒤 바로 공동 검증으로
   간다.
3. 리뷰 판정은 `PASS`, `CONDITIONAL`, `FAIL` 세 가지다. `CONDITIONAL`은 통과가
   아니며 지적사항 반영과 재리뷰가 끝나기 전까지 다음 그룹을 시작하지 않는다.
4. 리뷰 결과가 `docs/review/`에 들어오면 저장소의 `AGENTS.md` 절차대로 모든
   미처리 리뷰를 `work_status.md`에 반영한 뒤 `docs/review/done/`으로 옮긴다.
5. 그룹 리뷰 자료에는 최소한 구현 commit, 변경 계약, 실행한 검증, 알려진 제한,
   미구현 범위가 들어가야 한다.
6. 그룹 안 병렬 작업은 허용하지만 API 계약이 고정되기 전에 UI가 추측 구현을
   시작하지 않는다.

### 개별 리뷰가 필요한 예외

다음 항목은 오류 비용이 크므로 그룹 리뷰까지 기다리지 않고 **의존 작업 전에
개별 리뷰를 통과**해야 한다.

- `H-SEC1`: 악성 HAR 자원 제한과 JWT/cookie/header/query/body 리댁션
- `H-COV1`: T-571 ETW/WFP/TCP-owner 증거와 coverage ratio 공개 여부
- `H-SEC2`: CA 생성·신뢰 등록·제거, upstream TLS 검증, privilege 경계
- `C-SEM1`(선택 확장 착수 시): trace `ph:"X"`에서 만드는
  `BROWSER_LONG_TASK` 의미와 CPU sample 귀속

## 4. 전체 실행 순서

| 순서 | 리뷰 그룹 | 상태 | 다음 그룹 진입 조건 |
|---:|---|---|---|
| 0 | `PLAN-RG0` 실행 계획 | **완료** | 본 문서와 `work_status.md`가 일치 |
| 1 | `C-RG1` Chrome/V8 릴리스 구현 승인 | **완료 — PASS (2026-07-21)** | 독립 리뷰 `PASS` |
| 2 | `H-RG1` HAR 오프라인 분석 완성 | **완료 — 통합 PASS (2026-07-21)** | 종료 |
| 3 | `H-RG2` Windows coverage proof | **완료 — H-COV1 PASS (2026-07-27)** | 종료 |
| 4 | `H-RG3` 실시간 캡처 엔진 기반 | **완료 — H-SEC2 PASS (2026-07-28)** | 종료 |
| 5 | `H-RG4` 실시간 UI 및 Windows E2E | **완료 — PASS (2026-07-30)** | 종료 |
| 6 | `H-RG5` HTTP 세션 Diff | 준비 완료 — `H-RG4 PASS` 충족 | `H-RG4 PASS` |
| 7 | `X-RG1` HTTP × 프로파일/서버 증거 교차 분석 | 계획 | `H-RG5 PASS` |
| 8 | `R-RG1` 통합 릴리스 승인 | 계획 | `X-RG1 PASS` |

두 기능을 같은 커밋에 섞지 않는다. 단, `X-RG1`은 두 기능을 연결하는 것이 목적이므로
예외다.

## 5. Part A — Chrome/V8 프로파일 분석

### C-RG1 — 현재 릴리스 구현 승인

**상태:** 완료 — `PASS` (2026-07-21). 새 기능을 더 넣는 그룹이 아니라 이미 완료된
T-558~T-565를 새 리뷰 정책으로 승인한 그룹이다.

#### Codex 엔진 리뷰 범위

- [x] Chrome Performance trace `.json`/`.json.gz`와 V8 `.cpuprofile`/gzip 정규화
- [x] microsecond `int64` 단위, graph/time 불변식, hitCount-only 정책
- [x] bounded gzip/JSON streaming, 256 MiB guard, 500k weighted downsampling
- [x] source-aware frame identity·redaction·category/color
- [x] pre-collapse `cpu_sample_runs`/`cpu_activity`와 `SAMPLED_CPU_HOTSPOT`
- [x] `AnalyzeProfileEvidence` 단일 경로, Diff·Workspace·Export 연결
- [x] shared 15-fixture manifest 골든 테스트와 CLI/Wails parity

#### Claude UI 리뷰 범위

- [x] `BrowserCpuProfilePage` 수집 안내와 지원 확장자
- [x] sampled CPU run을 브라우저 Long Task로 오인하지 않는 문구
- [x] partial/downsample diagnostic 노출
- [x] flamegraph·drilldown·workspace 흐름
- [x] 독립 리뷰에서 발견되는 UI 회귀나 접근성 지적사항 반영

#### 그룹 PASS 기준

- 15개 shared fixture의 format/diagnostic/finding/duration golden이 통과한다.
- `.cpuprofile`과 trace 쌍의 3.1s 지점 210ms `renderList` run이 동일하게 나온다.
- downsample/hitCount-only 입력에서 시간축 주장을 하지 않는다.
- frontend state test와 production build, Go test/build, 로컬 package smoke 증거가
  재현 가능하다.
- 리뷰 문서가 "CPU sample run ≠ browser Long Task"와 bounded-result 계약을
  명시적으로 승인한다.

### C-EXT1 — 정확한 Chrome duration event 분석(선택 확장)

현재 CPU 프로파일 릴리스의 blocker가 아니다. 별도 릴리스 목표로 승격할 때만
착수한다.

- **Codex:** trace `ph:"X"`를 bounded streaming으로 모델링하고 renderer/process
  선택, `RunTask` 기반 `BROWSER_LONG_TASK`, Layout/Paint 구간, CPU sample 귀속을
  구현한다.
- **개별 게이트 `C-SEM1`:** task 경계·시간 귀속·다운샘플 억제 의미를 승인한다.
- **Claude:** Long Task/Layout/Paint overlay와 renderer 선택 UI를 구현한다.
- **그룹 리뷰:** 기존 sampled CPU 문구와 진짜 duration event가 UI·finding code에서
  섞이지 않는지 검증한다.

## 6. Part B — HTTP 캡처 및 분석

### H-RG1 — HAR 오프라인 분석 완성

Phase 1을 "MVP가 존재한다"에서 "설계 acceptance를 충족한다"로 올리는 첫 구현
그룹이다.

#### Codex 엔진 — 완료 (2026-07-21)

- [x] `CaptureTransaction`/timing state/fidelity 계약을 실제 Go 모델에 반영한다.
- [x] HAR 파서를 `detect → structural validate → dialect → normalize → model map →
  redact → analyze` 단계로 분리한다.
- [x] BOM, malformed/deep/oversized JSON, entry/string/body 수 제한과 deterministic
  diagnostic을 구현한다.
- [x] Chrome/Firefox/Safari/Charles/Fiddler/Proxyman/Insomnia/generic 방언을
  `dialect.go`의 1급 계약으로 분리한다.
- [x] `../projects-assets/test-data/har-fixtures/` manifest를 읽는 골든 테스트를
  연결하고 20개 합성 fixture의 dialect/diagnostic/redaction assertion을 고정한다.
- [x] URL뿐 아니라 header/query/cookie/JWT/body/process metadata까지 전용
  `capture/redact` 정책을 적용한다.
- [x] summary/series/table을 bounded `AnalysisResult`로 유지하고 HAR 상세 행의
  inline 상한과 잘림 diagnostic을 고정한다.
- [x] CLI·Wails 결과 parity와 실제 Chrome/Firefox export fixture 보강 절차를
  검증한다.

#### 개별 게이트 H-SEC1

악성 HAR resource-limit 테스트와 SEC-4~SEC-7 리댁션 테스트가 통과하고, 민감정보가
diagnostic·finding·export·Workspace에도 재등장하지 않는다는 리뷰가 `PASS`여야 UI
상세/내보내기 연결을 진행한다.

2026-07-21 remediation 재리뷰는 `PASS`를 반환했고 원 P1/P2/P3 finding은 모두
닫혔다. Phase 2+ SEC-8/10/16/17 구현 실측은 H-SEC2에 남으며 이 offline-import
게이트를 다시 열지 않는다.

#### Claude UI — 완료 (2026-07-21)

- [x] `(HAR 가져오기)` pseudo-process tree와 요약 카드
- [x] 전체 타임라인, brush 선택, 선택 구간 재집계
- [x] 트랜잭션 목록과 request/response/timing/process 상세 탭
- [x] method/status/host/path/MIME/duration/error/fidelity 필터
- [x] dialect·fidelity·redaction·parser diagnostic과 degenerate timestamp 안내
- [x] bounded row 렌더링, 빈 상태, 실패/부분 결과, Workspace 등록 회귀 테스트

#### 그룹 PASS 기준

20개 manifest fixture와 최소 2개 sanitized real export가 통과하고, UI에서 timeline
선택과 필터가 같은 분모를 사용하며, import-only 기능을 live capture로 오인시키는
버튼이나 문구가 없어야 한다.

통합 점검은 의도적으로 bounded인 inline-row 분모를 승인했다. 카드·목록·트리는
동일한 필터 행을 사용하고 UI는 이를 전체 세션 필터 합계가 아닌 하한으로 명시한다.
populated state, provenance, Workspace, typed component wiring, production build 증거로
Phase 1 UI 게이트를 닫았고 더 깊은 Wails component fixture는 비차단 hardening으로
남겼다. 전체 Go test/vet/build와 frontend state/build 검증은 모두 통과했다.

### H-RG2 — Windows coverage proof (T-571)

이 그룹은 중요한 단일 증거 작업이므로 그룹 자체가 `H-COV1` 개별 리뷰 역할을 한다.

#### Codex To-Do

- [x] Windows real-NIC target으로 ETW CAP-1/CAP-4를 재실측한다.
- [x] 실 NIC WFP allow-path attribution을 재실측하고 measured configuration의
  미지원 disposition을 기록한다. ALE audit policy는 활성화하지 않았으므로
  audit-enabled capability를 주장하지 않고 제품 coverage 후보에서 제거한다.
- [x] PowerShell polling 대신 직접 `GetExtendedTcpTable` 호출로 CAP-5 CPU overhead를
  재측정한다.
- [x] CAP-1~CAP-6 판정과 capability/fidelity matrix, source ledger를 갱신한다.
- [x] 실패한 scope의 absolute coverage ratio를 제거하고 self-observed five counters만
  남긴다.

**PASS 기준:** false attribution 0, 측정 재현 절차와 증거 tier/한계가 정직하게
기록되고, UI에 노출 가능한 값과 노출 금지 값이 명확히 승인돼야 한다.

2026-07-27 첫 독립 검토는 `CONDITIONAL`이었다. 보완 후 `CAP-3 N/A`인 TCP-owner는
개별 persistent endpoint 귀속만 허용하고, absolute coverage ratio는 금지하며
`counter_fallback: true`로 5개 self-observed 정수 카운터만 유지한다. 커밋된
observation은 정규화 증거 tier이며 source-level 원본 패키지는 아니다. CAP-6의
helper 수명·권한·IPC·설치 계약은 H-SEC2/H-RG3로 이관했다. 같은 날 독립
재검토가 모든 COV-1~COV-6 보완을 확인하고 `PASS`를 반환했다.

### H-RG3 — 실시간 캡처 엔진 기반

#### Codex 엔진 To-Do

- [x] session state machine과 idempotent start/stop/recovery API
- [x] append-only NDJSON/blob/manifest store, rebuildable index, versioned cursor paging
- [x] byte-bounded write/live/aggregate 3단 buffer와 disk slow/full 정책
- [x] captured/persisted/bodyOmitted/eventSkipped/kernelDropped/parseFailed counters
- [x] H1 semantic MITM + H2 passthrough `Proxy`/`Interceptor` production path
- [x] Windows direct TCP-owner process attribution과 짧은 연결의 불확실성 표시
- [x] live completion-order와 file replay aggregate parity
- [x] Wails `CaptureService`, sequence/snapshotVersion events, snapshot recovery
- [x] CA lifecycle, upstream TLS verify-always, 승인 기반 scoped passthrough

2026-07-27 구현과 자체 검증을 완료했다. Windows 현재 사용자 ROOT trust
backend는 설치 실패 rollback과 역순 제거를 사용하며, 공개 인증서 제거 기록을
owner-scoped 앱 저장소에 남겨 crash 후에도 정리할 수 있다. 프록시는 loopback에만
bind하며 upstream TLS 검증을 끌 수 없다. H2-only ALPN과 명시 승인 host는
`unsupported` passthrough 기록을 남긴다. 독립 `H-SEC2` 검토는 2026-07-28
`PASS`를 반환했으며, 이에 따라 H-RG3과 T-580이 완료되고 T-581 / H-RG4가
착수 가능해졌다.

#### 개별 게이트 H-SEC2 — `PASS` (2026-07-28)

CA 개인키 저장, trust-store 부분 실패 rollback, 제거/만료, upstream 검증, pinning
진단, passthrough scope/expiry, privilege IPC가 SEC-1~SEC-16 해당 항목을 통과했다:
모든 저장 이전에 redaction이 실행되고 평문 body는 저장되지 않으며, CA 개인키는
메모리 전용·비내보내기이고, 세션 파일은 owner-only, trust 제거는 트랜잭션적이며,
upstream TLS는 항상 검증되고, 프록시는 loopback 전용이며, CLI/headless 캡처 시작
경로가 없다. 두 조건이 게이트를 다시 열지 않으면서 다음 tier를 구속한다:
SEC-10 crash-dump 제외 preflight는 body 캡처 tier 이전에 선행해야 하고,
미귀속(SEC-17) 보존은 live UI가 저장 트랜잭션을 노출하기 전에 명시적
metadata-only opt-in 뒤에 두어야 한다.

#### 그룹 PASS 기준

disk full, crash recovery, event loss/re-entry, CA failure, pinning, cancellation,
streaming, H2 passthrough fixture와 long-session memory bound가 통과해야 한다.

### H-RG4 — 실시간 UI와 Windows E2E

**상태:** **완료 — 그룹 `PASS` (2026-07-30)**. 2026-07-29 세 번째 독립
재리뷰가 네 번째 `CONDITIONAL`을 반환했고, 코드 측 S1–S9 조건 검증 후
수정된 하네스가 거부된 artifact를 교체했다. 교체본은 source 1,023행 중
loopback fixture 1,012행만 보관하고 백그라운드 11행을 제외했으며,
confirmed attribution의 fixture pinning 실패 1행과 빈 contradiction 집합을
포함한다. SHA-256은
`69565684d57b20d763ed477f731a9eb836bcc8fbde657cdff10bce0085030111`이다.
2026-07-30 V1–V3에 한정한 네 번째 독립 재리뷰가 조건 해소를 검증하고
게이트를 닫았다
(`docs/review/done/2026-07-30_claude-code_H-RG4_windows-live-capture-ui-e2e-fourth-re-review.md`).
T-582는 해제됐고, 보류된 resolver 비용 실측은 `R-RG1`에서 수행한다.

#### Codex 통합

- [x] 고정된 CaptureService binding과 Windows E2E harness 제공
- [x] 엔진 snapshot/cursor/filter semantics를 UI acceptance fixture로 제공
- [x] 패키지/서명/권한 경계 smoke 지원
- [x] L2: redaction을 동시성 안전하게 만들고 stream race test로 고정
- [x] L1: passthrough progress에 비-semantic fidelity를 내보내고 stop-mid-tunnel 검증
- [x] L3: acceptance fixture를 제품 상수/transaction에 연결하고 Windows harness가
  캡처 row/stats를 readback하며 필수 client 부재 시 실패하도록 개선
- [x] L4–L7 백엔드 계약: bounded progress batch, 진행 행 terminal reconciliation,
  활성 SEC-17 정책 공개, observed/drop counter
- [x] L9/L11/L13: manager에서 platform availability 강제, CONNECT 가상 path 제거,
  confirmed attribution 보장 범위 문서화
- [x] R1/R5: 저장 transaction에서 finalized live provenance와 최약 fidelity를
  계산하고 capture-time redaction summary를 저장하며 HAR provenance는 HAR
  경로에만 유지
- [x] R3/R4: TLS handshake 실패를 attribution이 보존된
  `proxy_not_captured`/`unsupported`로 기록하고 tunnel 실패에는 실제
  `proxy_passthrough`/`unsupported`와 process 기반 coverage 유지
- [x] R6/R7: accepted client connection마다 TCP-owner attribution을 한 번만
  계산하고 두 번째 owner-table 조회에서 PID와 process start time을 모두 확인
  - R6 resolver 비용 실측은 H-RG4의 correctness/privacy gate에서 보류하고
    `R-RG1` Windows 통합 성능 확인으로 이관한다. 호출 수는 connection당 1회로
    bounded되고 현재 long-session acceptance가 통과했으며, 별도 실측은 UI·client
    부하와 분리된 전용 Windows run이 필요하기 때문이다.
- [x] R8 백엔드: renderer row cap, event-skip resync, page 재진입, finalized
  handoff를 버전된 `LiveCaptureContract`로 노출하고 fixture를 Go 계약에 연결.
  production state 소비는 Claude가 담당
- [x] R12 백엔드: terminal `aborted` 행을 저장하고 final stats, aggregate,
  analysis, acceptance evidence에 포함
- [x] R2 하네스와 교체 artifact: acceptance WebView2 CDP port와 실제 recovery
  session을 필수화하고 h2-only 및 명시적 CONNECT pinning, 장시간 세션, page
  재진입을 실행한 뒤 모든 시나리오를 제품 store에서 readback. 수정 실행은 빈
  contradiction 집합과 갱신된 체크섬을 가진 fixture-only schema-v4 증거를 보관
- [x] S2: 중지된 모든 진행 행을 `aborted` / `unsupported` terminal 등급으로
  바꾸어 finalized store에 `pending`이 남지 않도록 처리
- [x] S3: persisted 행과 같은 store flush lifecycle에 capture stats와 known
  redaction summary를 checkpoint. 구형 manifest는 저장 행 count와 명시적
  unknown redaction으로 보수적으로 복구해 거짓 0/clean 데이터를 금지
- [x] S5–S8: fixture와 PowerShell을 schema-v4 harness 계약에 연결하고
  loopback fixture origin만 허용, local path 제거, row cap, owner-only artifact
  ACL, explicit-proxy QUIC 비가시성, locale 독립 page 재진입과 제품 row 대조를
  강제
- [x] V1 하네스: Edge/Electron을 임시 프로필과 background-networking 차단
  플래그로 실행하고, 공개 `capture.rows`를 loopback fixture 행으로 제한하며
  `fixtureTrafficOnly`, source/archive/제외 행 수, local-path 부재를 실제 출력에서
  계산. fresh 실행은 fixture 1,012행을 보관하고 백그라운드 11행을 제외했으며
  local path와 장시간 세션 원문 secret을 포함하지 않음
- [x] S9: 사용되지 않는 generic `httpcapture.Build`를 제거해 live 행이 HAR
  provenance 경로로 다시 들어갈 수 없도록 처리

Codex는 위 백엔드 계약, 생성 binding, fixture, 엔진 검증을 고정한 뒤 UI로
인계한다. read-only `http-capture acceptance-evidence` 명령은 종료된 제품
session에서 bounded metadata를 owner-only 권한으로 내보낸다. Windows harness는
네 client의 HTTP/HTTPS뿐 아니라 h2-only, pinning, 장시간 세션, page 재진입,
recovery 증거를 필수로 검증하고 누락·계약 불일치 시 실패한다.
React/UI/state/i18n은 Claude가 담당한다.

#### 실시간 UI

- [x] 시작/정지, session state, CA 설치/제거와 최초 사용 위험 고지
- [x] process tree, 안정적인 live list, 진행 행 terminal reconciliation
- [x] 사용자 스크롤을 존중하는 auto-follow, batch update와 row cap
- [x] 명시적 drop 경고를 포함한 persisted/drop/backpressure/disk/recovery 상태
- [x] fidelity·coverage·passthrough·unattributed 경고를 숨기지 않는 UX
- [x] stop 후 같은 화면에서 finalized session lazy loading

- [x] R8 renderer: 시작 시 `LiveCaptureContract`를 읽어 row cap, event-skip
  resync, page 재진입 복원, finalized 인계를 계약에서 파생한다. 알 수 없는
  schema는 내장 기본값으로 되돌리고 그 불일치를 사용자에게 공개한다.
- [x] R9: `isDecodedLiveFidelity`가 `liveFidelityTone`을 통해 live table의
  fidelity 강조를 결정하므로, 교환을 실제로 읽지 않은 등급은 일반 캡처
  트래픽처럼 보이지 않는다.
- [x] R11: transaction state, session state, CA state, process attribution을
  raw engine token 대신 EN/KO 폐쇄 label map으로 표기한다.
- [x] S1: finalized hint를 provenance별로 선택한다
  (`CAPTURE_PROVENANCE_HINT_KEYS`). live proxy evidence는 ArchScope 측정
  문장을 사용하고, HAR/import 전용 문장은 `foreign_tool` 관측 지점에서만
  도달할 수 있다.
- [x] S4: finalized mode/fidelity/observation/detail-storage token을 EN/KO
  폐쇄 label map으로 번역하고 원본 token은 hover로 노출하며,
  `capture_mode_counts`/`fidelity_counts`/`coverage_counts` 분포를 집계 등급
  아래에 표시해 `mixed`와 최약 fidelity를 해석 가능하게 한다.
- [x] S3 renderer 후속: `redaction.known=false`를 "민감 정보 없음"이 아니라
  기록되지 않은 복구 metadata로 주의 표기와 함께 표시한다.

L10은 paired user guide와 JVM truststore harness 계약으로 닫혔다.

SEC-17은 renderer 아래에서 강제된다. unknown attribution은 기본적으로 저장과
progress 노출 전에 drop하며, 명시적 opt-in을 선택해도 리댁션된 metadata만
보존한다. Body는 계속 무조건 생략하므로 미래 body-capture tier에는 SEC-10이
여전히 선행되어야 한다. Acceptance package는
`cmd/archscope-app/testdata/t581_live_capture_acceptance.json`,
`capture_windows_e2e_test.go`,
`scripts/verify-windows-live-capture.ps1`이며 공유 계약은
`scripts/t581-live-capture-harness-contract.json`이다.

**PASS 기준:** Windows에서 browser/curl/JVM/Electron의 지원 tier 시나리오, UI
재진입, 장시간 세션, 실패 복구를 E2E로 통과하고 미지원 H2/QUIC/pinning을 성공처럼
보이지 않아야 한다.

### H-RG5 — HTTP 전용 세션 Diff

#### Codex 엔진 — 완료 (2026-07-30)

- [x] versioned URL template과 `{other}` top-K projection
- [x] endpoint/host/process 차원과 명시적 numerator/denominator
- [x] `aligned`/`duration_only`/`none` 시간 정렬 등급
- [x] bounded `http_capture_diff` 결과와 `HTTP_DIFF_*` findings
- [x] store 재스캔 없는 export projection과 Workspace routing contract

분석기는 각 `http_capture` 결과에 버전이 지정된 top-1,000 source projection을
붙이고 Diff 시 그 projection만 비교한다. Wails 백엔드는
`AnalyzeHttpCaptureDiff`, `GetHttpCaptureDiffContract`,
`ResolveWorkspaceComparison`을 노출하며, 이 입력에 legacy Diff를 사용하지 않고
새 NavKey도 요구하지 않는다고 명시한다. 회귀 테스트는 URL template 규칙,
차원별 합계 교차 검증, HAR process 차원 비활성화, 명시적 rate 분모, alignment
동작, top-K 결과 상한, store-free JSON export, 순서만 다른 동일 세션의 equality를
고정한다. 전체 Go test/vet/build가 통과했다. renderer 생성 바인딩은 Claude UI
handoff 범위로 이관되었고 아래에서 완료되었다.

#### Claude UI — 완료 (2026-07-30)

- [x] HttpCapturePage compare action과 Workspace 비교 진입
- [x] alignment grade에 따른 overlay 허용/억제
- [x] before/after delta, 분모, unmatched template, drilldown cursor UX

renderer 생성 바인딩은 module 고정 Wails CLI(alpha2.117)로 재생성되어 세
메서드와 요청/계약 모델을 포함한다. 공유 비교 패널
(`components/HttpCaptureComparisonPanel.tsx`)은 HttpCapturePage와 Analysis
Workspace 양쪽에 마운트되고, 선택/실행 lifecycle은
`state/httpCaptureDiff.ts`의 순수 reducer + module store가 관리해 두 표면이
같은 A/B 선택을 공유한다. 비교 라우팅은 renderer가 추측하지 않고
`ResolveWorkspaceComparison` 판정을 따르며, 시작 시점에
`GetHttpCaptureDiffContract`를 채택해 구현하지 않은 schema 버전이면 비교를
비활성화하고 mismatch를 공지한다(H-RG4 R8 패턴). overlay는 backend의
`overlay_allowed`+grade 판정만 따르고 인식 불가 등급은 fail-closed로
억제한다. 요약과 drilldown은 error rate/traffic share의 분자/분모, per-minute
분모(분), duration sample 수를 그대로 노출하고, rate가 불가능한 세션은 그
이유 코드를 닫힌 EN/KO 라벨로 표시한다. added/removed(unmatched template)
행의 부재 측은 `0`이 아니라 `—`로 렌더되고, HAR pseudo-process 쌍의 process
차원은 이유와 함께 비활성 카드로 노출된다. 비교 프로젝션이 없는(백엔드
핸드오프 이전에 분석된) 결과는 재분석 안내와 함께 사전에 차단된다. 순서만
다른 동일 세션의 빈 change 테이블은 명시적 "차이 없음" 상태로 렌더된다.
state 회귀는 contract 채택/거부, 닫힌 토큰 세트, overlay fail-closed 게이트,
race-safe 결과 provenance, 후보 필터링, projection 전제조건, 신규 키 전체의
EN/KO 커버리지를 고정한다. `npm run test:state`, `npm run build`(tsc+vite),
`go build ./...`, `go vet ./cmd/archscope-app/...`,
`go test ./cmd/archscope-app/... ./internal/analyzers/httpcapture/...`가
통과했다. Claude는 엔진 소스를 변경하지 않았다.

**PASS 기준:** 순서가 다른 동일 세션은 차이가 없어야 하고, degenerate timestamp와
HAR pseudo-process 비교에서 지원하지 않는 정규화/차원을 숨기거나 명시적으로
비활성화해야 한다.

## 7. 교차 기능 및 릴리스

### X-RG1 — HTTP × CPU/Jennifer/access log 교차 분석

- **Codex:** session ↔ CPU profile 시간 정렬, Jennifer `NETWORK_GAP` 대조, access
  log client/server 대조, 신뢰도·불일치 diagnostic을 bounded result로 만든다.
- **Claude:** 같은 시간 창의 HTTP transaction과 CPU run/server evidence를
  오가는 drilldown 및 overlay를 만든다.
- **리뷰:** 서로 다른 clock/offset에서 인과관계를 단정하지 않고 alignment grade와
  evidence provenance를 항상 보여 주는지 검증한다.

### R-RG1 — 통합 릴리스 승인

- Go 전체 test/vet/build, frontend state test/build, Windows GUI/live-capture E2E,
  macOS offline import/package smoke를 수행한다.
- English/Korean 문서, importer matrix, user/security/performance guide와 실제 기능을
  맞춘다.
- release note에는 offline HAR, Windows live tier, H2/QUIC/pinning, coverage 한계를
  구분해서 기록한다.
- `R-RG1 PASS` 전에는 버전 tag나 GitHub release를 만들지 않는다.

## 8. 첫 실행 지점

T-580 / `H-RG3` 엔진 구현은 2026-07-27 `REVIEW`에 진입했고 2026-07-28 독립
`H-SEC2` CA/TLS/권한 게이트를 통과했다. T-581 / H-RG4는 2026-07-30 독립
그룹 `PASS`로 닫혔다: 수정된 Windows 실행이 contradiction 없는 fixture-only
교체 artifact와 체크섬을 보관했고, V1–V3에 한정한 네 번째 재리뷰가 조건
해소를 검증했다. 다음 행동은 T-582 / `H-RG5` HTTP 세션 Diff다.
