# 데이터 팩과 오프라인 배포

데이터 팩은 실행 플러그인 대신 제한된 JSONL 메트릭 스냅샷 해석기를 추가한다.
기본 합성 팩은 `synthetic.metrics` 버전 `1.0.0`만 검증하며 실제 제품,
시계열, 변환기 또는 운영 부하 지원을 주장하지 않는다.

## 팩 소유권과 활성화

설정의 데이터 팩 또는 `archscope-engine packs --help`를 사용한다. 설치는
manifest 원본 바이트의 분리 Ed25519 서명, 앱/소유자가 등록한 공개키,
닫힌 스키마, 모든 파일 해시, 정상/거부 fixture 결과를 검증한다.
ZIP 내부 키로 신뢰를 얻을 수 없다. 소유자는 공개키를 등록할 수 있고,
관리 권한 키는 순서가 연속된 오프라인 키 교체/폐기 문서에 서명한다.
개인키는 앱 외부에서 관리한다. 기본 팩 개인키는 배포하지 않는다.
폐기된 키의 팩은 활성화/롤백할 수 없다. 키가 폐기되거나 없으면 해당 팩을
휴면으로 분리하며 의존성이 충족되지 않는 팩도 휴면으로 전환한다. 활성화 의도와
보관 파일은 유지하지만 새 작업의 매핑에서 제외한다. 기본 importer와 영향받지
않은 팩은 계속 사용한다. 재시작해도 제거/폐기된 기본 키를 복원하지 않는다.
공개키 등록, 대체 팩 설치, 비활성화, 원본 백업을 위한 관리 기능은 유지한다.
실행 중인 작업과 저장 결과의 기존 해석/provenance는 바꾸지 않는다.

설정에서 서명된 신뢰 변경의 순서, 서명자, 추가/폐기 공개키, 영향받는 활성 팩과
의존 팩, 범위 문제를 검토한 뒤 적용한다. CLI에서는 다음 두 단계로 실행한다.

```sh
archscope-engine packs update-trust --revision REV --in update.json --signature signature.json --preview
archscope-engine packs update-trust --revision REV --in update.json --signature signature.json --reviewed-sha256 REVIEW_SHA256
```

미리보기의 `review_sha256`은 현재 상태 revision과 변경 문서/서명 원본 바이트에
묶인다. 파일이나 상태가 바뀌면 다시 검토한다. 활성 팩 영향 또는 범위 문제가 있으면
검토 해시 없는 CLI 적용을 거부한다. 현재 관리 키 서명과 정확히 다음 trust sequence
검증은 그대로 유지한다. `packs list`의 `unavailable` / `problems`를 확인한 뒤
`packs deactivate --revision REV --scope SCOPE --id PACK_ID`로 비활성화하거나
현재 신뢰하는 소유자 키의 대체 팩을 설치/활성화한다. `packs export`는 저장된
SHA-256이 일치하는 보관 원본을 키 폐기 후에도 백업한다. 백업은 현재 신뢰 부여나
폐기된 팩 활성화 허가가 아니다. 기존 별칭 선택이 유효하지 않으면 범위를 명시적으로
복구해야 하며 자동 별칭 우선 정책은 적용하지 않는다.

설치는 활성화와 분리한다. 활성화에는 현재 상태 revision이 필요하며 오류 시
기존 상태를 유지한다. 롤백도 동일한 서명/호환성/conformance 검사를 거친다.
재시작 시 중단된 staging을 제거한다. 커널 잠금 아래 sync 후 atomic rename을
사용한다. 프로세스 재시작 복구 검증을 전원 장애 내구성 보장으로 해석하지 않는다.
application 활성화/롤백은 모든 활성 user/case 범위를 검증한 뒤 커밋한다.
어느 활성화 순서에서도 canonical/별칭 충돌을 거부하며 application 의존 팩 교체로
기존에 사용 가능한 로컬 팩을 조용히 휴면으로 만들 수 없다.
의존성은 digest와 버전 범위를 고정한다. 앱/해석기/확장 버전 범위를 검사하며
현재 앱 버전은 0.3.5다.

