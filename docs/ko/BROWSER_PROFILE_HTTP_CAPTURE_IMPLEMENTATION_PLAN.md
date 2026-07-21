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
  **완료 — 통합 PASS (2026-07-21)**다. 실시간 Windows 캡처는 T-571 실 NIC
  증거가 닫히기 전에는 시작하지 않는다.

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
| 3 | `H-RG2` Windows coverage proof | 대기 | `H-RG1 PASS`; Windows 실 NIC 측정과 `H-COV1 PASS` |
| 4 | `H-RG3` 실시간 캡처 엔진 기반 | 계획 | `H-RG2 PASS`; 그룹 내부 `H-SEC2 PASS` |
| 5 | `H-RG4` 실시간 UI 및 Windows E2E | 계획 | `H-RG3 PASS` |
| 6 | `H-RG5` HTTP 세션 Diff | 계획 | `H-RG4 PASS` |
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

- [ ] Windows real-NIC target으로 ETW CAP-1/CAP-4를 재실측한다.
- [ ] ALE audit 설정 후 WFP allow-path attribution을 재실측한다.
- [ ] PowerShell polling 대신 직접 `GetExtendedTcpTable` 호출로 CAP-5 CPU overhead를
  재측정한다.
- [ ] CAP-1~CAP-6 판정과 capability/fidelity matrix, source ledger를 갱신한다.
- [ ] 실패한 scope의 absolute coverage ratio를 제거하고 self-observed five counters만
  남긴다.

**PASS 기준:** false attribution 0, 측정 재현 절차와 raw evidence가 있고, UI에
노출 가능한 값과 노출 금지 값이 명확히 승인돼야 한다.

### H-RG3 — 실시간 캡처 엔진 기반

#### Codex 엔진 To-Do

- [ ] session state machine과 idempotent start/stop/recovery API
- [ ] append-only NDJSON/blob/manifest store, rebuildable index, versioned cursor paging
- [ ] byte-bounded write/live/aggregate 3단 buffer와 disk slow/full 정책
- [ ] captured/persisted/bodyOmitted/eventSkipped/kernelDropped/parseFailed counters
- [ ] H1 semantic MITM + H2 passthrough `Proxy`/`Interceptor` production path
- [ ] Windows direct TCP-owner process attribution과 짧은 연결의 불확실성 표시
- [ ] live completion-order와 file replay aggregate parity
- [ ] Wails `CaptureService`, sequence/snapshotVersion events, snapshot recovery
- [ ] CA lifecycle, upstream TLS verify-always, 승인 기반 scoped passthrough

#### 개별 게이트 H-SEC2

CA 개인키 저장, trust-store 부분 실패 rollback, 제거/만료, upstream 검증, pinning
진단, passthrough scope/expiry, privilege IPC가 SEC-1~SEC-15 해당 항목을 통과해야
Claude에게 CA와 live-control UI 계약을 handoff한다.

#### 그룹 PASS 기준

disk full, crash recovery, event loss/re-entry, CA failure, pinning, cancellation,
streaming, H2 passthrough fixture와 long-session memory bound가 통과해야 한다.

### H-RG4 — 실시간 UI와 Windows E2E

#### Codex 통합 To-Do

- [ ] 고정된 CaptureService binding과 Windows E2E harness 제공
- [ ] 엔진 snapshot/cursor/filter semantics를 UI acceptance fixture로 제공
- [ ] 패키지/서명/권한 경계 smoke 지원

#### Claude UI To-Do

- [ ] 시작/정지, session state, CA 설치/제거와 최초 사용 위험 고지
- [ ] process tree, 안정적인 live list, 진행 중 transaction 표시
- [ ] 사용자 스크롤을 존중하는 auto-follow, batch update와 row cap
- [ ] persisted/drop/backpressure/disk 상태와 recovery 표시
- [ ] fidelity·coverage·passthrough·unattributed 경고를 숨기지 않는 UX
- [ ] stop 후 같은 화면에서 finalized session lazy loading

**PASS 기준:** Windows에서 browser/curl/JVM/Electron의 지원 tier 시나리오, UI
재진입, 장시간 세션, 실패 복구를 E2E로 통과하고 미지원 H2/QUIC/pinning을 성공처럼
보이지 않아야 한다.

### H-RG5 — HTTP 전용 세션 Diff

#### Codex 엔진 To-Do

- [ ] versioned URL template과 `{other}` top-K projection
- [ ] endpoint/host/process 차원과 명시적 numerator/denominator
- [ ] `aligned`/`duration_only`/`none` 시간 정렬 등급
- [ ] bounded `http_capture_diff` 결과와 `HTTP_DIFF_*` findings
- [ ] store 재스캔 없는 export projection과 Workspace routing contract

#### Claude UI To-Do

- [ ] HttpCapturePage compare action과 Workspace 비교 진입
- [ ] alignment grade에 따른 overlay 허용/억제
- [ ] before/after delta, 분모, unmatched template, drilldown cursor UX

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

현재 다음 행동은 T-571 / `H-RG2`다. Windows 실 NIC ETW/WFP coverage와 direct
`GetExtendedTcpTable` CAP-5 overhead를 측정한 뒤, live-capture 엔진 그룹을 시작하기
전에 `H-COV1 PASS`를 획득한다.
