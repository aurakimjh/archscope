# 고정된 보고서 이미지와 표

시각 자료는 기존의 불변 분석 query를 소비하는 데스크톱 시각 자료 renderer를 구현합니다.
PNG, 외부 참조 없는 SVG, 편집 가능한 HTML 표와 TSV를 만들고 이식 가능한 ZIP으로 보관합니다.
기존 `SaveChartArtifact`는 이미 렌더된 바이트를 저장하는 API로 유지합니다.
슬라이드 작성 도구를 추가하지 않습니다. 네이티브 Office 수용 검증은 backend·브라우저 검사와 별개입니다. [영문 안내](../en/VISUAL_ARTIFACTS.md)도 제공합니다.

## 현재 분석 화면에서 생성

Generic 결과 페이지, ProfileSet 드릴다운 또는 엄격한 비교가 허용된 화면의
**보고서용 이미지·표**를 여세요. 화면 DOM 대신 현재 backend query/page를 고정합니다.
Generic 표는 처음 최대 8개 열을 고르며 필요한 열을 직접 선택할 수 있습니다.
제목·주석·폭(1600/2400/3200)·높이·글자 크기·이미지당 행·투명 배경을 설정하고
**선택 고정 후 이미지 생성**을 누릅니다. 행이 들어가지 않으면 행 수 축소/높이 증가를 안내합니다.
기본값은 2400×1600, 26px, 20행, 흰 배경입니다.

| Template | 적용 입력과 범위 |
|---|---|
| `provider_table` | 사용 가능한 generic 표/요약/상세 provider의 선택 페이지·최대 8개 열. 셀별 단위·availability를 유지하고 공통 분모를 만들지 않습니다. |
| `profile_top` | 기존 Top/self·callers/callees·evidence query의 metric·filter·selection·sort·cursor를 유지합니다. |
| `profile_flamegraph` | 기존 비음수 tree layout. Inclusive/self 문자열이 기준이며 면적은 표시용 근사치입니다. 선택 페이지/깊이만 그리며 숨긴 노드의 가로 공간을 유지하고 세로 시작 위치는 선택 페이지를 따릅니다. 필요하면 하위 트리에 집중하세요. 음수/계산 불가 layout은 거부합니다. |
| `profile_diff` | 기존 엄격 비교가 허용된 두 결과의 digest·별도 모집단·정확한 delta와 공통 0축을 사용합니다. 매핑/재분석 비교의 별도 자격 조건은 완화하지 않습니다. |

새 GPU/serving/latency producer는 후속 소비자이며 현재 검증한 별도 chart template이 아닙니다.
Renderer는 원본 로그를 재해석하거나 새 관측을 계산하지 않습니다. 별도의 숨은 네이티브
WebView에서 글꼴 SHA-256을 검증하고 로딩을 기다린 뒤 SVG를 고정 canvas로 바꿉니다.
화면 scroll·hover·theme·viewport·DPR에 의존하지 않습니다. Backend 60초/surface 50초,
동시 2건, native window/token binding을 적용하며 완료·취소 때 세션과 창을 정리합니다.

그림에 시간 범위(미선언이면 그 상태)·단위·분모·행 범위·페이지 번호·source/view/query 식별자·
품질 요약과 한계를 넣습니다. 긴 이름/축은 줄일 수 있으며 정확한 문자열과 전체 문맥은
`view.json` 및 표에 남습니다. Query 100행, view 200행/노드, 열 8개, 이미지 5개,
800만 pixel, view 4 MiB 이내입니다. 전체 데이터셋 내보내기로 오해하지 않도록 범위를 표시합니다.

## 복사·저장·보관

**이미지 복사**는 native PNG와 Windows DIBV5 또는 macOS TIFF 호환 표현을 씁니다.
**선택한 표 전체 복사**는 분할 이미지 전체에 걸친 선택 행을 HTML과 Unicode TSV로 씁니다.
OS 쓰기가 끝난 뒤 성공을 알리고 clipboard 사용 중/미지원 상태는 오류로 돌려줍니다.
실패하면 PNG/SVG/HTML/TSV 파일을 저장하세요. Linux/headless는 native clipboard를 지원하지 않습니다.

HTML은 spreadsheet 텍스트 서식을 요청하고 수식형 문자열만 보호합니다. TSV는 수식·숫자 분수/날짜·
지수 표기·앞자리 0 숫자·15자리 초과 숫자에만 선택적으로 apostrophe를 붙입니다. 일반 경로/URL과
머리글은 유지하고 수식형 머리글만 별도로 보호합니다. Manifest의 `table_protection`에 정책 버전과
TSV/HTML의 변경 셀 수·수식형 머리글 수를 기록하며 복사 UI에도 보여 줍니다. 이를 해석하지 않는
앱에는 보호 문자가 보일 수 있습니다. 원래 선택 문자열은 `view.json`에 유지하며 숫자 강제 변환이나
수식 실행을 요청하지 않습니다. Office는 PNG를 기본으로 사용하세요. SVG 삽입은 Office/WebView
버전에 따라 다르며 파일 자체에 글꼴·style·OFL license를 넣고 외부 참조를 쓰지 않습니다.
공유 전에 내용을 확인하세요. 알려진 패턴의 index 마스킹은 개인정보 안전 보장이 아니며 명시적
query 식별자와 출처도 아티팩트에 포함됩니다.