미서명 팩은 명시적 `--unsigned-local`과 `--scope user` 또는
`case:case-<32자리 소문자 hex>`를 요구한다. `locally_authored`로 표시하며
application 범위에 넣지 않는다. 가져오기도 같은 범위를 명시적으로 선택해야 한다.
별칭 충돌은 `--choices '{"alias":"canonical.mapping-id"}'`로 선택한다.
로컬 자동 우선 정책은 없고 canonical ID를 덮어쓸 수 없다. 실행 중인 작업은
카탈로그와 팩 digest를 고정한다. 저장 결과는 그 provenance를 유지한다.

매핑은 리터럴 producer/version 조건, JSON pointer, 순서 있는 필드 별칭,
정확한 양수 소수 배율, 단위와 gauge/counter만 허용한다. 코드, 동적 표현식,
정규식, shell, SQL, template는 거부한다. 한도는 압축 8 MiB, 해제 16 MiB,
파일 4 MiB, ZIP 128개, manifest 256 KiB, 보관 버전 64개, 활성 스냅샷
32 MiB, conformance 10,000행, JSONL 한 행 64 KiB와 기존 작업/결과 예산이다.
소수 문자열로 분수/0/큰 수를 보존한다. 결과는 metrics_snapshot 0.1.1이며
기존 0.1.0 해석은 바꾸지 않는다. 새 매핑 메트릭 이름은 표시 경계와 같은
판정 함수로 C0/C1과 양방향 제어 문자를 거부하며 일반 유니코드 이름은 정확히
보존한다. 과거 저장 결과는 계속 읽고 원본 그대로 내보낼 수 있으며 표시 provider가
위험한 문자를 이스케이프한다.

스냅샷은 사용 가능한 revision을 한 번만 읽고 의존성/순환 검사에서 재사용한다.
프로세스 내부 검증 캐시는 최대 64항목/80 MiB admission charge로 제한한다.
항목당 해제 크기의 두 배와 64 KiB로 계산하며 실제 RSS 측정 한도는 아니다.
범위, 압축 파일 SHA-256, trust sequence, 서명자 키 fingerprint로 구분한다.
캐시가 있어도 매번 제한된 압축 원본을 다시 읽고 해시를 검사한다. 신뢰 변경 시
검증 캐시를 비우며 실패/취소/불완전 검증은 저장하지 않는다. 호출자에게 매핑 필드를
복사하고 종료/재시작 시 캐시를 제거한다. 최초 접근과 cold startup은 전체 서명,
스키마, conformance 검증을 수행한다.

## 단절망 묶음 생성과 실행

`go run ./cmd/archscope-engine offline build --engine /local/archscope-engine --out /new/bundle`
명령은 새 디렉터리에 실행 파일, 해시 목록, 필수 release fixture ZIP,
로컬 매뉴얼, 글꼴/라이선스, 스키마와 기본 팩을 구성한다. `--app`으로
동일 플랫폼 데스크톱 실행 파일을 추가할 수 있다. 실패하면 새 디렉터리를 제거한다.
런타임은 형제 저장소를 읽지 않는다. 필수 fixture ZIP의 digest를 바이너리에
고정하며 등록된 모든 importer를 포함해야 한다. 누락/변조/미포함 importer는
배포 검증을 실패시킨다. `offline fixtures`는 독립 importer 검증을 실행한다.

배포 실행 파일에 `--offline-root /bundle --offline-report /local/audit.json`을
전달한다. 엔진 작업이나 WebView 생성 전에 자원과 실행 파일을 검증하며
다른 OS/아키텍처 묶음은 거부한다. 해시 목록은 우발적 변경을 검출하며
배포 진위는 서명/신뢰된 전달 경로에 의존한다. 목록 자체가 진위를 보장하지 않는다.
격리된 새 설치 검증에는 `ARCHSCOPE_STATE_ROOT=/new/local/state`를 사용하고
읽기 전용 배포 디렉터리에 상태를 저장하지 않는다.

