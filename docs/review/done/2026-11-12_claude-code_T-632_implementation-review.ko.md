# T-632 서명 데이터 팩·오프라인 배포 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-632 구현 커밋 `34dbd8f`(feat(packs): deliver signed declarative packs and offline distribution). 이 커밋은 T-630 후속 `1fdb84b` 위에 놓인다.
- 계약 기준: `work_status.md`의 T-632 행과 수용 조건, 배포 문서 `apps/engine-native/internal/distribution/assets/docs/ko/OFFLINE_PACKS.md`(`docs/ko/OFFLINE_PACKS.md`와 동일 내용)
- 증거: `docs/review/evidence/2026-11-11-t632/local.json`(`source_sha256` 170개 항목이 `34dbd8f` 커밋 내용과 모두 일치), `offline-syscalls.json`, `offline-operation-*.strace`, `offline-positive-control.strace`
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(처리자의 R8 수정 진행분, 사용자 HTTP 변경)은 수정하지 않았다.
- 방법: 소스 정독(`internal/packs`, `catalog/pack_session.go`, `ingestion/packs.go`, `distribution/bundle.go`, `netpolicy`, `cmd/archscope-app/{packservice,offline,evidenceservice}.go`, `PackSettings.tsx`), 저장소 밖 scratch worktree(`34dbd8f`)에서 미커밋 throwaway probe 테스트로 재현, 이 커밋에서 빌드한 실제 CLI로 `packs`/`offline` 실행(격리된 `ARCHSCOPE_STATE_ROOT`).
- 범위: 서명 팩 검증·신뢰키·활성화/롤백, 선언형 매핑 해석기, 팩 고정(pin), 오프라인 묶음·검증·정책, 고정 WebView2 사전 점검 코드. 순수 Go SQLite 스파이크는 문서·증거 JSON만 확인했고 재측정하지 않았다. R8(T-631) 후속 수정은 평가하지 않는다.

## 1 결론

**판정: 조건부.** 서명이 매니페스트의 원본 바이트를 덮는 점, ZIP 안의 키로는 신뢰를 얻을 수 없는 점, 닫힌 매핑 어휘와 정확한 소수 연산, 상태 revision과 원자적 교체, 변조·미선언 파일을 거부하는 오프라인 묶음 검증은 계약대로 동작한다. 다만 **정상 절차로 번들 부트스트랩 키를 폐기하면 앱과 `packs` CLI가 다음 실행부터 영구적으로 열리지 않으며 복구 명령도 같은 경로를 거쳐 실행되지 않는다**(R9-01). 이는 문서가 약속한 "폐기 후 새 가져오기가 거부될 수 있다"를 넘어서므로 닫기 전에는 T-632 `DONE`을 권하지 않는다. P1은 없다. P2 1건, P3 3건이다.

