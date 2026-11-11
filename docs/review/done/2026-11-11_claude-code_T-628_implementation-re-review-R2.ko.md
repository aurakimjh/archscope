# T-628 구현 재리뷰 (R2) — 2026-11-11

- 대상 커밋: archscope `63d3b6a` (fix(ingestion): address T-628 implementation review findings). 이전 리뷰 대상은 `a698d26`.
- 이전 리뷰: `docs/review/done/2026-11-11_claude-code_T-628_implementation-review.ko.md` (R-01~R-14, 원 판정 CONDITIONAL)
- 처리 기록: `work_status.md`의 "T-628 independent review disposition and remediation", 증거 `docs/review/evidence/2026-11-11-t628-remediation/local.json`
- 공유 자산: projects-assets `56255e2`. bundled `manifest.json`(1.0.1)·`server.log`·`wire-cases.json`·`measurement-descriptor.schema.json`이 공유본과 바이트 단위로 일치함을 sha256으로 확인했다.
- 작성: claude-code가 재리뷰 작성 역할만 맡았다. 이전 리뷰의 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 수행했다. 구현 코드와 기존 미커밋 HTTP 변경은 수정하지 않았다.
- 방법: 수정 diff(`a5f581f..63d3b6a`) 정독, 저장소 밖 scratch worktree에서 재실행·재현 테스트, 실제 CLI 실행. 증거 파일 `source_sha256` 40개 항목이 `63d3b6a`의 커밋 내용과 모두 일치한다.

## 1 결론

**판정: 조건부 승인(CONDITIONAL → 대부분 해소).** 이전 P1(R-01)과 P2(R-02~R-04)는 코드·재현으로 해소를 확인했다. 단 R-04는 **수정이 새 제약을 만들었다**(N-01). 신규 finding은 P2 1건, P3 3건이다. 이전 14건은 모두 반영된 것으로 판단한다. 다만 R-08은 문서가 밝힌 대로 최소 범위만 구현했다.

**T-629 착수: 가능.** 원 리뷰의 조건(R-01 수정과 깊은 profile 회귀 테스트)이 충족됐다. N-01은 T-629를 막지 않는다. T-631(compare provider)·T-635(baseline) 전에 닫아야 한다.

**T-628 `DONE` 복귀: 조건부 가능.** N-01을 수정하거나 "동일 입력 재현 비교만 지원"으로 계약과 지원표를 정정하면 가능하다. 다음 한계는 기존 미검증 항목으로 유지한다: 실제 WebView IPC/UI, Windows 네이티브 실행, hosted CI.

## 2 이전 finding 반영 확인

| ID | 등급 | 판정 | 확인 근거 |
|---|---|---|---|
| R-01 | P1 | **해소** | 용도별 상한 분리(`wirejson/decode.go` `ResultLimits`, `LegacyResultLimits`). 실제 CLI에서 256·1000·1100·2000·4000프레임 collapsed 입력의 `report json`과 `evidence export` 전부 exit 0. 이전 실패 경계(31프레임)는 통과. profiler가 자체로 트리 깊이를 제한해 출력 깊이가 1030에서 멈추므로 도달 가능한 입력은 새 상한(2112) 안에 든다. |
| R-02 | P2 | 해소 | 종료된 job만 회수하고 활성 job은 유지(`service.go` Start). 200개 job 이력·활성 job 보존 회귀 테스트가 있고 통과한다. |
| R-03 | P2 | 해소 | schema 역할·family·결과 type·payload 버전을 조립 시 대조하고 conformance 사례를 실행한다(`ingestion/importer.go`, `schema.go`). `analysis.New`가 result registry와 교차 검증한다. |
| R-04 | P2 | **부분 해소** | label 신뢰는 제거했고 같은 원본·옵션의 두 import 비교는 delta 0으로 동작한다. 그러나 다른 원본끼리는 항상 거부된다(N-01). |
| R-05 | P3 | 해소(소결함) | `max_lines`가 완료 결과 + `import_coverage.selection`으로 바뀌었고 legacy와 summary·tables·correlation_keys가 일치한다. 정확히 줄 수와 같은 값이면 잘리지 않았는데도 선택 표지가 붙는다(N-03). |
| R-06 | P3 | 해소 | 지원 범위를 descriptor 단일 출처로 사용, 미지 필드는 보존. 1.0.1 사례 테스트 존재. |
| R-07 | P3 | 해소 | detector 오류를 진단으로 격리하고 선택된 importer의 진단일 때만 실패. 모든 후보가 실패하면 `detection_failed`. |
| R-08 | P3 | 최소 범위 해소 | 결과 1,024개·512 MiB 상한, 같은 내용 Load는 handle 재사용(import 1 + Load 2 → 같은 handle 확인), `Release`, 파일 잠금과 시작 시 staging 정리. 장기 수명 모델은 T-630으로 남겼다. 공유 handle 해제 문제는 N-02. |
| R-09 | P3 | 해소 | retryable 표(`IMPORT_IO_FAILED` 등), 읽기 오류와 버퍼 초과 분리, 대기 timeout 코드 분리. |
| R-10 | P3 | 해소 | `AttachCorrelationKeys` 복원, parity 테스트가 correlation_keys 비교 포함. |
| R-11 | P3 | 해소 | manifest `expected`를 단정하는 테스트와 schema 변형·공유본 정합 갱신(1.0.1). |
| R-12 | P3 | 해소(소결함) | 예약 series 충돌 시 오류, collection 컬럼 availability 투영. 투영 크기는 상한이 없다(N-04). |
| R-13 | P3 | 해소 | 대기와 실행 deadline 분리(`QueueTimeoutMS`), Load도 같은 slot을 사용. |
| R-14 | P3 | 해소 | 수동 TS facade 사용, `tsc`·`npm run test:state` 통과. |

