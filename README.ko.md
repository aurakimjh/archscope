# ArchScope (한국어)

[English](./README.en.md) · [최상위 README](./README.md)

ArchScope는 애플리케이션 아키텍트가 운영 데이터(access log, GC log,
profiler output, thread dump, exception stack, JFR recording)를
**로컬 브라우저**에서 보고서용 차트와 진단 finding으로 변환하기 위한
도구입니다. 외부 SaaS로 데이터를 보내지 않습니다.

## 한 눈에 보는 기능

| 영역 | 기능 |
| --- | --- |
| **Profiler** | async-profiler `collapsed` / Jennifer APM CSV / FlameGraph.pl 및 async-profiler **SVG** / async-profiler **HTML**(인라인 SVG + JS 트리 폴백). **Differential flame**(빨강=느려짐 / 파랑=빨라짐), **Icicle**(역방향) 보기, **Min-width** 단순화, **Per-thread** 필터(`-t` collapsed), **Tree view** 정렬 가능 표, **pprof export**(gzipped, Pyroscope / Speedscope / `go tool pprof` 호환). 일반 크기는 SVG 플레임, 노드 4 000개 이상은 Canvas 자동 전환. |
| **JFR recording** | 바이너리 `.jfr`을 JDK `jfr` CLI(PATH / `JAVA_HOME` / `ARCHSCOPE_JFR_CLI`)로 자동 변환; 기존 JSON 경로도 그대로 동작. **이벤트 모드 필터**(`cpu` / `wall` / `alloc` / `lock` / `gc` / `exception` / `io` / `nativemem`), **시간범위 필터**(ISO / HH:MM:SS / `+30s` / `-2m` / `500ms`), **스레드 상태 필터**, **최소 지속시간 필터**. **Wall-clock 히트맵**(드래그로 From/To 자동 입력). **네이티브 메모리 leak 검출**(tail-ratio cutoff). |
| **GC log** | **JVM Info** 카드(Version / CPUs / Memory / Heap Min/Initial/Max / Region / Parallel·Concurrent workers / Compressed Oops / CommandLine flags) + 워커 vs CPU 미스매치 경고. Pause + Heap 타임라인 **드래그 사각형 줌**과 브러시. 구간 통계(count / avg / p95 / max). Collector별 비교 탭. **9개 토글 가능 힙 시리즈**(Heap before/after/committed, Young, Old, Metaspace) + 우측 축 Pause 오버레이. 큰 로그용 데이터 데시메이션. |
| **Access log** | 22-메트릭 요약: 총량 / 오류 / **p50 / p90 / p95 / p99**, **throughput**(req/s, bytes/s), **정적 vs API** 비율. 분당 시계열로 percentile 타임라인, status-class 분포, 오류율, throughput. **정렬 가능 URL 통계 표**(count / avg / p95 / 총 바이트 / 오류, API/정적 필터, 행마다 2·3·4·5xx 분포). **시간대별 오류** 뷰는 50%+ 오류율 분(分)을 강조. |
| **Thread dumps** | 5개 런타임 자동 감지 — `java_jstack`(JDK 21+ no-`nid` 변형 포함), `java_jcmd_json`, `go_goroutine`, `python_pyspy`/`python_faulthandler`/`python_traceback`, `nodejs_diagnostic_report`/`nodejs_sample_trace`, `dotnet_clrstack`/`dotnet_environment_stacktrace`. **다중 파일 선택**(폴더 단위 드롭 가능). 언어별 프레임 정규화. 멀티 덤프 상관 분석 finding: `LONG_RUNNING_THREAD`, `PERSISTENT_BLOCKED_THREAD`, `LATENCY_SECTION_DETECTED`, `GROWING_LOCK_CONTENTION`, `THREAD_CONGESTION_DETECTED`, `EXTERNAL_RESOURCE_WAIT_HIGH`, `LIKELY_GC_PAUSE_DETECTED`, `VIRTUAL_THREAD_CARRIER_PINNING`, `SMR_UNRESOLVED_THREAD`. **Lock contention** owner/waiter 그래프 + DFS 데드락 검출. **JVM signals** 탭(Carrier-pinning / SMR / Native methods / Class histogram). UTF-16 / BOM 자동 감지. |
| **Exception logs** | 전용 페이지에 페이지네이션 + 필터링 이벤트 표; 행 클릭 시 Sheet 팝업으로 메시지/시그니처/스택 풀 텍스트. 타입별 짧은 클래스명(hover 시 풀 FQN), top stack signatures. |
| **Thread → flamegraph** | 여러 런타임의 덤프 수백 개를 한 번에 FlameGraph 호환 collapsed 파일로 변환(CLI + HTTP). |
| **보고서** | AnalysisResult 단위 HTML / PowerPoint / before-after diff export. 차트별 이미지 export(PNG 1×/2×/3×, JPEG 2×, SVG 벡터). 페이지별 **"모든 차트 저장"** 일괄 export. **pprof** 프로파일러 내보내기. |
| **UI** | Tailwind v4 + shadcn/ui 셸, **Pretendard Variable** 폰트, 슬림 톱바, 접기 가능한 사이드바, light/dark/system 테마, 한국어 ↔ 영어 라벨, 다중 파일 드래그&드롭 FileDock. |