- **T-632 `DONE` 판정:** R9-01 처리 후 가능하다. 작성 측 기록처럼 실제 Windows 데스크톱 관측은 별도 수용 항목이다.
- **T-633 착수:** R9-01~R9-04는 T-633을 막지 않는다. 다만 T-636/T-639가 팩 활성 상태를 전제하므로 R9-02는 그 전에 닫는 편이 안전하다.
- 실제 WebView IPC/UI(작성 측 macOS 관찰 외), Windows 네이티브 실행, 데스크톱 외부 연결 관측, hosted CI는 미검증이다(5절).

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| 서명은 매니페스트 원본 바이트를 덮고, 신뢰키는 ZIP 밖 | 충족 | `archive.go` `Validate`: `ed25519.Verify(public, raw, sig)`가 `raw`(매니페스트 원본)에 대해 실행되고 `keys`는 호출자 저장소에서만 온다. 서명자 없음·폐기·키 ID 불일치는 `PACK_SIGNER_UNTRUSTED`. |
| ZIP 안전성 | 충족(코드·기존 테스트) | EOCD 선검사로 주석·분할·후행 데이터·ZIP64 거부, 이름 규칙(대소문자 중복, 장치 이름, 비 ASCII, `..`), 실행 비트·암호화·extra·링크 거부, 선언 크기와 실제 크기·CRC 일치, 압축 해제 16 MiB 상한. |
| 닫힌 매핑 어휘와 정확한 소수 | 충족 | probe: `1.5`×`0.001`→`"0.0015"`, 문자열 `9007199254740993`→`"9007199254740.993"`(정밀도 보존), `null`은 0이 아니라 거부, counter의 음수 거부, `-0`→`"0"`, `"1/3"` 문자열 거부, `1e-400`×`0.001`은 `PACK_SCALE_INEXACT`로 정직하게 거부. 코드·정규식·동적 표현식 경로 없음. |
| 신뢰 갱신은 관리 키 서명·순서 연속 | 충족 | `trust.go`: 관리 키 서명 검증, `sequence`가 정확히 +1, 같은 ID에 다른 키 재등록 거부(`PACK_TRUST_CONFLICT`), 폐기 후 활성 관리 키 0개 거부. 실제 CLI로 등록→갱신이 `registered:true`, `updated:true`로 적용됨. |
| 설치와 활성화 분리, 상태 revision | 충족 | `Activate`/`Deactivate`/`RegisterKey`가 `expected`≠현재 revision이면 `PACK_STATE_STALE`. 상태 파일은 sync 후 atomic rename, 시작 시 staging 제거. |
| 미서명은 명시 범위만, 로컬 자동 우선 없음 | 충족 | `AllowUnsigned`+`user`/`case:case-<hex32>`가 아니면 `PACK_SIGNATURE_REQUIRED`. `Extend`가 canonical ID 덮어쓰기와 별칭 충돌을 거부하고 명시 `choices`만 수용. |
| 작업 pin | 충족(코드·기존 테스트) | `Service.Start`가 시작 시점의 `CatalogSnapshot`(카탈로그·팩 pin·revision)을 job에 고정. |
| 오프라인 묶음 | 충족 | 실제 빌드(83개 파일, 약 40 MB, 1.5 s). `offline verify` 통과, 자원 파일 1바이트 변조는 `OFFLINE_HASH_MISMATCH`, 미선언 파일은 `OFFLINE_UNDECLARED_FILE`. `--offline-root` 실행은 0.06 s. |
| 필수 fixture는 등록된 모든 importer를 포함 | 충족 | `offline fixtures`가 `data.synthetic-metrics`, `profile.set-json`, `server.tomcat` 3개를 검증(등록 importer 3개와 일치). |
| 오프라인 정책 | 충족(범위 한정) | 앱 소유 네트워크·exec 진입점 7곳(`proxy.listen`, `automation.listen/invoke`, `http.request`, `capture.start`, `converter.jfr`, `ai.ollama`)에 `RequireOnline`이 있고 그 밖의 `net.Dial`/`http.Client`/`exec` 사용처는 없다. 거부 횟수는 OS/WebView 관측과 구분된다고 문서·보고 필드에 명시. |
| 증거 무결성 | 충족 | `local.json`의 `source_sha256` 170개가 `git show 34dbd8f:<path>`와 모두 일치. syscall 요약은 4개 명령 INET 0건, 실패하는 연결 양성 대조군은 `connect(...ENETUNREACH)` 2행으로 관측기가 작동함을 보인다. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R9-01 | **P2** | 번들 부트스트랩 키를 폐기하면 앱과 `packs` CLI가 열리지 않고 복구 명령도 실행되지 않음 | `packs/builtin.go:35`, `catalog/pack_session.go:27`, `cmd/archscope-app/evidenceservice.go:47` |
| R9-02 | P3 | 상위(application) 범위 활성화가 기존 user/case 범위의 충돌을 검사하지 않아 그 범위가 나중에 깨짐 | `packs/store.go:448,477` |
| R9-03 | P3 | `Snapshot`이 호출마다 모든 활성 팩을 세 번씩 풀고 재검증함 | `packs/store.go:334-395`, `catalog/pack_session.go:34` |
| R9-04 | P3 | 매핑된 metric 이름의 BiDi·제어 문자가 그대로 통과함(R8-03과 같은 부류) | `packs/mapping.go` `Evaluate` |

## 4 Finding 상세

### R9-01 부트스트랩 키 폐기 시 영구 시작 실패 (P2)

**근거**: `OpenDefault`는 실행할 때마다 내장 `bootstrap.zip`을 `m.Install`로 다시 검증한다(`builtin.go:35`). 설치 검증은 현재 `state.Keys`를 사용하므로 번들 서명 키가 폐기되면 `PACK_SIGNER_UNTRUSTED`로 실패하고 `OpenDefault`가 `closeFail`로 매니저를 닫는다. `OpenPackDirectory`(`pack_session.go:27`)는 그 오류를 그대로 반환하고, 앱의 `evidence()`(`evidenceservice.go:47`)와 CLI의 모든 `packs`/`evidence` 경로가 이 함수를 통해 열린다. 번들 키는 `manage_trust: true`이며 서명된 갱신으로 폐기할 수 있다(`trust.go:99`, 관리 키가 하나 남아 있으면 통과). 설정 화면의 "서명된 교체 / 폐기 적용"과 `packs update-trust`가 모두 이 경로다.

