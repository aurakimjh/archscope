# T-628 공통 기반 구현 독립 코드 리뷰 — 2026-11-11

- 대상 커밋: archscope `a698d26` (feat(ingestion): implement T-628 catalog jobs and exact wire contracts), 기준 `eb3e782`
- 공유 자산: projects-assets `8936c2b`. bundle된 `manifest.json`·`server.log`·`wire-cases.json`·`measurement-descriptor.schema.json`이 공유본과 바이트 단위로 같음을 sha256으로 확인했다.
- 계약 기준: `docs/ko/PARSER_DESIGN.md` C1~C4(등록·선택·실행·Job 계약과 구현 기록), `docs/ko/DATA_MODEL.md` W01~W10(T-628 측정과 호환 상세 계약, wire 구현과 검증 연결)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. **처리는 다른 AI가 한다.** 구현 코드, `work_status.md`, 기존 미커밋 HTTP 변경(`docs/*/SYSTEM_HTTP_CAPTURE.md`, `scripts/verify-http-desktop.mjs`)은 수정하지 않았다.
- 방법: 소스 정독과 계약 대조를 했고, Mac(arm64)에서 다음을 실행했다.
  - 테스트 재실행
  - 실제 CLI 실행으로 회귀 재현. 부모 커밋 `eb3e782`를 별도 worktree에 빌드해 비교했다.
  - 리뷰 전용 재현 테스트 9건. 저장소 밖 scratch worktree에서만 실행했고 커밋하지 않았다.
- 증거 무결성: `docs/review/evidence/2026-11-11-t628/local.json`의 `sourceSHA256` 53개 파일 해시가 `a698d26`의 커밋 내용과 모두 일치한다.

## 1 결론

**판정: 조건부. T-628의 `DONE` 판정은 보류하고 재작업이 필요하다.** 계약의 골격은 대부분 설계대로 구현되어 있다.

- immutable catalog
- verified/hint 선택
- owned snapshot과 두 번째 hash 확인
- 동기 bounded sink
- 같은 mutex로 직렬화한 cancel/commit
- 안전 partial 조건
- 정밀도 encoding 검증
- operation별 버전 차단

panic 복구, `needs_selection` 중 취소, 이전 리비전 보존도 재현 테스트로 확인했다.

다만 **P1 1건**이 남아 있다. 새 bounded JSON decoder가 기존 `report` CLI와 새 evidence Load 경로 전체에 깊이 64 상한으로 적용됐다. 그 결과 **31프레임 이상 스택을 가진 profiler 결과를 더 이상 열 수 없다.** 부모 커밋에서는 같은 파일이 정상 처리된다. 기존 CLI·프로파일 호환을 깨는 회귀이고, T-629가 다룰 바로 그 입력이다.

P2 3건도 있다. 이들은 후속 소비자(T-631·T-632·T-635)가 이 기반 위에 쌓이기 전에 고쳐야 한다.

- job 이력 미회수로 128회 후 영구 거부
- 조립 시점 교차 검증 누락
- 서로 다른 리비전의 정확 비교가 사실상 불가능하거나 label만 신뢰

**T-629 착수: 현재 상태로는 불가.** R-01을 고치고 깊은 profile 회귀 테스트를 추가하면 착수할 수 있다. R-02·R-03은 T-629와 병행해도 되지만 T-631 전에 닫아야 한다. 7절에 순서를 적었다.

| 검토 축 | 판정 | 관련 |
|---|---|---|
| catalog ID·alias·schema·family 검증, 모호한 형식 선택 | 선택 로직은 적합. 조립 시 교차 검증 누락, 무관한 detector 오류가 전체 차단 | R-03, R-07 |
| SourceLease 일관성, 자원 제한, 취소·partial·atomic commit·cleanup | 핵심 경로 적합. job 이력 미회수, 대기 시간이 timeout 소모, Load 무제한 누적 | R-02, R-08, R-13 |
| 숫자 정밀도, unknown metadata, null/0/부재, 버전별 연산 차단 | 값 검증은 적합. 비교의 모집단 동일성 미정의, measurement patch 버전 모순 | R-04, R-06, R-12 |
| 기존 CLI·Wails·프로파일·thread 호환과 테스트 누락 | **깊은 profile 결과 회귀(P1)**, max_lines 의미 변경, correlation key 누락 | R-01, R-05, R-10, R-11, R-14 |

## 2 Finding 요약

등급 기준은 다음과 같다.

