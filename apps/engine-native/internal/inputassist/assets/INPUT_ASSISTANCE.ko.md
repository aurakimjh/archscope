# 입력 안내와 수동 수집

Analysis Workspace의 입력 안내에서 선택한 로컬 파일을 먼저 확인한다.
현재 범위에 등록된 일반 importer 후보, 신뢰도·근거 등급, 선언 기능과 제한을
표시하며 분석 결과를 만들지는 않는다. hint나 확장자 일치는 전체 형식/생성 도구
검증이 아니다. 가져오기는 기존 파서/분석기 검증을 다시 수행한다. 개별 기존 분석
화면의 지원표는 유지하며 새로운 생성 도구를 등록하지 않는다. 바이너리 JFR은
변환 안내를, JFR JSON 후보는 일반 importer 대신 개별 JFR 화면 안내를 제공한다.

탐색은 최대 64 MiB 원본을 소유 스냅샷으로 만들고 전체 해시로 바이트 식별을
검사하며 최대 64 KiB만 형식 감지에 사용한다. 한도는 30초·동시 2건이다.
디렉터리 탐색, 인접 파일 자동 읽기, 도구 실행은 하지 않는다. 성공/실패/취소 시
임시 스냅샷을 제거하고 과대/변경 입력은 명시적으로 거부한다.

선택적으로 지정한 수집 sidecar는 2 MiB로 제한하고 기존 캡처 계약을 검증한다.
파일 SHA-256과 십진수 바이트 크기가 선택한 원본과 일치해야 한다. 해당 파일에
연결된 entity ID만 맥락 보완에 사용한다. 일치는 파일 결합을 확인하며 사용자가
선언한 생성 도구·시계·프로세스·워크로드의 진실성을 검증하지 않는다. 생성 도구와
버전, 시작/종료, 시간대, 시계 기준, 호스트, 프로세스 수명 등 모르는 값은 표시한다.
파일 수정 시각을 수집 시각으로 쓰지 않는다. 맥락 부족으로 기본 분석을 막지는
않으며 상관/비교는 각각의 근거 조건을 계속 적용한다.

‘새 수집 초안 저장’은 관측한 파일 해시·크기와 새 산출물 ID를 넣고 수집 식별,
시계, 생성 도구는 unknown으로 남긴 sidecar 골격을 저장한다. 제공된 선언을
병합하거나 덮어쓰지 않는다. 텍스트 편집기에서 명시적 근거를 채우고 기존 케이스
흐름으로 첨부한다. 이식 불가능한 파일명은 `selected-source.bin`으로 대체하고
검토 경고를 남기므로 사용 전 파일명을 확인한다. 인증 정보, 환경 덤프, 개인키,
전체 실행 명령을 sidecar에 넣지 않는다.

## 별도 수동 수집 키트

‘수동 수집 키트 저장’ 또는 `evidence collection-kit --out /new/manual.zip`으로
비실행 EN/KO 매뉴얼, `recipes.json`, 수집 schema를 내보낸다. ZIP은 데이터 전용
팩과 분리하며 실행 수집기를 포함하지 않는다. JDK JFR 수동 수집/변환 명령과 기존
내보내기 확인 항목을 제공한다. 앱은 레시피 명령 실행, 프로세스 자동 연결, 권한
상승, 도구 다운로드를 하지 않는다. 수집 환경에서 권한·대상 PID·저장 경로·기간·
부하를 직접 검토하며 공유 전에 출력의 민감정보를 확인한다.

```sh
archscope-engine evidence discover --in /local/source --capture /local/capture.json
archscope-engine evidence discover --in /local/source --pack-store /local/packs --preset-scope user
archscope-engine evidence collection-kit --out /new/manual.zip
```

`--capture`는 선택 사항이다. CLI 탐색 JSON의 `capture_draft`를 초안으로 쓸 수
있다. 포함된 schema를 사용하며 모르는 맥락을 측정된 0으로 취급하지 않는다.

## 명시적 설치 JDK 변환

