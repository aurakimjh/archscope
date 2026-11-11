# T-630 사건 수명주기·capture·안전한 intake 구현 독립 코드 리뷰 — 2026-11-11

- 대상: T-630 구현 커밋 `82bc6a8`(feat(cases): implement T-630 durable evidence lifecycle). 확인 기준 HEAD도 `82bc6a8`이다.
- 계약 기준: `docs/ko/DATA_MODEL.md` "T-630 사건·capture 저장 계약", `docs/ko/PARSER_DESIGN.md` "T-630 안전한 intake·adapter 경계", `docs/ko/LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md` "T-630 로컬 구현 체크포인트", `work_status.md`의 T-630 수용 조건(R-08: durable saved-case 참조, cache 이관·복구, 중복 제거 handle 해제 전 참조 소유권, 사용자 보존·삭제·복원, index 수명주기)
- 증거: `docs/review/evidence/2026-11-11-t630/local.json` (`source_sha256` 35개 항목이 `82bc6a8`의 커밋 내용과 모두 일치. 공유 자산 `capture-context.schema.json`·`evidence-text-v1.json`은 `projects-assets`와 바이트 단위로 같고 `manifest.json`의 digest와도 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 HTTP 변경은 수정하지 않았다.
- 방법: 소스 정독, 저장소 밖 scratch worktree(`82bc6a8`)에서 재실행·재현(미커밋 throwaway probe 테스트), 이 커밋에서 빌드한 실제 CLI로 사건 생성→원본 intake→결과 저장→내보내기→portable 왕복→삭제 계획→purge를 격리 저장소에서 실행.
- 범위: T-630(`internal/cases`, `analysis` 소유권·이관, Wails/CLI 사건 메서드, Workspace 저장·열기). generic 사건 화면·탐색·검색·원본 재검증은 T-631/T-633/T-635이므로 평가하지 않는다.

## 1 결론

**판정: 조건부.** 저장 계약의 핵심(정밀 수치 원문 보존, revision/HEAD 원자 확정, hash 대조, 안전한 archive 검사, encoding 위치 보존, portable 승인 해제, 소유권 있는 lease)은 계약대로 동작하고 CLI 왕복에서도 확인된다. 다만 **문서가 보장한다고 적은 내보내기 덮어쓰기 방어가 대소문자 비구분 볼륨(macOS 기본)에서 뚫리고**(R6-01), **durable saved case를 다시 열려면 용량 한도가 있고 비울 방법이 없는 공유 evidence-cache를 거쳐야 해서** 수명주기가 한쪽으로만 닫혀 있다(R6-02). P1은 없다. P2 2건, P3 5건이다.

- **T-630 `DONE` 판정:** R6-01·R6-02 처리(또는 R6-02를 명시적 범위 밖으로 문서화) 후 재리뷰를 권한다. 그때까지 `REVIEW`를 유지하는 것이 맞다.
- **T-631/T-633/T-635 착수:** 이 리뷰로 막히지 않는다. 다만 R6-02는 T-635의 "case 열기·재사용" 경로가 그대로 의존하므로 T-635 전에 닫는다.
- 실제 WebView IPC/UI, Windows 네이티브 실행, hosted CI는 미검증이다(5절).

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| 정밀 수치·unknown metadata·null/0/부재 원문 보존 | 충족 | `save-result`→`export-result`로 `9007199254740993`, `0.123456789012345678901234567890`, null, 0을 담은 결과를 왕복해 입력과 `cmp` 동일. portable `pack`→`preview`→`restore`→`export-result`도 바이트 동일. |
| revision/HEAD 원자 확정, 낡은 revision 거절 | 충족 | 오래된 `expected_revision`으로 편집하면 `CASE_REVISION_STALE`. 코드상 object→revision→HEAD 순서이며 HEAD 앞에서 중단된 revision은 채택되지 않는다. |
| 단일 writer | 충족 | 다른 프로세스가 `.lock`을 잡은 상태에서 CLI가 `CASE_STORE_BUSY`로 거절. |
| 삭제는 논리 삭제 + 현재 revision에 고정된 token 필요 | 충족 | 삭제 전 `purge`는 `CASE_DELETE_SCOPE_REQUIRED`. 논리 삭제 후 계획 token으로 purge하면 `cases/` 디렉터리가 비워지고 `list`가 `[]`. 외부 원본·export 제외 범위가 계획에 명시된다. |
| 외부 원본은 읽기 전용, 내보내기가 원본을 덮지 않음 | 충족(정확한 경로) | 외부 원본 경로와 대소문자만 다른 대상도 `CASE_EXPORT_DESTINATION_UNSAFE`로 거절(`os.SameFile` 대조). 단 managed 저장소 쪽은 R6-01. |
| capture는 observed/user/estimated/unknown, 미측정은 null, mtime을 anchor로 쓰지 않음 | 충족(코드) | `claim()`이 unknown↔null을 강제하고 clock 도메인이 명시되어야 한다. 파일 시간으로 값을 만드는 경로가 없다. |
| encoding: CP949와 strict EUC-KR 분리, BOM 충돌 거절, 원본 byte 좌표 | 충족(코드·작성 측 테스트) | `runeAt`이 euc-kr에서 lead/trail 범위를 제한하고 cp949는 확장을 허용한다. 행 좌표는 decimal string. |
| archive 안전성(ZIP64·암호화·링크·중첩·경로 충돌·CRC) | 충족(코드·작성 측 테스트) | `scanArchive`가 central directory를 표준 라이브러리 할당 전에 검증하고 모든 항목의 실제 크기/CRC를 읽은 뒤에만 revision을 발행한다. |
| portable: 선택한 검토 완료 항목만, 외부 경로 제거, 복원 시 새 ID·공유 승인 해제 | 충족 | `restore` 결과가 새 case ID, `state: restored`, `sharing_reviewed: false`, `availability: snapshot-only`. |
| 공유 자산 계약 byte parity | 충족 | `projects-assets`(`d240ac6`)와 내장 사본의 SHA-256이 같고 manifest와 일치. |
| lease 소유권 | 충족(단위 테스트) | 두 호출자가 서로 다른 lease를 받고, owner가 남으면 `Release`가 `IMPORT_RESULT_IN_USE`로 거절. 이중 해제 거절. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R6-01 | **P2** | 내보내기 대상 보호가 대소문자만 다른 경로로 우회되어 managed 사건 파일을 덮어쓸 수 있음 | `cases/export_guard.go` |
| R6-02 | **P2** | 공유 evidence-cache가 영구 app-data로 옮겨졌지만 비울 방법이 없고, 가득 차면 saved case도 열 수 없음 | `analysis/store.go` `publish`, `analysis/ownership.go` `LoadSnapshot`, `cmd/archscope-app/evidenceservice.go` |
| R6-03 | P3 | 같은 결과를 다시 저장할 때마다 참조가 중복되고 revision이 늘어남 | `cases/edit.go` `SaveResult`, `frontend/src/state/analysisWorkspace.ts` `saveWorkspaceCase` |
| R6-04 | P3 | manifest/HEAD가 손상된 사건은 삭제 계획·purge가 모두 불가능함 | `cases/store.go` `Purge`, `DeletionPlan` |
| R6-05 | P3 | 이관이 fail-closed여서 legacy cache의 결함 하나가 evidence 서비스를 영구히 막고 복구 경로가 없음 | `analysis/migration.go` |
| R6-06 | P3 | 원본 페이지 조회가 호출마다 전체 원본을 디코딩·색인하고, 행이 100만을 넘으면 전체를 거절함 | `cases/store.go` `ReadSource`, `cases/encoding.go` |
| R6-07 | P3 | 콜론이 든 로컬 파일명이 "unsafe source name"으로 거절됨 | `cases/edit.go` `prepareSource`, `cases/archive.go` `safeName` |

## 4 Finding 상세

### R6-01 내보내기 대상 보호가 대소문자 alias로 우회됨 (P2)

**근거**: `CheckExportDestination`은 대상 경로를 `filepath.Abs` + 부모 `filepath.EvalSymlinks`로 정규화한 뒤 `filepath.Rel(root, target)`이 `..`로 시작하지 않으면 managed 저장소로 보고 거절한다(`export_guard.go:36-47`). 그러나 macOS(APFS 기본)와 Windows 볼륨은 대소문자를 구분하지 않고, macOS의 `EvalSymlinks`는 대소문자를 정규화하지 않는다. 그래서 저장소 경로의 대소문자만 바꾼 대상은 `Rel`이 `..`로 시작해 "저장소 밖"으로 판정된다. 외부 원본 쪽은 `os.SameFile`로 보완했지만 managed 저장소 쪽에는 같은 보완이 없다. DATA_MODEL과 PARSER_DESIGN은 "managed 저장소와 외부 원본 참조/alias를 덮어쓰는 경로를 거부"한다고 적는다.

**재현**(Mac arm64, 대소문자 비구분 볼륨 확인됨, throwaway probe):
1. 사건을 만들고 결과를 저장한 뒤 정확한 경로 `<store>/cases/<id>/HEAD.json`을 검사하면 `CASE_EXPORT_DESTINATION_UNSAFE: export outside managed case storage`(정상 거절).
2. 같은 파일을 `<STORE 대문자>/CASES/<id>/HEAD.json`으로 지정하면 `CheckExportDestination`이 `nil`을 반환한다.
3. 이어서 `analysis.AtomicExport`로 저장된 결과를 그 경로에 내보내면 성공(`err=<nil>`)하고, 이후 `Store.Get`은 `json: unknown field "type"`으로 실패한다. HEAD.json이 AnalysisResult로 교체되어 사건을 읽을 수 없다.

**영향**: 내보내기 경로는 Wails `ExportCaseResult`/`ExportCaseArchive`와 CLI `export-result`/`pack`이 받는 문자열이고 이 검사가 유일한 방어다. 파일 선택 대화상자는 보통 정규 경로를 주므로 실사용 확률은 낮지만, 방어가 보장하는 불변식("authoritative case data를 내보내기가 덮지 않는다")이 문서의 약속대로 성립하지 않는다. 덮어쓰면 해당 사건 전체가 읽기 불가가 되고 복구 경로가 없다. Windows는 실행하지 않았으므로 Windows에서의 동작은 미확인이다(`EvalSymlinks`의 정규화 정도가 달라 결과가 다를 수 있다).

**수정 제안**:
1. 대상(또는 존재하는 가장 가까운 조상)과 저장소 root를 `os.SameFile`로 비교해 조상 체인 중 하나가 root와 같은 파일이면 거절한다. 문자열 접두사 비교는 보조로만 쓴다.
2. 비교 문자열을 만들 때는 OS별 정규 경로(macOS `F_GETPATH`/`realpath`, Windows `GetFinalPathNameByHandle`)를 사용하거나 대소문자 비구분 플랫폼에서 `strings.EqualFold` 접두사 비교를 한다.
3. 대소문자만 다른 managed 경로(디렉터리·파일 각각)와 하드링크 대상을 거절하는 회귀 테스트를 추가한다.

### R6-02 공유 evidence-cache: 정리 수단이 없고 saved case 열기가 그 용량에 의존함 (P2)

**근거**: T-630은 evidence 결과 cache를 OS cache에서 `UserConfigDir()/ArchScope/diagnostics/evidence-cache`(영구 app-data)로 옮겼다(`evidenceservice.go:38-42`). 이 저장소는 결과 1,024개·512 MiB 상한을 두고 초과하면 `IMPORT_STORE_FULL`로 거절하며 "release unused revisions or export a saved file"이라고 안내한다(`store.go:17-19,79-80`). 그런데 이 안내를 실행할 수 있는 제품 경로가 없다. 공개 `Release`는 Wails에서 제거되었고(T-628 R2 N-02 조치), 새로 추가된 `ReleaseEvidenceLease`는 lease만 해제할 뿐 저장된 바이트를 지우지 않는다(`ownership.go:46-56`). `CleanupCaseCache`/`case cleanup-cache`는 case-store의 index·staging만 지운다. 문서는 `IMPORT_STORE_FULL`이나 evidence-cache 정리를 언급하지 않는다. 한편 `ReopenCaseResult`는 saved case의 바이트를 같은 cache에 `LoadSnapshot`→`publish`로 다시 올려야 handle을 얻는다.

**재현**(throwaway probe, `analysis.Service`):
- 서로 다른 작은 결과 1,100개를 `LoadSnapshot`으로 넣으면 1,024개에서 멈추고 1,025번째가 `IMPORT_STORE_FULL: release unused revisions or export a saved file`로 실패한다.
- 그 상태에서 cache에 없는 새 스냅샷(saved case의 결과가 cache에서 사라진 경우에 해당)을 `LoadSnapshot`하면 같은 `IMPORT_STORE_FULL`로 실패한다. 즉 durable하게 저장된 사건의 결과를 다시 열지 못한다.
- 코드 검색으로 `Release(`·`ReleaseResultLease`의 호출처가 테스트 외에 `SaveCaseResult`의 지연 lease 해제와 Workspace의 lease 해제뿐임을 확인했다. 바이트를 지우는 호출자는 없다.

**영향**: 이전에는 같은 상한이 OS cache 위치에 있어 OS·정리 도구가 비워 주었고 사용자가 폴더를 지울 수도 있었다. 이제 위치가 영구 app-data라서 상한은 사실상 영구 소모이고, 도달하면 새 분석 import와 saved case 열기가 모두 막힌다. "saved case는 cache와 독립적으로 재개된다"는 R-08의 핵심 약속이 cache가 가득 차면 깨진다. 512 MiB는 결과가 수 MB인 환경에서 수백 건이면 도달한다.

**수정 제안**:
1. `ReopenCaseResult`가 cache 용량 때문에 실패하지 않게 한다. 예: lease가 없는 항목을 LRU로 제거한 뒤 publish하거나, saved snapshot 열기는 별도의 비정량 슬롯을 쓴다.
2. 사용자가 호출할 수 있는 cache 정리(소유 lease가 없는 handle만 제거, 제거 수·바이트 보고)를 Wails·CLI에 추가하고, `IMPORT_STORE_FULL` 안내가 그 명령을 가리키게 한다.
3. DATA_MODEL의 "cache 정리" 문단에 evidence-cache의 상한·정리 방법·legacy 사본 유지 범위를 명시한다.
4. 한도 도달 상태에서 saved case 재개와 새 import를 검증하는 회귀 테스트를 추가한다.

### R6-03 같은 결과 재저장 시 참조 중복 (P3)

**근거**: `SaveResult`는 호출마다 새 `ResultRef`를 추가하고 같은 SHA-256인지, 같은 source/selection인지 확인하지 않는다(`edit.go:420-428`). Workspace의 "선택한 결과 저장" 버튼은 이미 저장된 사건이 있으면 그 사건에 다시 `save`를 호출한다(`analysisWorkspace.ts`의 `saveWorkspaceCase`).

**재현**: 같은 결과 본문을 `SaveResult`로 3번 저장하면 `results`가 3개가 되고 revision이 4다(object는 digest로 중복 제거되지만 참조와 revision이 늘어난다). 128개에 도달하면 `CASE_BUDGET_EXCEEDED`가 된다.

**영향**: 재클릭만으로 manifest와 revision 이력이 불필요하게 커지고 사건 열기 메뉴에 동일 항목이 반복된다. 이력은 purge 전까지 남으므로 되돌릴 수도 없다.

**수정 제안**: 같은 `sha256`·`source_ids`·`options_hash`·`capture_id`·`selection`의 기존 참조가 있으면 그 참조를 반환(revision 증가 없음)하거나, UI가 이미 저장된 항목을 감지해 다시 저장하지 않게 한다.

### R6-04 손상된 사건은 삭제 계획·purge가 모두 불가능 (P3)

**근거**: `Purge`와 `DeletionPlan`은 먼저 `get`/`expect`로 manifest를 읽고 hash를 대조한다(`store.go:607-630`). 문서는 "object 손상 시에도 논리 삭제·purge는 가능"이라고 약속하는데, manifest/HEAD 손상은 약속 범위 밖이고 대체 경로도 없다.

**재현**: 논리 삭제한 사건의 `HEAD.json`이 가리키는 digest를 바꾸면 `Purge`가 `CASE_HASH_MISMATCH: manifest digest mismatch`, `DeletionPlan`도 같은 오류로 실패한다. `List`는 그 사건을 `Problem` 항목으로 계속 보여주고 `MaxCases`(128) 계산에도 포함한다.

**영향**: 손상 사건 안의 managed 원본·결과를 앱에서 삭제할 수 없고 슬롯도 반환되지 않는다. 사용자의 삭제 요구(R-08)와 개인정보 보존 측면에서 문서에 "수동 파일 삭제" 같은 대안도 없다.

**수정 제안**: 손상 사건에 한해 case ID와 제거 대상 디렉터리를 보여 주고 명시 확인 token으로 `cases/<id>` 전체를 제거하는 경로를 제공하거나, 최소한 수동 삭제 절차를 DATA_MODEL에 적는다.

### R6-05 이관 fail-closed에 복구 경로가 없음 (P3)

**근거**: `OpenDurable`은 legacy 디렉터리가 있으면 `results`·`staging`이 모두 실제 디렉터리여야 하고, 어떤 snapshot이든 읽기·검증·publish에 실패하면 전체를 실패시킨다(`migration.go:76-121`). 작성 측 테스트도 "유효하지 않은 snapshot이 있으면 이관이 실패하고 marker를 쓰지 않는다"를 고정한다. 의도된 보수적 동작이지만 `evidence()`가 매번 이 이관을 먼저 시도하므로 실패는 영구적이다.

**재현**(throwaway probe): 빈 legacy 디렉터리는 `lstat .../legacy/results: no such file or directory`, `results`만 있는 디렉터리는 `lstat .../legacy/staging: no such file or directory`로 `OpenDurable`이 실패한다. 손상된 snapshot 하나도 같은 결과다(작성 측 테스트).

**영향**: legacy 폴더가 일부 삭제되었거나 결과 파일 하나가 깨졌을 때 앱의 evidence import·열기 전체가 시작되지 않는다. 오류 메시지에 해결 방법이 없고 문서는 "recover explicitly"라고만 적는다. 사용자가 할 수 있는 일은 수동으로 legacy 폴더를 지우는 것인데 어디에도 적혀 있지 않다.

**수정 제안**: 읽을 수 없는 항목만 건너뛰고 보고서(건너뛴 handle, 사유)와 함께 완료 marker를 쓰는 모드, 또는 사용자가 명시적으로 이관을 건너뛰는 경로를 제공한다. 최소한 오류가 legacy 경로와 조치(폴더 이름 변경 후 재시작)를 알려야 한다.

### R6-06 원본 페이지 조회 비용과 100만 행 거절 (P3)

**근거**: `GetCaseSourcePage`는 호출마다 `ReadSource`로 원본 전체를 읽고 hash를 대조한 뒤 `DecodeText`로 전체 행 색인을 만들고 요청한 200행만 잘라낸다(`caseservice.go:221-225`, `store.go:634-677`, `encoding.go:121-175`). 행 색인은 1,000,000행이 한도다(`encoding.go:128`).

**재현**(throwaway probe, Mac arm64): 60 MiB·78바이트 행 786,432개 파일은 한 번의 페이지 요청이 약 340 ms이고 약 770 MiB를 할당한다(연속 3회 1.0 s). 이 파일을 200행씩 끝까지 넘기려면 약 3,900번의 전체 디코딩이다. 같은 60 MiB를 11바이트 행으로 만들면 `CASE_BUDGET_EXCEEDED: source line index limit`로 **파일 전체**가 열리지 않는다.

**영향**: 현재 UI는 이 API를 쓰지 않아 사용자 영향은 없지만, T-635의 원본 링크·drilldown이 이 API 위에 올라가면 페이지마다 수백 ms와 GB 단위 할당이 반복되고 행이 짧은 큰 로그는 아예 열 수 없다.

**수정 제안**: 디코딩 색인을 `index_revision` 단위로 캐시하거나 원본 byte offset 기반 창(window)으로 읽는다. 행 한도 초과는 전체 거절 대신 한도까지 색인하고 "더 있음"을 표시한다.

### R6-07 콜론이 있는 로컬 파일명 거절 (P3)

**근거**: `prepareSource`는 `edit.Name`이 비어 있으면 `filepath.Base(path)`를 이름으로 쓰고 `safeName`에 통과시킨다. `safeName`은 Windows 호환 규칙(`:`, `\`, 예약어 등)을 archive 항목 이름과 **로컬 외부 파일의 표시 이름**에 똑같이 적용한다(`archive.go:36`, `edit.go:326-331`).

**재현**(실제 CLI): `jstack_10:30:01.txt`처럼 콜론이 든 파일(macOS·Linux에서 흔한 덤프 이름)을 `operation: "source"`로 넣으면 `archscope-engine: CASE_INVALID: unsafe source name`으로 거절된다. `name`을 직접 주면 통과하지만 Workspace UI에는 그 입력이 없다.

**영향**: 파일을 외부 참조로만 가리키는 경우에도 이름 때문에 intake가 막히고 메시지가 원인(콜론)과 조치(이름 지정)를 알려주지 않는다. portable·archive에서 안전한 이름이 필요한 것과 로컬 표시 이름은 별개다.

**수정 제안**: 로컬 외부 원본은 표시 이름을 제어 문자 정도만 제한하고, portable/archive 경로에서만 안전 이름 규칙을 적용한다. 거절 시 오류가 규칙과 해결책(`name` 지정)을 알려야 한다.

## 5 검증 범위

### 실행한 것 (Mac arm64)

| 항목 | 결과 |
|---|---|
| `go vet ./...` | 통과 |
| 전체 `go test -race -count=1 ./...` | 96개 패키지 통과, 실패 없음 |
| Windows amd64 교차 `go vet`(`internal/cases`, `internal/analysis`) | 통과 |
| 프런트엔드 `tsc --noEmit`, `npm run test:state` | 통과 |
| 실제 CLI(이 커밋에서 빌드, 격리 저장소) | create→source intake→stale 거절→삭제 전 purge 거절→delete-plan→단일 writer 충돌→purge, save-result→export-result 바이트 동일, pack→preview→restore→export 바이트 동일, 콜론 파일명 거절(R6-07) |
| 재현 probe(scratch worktree, 미커밋) | R6-01~R6-04 재현, R6-05·R6-06 측정 |
| 증거 `source_sha256` 35개, 공유 자산 digest | 일치 |

### 코드·테스트 정독으로만 확인한 것

encoding 변환 세부(UTF-16 surrogate, CP949/EUC-KR 경계), ZIP/TAR 검증 분기, `commit`의 취소·롤백 경로, `capacity()` 합산 방식, Wails 메서드 시그니처는 정독과 작성 측 회귀 테스트(전체 통과)로 확인했고 별도 재현은 하지 않았다. `commit`이 저장소 합계와 참조 object 크기를 함께 한도 검사에 넣어 이미 저장된 object가 이중 계산되는 것으로 읽히나 4 GiB 경계는 재현하지 않았다. 사건 128개·revision 1,024개·저장소 4 GiB 같은 상한의 정확한 경계도 직접 재현하지 않았다. 7×60 MiB managed 원본을 가진 사건의 `Get`(약 0.18 s)과 세부 정보 편집(약 0.20 s)은 이 환경(SSD)에서 빨랐으므로 성능 지적에 넣지 않았다. 결과 저장이 `SourceHashes`를 manifest의 원본 hash에서 복사하며 결과 분석 입력과 대조하지 않는 점은 문서의 "declared" 계약과 T-635 R3-02 조건에 해당하므로 finding으로 올리지 않았다. CLI는 데스크톱 앱이 저장소를 열고 있으면 `CASE_STORE_BUSY`로 실패하는데, 설계된 단일 writer 동작이지만 오류가 앱을 닫으라고 안내하지 않는다.

### 실행하지 않은 것 (기존 미검증 항목과 구분)

- 실제 WebView IPC/UI: 새 사건 메뉴(저장·열기)를 눌러 본 적이 없다. 보존·삭제·복원·purge·portable 조작은 UI가 없고 CLI/Wails 메서드로만 있다.
- Windows 네이티브 실행: 교차 빌드·vet만 있다. `LockFileEx`, `os.Root`, 예약 이름, 대소문자 경로 동작(R6-01)을 Windows에서 실행하지 않았다.
- hosted CI, 실제 producer 입력(Q01~Q04), 서명 pack 검증(T-632).
- 대용량 사건·저장소 한도 경계와 중단 복구(프로세스 강제 종료)의 실제 실행.

## 6 처리 권장 순서

1. **R6-01**: 정규 경로·`os.SameFile` 기반으로 export 보호를 고치고 대소문자·하드링크 회귀를 추가한다.
2. **R6-02**: saved case 열기가 cache 용량에 막히지 않게 하고 사용자 호출 가능한 cache 정리와 문서를 추가한다. T-635 전에 닫는다.
3. R6-03·R6-04: 저장 멱등성과 손상 사건 삭제 경로는 사건 UI(T-631/T-635)를 붙이기 전에 처리한다.
4. R6-05·R6-06·R6-07: 해당 영역을 손댈 때 처리하되 R6-06은 T-635 원본 링크 설계에 반영한다.