- P1: T-628 완료와 T-629 착수 전에 수정한다.
- P2: 해당 기반을 소비하는 다음 작업(T-631/T-632/T-635) 전에 수정한다.
- P3: 개선 또는 문서 정정.

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R-01 | **P1** | 깊이 64 JSON 상한이 전체 결과에 적용되어 31프레임 이상 profiler 결과의 `report *`·`evidence export`가 실패(회귀) | `cmd/archscope-engine/helpers.go:121`, `internal/wirejson/decode.go:22`, `internal/resultcontract/contract.go:199-201` |
| R-02 | P2 | job map을 회수하지 않아 프로세스당 128번째 이후 `Start`가 영구 거부 | `internal/analysis/service.go:120,125` |
| R-03 | P2 | catalog 조립이 schema 역할·descriptor family·결과 type·result registry 등록을 교차 검증하지 않음 | `internal/ingestion/importer.go:197-223`, `internal/catalog/builtin.go:141-169` |
| R-04 | P2 | 리비전 간 정확 비교에서 모집단 동일성이 label 문자열로만 판정되고, label이 없으면 서로 다른 import끼리 비교할 수 없음 | `internal/resultcontract/contract.go:511-520` |
| R-05 | P3 | `max_lines` 옵션이 legacy와 달리 partial로 분류되어 `--allow-partial` 없이는 실패 | `internal/parsers/serverlog/importer.go:93-96`, `internal/analysis/service.go:390-399` |
| R-06 | P3 | measurement extension 범위는 `[1.0.0,1.1.0)`인데 검증은 `"1.0.0"`만 허용, 알 수 없는 descriptor 필드는 거부 | `internal/resultcontract/contract.go:189-192,230`, `internal/catalog/builtin.go:106` |
| R-07 | P3 | 무관한 detector 하나의 오류가 명시 선택 import까지 전부 실패시킴 | `internal/ingestion/importer.go:263-265`, `internal/analysis/service.go:297-301` |
| R-08 | P3 | Load가 job 상한 밖에서 매번 새 리비전을 저장하고, 결과·staging 회수 경로가 없음 | `internal/analysis/results.go:45-94`, `cmd/archscope-app/evidenceservice.go:29-37`, `internal/analysis/service.go:82-92` |
| R-09 | P3 | JobProblem의 retryable이 항상 false이고 원인 분류가 부정확(I/O 오류 → 예산 초과) | `internal/analysis/service.go:261`, `internal/parsers/serverlog/importer.go:84-86`, `internal/ingestion/importer.go:256-262` |
| R-10 | P3 | 새 경로의 server_log 결과에서 legacy의 `metadata.correlation_keys`가 빠짐(문서화 없음) | `internal/analyzers/serverlog/analyzer.go:180-189` |
| R-11 | P3 | 적합성 manifest의 `expected` 값과 measurement JSON Schema가 실행 검증에 쓰이지 않음 | `internal/catalog/contracts/manifest.json`, `internal/catalog/builtin.go:171-205`, `internal/resultcontract/contract.go:164-177` |
| R-12 | P3 | 표시 projection이 `series.measurement_availability`를 무조건 덮어쓰고, summary descriptor만 투영 | `internal/resultcontract/presentation.go:28-50` |
| R-13 | P3 | job timeout이 slot 대기 시간부터 소모됨 | `internal/analysis/service.go:117,263-269` |
| R-14 | P3 | profiler Metadata의 Wails TS 타입이 `any`가 되어 프런트엔드 타입 검사가 사라짐 | `frontend/bindings/.../internal/profiler/models.ts:410` |

## 3 P1 상세

### R-01 깊이 64 JSON 상한이 전체 결과 읽기에 적용되어 profiler 결과 호환이 깨짐

**근거**

- `wirejson.DefaultLimits`는 깊이 64, 64 MiB, 배열 1,000,000개, 객체 key 100,000개다(`internal/wirejson/decode.go:22`). 설계(DATA_MODEL "T-628 wire 구현과 검증 연결")는 이 decoder를 **metadata·opaque extension의 read boundary**로 기술한다.
- 이 commit은 같은 기본 상한을 **결과 전체**를 읽는 경로에도 적용했다.
  - `cmd/archscope-engine/helpers.go:121` `readJSONFile`: `report html|json|csv|pptx`의 입력. 기존에는 `json.Unmarshal`이었다.
  - `internal/exporters/reportdiff/exporter.go:193,197,403`: `report diff`의 입력.
  - `internal/resultcontract/contract.go:199-201` `Decode`: `analysis.Service.Load`(`internal/analysis/results.go:74`)와 `Read`(`internal/analysis/service.go:513`)에서 쓴다. 즉 `evidence export`와 Wails `LoadEvidenceResult`의 입력이다.
- profiler flame tree는 프레임마다 JSON 깊이가 2씩 늘어난다(`children` 배열 + 노드 객체). 30프레임 결과의 실측 깊이가 정확히 64였고, 31프레임부터 상한을 넘는다. Java·Python 실서비스 스택은 대개 30프레임을 넘는다.
- `catalog.ResultRegistry`는 `profiler_collapsed`·`profiler_jennifer`·`profiler_diff`·`profile_evidence`를 display/save/json/csv/html/report-pack reader로 등록했다(`internal/catalog/builtin.go:91-92`). 그러나 이 등록이 실제로 동작하는 것은 얕은 fixture뿐이다. W10의 "legacy profiler opaque 왕복" 테스트도 얕은 결과만 사용한다.
- 64 MiB 상한도 새로 생겼다. 64 MiB를 넘는 기존 분석 결과(대형 access log 등)를 `report *`로 열던 경로가 거부된다. 실측은 하지 않았고 코드로 확인했다.

**재현** (Mac arm64. `engine`은 `a698d26`, `engine-base`는 `eb3e782` 빌드)