**Evidence Board에 보관**은 참조를 카드에 저장합니다. Board의 **시각 자료 보관함**은 실제
이미지/spec/view/manifest를 케이스와 별도로 불변 보관하며 명시적으로 삭제합니다(32건/총512 MiB).
다른 PC로 옮기려면 ZIP을 저장하세요. 동일 ZIP 바이트는 중복 저장하지 않고 새 리비전은 과거
파일을 덮어쓰지 않습니다. 보관함에서 무결성 확인된 ZIP 열기·과거 PNG 열기·묶음 저장과 선택 1–4건의
시각 자료 Report Pack을 제공합니다. 선택하면 현재 결과 최대 8개의 기존 backend JSON/HTML
계약을 부록으로 넣습니다. 원본 데이터나 새 분석을 자동 포함하지 않습니다.
관리 경로·기존 파일 덮어쓰기, ZIP 경로 탈출·symlink·hash 누락/변조, SVG script/외부 참조는 거부합니다.
ZIP은 압축96 MiB/해제192 MiB 이내입니다.

## 재현과 과거 버전

`spec.json`은 결과 SHA-256/type/payload 버전, 실제 query, view hash, 표시 옵션,
template/renderer·글꼴 hash·마스킹 버전을 고정합니다. `view.json`에 값·단위·분모·모집단·품질·
경고를 보관하고 `manifest.json`에 파일별 hash·페이지 수·실행 환경을 남깁니다.
PNG/SVG/표/license를 함께 보관하며 임시 result handle은 저장하지 않습니다.

과거 그림은 저장 바이트로 엽니다. **원버전 재생성**에는 고정 결과를 Workspace에서 다시 열고
동일한 template/renderer/font/masking 버전이 있어야 합니다. 의존성이 없거나 선택/표시가 달라지면
거부합니다. **새 버전으로 리비전 생성**은 이전 ID와 연결된 새 묶음을 명시적으로 만들며 과거
바이트를 유지합니다. 선택·값·layout 입력 재현과 다른 플랫폼 rasterizer의 바이트 동일성은
구분합니다. 같은 Chromium 환경에서만 DPR/viewport 변경 후 바이트 동일성을 확인했습니다.

## CLI

```sh
archscope-engine visual create-spec --in result.json --query '{"kind":"profile_top","offset":0,"limit":25}' --out visual.json
archscope-engine visual validate --in visual.json
archscope-engine visual create-spec --in before.json --target-in after.json --query '{"kind":"profile_diff","offset":0,"limit":25}' --out difference.json
archscope-engine visual inspect --in saved-visual.zip
archscope-engine visual export --in saved-visual.zip --format png --page 0 --out image.png
archscope-engine visual export --in saved-visual.zip --format html --out table.html
```

Spec 생성·검증과 저장된 ZIP/PNG/SVG/HTML/TSV 내보내기는 브라우저 없이 가능합니다.
`visual render`와 `visual clipboard`는 `VISUAL_DESKTOP_RENDERER_REQUIRED` /
`VISUAL_CLIPBOARD_UNSUPPORTED`로 명시적으로 거부합니다. 저장 SVG/PNG 내보내기는 신규 렌더가
아닙니다. 모든 출력은 새 파일로 씁니다.

## 남은 Windows·Office 실제 수용 검증

Windows PC의 패키지 앱으로 commit/build ID, Windows/WebView2/Office 버전, canvas/font 옵션,
artifact/view hash를 기록하세요. 한글·긴 이름·음수 delta·`9007199254740993`·`1/3`·수식 시작
문자열과 2장 이상 분할 이미지를 포함하는 합성 입력을 사용합니다.

1. Profile Top·집중 flamegraph·양/음 diff·generic 표를 생성해 현재 backend 선택과 합계·분모·
   품질을 대조합니다. Resize/scroll/theme 변경 후 값·배치·행 범위를 다시 확인합니다.
   한글과 흰 Office 슬라이드 위의 투명 PNG를 확인하고 PNG/SVG 크기를 spec과 대조합니다.
2. 앱에서 이미지를 복사해 PowerPoint에 붙입니다. 실제 그림 객체, 글자·단위·품질·크기·투명도를
   확인하고 발표자료 저장/재열기를 합니다. 저장 SVG는 별도로 삽입해 미지원도 그대로 기록하고
   PNG 대안을 유지합니다.