**재현**(이 커밋에서 빌드한 실제 CLI, 새 `ARCHSCOPE_STATE_ROOT`):
1. `packs register-key`로 소유자 관리 키를 등록한다(`registered:true`).
2. 그 키로 서명한 갱신(`revoke: ["bundled-bootstrap-20261111"]`)을 `packs update-trust`로 적용한다(`updated:true`).
3. 이후 `packs list`는 `archscope-engine: PACK_SIGNER_UNTRUSTED: signer is absent/revoked; register trust separately`로 종료 코드 1이 된다. `deactivate`, `register-key`, `update-trust`, `export`도 같은 경로로 열리므로 제품 명령으로는 복구할 방법이 없다(저장소를 수동으로 고치거나 지우는 방법만 남음). 패키지 수준에서는 같은 상태에서 `Open`(번들 설치 없음)은 성공하므로 상태 자체는 정상이다.

같은 클래스의 완화된 경우: 활성 팩의 서명 키가 폐기되면 `Snapshot`이 실패하고 `Service.Start`가 그 오류를 그대로 돌려주어 **내장 importer를 포함한 모든 새 가져오기**가 거부된다(문서의 "새 가져오기가 거부될 수 있다"는 이를 팩 한정으로 읽게 한다). 이때는 설정 화면이 열려 팩 비활성화로 복구할 수 있다.

**영향**: 키 교체는 이 기능이 약속한 핵심 운영 절차다. 번들 키가 유일한 관리 키로 시작하므로 소유자가 자기 키로 교체하며 번들 키를 폐기하는 것이 가장 자연스러운 순서이고, 그 순간 앱이 다음 실행부터 시작되지 않는다. 오프라인 설치에서는 사용자가 원인을 검색할 수도 없다.

**수정 제안**:
1. `OpenDefault`에서 번들 설치가 `PACK_SIGNER_UNTRUSTED`(폐기·부재)로 실패하면 열기를 중단하지 말고 기본 팩만 건너뛴다. 기존 활성 상태는 건드리지 않는다.
2. 활성 팩의 서명 키가 폐기될 때 `Snapshot`이 해당 팩만 휴면으로 표시하고 나머지와 내장 importer는 계속 제공하는 방안을 검토한다. 최소한 오류 메시지가 해당 팩과 비활성화 방법을 가리키게 한다.
3. 폐기가 활성 팩 서명 키를 포함할 때 적용 전에 영향 팩 목록을 거부 또는 경고로 보여준다.
4. 회귀 테스트: 번들 키 폐기 후 `OpenDefault`가 열리고 `Deactivate`/`RegisterKey`가 동작한다. 문서에 복구 절차를 추가한다.

### R9-02 범위 간 활성화 충돌이 사후에 발생 (P3)

**근거**: `Activate`는 `m.snapshot(ctx, next, scope)`로 활성화하는 범위 하나에 대해서만 충돌을 검사한다(`store.go:477`). `snapshot`은 `application|*`와 `scope|*`만 읽으므로 application 범위를 활성화해도 이미 활성화된 user·case 범위와의 canonical ID 충돌은 확인하지 않는다.

**재현**(패키지 수준 probe, 카탈로그의 canonical ID 충돌 규칙을 `check`로 흉내): 같은 매핑 ID를 가진 팩을 user 범위(미서명)와 application 범위로 둔다. user를 먼저 활성화한 뒤 application을 활성화하면 `nil`(성공)이 반환되고, 이후 `Snapshot("user")`는 `PACK_ALIAS_CONFLICT: duplicate canonical data.synthetic-metrics`로 실패한다. `Snapshot("")`(application)은 정상이다. 반대 순서에서는 user 활성화가 거부되므로 순서에 따라 결과가 갈린다.

**영향**: 사용자 범위 가져오기가 활성화 시점이 아닌 나중에 실패하고, 원인이 방금 한 application 활성화라는 안내가 없다. 복구는 가능하다(application 비활성화).