```bash
python3 -c "print(';'.join('com.example.Frame%d.call'%i for i in range(40))+' 10')" > deep40.collapsed
engine profiler analyze-collapsed --in deep40.collapsed --out deep40.json   # JSON 깊이 84
engine report html --in deep40.json --out deep40.html          # exit 1: JSON depth budget exceeded
engine report json|csv|pptx --in deep40.json --out ...          # 모두 exit 1
engine report diff --before deep40.json --after deep40.json --out d.json   # exit 1
engine evidence export --in deep40.json --export-format json --out x.json  # exit 1
engine-base report html|json|csv|pptx|diff ... (같은 입력)       # 모두 exit 0
```

경계값: 29·30프레임은 통과(깊이 64), 31·32프레임은 실패했다.

**영향**

- 기존 사용자의 CLI 보고서 재생성과 비교가 실제 profiler 결과에서 실패한다.
- 새 evidence 경로는 등록된 profile reader 유형을 실질적으로 열지 못한다. T-629(ProfileSet → `profile_evidence`)가 바로 이 decoder로 결과를 저장·재열기·비교하게 되므로 선행 차단 요인이다.
- Wails 기존 분석 경로는 `models.Metadata`/`profiler.Metadata`만 wirejson으로 읽고 charts는 표준 decoder로 읽으므로 이번 회귀의 직접 영향은 없다. 코드로 확인했고 실행은 하지 않았다.

**수정 제안**

1. 상한을 용도별로 분리한다.
   - metadata·extension·option·record용 상한(현행 유지 가능)
   - 앱이 만든 결과 전체를 읽는 상한
2. 전체 결과의 깊이 상한은 현실적인 스택 깊이를 수용해야 한다. 예를 들어 sink의 `StackDepth` 1,024의 2배에 여유를 둔 값으로 하고, 설계 문서에 근거를 적는다. 바이트 상한은 `Budgets.DecodedBytes`와 일치시킨다. legacy `report *` 경로는 기존 무제한 동작을 유지할지, 명시적 상한과 오류 메시지를 둘지 결정해 문서화한다.
3. 회귀 테스트를 추가한다.
   - 256프레임 이상 profiler 결과로 `report html|json|csv|pptx|diff`
   - `evidence export`와 `Service.Load` → `Export`
   - Wails `LoadEvidenceResult`
4. DATA_MODEL W05의 "깊은 JSON 거부" 서술이 어느 boundary에 적용되는지 정정한다.

**완료 기준**: 위 재현 명령이 모두 exit 0이다. 깊은 결과가 depth 오류 없이 처리되고, 의도된 상한을 넘는 입력만 명시적 코드로 거부된다.

## 4 P2 상세

### R-02 job 이력을 회수하지 않아 128번째 이후 import가 영구 거부됨

**근거**: `Start`는 `len(s.jobs) >= 128`이면 `IMPORT_BUDGET_EXCEEDED: service job capacity reached`를 반환한다(`service.go:120`). 그런데 `s.jobs`에서 항목을 지우는 코드가 저장소 어디에도 없다(grep으로 `delete(s.jobs` 0건 확인). 데스크톱은 앱 프로세스당 Service 하나를 지연 생성해 공유한다(`evidenceservice.go:23-43`). 설계 구현 기록의 "활성 job 4개, process 내 job history 128개로 제한"은 이력을 회수한다는 뜻으로 읽힌다. 현재 구현은 프로세스 수명 동안 받을 수 있는 import 수를 128로 제한한다.

**재현**: 존재하지 않는 경로로 128개 job을 시작해 모두 `failed`로 끝낸 뒤 129번째 `Start`를 호출하면 `IMPORT_BUDGET_EXCEEDED: service job capacity reached`가 반환된다(재현 테스트 `TestReproJobHistoryNeverEvicted`). 실패·취소 job도 슬롯을 계속 차지한다.

**영향**: 현재는 이 API를 호출하는 UI 화면이 없다(T-631). CLI는 명령마다 Service를 새로 만든다. 그래서 잠복 결함이지만, generic job 화면이 붙는 순간 장시간 세션에서 재시작 전까지 import가 불가능해진다.

**수정 제안**: 활성 job(비 terminal) 수와 이력 수를 분리한다. 이력이 128을 넘으면 가장 오래된 terminal job부터 회수하고, 회수된 ID 조회는 `IMPORT_JOB_STALE`로 응답한다. 129·200번째 Start 테스트를 추가한다.

### R-03 catalog 조립이 역할·family·결과 type을 교차 검증하지 않음

**근거**: `NewCatalog`(`importer.go:197-223`)가 검사하는 것은 다음뿐이다.

- 네 schema ref가 등록되어 있는지(존재 여부만)
- `RecordFamily == NormalizerInput`인지
- `NormalizerOutput == analyzer.Family`인지

다음은 검사하지 않는다.

- 각 ref의 **역할**: option 자리에 record schema를 넣어도 통과한다.
- descriptor `Family`와 analyzer `ResultType`·`ResultSchema`의 대응.
- analyzer `ResultType`이 `resultcontract.Registry`에 등록되어 있는지. catalog와 registry는 `Builtin()`에서 따로 만들어진다(`builtin.go:163-168`).

schema hash는 문서용 `Definition` 텍스트만 고정하고, 실제로 실행되는 Go `Validate` 함수와의 일치는 보장하지 않는다(`builtin.go:28-85`).

