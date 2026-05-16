# 중기 로드맵(T-431~T-451) 코드 검토

- 작성일: 2026-05-16
- 검토자: claude-code (Opus 4.7, 1M context)
- 대상: 0.3.3 릴리스 시점의 `apps/engine-native/` 트리
- 범위: T-431~T-436 (Incident Timeline), T-437~T-440 (SLO/Golden Signals),
  T-441~T-444 (Service Flow), T-445~T-451 (Report Pack & AI 게이트)
- 검토 방식: 영역별 3개 서브에이전트 병렬 정적 분석. 코드 수정 없음.

## 총평

기능 카드 31개 모두 "Completed"로 마킹됐고 Wails UI에서 동작은 한다. 다만
**세 개의 가드레일이 동시에 깨졌고**, 그 결과 일부 카드는 "구현은 됐지만
의도대로 동작하지 않음" 상태이다. 0.3.3은 데모 가능한 MVP 수준으로 보고,
0.3.4 패치에서 아래 P0 6건 + 가드레일 복구(P1 7~9)에 집중하는 정리 릴리스를
권한다. 특히 T-432가 안 도는 점과 AI 게이트가 advisory인 점은 외부 데모
전에 반드시 막아야 한다.

---

## P0 — 지금 잘못 동작하거나 게이트가 무력한 항목

### 1. Incident Timeline의 cross-analyzer 매핑이 비어있음 (T-432)

- `state/incidentTimeline.ts:365` `eventsFromThreadDump`는 `tables.deadlocks`,
  `tables.contended_locks`, `tables.blocked_threads`, `tables.long_running_threads`를
  읽지만 **Go `thread_dump` analyzer는 `tables.threads`만 emit**한다
  (`internal/analyzers/threaddump/analyzer.go:168`).
- `lock_contention`(실제 키: `tables.locks`, `tables.deadlock_chains`),
  `multi_thread_dump`(실제 키: `tables.persistent_blocked_threads` 등)도 동일.
- `eventsFromException`도 존재하지 않는 `tables.exception_groups`까지 참조
  (`incidentTimeline.ts:347` ↔ `internal/analyzers/exception/analyzer.go:183`).
- **결과: 타임라인에 deadlock/contention 이벤트가 영구히 0건.** T-432의 핵심
  가치가 무력화됨.

### 2. AI 게이트가 사실상 advisory (T-450 / T-451)

- **이중 구현**: Go `internal/aiinterpretation/` 게이트는 production caller가
  없어 dead code. 실제 게이트는 `state/aiInterpretation.ts:171-266` (TS).
- **TS가 Go보다 약함**: Go는 `model/summary/reasoning` non-empty 강제, TS는
  강제 안 함 (`validation.go:63` ↔ `state/aiInterpretation.ts:186-220`).
- **Quote-to-source 검사가 optional**: `evidence_quotes` 필드를 LLM이 빼면
  `EVIDENCE_QUOTE_MISMATCH`를 회피 (`validation.go:117-127, :158`). T-450의
  "quote-to-source matching"은 실제로 강제되지 않음.
- **MinConfidence 하드코딩 0.3** (`state/aiInterpretation.ts:207`) — 운영자가
  게이트 강도를 조정할 수 없음.
- Go 쪽 `runtime.go:81-89`는 검증 실패 시 `(nil, error)`를 반환하지만 caller가
  없어 provenance(provider/model/prompt_version) 스탬프와 검증 결과가 함께
  소실되는 구조.

### 3. Privacy redactor가 deny-list (`privacy.go:5-15`)

이메일/IPv4/`token=|secret=|password=|api_key=`만 마스킹. **스택트레이스,
hostname, IPv6, JWT, `Authorization: Bearer …`, SQL, 한글 PII는 전부 LLM에
그대로 전송됨.** 보안/컴플라이언스 관점에서 P0. allow-list 또는 최소한 위
패턴 추가 필요.

### 4. 단위 불일치로 SLO가 영구히 안 터짐

- `internal/analyzers/traceimport/analyzer.go:627` (와 `:421`)은 `error_rate`를
  **0..1 fraction**으로 emit, `internal/analyzers/accesslog/analyzer.go:515`는
  **0..100 percent**로 emit.