## 3 신규 finding

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| N-01 | **P2** | population identity에 원본 hash를 포함해 서로 다른 원본의 정확 비교가 항상 거부됨 | `resultcontract/contract.go:519-533`, `catalog/builtin.go` BindSelection |
| N-02 | P3 | 중복 제거된 handle을 여러 호출자가 공유하는데 `Release`가 참조 수 없이 즉시 삭제 | `analysis/store.go` publish·Release, `evidenceservice.go:136` |
| N-03 | P3 | `max_lines`가 전체 줄 수와 같아도 선택 표지(`selection:"max_lines"`)를 남김 | `parsers/serverlog/importer.go:83-87` |
| N-04 | P3 | 표시 projection이 collection 행×컬럼 항목을 상한 없이 생성 | `resultcontract/presentation.go` |

### N-01 서로 다른 원본의 비교가 구조적으로 불가능 (P2)

**근거**: `populationIdentity`는 `source_sha256` 집합, `selection_hash`, `semantic_revision`, `coverage`가 **모두 일치**해야 비교를 허용한다(DATA_MODEL "R-04"). pilot은 `source_sha256`에 해당 import의 원본 hash, `selection_hash`에 옵션 hash를 넣는다. 비교의 실사용은 배포 전후 같은 서비스의 서로 다른 로그이므로 원본 hash는 반드시 다르다. 같은 원본·같은 옵션이면 결과도 같아 delta는 항상 0이다. 즉 공개된 `CompareEvidenceScalar`는 **0이 아닌 delta를 반환할 수 없다.**

**재현**(scratch worktree, `TestR2DifferentSourcesCompare`): `server.log`와, 그 파일에 ERROR 1줄을 덧붙인 `after.log`를 같은 옵션으로 import한 뒤 `/summary/total_events`를 비교하면 둘 다 `completed`인데 `population identity mismatch`가 반환된다. 같은 원본 두 번 import는 delta 0으로 성공한다(기존 테스트).

**영향**: R-04의 원인(label 문자열 신뢰)은 사라졌지만 비교 기능이 실질적으로 비활성이다. T-631 compare provider와 T-635 baseline이 이 동작을 상속하면 같은 입력끼리만 비교하게 된다. coverage 문자열에 레코드 수가 들어 있어 같은 원본이라도 레코드 예산 도달 여부가 다르면 거부되는 점도 의도 확인이 필요하다.

**수정 제안**: 모집단 정체성을 두 층으로 나눈다.
1. 비교 가능성 조건: semantic revision, 선택 규칙(options hash), kind·unit·aggregation·coverage 종류 일치.
2. 관측 대상 식별: source hash는 비교 가능성 조건이 아니라 결과 provenance와 baseline 참조에 기록한다. 서로 다른 원본의 비교는 명시적 baseline/target 참조(T-635)로 허용하고, 허용되었음을 delta 응답에 남긴다.

다른 원본의 before/after 비교 테스트와, 선택 규칙 불일치 거부 테스트를 추가한다. 의도적으로 "동일 입력 재현 비교만 허용"이라면 DATA_MODEL과 지원표에 명시하고 `CompareEvidenceScalar`의 용도를 그 범위로 문서화한다.

### N-02 공유 handle의 Release 의미 (P3)

**근거**: 같은 내용을 Load하면 기존 handle을 반환한다(`publish`가 hash로 중복 제거). `Release`는 참조 수 확인 없이 파일을 지운다.

