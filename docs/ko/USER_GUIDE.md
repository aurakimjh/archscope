# ArchScope 사용자 매뉴얼

이 매뉴얼은 Go 엔진과 Wails 네이티브 데스크톱 앱을 사용하는 사람을 위한 안내입니다.
화면 설명은 현재 저장소의 메뉴와 동작을 기준으로 합니다. 운영자·개발자·아키텍트·TA·DBA가
같은 자료를 보면서 **무슨 일이 일어났는지, 무엇이 아직 확인되지 않았는지, 다음에 어떤
자료를 확인해야 하는지** 판단하는 것을 목표로 합니다.

처음 사용하는 경우 1–4장을 읽고 자신의 증상에 맞는 분석 화면으로 이동하세요.
분석 경험이 있어도 [분석 작업 공간](#analysis_workspace)의 저장·비교 절차를 먼저 확인하면
분석한 자료를 다시 찾거나 보고서에 연결하기 쉽습니다. 영어판은 [User Guide](../en/USER_GUIDE.md)입니다.

## 목차

- [1. ArchScope로 하는 일](#purpose)
- [2. 시작하기와 화면 구성](#start)
- [3. 증상에 따라 파일과 분석 선택하기](#choose)
- [4. 모든 분석에서 지켜야 할 읽기 순서](#reading)
- [5. 분석 화면 12개](#analyzers)
  - [프로파일러](#profiler) · [비교](#diff) · [액세스 로그](#access_log) · [GC 로그](#gc_log)
  - [JFR](#jfr) · [예외 분석](#exception) · [쓰레드 덤프](#thread_dump) · [MSA 타임라인](#msa_profile)
  - [Trace Import](#trace_import) · [HTTP capture](#http_capture) · [Browser CPU](#browser_cpu) · [Lighthouse 감사](#browser_audit)
- [6. 분석 작업 공간: 자료를 조사 기록으로 만들기](#analysis_workspace)
- [7. 워크스페이스 도구 7개](#workspace-tools)
  - [Export 센터](#export_center) · [리포트 비교](#report_diff) · [차트 스튜디오](#chart_studio)
  - [장애 타임라인](#incident_timeline) · [SLO / Golden Signals](#slo_golden_signals)
  - [Service Flow](#service_flow) · [Evidence Board](#evidence_board)
- [8. 설정](#settings)
- [9. 상황별 실습](#walkthroughs)
- [10. 문제 해결·용어·완료 확인](#troubleshooting)
- [부록. 빌드·CLI와 심화 문서](#appendix)

<a id="purpose"></a>

## 1. ArchScope로 하는 일

서비스가 느려졌다는 사실만으로 원인을 알 수는 없습니다. 액세스 로그는 **어떤 요청이
느렸는지**, 프로파일은 **어떤 호출 경로에서 실행 또는 대기 비중이 컸는지**, 쓰레드 덤프는
**그 순간 쓰레드가 어디에 있었는지**, 분산 추적은 **서비스 사이에서 요청이 어떻게
이어졌는지**를 보여줍니다. 서로 다른 질문에 답하는 자료를 함께 보아야 조치 대상을 좁힐 수 있습니다.

ArchScope는 수집·내보낸 파일을 로컬에서 분석하고, 분석 결과를 작업 공간에 모아 비교,
시각화, 근거 정리와 보고서 작성을 돕습니다. 일반적인 흐름은 다음과 같습니다.

1. 증상과 조사 범위를 적습니다. 예: “배포 후 주문 조회의 느린 요청이 증가했는가?”
2. 해당 시각과 대상을 포함하는 로그·프로파일·덤프를 준비합니다.
3. 분석 화면에서 파일을 읽고, 파싱 상태와 관측 범위를 먼저 확인합니다.
4. 상위 항목을 좁혀 원본·호출 경로·개별 이벤트를 확인합니다.
5. 작업 공간에 필요한 결과를 모으고 케이스로 저장합니다.
6. 다른 근거로 가설을 확인하고, 비교 가능한 조건의 개선 전후 결과를 비교합니다.
7. 확인된 사실·추정·미확인 사항을 구분해 보고서로 전달합니다.

**분석 결과의 경고는 조사 출발점입니다.** 예를 들어 긴 DB 호출이 발견돼도 실행계획,
잠금, 연결 획득, 네트워크, DB 서버 부하 중 무엇이 원인인지는 추가 자료가 필요합니다.
파일에 기록되지 않은 사건까지 자동으로 복구하거나 원인을 확정하지는 않습니다.

현재 지원 범위는 [입력 형식 지원표](IMPORTER_SUPPORT_MATRIX.md)를 기준으로 확인하세요.
언어 이름이 같아도 생성 도구·버전·내보내기 형식에 따라 지원 여부가 다릅니다.
Java heap dump `.hprof`를 읽는 힙 덤프 분석 엔진이나 소스 코드 자동 수정 기능은 제공하지 않습니다.
프로파일·시스템·GPU 근거의 일반 가져오기는 이미 수집된 자료를 읽는 경로이며,
Windows 실시간 HTTP 캡처는 별도의 명시적 수집 기능입니다.

<a id="start"></a>

## 2. 시작하기와 화면 구성

### 설치·실행과 첫 분석

설치한 ArchScope 데스크톱 앱을 실행합니다. 소스에서 빌드한다면 [부록](#appendix)과
[네이티브 앱 안내](NATIVE_APP.md)를 따르세요. 보관용 구형 Python/FastAPI 앱은 현재 사용 경로가 아닙니다.

첫 연습은 저장소의 `examples/access-logs/sample-nginx-access.log`로 할 수 있습니다.

1. **액세스 로그** 화면에서 파일 선택으로 샘플을 엽니다.
2. 입력 형식을 nginx로 선택하고 분석을 실행합니다.
3. 처리 상태와 파서 경고를 확인한 뒤 요청 수·지연·상태 코드·URL별 표를 봅니다.
4. **분석 작업 공간**으로 이동해 생성된 결과 카드를 확인합니다.
5. **Export 센터**에서 그 결과를 선택하고 HTML 또는 JSON으로 저장합니다.

샘플은 화면을 익히기 위한 자료입니다. 표시된 경고나 수치를 운영 환경의 측정 결과로 사용하지 마세요.

### 메뉴와 공통 조작

앱의 분석 메뉴는 원본을 읽는 도구, 워크스페이스 메뉴는 이미 만든 결과를 재사용하는 도구입니다.
현재 메뉴는 분석 화면 12개, 분석 작업 공간과 후속 도구 8개, 설정 화면 1개로 구성됩니다.
창 폭이나 OS에 따라 메뉴 표현은 달라도 화면 이름을 기준으로 이동할 수 있습니다.

| 화면 요소 | 사용법과 확인할 점 |
| --- | --- |
| 파일 선택·드롭 영역 | 해당 분석기가 받을 로컬 파일을 고릅니다. 복수 파일은 복수 입력을 지원하는 화면에서 선택합니다. |
| 최근 파일 | 경로를 다시 선택하는 편의 기능입니다. 원본이 이동·삭제·변경되면 이전 분석이 복구되는 것은 아닙니다. |
| 분석 옵션 | 형식·시간 범위·상한·모드를 지정합니다. 파싱과 계산에 쓰이는 옵션을 바꾸면 새 분석을 실행해야 합니다. |
| 분석·취소와 처리 상태 | 작업 완료·실패·취소를 구분합니다. 마지막으로 성공한 화면이 보인다는 이유만으로 새 분석이 완료됐다고 판단하지 마세요. |
| 요약 카드 | 현재 범위의 수치입니다. 단위와 분모를 확인하고 상세 표로 내려갑니다. |
| 검색·정렬·필터 | 현재 결과를 좁힙니다. 화면별 필터와 분석 전 옵션은 역할이 다르며, 모든 필터가 통계 분모를 다시 계산하는 것은 아닙니다. |
| 도움말 아이콘 | 카드나 옵션의 의미를 설명합니다. 엔진 경고의 상세 내용도 함께 확인하세요. |
| 결과의 출처·시각 | 입력 파일, 수집 시각, 결과 생성 시각, 작업 공간 등록 시각을 구분합니다. |

분석이 성공하면 여러 개별 분석 화면은 결과를 작업 공간에 자동 등록합니다.
MSA의 선택 범위 결과나 공통 근거 가져오기는 별도의 **Workspace에 추가** 동작을 사용합니다.
분석 후에는 작업 공간 카드의 유형·원본·생성 시각을 확인해 원하는 결과가 들어왔는지 점검하세요.

<a id="choose"></a>

## 3. 증상에 따라 파일과 분석 선택하기

### “무슨 파일을 열어야 할까?” 선택표

| 궁금한 점·증상 | 준비할 자료 | 시작 화면 | 다음에 연결할 자료 |
| --- | --- | --- | --- |
| 어떤 URL이 느리거나 오류가 많은가 | 지원 웹 서버/프록시 액세스 로그 | 액세스 로그 | 같은 요청의 trace·MSA, 같은 시간의 GC·프로파일 |
| CPU가 높거나 특정 코드 경로가 의심됨 | CPU 프로파일·collapsed stack 등 | 프로파일러 | 동일 구간 CPU 메트릭, 변경 전 프로파일 |
| 응답은 느린데 CPU가 낮음 | wall-clock 프로파일, 연속 쓰레드 덤프 | 프로파일러·쓰레드 덤프 | 잠금 소유자, 연결 풀·DB·외부 서비스 근거 |
| JVM이 주기적으로 멈추거나 메모리가 출렁임 | 지원 GC 로그, JFR 이벤트 | GC 로그·JFR | 요청 지연 시각, 할당 프로파일·메모리 메트릭 |
| 예외가 반복되거나 장애 원인 메시지가 필요함 | 예외 로그·stack trace | 예외 분석 | 요청 ID/시각의 액세스 로그·trace |
| 쓰레드가 같은 위치에 머물거나 교착 의심 | 같은 프로세스의 순서가 있는 덤프 여러 개 | 쓰레드 덤프 | lock owner, wall-clock 프로파일 |
| MSA 요청의 서비스·SQL 구간을 나눠 보고 싶음 | Jennifer transaction profile export | MSA 타임라인 | callee 프로파일, DB 자료, 분산 추적 |
| 여러 서비스의 호출 관계·오류 전파를 확인 | 지원 OTel 등 trace export | Trace Import | Service Flow, 관련 HTTP·로그 |
| 클라이언트가 보낸 HTTP 요청·응답을 확인 | HAR 또는 지원 Windows 로컬 캡처 세션 | HTTP capture | 서버 trace·access log, 세션 비교 |
| 브라우저·Node JavaScript 실행이 느림 | Chrome Performance JSON 또는 V8 .cpuprofile | Browser CPU | Lighthouse·HTTP 자료 |
| 페이지 로딩 품질·리소스 낭비가 의심됨 | 이미 생성한 Lighthouse JSON | Lighthouse 감사 | Browser CPU·HTTP, 같은 조건의 재측정 |
| OS 대기·메트릭·추론 서빙·GPU 실행을 조사 | 지원 형식 원본과 수집 조건 | 분석 작업 공간의 근거 가져오기 | 형식별 상세 보기·프로파일 투영·비교 |
| 개선 전후에 실제 변화가 있었는가 | 조건이 맞는 전후 프로파일 또는 저장 결과 | 비교·리포트 비교·작업 공간 비교 | 같은 부하의 재실행과 품질/범위 확인 |

### 분석 전에 기록할 최소 맥락

- 증상, 기대 동작, 관측한 사용자 영향과 조사 시간대.
- 서비스·호스트·프로세스·재시작 여부, 수집 도구·버전과 파일 형식.
- 수집 시작/종료, 시간대, 상대 시각인지 실제 시각인지, 샘플링 주기와 누락 여부.
- 요청량·동시성·데이터 규모·배포 버전 등 비교에 필요한 조건.
- 원본 경로와 보관 위치. 공유본에서는 토큰·개인정보·쿼리·업무 데이터 등 민감정보 제거 여부.

모르는 값은 모름으로 남겨 두세요. 파일 수정 시각을 수집 시각으로 대체하면 시간 상관이 틀릴 수 있습니다.
파일 확장자만 바꾸거나 이종 형식을 하나로 붙여 지원 형식처럼 만들지 마세요.
입력 안내와 수동 수집 키트는 [입력 안내](INPUT_ASSISTANCE.md)에 설명되어 있습니다.

<a id="reading"></a>

## 4. 모든 분석에서 지켜야 할 읽기 순서

### 4.1 파일이 제대로 읽혔는지 먼저 확인

분석 완료 메시지 다음에는 원본 목록·처리 건수·시간 범위·파서 진단을 봅니다.
잘못된 형식, 지원하지 않는 줄, 시간 파싱 실패, 상한 도달, 부분 결과는 결론에 영향을 줍니다.
전체 1시간 중 앞 5분만 읽었다면 “1시간 동안 문제가 없다”는 결론을 내릴 수 없습니다.

`Top N`은 해당 화면의 순위/출력 행 상한일 수 있고, `max lines`는 읽는 입력 범위를 제한합니다.
일부 분석 옵션은 이벤트 선택에도 적용되므로 도움말과 파서 진단을 확인하세요.
표의 행 수를 전체 관측 수로 단정하지 마세요.

### 4.2 값·단위·분모를 함께 읽기

| 표시 | 뜻 | 사용자가 할 일 |
| --- | --- | --- |
| 관측된 0 | 해당 측정 범위에서 실제 0으로 기록됨 | 측정 범위와 단위를 확인 |
| 비어 있음·—·null·unavailable | 값을 얻지 못했거나 계산 조건이 없음 | 이유·수집 옵션·원본을 확인 |
| partial·truncated·상한 도달 | 일부 범위만 읽거나 표시됨 | 빠진 범위를 기록하고 필요하면 입력을 분리해 재분석 |
| pending·처리 중 | 아직 결정되지 않음 | 완료 상태를 확인한 뒤 사용 |
| unsupported·비교 불가 | 현재 형식·기능·조건으로 계산할 수 없음 | 지원 형식으로 내보내거나 조건을 맞춘 새 자료 준비 |

**개수와 시간은 다릅니다.** 샘플 1,000개를 1,000ms로 읽을 수 없으며, 측정 의미와 주기가 필요합니다.
이미 시간 가중치를 가진 프로파일에 주기를 다시 곱하면 잘못된 값이 됩니다.
**비율의 분모도 다릅니다.** 전체 요청의 오류율과 특정 URL의 오류율, 전체 관측과 화면에 보이는
상위 행의 합계를 구분하세요. 필터 후 분모가 유지되는 조회도 있습니다.

p95는 사용한 계산 정책과 표본 안에서 약 95%의 관측이 그 값 이하였다는 뜻입니다.
“요청의 95%가 그 시간만큼 걸렸다”는 뜻이 아닙니다. 평균이 좋아도 소수의 긴 지연이 남을 수 있습니다.
요청 수가 매우 적으면 p99 변화 해석도 불안정합니다.
서로 다른 그룹의 p95를 평균내 전체 p95를 만들지 마세요. 자세한 정책은
[백분위 정책](PERCENTILE_POLICIES.md)을 참고하세요.

### 4.3 관측 → 가설 → 추가 확인 → 조치

예: “주문 조회 p95 증가”는 관측, “DB 잠금 대기 때문”은 가설입니다.
SQL 시간과 동일 구간의 DB 대기/잠금 자료를 확인한 뒤 조치 대상을 결정합니다.
단순히 경고가 많다는 이유로 메모리·쓰레드·연결 풀 크기를 늘리지 마세요.
변경 후에는 같은 조건으로 다시 수집해 지연·오류·처리량과 새로운 부작용을 확인합니다.

### 4.4 샘플 원본에서 분석 질문을 만드는 방법

저장소에 포함된 연습 파일로 원본과 결과를 연결해 볼 수 있습니다.
아래 해석은 샘플의 해당 필드를 설명하며 운영 장애를 판정한 결과가 아닙니다.

| 샘플 | 원본에서 읽는 내용 | ArchScope에서 확인하고 다음에 할 일 |
| --- | --- | --- |
| [nginx 액세스 로그](../../examples/access-logs/sample-nginx-access.log) | 첫 행은 GET, 상태 200, 마지막 응답시간 0.123초입니다. 500인 DELETE 행은 1.500초입니다. | 이 형식에서 각각 123ms·1,500ms로 읽힙니다. 느리고 실패한 DELETE와 정상 GET을 같은 요청 유형으로 섞지 말고 method/URL별로 확인합니다. |
| [Java 쓰레드 덤프](../../examples/thread-dumps/sample-java-thread-dump.txt) | 요청 쓰레드는 OrderService.find에서 BLOCKED이고 특정 monitor를 기다립니다. Housekeeper는 TIMED_WAITING입니다. | 기다리는 lock ID와 stack을 확인합니다. 이 샘플만으로 owner나 교착 cycle을 확정할 수 없으므로 추가 덤프·소유자 근거를 준비합니다. Housekeeper 대기만으로 연결 풀 고장을 판단하지 않습니다. |
| [wall 프로파일](../../examples/profiler/sample-wall.collapsed) | 세미콜론으로 연결한 frame 경로 뒤에 가중치가 있습니다. socketRead 경로의 13282는 그 행의 가중치입니다. | 수집 단위/주기를 확인하기 전에는 13,282ms로 읽지 않습니다. 네트워크 읽기 대기 경로를 찾아 요청·외부 서비스 근거로 조사합니다. |

현재 포함된 nginx 샘플을 분석하면 요청 6건과 오류 2건을 확인할 수 있습니다.
이는 분석기의 샘플 집계이며 “500만 오류”라는 운영 정책을 뜻하지 않습니다.
상태별 표에서 404와 500을 나눠 보고 어떤 상태를 장애로 처리할지는 서비스 맥락으로 판단하세요.

<a id="analyzers"></a>

## 5. 분석 화면

<a id="profiler"></a>

### 5.1 프로파일러

**왜 사용하는가:** 요청이나 작업을 처리하는 동안 어떤 호출 경로의 비중이 컸는지 찾습니다.
CPU 프로파일은 실행 비용을, wall-clock 프로파일은 대기를 포함한 경과 시간 관점을 조사하는 데 유용합니다.
Collapsed 입력의 Profile kind는 wall/cpu/lock을 선택합니다. 할당량 등 다른 측정량은
지원되는 원본과 작업 공간의 metric 선택을 확인하고 수집기가 기록한 의미를 따릅니다.

**입력과 조작**

1. 지원하는 collapsed stack, Jennifer export, flamegraph 파일 등을 선택하고 형식을 맞춥니다.
2. Profile kind와 값 단위를 원본에 맞춥니다. 샘플 주기를 알 때만 interval을 지정합니다.
   elapsed를 직접 입력하는 경우 실제 수집 구간과 근거를 확인합니다.
3. Top N과 메모리 보호 상한을 확인한 뒤 분석합니다.
4. 요약·진단 → 상위 스택 → flamegraph → 경로별 상세 순으로 확인합니다.
5. frame을 눌러 확대하고, include/exclude 또는 regex와 경로 조건으로 범위를 좁힙니다.
   부모 경로를 유지하는 보기와 선택 경로로 다시 시작하는 보기는 맥락 표현이 다릅니다.
6. timeline base가 필요하면 트랜잭션 경계를 대표하는 메소드 패턴을 지정하고 다시 분석합니다.

**결과 읽기:** flamegraph에서 넓은 frame은 해당 경로가 차지하는 가중치가 크다는 뜻입니다.
가로 위치를 사건의 발생 순서로 읽지 마세요. 부모와 자식은 포함 관계이므로 모두 더하면 중복입니다.
Self는 해당 frame 자체, inclusive는 자식을 포함한 비용입니다. 높은 inclusive·낮은 self라면
자식 호출을 먼저 봅니다. wall-clock에서 DB/네트워크 대기가 넓다고 해서 그 frame이 CPU를 많이 쓴 것은 아닙니다.

실행 시간 구성과 timeline의 분류는 관측·분류 규칙을 바탕으로 한 설명입니다.
순서가 없는 집계 stack으로부터 실제 요청 하나의 정확한 실행 순서를 복구한 것으로 해석하지 마세요.
분류된 category는 원인 확정이 아닙니다. 주기·elapsed가 없거나 추정이면 시간도 추정으로 전달합니다.

**다음 확인:** CPU hotspot은 같은 시간의 CPU 부하와 코드 경로를, 대기 hotspot은
연속 덤프·SQL·외부 호출 근거를 확인합니다. 원본 py-spy/Speedscope/pprof의 정밀 선택은
[작업 공간의 프로파일 조회](#profile-query)를 사용하세요.

<a id="diff"></a>

### 5.2 비교: 두 프로파일의 차이

**왜 사용하는가:** 개선 후 어떤 경로의 비중이 줄거나 늘었는지 확인합니다.

1. Baseline에는 변경 전, Target에는 변경 후 프로파일을 선택합니다.
2. 두 수집의 프로파일 종류·단위·부하·샘플링·기간이 비교 가능한지 확인합니다.
3. 샘플 총량이 다를 때 normalize를 사용할지 결정하고 비교합니다.
4. 증가·감소·새로 나타난 경로·사라진 경로를 확인하고 각 원본 프로파일로 돌아가 맥락을 봅니다.

정규화는 총량 차이를 조정해 상대적인 구성 변화를 보기 위한 것입니다.
실제 전체 응답시간이나 CPU 사용량이 줄었다는 증거를 대신하지 않습니다.
예를 들어 경로 A의 비율이 줄어도 전체 실행 시간이 늘었다면 절대 비용은 증가할 수 있습니다.
수집 기간이나 메트릭이 다른 자료를 색깔만 보고 개선으로 판정하지 마세요.

이 화면은 프로파일 입력 비교입니다. 저장된 일반 분석 결과는 [리포트 비교](#report_diff),
원본 재검증과 고정 기준이 필요한 조사는 [케이스 baseline](#case-baseline)을 사용합니다.

<a id="access_log"></a>

### 5.3 액세스 로그

**왜 사용하는가:** 사용자가 겪은 지연과 실패를 URL·상태 코드·시간대로 좁힙니다.
내부 원인을 찾기 전에 어떤 요청 집단이 영향을 받았는지 확인하는 출발점입니다.

1. 지원 서버 로그와 형식을 선택합니다. 응답시간 필드가 실제 존재하는지 확인합니다.
2. 필요한 시작/종료 시각과 max lines를 지정하고 분석합니다.
3. 요청 수·지연 분포·상태 코드 분포를 확인합니다.
4. URL별 표에서 요청 수, 지연, 오류를 함께 봅니다. 소수의 긴 요청과 다수 사용자의 영향을 구분합니다.
5. 오류 timeline과 지연 변화가 겹치는 구간을 찾아 trace·GC·프로파일과 조사 시간을 맞춥니다.

평균만 보면 긴 꼬리 지연을 놓칠 수 있어 p95/p99·최대값을 함께 봅니다.
4xx가 늘었다고 모두 서버 장애는 아니며, 5xx도 애플리케이션·프록시·upstream 중 어느 계층인지 확인해야 합니다.
로그의 측정 지점에 따라 latency에 포함되는 대기·전송 범위가 다릅니다.
응답시간이 없는 로그의 빈 지연 값을 0ms로 기록하지 마세요.

**다음 확인:** 느린 URL의 요청 ID나 같은 시간대 trace를 찾고 SQL·외부 호출·CPU·GC 중
어디에 시간이 쓰였는지 조사합니다. 원본 시간대, 로테이션 누락, URL별 표본 수를 보고서에 남깁니다.

<a id="gc_log"></a>

### 5.4 GC 로그

**왜 사용하는가:** JVM 메모리 회수와 정지가 지연·처리량 저하에 관련되는지 확인합니다.

1. 지원 GC 로그를 선택하고 필요하면 strict·행 상한·Top N을 설정합니다.
2. JVM 정보에서 collector, heap 설정, 기록 범위를 확인합니다.
3. pause timeline에서 긴 정지와 반복 패턴을 찾습니다.
4. heap 전후 변화·할당/승격 지표가 제공되면 함께 보고, GC 종류·원인·개별 이벤트를 확인합니다.
5. 문제 시각의 액세스 로그 지연과 대조하고 관련 결과를 작업 공간에 남깁니다.

Heap이 톱니 모양으로 오르내리는 것은 할당과 회수 과정에서 나타날 수 있습니다.
회수 후 점유가 계속 증가하는지, 높은 점유가 실제 지연과 함께 나타나는지를 봅니다.
로그가 제공하는 GC throughput을 서비스 요청 처리량으로 읽지 마세요.
Full GC나 긴 pause 하나만으로 메모리 누수를 확정할 수 없습니다.
기간이 짧거나 로그가 불완전하면 추세 판단을 유보합니다.

**다음 확인:** 같은 구간 JFR allocation·프로파일, 객체 보유 분석이 필요하면 별도 전문 도구,
native memory가 의심되면 JFR native 이벤트를 검토합니다.
우선 collector·JVM 버전·부하를 확인하고 크기 변경 후 동일 조건으로 재검증합니다.

<a id="jfr"></a>

### 5.5 JFR

**왜 사용하는가:** 같은 recording 안에서 CPU·wall-clock·할당·잠금·GC·예외·I/O 이벤트를
선택해 특정 시각에 어떤 활동이 있었는지 조사합니다.

1. 지원 JFR JSON을 선택합니다. 바이너리 `.jfr`은 호환 JDK에서 미리 변환하거나
   [입력 안내](INPUT_ASSISTANCE.md)의 명시적 설치 JDK 변환 절차를 사용합니다.
2. 조사 목적에 맞는 mode를 고르고 시간창·thread state·Top N을 설정합니다.
3. scope/계약과 diagnostics에서 실제 존재하는 이벤트·필드·누락을 확인합니다.
4. 요약·flamegraph·event breakdown·개별 이벤트를 읽습니다.
5. Wall-clock heatmap이 제공되면 관심 bucket을 선택해 시작 범위를 좁히고,
   시간창을 다시 확인한 뒤 분석합니다.

이벤트가 recording 설정에서 비활성화됐다면 결과가 없는 것은 “문제가 없었다”는 뜻이 아닙니다.
JFR event duration과 CPU sample weight를 같은 시간으로 더하지 마세요.

**Native Memory 모드:** allocation/free 이벤트를 짝지어 recording 끝에 해제되지 않은 후보를 봅니다.
Leak-only는 매칭된 allocation/free를 제외하며 tail 옵션은 끝부분의 짧은 수명 노이즈를 줄입니다.
표의 bytes와 event count를 구분하고 top site의 stack을 확인합니다.
미해제 후보는 recording 범위·이벤트 누락·해제 지연에 영향을 받으므로 누수 확정이 아닙니다.
실제 0건, 모드 처리 중, 입력 실패, 관련 이벤트 미수집을 구분하세요.
이 모드는 Java heap dump 분석을 대신하지 않습니다.

<a id="exception"></a>

### 5.6 예외 분석

**왜 사용하는가:** 반복되는 예외를 묶어 중요한 유형과 공통 호출 위치를 찾습니다.

1. 지원 예외 로그/stack trace를 선택하고 형식과 분석 옵션을 확인합니다.
2. 유형별 수와 signature별 집계를 확인합니다.
3. 관심 이벤트를 열어 메시지·전체 stack·중첩 원인과 원본 맥락을 읽습니다.
4. 검색으로 요청·서비스·예외 패턴을 좁히고 중요한 관측을 근거로 남깁니다.

같은 예외가 로그의 여러 계층에서 중복 기록될 수 있어 발생 수와 실패한 요청 수는 같지 않을 수 있습니다.
Signature는 유사 stack을 묶는 방법이며 같은 업무 원인이라는 보장은 아닙니다.
Root-cause 표시는 파싱한 예외 체인의 원인으로, 시스템 전체의 최종 장애 원인과 구분합니다.

**다음 확인:** 최초 발생 시각, 직전 배포/설정 변경, 요청 ID와 downstream 오류를 확인합니다.
메시지에 포함된 개인정보·토큰을 보고서로 옮기지 않도록 원본과 내보내기를 검토하세요.

<a id="thread_dump"></a>

### 5.7 쓰레드 덤프

**왜 사용하는가:** 멈춤·쓰레드 고갈·경합이 의심될 때 실행 위치와 대기 관계를 확인합니다.
덤프는 특정 순간의 사진이므로 연속 자료가 지속성 판단에 더 유용합니다.

1. Single은 한 시점, Multi는 여러 덤프의 지속 패턴, Locks는 지원 lock 메타데이터의 경합을 조사할 때 사용합니다.
2. 같은 프로세스에서 시간 순서대로 수집한 파일을 고릅니다. 재시작이나 서로 다른 프로세스를 섞지 마세요.
3. 자동 형식 감지가 맞지 않을 때만 format override를 사용합니다.
4. Multi에서는 threshold를 확인하고, 제공되는 상관 맥락도 검토합니다. Threshold는 몇 개 연속 덤프에
   나타나야 지속 패턴으로 볼지 결정하며 실제 지속 초와 같지 않습니다.
5. 상태 분포 → signature 그룹 → 개별 쓰레드 → lock 소유·대기 관계와 deadlock cycle 순서로 봅니다.

WAITING/TIMED_WAITING은 유휴 worker나 정상 대기일 수 있습니다.
RUNNABLE도 덤프 하나만으로 지속적인 CPU 사용을 증명하지 않습니다.
같은 stack이 반복되면 수집 간격·대상 식별이 맞는지 확인하고 실제 진행 중인 반복 작업인지
막힌 상태인지 다른 자료로 확인합니다. Lock 정보가 없는 형식에서는 소유 관계를 추정하지 마세요.

**다음 확인:** 기다리는 쓰레드뿐 아니라 lock owner, 연결 풀, DB 대기, 외부 timeout도 확인합니다.
언어별 형식·식별 한계는 [다중 언어 쓰레드 안내](MULTI_LANGUAGE_THREADS.md)를 참고하세요.

<a id="msa_profile"></a>

### 5.8 MSA 타임라인

**왜 사용하는가:** Jennifer profile export에서 요청의 서비스 호출·SQL·메소드·연결 획득 등
응답시간 구성을 나눠 조사합니다. 긴 구간을 찾은 뒤 담당 서비스나 DB로 조사를 넘기는 데 유용합니다.

1. 관련 profile 파일을 함께 선택하고 파싱합니다. 파일 오류·profile 완성도·누락을 먼저 봅니다.
2. **단일 GUID**는 실제 요청 하나의 호출 구조, **평균 signature**는 유사한 transaction 모양의 집계를 선택합니다.
3. 필요한 TXID 또는 애플리케이션 URL을 선택해 범위를 좁힙니다.
4. 선택 범위 계산이 완료된 후 timeline·응답시간 구성·SQL·외부 호출·네트워크 관련 표를 확인합니다.
5. 현재 선택 아래의 JSON/HTML 저장 또는 Workspace 추가를 사용합니다.
   계산 중·잘못된 선택에서 버튼이 비활성화되면 완료 상태와 선택을 다시 확인합니다.

| 결과 | 읽는 방법 |
| --- | --- |
| 단일 GUID timeline | 실제 관측한 호출과 elapsed를 따라갑니다. 누락 callee나 불완전 profile이 있는지 확인합니다. |
| 평균 signature | 포함된 표본/후보 수와 제외 그룹을 봅니다. 평균 모양을 실제 요청 하나의 순서로 읽지 않습니다. |
| SQL | 호출 수·합계·평균·긴 호출을 함께 봅니다. DB 실행계획이나 잠금 원인을 단독으로 증명하지 않습니다. |
| External_Call과 API | Callee 자신의 시간과 caller가 측정한 외부 호출 elapsed를 구분합니다. 병렬·중첩 구간을 단순 합산하지 않습니다. |
| Network Prep | 외부 호출을 감싸는 wrapper에서 내부 외부 호출 시간을 제외한 잔여 구간입니다. 순수 회선 지연이 아닙니다. |
| 서비스 호출 네트워크 타임 | 매칭된 caller/callee의 timing 차이를 봅니다. clock·매칭 품질·누락의 영향을 확인합니다. |
| 메소드 미분류 응답시간 | 식별된 자식 작업을 빼고 남은 METHOD 구간입니다. 일반적인 느린 메소드 순위나 순수 CPU 시간과 다릅니다. |
| 프로파일 미수집 외부호출 | caller 호출은 있으나 대응 callee profile이 없습니다. 호출 실패 또는 호출이 없었다는 뜻은 아닙니다. |

사용자 정의 메소드 규칙·카드는 알려진 패턴을 분류하는 도구입니다. 규칙 변경 후에는 이전 결과와
분류 조건이 같은지 확인하세요. Method 잔여 합계와 구성 카드가 스코프·겹침·표시 한도 때문에
완전히 같지 않을 수 있습니다. 의미와 예시는 [MSA dispatch 안내](MSA_SERVLET_DISPATCH.md)를 참고하세요.

**다음 확인:** 긴 SQL이면 DB 근거, 긴 외부 호출이면 callee profile/trace,
긴 잔여 메소드면 프로파일러·쓰레드 덤프로 이동합니다. 선택 범위를 보고서에 반드시 적습니다.

<a id="trace_import"></a>

### 5.9 Trace Import

**왜 사용하는가:** 분산 요청의 span·서비스 의존성·지연·오류 경로를 확인합니다.

1. 지원 trace export를 선택하고 형식과 Top N을 확인한 뒤 가져옵니다.
2. 서비스 요약과 dependency를 보고 관심 trace를 찾습니다.
3. trace·span 표에서 parent 관계, 서비스, duration, 오류를 확인합니다.
4. Critical path 표에서는 경로 의미와 duration 가용성을 확인합니다.
5. 작업 공간의 Service Flow와 장애 타임라인에서 다른 결과와 함께 봅니다.

병렬 span의 duration을 모두 더하면 요청의 실제 경과시간보다 커질 수 있습니다.
Critical path의 관측된 span chain 추정값은 전체 분산 시스템의 정확한 실행 시간 보증이 아닙니다.
끊긴 parent, 샘플링, 누락 서비스, clock 차이로 호출 구조가 불완전할 수 있습니다.
같은 시각에 나타났다는 이유만으로 다른 로그를 같은 요청에 연결하지 마세요.

**다음 확인:** 느린 trace의 서비스·endpoint·명시적 ID를 따라 access log·HTTP·DB 근거를 찾습니다.
오류가 caller에서 처음 보인 지점과 실제 실패를 기록한 downstream을 구분합니다.

<a id="http_capture"></a>

### 5.10 HTTP capture

**왜 사용하는가:** 클라이언트 관측 지점에서 요청·응답·상태·elapsed·헤더 등의 근거를 확인합니다.
서버 내부 SQL/CPU 시간을 직접 보여주는 자료와는 관측 지점이 다릅니다.

#### HAR 가져오기와 분석

1. 지원 도구에서 내보낸 HAR 1.2를 선택합니다. 원본의 인증/업무 데이터와 body 포함 여부를 확인합니다.
2. 가져오기 후 summary·fidelity·capture mode·진단을 먼저 읽습니다.
3. 요청 목록의 검색/필터·정렬·그룹을 이용해 method·host·path·상태·지연을 좁힙니다.
4. 개별 transaction 상세에서 URL/query·요청/응답 헤더·timing·가용한 body와 미수집 사유를 확인합니다.
5. 중요한 행에 별표·주석을 남길 때는 필요한 저장/내보내기 동작까지 완료하고 다시 열어 확인합니다.

**저장 / 내보내기**에서는 먼저 형식과 범위를 고릅니다.
ArchScope session으로 보관하거나 지원 HAR·텍스트·HTML 등으로 전달할 수 있습니다.
**저장된 전체 요청**과 **현재 표시된 행**을 구분한 뒤 **저장…**을 누르세요.
표시 범위를 저장한 파일을 전체 세션으로 오인하지 않도록 파일명과 보고서에 범위를 남깁니다.
다시 조사할 세션은 **세션 열기**로 열고 주석·선택 범위·누락 필드를 확인합니다.

가져오기 시점 리댁션이 있어도 모든 업무 비밀을 탐지하는 것은 아닙니다.
보존된 이미지 픽셀은 텍스트 마스킹과 별개이므로 공유 전에 이미지 내용도 확인하세요.
누락·생략된 body는 빈 body와 다릅니다. HAR 도구가 기록하지 않은 필드는 복구되지 않습니다.
상세 기능과 프로토콜 제한은 [HTTP 캡처 안내](SYSTEM_HTTP_CAPTURE.md)를 기준으로 확인하세요.

#### Windows 실시간 캡처

실시간 기능은 지원 Windows 환경에서 사용합니다. macOS에서 파일 분석이 가능하다는 사실과
Windows 수집 기능을 구분하세요.

1. 화면의 최초 사용 프록시/CA 안내를 읽고 수집 대상을 결정합니다.
2. HTTPS 가로채기가 필요하면 제공되는 임시 CA 설치 절차를 따릅니다.
3. 캡처를 시작하고 의도한 테스트 클라이언트가 화면의 loopback proxy를 사용하도록 설정합니다.
4. 대상 요청만 실행하고 live 목록·상태·귀속·drop/저장 상태를 확인합니다.
5. 중지 후 finalized 상태를 확인하고 저장된 세션을 분석합니다.
6. 외부 클라이언트에 따로 설정한 프록시·신뢰 설정은 사용 후 원래대로 정리합니다.

기본은 body 생략입니다. Body opt-in은 지원/사전 조건이 충족될 때만 나타나는 세션별 옵션입니다.
Windows 신뢰 저장소와 JVM/NSS 등의 별도 신뢰 저장소는 다를 수 있습니다.
미귀속 metadata 보존은 필요한 경우에만 명시적으로 선택하며 기본은 drop입니다.

Live 화면의 최신 500행은 전체 저장 내역이 아닙니다. 장시간 캡처 결과는 중지 후 저장 세션과
counter·누락·drop을 함께 확인하세요. Clear 같은 화면 조작 후에도 무엇이 표시되고 무엇이
보존되는지는 현재 세션 상태와 안내를 확인합니다.
`confirmed` 귀속은 특정 connection의 endpoint/PID 근거이며 전체 트래픽 수집 보증이 아닙니다.
`pending`·`unsupported`·passthrough·not captured를 성공한 의미 분석과 구분하세요.
앱이나 프로토콜의 프록시 우회·인증서 고정·QUIC 등은 관측 범위를 제한할 수 있습니다.

#### 세션 비교·상관·요청 작성

전후 세션은 작업 공간의 HTTP 비교에서 기준/대상을 지정합니다.
상관 패널은 지원하는 HTTP와 다른 근거를 선택하고 시계 정렬의 허용/거절 이유를 확인합니다.
시간적으로 겹친다는 것은 인과관계가 아닙니다.

요청 작성/전송 기능을 사용할 때는 method·대상·헤더·body를 확인한 뒤 명시적으로 전송합니다.
특히 변경을 일으키는 요청은 실행할 환경과 권한을 확인하세요. 저장된 요청을 여는 것과 전송은
별개 동작입니다. 재현 결과도 클라이언트·인증·네트워크 조건이 달라지면 원래 요청과 달라질 수 있습니다.

<a id="browser_cpu"></a>

### 5.11 Browser CPU

**왜 사용하는가:** Chrome Performance trace와 V8 `.cpuprofile`에서 JavaScript 실행의
sampled CPU 경로와 긴 관측 run을 찾습니다. Node `--cpu-prof`나 CDP export도 지원 형식에 맞아야 합니다.

1. 지원 JSON 또는 지원 gzip 파일을 선택해 분석합니다.
2. 원본 유형·처리 범위·partial 경고를 확인합니다.
3. flamegraph에서 넓은 경로를 찾고 top frames의 self/inclusive를 비교합니다.
4. sampled run 표와 stack을 확인해 긴 관측 경로를 좁힙니다.
5. 같은 동작의 HTTP와 Lighthouse 자료를 함께 조사합니다.

가중치의 단위와 출처를 보세요. V8 시간 가중치를 JVM 샘플 수와 직접 비교하거나 주기를 다시 곱하지 마세요.
Self는 직접 실행된 비용, inclusive는 자식 포함 비용입니다. 이 화면은 CPU sample 분석이며
네트워크·레이아웃·페인트를 완전히 귀속하는 전체 DevTools Performance 화면을 대신하지 않습니다.
Sampled run을 브라우저의 모든 long task를 빠짐없이 기록한 결과로 읽지 마세요.

자세한 형식·측정 범위는 [Chrome/V8 CPU 안내](CHROME_DEVTOOLS_CPUPROFILE.md)를 참고하세요.

<a id="browser_audit"></a>

### 5.12 Lighthouse 감사

**왜 사용하는가:** 이미 생성한 Lighthouse report에서 로딩 지표·audit·리소스 개선 후보를 검토합니다.

1. 로컬 Lighthouse JSON을 선택해 가져옵니다. 앱에서 URL을 입력해 감사를 자동 실행하는 흐름이 아닙니다.
2. 원본 score와 보고서 조건을 확인합니다.
3. Core Web Vitals 관련 지표, findings, audit 목록을 차례로 봅니다.
4. network/resource 표로 큰 리소스·요청 비용을 확인하고 Browser CPU/HTTP 자료와 연결합니다.
5. 제공되는 JSON/HTML 내보내기로 검토 결과를 보관합니다.

단일 실험의 score를 모든 실제 사용자의 성능이나 운영 SLO 충족으로 해석하지 마세요.
기기·네트워크·throttling·cache·Lighthouse 버전이 달라지면 값이 달라질 수 있습니다.
개선 후에는 같은 조건으로 재측정하고, 브라우저 실행과 서버 응답 중 어느 부분이 바뀌었는지 확인합니다.

<a id="analysis_workspace"></a>

## 6. 분석 작업 공간: 자료를 조사 기록으로 만들기

### 6.1 이 화면을 사용하는 이유

한 번의 장애 조사에는 access log, GC, trace, 프로파일 등 여러 결과가 필요합니다.
**분석 작업 공간(Analysis Workspace)**은 결과를 모아 선택하고, 재사용 가능한 케이스로 저장하며,
차트·비교·근거 카드·보고서로 연결하는 중심 화면입니다.

| 개념 | 뜻 | 언제 쓰는가 |
| --- | --- | --- |
| 원본 파일 | 수집·내보낸 로그, 프로파일, 덤프 | 다시 파싱하거나 원본 근거를 확인할 때 |
| 분석 결과 | 특정 원본·옵션으로 만든 계산 결과 | 요약·표·관측을 재사용할 때 |
| 작업 공간 카드 | 현재 앱에서 선택·재사용하는 결과 항목 | 도구 간 결과를 넘길 때 |
| 저장된 케이스 | 명시적으로 저장한 결과와 원본 참조·조사 맥락 | 앱 재시작 후 복구하거나 조사를 이어갈 때 |
| 고정 baseline | 특정 저장 결과·원본·조건에 연결된 불변 비교 기준 | 변경 후에도 같은 기준으로 검증할 때 |
| 근거 카드 | 보고서에 남길 관측·행·그림과 출처 | 중요한 사실을 추려 전달할 때 |

**화면에 카드가 보이는 것과 케이스 저장은 다릅니다.** 필요한 결과는 케이스나 JSON으로 명시적으로
저장하세요. 원본 참조를 붙였다고 외부 원본 파일 전체가 자동 백업되었다고 생각하지 마세요.
원본을 이동하거나 변경하면 검색·원본 좌표 조회·재분석 검증이 실패할 수 있습니다.

### 6.2 결과 모으기·선택·정리

1. 개별 분석 후 생성된 카드를 확인하거나 아래 공통 가져오기로 새 결과를 만듭니다.
2. 카드에서 사용/선택 동작으로 활성 결과를 정합니다.
3. 유형·파일·등록 시각·요약을 확인하고 Export 센터 등 후속 도구에서 같은 결과가 선택됐는지 봅니다.
4. 중요한 결과는 먼저 저장하고, 필요 없는 카드를 제거합니다.
5. 전체 비우기는 현재 결과 목록을 정리하는 동작입니다. 필요한 결과의 보관을 확인한 뒤 사용합니다.

현재 작업 공간 목록은 최근 80개 항목으로 제한됩니다. 많은 파일을 반복 분석할 때는
오래된 카드가 목록에서 빠질 수 있으므로 중요한 결과를 먼저 케이스/JSON으로 저장하세요.

일부 도구는 **선택한 결과 하나**, 장애 타임라인·Service Flow 같은 도구는 **현재 작업 공간의 여러
결과**를 사용합니다. 하나를 활성화했다고 나머지가 자동 제외되지는 않습니다.
기준/대상, 다른 사건, 재분석 결과가 한 공간에 섞이면 조사 의도를 다시 확인하세요.

### 6.3 근거 가져오기 및 조회: 일반 파일·정밀 결과

1. **근거 가져오기 및 조회**에서 로컬 경로를 입력하거나 **파일 선택**을 누릅니다.
2. **형식 감지**와 입력 안내로 후보·지원 조건을 확인합니다. 모호하면 올바른 importer를 직접 고릅니다.
3. 원본 지원 패널이 있는 형식은 **원본 확인 → profile/metric·수집 조건 선택 → 새 모델 저장**을
   먼저 합니다. 패널이 정한 가져오기 형식과 옵션을 확인하세요.
4. **가져오기 옵션**의 JSON과 부분 결과 허용 여부를 검토한 뒤 **분석**을 누릅니다.
   형식이 요구하는 옵션을 사용하고 원본·정규 모델·분석 결과 JSON을 혼동하지 마세요.
5. 후보 선택을 요구하면 형식을 선택하고 완료/부분/실패/취소 상태와 진단을 확인합니다.
6. **결과 보기**에서 제공되는 summary·표·timeline·비교 등 지원 보기를 고릅니다.
   이전/다음으로 페이지를 넘기고 표시 제한·총 행 수·가용성 사유를 확인합니다.
7. **Workspace에 추가**로 결과를 등록하고 **결과 JSON 저장** 또는 케이스 저장으로 보관합니다.

이미 저장한 AnalysisResult JSON은 **저장 결과 열기**로 엽니다.
새 원본 분석과 저장 결과 열기는 다릅니다. 저장된 결과를 열었다고 원본을 재분석하지 않습니다.
오래된 결과는 기본 읽기 전용 보기만 가능할 수 있습니다.
만료/오래된 결과 참조 오류가 나오면 저장 JSON이나 케이스 스냅샷을 다시 열어 새 참조를 얻습니다.

정밀 결과는 큰 정수·소수의 의미를 유지하는 백엔드 경로를 사용합니다.
화면에서 짧게 표시한 값이나 복사한 요약만을 유일한 기록으로 쓰지 말고 원본 JSON을 보관하세요.
알 수 없는 버전·지원하지 않는 보기는 오류나 비활성 사유를 먼저 확인합니다.

<a id="profile-query"></a>

### 6.4 공통 프로파일 drilldown과 근거

프로파일 형식마다 화면이 달라도 조사 질문은 같습니다.
“어느 frame의 자체 비용이 큰가?”, “누가 이 함수를 호출했는가?”, “어떤 원본 관측이 뒷받침하는가?”입니다.

1. 원본 py-spy/Speedscope/pprof 패널에서 확인 후 **profile과 metric을 명시적으로 선택**합니다.
   worker·thread가 여러 개면 첫 항목이 전체를 대표한다고 가정하지 마세요.
2. 분석 결과에서 **공통 프로파일 drilldown과 근거**를 엽니다.
3. Top의 self/inclusive 정렬로 직접 비용과 자식 포함 비용을 나누어 봅니다.
4. Tree/flamegraph에서 경로를 펼치거나 focus합니다.
5. 관심 frame ID로 callers/callees를 조회해 호출 주체와 비용이 전달되는 대상을 봅니다.
6. Evidence에서 관측·원본 참조를 확인하고, 필요한 경우 순서 있는 frame-ID 결합 필터를 사용합니다.
7. 선택을 후속 도구에 넘기려면 **선택을 새 결과로 추가**합니다. 기존 결과는 그대로 남습니다.

문자열 검색과 표시 필터는 선택 전체 분모를 바꾸지 않습니다.
선택 합계·metric·unit과 현재 행의 비율을 함께 읽으세요. Evidence 시간창은 선언된 clock의
시작 시각 기준 `[from,to)`이며 구간을 임의로 잘라 비용을 재배분하지 않습니다.
시각이 없는 관측을 억지로 timeline에 배치하지 않습니다.

Self가 작고 inclusive가 큰 메소드는 비싼 자식 호출의 진입점일 수 있습니다.
넓은 frame의 이름만 보고 그 메소드 자체를 최적화 대상으로 정하지 말고
callee와 관측 경로로 내려가세요. 정밀 원본의 수집·단위별 의미는
[원본 프로파일 가져오기](PROFILE_SOURCE_IMPORT.md), 조회는
[케이스 근거 조회](CASE_EVIDENCE_QUERIES.md)를 참고하세요.

### 6.5 같은 작업 공간에서 다루는 추가 근거

다음 항목은 별도 최상위 메뉴가 아니라 **근거 가져오기** 안의 원본 지원·결과 상세 패널입니다.
표의 문서는 실제 지원 producer/버전·수집 조건과 제한을 설명합니다.

| 근거와 문서 | 화면에서 하는 일 | 결과를 판단할 때 주의할 점 |
| --- | --- | --- |
| [메트릭 이력](METRIC_HISTORY.md) | 원본 확인·시계열 선택, 시간창 구간 조회, 새 선택 결과와 CSV 저장 | gauge·counter·rate 구분. 첫 counter 이전 값을 0으로 만들지 않음. reset/gap/결측을 확인 |
| [Apple Silicon·Ollama](APPLE_SILICON_EVIDENCE.md) | 지원 수동 캡처를 가져와 OS 메모리·서빙 관측 확인 | swap 사용·메모리 압력은 실제 관측으로 판단. 비어 있음을 정상 0으로 바꾸지 않음 |
| [시스템 perf·제한된 BPF](SYSTEM_EVIDENCE.md) | events/intervals/histograms 조회, 지원 metric을 프로파일로 생성 | on/off-CPU·run queue·I/O의 의미가 다름. off-CPU를 lock 대기로 단정하지 않음 |
| [GPU·추론 서빙](INFERENCE_EVIDENCE.md) | 장치/서빙 지표·signal·연결 상태 검토 | GPU 사용률과 요청 지연은 다른 측정. 미연결 series·가용성·실제 producer 검증 범위 확인 |
| [LLM trace](LLM_TRACE_EVIDENCE.md) | trace·span·요청 및 token 관련 근거 확인 | TTFT·전체 지연·요청별/전체 throughput의 관측 지점과 모집단 구분 |
| [런타임](RUNTIME_EVIDENCE.md) | 지원 runtime evidence와 metric 연결, 상태·원본 확인 | 선언된 프로세스·수명·시계를 확인. 임의 텍스트를 지원 덤프로 간주하지 않음 |
| [benchmark·crash 실행 근거](EXECUTION_EVIDENCE.md) | 지원 benchmark 요약/JTL·hs_err 자료의 측정·비교·관측 확인 | 성공 요청과 전체 요청, 평균과 percentile, benchmark와 crash를 구분. crash 로그는 core/heap 분석이 아님 |
| [CPU/GPU 실행 구간](GPU_EXECUTION_EVIDENCE.md) | 지원 PyTorch/Nsight 원본의 CPU operator·kernel·memcpy·연결/겹침 확인 | CPU 관측을 CUDA 실행으로 추정하지 않음. 요약 CSV로 상세 timeline을 복원하지 않음 |
| [DB·OS 스냅샷](SYSTEM_DB_SNAPSHOTS.md) | 지원 수동 스냅샷을 가져와 구간·시계열 확인 | 한 시점 상태와 장시간 추세를 구분. 다른 adapter는 실제 지원표 확인 |

메트릭 이력의 시간창은 **구간 종료 시각** 기준 `[from_ns,to_ns)`입니다.
시작이 창 밖인 구간도 전체를 유지하므로 프로파일 evidence 시간창과 선택 기준이 다릅니다.
서로 다른 단위·resource·속성·관측 지점을 자동 합산하지 마세요.
Histogram bucket이나 이미 계산된 percentile만으로 임의의 새로운 전체 p95를 만들지 않습니다.

### 6.6 케이스 저장·다시 열기·검색

빠르게 현재 결과를 보관하려면 상단 **저장된 케이스** 메뉴에서 **선택한 결과 저장**을 누릅니다.
다시 사용할 때 **케이스 열기 → 케이스 → 저장된 결과**를 선택해 스냅샷을 엽니다.
해당 저장은 백엔드 결과가 있는 항목에서 지원됩니다.

원본 연결과 검색이 필요하면 **케이스 검색과 재사용 baseline** 패널을 사용합니다.

1. **케이스 생성** 또는 **케이스 목록**에서 조사할 케이스를 고릅니다.
2. **현재 리비전 조회**로 최신 상태를 읽습니다.
3. **원본 참조 추가**로 읽기 전용 외부 원본을 연결합니다.
4. 원본을 선택한 뒤 **현재 결과와 원본 연결 저장**을 실행합니다.
5. 케이스의 원본/결과 범위와 검색 필드를 지정하고 문자열·ID·digest를 검색합니다.
6. 결과의 원본 좌표를 열어 실제 근거를 확인합니다. **정밀 스냅샷 열기**는 저장 결과를 복구합니다.

검색은 색인된 범위만 대상으로 하며 prefix·snippet·예산 한도가 있습니다.
검색 결과가 없다고 원본 전체에 그 문자열이 없다는 뜻은 아닙니다.
Typed 시간창 검색은 결과 하나와 선언 clock/단위를 선택하며 시각 없는 행 수를 확인합니다.
임의 로그의 시각이나 형식별 구조 좌표를 만들어 내지 않습니다.

외부 원본이 바뀌었거나 변경 요청을 취소한 뒤에는 케이스를 다시 조회하세요.
취소 직전 저장이 이미 끝났을 수 있습니다. 오래된 리비전으로 계속 저장하거나 hash를 고쳐
오류를 우회하지 마세요. 캐시 정리는 화면이 소유하지 않는 임시 결과를 정리하는 별도 동작이며
필요한 결과는 먼저 저장/내보내기합니다.

<a id="case-baseline"></a>

### 6.7 원본을 검증한 baseline 고정과 전후 비교

**왜 단순 전후 비교와 다른가:** 어떤 결과를 기준으로 썼는지, 원본이 바뀌지 않았는지,
동일한 선택·측정 조건인지를 재확인하며 비교를 반복하기 위해서입니다.

1. 지원되는 정밀 원본 프로파일 결과와 그 원본을 케이스에 연결 저장합니다.
2. 저장 결과·원본을 고르고 **원본 재분석 후 새 baseline 고정**을 누릅니다.
3. 변경 후 자료도 원본과 연결 저장합니다.
4. 고정 baseline과 target 결과/원본을 선택합니다.
5. 필요한 경우에만 전체 entity 매핑과 근거를 명시합니다. 의미가 다른 항목을 강제로 같게 만들지 마세요.
6. **원본 검증 후 비교**를 실행하고 허용/거절 이유·단위·선택 범위·delta를 확인합니다.

Baseline은 후속 분석을 자동으로 따라가지 않습니다. 원본을 덮어쓰지 않고 새 결과와 새 pin을 만듭니다.
원본 누락/변경, 측정·선택·clock·조건 차이는 비교를 막을 수 있습니다.
`declared` 식별은 수집자가 선언한 맥락이며, `verified_by_reimport`는 해당 원본 바이트와
옵션을 재분석한 모델의 일치 검증입니다. 수집 환경이나 도구의 진위를 인증하는 표시는 아닙니다.
저장 결과를 열기만 한 snapshot-only 케이스는 원본 재분석 검증을 대신하지 않습니다.

비교가 거절되면 사유에 맞춰 원본 보관·올바른 선택·동일 조건의 재수집을 해결합니다.
서로 다른 단위를 라벨만 바꾸거나 미상 clock을 임의로 같게 지정하지 마세요.
정확한 지원 범위와 매핑 조건은 [케이스 근거 조회](CASE_EVIDENCE_QUERIES.md)에 있습니다.

### 6.8 HTTP 비교·상관과 AI 해석

HTTP 결과가 있으면 세션 비교와 상관 패널이 조건에 따라 나타납니다.
기준과 대상을 별도로 지정하고, 비교/상관의 지원 후보와 제외 이유를 확인하세요.
상관은 가능한 시간 정렬과 관측 겹침을 보여주며 장애 원인을 자동 확정하지 않습니다.

AI 해석은 선택적인 로컬 기능입니다. 해석 payload가 있을 때 provider·model·prompt version·
비활성 상태·finding 수·gate를 확인합니다. 근거 참조를 검토한 뒤 gate를 통과한 finding만
Evidence Board나 Report Pack에 사용합니다.
AI가 만든 설명과 결정적 분석 결과를 구분하세요. AI를 사용할 수 없어도 파일 분석과 내보내기는 계속됩니다.
로컬 모델 설치와 사용 조건은 [AI 해석 안내](AI_INTERPRETATION.md)를 따릅니다.

<a id="workspace-tools"></a>

## 7. 워크스페이스 도구

<a id="export_center"></a>

### 7.1 Export 센터

**왜 사용하는가:** 선택한 결과를 다른 사람에게 전달하거나 다음 조사에 재사용할 파일로 보관합니다.

1. 작업 공간 결과 목록에서 정확한 항목을 고릅니다.
2. 미리보기의 유형·원본·요약을 확인합니다.
3. 지원되는 HTML·PowerPoint·CSV·CSV 디렉터리·JSON 중 목적에 맞는 형식을 고릅니다.
4. 출력 위치를 선택하고 실행한 뒤 완료 경로와 파일 내용을 확인합니다.

| 목적 | 권장 산출물 |
| --- | --- |
| 정밀 결과 재사용 | 원본 JSON. 프로파일 가중치와 큰 정수/소수를 보존하는 경로 사용 |
| 사람이 읽는 보고서 | 지원 HTML 또는 PowerPoint |
| 스프레드시트에서 검토 | 지원 CSV. 어떤 표/요약을 내보내는지 확인 |
| 슬라이드에 선택 그림·표 삽입 | 결과 상세의 보고서용 이미지·표 또는 차트 스튜디오 |
| 여러 근거와 맥락 전달 | Evidence Board의 Report Pack |

모든 결과가 모든 형식을 지원하지 않습니다. 백엔드 정밀 결과에서는 PowerPoint와 CSV 디렉터리
선택이 비활성화되며 HTML/CSV도 해당 결과 형식의 지원이 필요합니다.
CSV가 원본의 모든 이벤트·필드를 보존한다고 가정하지 마세요.
필요하면 JSON과 사람이 읽는 보고서를 함께 보관합니다.
원본 파일·헤더·URL·주석 등 공유에 포함될 수 있는 값을 검토하세요.

<a id="report_diff"></a>

### 7.2 리포트 비교

**왜 사용하는가:** 저장한 두 AnalysisResult JSON의 요약 수치와 finding 변화를 확인합니다.

1. Before JSON과 After JSON을 각각 선택하고 알아보기 쉬운 라벨을 넣습니다.
2. 분석을 실행하고 유형·변경된 metric·finding 변화 수를 봅니다.
3. 값의 단위·조사 범위·표본 수·옵션이 맞는지 원본 결과에서 확인합니다.

이 화면의 수치 차이는 동일 조건의 실험이라는 보증이 아닙니다.
경고가 사라진 이유가 개선인지, 수집 누락·범위 축소·판정 조건 변경인지 구분하세요.
원본 검증과 재사용 가능한 비교 기준이 필요하면 케이스 baseline을 사용합니다.

<a id="chart_studio"></a>

### 7.3 차트 스튜디오

**왜 사용하는가:** 호환되는 작업 공간 결과에서 설명에 필요한 차트를 구성해 재사용합니다.

1. 결과를 선택하고 사용할 수 있는 차트 template을 고릅니다.
2. Canvas/SVG renderer를 선택하고 미리보기를 확인합니다.
3. 제목·범례·축·단위·데이터 범위를 점검한 뒤 PNG·SVG·CSV 저장 기능을 사용합니다.

호환 template이 없으면 입력 결과의 유형과 제공 chart를 확인합니다.
Renderer 변경은 그리는 방식의 변경이며 원본 통계를 바꾸지 않습니다.

결과 상세의 **보고서용 이미지·표**에서는 열 선택, 제목·크기·글자·행 수·주석을 정하고
**선택 고정 후 이미지 생성**을 누를 수 있습니다.
그 뒤 이미지 복사, 선택한 표 전체 복사, 이미지/출처 묶음 저장, Evidence Board 보관을 사용합니다.
그림은 고정한 선택의 스냅샷이므로 필터·원본을 바꿨다면 새로 생성하세요.
표가 여러 이미지로 나뉘면 모든 페이지와 출처를 보존합니다.
실제 렌더/클립보드에는 네이티브 데스크톱 세션이 필요합니다.
[시각 자료 안내](VISUAL_ARTIFACTS.md)에서 지원 범위와 표 텍스트 보호를 확인할 수 있습니다.

<a id="incident_timeline"></a>

### 7.4 장애 타임라인

**왜 사용하는가:** 작업 공간의 여러 결과를 사건 시간순으로 정리해 “무엇이 먼저 보였는가”를 확인합니다.

1. 같은 사건의 관련 결과를 작업 공간에 모읍니다.
2. 이벤트·그룹·출처 수와 부분/생략 상태를 확인합니다.
3. 검색, analyzer, severity, group으로 관심 범위를 좁힙니다.
4. Narrative와 개별 이벤트의 시각·출처·correlation·evidence reference를 읽습니다.
5. 보고서에 쓸 이벤트를 Evidence Board에 추가합니다.

시각이 없거나 서로 맞지 않는 관측은 정확한 순서를 보장하지 않습니다.
같은 시각의 GC 경고와 오류 증가는 상관 후보이며 선후·인과를 확정하는 증거는 아닙니다.
원본 시간대·clock과 누락 범위를 함께 기록하세요. 필터로 숨긴 행이 원본에서 삭제되는 것은 아닙니다.

<a id="slo_golden_signals"></a>

### 7.5 SLO / Golden Signals

**왜 사용하는가:** 지연(latency)·트래픽(traffic)·오류(errors)·포화(saturation)와
가용한 SLI를 모아 사용자 영향과 목표 위반을 검토합니다.

1. 관련 결과를 작업 공간에 모으고 signal inventory에서 어떤 source가 어느 신호를 제공하는지 봅니다.
2. SLI의 값·단위·대상·관측 범위를 확인합니다.
3. 제공된 목표와 비교한 violations에서 actual·threshold·scope·budget 등을 확인합니다.
4. Signal kind·severity·검색으로 범위를 좁히고 affected scope를 확인합니다.
5. 중요한 위반을 근거 카드로 보관하고 원본 요청/이벤트로 돌아갑니다.

위반이 없다는 것이 모든 SLO를 충족했다는 뜻은 아닙니다.
목표가 없거나 평가 불가능한 항목 수를 확인하세요.
요청 개수·비율·기간이 다른 자료를 섞거나, 단기 실험의 p95 하나로 월간 SLO를 판정하지 마세요.
Report Pack은 현재 작업 공간의 원본에서 파생 측정값을 다시 계산합니다.
저장된 SLO 결과의 유효한 목표 설정과 과거 측정값/위반 재사용은 구분됩니다.

<a id="service_flow"></a>

### 7.6 Service Flow

**왜 사용하는가:** trace와 MSA 등 작업 공간 근거를 caller→callee 관계로 묶어
느린 연결·오류·매칭되지 않은 호출을 검토합니다.

1. 같은 조사 대상의 trace/MSA 결과를 준비합니다.
2. edge·call·error·unmatched 요약을 확인합니다.
3. caller/callee별 count·평균/최장 latency·network gap·source를 읽습니다.
4. 검색과 severity로 범위를 좁히고 finding과 edge를 근거 카드에 추가합니다.
5. 필요하면 Mermaid 또는 JSON으로 관계를 내보냅니다.

서비스 그림은 가져온 근거에서 보이는 호출 관계입니다. 전체 시스템의 최신 구성도와 같지 않을 수 있습니다.
Unmatched는 callee 자료가 없거나 매칭되지 않았다는 뜻이며 호출 실패 확정이 아닙니다.
Network gap을 순수 네트워크 전송 시간으로 단정하지 말고 양쪽 계측·겹침·매칭 품질을 확인하세요.
서로 다른 수집의 중복 요청을 실제 호출 증가로 해석하지 않도록 source를 살펴봅니다.

<a id="evidence_board"></a>

### 7.7 Evidence Board

**왜 사용하는가:** 모든 표를 복사하는 대신 결론을 뒷받침할 관측과 출처를 선별합니다.

1. 작업 공간 카드, timeline·SLO·Service Flow의 행, 지원 그림/표에서 **근거 추가**를 사용합니다.
2. Board에서 제목·요약·severity·analyzer·source reference를 확인합니다.
3. 같은 관측의 중복과 조사 범위를 검토하고 필요한 카드만 남깁니다.
4. 카드 HTML/JSON 또는 Report Pack HTML/ZIP 중 필요한 내보내기를 선택합니다.
5. 산출물을 열어 근거·단위·그림·출처와 빠진 항목을 확인합니다.

좋은 근거는 “DB가 문제”보다 “해당 범위에서 SQL 지연이 증가했고, 이 행/시각이 근거이며,
DB 잠금 여부는 아직 미확인”처럼 사실과 가설을 나눕니다.
요약만 담은 카드는 원본 전체의 대체물이 아니므로 필요한 JSON과 원본도 별도로 보관하세요.
Board 비우기 전에 산출물이 저장됐는지 확인합니다. AI 카드는 gate와 명시적 근거를 확인합니다.

<a id="settings"></a>

## 8. 설정

언어, 밝게/어둡게/시스템 테마, 기본 sample interval·Top N·profile kind,
최근 파일과 앱 정보를 관리합니다.

1. 기본 분석 값을 실제 자주 쓰는 수집 조건에 맞추고 저장합니다.
2. 새 분석에서 옵션이 어떻게 초기화됐는지 다시 확인합니다.
3. 최근 파일 초기화는 최근 경로 목록 정리이며 원본 로그 삭제와 구분합니다.
4. 문의·결과 공유 시 앱 버전과 build 정보를 함께 기록합니다.

언어를 바꿔도 가져온 로그·함수명·메시지가 자동 번역되지는 않습니다.
기본값을 바꿔도 기존 분석 결과를 다시 계산하지 않습니다.
모든 입력에 같은 interval을 적용하지 말고 원본이 선언한 단위를 우선하세요.

<a id="walkthroughs"></a>

## 9. 상황별 실습

아래는 사용법을 설명하는 **가상 조사 시나리오**입니다. 실제 성능 측정 결과나 자동 판정 기준이 아닙니다.

### 9.1 주문 조회가 배포 후 느려졌다

1. **질문 정하기:** 같은 요청량·데이터 조건에서 주문 조회의 지연이 증가했는가?
2. **액세스 로그:** 같은 시간 길이와 URL 범위로 전후 자료를 분석합니다.
   요청 수·오류·p95와 긴 요청이 나온 시각을 확인합니다.
3. **MSA/Trace:** 느린 요청의 명시적 ID를 찾아 SQL·외부 호출·메소드 구간을 나눕니다.
   ID가 없으면 시간 상관의 한계를 기록합니다.
4. **가설 분기:** SQL이 길면 DB 실행계획·대기/잠금 자료를 확인합니다.
   외부 호출이 길면 callee trace, 잔여 메소드가 길면 CPU/wall 프로파일을 봅니다.
   GC 정지와 겹치면 GC/JFR도 확인합니다.
5. **작업 공간:** 관련 결과를 모으고 원본과 케이스에 연결 저장합니다.
   Timeline에는 시각 근거, Board에는 중요한 행과 미확인 가설을 남깁니다.
6. **변경 후:** 부하·버전·수집 옵션을 기록하고 다시 분석합니다.
   프로파일 비교 또는 검증된 baseline으로 변화 경로를 확인하고, 요청 지연·오류도 함께 확인합니다.
7. **보고:** “관측”, “근거”, “가능한 설명”, “추가 확인”, “변경 후 검증”을 구분합니다.

### 9.2 응답이 멈추는데 CPU는 높지 않다

1. 같은 프로세스에서 시간 간격을 기록한 연속 쓰레드 덤프와 wall-clock 프로파일을 준비합니다.
2. Multi 모드에서 반복 stack을 찾고 Locks 모드가 지원되면 owner/wait 관계를 봅니다.
3. WAITING 자체를 장애로 판정하지 않고 유휴 worker와 요청 처리 중 대기를 구분합니다.
4. 연결 획득 대기이면 풀 메트릭·DB 세션 상태, 외부 호출 대기이면 downstream과 timeout을 확인합니다.
5. 케이스에 덤프 순서·간격·프로세스 수명을 기록하고 중요한 stack을 근거로 보존합니다.
6. 조치 후 대기 패턴뿐 아니라 지연·오류·처리량이 함께 개선됐는지 확인합니다.

### 9.3 브라우저가 느리지만 서버 응답은 빠르다

1. 동일 사용자 동작의 HTTP/HAR로 서버 응답 관측 범위를 확인합니다.
2. Browser CPU에서 self가 큰 JavaScript 경로와 긴 sampled run을 찾습니다.
3. Lighthouse JSON의 audit·리소스를 보고 큰 script/리소스와 연결해 조사합니다.
4. CPU sample이 없는 구간을 자동으로 네트워크/렌더 지연으로 분류하지 말고 필요한 DevTools 자료를 추가 수집합니다.
5. 같은 기기·cache·네트워크 조건으로 다시 측정하고 그림과 출처를 Board에 보관합니다.

### 9.4 LLM 응답 시작이 늦거나 token 생성이 느리다

1. 요청의 TTFT와 전체 latency 중 어느 구간이 문제인지 구분합니다.
2. 작업 공간에서 지원 LLM trace와 추론 서빙·메트릭 이력을 가져옵니다.
3. Queue·prefill·decode 구분이 실제 기록돼 있는지 확인하고, 없으면 분해 시간을 만들지 않습니다.
4. 요청량·동시성·model/token 조건을 확인하고 장치 지표와 같은 시각의 근거를 비교합니다.
5. GPU 실행 자료가 있으면 지원 CPU/GPU 구간 패널로 보완하되, GPU 사용률 하나로 병목을 확정하지 않습니다.
6. 성공 요청만의 분포와 전체 error/started/completed를 구분해 비교하고 누락 신호도 보고합니다.

<a id="troubleshooting"></a>

## 10. 문제 해결·용어·완료 확인

### 자주 막히는 상황

| 상황 | 확인 순서 |
| --- | --- |
| 파일을 열었지만 결과가 비어 있음 | 형식/producer 지원 → 원본 내용 → 파서 진단 → 시간창/필터 → 해당 이벤트 수집 여부 |
| 파일이 너무 크거나 예산 오류 | 어떤 입력·모델·결과 한도인지 확인 → 수집 시간/프로파일/시계열 선택 축소 → 새 결과 생성 |
| JFR 변환 불가 | 호환 설치 JDK와 절대 경로 → 입력·출력 한도/새 파일명 → 수집 환경에서 미리 JSON 변환 |
| 비교 대상이 비활성화·비교 거절 | metric/unit → 선택 → clock/entity → 수집 조건/quality → 원본 가용성과 버전 |
| 저장 케이스 원본 조회 실패 | 외부 파일 존재·변경 여부 → 케이스 최신 리비전 → 원본 연결·digest → 지원 좌표 범위 |
| 상세 기능이 없는 오래된 결과 | 읽기 전용 기본 표시/JSON 보관 → 필요하면 지원 원본에서 명시적으로 재분석 |
| 결과 참조가 오래됨 | 저장 JSON 또는 케이스 스냅샷 다시 열기 → 활성 결과 다시 선택 |
| HTTP가 안 보임 | 지원 OS/세션 상태 → 클라이언트 proxy → CA/truststore → 프로토콜/우회 → attribution/drop |
| Export 형식이 비활성화 | 선택 결과 유형과 지원 형식 확인 → JSON 또는 지원 HTML/CSV/그림 사용 |
| 차트·타임라인에 자료가 없음 | 결과 유형/활성 선택 → 작업 공간에 추가됐는지 → 시각/단위 가용성 → 필터·지원 보기 |
| AI를 사용할 수 없음 | deterministic 결과로 분석 계속 → 로컬 provider/model·비활성 이유·gate 확인 |

원본이 큰 경우 텍스트를 임의 byte 위치에서 잘라 stack/event 구조를 손상시키지 마세요.
생성 도구가 지원하는 시간창·내보내기 선택으로 범위를 줄이는 것이 좋습니다.
64 MiB 케이스 결과 한도처럼 원본보다 정규화 결과가 먼저 커지는 경우도 있습니다.
오류에 표시된 실제 크기와 한도를 확인하세요.

### 주요 용어

| 용어 | 뜻 |
| --- | --- |
| Evidence / 근거 | 관측·계산값과 그 출처. 관측되지 않은 가설과 구분 |
| Finding | 분석기가 근거를 바탕으로 만든 관측/경고. 최종 원인 확정과 다름 |
| Profile / Metric | 어떤 관측 집합을 어떤 측정량·단위로 읽는지 정하는 선택 |
| Self / Inclusive | frame 자체 비용 / 자식 포함 비용 |
| Signature | 유사 stack·transaction 모양을 묶는 식별 기준 |
| Baseline / Target | 비교 기준 / 비교 대상. 순서를 바꾸면 delta 의미도 바뀜 |
| Capture / Provenance | 수집 맥락 / 원본·도구·변환·선택의 출처 기록 |
| Coverage / Availability | 관측/처리 범위 / 값을 제공할 수 있는 상태와 사유 |
| Counter / Gauge / Rate | 누적 계수 / 시점 값 / 시간당 변화율 |
| Clock / 시간창 | 시각의 기준 / 관측을 선택하는 범위. 상대 clock과 실제 시각은 자동 동일하지 않음 |

### 조사 완료 전에 확인할 것

- 증상·대상·시간대·비교 조건과 원본 보관 위치를 기록했는가?
- 전체/부분 처리, 누락·파서 경고·단위·분모를 확인했는가?
- 요약에서 원본 이벤트/stack까지 내려가 근거를 확인했는가?
- 사실·가설·미확인 사항을 구분하고 다음 담당자의 확인 항목을 적었는가?
- 필요한 결과를 케이스/JSON으로 저장하고 다시 열어 보았는가?
- 보고서의 그림·표에 선택 범위·단위·출처가 있으며 민감정보를 검토했는가?
- 변경 후 같은 조건의 지연·오류·처리량과 부작용을 재검증했는가?

<a id="appendix"></a>

## 부록. 빌드·CLI와 심화 문서

설치·개발 빌드는 [네이티브 앱](NATIVE_APP.md), 전체 입력은
[입력 형식 지원표](IMPORTER_SUPPORT_MATRIX.md)를 참고하세요.

### 소스 빌드와 패키징

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```

데스크톱 패키징:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
cd apps/engine-native/cmd/archscope-app
task package
```

### CLI 예시

```bash
cd apps/engine-native

go run ./cmd/archscope-engine access-log analyze \
  --in ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out access.json

go run ./cmd/archscope-engine profiler analyze-collapsed \
  --in ../../examples/profiler/sample-wall.collapsed \
  --out profiler.json

go run ./cmd/archscope-engine thread-dump analyze \
  --in ../../examples/thread-dumps/sample-java-thread-dump.txt \
  --out thread.json

go run ./cmd/archscope-engine trace import \
  --in ../../examples/traces/sample-otlp-traces.jsonl \
  --format auto \
  --out trace.json

go run ./cmd/archscope-engine database-log analyze \
  --in ../../examples/database/sample-postgres.log \
  --format postgres-text \
  --out database.json

go run ./cmd/archscope-engine broker-log analyze \
  --in ../../examples/broker/sample-broker.log \
  --format auto \
  --out broker.json

# Chrome Performance trace 또는 V8 .cpuprofile (Node --cpu-prof, CDP)
go run ./cmd/archscope-engine profile import \
  --in ./trace.json.gz \
  --format auto \
  --out browser-profile.json

# Local Lighthouse report (score 보존, URL 리댁션)
go run ./cmd/archscope-engine browser import \
  --in ./lighthouse-report.json \
  --format lighthouse-json \
  --out browser-audit.json

# 리댁션된 HAR 가져오기 (방언 자동 판별, entry 상한)
go run ./cmd/archscope-engine http-capture analyze \
  --in ./session.har \
  --out http-capture.json

go run ./cmd/archscope-engine api-contract analyze \
  --openapi ../../examples/api-contract/openapi-orders.json \
  --access-result ../../examples/api-contract/access-result.json \
  --asyncapi ../../examples/api-contract/asyncapi-orders.json \
  --broker-result ../../examples/api-contract/broker-result.json \
  --out contract.json

go run ./cmd/archscope-engine stitch analyze \
  --in ../../examples/stitching/access-result.json \
  --in ../../examples/stitching/trace-result.json \
  --in ../../examples/stitching/database-result.json \
  --time-window-seconds 60 \
  --out stitched.json

go run ./cmd/archscope-engine architecture-docs draft \
  --in contract.json --in stitched.json \
  --out architecture-docs.json

go run ./cmd/archscope-engine report html \
  --in architecture-docs.json \
  --out architecture-docs.html
```

전체 command 목록은 `go run ./cmd/archscope-engine --help`로 확인합니다.
현재 지원 evidence family는 [입력 형식 지원표](IMPORTER_SUPPORT_MATRIX.md)에 정리되어
있습니다.


### MSA 선택 범위를 CLI로 저장

GUI와 같은 선택 범위를 지정할 수 있습니다. GUID와 signature hash는 범위를 지정하지 않은
분석 결과에서 확인합니다. TXID 또는 application을 생략하면 GUID/signature 전체를 선택합니다.

```sh
archscope-engine jennifer-profile analyze --in profiles.txt --guid GUID --txid TXID --out scope.json
archscope-engine jennifer-profile report-html --in profiles.txt --guid GUID --txid TXID --out scope.html
archscope-engine jennifer-profile analyze --in profiles.txt --signature HASH --application /orders --out average-scope.json
```

### 목적별 심화 문서

- [입력 안내와 수동 수집](INPUT_ASSISTANCE.md): 형식 확인·수집 키트·JFR 변환.
- [원본 프로파일](PROFILE_SOURCE_IMPORT.md), [케이스 조회와 baseline](CASE_EVIDENCE_QUERIES.md): 정밀 선택·출처·비교.
- [메트릭 이력](METRIC_HISTORY.md), [시스템 근거](SYSTEM_EVIDENCE.md): 구간·rate·OS 근거.
- [보고서용 이미지·표](VISUAL_ARTIFACTS.md): 렌더·클립보드·산출물 보관.
- [HTTP 캡처](SYSTEM_HTTP_CAPTURE.md), [브라우저 CPU](CHROME_DEVTOOLS_CPUPROFILE.md): 실제 지원 범위와 제한.
- [AI 해석](AI_INTERPRETATION.md), [오프라인 팩](OFFLINE_PACKS.md): 선택 기능과 설치/운영 조건.
