# 멀티 언어 Thread Dump 프레임워크

Phase 5에서 ArchScope는 언어에 의존하지 않는 thread dump 파이프라인을
도입했습니다. 한 번의 멀티 덤프 실행으로 아래 런타임 중 어느 조합이든
받아 들이고, 스냅샷 사이의 동일 쓰레드를 상관시켜 JVM 프레임 이름에
의존하지 않는 finding을 만들어 냅니다.

## 지원 포맷

| Format ID                    | 런타임           | 자동 감지 시그니처                                                  |
| ---------------------------- | ---------------- | ------------------------------------------------------------------- |
| `java_jstack`                | JVM              | `Full thread dump …` 헤더 **또는** 따옴표 이름 + `nid=0x…`          |
| `go_goroutine`               | Go               | `^goroutine \d+ \[\w` (`runtime.Stack`, panic, debug.Stack)         |
| `python_pyspy`               | Python (py-spy)  | `Process N:` 다음에 `Python vX.Y`                                   |
| `python_faulthandler`        | Python (stdlib)  | `Thread 0xHEX (most recent call first):`                            |
| `nodejs_diagnostic_report`   | Node.js (12+)    | `"header"` + `"javascriptStack"`을 가진 JSON 객체                   |
| `dotnet_clrstack`            | .NET             | `OS Thread Id: 0xHEX` 블록 + `Child SP / IP / Call Site`            |

레지스트리는 모든 입력의 **첫 4 KB**만 읽어 헤더를 검사합니다. 둘
이상의 포맷이 한 입력에 매칭될 가능성이 있으면 더 구체적인 플러그인
먼저 등록되며, 어느 플러그인도 매칭되지 않으면 `UnknownFormatError`
를 던집니다. 자동 감지를 우회하려면 CLI는 `--format`, HTTP는
`format` 필드로 강제할 수 있습니다 — 큰 로그에서 헤더 없는 덤프
조각을 잘라낸 경우 유용합니다.

여러 파일을 입력했을 때 서로 다른 포맷으로 인식되면 즉시
`MixedFormatError`로 거부됩니다. `--format`을 지정하면 검사를
건너뛰고 모든 파일이 지정된 플러그인으로 파싱됩니다.

## 정규화된 데이터 모델

모든 플러그인이 같은 세 가지 레코드를 만듭니다.

- **`StackFrame`** — `function`, `module`, `file`, `line`, `language`.
  `language` 디스크리미네이터 덕분에 enrichment 플러그인은 자기가
  이해하는 프레임만 건드립니다.
- **`ThreadSnapshot`** — `snapshot_id`, `thread_name`, `thread_id`,
  `state`, `category`, `stack_frames`, `lock_info`, `metadata`,
  `language`, `source_format`.
- **`ThreadDumpBundle`** — 한 덤프 파일에서 추출한 모든 스냅샷에
  `dump_index`, `dump_label`, `captured_at`, `metadata`를 추가.

기존 단일 덤프 `ThreadDumpRecord`(`models/thread_dump.py`)는 그대로
유지되어 기존 Java 단일 덤프 분석기는 byte 단위로 동일한 결과를
유지합니다.

## ThreadState enum

`models/thread_snapshot.ThreadState`가 통합 상태 모델입니다.

`RUNNABLE · BLOCKED · WAITING · TIMED_WAITING · NETWORK_WAIT · IO_WAIT
· LOCK_WAIT · CHANNEL_WAIT · DEAD · NEW · UNKNOWN`

`coerce()` 헬퍼는 런타임별 별칭(`RUNNING`, `parked`, `sleeping`,
`chan receive`, `chan send`, `select`, …)을 표준 상태로 매핑합니다.

## 언어별 enrichment 매트릭스

각 파서 플러그인은 자기 언어 한정으로 후처리를 실행해 일반적인
`RUNNABLE` / `UNKNOWN` 상태를 더 구체적인 대기 카테고리로 승격
시킵니다. 그 결과 멀티 덤프 상관 엔진이 언어 비의존 finding을 만들 수
있습니다.

| 언어        | 프레임 정규화                                                                                                                              | 상태 승격                                                                                                                                                                                                                          |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Java        | `$$EnhancerByCGLIB$$<hex>` 제거 · `$$FastClassByCGLIB$$<hex>` 제거 · `$$Proxy<digits>` 숫자 제거 · `(GeneratedMethodAccessor)<digits>` · `(Accessor)<digits>` | `EPoll.epollWait` / `EPollSelectorImpl.doSelect` / `socketAccept` / `socketRead0` / `NioSocketImpl.read` → `NETWORK_WAIT`; `FileInputStream.read*` / `FileChannelImpl.read` / `RandomAccessFile.read*` → `IO_WAIT`. `BLOCKED`는 절대 미덮어씀. |
| Go          | `gin.HandlerFunc.func1` → `gin.HandlerFunc`; 끝의 `.func1.func2` 체인 strip; Echo / Chi / Fiber receiver는 보존.                            | `gopark` / `runtime.selectgo` / `chanrecv` / `chansend` → `CHANNEL_WAIT`; `runtime.netpoll` / `netpollblock` / `net.(*conn).Read` → `NETWORK_WAIT`; `semacquire` / `sync.(*Mutex).Lock` → `LOCK_WAIT`; file IO → `IO_WAIT`.            |
| Python      | function이 `__call__` / `MiddlewareMixin.__call__` / `solve_dependencies` / `run_endpoint_function` / `view_func` / `wrapper` / `dispatch_request`이고 file이 starlette/fastapi/django/flask/gunicorn/uvicorn/werkzeug 디렉토리에 있으면 drop. | socket `recv`/`send`/`accept`/`connect` 또는 urllib3 / requests / httpx → `NETWORK_WAIT`; `threading.{acquire,wait}` / `queue.get` → `LOCK_WAIT`; `select.{select,poll,epoll,kqueue}` / asyncio `sleep`/`run_forever` / gevent → `IO_WAIT`. |
| Node.js     | Express `Layer.handle [as handle_request]` alias 정리.                                                                                       | `payload["libuv"]`를 검사: 활성 `tcp`/`udp`/`pipe` 핸들 → `NETWORK_WAIT`; 활성 `timer`/`fs_event`/`fs_poll` → `IO_WAIT`. JS 프레임만 대상 — native(uv worker pool)는 보고된 상태 그대로.                                          |
| .NET        | `<Outer>g__Inner\|3_0` 합성 로컬 함수 → `Outer.Inner`; `MyApp.<DoWorkAsync>d__0.MoveNext` → `MyApp.DoWorkAsync.MoveNext`.                    | `Monitor.Enter` / `SpinLock` / `SemaphoreSlim` → `LOCK_WAIT`; `Socket.Receive`/`Send` / `HttpClient.Send` / `NetworkStream` → `NETWORK_WAIT`; `FileStream.Read` → `IO_WAIT`.                                                          |

