# T-631 결과 capability provider·공통 소비자 구현 독립 코드 리뷰 — 2026-11-11

- 대상: T-631 구현 커밋 `ff06c54`(feat(results): implement T-631 generic capability consumers). 이후 `e2c1c7d`·`4bdd10a`까지 T-631 관련 파일(`resultcontract`, `analysis/providers.go`, `resultproviderservice.go`, `EvidenceResultPanel.tsx`, `resultProviders.ts`, `cmd_resultproviders.go`)에는 변경이 없다.
- 계약 기준: `docs/ko/DATA_MODEL.md` "T-631 결과 provider와 제한된 공통 소비자", `docs/ko/PARSER_DESIGN.md`의 T-631 절, `work_status.md`의 T-631 수용 조건
- 증거: `docs/review/evidence/2026-11-11-t631/local.json` (`source_sha256` 27개 항목이 `ff06c54`의 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경은 수정하지 않았다.
- 방법: 소스 정독, 저장소 밖 scratch worktree(`ff06c54`)에서 재실행·재현(미커밋 throwaway probe 테스트), 이 커밋에서 빌드한 실제 CLI로 `evidence describe/query/compare` 실행.
- 범위: T-631(provider 레지스트리, 분석 서비스 메서드, Wails/CLI 서비스, Workspace 결과 패널). producer별 timeline/SLI 어댑터와 case 검색·baseline은 T-634/T-635/T-638이므로 평가하지 않는다.

## 1 결론

**판정: 조건부.** capability를 가져온 문자열이 아니라 컴파일된 선언에서만 부여하는 원칙, 정밀 값의 문자열 전달, 페이지·셀 상한, 선언 identity 표기, 비교 거절의 숫자 delta 제거는 계약대로 동작한다. 다만 **timeline/SLI provider의 payload 검증이 행 수의 제곱에 비례해 느려지고 그 검증이 모든 describe·페이지 조회마다 반복되며**(R8-01), 이는 T-631이 전달하는 계약의 핵심 기능이므로 producer가 이를 등록하기 전에 고쳐야 한다. P1은 없다. P2 1건, P3 3건이다.

- **T-631 `DONE` 판정:** R8-01 처리 후 가능하다. 현재 내장 descriptor가 timeline/SLI provider를 등록하지 않아 지금 사용자에게 보이는 영향은 없다.
- **T-634·T-638·T-640 착수:** R8-01은 첫 timeline/SLI producer 등록 전에 닫는다. T-635는 이 리뷰로 막히지 않는다.
- 실제 WebView IPC/UI(작성 측 macOS 관찰 외), Windows 네이티브 실행, hosted CI는 미검증이다(5절).

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| 가져온 capability 문자열은 기능을 부여하지 않음 | 충족 | 결과 metadata에 `capabilities: ["timeline","compare"]`를 넣어도 provider 목록에 timeline/compare가 없다(작성 측 테스트 정독, 실제 CLI에서 알 수 없는 type의 `describe`는 generic 4종과 표 fallback만 나열). |
| 정밀 값·null·0·부재 구분 | 충족 | CLI `query`: `9007199254740993`, `1E-400`, `0`이 문자열 그대로, 미측정은 `—`+`null:true`, 중첩 객체는 `[structured value: backend export]`로 표시된다. 부재 필드는 `availability: absent`. |
| 페이지·열·셀 상한 | 충족 | 한 행이 45열이면 32열로 줄고 `truncated:true`. 101행 요청은 `invalid page bounds`로 거절. 표가 200개일 때 provider 목록은 128개로 절단되고 `truncated:true`(출력 26 KiB). |
| 알 수 없는 type은 읽기 전용 fallback | 충족 | `compatibility: {allowed:false, reason:"unknown_result_type", readOnly:true}`이고 요약·진단·표 provider만 제공. 차트는 `renderer_not_registered`로 비활성. |
| 비교는 등록된 scalar에만, 순서 있는 digest와 선언 identity | 충족 | 내장 Tomcat 결과 자기 비교가 delta 0, percent 0, `identity_basis: declared`, baseline→target digest를 반환. 옵션이 다른 결과는 숫자 없이 `selection_mismatch`와 사유만 반환. 등록되지 않은 경로(`generic:summary`)는 비교 provider가 아니다. |
| 고정 handle·digest에 묶인 페이지 | 충족(코드·프런트 테스트) | `ProviderPage`가 handle과 `result_sha256`을 담고 `validateProviderPage`가 불일치를 거절한다. |
| 취소·늦은 응답 억제 | 충족(코드·상태 테스트) | `ResultRequestScope`가 요청 세대를 올려 늦은 결과를 버리고, 늦게 도착한 import job ID는 취소한다. lease는 획득·해제가 짝을 이룬다. |
| 실제 서버 이벤트·ProfileSet 상세 provider | 충족 | Tomcat 결과에서 `server:events` 상세 페이지(11열, 총 2행)를 CLI로 조회했다. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R8-01 | **P2** | timeline/SLI payload 검증이 행 수의 제곱으로 느려지고 매 describe·페이지 조회마다 반복됨 | `resultcontract/providers.go` `validateProvider`, `resultcontract/contract.go` `referenced` |
| R8-02 | P3 | 모든 describe·페이지 호출이 결과 전체를 다시 읽고 디코딩하며 전체 행을 훑음 | `analysis/providers.go`, `resultcontract/providers.go` `Project` |
| R8-03 | P3 | 가져온 문자열의 BiDi·제어 문자가 그대로 표시됨 | `resultcontract/providers.go` `displayCell`, `EvidenceResultPanel.tsx` |
| R8-04 | P3 | 비교 대상이 호환되지 않을 때 타입화된 거절이 아닌 일반 오류가 반환됨 | `resultcontract/contract.go` `Compare` |

## 4 Finding 상세

### R8-01 timeline/SLI 검증의 제곱 비용 (P2)

**근거**: `validateProvider`는 timeline 행마다 `referenced(r, "clock_ids", …)`와 `referenced(r, "evidence_ids", …)`를 호출하고, SLI 행마다 `referenced(r, "population_ids", …)`를 호출한다. `referenced`는 호출할 때마다 `Metadata.Extra[key]` 전체를 `json.Marshal`한 뒤 다시 `wirejson.Decode`해 슬라이스를 만들고 선형 탐색한다(`contract.go:405-413`). 행이 N개이고 참조 ID 목록도 N개면 총 O(N²)에 JSON 변환이 N번 곁들여진다. 이 검증은 `Capabilities`에서 일어나므로 `DescribeResult`, `QueryResult`(페이지마다), `CompareResult`가 모두 반복해서 치른다.

**재현**(Mac arm64, throwaway probe: 임시 descriptor로 timeline provider를 등록하고 행마다 고유 `evidence_id`를 둔 결과의 `Capabilities` 시간):

| 행 수 = evidence ID 수 | `Capabilities` 1회 |
|---|---|
| 2,000 | 1.2 s |
| 8,000 | 18.9 s |
| 16,000 | 75.5 s |

두 배가 되면 시간이 약 4배가 된다. 결과는 모든 경우 `available`로 정상이다. 즉 올바른 입력이 거절되는 것이 아니라 비용이 문제다.

**영향**: 내장 descriptor 중에는 아직 timeline/SLI provider를 등록한 것이 없어 지금은 드러나지 않는다. 그러나 이 계약은 T-631이 전달하는 기능이며, 이벤트마다 evidence 참조를 가진 timeline은 수천~수만 행이 일반적이다. 첫 producer가 등록되는 순간 화면에서 결과를 열 때와 페이지를 넘길 때마다 수십 초가 걸린다. 취소는 행마다 확인되어 중단은 되지만 사용 불가에 가깝다.

**수정 제안**:
1. 참조 ID 집합(`clock_ids`/`evidence_ids`/`population_ids`)을 검증 시작 시 한 번 `map`으로 만들고 행마다 조회한다. `referenced`는 호출마다 다시 변환하지 않는다.
2. provider 검증 결과를 결과 digest 단위로 캐시한다(같은 handle의 describe·페이지·비교가 재검증하지 않도록).
3. 행 수 8,000·16,000인 timeline/SLI 회귀 테스트와 시간 상한(예: 1 s 이내)을 추가한다.

### R8-02 호출마다 결과 전체를 다시 읽고 훑음 (P3)

**근거**: `DescribeResult`와 `QueryResult`는 매번 `Service.Read`로 저장 파일 전체를 읽고 디코딩한 뒤(`providers.go`), `Project`는 요청한 25행을 만들기 위해 표의 **모든 행**을 `objectRow`로 순회하며 열 집합을 계산하고(`providers.go` `Project`) 그 앞에서 `Capabilities`도 다시 계산한다.

**재현**(throwaway probe, 10개 문자열 열 + 숫자 열, 일반 표):

| 행 수 | 결과 크기 | describe | 25행 페이지 |
|---|---|---|---|
| 10,000 | 2 MiB | 50 ms | 51 ms |
| 50,000 | 11 MiB | 248 ms | 256 ms |
| 100,000 | 22 MiB | 500 ms | 513 ms |

첫 페이지와 마지막 페이지의 시간이 같다. 100,000행을 25행씩 끝까지 넘기려면 4,000번 호출이고 호출마다 약 0.5 s다.

**영향**: 지금 UI는 25행 단위로 넘기므로 큰 표에서 매 클릭이 반 초 걸린다. 오류는 아니지만 "제한된 페이지" 목표와 어긋나고 R8-01과 같은 원인(재계산)이다.

**수정 제안**: handle·digest별로 디코딩된 결과와 열 집합·provider 목록을 한 번만 계산해 캐시(수명은 handle과 lease를 따름)하고, 페이지 호출은 해당 구간만 직렬화한다.

### R8-03 BiDi·제어 문자 무가공 표시 (P3)

**근거**: `displayCell`은 문자열 값을 길이만 제한하고 그대로 셀 텍스트로 전달하며 UI는 그대로 렌더링한다. 가져온 결과의 문자열은 신뢰할 수 없는 입력이다.

**재현**: 요약 값 `"abc‮def\u0007"`가 CLI `query` 결과에서 `'abc‮def\x07'` 그대로 나온다(BiDi 우회 문자와 BEL이 제거·표시되지 않는다).

**영향**: 파일 이름·메시지처럼 사람이 읽는 셀에서 U+202E 같은 문자로 표시 순서가 뒤집히거나 제어 문자가 보이지 않게 섞여 사용자를 오도할 수 있다. 정밀 값 원문은 내보내기에 보존되므로 변조는 아니고 표시 문제다.

**수정 제안**: 표시 셀에서만 BiDi 제어·C0/C1 제어 문자를 보이는 기호(예: `⟦U+202E⟧`)로 바꾸고 `escaped:true` 같은 표시를 남긴다. 내보내기와 원본 값은 바꾸지 않는다.

### R8-04 호환되지 않는 비교 대상이 일반 오류로 반환됨 (P3)

**근거**: `Registry.Compare`는 양쪽 결과의 `Check(…, "compare")`가 거절되면 타입화된 `ComparisonError`가 아니라 `fmt.Errorf(reason)`을 반환한다(`contract.go:556-559`). `CompareResult`는 `ComparisonError`만 구조화된 거절로 바꾸고 나머지는 오류로 그대로 올린다. DATA_MODEL은 "타입화된 거절 코드를 숫자 delta 없이 반환"한다고 적는다.

**재현**(실제 CLI): 기준을 Tomcat 결과로 하고 대상을 알 수 없는 type의 결과로 `evidence compare`를 실행하면 `{"allowed":false,"code":…}` 구조가 아니라 `archscope-engine: unknown_result_type`이라는 종료 오류가 나온다. 옵션이 다른 두 Tomcat 결과는 정상적으로 `selection_mismatch` 구조 거절을 반환한다.

**영향**: Workspace의 비교 대상 목록은 backend handle이 있는 모든 항목을 보여 주므로 비호환 대상을 고를 수 있다. 이때 UI는 `request_failed` 오류로 보여 주고 사유 코드·안내가 구조화되지 않는다.

**수정 제안**: `Compare`의 호환성 거절을 `ComparisonUnsupported…` 같은 타입화된 코드로 감싸 `ProviderComparison{allowed:false, code, reason}`로 반환하고, UI는 호환되지 않는 항목을 대상 목록에서 비활성화한다.

## 5 검증 범위

### 실행한 것 (Mac arm64)

| 항목 | 결과 |
|---|---|
| `go vet ./...` | 통과(출력 없음) |
| 전체 `go test -race -count=1 ./...` | 96개 패키지 통과, 실패 없음 |
| Windows amd64 교차 `go vet`(`resultcontract`, `analysis`, `cmd`) | 통과 |
| 프런트엔드 `tsc --noEmit`, `npm run test:state`(`ff06c54` worktree) | 통과 |
| 실제 CLI(이 커밋에서 빌드) | fallback describe·query(정밀 값, 중첩, null, 제어 문자), 표 200개·45열 절단, 101행 거절, Tomcat 결과 상세 페이지·자기 비교·옵션 불일치 거절·비호환 대상(R8-04) |
| 재현 probe(scratch worktree, 미커밋) | R8-01 시간 측정, R8-02 페이지 시간 측정 |
| 증거 `source_sha256` 27개 | 일치 |

### 코드·테스트 정독으로만 확인한 것

`ResultRequestScope`의 요청 취소·세대 처리, import job 늦은 ID 취소, lease 획득·해제 순서(`EvidenceResultPanel`), `addToWorkspace`의 표시 전용 요약 투영은 정독과 작성 측 상태 테스트로 확인했고 실제 UI에서 재현하지 않았다. provider 목록이 128개로 절단될 때 알파벳 순서상 뒤에 오는 표(예: `wide`)가 목록에서 빠지는 것은 문서에 적힌 제한이며(내보내기로 확인) finding으로 올리지 않았다. 작성 측이 기록한 macOS 네이티브 WebView 관찰(import·상세·비교·fallback·페이지·Workspace 카드)은 직접 재현하지 않았다.

### 실행하지 않은 것 (기존 미검증 항목과 구분)

- 실제 WebView IPC/UI: 이 리뷰에서 패널을 눌러 보지 않았다.
- Windows 네이티브 실행: 교차 vet만 있다.
- hosted CI, 실제 producer의 timeline/SLI 입력(등록된 producer가 아직 없다).
- 큰 결과에서의 Wails 직렬화 지연과 메모리 사용량.

## 6 처리 권장 순서

1. **R8-01**: 참조 ID 집합 선계산과 검증 결과 캐시, 16,000행 회귀 테스트. 첫 timeline/SLI producer 등록 전에 닫는다.
2. R8-02: R8-01의 캐시 설계에 디코딩된 결과·열 집합을 함께 넣는다.
3. R8-04: 비교 대상 호환성 거절을 타입화하고 UI 대상 목록을 걸러낸다.
4. R8-03: 표시 계층에서 BiDi·제어 문자를 무력화한다.
