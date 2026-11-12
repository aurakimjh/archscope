# T-633 입력 안내·수동 수집·로컬 JFR 변환 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-633 구현 커밋 `3d443d3`(feat(input): add evidence discovery and bounded local JFR conversion). 직전 `0ce79e7`(T-632 리뷰 후속 수정) 위에 놓인다.
- 계약 기준: `work_status.md`의 T-633 행과 수용 조건, `docs/ko/INPUT_ASSISTANCE.md`, `docs/ko/PARSER_DESIGN.md` T-633 절
- 증거: `docs/review/evidence/2026-11-12-t633/local.json`(`source_sha256` 56개와 `evidence_sha256` 32개가 `3d443d3` 커밋 내용과 모두 일치), `jdk-summary.json`, `offline-summary.json`, `offline-*.strace`
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(사용자 HTTP 변경 등)은 수정하지 않았다.
- 방법: 소스 정독(`inputassist`, `localconvert`, `localfiles`, `inputservice.go`, `cmd_inputassist.go`, `parsers/jfr/recording.go`, `InputAssistance.tsx`), 저장소 밖 scratch worktree(`3d443d3`)에서 빌드한 실제 CLI로 `evidence discover`/`collection-kit`/`jfr convert`/`jfr analyze-native-memory` 실행(격리 `ARCHSCOPE_STATE_ROOT`), 설치된 Temurin 11/17/21과 합성·실제 JFR 녹화, 가짜 도구 스크립트로 경계 시나리오 재현.
- 범위: 파일 발견·후보·누락 맥락 안내, 명시적 sidecar 결합, 첫 행 매핑 미리보기, 수동 수집 키트, 고정 인자 로컬 JFR 변환(취소·한도·provenance), 레거시 바이너리 JFR 분석 경로 변경, 오프라인 거부. 네이티브 바이너리 JFR 디코더는 범위 밖이다.

## 1 결론

**판정: 조건부.** 발견 기능은 추측 없이 동작한다(파일 바이트 해시 식별, sidecar는 해시·크기가 맞을 때만 결합, 누락 맥락은 unknown으로 유지, mtime 추론 없음). 변환기도 고정 인자, 프로세스 트리 취소, 임시 파일 정리, 출력 새 파일 게시, 오프라인 거부 같은 구조를 갖추었다. 그러나 **변환 출력 상한 64 MiB가 실제 JFR 녹화에서 매우 일찍 걸려, 약 10 MB 바이너리 JFR이 변환되지 않는다**(R10-01). JDK 21 `jfr print --json`이 10.5 MB 녹화에 310 MB JSON을 만들었고, 같은 JSON을 미리 변환해 `jfr analyze-json`으로 분석하는 것은 성공한다. 레거시 바이너리 JFR 분석 경로(`ARCHSCOPE_JFR_CLI`)도 같은 상한을 상속해 이전에는 처리되던 일반적인 프로파일 녹화가 이제 거부된다. P1은 없다. P2 1건, P3 4건이다.