**재현**: 다음 조건의 catalog가 `NewCatalog` 오류 없이 생성됐다(`TestReproCompositionNotCrossChecked`).

- `OptionSchema`에 record schema를 넣음
- `Family`를 `gpu_metrics`로 지정
- analyzer `ResultType`을 `unregistered_result`로 지정

이 catalog로 import하면 job마다 실행 단계에서야 `IMPORT_RESULT_INVALID`로 실패한다.

**영향**: 설계는 이런 오류를 조립 시 `IMPORT_REGISTRY_INVALID`로 막고 "검증되지 않은 일부 등록으로 계속 진행하지 않음"을 요구한다(PARSER_DESIGN 등록과 형식 선택 계약, 오류 코드 표). 지금 구조로는 T-632 선언형 pack이나 T-645 adapter가 잘못 등록되어도 활성화되고, 사용자 입력마다 실패로 드러난다.

**수정 제안**

- catalog와 result registry를 하나의 조립 함수에서 함께 검증한다.
- schema에 역할 표지(input/option/record/result)를 두고 descriptor slot과 대조한다.
- `ResultSchema`와 analyzer `ResultType`과 registry descriptor의 일치를 검사한다.
- 각 schema의 `Validate`를 bundled 적합성 fixture로 한 번씩 실행해 Definition과의 drift를 잡는다.
- 각 경우에 대한 negative 테스트를 추가한다.

### R-04 리비전 간 정확 비교의 모집단 동일성이 정의되지 않음

**근거**: `Registry.Compare`(`contract.go:489-550`)는 두 결과의 measurement가 kind·unit·scope·aggregation·`population_ref`에서 같은지 문자열로 비교한다. `population_ref`가 비어 있으면 두 결과의 JSON 전체가 바이트 단위로 같을 때만 허용한다(`contract.go:514-520`).

- `population_ref`는 각 결과 **안의** `population_ids`에서만 해석한다(`contract.go:340-348,353`). 그래서 서로 무관한 두 결과가 같은 label(예: `"all"`)을 쓰면 같은 모집단으로 취급된다. 설계는 "서로 다른 리비전의 비교에는 일치하는 명시적 population reference"를 요구하지만, 리비전을 넘는 동일성을 판정하는 규칙(source hash, 선택 범위, option hash 등)이 없다.
- pilot인 server_log measurement에는 population이 없다. 같은 원본을 두 번 import해도 `created_at`·handle이 달라 비교가 거부된다.

**재현**: 같은 `server.log`를 두 번 import한 뒤 `Compare(a, b, "/summary/total_events")`를 호출하면 `population reference required for cross-revision comparison`이 반환된다(`TestReproExactCompareDistinctImportsRefused`). W07 테스트(`contract_test.go:94`, `evidenceservice_test.go:151`)는 **같은 결과 객체끼리** 비교한 경우만 다룬다.

**영향**: `CompareEvidenceScalar`가 공개된 Wails 메서드인데, 실제 자료에서는 항상 거부되거나 label만 맞추면 통과한다. T-631 compare provider와 T-635 baseline이 이 동작을 그대로 상속한다.

**수정 제안**: 모집단 정체성을 descriptor 수준에서 정의한다. 예: `population_ref` → `{id, source_sha256 set, selection/options hash, semantic revision}`. Compare가 이 값을 두 결과에서 각각 해석해 비교하게 한다. pilot server_log에 import provenance 기반 population을 부여하거나, 비교 불가를 명시적 사유 코드로 반환한다. 서로 다른 두 import로 동일·상이 모집단 사례를 테스트한다.

## 5 P3 상세

### R-05 `max_lines`가 partial로 분류되어 legacy와 동작이 다름

Tomcat importer는 `max_lines` 도달 시 `SafeBoundary=true, PartialReason="max_lines", Complete=false`를 반환한다(`importer.go:93-96`). 그래서 `--allow-partial` 없이 실행하면 `IMPORT_PARTIAL_UNSAFE`로 실패한다(`service.go:390-399`). legacy `ForEachRecord`는 같은 옵션으로 정상 완료한다(`parsers/serverlog/parser.go:276-278`).

- 재현: `{"max_lines":1}`로 실행하면 새 경로는 `failed/IMPORT_PARTIAL_UNSAFE`, legacy는 `total_events=1` 정상 결과다.
- 경계 사례: 정확히 N줄 뒤에 빈 줄만 남은 파일도 partial로 분류된다.
- C4 parity 테스트는 기본 옵션만 비교한다.
- 수정 제안: 사용자가 요청한 truncation은 coverage metadata를 남기는 `completed`로 볼지, partial로 볼지 정해 설계에 적는다. 어느 쪽이든 legacy와의 차이를 지원표에 명시하고 옵션별 parity 테스트(`max_lines`, `strict`, `top_n`)를 추가한다.

### R-06 measurement extension의 patch 버전이 모순됨

server_log descriptor는 measurement 범위를 `[1.0.0,1.1.0)`으로 선언했다(`builtin.go:106`). 그러나 `Validate`는 `versions["measurement"] != "1.0.0"`이면 거부하고(`contract.go:230`), `Measurements`는 `DisallowUnknownFields`로 추가 필드를 거부한다(`contract.go:189-192`).