**재현**(`TestR2SharedHandleRelease`): import 결과 handle, Load 1, Load 2가 모두 같은 handle이다. 첫 보유자가 `Release`하면 성공하고 둘째 보유자의 `Read`가 `IMPORT_RESULT_STALE`로 실패한다.

**영향**: Wails가 `ReleaseEvidenceResult`를 공개하므로 한 화면이 해제하면 다른 화면의 Workspace handle이 끊긴다. 작성 측이 T-630에 "참조 소유권 이후 해제"를 넘겼지만, 그 전에는 UI가 호출하지 않는다는 전제에 의존한다.

**수정 제안**: T-630 수용 기준의 일부로 유지하되, 그때까지 `Release`를 내부 전용으로 두거나(Wails 공개 제거), 호출자별 lease 참조 수를 둔다.

### N-03 정확히 줄 수와 같은 `max_lines` (P3)

**근거**: parse 루프 맨 앞에서 `line >= MaxLines`면 EOF 확인 없이 `SelectionReason="max_lines"`로 종료한다.

**재현**(`TestR2MaxLinesExactCount`): 3줄 파일에 `{"max_lines":3}`을 주면 `complete_records:3`, `partial:false`, `selection:"max_lines"`가 기록된다. 잘린 것이 없는데 coverage가 선택 적용을 주장한다.

**수정 제안**: 다음 줄이 있는지 확인한 뒤에만 선택 표지를 남기거나, 표지를 "옵션이 적용됨"으로 정의하고 문서화한다. legacy parity(summary 등)는 이미 일치한다.

### N-04 projection 크기 상한 없음 (P3)

**근거**: collection 투영이 행마다 컬럼별 항목을 `series.measurement_availability`에 추가한다. 입력 table이 상한(레코드 10만, 컬럼 수)까지 커지면 export용 사본이 행×컬럼 배수로 커진다. 코드 정독으로 확인했고 크기 측정은 하지 않았다.

**수정 제안**: 투영 항목 수 상한을 두거나, 전부 observed인 컬럼은 요약 한 줄로 압축한다. 상한 초과 시 명시적 오류 코드를 반환한다.

## 4 검증 범위

### 실행한 것 (Mac arm64)

| 항목 | 결과 |
|---|---|
| `go vet ./...` | 통과 |
| `go test -race -count=1 ./...` | 실패·race 0 |
| Windows amd64 `go vet ./internal/analysis ./internal/ingestion` | 통과(교차 vet. 실행 아님) |
| 프런트엔드 `tsc --noEmit`, `npm run test:state` | 통과 |
| 실제 CLI 깊은 profile (256~4000프레임) `report json`, `evidence export` | 전부 성공 |
| 재현 테스트 4건(scratch worktree, 미커밋) | N-01·N-02·N-03 재현, 위 3절 |
| 증거 파일 `source_sha256` 40개, 공유 자산 바이트 | 일치 |

### 코드·테스트 정독으로만 확인한 것

R-06·R-09·R-11·R-12·R-13·R-14는 수정 diff와 작성 측 회귀 테스트(전체 통과)로 확인했고 별도 재현은 하지 않았다. 신규 의존성 `santhosh-tekuri/jsonschema/v6`(go.mod)가 폐쇄망 빌드에서 모듈 캐시 없이 해결되는지는 확인하지 않았다. 이 환경에서는 빌드가 통과했다.

### 실행하지 않은 것 (기존 미검증 항목과 구분)

- 실제 WebView IPC/UI: 새 import API를 호출하는 화면이 없다(T-631). Go 메서드 직접 호출과 문자열 전송 모사까지만 검증됐다.
- Windows 네이티브 실행: `LockFileEx` 기반 저장소 잠금, rename·삭제, cache 경로 동작은 미확인. 교차 빌드·vet만 있다.
- hosted CI, 실제 producer 로그, 장시간 실사용 부하.
- 저장소 상한 도달(1,024개·512 MiB)과 재시작 시 `initStore` 해시 계산 비용은 측정하지 않았다.

## 5 처리 권장 순서

1. T-629 착수. N-01과 독립적이다.
2. **N-01**: 비교 가능성과 관측 대상 식별을 분리하거나 계약을 정정한다. T-631·T-635 전에 닫는다.
3. N-02: T-630 수용 기준에 유지하고, 그 전에는 `ReleaseEvidenceResult` 공개 범위를 재검토한다.
4. N-03·N-04: 해당 영역을 손댈 때 처리한다.
5. N-01 정리 후 T-628의 `DONE` 복귀를 판단한다. 독립 재리뷰(이 문서) 이후에도 Windows 네이티브·WebView IPC 증거는 별도 작업이다.