- **T-633 `DONE` 판정:** R10-01 처리 후 가능하다. 입력 상한과 출력 상한의 관계를 문서와 오류 메시지로 정직하게 드러내는 것도 R10-01에 포함된다.
- **후속 작업(T-634 등) 착수:** R10-01~R10-05는 막지 않는다.
- 실제 WebView UI, Windows Job Object 네이티브 실행, 데스크톱 외부 연결 관측, hosted CI는 미검증이다(5절).

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| 발견은 소유 스냅샷·바이트 해시·64 KiB probe, 인접 파일 탐색·도구 실행 없음 | 충족 | `discover.go`: `ingestion.Acquire`로 소유 스냅샷을 만들고 `lease.Verify`로 끝에서 재확인한다. 실제 CLI에서 50 MiB JSONL의 크기·`probe_bytes=65536`을 확인. 64 MiB 초과(70 MiB, 300 MiB)는 `IMPORT_BUDGET_EXCEEDED`(가져오기와 같은 상한). |
| sidecar는 명시 선택, 해시·크기가 맞아야 결합 | 충족 | 초안 sidecar는 `matched_declared`. 해시를 바꾸면 `CAPTURE_FILE_MISMATCH`, 크기 표기가 다르면(`0191`) `CAPTURE_INVALID`. producer를 채운 sidecar는 그 필드만 누락 목록에서 빠진다(`producer`, `producer_version` 제거). 어떤 경우에도 시계·호스트·프로세스 수명은 선언이 없으면 누락으로 남는다. |
| 새 초안은 관측한 파일 정체성 + unknown | 충족 | 초안은 `producer/producer_version/identity_basis: unknown`, `clock.domain: unknown`, mtime 경고 포함. 이름에 U+202E·BEL이 들어간 파일은 초안 파일명이 `selected-source.bin`으로 바뀌고 경고가 붙는다. |
| 첫 행 매핑 미리보기는 제한된 샘플, 지원 부여 없음 | 충족 | 번들 팩의 fixture 첫 행: `status: sample_only`, source `12.50` → normalized `0.0125` seconds, `escaped:false`. importer가 아니거나 첫 행이 probe보다 크면 `unavailable`. |
| 수동 키트는 비실행 자산 | 충족 | `recipes.json`은 문자열 명령 템플릿(`jcmd <PID> …`, `jfr print --json <INPUT>.jfr`)과 맥락 목록뿐이며 실행 경로가 없다. `Kit()`은 고정 시각·Store로 ZIP을 만든다. 공유 자산 `projects-assets/collection/manual-evidence-v1/recipes.json`과 바이트가 같다(작업 사본 기준). |
| 변환: 고정 인자, 취소, 정리 | 충족 | 실제 JDK 11/17/21 `jfr`로 변환 성공. 가짜 도구가 자식 프로세스를 띄운 상태에서 SIGINT를 보내면 `CONVERTER_CANCELLED`, 남은 `sleep` 프로세스와 `archscope-convert-*` 임시 디렉터리가 모두 없다. 상대 경로 도구는 `CONVERTER_EXPLICIT_TOOL_REQUIRED`, JFR 매직이 아닌 입력은 `CONVERTER_INPUT_INVALID`, `--version` 출력이 버전 형식이 아니면 `CONVERTER_VERSION_INVALID`. |
| 출력은 새 파일만, 기존 파일 교체 없음 | 충족 | 이미 있는 출력은 `file exists`로 실패하고 원본은 유지된다(`localfiles.Publish`의 `os.Link`). 관리 상태 디렉터리 안의 출력은 `OUTPUT_MANAGED_PATH`로 거부(실제 CLI). |
| 오프라인에서는 변환을 실행 파일 접근 전에 거부 | 충족 | `JFR()`의 첫 줄이 `netpolicy.RequireOnline("converter.jfr")`. 증거의 `offline-summary.json`은 7개 명령 INET 0건과 `converter.jfr: 1` 거부, 양성 대조군 2건을 보인다. |
| 레거시 경로는 명시 도구만 | 충족 | 환경변수 없이 `jfr analyze-native-memory --in x.jfr`은 `ARCHSCOPE_JFR_CLI` 안내 오류, `JAVA_HOME`만 있어도 같다. `ARCHSCOPE_JFR_CLI`를 주면 `native_memory` 결과와 `metadata.conversion` provenance가 나온다. |
| UI는 가져온 문자열을 표시 정규화 | 충족(코드) | `InputAssistance.tsx`가 소스 이름·importer·사유·한도를 모두 `safeDisplayText`로 렌더링한다. |
| 증거 무결성 | 충족 | `source_sha256` 56개, `evidence_sha256` 32개 모두 커밋 내용과 일치. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R10-01 | **P2** | 변환 출력 상한 64 MiB가 약 2~10 MB JFR에서 걸려 일반 녹화가 변환·분석되지 않고 오류 안내가 원인을 가림 | `localconvert/jfr.go:23,224`, `parsers/jfr/recording.go` |
| R10-02 | P3 | provenance가 임의 실행 파일에도 `tool: "jdk.jfr"`을 기록하고 도구 이름·종류를 확인하지 않음 | `localconvert/jfr.go:164,266` |
| R10-03 | P3 | 도구 stderr가 16 KiB를 넘으면 정상 종료한 변환이 실패함 | `localconvert/jfr.go:116,126` |
| R10-04 | P3 | 출력이 이미 있어도 전체 변환을 끝낸 뒤에야 실패하고 내부 임시 경로가 노출됨, 메모리는 출력의 약 4.4배 | `localconvert/save.go`, `localconvert/jfr.go:239` |
| R10-05 | P3 | 기존 설계 문서가 JFR CLI 탐색 순서를 옛 동작(PATH/JAVA_HOME)으로 기술함 | `docs/{ko,en}/PARSER_DESIGN.md:345/351` |

## 4 Finding 상세

### R10-01 변환 출력 상한이 실제 녹화에서 일찍 걸림 (P2)