- `state/sloGoldenSignals.ts:766-775`는 둘 다 `unit:"percent"`로 태깅.
- 결과: trace-import 기반 dependency error_rate는 **실제 500%를 넘기 전까지
  SLO 위반으로 잡히지 않음.**

### 5. Trace Import × Jennifer MSA 더블 카운트

`state/sloGoldenSignals.ts:1304-1339`가 `canonicalScope(scope)`만으로 그룹핑
→ 같은 edge `caller→callee`를 두 분석기가 `kind:"traffic", aggregation:"count"`로
emit하면 `aggregateSignalValues`(`:1361`)에서 **그대로 SUM**. 한 트래픽이
두 번 카운트됨.

### 6. ZIP path traversal 표면 (`state/reportPack.ts:644-678`)

`ZipFile.path`를 정규화 없이 헤더에 기록. 현재 호출부는 안전한 리터럴이지만
카드 ID 등이 흘러 들어가면 `..\..\evil.html` 가능. 헬퍼 단계에서
`path.normalize` + leading `/`/`..` 제거 필요.

---

## P1 — 아키텍처 가드레일 위반

### 7. 로드맵 절반이 UI에 구현됨 (CLAUDE.md "parser/analyzer/exporter/UI 분리" 위반)

- T-431~T-436 (Incident Timeline), T-437~T-440 (SLO/Golden Signals),
  T-441~T-444 (Service Flow), T-445~T-447 (Report Pack)이 **전부
  `cmd/archscope-app/frontend/src/state/*.ts`에 위치**. Go `internal/analyzers/`에
  대응 패키지 없음.
- 결과:
  - CLI(`cmd/archscope-engine`)에서 incident timeline / SLO / service flow /
    report pack 생성 불가.
  - 같은 분류 로직이 Go(파서/분석기)와 TS(2차 분석)로 양분되어 스키마 변경 시
    silent breakage 발생 (이미 P0-1 사례).
  - 서버 사이드 자동화·테스트·exporter 연계 불가.

### 8. AnalysisResult 컨트랙트 드리프트 (CLAUDE.md "공통 AnalysisResult 보존" 위반)

- `IncidentTimelineAnalysisResult` (`incidentTimeline.ts:92-103`)은
  `metadata: {schema_version, projection, source_results}`만 있고,
  `models.AnalysisResult`(`internal/models/analysis_result.go:88-99`)가 요구하는
  **`parser`, `findings`, `diagnostics`, `extra` 누락.**
- `SloAnalysisResult` (`sloGoldenSignals.ts:457-476`)은 `type`/`source_files`/`series`도
  없음.
- `ServiceFlowExportPayload`는 아예 `series/tables/charts/metadata` 봉투를 안 씀
  (`serviceFlow.ts:101-116`).
- → 이 셋은 "exportable AnalysisResult"라고 하지만 **Go exporter가 다시 못
  읽는 가짜 AnalysisResult**.

### 9. Exporter가 분석을 재실행함

`state/reportPack.ts:9-11`이 `buildIncidentTimelineAnalysisResult`,
`buildServiceFlowAnalysis`, `analyzeSloViolationsFromEntries`를 export 시점에
호출. "Exporter는 분석을 재계산하지 않는다" 가드레일 위반 + 입력 결과와
export 결과가 시간차로 달라질 수 있음.

### 10. Service-edge 통합이 실제로는 통합 안 됨 (T-441/T-442)

`serviceFlow.ts:138-150`이 `caller.toLowerCase()/callee.toLowerCase()`로만
머지. Jennifer는 `OrderService` 형태, trace-import는 `order-service` 형태 —
**실제 환경에서 매칭 실패**, 같은 edge가 두 노드로 분리됨.

### 11. AI provenance가 단순 복사 (T-446)

`state/reportPack.ts:419-451`이 provider/model/prompt_version만 복사.
**`prompt_hash`, `temperature`, `seed`, `token counts`, 무결성 해시 모두
누락.** "tamper-evident provenance"라고 부르기엔 부족.

### 12. AI vs deterministic 구분이 EvidenceBoard에서 사라짐 (T-449)