- 재현: 결과의 extensions를 `1.0.1`로 바꿔 Load하면 `invalid_result: measurement extension version required`가 나온다.
- 수정 제안: 지원 범위를 단일 출처로 두고 `Validate`도 범위를 사용한다. patch 버전에서 추가 필드를 허용할지(무시·보존) 버전 정책에 적는다.

### R-07 무관한 detector 오류가 모든 import를 차단함

`DetectCandidates`는 어느 detector든 오류를 내면 전체를 `IMPORT_DETECTOR_INVALID`로 실패시킨다(`importer.go:263-265`). 서비스는 importer가 명시된 경우에도 detection을 먼저 수행한다(`service.go:297-301`).

- 재현: tomcat 외에 오류를 내는 detector를 하나 더 등록하고 `--importer tomcat`으로 실행하면 `failed/IMPORT_DETECTOR_INVALID(probing)`이 된다.
- 현재 built-in은 하나뿐이라 잠복 상태다. T-632 선언형 pack으로 detector가 늘면 pack 하나의 결함이 앱 전체 import를 막는다.
- 수정 제안: 오류를 낸 detector를 해당 후보의 진단으로 격리한다. 그 detector가 명시 선택 대상이거나 verified 충돌 판정에 필요할 때만 실패시킨다. 비유한 점수는 현행대로 등록 결함으로 기록한다.

### R-08 Load와 결과 저장소에 수명 관리가 없음

- `Service.Load`는 job slot·이력 상한 밖에서 실행되고, 호출마다 새 handle로 `results/`에 복사본을 저장한다(`results.go:45-94`). 같은 파일을 세 번 Load하면 결과 파일이 4개가 된다(import 1 + load 3).
- 결과 저장 위치는 `os.UserCacheDir()/ArchScope/evidence-imports`(`evidenceservice.go:29-37`)이고 회수 경로가 없다.
- 프로세스 crash로 남은 `staging/*` 정리도 시작 시점에 하지 않는다(`service.go:82-92`).
- macOS는 cache 디렉터리를 정리할 수 있으므로 Workspace에 저장된 `backend_result_handle`이 끊어질 수 있다.
- 설계는 case/cache 수명을 T-630으로 넘겼다. 다만 T-628이 공개한 Wails 메서드가 무제한 누적을 만들므로 다음 중 하나가 필요하다.
  - 최소 상한(개수·바이트)과 시작 시 orphan staging 정리를 추가한다.
  - 미해결 위험으로 T-630 수용 기준에 명시한다.
- Load에는 UI 취소 경로도 없다. `context.Background()`와 5분 timeout만 있다(`evidenceservice.go:92`).

### R-09 JobProblem의 재시도 가능 여부와 원인 분류가 부정확함

- `JobProblem.Retryable`은 항상 false이고 메시지는 고정 문구다(`service.go:261`). 설계는 code·stage·retryability·bounded diagnostic reference를 요구한다. 그런데 `SOURCE_CHANGED`·`IMPORT_BUDGET_EXCEEDED`(timeout)는 재시도 가능한 대표 사례다.
- Tomcat parser는 `ReadSlice`의 EOF 외 모든 오류를 `IMPORT_BUDGET_EXCEEDED "line exceeds parser buffer"`로 바꾼다(`importer.go:84-86`). 실제 I/O 오류와 lease 종료도 예산 초과로 보고된다.
- `DetectCandidates`는 부모 context 취소를 `IMPORT_BUDGET_EXCEEDED`로 반환한다(`importer.go:256-262`). 서비스는 ctx 상태로 이를 `cancelled`로 바로잡지만 catalog API를 직접 쓰는 소비자는 그렇지 않다.
- 수정 제안: `bufio.ErrBufferFull`만 예산 초과로 분류한다. detector timeout은 detector ctx의 deadline으로만 판정한다. 코드별 retryable 표와 sanitized 원인 코드를 둔다.

### R-10 새 경로의 server_log 결과에서 `correlation_keys`가 빠짐

`buildFromIterator`는 captured source metadata가 주어지면 `AttachCorrelationKeys`를 호출하지 않는다(`analyzers/serverlog/analyzer.go:180-189`). 같은 입력이라도 legacy `AnalyzeServerLog`는 `metadata.correlation_keys`를 emit하는데 새 import 결과에는 없다. C4 parity 테스트는 `Summary`만 비교한다(`service_test.go:87-91`). 문서에는 이 차이가 없다.

현재 Go와 프런트엔드에 이 key를 읽는 곳은 없다(grep 확인). 그래서 영향은 계약 차이에 그치지만, T-635 상관 분석에서 다시 필요해진다.

수정 제안: 차이를 의도한 것이면 문서화한다. 아니면 correlation key를 붙이고 parity 비교 대상을 summary·tables·metadata의 주요 key로 넓힌다.

### R-11 적합성 자산의 기대값과 schema가 실행 검증에 쓰이지 않음