**근거**: 변환은 `jfr print --json` 표준 출력을 `MaxOutput = 64 MiB`로 제한하고(`jfr.go:23,224`) 초과하면 `CONVERTER_OUTPUT_LIMIT`으로 실패한다. 문서는 "입력/출력 각각 64 MiB"를 한도로 설명하지만 `print --json`은 이진 JFR을 수십 배로 부풀린다. 레거시 `ParseRecording` 경로도 `convertContext`→`localconvert.JFR`을 거쳐 같은 한도를 상속한다. 오류 문구는 "호환되는 설치 JDK 도구를 선택하라"여서 원인이 크기라는 점을 가린다.

**재현**(Temurin 21 `jfr`, 이 커밋에서 빌드한 CLI):

| 입력 | JSON 출력 | 결과 |
|---|---|---|
| 합성 JFR 424 KB(`settings=profile`, 단일 스레드) | 2.27 MB | 변환 성공(JDK 11/17/21 모두) |
| 실제 JFR 10.5 MB(`settings=profile`, 8 스레드 40초) | **310 MB** | `jfr convert`: `CONVERTER_OUTPUT_LIMIT`. `jfr analyze-native-memory`(`ARCHSCOPE_JFR_CLI` 지정)도 같은 오류. |
| 같은 10.5 MB 녹화를 수동 변환한 310 MB JSON | — | `jfr analyze-json`은 성공(3 s, 최대 RSS 약 2.4 GB, 결과 43.8 MB) |

확장 배율이 약 5배(작은 녹화)에서 약 30배(이벤트가 많은 녹화)이므로 사실상의 입력 한계는 2~12 MB 수준이다. 이는 문서가 말한 64 MiB와 크게 다르다. 변경 전 레거시 경로는 출력에 상한을 두지 않았고(파일 머리말에 "수백 MB도 처리"라고 적혀 있었다), 전체 분석 경로에서 이전에 처리되던 일반적인 프로파일 녹화가 거부된다. 작성 측 증거(`jdk-summary.json`)는 424 KB 합성 녹화 하나만 사용해 이 한계를 드러내지 못했다.

**영향**: 가장 흔한 사용 시나리오(운영 JFR 분석)가 새 변환기를 쓰면 막히고, 사용자는 오류 문구만으로는 JDK 문제로 오해한다. 우회로(수동 `jfr print --json` 후 JSON 분석)는 있지만 이 과업의 목적이 로컬 변환 안내다.

**수정 제안**:
1. 출력 상한을 JSON 분석 경로가 실제 처리하는 크기와 맞추거나 구성 가능한 상한(예: 입력 크기에 비례, 디스크 스풀 기준)으로 바꾼다. 상한을 유지한다면 입력 상한을 출력 배율에서 역산해 문서화하고, 입력 단계에서 미리 거부한다.
2. `CONVERTER_OUTPUT_LIMIT`·`CONVERTER_BUDGET_EXCEEDED`·`IMPORT_BUDGET_EXCEEDED`마다 크기가 원인임을 알리는 별도 문구를 준다(JDK 선택 안내와 구분).
3. 회귀 테스트: 이벤트가 많은 합성 도구 출력(예: 100 MB)으로 상한 동작과 오류 문구를 고정한다. 증거에 10 MB급 실제 녹화를 포함한다.

### R10-02 provenance 도구 정체성 (P3)

**근거**: `JFR()`은 절대 경로인지(`jfr.go:164`)만 확인하고 파일 이름·형식은 확인하지 않는다. 결과 provenance는 항상 `Tool: "jdk.jfr"`(`jfr.go:266`)이다.

**재현**: `--version`에 `9.9`를 출력하고 JSON을 내는 셸 스크립트(`notjfr.sh`)를 `--cli`로 주면 변환이 성공하고 provenance가 `"tool":"jdk.jfr","version":"9.9"`로 저장된다. 실행 파일 SHA-256은 정확히 기록되므로 식별은 가능하지만 도구 이름은 거짓이다. 또 JDK 11/17의 `jfr --version`은 JDK 버전이 아니라 도구 버전 `1.0`을 출력해 두 JDK의 provenance `version`이 같다(문서 87행이 이를 인정). Wails 바인딩 `ConvertEvidenceJFR`가 렌더러가 준 임의 절대 경로를 고정 인자(`--version`, `print --json <파일>`)로 실행할 수 있다는 점도 같은 근거다.