Workspace 페이지(`AiInterpretationPanel.tsx`)에선 분리되지만
`pages/EvidenceBoardPage.tsx:120-150`은 동일 `EvidenceCard` 컴포넌트로 렌더 —
캡처 후엔 AI 뱃지/confidence/limitations가 안 보임. 시각적 분리가 페이지에
따라 달라짐. `pages/AnalysisWorkspacePage.tsx:43-67`에서 캡처 시 `analyzer:
'${result_type}:ai'`, `source_kind: 'ai_finding'`로 태깅만 되고 렌더 단계에서
사용되지 않음.

---

## P2 — 결정성 · 견고성 · 테스트

- **Determinism 붕괴** (T-436 "deterministic narrative", T-446 report diff):
  `new Date().toISOString()`이 `incidentTimeline.ts:142`,
  `serviceFlow.ts:126,162,184`, `reportPack.ts:143,271,378,845`, ZIP
  `setDosDateTime`(`reportPack.ts:723`)에 박혀있고, `incidentTimeline.ts:456`은
  `toLocaleString()`(host locale!)으로 payload를 채움. 동일 입력의
  byte-identical export 불가.
- **Unix-초 timestamp 누락**: `incidentTimeline.ts:772-788` `parseTime`이
  ms/μs/ns만 인식 → 초 단위면 `null` 반환 → `created_at`로 fallback돼 정렬
  붕괴.
- **에러-버짓 수식 오류**: `sloGoldenSignals.ts:1475-1500`이 `objective_percent`를
  무시하고 `threshold` 자체를 분모로 사용. burn-rate가 사실상 `actual/threshold`가
  됨. `consumedForLowerBound`(`:497-499`)도 `100 - threshold`를 분모로 사용.
- **`Math.max(...arr)` 스택 오버플로 잠재** (`sloGoldenSignals.ts:1183, :1189,
  :570`): 현재는 cap이 있어 안전하나 GC `series.heap_after_mb`는 다운샘플 후에도
  만 단위 가능. `addPeakSeries`가 전 시리즈를 spread.
- **Default SLO 타깃 하드코딩**: `sloGoldenSignals.ts:210-416` `DEFAULT_SLO_TARGETS`
  외에 설정/오버라이드 경로 없음 — T-439의 "SLO target configuration" 미충족.
- **runtime-stack 분기 누락**: `signalsFromEntry`(`sloGoldenSignals.ts:584-621`)가
  `exception_stack`만 처리. `nodejs_stack`/`python_traceback`/`go_panic`/
  `dotnet_exception_iis`(`internal/analyzers/runtime/analyzer.go:65-68`)는 generic
  분기로 흘러 신호가 비어 나옴.
- **테스트 0**: `cmd/archscope-app/frontend/src` 전체에 Vitest/Jest 스펙 부재.
  T-431~T-451 거의 전부 untested. Go 쪽도 신규 analyzer 부재로 골든픽스처 없음.
  Go AI 게이트 테스트(`aiinterpretation_test.go`)는 redaction 엣지(스택트레이스,
  hostname, JWT 등)와 evidence-refs 누락 시 Execute 실패 경로 미커버.
- **공유 유틸 중복**: `severityRank`, `arrayOfObjects`, `numberValue`,
  `uniqueStrings` 등이 `incidentTimeline.ts:810-869`, `serviceFlow.ts:606-744`,
  `reportPack.ts:749-820`에 각각 재구현.
- **Mermaid 출력 결함**: `serviceFlow.ts:217-220`은 `services.length === 0`이고
  `sourceOnlyFindings.length > 0`인 경우 미선언 참여자에 `Note over S1`을 출력 →
  malformed sequence diagram. `mermaidText`(`:644-646`)는 `JSON.stringify` 결과를
  그대로 라벨에 넣어 따옴표가 시각적으로 노출.
- **Narrative 정렬 O(N² log N)**: `incidentTimeline.ts:522-524`가 `.sort()` 콜백
  안에서 `events.find(...)` 호출. `Map<id, sort_time>` 사전 구축으로 충분.
- **Dedupe 키가 payload 직렬화 의존**: `incidentTimeline.ts:613-621`이
  `JSON.stringify(payload).slice(0, 120)` 사용 — 키 순서 차이로 미스매치 가능.

---

## 권장 액션 (우선순위 순)