- `manifest.json`의 `expected`(server_records 3, server_errors 1, server_warnings 1)를 단정하는 테스트가 없다(grep 0건). parity 테스트는 새 경로와 legacy가 서로 같은지만 본다. 둘이 함께 틀려도 통과한다.
- `measurement-descriptor.schema.json`은 hash만 확인한다(`builtin.go:196-203`). 이 schema는 `additionalProperties:false`인데, Go `Measurement`가 허용하는 `columns`·`availability_column`·`reason_column`(`contract.go:174-176`)이 schema에 없다. 공유 schema와 실행 코드의 정합성을 검사하는 장치가 없다.
- 수정 제안: manifest `expected`를 C4 테스트에서 단정한다. Go struct의 필드 집합과 schema properties를 비교하는 테스트를 둔다. collection descriptor schema를 분리 정의한다.

### R-12 표시 projection이 기존 series를 덮어씀

`ExportPresentation`은 `copy.Series["measurement_availability"]`를 무조건 대입한다(`presentation.go:50`). 분석기가 같은 이름의 series를 내면 CSV/HTML에서 원래 값이 조용히 사라진다. 또 summary descriptor만 투영하고 series/tables 컬럼 descriptor의 availability는 표시하지 않는다(`presentation.go:40-42`). 수정 제안: 예약 이름 충돌을 오류로 처리하거나 별도 namespace를 쓴다. collection 투영의 범위를 문서화한다.

### R-13 job timeout이 대기 시간부터 소모됨

job context의 timeout은 `Start` 시점에 시작된다(`service.go:117`). slot 획득은 `run` 안에서 한다(`service.go:263-269`). 활성 4개를 넘는 job은 대기하는 동안 5분 예산을 소모하고, 한 번도 실행되지 않은 채 `IMPORT_BUDGET_EXCEEDED`로 끝날 수 있다. 수정 제안: 대기 시간과 실행 시간을 분리하거나, 대기 중 상태(`queued`)를 별도 상한으로 둔다.

### R-14 profiler Metadata의 TS 타입이 `any`로 바뀜

custom marshaler 때문에 Wails 생성기가 `profiler.Metadata`를 `export type Metadata = any`로 내보낸다(`bindings/.../internal/profiler/models.ts:410`). 공통 `models.Metadata`는 이전부터 `any`였다. DiagnosticsPanel은 수동 bridge 타입으로 바꿨지만, 생성 profiler 타입을 import하는 `ProfilerAnalyzerPage.tsx`·`DiffPage.tsx`·`DrilldownPanel.tsx`의 `metadata` 하위 접근(예: `ProfilerAnalyzerPage.tsx:865`의 `timeline_scope`)은 타입 검사 없이 컴파일된다. 문서화된 결정이지만 보완 장치가 없다. 수정 제안: 수동 TS 타입(`bridge/types`)에 profiler metadata 필드를 정의하고 사용처가 그 타입을 쓰게 한다.

## 6 계약별 대조 결과

### 6.1 C1 등록과 형식 선택

| 계약 항목 | 결과 | 근거 |
|---|---|---|
| canonical ID/alias 정규식, `auto` 예약, 중복 거부, lookup 정규화 | 적합 | `importer.go:165,198-209,244-247`, `importer_test.go` |
| schema ID/version/hash resolve, 외부 URL 미사용 | 존재 확인만 적합, 역할·실행 검증 없음 | R-03 |
| record/normalizer/analyzer family 대조 | 부분 적합 | 사슬은 검사, descriptor family·결과 type은 미검사(R-03) |
| verified/hint 분리, 정렬(등급→confidence→ID), 단일 verified 자동 선택, 충돌·hint-only는 선택 요구, 비유한 점수 거부 | 적합 | `importer.go:248-302` |
| 명시 선택 시 구조 검증, verified 충돌 시 `IMPORT_FORMAT_MISMATCH`, 다른 parser로 재시도 안 함 | 적합 | `service.go:329-364`, `service_test.go:280-296` |
| 미지 producer version을 최신으로 간주하지 않음 | 형식상 적합 | 검사는 detector가 version을 보고할 때만 동작. Tomcat은 보고하지 않으므로 미적용(문서화됨) |
| fixture manifest 없는 importer의 release 제외 | 적합 | `importer.go:219-223`. `SupportStatus` 값 집합 검증은 없음 |
| legacy confidence wrapper | 테스트 전용 | `catalog.LegacyRegistry`의 운영 호출처 0건. 기존 `FormatRegistry`도 운영 호출처가 없음 |
| 기존 thread mixed/override | 유지 | thread 패키지 테스트 전부 통과 |

### 6.2 C2 실행·소유권·Job