**수정 제안**: 기본 이름이 `jfr`/`jfr.exe`인지 검사하고(아니면 명시 거부 또는 provenance에 `declared_unverified` 표기), 가능하면 JDK 릴리스 파일(`../release`)의 버전을 별도 필드로 기록한다.

### R10-03 stderr 한도로 성공한 변환이 실패 (P3)

**근거**: stderr는 `io.Discard`로 보내면서 16 KiB 한도 초과를 `overflow`로 취급해 `CONVERTER_OUTPUT_LIMIT`으로 실패시킨다(`jfr.go:116,126`). **재현**: 20,000바이트를 stderr에 쓰고 정상 JSON을 내는 도구가 `CONVERTER_OUTPUT_LIMIT`으로 실패한다(종료 코드 1). 경고를 많이 출력하는 JDK/래퍼에서 정상 변환이 막힌다. **수정 제안**: stderr는 한도까지만 버리고(종료 코드로 성공 판정) 초과분을 잘라낸다.

### R10-04 충돌 늦은 검사·임시 경로 노출·메모리 (P3)

**근거**: 출력 대상이 이미 있는지는 변환을 마친 뒤 `SaveNew`에서야 확인되어 최대 2분짜리 변환이 낭비되고, 오류가 `link /…/.input-assist-<난수> /…/c21.json: file exists`처럼 내부 임시 파일 경로를 그대로 노출한다(`save.go`, `localfiles/publish.go`). 또한 변환 후 `os.ReadFile`로 출력 전체를 읽고(`jfr.go:239`) `Unmarshal`하며 `SaveNew`에서 한 번 더 읽는다. 62.9 MB 출력에서 최대 RSS 275 MB(약 4.4배)였고 동시 2건이면 두 배다. **수정 제안**: 변환 전에 대상 존재를 확인하고, 오류를 `OUTPUT_EXISTS` 같은 코드로 감싸며, 구조 검증은 스트리밍 토큰 검사로 한다.

### R10-05 문서 불일치 (P3)

**근거**: `PARSER_DESIGN.md` 345행(ko)과 351행(en)의 기존 JFR 절이 여전히 `ARCHSCOPE_JFR_CLI → JAVA_HOME/bin/jfr → PATH`를 기술한다. 새 `INPUT_ASSISTANCE` 문서는 "PATH/JAVA_HOME 탐색만으로 실행을 허가하지 않는다"고 명시하고 실제 동작도 후자다(재현 완료). 같은 문서 안에서 두 설명이 충돌한다. **수정 제안**: 기존 JFR 절을 새 동작으로 고치거나 T-633 절을 가리키게 한다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `3d443d3`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 102개 패키지 모두 `ok`
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON의 소스·증거 해시 88개 대조
- 실제 CLI: `evidence discover`(50/70/300 MiB, sidecar 변형 4종, 악성 파일명, 번들 팩 첫 행 미리보기), `collection-kit`(ZIP 5개 파일 확인, 관리 경로 `OUTPUT_MANAGED_PATH` 거부, 기존 파일 충돌), `jfr convert`(Temurin 11/17/21, 가짜 도구 6종, 취소, 한도, 충돌), 레거시 `jfr analyze-native-memory`(환경변수 유무), 10.5 MB 실제 녹화 변환과 수동 변환 JSON 분석
- throwaway 스크립트와 녹화 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 WebView UI 클릭 흐름(작성 측은 화면 잠금으로 macOS UI를 관찰하지 못했다고 기록)
- Windows 네이티브: Job Object·CREATE_SUSPENDED 변환 실행, 취소, 프로세스 트리 종료(`process_windows.go`는 정독·교차 vet만)
- 데스크톱 외부 연결 관측, hosted CI, 실제 producer 폭넓은 qualification
- 오프라인 syscall 관측은 재실행하지 않았다(증거·스크립트 정독). 증거는 Linux CLI 프로세스 트리 7개에 한정된다.
- 도구 해시를 실행 직전에 계산한 뒤 실행하는 사이의 교체(TOCTOU)는 실행 중·후 재해시로만 탐지하며 별도 재현은 하지 않았다.

## 6 권고 처리 순서

1. R10-01 — 출력 한도·입력 한도 관계 재설계, 구분된 오류 문구, 10 MB급 실제 녹화 회귀. T-633 닫기 전.
2. R10-03, R10-04 — 변환 안정성(stderr 허용, 사전 충돌 검사, 스트리밍 검증).
3. R10-02 — 도구 정체성 표기.
4. R10-05 — 문서 정정.
