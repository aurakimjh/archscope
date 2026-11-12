# T-636 고정된 보고서 이미지·표(Visual Artifact) 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-636 구현 커밋 `4864ddd`(feat(reports): implement T-636 pinned visual artifacts). 직전 `4542e89`(T-638 리뷰) 위에 놓이며, 이 커밋은 T-638 리뷰 R13-01의 로컬 수정(`metrics_timeseries` 0.2.0)도 함께 포함한다.
- 계약 기준: `work_status.md`의 T-636 행과 완료 기록, `docs/ko/VISUAL_ARTIFACTS.md`, `docs/ko/DATA_MODEL.md`·`METRIC_HISTORY.md`의 T-636/R13-01 절
- 증거: `docs/review/evidence/2026-11-12-t636/local.json`(`source_repository_blob_sha256` 62개와 `evidence_sha256` 26개가 `4864ddd` 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(HTTP 문서 변경 등)은 수정하지 않았다.
- 방법: 소스 정독(`visuals/{model,render,artifact,report}.go`, `analysis/visual_view.go`, `cmd_visual.go`, Wails `visualservice.go`, `visualclipboard/*`, 프런트엔드 `VisualArtifactActions/Library.tsx`·`visualRaster.ts`), 저장소 밖 scratch worktree(`4864ddd`)에서 빌드한 실제 CLI로 격리 상태 루트에서 실제 Go heap pprof 두 개를 정규화한 결과에 `visual create-spec`(Top·flamegraph·diff)·`validate`·`inspect`·`export`를 끝까지 실행했다. throwaway Go 테스트로 표 변환 18종, ZIP 변조, 크기·Report Pack 한도를 측정하고 삭제했다. R13-01 수정은 합성 OTLP로 다시 측정했다.
- 범위: AnalysisView/VisualArtifactSpec 고정, 4개 template(provider table, profile Top, flamegraph, diff), SVG·표(TSV/HTML) 생성, ZIP 보관·검증·가져오기·Report Pack, CLI 명령, Wails 렌더 세션·클립보드 서비스, 그리고 T-638 R13-01 수정의 확인. 실제 숨은 WebView 렌더, 네이티브 클립보드, Office 붙여넣기, Windows 실행은 범위 밖(작성 측도 미관찰)이다.

## 1 결론

**판정: 조건부(P2 1건, P3 6건).** 고정 계약의 핵심은 실제 실행에서 성립했다. view·spec 바인딩 변조가 모두 거부되고(7종 중 4종 바인딩 거부, 2종 schema 거부, 1종은 의도된 과거 spec 허용), Top 그림의 값(분모 3,529,624, self 순 `StartCPUProfile` 1,179,824 등)이 `profile-query`와 정확히 일치하며, 가져온 ZIP의 HTML은 신뢰하지 않고 view에서 다시 만들고, CLI는 새 렌더와 클립보드를 명시적으로 거부하며 저장 바이트를 그대로 내보낸다. T-638 R13-01 수정도 실제로 동작한다. 다만 **편집 가능한 표 복사/TSV/HTML이 `/`가 들어간 모든 문자열 앞에 `'`를 붙여** 기본 Top 표의 Go 패키지·경로 이름을 바꾸고(R14-01), 그 외 diff 순서, offset 무시, 5장 초과 spec 통과, 자기 증명 ZIP, 이미지당 10 MB 글꼴 포함에 따른 용량, R13-01 이후 generic 표의 정보 손실이 P3로 남았다.

- **T-636 `DONE` 판정:** R14-01(P2)을 처리한 뒤 `DONE` 복귀를 허용한다. R14-02~R14-07은 후속 큐에 넣으면 된다. 네이티브 WebView 렌더·클립보드·Office 붙여넣기·Windows 수용은 작성 측이 밝힌 대로 별도 항목이며 이 리뷰로 대체되지 않는다.
- **후속 작업과의 관계:** T-637·T-639~T-642가 이 view/spec 계약 위에 template을 추가한다. R14-02(diff 순서)와 R14-05(ZIP 증명 범위)는 새 template이 늘기 전에 정하는 편이 좋다.

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| Top 그림은 기존 query 값을 그대로 고정 | 충족 | 실제 Go heap `alloc_space`: view `denominator 3529624`, 행 순서와 self/inclusive가 `profile-query`의 self 정렬과 같다(`StartCPUProfile` 1,179,824, `main.main` 1,083,368, `compress/flate.NewWriter` 663,552). `quality_label`에 observed·lost·symbols·clock·conversion flags가 들어간다. 이는 T-635 R12-02 수정이 visual에 연결된 것이기도 하다. |
| profile cursor가 있으면 그 쪽을 그린다 | 충족 | 질의 cursor를 준 경우 `view.offset 5`, 6~10번째 행. |
| flamegraph | 충족 | 238행 트리에서 limit 100 → 노드 100개, 깊이 0~10, `x+width` 최대 0.984(≤1), "일부 페이지만 그리며 숨긴 노드의 공간을 유지" 경고가 view에 남는다. 음수/계산 불가 layout은 코드에서 거부. |
| diff | 충족(범위 한정) | 두 결과의 digest 2개, `separate source populations; no common denominator`, 엄격 비교만 허용. `runtime.main 2263560 → 4466216 (delta 2202656)` 같은 정확한 값. |
| spec·view 변조 | 충족 | view 값 변경, spec의 `view_sha256`/template 변경, view의 denominator 변경은 `VISUAL_SPEC_BINDING_MISMATCH`, presentation 폭 1700·제목 300자는 schema 거부. 글꼴 hash가 다른 spec은 과거 spec으로 허용되며 `exact_dependencies_available:false`(의도된 동작). |
| ZIP 무결성(부분 변조) | 충족 | 이미지만 바꾸고 manifest를 그대로 두면 `VISUAL_ARTIFACT_HASH_MISMATCH`. |
| 불신 HTML | 충족 | 가져온 ZIP의 `table.html`에 `<script>`를 넣어도 CLI/Wails/Report Pack 모두 view에서 HTML을 다시 만들어 사용한다. 내보낸 HTML 캡션은 `rows 1–5 / 161 · bytes · denominator 3529624`. |
| CLI 경계 | 충족 | `visual render`는 `VISUAL_DESKTOP_RENDERER_REQUIRED`, `visual clipboard`는 `VISUAL_CLIPBOARD_UNSUPPORTED`. 저장 ZIP에서 html/tsv/png/svg/view/spec/zip 내보내기는 저장 바이트와 같고(PNG 18,598 B), 범위 밖 `--page 3`은 `VISUAL_PAGE_INVALID`, 기존 파일은 `OUTPUT_EXISTS`로 덮어쓰지 않는다. |
| 렌더 세션 설계(코드 정독) | 충족(미실행) | 토큰은 128비트 난수·세션당 창 이름 `visual-<token>`에 묶이고, 동시 2건·60초/50초 한도, 완료·취소 때 창과 job 정리, PNG 크기·차원 검증. 글꼴 hash를 렌더 전에 대조하고 불일치는 PNG를 만들지 않는다. |
| 증거 무결성 | 충족 | `source_repository_blob_sha256` 62개, `evidence_sha256` 26개가 커밋 내용과 모두 일치. |
| T-638 R13-01 수정 확인 | 충족 | 합성 OTLP 90,000점/10 series: series 3개 선택 17.7 MB, 전체 선택 59.0 MB(이전에는 둘 다 `METRIC_RESULT_BUDGET`). 40,000점/4 series: 1개 선택 6.56 MB, 전체 26.2 MB. 번들된 0.1.0 저장 결과도 `metrics history-query`로 읽힌다(`first_cumulative_point`, `derived`). 오류 문구에는 바이트·점·series 수가 포함된다. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R14-01 | P2 | 표 복사/TSV/HTML이 `/`가 있는 모든 문자열 앞에 `'`를 붙여 기본 Top 표의 이름을 바꿈 | `visuals/render.go:51-70` |
| R14-02 | P3 | diff 그림의 행이 노드 ID 문자열 순이라 변화가 큰 행이 아니라 임의 행이 첫 이미지에 나옴 | `profilemodel/operations.go:273`, `analysis/visual_view.go:227` |
| R14-03 | P3 | profile Top/flamegraph에서 `offset`을 조용히 무시하고 spec에는 그대로 기록 | `analysis/visual_view.go:118-161` |
| R14-04 | P3 | 5장을 넘는 presentation이 `create-spec`/`validate`를 통과하고 렌더 때 실패 | `visuals/model.go:162`, `render.go:120` |
| R14-05 | P3 | 보관·가져온 ZIP과 `visual validate`가 자기 증명이라 일관되게 바꾼 PNG도 통과 | `visuals/artifact.go:245-318` |
| R14-06 | P3 | 이미지마다 10.4 MB 글꼴을 포함(SVG 13.9 MB)해 5장 묶음이 33 MB, 보관함 약 16건, Report Pack 3건부터 실패 | `visuals/render.go:124`, `report.go:90` |
| R14-07 | P3 | R13-01 수정 후 generic 상세/HTML/Report Pack의 metric 표가 `point_id`·`status`만 담음 | `analyzers/metrichistory/analyzer.go:83-100` |

## 4 Finding 상세

### R14-01 표 보호 apostrophe의 범위 (P2)

**근거**: `tableText`는 수식 시작 문자(`= + @ -`)뿐 아니라 `strings.Contains(trimmed, "/")`인 모든 문자열과 15자리를 넘는 숫자 문자열에 `'`를 붙인다(`render.go:66`). 같은 함수가 TSV와 HTML 표 셀, 표 머리글, 클립보드(`CopyVisualArtifact`의 `HTML`·`Text`)에 모두 쓰인다. HTML은 이미 셀 스타일에 `mso-number-format:'\@'`(텍스트 형식)을 요청한다.

**재현**(`tableText` 18종 + 실제 Top 표 내보내기):

| 입력 | TSV/HTML 셀 |
|---|---|
| `runtime/pprof.StartCPUProfile` | `'runtime/pprof.StartCPUProfile` |
| `compress/flate.NewWriter`류 모든 Go 패키지·`/usr/lib/…` 경로·URL | 앞에 `'` |
| 머리글 `Frame / evidence` | `'Frame / evidence` |
| `1/3`, `=1+1`, `@SUM(1)`, `-1.5e3`, 단독 `-`, 16자리 이상 정수 | 앞에 `'` |
| `2024-01-02`, `1e999999` | 그대로(문서는 "날짜처럼 보이는 문자열"도 보호한다고 설명) |
| 한글, `0.1`, `-5`, `a-b`, `C:\a\b` | 그대로 |

실제 Top 표를 `visual export --format tsv`로 내보내면 첫 행이 `'runtime/pprof.StartCPUProfile`, 헤더가 `'Frame / evidence`로 나온다.

**영향**: 문서도 "해석하지 않는 앱에는 보호 문자가 보일 수 있다"고 밝히지만, 기본 Top 표의 상당수 이름(Go 패키지 경로, Python/Node 파일 경로, URL, `Frame / evidence` 같은 헤더)이 보호가 필요 없는데도 바뀐다. 이 표의 용도는 Word/PowerPoint에 붙여 넣는 "정확한 문자열" 보고이고, HTML에는 Excel용 텍스트 지정이 이미 있다. 네이티브 Office에서 `'`가 보이는지는 관찰하지 못했다(작성 측도 미관찰). 보이지 않더라도 Excel TSV 붙여넣기에서 날짜형 문자열은 오히려 보호되지 않는다.

**수정 제안**: 보호 규칙을 실제 위험 패턴(수식 시작 문자, 분수 `숫자/숫자`, 날짜형, 긴 정수)으로 좁히고 형식별로 나눈다(HTML은 `mso-number-format`으로 충분하면 문자 추가 없이, TSV만 선택 보호). 머리글에는 적용하지 않는다. 원래 문자열과 화면 문자열이 달라지는 셀 수를 view/manifest에 남겨 사용자가 알 수 있게 한다.

### R14-02 diff 그림의 행 선택 (P3)

**근거**: `Compare`가 노드 ID를 `sort.Strings`로 정렬한 순서로 행을 만들고(`operations.go:273`, T-629부터의 동작), `profile_diff` view는 그 목록을 `offset`·`limit`으로 자르기만 한다(`visual_view.go:227`). 열 이름은 `Before/After/Delta`뿐이라 inclusive 값이라는 사실이 그림에 없다.

**재현**: heap 두 개 diff, 행 829개. limit 6의 첫 이미지: `uint64 0→0`, `makeslice 0→1152`, `int64 0→32`, `varint 0→128`, `runtime.main 2263560→4466216(+2,202,656)`, `newProfileBuilder 0→0` — 변화 없는 행이 섞이고 가장 큰 변화는 5번째에 있다. 5장×25행이 최대라 829행 중 125행만 담을 수 있다.

**수정 제안**: `profile_diff` query에 `sort: delta_abs|delta|name`과 `omit_zero_delta`를 추가해 spec에 고정하고, 열 이름에 inclusive를 표시한다. percent/share_delta 열 포함 여부도 정한다.

### R14-03 `offset` 무시 (P3)

**근거/재현**: 문서 CLI 예시는 `{"kind":"profile_top","offset":0,"limit":25}`인데 profile 종류는 `offset`을 쓰지 않고 `query.cursor`만 쓴다(`visual_view.go:118-161`, `out.Offset`은 cursor에서 계산). `offset:5,limit:5`로 만들면 view는 `offset 0`, 첫 5행이고 `spec.query.offset`은 5로 기록된다. 거부도 경고도 없다. diff와 provider table은 `offset`을 쓴다.

**수정 제안**: profile 종류에서 0이 아닌 `offset`은 `VISUAL_QUERY_INVALID`로 거부하거나 cursor로 변환하고, 문서 예시를 종류별로 나눈다.

### R14-04 5장 한도 사전 검증 부재 (P3)

**근거/재현**: 이미지 5장 한도는 `SVG` 생성(`render.go:120`)에서만 검사한다. `{"kind":"profile_top","limit":100}`에 `rows_per_page:5`(20장 필요)로 `create-spec`이 성공하고 `visual validate`가 `valid:true`를 반환한다. 데스크톱은 렌더 시작 후에야 `VISUAL_PAGE_BUDGET`으로 실패한다(창 생성 전이라 부작용은 없음).

**수정 제안**: `Spec.Validate`/`create-spec`에서 `Pages(view, presentation) ≤ 5`를 검사하고 `validate` 응답에 필요한 이미지 수를 포함한다.

### R14-05 자기 증명 ZIP (P3)

**근거/재현**: `ReadArtifact`는 manifest의 파일별 hash와 spec/view 일관성, PNG의 디코드·크기, SVG의 구조·글꼴만 검사한다. throwaway 테스트에서 `image-01.png`를 전혀 다른(빨간) PNG로 바꾸고 manifest의 hash도 다시 계산해 묶으면 `ReadArtifact`가 성공한다. PNG와 SVG·view가 같은 내용을 그렸는지는 어디서도 확인되지 않는다. `visual validate`도 spec과 view의 일치만 보고 `valid:true`를 낸다(가짜 view를 일관된 hash로 만들어도 통과). 부분 변조는 막지만 서명이나 출처 증명이 없어 "검증된 ZIP"이라는 표현이 과하다.

**수정 제안**: 문서에서 "손상·부분 변조 감지"로 범위를 한정하고, 필요하면 Ed25519 서명(T-632의 신뢰 키 저장소 재사용)으로 생성자를 증명한다. SVG를 view에서 다시 만들어 저장된 SVG와 대조하는 검사(`SVG` 함수가 결정적)는 비용이 작다. PNG는 WebView 렌더 결과라 같은 방식으로 검증할 수 없음을 명시한다.

### R14-06 이미지당 글꼴 포함에 따른 용량 (P3)

**근거/재현**: 각 SVG가 10,414,588 B 글꼴을 base64로 포함한다(`render.go:124`). 100행 표(5장) 측정: SVG 13.89 MB × 5, ZIP 33.4 MB. 문서의 보관함 한도 "32건/총 512 MiB"는 5장 묶음 기준 약 16건이다. Report Pack: 5장 묶음 1건 33.4 MB, 2건 66.8 MB 성공, **3건·4건은 `VISUAL_REPORT_BUDGET: select fewer artifacts/results`**(해제 192 MiB 초과). 문서는 "선택 1–4건의 Report Pack"과 "압축 96 MiB/해제 192 MiB"를 말한다.

**수정 제안**: 사용 문자만 담은 글꼴 부분집합(OFL 허용 범위 확인)을 SVG에 포함하거나, ZIP 안에 글꼴을 한 번만 두되 SVG 자기 완결성 요구와의 절충을 정한다. 어느 쪽이든 5장 묶음 기준 실효 한도를 문서에 적는다.

### R14-07 R13-01 이후 generic 표의 정보 (P3)

**근거/재현**: 0.2.0 결과의 `tables.points`는 `point_id`·`status`, `timed_points`는 `time`·`clock`·`point_id`만 갖는다(`analyzer.go:83-100`). 합성 4점 counter를 `evidence import --export-format html`로 내보낸 HTML의 표에는 이름·단위·값·delta·rate가 없고 point ID와 상태뿐이다. 값은 전용 질의(`metrics history-query`, 전용 탐색기)로만 볼 수 있다. 이전(0.1.0) HTML과 Report Pack 부록에는 구간 값이 있었다. 문서는 CSV가 요약만 담는다는 점만 정정했고 HTML/Report Pack/generic 상세가 같은 상태가 되었음은 설명하지 않는다.

**수정 제안**: 문서에 generic 표가 ID·상태·시각 투영이라고 명시하거나, 값 열을 담는 별도 투영을 후속(R13-06 CSV 투영과 함께)으로 정한다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `4864ddd`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 107개 패키지 모두 `ok`(테스트 파일 없는 5개 제외)
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON 해시 88개 대조
- 실제 CLI 시나리오: Top(offset/cursor)·flamegraph·diff·provider 경로 `create-spec`, `validate` 변조 7종, 표 변환 18종, 5장 초과 spec, ZIP 변조(부분/일관된 manifest), 저장 ZIP의 7개 형식 내보내기와 거부(render/clipboard/범위 밖 page/기존 파일), 크기·Report Pack 한도, T-638 R13-01 규모 측정과 0.1.0 reader
- throwaway 테스트·생성 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 숨은 네이티브 WebView 렌더(글꼴 로딩 후 SVG→canvas→PNG), 네이티브 클립보드 쓰기, Office 붙여넣기(그림·편집 가능한 표·`'` 가시성), Windows 실행. 작성 측도 Mac 잠금으로 관찰하지 못했다고 기록했고 이 리뷰도 새 근거를 만들지 못했다.
- `visualRaster.ts`의 글꼴 준비 검사는 문서에 붙인 `FontFace`를 확인하는데 실제 SVG는 자체 `@font-face`(data URL)를 쓴다. SVG-as-image 안의 글꼴이 `image.decode()` 직후 사용 가능한지는 Chromium 증거(작성 측 receipt)에 의존하며, macOS WKWebView에서의 동작은 확인하지 못했다.
- 작성 측 Chromium raster 동일성(DPR·viewport 변경 후 PNG SHA-256 동일) 재실행은 하지 않았다(receipt 정독).
- 오프라인 syscall 관측은 재실행하지 않았다(증거 정독). hosted CI.
- DIBV5/CF_HTML 바이트 구성은 코드와 단위 테스트 통과만 확인했고 OS 클립보드에는 쓰지 않았다.
- T-638 R13의 P3 후속 큐(R13-02·03·04·05·06·07)는 이 리뷰에서 다시 평가하지 않았다.

## 6 권고 처리 순서

1. R14-01 — 표 보호 규칙 축소와 형식 분리, 변경된 셀 수 공개. 새 template 전.
2. R14-02 — diff 정렬·0 변화 제외·열 이름. R14-05와 함께 template 계약 확정.
3. R14-03·R14-04 — offset 거부/변환, `create-spec`·`validate`의 5장 검사.
4. R14-05 — 문서 범위 한정, SVG 재생성 대조 또는 서명.
5. R14-06 — 글꼴 부분집합 또는 한도 문서화.
6. R14-07 — generic 표 정보 범위 문서화 또는 값 투영.