## 기술 스택

- **프론트엔드** — React 18 + Vite 8 + TypeScript + Tailwind v4 + shadcn/ui(Radix 기반) + lucide 아이콘. 차트는 D3(timeline / bar / flamegraph + Canvas flamegraph)와 ECharts(레거시 패널). 이미지 export는 `html-to-image`와 native `canvas.toDataURL()`.
- **백엔드** — FastAPI 0.110+ + uvicorn(단일 in-process Python). React 빌드는 `/`에서 정적 서빙, analyzer dispatcher는 `/api/analyzer/execute`.
- **엔진** — Pure Python(`archscope-engine`, Python ≥ 3.9), Typer CLI, defusedxml(XXE 안전 SVG 파싱), python-multipart 업로드. 서브프로세스 분기 없이 in-process 호출.

## 빠른 시작

```bash
# 1. 엔진 + 웹 서버
cd engines/python
python -m venv .venv
source .venv/bin/activate          # 또는 Windows에서 .venv\Scripts\activate
pip install -e .

# 2. UI 빌드 + 서버 실행 (한 번에)
cd ../..
./scripts/serve-web.sh             # apps/frontend/dist 빌드 후 서버 기동

# 3. 브라우저에서 http://127.0.0.1:8765 접속
```

UI 핫 리로드가 필요한 개발 루프:

```bash
# 터미널 1 — 엔진 자동 리로드
archscope-engine serve --reload

# 터미널 2 — Vite dev 서버 (/api → :8765 프록시)
cd apps/frontend && npm install && npm run dev
# http://127.0.0.1:5173 접속
```

## CLI 치트시트

```bash
# 웹 서버
archscope-engine serve [--host 127.0.0.1 --port 8765 --reload \
                        --static-dir apps/frontend/dist --no-dev-cors]

# 프로파일러
archscope-engine profiler analyze-collapsed       --wall flame.collapsed --out result.json
archscope-engine profiler analyze-flamegraph-svg  --file flame.svg       --out result.json
archscope-engine profiler analyze-flamegraph-html --file flame.html      --out result.json
archscope-engine profiler analyze-jennifer-csv    --file flame.csv       --out result.json

# GC, JFR, Exception, Access log
archscope-engine gc-log    analyze     --file gc.log     --out result.json
archscope-engine jfr       analyze-json --file jfr.json  --out result.json
archscope-engine exception analyze     --file ex.txt     --out result.json
archscope-engine access-log analyze    --file access.log --format nginx  --out result.json

# Thread dumps
archscope-engine thread-dump analyze       --file dump.txt --out result.json
archscope-engine thread-dump analyze-multi --input d1.txt --input d2.txt --input d3.txt \
                                           --out multi.json
archscope-engine thread-dump to-collapsed  --input d1.txt --input d2.txt \
                                           --output flame.collapsed [--format <id>]

# 보고서
archscope-engine report html --input result.json --out report.html
archscope-engine report pptx --input result.json --out report.pptx
archscope-engine report diff --before before.json --after after.json \
                             --out diff.json --html-out diff.html
```

페이지별 / CLI별 상세 사용법은 [`docs/ko/USER_GUIDE.md`](docs/ko/USER_GUIDE.md)를,
멀티 언어 thread dump 상세는
[multi-language thread dump 가이드](docs/ko/MULTI_LANGUAGE_THREADS.md)를
보세요.

## 로컬 우선

- 모든 파싱 / enrichment / 멀티 덤프 상관 / export가 로컬 Python
  프로세스 안에서 실행됩니다. 엔진은 기본적으로 `127.0.0.1`에 바인딩.
- 업로드된 파일은 `~/.archscope/uploads/`, 설정은
  `~/.archscope/settings.json`에 저장됩니다. 언제든 삭제 가능.
- 선택적 AI interpretation은 **로컬** Ollama 인스턴스만 호출합니다.
  원격 LLM 호출 없음.

## 라이선스

MIT — [LICENSE](./LICENSE) 참고.