**수정 제안**: application 범위 활성화는 활성 상태가 있는 모든 범위의 스냅샷을 검증한 뒤 커밋하거나, 충돌을 보고하고 거부한다. 순서 두 가지를 모두 다루는 회귀 테스트를 추가한다.

### R9-03 `Snapshot`의 반복 로드 비용 (P3)

**근거**: `snapshot`은 활성 팩마다 `m.load`를 세 번(별칭·크기 집계, 의존성 검사, 순환 검사) 호출하고, 각 `load`는 아카이브 읽기, 해시, 압축 해제, 서명 검증, 전체 conformance를 수행한다(`store.go:356,376,391`). `Provider`는 가져오기 시작과 `ListEvidenceImporters`마다 `Snapshot`을 호출하며 revision별 컴파일 카탈로그 캐시는 그 뒤에서만 쓰인다(`pack_session.go:34`). 호출 전체가 매니저 뮤텍스 아래 직렬로 실행된다.

**재현**(probe): 해제 후 약 6.3 MiB인 팩 두 개(합계 12 MiB, 아카이브는 각 약 9 KB)를 활성화하고 `Snapshot` 5회 평균 **93 ms**. 기본 번들 팩 하나(작음)는 1 ms 미만이다.

**영향**: 활성 상한 32 MiB에서는 호출당 약 0.25 s를 넘기고 UI 새로고침·가져오기 시작마다 반복된다. 지금 사용자에게 보이는 영향은 작다.

**수정 제안**: 검증된 `Revision`을 `(archive_sha256, trust 시퀀스)` 키로 캐시하고 한 `snapshot` 안에서는 팩당 한 번만 로드한다.

### R9-04 metric 이름의 표시 문자 (P3)

**근거**: `Mapping.Evaluate`는 metric 이름에 길이(256)만 검사하고 문자 구성을 검사하지 않는다. probe에서 `"a‮\u0007"`가 `Record.Metric`에 그대로 담겼다. 가져온 이름은 표 셀로 이어지므로 R8-03(BiDi·제어 문자 표시)과 같은 표시 문제가 된다.

**수정 제안**: R8-03의 표시 정규화를 공유 함수로 만들어 매핑 출력과 `displayCell`이 함께 쓰거나, 제어·BiDi 문자를 매핑 입력에서 거부한다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `34dbd8f`, Mac arm64):
- `go vet ./...` 통과
- `go test -race -count=1 ./...` 99개 패키지 모두 `ok`
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리에서 심볼릭 링크)
- 증거 JSON `source_sha256` 170개 대조, 실제 CLI `packs`·`offline build/verify/fixtures`·변조 시나리오, `--offline-root` 실행과 보고 파일
- throwaway probe(미커밋, 삭제): 부트스트랩 키 폐기 재현, 범위 간 충돌, 큰 팩 `Snapshot` 시간, 매핑 정확도·경계값

**미검증**:
- 실제 WebView IPC/UI 클릭 흐름(작성 측 macOS 관찰 기록만 있음), 설정 화면의 키 폐기 적용 UI는 코드 정독으로 확인
- Windows 네이티브 실행: 고정 WebView2 런타임 시작, `checkRuntimeVersion`(Windows 전용 코드) 실행, 브라우저 프로세스 외부 연결. 교차 vet만 확인.
- 데스크톱(WebView) 외부 연결 관측. syscall 증거는 Linux CLI 프로세스 트리 4개 명령에 한정되며 `capture`/HTTP/Ollama 같은 거부 경로는 관측하지 않았다.
- hosted CI, 실제 producer/버전 qualification, 전원 장애 내구성
- Linux 컨테이너 syscall 관측은 재실행하지 않았다(증거 파일과 스크립트만 정독).
- 묶음 `Verify`는 실행 중인 파일이 목록의 엔진과 같다는 것을 확인하지 않는다(해시 목록은 우발적 변경 검출이라는 문서 설명과 일치). 진위는 전달 경로에 달려 있다.
- SQLite 스파이크 수치는 재측정하지 않았다. 결정(JSON/인덱스 유지, 의존성 추가 없음)은 증거 JSON을 확인했다.

## 6 권고 처리 순서

1. R9-01 — 번들 키 폐기 시 열림 유지, 활성 팩 키 폐기 영향 격리, 회귀 테스트와 복구 문서. T-632 닫기 전.
2. R9-02 — 범위 간 활성화 검증. T-636/T-639 전.
3. R9-03 — 검증된 팩 캐시와 단일 로드.
4. R9-04 — R8-03 표시 정규화와 함께 처리.