오프라인 세션은 capture, HTTP 요청, Ollama, automation 제어와 미검증 JFR
변환 실행을 자원 할당 전에 거부하고 대상 주소/본문 없이 작업 시도 횟수를 기록한다.
JFR은 미리 변환한 JSON을 사용한다. 명시적 설치 도구 변환은 [입력 안내](INPUT_ASSISTANCE.md)를 참조한다. UI는 동일 출처
CSP를 적용하고 위반을 기록한다. 글꼴/차트는 로컬이며 CDN, updater, telemetry,
symbol downloader를 구성하지 않는다. 앱 수준 거부 횟수와 OS/WebView 동작은
구분한다. 거부 횟수 0만으로 외부 연결 시도 0을 주장하지 않는다. CLI는 syscall
관측과 실패하는 네트워크 양성 대조군을 함께 사용하며 데스크톱은 별도 실행 근거가 필요하다.

Windows 데스크톱은 `--webview-runtime /local/fixed-runtime`과
`--webview-version <version>`이 필수다. 대상 아키텍처의 라이선스가 있는
Fixed Version 전체 파일을 제공한다. `msedgewebview2.exe`, `msedge.dll`,
대상 아키텍처의 `EBWebView/{x86|x64|arm64}/EmbeddedBrowserWebView.dll`,
locales, resources, 라이선스를 포함해야 한다. bootstrapper/다운로드/Evergreen
대체 경로를 쓰지 않는다. PE 아키텍처, 네이티브 product version과 전체 해시를 검사하고 시작 전 검증 후
Wails `WebviewBrowserPath`를 설정한다. 런타임 갱신은 소유자가 관리한다.
실제 Windows 시작/브라우저 프로세스 외부 연결 관측은 Windows PC 검증 항목이다.
[Microsoft 배포 지침](https://learn.microsoft.com/microsoft-edge/webview2/concepts/distribution)을 참조한다.
macOS는 OS WKWebView를 사용하며 OS 의존성은 묶음에 포함하지 않는다.
Linux CLI에는 WebView가 필요 없다. Linux 데스크톱 의존성 검증은 별도다.

내장 라이선스 목록에 호스트에서 컴파일한 Go 모듈, 설치된 npm 패키지(빌드 도구 포함),
Inter 원문 고지와 의존성 고지를 포함한다. 플랫폼/시스템/Fixed Version 런타임의
라이선스는 해당 플랫폼 배포에서 검토해야 한다.

Noto Sans KR 원본을 font manifest의 Google Fonts commit에 고정해 OFL-1.1과
원 저작권 고지와 함께 배포한다. 설정에 글꼴 로드 상태를 표시한다. fallback을
글꼴/레이아웃 검증 통과로 해석하지 않는다.

<a id="sqlite-결정"></a>

## 로컬 인덱스와 SQLite 범위

데이터 팩은 제한된 JSONL과 compact 메모리 인덱스를 사용합니다. Nsight importer는 별도로 고정된 순수 Go SQLite 의존성으로 불변 export 복사본에 고정 query를 실행합니다. 임의 SQL이나 설치 SQLite 도구를 실행하지 않으며 케이스/조회 상태를 일반 SQL 저장소로 바꾸지 않습니다. [GPU 실행 근거](GPU_EXECUTION_EVIDENCE.md)를 참조하세요.

입력 탐색, 별도 수동 수집 키트, 명시적 설치 도구의 제한 변환은
[입력 안내](INPUT_ASSISTANCE.md)를 참조한다. 오프라인 세션은 미리 변환한 JSON을
사용하며 변환 도구 실행을 조용히 허가하지 않는다.