| 계약 항목 | 결과 | 근거 |
|---|---|---|
| 선택 전 전체 파싱 금지, 같은 선택 멱등, 실행 중 변경 거부 | 적합 | `service.go:156-183`, `service_test.go:215-279` |
| stale generation | 형식상 적합 | generation이 항상 1로 고정되어 증가 경로가 없다. 설계상 재선택은 새 job이므로 수용 가능 |
| owned snapshot: SameFile·크기·mtime·두 번째 hash, parse 후 snapshot 재검증 | 적합 | `lease.go:151-213`, `service.go:406-409`, tamper 테스트 |
| parser가 닫지 않은 reader를 lease가 닫음 | 적합 | `lease.go:69-119` |
| 예산은 낮출 수만 있고 올릴 수 없음 | 적합 | `lease.go:32-58` |
| record·바이트·stack·unique key·temp 예산 | 적합 | `sink.go:30-83`. stack depth는 parser가 자기 신고한 값 |
| 취소 전 commit 금지, 취소 후 `already_terminal`, 취소 멱등 | 적합 | `service.go:184-199,466-482`. commit 경계 직전·직후의 경합 테스트는 없음(정독으로 확인) |
| `needs_selection` 중 취소 | 적합 | 재현 테스트에서 `cancelled` |
| 안전 partial(허용·importer 지원·안전 경계·완결 record·이유) | 적합 | `service.go:390-401`, 테스트 2종. 의미 차이는 R-05 |
| 실패·취소 시 staging 정리, 이전 리비전 보존 | 적합 | `service_test.go:114-124`, 재현(panic 후 staging 비어 있음) |
| compiled importer panic | 적합 | `service.go:238-240`. `IMPORT_FAILED`로 기록되고 slot이 반환됨 |
| atomic publish | 대체로 적합 | 파일 fsync 후 같은 파일시스템에서 rename한다. 디렉터리 fsync가 없어 정전 시 rename이 유실될 수 있다(경미) |
| job history 128 | **부적합** | R-02 |

### 6.3 C3 / W01~W10

| 사례 | 결과 | 비고 |
|---|---|---|
| W01 unknown object/array/null/큰 정수 | 적합(Go·opaque 문자열) | JS object를 거치는 기존 full-result 경로는 문서대로 범위 밖 |
| W02 decimal string·0 | 적합 | `numeric`의 scale·문법 검증 |
| W03 typed 우선·Extra 부활 금지 | 적합 | common·profiler 모두 |
| W04 이중 decode 원자성 | 적합 | wirejson이 임시 값에 decode한 뒤 교체 |
| W05 중복 key·깊이·NaN | 값 검증은 적합, **적용 범위 과다** | R-01 |
| W06 Pointer·target·column·encoding | 적합 | escape 처리 포함 |
| W07 null/0/부재 | 표시는 적합, 비교는 미흡 | R-04, R-12 |
| W08 operation별 required, 미지 optional | 적합 | patch 버전 모순은 R-06 |
| W09 version만 바꾼 과거 profile | 적합 | `profile_evidence 0.2.0` compute 거부 |
| W10 Wails·Report Pack 왕복 | Go 메서드·문자열 전송 모사 수준에서 적합 | 실제 WebView IPC는 미실행(8절). 깊은 profile은 R-01로 실패 |

참고: common `Metadata.Extra`는 binary로 정확히 표현되는 값만 float64로 투영한다(`wirejson.Float64Exact`). 그래서 `0.1` 같은 일반 소수는 load 후 `json.Number`로 남고, `Findings` 내부 숫자는 모두 `json.Number`가 된다. 운영 코드에서 `Extra` 값을 `float64`로 단언하는 곳은 찾지 못했다(grep). html·reportdiff·stitching은 `json.Number`를 처리한다. 지금은 결함이 아니지만 후속 consumer가 `Extra` 숫자를 `float64`로 가정하지 않도록 DATA_MODEL에 명시할 것을 권한다.

### 6.4 C4 pilot과 확장성

- Tomcat pilot은 CLI `evidence formats|import|export`와 Wails 11개 메서드에 연결됐다. 실제 CLI import→export(json)를 이번 리뷰에서도 재실행했다.
- 같은 family의 합성 adapter를 service/CLI/UI 수정 없이 추가할 수 있음은 `extension_test`가 보여 준다. 다만 이 테스트는 별도 catalog를 직접 조립한다. 운영 조립(`Builtin()`)은 형식 목록이 하드코딩되어 있고(`builtin.go:152`), 쓰이지 않는 apache/nginx alias 분기(`builtin.go:154-159`)가 남아 있다. 새 형식은 composition root 편집이 필요하다. 이는 설계가 허용한 "등록"에 해당하지만, 교차 검증이 없으므로 R-03이 선결되어야 "등록만으로 추가"가 안전해진다.

## 7 처리 권장 순서와 T-629 착수 조건

1. **R-01**: 상한 분리, 깊은 profile 회귀 테스트, DATA_MODEL W05 범위 정정. T-628 재완료 판정과 T-629 착수 조건이다.
2. **R-02, R-03**: 작은 수정이다. T-628 후속으로 함께 처리하는 것을 권한다. T-631·T-632 전에 반드시 닫는다.
3. **R-04**: 모집단 정체성 규칙을 정한다. T-631 compare provider와 T-635 baseline 전에 닫는다.
4. **R-07**: T-632 선언형 pack 전에 닫는다.
5. **R-08**: T-630 수용 기준에 반영하거나 최소 상한을 추가한다.
6. 나머지 P3: 해당 영역을 손댈 때 처리하고 지원표·문서에 반영한다.

**T-629 착수 가능 여부: 현재는 불가.** R-01은 T-629가 저장·재열기·비교할 profile 결과가 새 decoder를 통과하지 못하는 문제다. R-01 수정과 깊은 profile 회귀 테스트가 들어가면 착수할 수 있다. T-630은 R-01과 무관하게 착수할 수 있다. 다만 R-08을 수용 기준에 넣는 것을 권한다.