수집 환경의 호환 도구로 미리 변환하는 방식을 우선한다. 로컬 바이너리 JFR 변환은
신뢰하는 설치 JDK의 `jfr` 절대 경로와 새 JSON 출력명을 선택해야 한다.
`--version`, `print --json <소유한 원본>` 고정 argv만 실행하며 shell 확장이나
임의 옵션은 허용하지 않는다. 버전 확인 5초, 전체 2분, 입력/출력 각각 64 MiB,
stderr 16 KiB, 동시 2건으로 제한한다. 출력은 비공개 임시 파일로 스트리밍한다.
과다 출력, 잘못된 JSON/이벤트 배열 구조, 비정상 종료, 실행 파일/원본 변경,
취소 시 임시/출력 자료를 제거한다. Unix에서는 프로세스 그룹, Windows에서는
중지 상태로 시작해 Job Object에 결합한 프로세스 트리를 종료한다. 실제 Windows
프로세스 실행은 별도 플랫폼 수용 검증 항목이다.

```sh
archscope-engine jfr convert --in /local/recording.jfr --cli /installed/jdk/bin/jfr --out /new/recording.json
archscope-engine jfr analyze-json --in /new/recording.json --out /new/result.json
```

성공 시 JSON과 `<output>.conversion.json`에 도구/버전, 실행 파일 SHA-256,
고정된 정제 인자, 입출력 해시, 크기, local 위치를 저장한다. 두 출력명은 새 파일이어야
한다. provenance 저장에 실패하면 이번에 만든 JSON을 제거한다. 관리 상태 내부로
내보내지 못하며 외부 stderr와 환경 값은 사용자 진단에 노출하지 않는다. 자식 환경은
Java agent/tool-option 주입 변수를 제외한다. 실행 sandbox나 전체 JDK 설치의
암호학적 적합성 검증은 아니며 소유자가 신뢰하는 설치 도구를 선택한다. 원본 JFR과
provenance를 JSON과 함께 보존한다. 후속 JFR 파서는 이벤트를 별도로 해석/검증하므로
구조적 변환 성공이 전체 이벤트/생성 도구 지원을 의미하지 않는다.

기존 바이너리 JFR 분석도 같은 제한 실행기를 사용한다. 절대 경로
`ARCHSCOPE_JFR_CLI`를 명시해야 하며 PATH/JAVA_HOME 탐색만으로 실행을 허가하지
않는다. 기존 JSON 분석은 유지한다. 도구 없음, 지원하지 않는 버전 출력, 입력 불일치,
시간/출력 초과, 도구 실패는 안정적인 거부 코드와 호환 환경에서의 사전 변환 안내를
제공한다. UI 요청 취소와 CLI 인터럽트를 변환에 전달한다. 네이티브 Go JFR 디코더는
보류한다. 명시적 오프라인 세션은 도구 실행 전에 변환을 계속 거부하므로 미리 변환한
JSON을 가져온다. 수동 키트, 탐색, 내보내기는 오프라인에서 동작한다.

공식 구문: [JDK jcmd](https://docs.oracle.com/en/java/javase/26/docs/specs/man/jcmd.html),
[JDK jfr](https://docs.oracle.com/en/java/javase/21/docs/specs/man/jfr.html).
실제 설치 버전의 help에서 옵션을 확인한다.

기록한 버전은 `jfr --version`의 도구 문자열이며 일부 과거 JDK 도구는 `1.0`을
반환한다. 전체 JDK 배포 버전을 별도로 측정한 값이 아니다. 설치 JDK 도구가
확장자를 요구하므로 소유 입력에 `.jfr` 확장자를 유지한다.

활성 선언형 매핑 후보는 최대 4개의 첫 행 미리보기로 원본 수치 텍스트 → 정규화
수치/단위/종류와 팩 SHA를 표시할 수 있다. 같은 제한 해석기를 사용하지만 전체
가져오기나 생성 도구 적합성 검증은 아니며 감지 한도를 넘는 불완전한 행은 표시하지
않는다. 표시 이스케이프/길이 제한을 적용하면 이를 알린다.