## 멀티 덤프 상관 findings

`analyzers/multi_thread_analyzer.analyze_multi_thread_dumps()`는
순서가 있는 `ThreadDumpBundle` 리스트를 받아
`AnalysisResult(type="thread_dump_multi")`를 만들고 세 가지
finding을 발화합니다.

- **`LONG_RUNNING_THREAD`** *(warning)* — 같은 쓰레드 이름이 같은
  스택 시그니처로 `RUNNABLE` 상태를 ≥ N 연속 덤프 동안 유지. 기본
  threshold = 3.
- **`PERSISTENT_BLOCKED_THREAD`** *(critical)* — 같은 쓰레드가
  `BLOCKED` 또는 `LOCK_WAIT` 상태로 ≥ N 연속 덤프 동안 유지.
- **`LATENCY_SECTION_DETECTED`** *(warning, T-203)* — 같은 쓰레드가
  `NETWORK_WAIT`, `IO_WAIT`, `CHANNEL_WAIT` 중 하나의 상태로 ≥ N 연속
  덤프 동안 유지. 언어 비의존 — 언어별 enrichment 플러그인이 채워준
  `ThreadState`만 사용. `LOCK_WAIT`은 의도적으로 제외했습니다 —
  `PERSISTENT_BLOCKED_THREAD`가 이미 그 신호를 다루기 때문입니다.

CLI에서는 `--consecutive-threshold`, HTTP에서는 `consecutiveThreshold`
필드로 임계치를 조절할 수 있습니다. finding은 결과의 `summary`(개수)
와 `tables`(상세 행)에도 같이 들어갑니다.

## CLI

단일 덤프(레거시, Java 전용):

```bash
archscope-engine thread-dump analyze --file dump.txt --out result.json
```

멀티 덤프(언어 자유 조합):

```bash
archscope-engine thread-dump analyze-multi \
  --input dump-1.txt --input dump-2.txt --input dump-3.txt \
  --out multi-result.json \
  [--format <plugin-id>] \
  [--consecutive-threshold N] \
  [--top-n N]
```

성공하면 한 줄 요약(`<dumps> dumps, <threads> threads, <findings>
findings`)이 출력됩니다.

## HTTP / UI

FastAPI 엔진은 `POST /api/analyzer/execute`로 같은 멀티 덤프 요청을
받습니다.

```json
{
  "type": "thread_dump_multi",
  "params": {
    "filePaths": ["/tmp/uploads/d1.txt", "/tmp/uploads/d2.txt"],
    "consecutiveThreshold": 3,
    "format": null,
    "topN": 20
  }
}
```

오류는 `UNKNOWN_THREAD_DUMP_FORMAT` / `MIXED_THREAD_DUMP_FORMATS`로
매핑되어 UI가 명확한 메시지를 보여 줄 수 있습니다.

리뉴얼된 `Thread Dump` 페이지(Phase 2 shell)는 누적 파일 업로드를
지원하고, threshold와 format-override 입력을 노출하며, 세 가지
finding을 심각도별로 색상 카드와 전용 테이블로 보여 줍니다.

## Profiler SVG / HTML 입력 (Phase 4 교차 참조)

ArchScope는 FlameGraph.pl / async-profiler의 SVG와 HTML 입력도
받습니다(T-184~T-187). 이 파일들은 thread dump 프레임워크가 아니라
기존 collapsed profile 파이프라인에 연결됩니다.

- `archscope-engine profiler analyze-flamegraph-svg --file flame.svg --out result.json`
- `archscope-engine profiler analyze-flamegraph-html --file flame.html --out result.json`

UI에서는 `profileFormat` 셀렉터에 `flamegraph_svg` /
`flamegraph_html` 옵션이 추가됐고, FileDock의 `accept`가 자동으로
`.svg` / `.html,.htm`로 전환됩니다.

## 범위 밖(연기)

- **힙 덤프 분석.** 현재 `.hprof`를 파싱하지 않습니다. thread dump
  프레임워크는 *왜 쓰레드가 멈췄는가*를 보는 도구이지 *할당이 어디에
  살아 있는가*를 보는 도구가 아닙니다.
- **프로세스/시스템 모니터링.** CPU%, RSS, syscall count는 별도 APM
  도구에서 곁들여 보세요.
- **async-profiler 3.x packed JSON.** 인라인 SVG가 들어 있는 HTML과
  레거시 embedded-tree HTML은 지원합니다. packed-binary HTML 포맷은
  미지원이므로 `asprof`에서 `--format svg`로 추출해서 사용하세요.