## 8 검증 범위와 미검증 항목

### 이번 리뷰에서 실행한 것

| 항목 | 결과 |
|---|---|
| `go vet ./...` (Mac arm64) | 통과 |
| `go test -count=1 ./...` | 93개 패키지 ok, 실패 0 |
| `go test -race` (ingestion·analysis·catalog·resultcontract·models·profiler·exporters·serverlog·threaddump·CLI) | 통과 |
| 프런트엔드 `tsc --noEmit`, `npm run test:state` | 통과 |
| CLI 회귀 비교(`a698d26` vs `eb3e782`) | R-01 재현: head 실패, base 통과 |
| 재현 테스트 9건(scratch worktree, 미커밋) | R-02·R-03·R-04·R-05·R-06·R-07·R-08 재현. panic 복구·`needs_selection` 취소는 정상 확인 |
| 공유 자산 바이트 일치, 증거 파일 해시 53개 | 일치 |

테스트 통과를 계약 충족의 근거로 보지 않았다. 위 표의 결함들은 모두 **기존 테스트가 통과하는 상태에서** 재현됐다.

### 기존 테스트에 없는 경계·실패 경로

- 31프레임 이상 또는 64 MiB 초과 결과의 `report *`·Load·Export
- 129번째 job, 장시간 세션
- commit 직전·직후의 취소·timeout 경합(정독으로는 안전하다고 판단)
- 서로 다른 두 import의 비교, 모집단 동일·상이 사례
- 조립 시 schema 역할·결과 type·registry 불일치
- 옵션별 legacy parity(`max_lines`·`strict`·`top_n`), 빈 줄로 끝나는 파일
- manifest `expected` 단정
- Acquire 도중 실패(예산 초과·복사 오류) 시 staging 정리. 서비스의 `RemoveAll`로 정리된다고 정독으로 확인
- 디렉터리 fsync 없는 rename의 내구성

### 실행하지 않은 것 (기존 미검증 항목과 구분)

| 항목 | 상태 |
|---|---|
| 실제 WebView IPC/UI | **미실행.** 이번 커밋에는 `ImportEvidence`·`LoadEvidenceResult`를 호출하는 UI 화면이 없다(generic job 화면은 T-631). 증거도 Go 메서드 직접 호출과 JSON 문자열 전송 모사뿐이다. 프런트엔드의 `backend_result_handle` Report Pack 경로는 state 테스트로만 실행되며 운영 UI에서 handle을 설정하는 곳이 없다. |
| Windows 네이티브 실행 | **미실행.** 구현 측 증거는 amd64/386 교차 빌드와 교차 vet뿐이고, 이번 리뷰는 교차 빌드도 다시 하지 않았다. Windows에서 rename 덮어쓰기, lease reader가 열린 상태의 `RemoveAll`, `UserCacheDir` 경로 동작은 확인되지 않았다. 기존 Windows 미검증 항목(T-627 메뉴/DPI 매트릭스 등)과는 별개다. |
| macOS 패키지·서명 | 이번 리뷰에서 재실행하지 않았다(구현 측 증거에 기록). |
| hosted CI | 실행 기록 없음. |
| 실제 Tomcat 운영 로그·대용량 입력 | 합성 3줄 fixture만 사용했다. 1 MiB 초과 줄, 64 MiB 경계, CP949 입력 거부 동작은 코드로만 확인했다. |

## 9 재현 테스트 개요 (처리자 참고)

재현 테스트는 저장소에 넣지 않았다. 처리자가 회귀 테스트로 옮길 수 있도록 핵심만 적는다. 모두 `internal/analysis` 패키지의 기존 `setup/start/wait` 도우미를 사용한다.

| 이름 | 절차 | 관찰 |
|---|---|---|
| JobHistoryNeverEvicted | 없는 경로로 128회 Start→Wait, 129번째 Start | `IMPORT_BUDGET_EXCEEDED: service job capacity reached` |
| MaxLinesOptionNeedsAllowPartial | `{"max_lines":1}` import와 legacy `Analyze(MaxLines:1)` | 새 경로 `failed/IMPORT_PARTIAL_UNSAFE`, legacy `total_events=1` |
| ExactCompareDistinctImportsRefused | 같은 파일 2회 import 후 `Compare` | `population reference required ...` |
| MeasurementPatchVersion | 결과의 extensions를 `measurement:"1.0.1"`로 바꿔 Load | `invalid_result: measurement extension version required` |
| ParserPanicRecovered | ParserFactory가 panic | `failed/IMPORT_FAILED(parsing)`, staging 정리됨(정상) |
| FaultyDetectorBlocksExplicitImporter | 오류 detector 추가 후 `Importer:"tomcat"` | `failed/IMPORT_DETECTOR_INVALID(probing)` |
| LoadStoresNewRevisionEachTime | import 1회 + 같은 파일 Load 3회 | `results/` 파일 4개 |
| CancelWhileNeedsSelection | hint-only detector로 대기 중 Cancel | `cancelled`(정상) |
| CompositionNotCrossChecked | option 자리에 record schema, family `gpu_metrics`, 미등록 ResultType | `NewCatalog` 오류 없음, job은 실행 중 `IMPORT_RESULT_INVALID` |
