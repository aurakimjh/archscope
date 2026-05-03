# ArchScope 사용자 가이드

이 문서는 ArchScope Desktop Application의 사용자를 위한 종합 가이드입니다.

---

## 목차

1. [ArchScope 소개](#archscope-소개)
2. [설치 및 실행](#설치-및-실행)
3. [화면 구성 개요](#화면-구성-개요)
4. [Dashboard](#dashboard)
5. [Access Log Analyzer](#access-log-analyzer)
6. [Profiler Analyzer](#profiler-analyzer)
7. [GC Log Analyzer](#gc-log-analyzer)
8. [Thread Dump Analyzer](#thread-dump-analyzer)
9. [Exception Analyzer](#exception-analyzer)
10. [Chart Studio](#chart-studio)
11. [Demo Data Center](#demo-data-center)
12. [Export Center](#export-center)
13. [Settings](#settings)
14. [공통 기능](#공통-기능)
15. [문제 해결](#문제-해결)

---

## ArchScope 소개

### 이 도구는 무엇인가요?

ArchScope는 애플리케이션 운영 데이터(access log, GC log, profiler output, thread dump, exception stack trace 등)를 분석하여 아키텍처 진단 근거로 변환하는 Desktop 도구입니다.

### 누구를 위한 도구인가요?

- **애플리케이션 아키텍트**: 운영 데이터를 보고서용 진단 근거로 정리해야 하는 분
- **성능 엔지니어**: 병목 구간 분석과 시각화가 필요한 분
- **개발팀 리더**: 시스템 상태를 팀에 공유할 보고서를 만들어야 하는 분

### 핵심 가치

- **오프라인 동작**: 모든 분석이 로컬에서 수행되며, 외부 서버로 데이터를 전송하지 않습니다
- **보고서 중심**: 단순 조회가 아니라, 바로 보고서에 넣을 수 있는 차트와 표를 생성합니다
- **다중 런타임**: Java/JVM뿐 아니라 Node.js, Python, Go, .NET 스택도 분석합니다

### 진단 흐름

```
원천 데이터 → 파싱 → 분석/집계 → 시각화 → 보고서 Export
```

ArchScope는 이 전체 흐름을 하나의 도구 안에서 처리합니다.

---

## 설치 및 실행

### 시스템 요구사항

| 항목 | 요구사항 |
|------|----------|
| OS | macOS 12+, Windows 10+ |
| Python | 3.10 이상 |
| Node.js | 18 이상 (개발 환경) |
| 메모리 | 8GB 이상 권장 |
| 디스크 | 500MB 이상 여유 공간 |

### 설치 방법

#### 방법 1: 패키징된 Desktop App (향후 배포)

패키징된 앱은 별도 설치 없이 실행 파일을 더블클릭하여 시작합니다. Python engine이 sidecar binary로 포함되어 있어 추가 설정이 필요하지 않습니다.

#### 방법 2: 개발 환경에서 실행

```bash
# 1. Python Engine 설치
cd engines/python
python -m venv .venv
source .venv/bin/activate    # macOS/Linux
# .venv\Scripts\activate     # Windows
pip install -e .

# 2. Desktop UI 실행
cd apps/desktop
npm install
npm run dev
```

### 첫 실행 확인

앱을 실행하면 좌측 사이드바와 함께 Dashboard 화면이 표시됩니다. 상단 파이프라인 다이어그램(Raw Data → Parsing → Analysis → Visualization → Export)이 보이면 정상 실행된 것입니다.

---

## 화면 구성 개요

### 전체 레이아웃

ArchScope의 화면은 크게 세 영역으로 나뉩니다.

```
┌─────────────────────────────────────────────────────┐
│                    상단 바 (TopBar)                    │
├──────────┬──────────────────────────────────────────┤
│          │                                          │
│  사이드바  │              메인 컨텐츠                   │
│ (Sidebar)│            (Main Panel)                  │
│          │                                          │
│          │                                          │
└──────────┴──────────────────────────────────────────┘
```

### 상단 바 (TopBar)

- **ArchScope 로고**: 앱 이름과 브랜드 표시
- **파이프라인 다이어그램**: 현재 진단 흐름을 시각적으로 보여줍니다
- **언어 전환**: English / 한국어 전환 드롭다운

### 사이드바 (Sidebar)

좌측 280px 너비의 네비게이션 패널입니다. 아래 메뉴를 순서대로 포함합니다:

| 메뉴 | 설명 |
|------|------|
| Dashboard | 분석 결과 요약 대시보드 |
| Access Log | 웹 서버 접근 로그 분석 |
| Profiler | CPU/Wall 프로파일 분석 |
| GC Log | JVM GC 로그 분석 |
| Thread Dump | 스레드 덤프 분석 |
| Exception | 예외 스택 분석 |
| Chart Studio | 차트 템플릿 미리보기 |
| Demo Data Center | 데모 시나리오 실행 |
| Export Center | 보고서 내보내기 |
| Settings | 설정 |

### 메인 패널 (Main Panel)

선택한 메뉴에 따라 해당 기능의 화면이 표시됩니다. Analyzer 페이지들은 공통적으로 두 영역으로 나뉩니다:

- **왼쪽 도구 패널 (Tool Panel)**: 파일 선택, 옵션 설정, 실행 버튼
- **오른쪽 결과 패널 (Results Panel)**: 메트릭 카드, 차트, 테이블

---

## Dashboard

### 화면 설명

Dashboard는 ArchScope를 실행했을 때 가장 먼저 보이는 화면입니다. 샘플 분석 결과를 사용하여 주요 지표와 차트를 한눈에 보여줍니다.

### 표시 항목

#### 메트릭 카드 (4개)

| 카드 | 설명 |
|------|------|
| Total Requests | 분석된 총 요청 수 |
| Avg Response Time | 평균 응답 시간 (ms) |
| p95 Response Time | 95 퍼센타일 응답 시간 (ms) |
| Error Rate | 에러 비율 (%) |

#### 차트

- **요청 수 추이**: 시간별 요청량 변화를 라인 차트로 표시
- **응답 시간 추이**: 시간별 평균/p95 응답 시간 추이
- **상태 코드 분포**: HTTP 상태 코드별 비율 (파이 차트 또는 바 차트)

### 사용법

Dashboard는 별도 조작 없이 자동으로 샘플 데이터를 로드합니다. 실제 분석을 수행하려면 좌측 사이드바에서 원하는 Analyzer를 선택하세요.

---

## Access Log Analyzer

### 화면 설명

웹 서버(NGINX, Apache, OHS, WebLogic, Tomcat 등)의 접근 로그를 분석하여 요청 통계, 응답 시간 분포, 에러율을 산출합니다.

### 지원 로그 형식

| 형식 | 설명 |
|------|------|
| nginx | NGINX combined/custom log format |
| apache | Apache combined log format |
| ohs | Oracle HTTP Server log format |
| weblogic | WebLogic access log format |
| tomcat | Tomcat access log format |
| custom_regex | 사용자 정의 정규식 패턴 |

### 사용법 (단계별)

#### 1단계: 파일 선택

- **드래그 앤 드롭**: 분석할 로그 파일을 파일 드롭 영역으로 끌어다 놓습니다
- **또는 찾아보기**: "Browse" 버튼을 클릭하여 파일 선택 대화상자에서 파일을 선택합니다

파일이 선택되면 드롭 영역에 파일 경로가 표시됩니다.

#### 2단계: 옵션 설정

| 옵션 | 필수 | 기본값 | 설명 |
|------|------|--------|------|
| Log Format | 예 | nginx | 분석할 로그의 형식 |
| Max Lines | 아니오 | 전체 | 분석할 최대 줄 수 (대용량 파일 시 유용) |
| Start Time | 아니오 | - | 분석 시작 시각 필터 |
| End Time | 아니오 | - | 분석 종료 시각 필터 |

**팁**: 수백MB 이상의 대용량 파일은 Max Lines를 설정하여 먼저 샘플 분석한 뒤, 필요시 전체 분석을 수행하세요.

#### 3단계: 분석 실행

- 파일과 Log Format이 모두 설정되면 **"Analyze"** 버튼이 활성화됩니다
- 버튼을 클릭하면 분석이 시작됩니다
- 분석 중에는 "Analyzing..." 상태가 표시되며, **"Cancel"** 버튼으로 중단할 수 있습니다

#### 4단계: 결과 확인

분석이 완료되면 오른쪽 결과 패널에 다음이 표시됩니다:

**메트릭 카드:**
- Total Requests: 총 요청 수
- Avg Response Time: 평균 응답 시간
- p95 Response Time: 95 퍼센타일 응답 시간
- Error Rate: HTTP 4xx/5xx 에러 비율

**차트:**
- 시간대별 요청 수 추이 (라인 차트)

**테이블:**
- Top URLs by Response Time: 응답 시간 기준 상위 URL 목록
  - URI: 요청 URL 경로
  - Count: 해당 URL의 요청 횟수
  - Response Time: 평균 응답 시간

**진단 정보 (Diagnostics Panel):**
- Total Lines: 파일의 전체 줄 수
- Parsed Records: 정상 파싱된 레코드 수
- Skipped Lines: 건너뛴 비정상 줄 수
- Samples: 건너뛴 줄의 예시 (있는 경우)

**엔진 메시지 (Engine Messages Panel):**
- 분석 엔진에서 전달하는 정보성 메시지

### 결과 해석 가이드

- **Skipped Lines가 많은 경우**: Log Format 설정이 실제 파일 형식과 맞는지 확인하세요
- **Error Rate가 높은 경우**: 특정 시간대에 에러가 집중되었는지 차트에서 확인하세요
- **p95가 평균보다 크게 높은 경우**: 일부 요청에서 심각한 지연이 발생하고 있습니다. Top URLs 테이블에서 원인 URL을 확인하세요

---

## Profiler Analyzer

### 화면 설명

CPU 또는 Wall-clock 프로파일러(async-profiler 등)의 collapsed stack 출력을 분석하여 Flamegraph, 상위 스택, 실행 시간 분포를 보여줍니다.

### 지원 입력 형식

- **Collapsed Stack**: async-profiler의 collapsed 형식 출력 (`-o collapsed`)
- **Jennifer APM CSV**: Jennifer APM에서 내보낸 flamegraph CSV (별도 메뉴)

### 사용법 (단계별)

#### 1단계: 파일 선택

Wall-clock collapsed stack 파일을 드래그 앤 드롭하거나 "Browse"로 선택합니다.

#### 2단계: 옵션 설정

| 옵션 | 필수 | 기본값 | 설명 |
|------|------|--------|------|
| Wall Interval (ms) | 예 | 100 | 프로파일링 샘플링 간격 (밀리초) |
| Elapsed Seconds | 아니오 | - | 프로파일링 총 소요 시간 (초) |
| Top N | 아니오 | 20 | 상위 N개 스택 표시 |
| Filter Pattern | 아니오 | - | 스택 필터링 텍스트/정규식 |
| Filter Type | 아니오 | include_text | 필터 적용 방식 |
| Match Mode | 아니오 | anywhere | 매칭 모드 |
| View Mode | 아니오 | preserve_full_path | 결과 표시 방식 |

**Filter Type 옵션:**

| 값 | 설명 |
|----|------|
| include_text | 지정 텍스트를 포함하는 스택만 표시 |
| exclude_text | 지정 텍스트를 제외한 스택 표시 |
| regex_include | 정규식에 매칭되는 스택만 표시 |
| regex_exclude | 정규식에 매칭되는 스택 제외 |

**Match Mode 옵션:**

| 값 | 설명 |
|----|------|
| anywhere | 스택 어디에서든 패턴이 나타나면 매칭 |
| ordered | 패턴이 순서대로 나타나야 매칭 |
| subtree | 패턴 이하 서브트리만 추출 |

**View Mode 옵션:**

| 값 | 설명 |
|----|------|
| preserve_full_path | 전체 콜 스택 경로 유지 |
| reroot_at_match | 매칭 지점을 루트로 재설정 |

#### 3단계: 분석 실행

파일과 Wall Interval이 설정되면 "Analyze" 버튼이 활성화됩니다. 클릭하여 실행합니다.

#### 4단계: 결과 확인

**메트릭 카드:**
- Total Samples: 전체 샘플 수
- Estimated Time: 추정 실행 시간

**Flamegraph:**
- 스택 프레임의 실행 시간 분포를 시각화합니다
- 각 프레임 위에 마우스를 올리면 상세 정보를 볼 수 있습니다
- 프레임을 클릭하면 해당 서브트리로 드릴다운합니다

**Top Stacks 테이블:**
- 실행 시간이 가장 긴 상위 스택 프레임 목록

**Execution Breakdown 테이블:**
- 런타임 분류별 실행 시간 분포 (예: JDBC, Spring, JVM Internal 등)

### Flamegraph 드릴다운 사용법

1. Flamegraph에서 관심 있는 프레임을 클릭합니다
2. 해당 프레임이 루트가 되어 하위 콜 스택이 확대됩니다
3. 상단 브레드크럼을 클릭하면 이전 레벨로 돌아갑니다
4. 필터를 적용하면 특정 패키지/클래스만 포커싱할 수 있습니다

### 결과 해석 가이드

- **넓은 프레임**: 해당 메서드에서 많은 시간을 소비하고 있음을 의미합니다
- **깊은 스택**: 콜 체인이 깊을수록 추적이 복잡해질 수 있습니다
- **Execution Breakdown에서 특정 분류 비율이 높은 경우**: 해당 영역이 병목 후보입니다
  - JDBC 비율 높음 → DB 쿼리 최적화 검토
  - Network/HTTP 비율 높음 → 외부 호출 지연 점검
  - GC 비율 높음 → 메모리 설정 검토

---

## GC Log Analyzer

### 화면 설명

JVM Garbage Collection 로그를 분석하여 GC pause 시간 추이, 힙 사용량, Collector 원인별 분류를 제공합니다.

### 지원 입력 형식

- HotSpot GC log (JDK 8 이상의 `-Xlog:gc*` 또는 `-verbose:gc` 형식)

### 사용법 (단계별)

#### 1단계: 파일 선택

GC 로그 파일을 드래그 앤 드롭하거나 "Browse"로 선택합니다.

#### 2단계: 분석 실행

파일이 선택되면 "Analyze" 버튼이 활성화됩니다. 클릭하여 분석을 시작합니다.

#### 3단계: 결과 확인

**주요 분석 항목:**
- GC pause 시간 타임라인
- 힙 사용량 추이 (Before/After GC)
- Collector 유형 및 원인별 분류
- Full GC 발생 빈도

### 결과 해석 가이드

- **Pause 시간이 점진적으로 증가**: 메모리 누수 또는 힙 크기 부족 가능성
- **Full GC 빈번**: Old Generation 크기 확대 또는 메모리 누수 점검 필요
- **특정 시각에 GC pause 집중**: 해당 시간대의 트래픽 또는 배치 작업 확인

---

## Thread Dump Analyzer

### 화면 설명

Java Thread Dump를 분석하여 스레드 상태 분포, 블로킹 스레드 그룹, 스택 시그니처를 분류합니다.

### 지원 입력 형식

- `jstack` 출력 또는 동등한 JVM thread dump 텍스트

### 사용법 (단계별)

#### 1단계: 파일 선택

Thread dump 파일을 드래그 앤 드롭하거나 "Browse"로 선택합니다.

#### 2단계: 분석 실행

파일이 선택되면 "Analyze" 버튼이 활성화됩니다. 클릭하여 분석을 시작합니다.

#### 3단계: 결과 확인

**주요 분석 항목:**
- 스레드 상태 분포 (RUNNABLE, WAITING, BLOCKED, TIMED_WAITING)
- 블로킹 스레드 그룹 (같은 락을 대기하는 스레드 묶음)
- 스택 시그니처 분석 (빈도 높은 콜 스택 패턴)

### 결과 해석 가이드

- **BLOCKED 스레드 다수**: 락 경합(Lock Contention) 문제. 어떤 리소스를 대기 중인지 확인
- **WAITING 스레드 과다**: 커넥션 풀 고갈 또는 큐 대기 확인
- **동일 스택 시그니처 반복**: 해당 코드 경로가 병목 후보

---

## Exception Analyzer

### 화면 설명

Java 예외 스택 트레이스를 분석하여 예외 발생 추이, 근본 원인 그룹, 스택 시그니처를 분류합니다.

### 지원 입력 형식

- Java 예외 스택 트레이스 (표준 `printStackTrace()` 형식)

### 사용법 (단계별)

#### 1단계: 파일 선택

예외 로그 파일을 드래그 앤 드롭하거나 "Browse"로 선택합니다.

#### 2단계: 분석 실행

파일이 선택되면 "Analyze" 버튼이 활성화됩니다. 클릭하여 분석을 시작합니다.

#### 3단계: 결과 확인

**주요 분석 항목:**
- 예외 유형별 발생 빈도 추이
- 근본 원인(Root Cause) 그룹
- 스택 시그니처 테이블

### 결과 해석 가이드

- **특정 예외 급증**: 배포 또는 설정 변경 시점과 대조하세요
- **NullPointerException 반복**: 방어 코드 또는 입력 검증 누락 확인
- **동일 Root Cause 그룹**: 하나의 수정으로 여러 예외를 해결할 수 있는 기회

---

## Chart Studio

### 화면 설명

Chart Studio는 ArchScope의 차트 템플릿을 미리보기하고 설정을 조정할 수 있는 개발/검토 도구입니다. 실제 분석 결과를 사용하기 전에 차트의 외형과 구성을 확인하는 데 활용합니다.

### 사용법 (단계별)

#### 1단계: 차트 템플릿 선택

상단 드롭다운에서 사용 가능한 차트 템플릿을 선택합니다. 템플릿은 분석 유형별로 분류되어 있습니다.

#### 2단계: 설정 조정

| 설정 | 설명 |
|------|------|
| Custom Title | 차트 제목 커스터마이즈 |
| Renderer | Canvas (성능 우선) 또는 SVG (선명도 우선) 선택 |
| Theme | Light 또는 Dark 테마 전환 |

#### 3단계: 미리보기 확인

- 중앙에 차트 미리보기가 실시간으로 렌더링됩니다
- 설정을 변경할 때마다 차트가 즉시 업데이트됩니다

#### 4단계: Option JSON 확인

- 하단에 생성된 ECharts option JSON을 확인할 수 있습니다
- 이 JSON은 보고서 커스터마이징이나 디버깅에 참고할 수 있습니다

### 활용 팁

- **보고서 준비 시**: Light/Dark 테마를 전환하여 발표 환경에 맞는 차트 스타일을 확인하세요
- **인쇄용**: SVG renderer를 선택하면 확대해도 선명한 출력물을 얻을 수 있습니다
- **프레젠테이션용**: Canvas renderer가 렌더링 성능이 좋습니다

---

## Demo Data Center

### 화면 설명

Demo Data Center는 미리 정의된 시나리오를 실행하여 ArchScope의 전체 분석 흐름을 체험할 수 있는 기능입니다. 테스트 데이터를 사용하여 분석 → 시각화 → 보고서 생성의 전 과정을 확인합니다.

### 사용법 (단계별)

#### 1단계: Manifest Root 선택

"Browse" 버튼을 클릭하여 demo-site manifest가 있는 디렉터리를 선택합니다.

기본 위치: `projects-assets/test-data/demo-site`

#### 2단계: 데이터 소스 필터

| 필터 | 설명 |
|------|------|
| All | 모든 시나리오 표시 |
| Synthetic | 합성(생성된) 테스트 데이터만 표시 |
| Real | 실제 샘플 데이터만 표시 |

#### 3단계: 시나리오 선택

드롭다운에서 실행할 시나리오를 선택합니다. 각 시나리오는 특정 진단 상황을 시뮬레이션합니다 (예: GC 압력, 느린 쿼리, 높은 에러율 등).

#### 4단계: 실행

"Run demo data" 버튼을 클릭하면 선택된 시나리오가 실행됩니다.

#### 5단계: 결과 확인

실행 완료 후 다음 정보가 표시됩니다:

**요약 메트릭:**
- Analyzer Outputs: 생성된 분석 결과 수
- Failed Analyzers: 실패한 분석기 수
- Skipped Lines: 건너뛴 줄 수
- Reference Files: 참조 파일 수
- Findings: 발견된 항목 수

**시나리오 결과 (확장 가능):**
- **Artifact 테이블**: 생성된 분석 파일 목록
  - "Open" 버튼: 시스템 뷰어에서 파일 열기
  - "Send to Export Center" 버튼: Export Center로 파일 전달
- **실패 목록**: 분석에 실패한 항목 (있는 경우)
- **Skipped Lines**: 건너뛴 비정상 줄 수
- **참조 파일**: 시나리오에 포함된 참고 자료

### 활용 팁

- ArchScope를 처음 사용하는 경우, Demo Data Center에서 시나리오를 실행하여 각 Analyzer의 출력을 미리 확인하세요
- "Send to Export Center"를 활용하면 데모 결과를 바로 보고서로 변환할 수 있습니다

---

## Export Center

### 화면 설명

Export Center는 분석 결과(AnalysisResult JSON)를 다양한 형식의 보고서 파일로 변환합니다.

### 지원 Export 형식

| 형식 | 설명 | 입력 |
|------|------|------|
| HTML Report | 인터랙티브 HTML 보고서 | JSON 1개 |
| Before/After Diff | 개선 전후 비교 보고서 | JSON 2개 (Before, After) |
| PowerPoint | 발표용 PPTX 슬라이드 | JSON 1개 |

### 사용법 (단계별)

#### 1단계: Export 형식 선택

상단 드롭다운에서 원하는 Export 형식을 선택합니다.

#### 2단계: 입력 파일 선택

선택한 형식에 따라 입력 UI가 변경됩니다:

**HTML Report / PowerPoint:**
- "Browse" 버튼으로 분석 결과 JSON 파일 1개를 선택합니다

**Before/After Diff:**
- Before JSON: 개선 전 분석 결과 파일
- After JSON: 개선 후 분석 결과 파일

#### 3단계: Export 실행

"Export" 버튼을 클릭하면 보고서 생성이 시작됩니다.

#### 4단계: 결과 확인

생성 완료 후:
- **출력 파일 경로**: 생성된 보고서 파일의 위치가 표시됩니다
- **엔진 메시지**: 변환 과정의 정보성 메시지
- **에러 정보**: 실패 시 상세 에러 내용

### Export 형식별 상세

#### HTML Report

- 단일 HTML 파일로 생성됩니다 (외부 의존성 없음)
- 브라우저에서 바로 열어 확인할 수 있습니다
- 포함 내용: summary, findings, diagnostics, series 차트, tables, chart data preview
- Profiler 결과의 경우 static HTML flamegraph가 포함됩니다

#### Before/After Diff

- 두 분석 결과의 주요 지표를 비교합니다
- numeric summary field와 finding count의 변화를 보여줍니다
- `--html-out` 옵션 사용 시 비교 HTML도 함께 생성됩니다

#### PowerPoint

- `.pptx` 형식으로 생성됩니다
- 포함 슬라이드: 제목, source metadata, summary metrics, findings
- 발표 자료에 바로 활용할 수 있습니다

### 활용 팁

- **정기 보고 시**: HTML Report로 빠르게 공유 가능한 보고서를 생성하세요
- **성능 개선 보고 시**: Before/After Diff로 개선 효과를 명확히 보여주세요
- **경영진 보고 시**: PowerPoint로 요약 슬라이드를 자동 생성하세요

---

## Settings

### 화면 설명

Settings 화면에서는 ArchScope의 전반적인 동작을 설정합니다.

### 설정 항목 (향후 제공)

| 항목 | 설명 |
|------|------|
| Engine Path | Python engine 실행 파일 경로 |
| Default Chart Theme | 기본 차트 테마 (Light/Dark) |
| Locale | UI 및 보고서 기본 언어 (English/Korean) |

---

## 공통 기능

### 파일 드롭 영역 (FileDropZone)

모든 Analyzer 화면에서 공통으로 사용되는 파일 입력 컴포넌트입니다.

**사용 방법:**
1. 파일을 마우스로 드래그하여 점선 영역 위에 놓습니다
2. 또는 "Browse" 버튼을 클릭하여 파일 탐색기에서 선택합니다
3. 파일이 선택되면 경로가 표시되며, 다른 파일을 다시 드롭하면 교체됩니다

### 언어 전환

- 상단 바 우측의 언어 드롭다운으로 English ↔ 한국어를 전환합니다
- 전환 시 모든 UI 라벨, 플레이스홀더, 에러 메시지가 즉시 변경됩니다
- 분석 결과의 데이터 값은 변경되지 않습니다

### 분석 취소

- 분석 진행 중 "Cancel" 버튼이 나타납니다
- 클릭하면 진행 중인 분석을 즉시 중단합니다
- 이전 결과는 유지되며, 새로운 분석을 다시 실행할 수 있습니다

### 상태 표시

모든 Analyzer 화면은 동일한 상태 모델을 따릅니다:

| 상태 | UI 표시 | 설명 |
|------|---------|------|
| Idle | 빈 결과 영역, Analyze 비활성화 | 아직 분석 시작 전 |
| Ready | Analyze 활성화 | 필수 입력이 모두 설정됨 |
| Running | 로딩 표시, Cancel 활성화 | 분석 진행 중 |
| Success | 메트릭, 차트, 테이블 표시 | 분석 완료 |
| Error | 에러 패널 표시 | 분석 실패 |

### 에러 메시지

에러 발생 시 다음 정보가 표시됩니다:

| 필드 | 설명 |
|------|------|
| Code | 에러 코드 (기술 지원 시 참조) |
| Message | 사용자 친화적 에러 설명 |
| Detail | 상세 에러 정보 (확장하여 확인) |

**주요 에러 코드:**

| 코드 | 의미 | 대처 방법 |
|------|------|-----------|
| FILE_NOT_FOUND | 파일을 찾을 수 없음 | 파일 경로와 존재 여부 확인 |
| INVALID_OPTION | 옵션 값이 잘못됨 | 필수 옵션 확인 (형식, 간격 등) |
| ENGINE_EXITED | 분석 엔진 비정상 종료 | Python engine 설치 상태 확인 |
| ENGINE_OUTPUT_INVALID | 엔진 출력 형식 오류 | 입력 파일 형식이 올바른지 확인 |
| IPC_FAILED | 엔진 통신 실패 | 앱 재시작 후 재시도 |
| ANALYZER_NOT_CONNECTED | 분석 엔진 미연결 | 앱 재시작 또는 Engine Path 확인 |

---

## 문제 해결

### 자주 묻는 질문

#### Q: 분석 결과에 Skipped Lines가 많이 나옵니다

**A:** 로그 형식(Log Format) 설정이 실제 파일과 맞지 않을 가능성이 높습니다.
- 파일의 첫 몇 줄을 텍스트 에디터로 열어 형식을 확인하세요
- NGINX와 Apache는 유사하지만 미묘한 차이가 있습니다
- 표준 형식에 맞지 않으면 `custom_regex` 옵션을 고려하세요

#### Q: 분석이 오래 걸립니다

**A:** 대용량 파일의 경우:
- Max Lines 옵션을 설정하여 샘플 분석을 먼저 수행하세요 (예: 100000)
- Start Time / End Time 필터로 관심 구간만 분석하세요
- 수GB 파일은 사전에 필요한 시간대만 추출하는 것을 권장합니다

#### Q: Flamegraph가 표시되지 않습니다

**A:**
- 입력 파일이 collapsed stack 형식인지 확인하세요 (각 줄이 `스택;프레임;프레임 샘플수` 형식)
- Wall Interval 값이 양수인지 확인하세요
- 파일 인코딩이 UTF-8인지 확인하세요

#### Q: Export에 실패합니다

**A:**
- 입력 JSON이 유효한 AnalysisResult 형식인지 확인하세요
- 출력 경로에 쓰기 권한이 있는지 확인하세요
- 디스크 여유 공간을 확인하세요

#### Q: 앱이 시작되지 않습니다

**A:**
- (개발 환경) `npm install`이 완료되었는지 확인하세요
- (개발 환경) Python engine이 설치되어 있는지 확인하세요: `archscope-engine --help`
- macOS에서 "개발자를 확인할 수 없음" 경고가 뜨면: 시스템 설정 > 개인 정보 보호 및 보안에서 허용하세요

### Parser Debug Log 활용

분석 결과에 문제가 있을 때 parser debug log를 활용할 수 있습니다.

**Debug log란?**
- 파싱 과정에서 건너뛴 줄의 원인과 맥락을 기록한 파일입니다
- 개인정보는 기본 redaction이 적용되어 안전하게 공유할 수 있습니다
- 기술 지원 시 이 파일을 전달하면 문제 진단에 도움이 됩니다

**CLI에서 생성하기:**
```bash
archscope-engine access-log analyze \
  --file your-log-file.log \
  --format nginx \
  --out result.json \
  --debug-log \
  --debug-log-dir ./archscope-debug
```

생성된 `archscope-debug/` 폴더를 기술 지원팀에 전달하세요.

---

## 부록: CLI 명령 참조

Desktop UI 외에 CLI(Command Line Interface)로도 모든 분석을 실행할 수 있습니다.

### Access Log 분석

```bash
archscope-engine access-log analyze \
  --file <로그파일경로> \
  --format <nginx|apache|ohs|weblogic|tomcat|custom_regex> \
  --out <출력JSON경로>
```

### Profiler 분석

```bash
# Collapsed stack 분석
archscope-engine profiler analyze-collapsed \
  --wall <collapsed파일경로> \
  --wall-interval-ms <샘플링간격> \
  --elapsed-sec <총소요시간> \
  --out <출력JSON경로>

# Jennifer APM CSV 분석
archscope-engine profiler analyze-jennifer-csv \
  --file <CSV파일경로> \
  --out <출력JSON경로>

# 드릴다운
archscope-engine profiler drilldown \
  --wall <collapsed파일경로> \
  --filter <필터패턴> \
  --out <출력JSON경로>

# Execution Breakdown
archscope-engine profiler breakdown \
  --wall <collapsed파일경로> \
  --filter <필터패턴> \
  --out <출력JSON경로>
```

### GC Log 분석

```bash
archscope-engine gc-log analyze \
  --file <GC로그경로> \
  --out <출력JSON경로>
```

### Thread Dump 분석

```bash
archscope-engine thread-dump analyze \
  --file <덤프파일경로> \
  --out <출력JSON경로>
```

### Exception 분석

```bash
archscope-engine exception analyze \
  --file <예외로그경로> \
  --out <출력JSON경로>
```

### 보고서 생성

```bash
# HTML 보고서
archscope-engine report html \
  --input <분석결과JSON> \
  --out <출력HTML경로>

# Before/After 비교
archscope-engine report diff \
  --before <이전JSON> \
  --after <이후JSON> \
  --out <비교결과JSON경로> \
  --html-out <비교HTML경로>

# PowerPoint
archscope-engine report pptx \
  --input <분석결과JSON> \
  --out <출력PPTX경로>
```

### Multi-Runtime 분석

```bash
# Node.js
archscope-engine nodejs analyze --file <파일> --out <출력>

# Python Traceback
archscope-engine python-traceback analyze --file <파일> --out <출력>

# Go Panic
archscope-engine go-panic analyze --file <파일> --out <출력>

# .NET/IIS
archscope-engine dotnet analyze --file <파일> --out <출력>

# OpenTelemetry
archscope-engine otel analyze --file <파일> --out <출력>
```