3. 설치된 Word·PowerPoint·Excel에 표를 붙입니다. 셀이 편집 가능하고 선택 행/열 전체·한글·
   긴 정수·분수가 보존되어야 합니다. 수식 문자열은 텍스트여야 하며 앱이 표시하는 보호 문자는
   위 설명과 대조합니다. 한 앱의 성공을 모든 Office 버전의 성공으로 확대하지 않습니다.
4. 렌더 취소/시작 화면 닫기·사용 중인 clipboard 재시도·ZIP 저장/재열기·과거 그림 hash를
   확인합니다. 고정 결과를 다시 열어 원버전의 새 child 리비전을 만들고 표시 변경은 명시적 새
   리비전으로만 합니다. 미지원 과거 template은 저장 PNG를 열 수 있으나 원버전 재생성은 거부합니다.
5. 네트워크를 끊고 render/copy/save/library/report-pack을 반복하며 desktop process tree의
   outbound 동작을 별도로 관측합니다. CLI syscall·cross-build는 native WebView offline 증거가 아닙니다.

로컬 근거는 `docs/review/evidence/2026-11-12-t636/`에 둡니다. Chromium raster·계약 테스트·
clipboard wire format·cross-build를 네이티브 Windows/Office 수용으로 주장하지 않습니다.
이번 실행에서는 Mac 데스크톱이 잠겨 있었습니다.

<a id="r14-후속-처리와-검증-범위"></a>

## 검증 범위

지원 SVG는 고정 view/spec/글꼴로 결정적으로 재생성해 바이트를 대조합니다. ZIP hash는 작성자 인증이나 PNG 픽셀 증명이 아닙니다. 미지원 과거 renderer는 구조/hash 검사만 유지하며 실제 Office 동작은 별도 수용 검증이 필요합니다. 독립 이미지는 고정 전체 글꼴을 포함하고 실제 압축/확장 byte를 표시합니다.

spreadsheet-text-0.2.1은 0.1·1.5·3.14·12.5 같은 일반 소수를 TSV 숫자로 유지한다. 연도가 있는 dotted date는 보호한다. 과거 0.2.0 receipt는 당시 count로 검증하고 현재 export/descriptor는 0.2.1을 사용하며 보관 byte는 재작성하지 않는다.


새 profile_diff는 sort=delta_abs, omit_zero_delta=true가 기본이며 delta(부호 있는 내림차순)와 name을 선택할 수 있습니다. 동률은 프레임 이름·노드 ID로 정하고 페이지 처리 전에 view/spec에 고정합니다. 열 이름에 inclusive를 표시합니다. 백분율·share delta는 정확한 비교 API에 남기고 여섯 열 이미지에는 추가하지 않습니다. 기존 저장 view는 유지합니다.

프로파일 시각화는 offset=0과 profile.cursor 페이지 처리를 사용합니다. 0이 아닌 offset은 결과 획득 전에 VISUAL_QUERY_INVALID로 거부합니다. diff/provider는 offset을 사용하며 과거 저장 view를 재해석하지 않습니다.

명세 생성·검증에서 렌더링 전에 required_images ≤5를 검사합니다. CLI visual validate는 required_images를 반환하고 초과 오류에는 필요한 이미지 수와 상한을 표시합니다.

지원하는 SVG는 고정 view/spec·글꼴로 결정적으로 재생성하여 바이트 단위로 비교하므로 manifest 해시까지 바꾼 SVG 교체도 거부합니다. 미지원 과거 생성기는 구조·해시 검사만 유지하고 새 렌더링을 승인하지 않습니다. ZIP 검사는 작성자를 인증하지 않고 PNG/SVG 픽셀 동등성을 보증하지 않습니다.

절충안: Unicode 글리프 누락과 외부 의존성을 피하기 위해 renderer1.0.0은 독립 SVG마다 전체 고정 글꼴을 유지하며 부분집합·중복 제거를 활성화하지 않습니다. descriptor/UI에서 실제 ZIP·해제 byte를 표시합니다. 다섯 이미지 묶음은 약33MB여서512MiB 보관함에는 대략16건, 해제192MiB Report Pack에는 명목상4건이 아니라 보통2건만 들어갑니다. 이미지·분석 부록에 따라 더 줄 수 있으며 byte 한도가 우선합니다. 거부 오류에 실제 해제 크기와 상한을 표시합니다.

컴파일된 공통 상세 provider metrics:intervals가 검증된 메트릭 캐시로 정확한 값/delta/rate/시각/상태/ref 페이지를 제공합니다. HTML에는 관측·파생 bucket JSON을 포함한 일회성 metric_intervals를 추가하고 Report Pack은 원본 SHA에 연결한 별도 표시 자료를 포함합니다. 저장0.2 최소 표·JSON 바이트는 유지합니다. 표시 한도를 넘으면 단독 HTML은 거부하고 Report Pack은 원본 JSON을 보존하면서 표시 불가·한도 사유를 명시합니다. 전체 선택은 전용 구간 CSV를 사용합니다.