| 순서 | 액션 | 영향 |
|---|---|---|
| 1 | `eventsFromThreadDump`/`Exception`/`LockContention`이 참조하는 테이블 키를 Go analyzer가 실제로 emit하는 키로 정정하거나, **분석기에 누락된 테이블을 추가**. 회귀 픽스처 1개씩 추가. | T-432 실효화 |
| 2 | AI 게이트를 Go(`internal/aiinterpretation`)로 일원화하고 Wails 바인딩 통해 호출. TS는 표시만. `evidence_quotes` 필수화, `MinConfidence` 설정화. | AI 안전성 |
| 3 | `privacy.go`를 **allow-list** 또는 최소한 stack/hostname/IPv6/Bearer/JWT 패턴 추가. 한글 PII 룰 별도 검토. | 컴플라이언스 |
| 4 | `traceimport.error_rate`를 0..100 percent로 정규화하거나 `error_fraction`으로 키 분리. SLI 단위 변환 단계 신설. | SLO 정확성 |
| 5 | Service Flow / SLO / Incident Timeline 중 **하나만** Go analyzer로 옮겨 패턴 잡기 (Service Flow가 가장 단순). 이후 나머지 점진 이관. | 가드레일 복원 |
| 6 | `models.AnalysisResult` 호환 헬퍼 추가(`parser`/`findings` 필수화) → TS 프로젝션이 그 스키마를 만족하도록 강제. | 컨트랙트 회복 |
| 7 | 모든 `new Date()`를 명시적 `now: Date` 매개변수 주입으로 교체. ZIP `setDosDateTime`은 고정 epoch 옵션. | 결정성 |
| 8 | `state/`의 7개 신규 파일에 Vitest 스펙 추가 (빈 입력, 단일 이벤트, 단위 변환, 게이트 우회). | 회귀 방지 |
| 9 | Service-edge 매칭에 케이스/하이픈 정규화 함수 + 별칭 테이블. | T-441 실효화 |
| 10 | `ZipFile.path` 정규화/검증 헬퍼. AI provenance에 `prompt_hash` 추가. | 보안/무결성 |

---

## 참고 파일 경로

- `apps/engine-native/cmd/archscope-app/frontend/src/state/incidentTimeline.ts`
- `apps/engine-native/cmd/archscope-app/frontend/src/state/serviceFlow.ts`
- `apps/engine-native/cmd/archscope-app/frontend/src/state/sloGoldenSignals.ts`
- `apps/engine-native/cmd/archscope-app/frontend/src/state/reportPack.ts`
- `apps/engine-native/cmd/archscope-app/frontend/src/state/aiInterpretation.ts`
- `apps/engine-native/cmd/archscope-app/frontend/src/state/evidenceBoard.ts`
- `apps/engine-native/cmd/archscope-app/frontend/src/pages/IncidentTimelinePage.tsx`
- `apps/engine-native/cmd/archscope-app/frontend/src/pages/ServiceFlowPage.tsx`
- `apps/engine-native/cmd/archscope-app/frontend/src/pages/SloGoldenSignalsPage.tsx`
- `apps/engine-native/cmd/archscope-app/frontend/src/pages/EvidenceBoardPage.tsx`
- `apps/engine-native/cmd/archscope-app/frontend/src/pages/AnalysisWorkspacePage.tsx`
- `apps/engine-native/internal/aiinterpretation/{evaluation,evidence,privacy,prompting,runtime,validation}.go`
- `apps/engine-native/internal/aiinterpretation/aiinterpretation_test.go`
- `apps/engine-native/internal/analyzers/threaddump/analyzer.go`
- `apps/engine-native/internal/analyzers/lockcontention/analyzer.go`
- `apps/engine-native/internal/analyzers/exception/analyzer.go`
- `apps/engine-native/internal/analyzers/traceimport/analyzer.go`
- `apps/engine-native/internal/analyzers/accesslog/analyzer.go`
- `apps/engine-native/internal/analyzers/runtime/analyzer.go`
- `apps/engine-native/internal/models/analysis_result.go`

## 한 문장 요약

**0.3.3는 "데모 가능한 MVP" 수준이고, 0.3.4는 위 P0 6건 + 가드레일
복구(P1 7~9)에 집중하는 정리 릴리스로 잡는 걸 권한다.** 특히 T-432가
안 도는 점과 AI 게이트가 advisory인 점 두 가지는 외부 데모 전에 반드시
막아야 한다.
