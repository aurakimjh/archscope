# 범위가 있는 런타임·TA·실행계획 관측 계약 1.0.0

워크스페이스 앱에서 일반화한 CC0 합성 자료다. 고객·운영 자료가 아니다.

- `generate_python.py`: 실제 CPython 3.14.4 asyncio task/get_stack, threading native ID, tracemalloc statistics, gc callback을 이용한다. Task 상태는 순간 관측이고 coroutine 대기 시간·GIL 보유율을 복원하지 않는다. Allocation은 살아 있는 traced block/bytes snapshot이며 누적 allocation rate나 객체 그래프가 아니다. PID·경로·시각은 비식별 값으로 바꿨다.
- `generate_node.mjs`: 실제 Node.js 25.9.0 getReport(reportVersion 5), worker report, perf_hooks delay histogram·eventLoopUtilization을 사용한 v1 export다. 실제 API 표본을 비식별 부분집합으로 만든 것이며 모든 Node 버전의 전체 report를 검증하지 않는다. Native stack은 report를 만든 thread의 stack이고 worker pool이 아니다. Handle은 active/inactive inventory이지 현재 blocking의 증명이 아니다.
- `generate_rust.py`: 실제 rustc 1.94.1 v0 object symbol 및 합성 panic/backtrace. Legacy mangling과 C++/이름 충돌 사례는 명시적 합성 표본이다. 원본 symbol과 display를 분리하고 hash를 identity에서 제거하지 않는다. Panic 본문은 정규 모델에서 제외한다.
- `ps-l.csv`·`jstack.txt`: 선언한 CSV 및 JDK 21.0.8 jstack 형태의 합성 자료. 같은 host/boot/namespace/PID/process start와 thread lifetime, clock, CPU window, nid/TID 인코딩을 모두 맞춘 비교용이다. 실제 OS 수집 또는 귀속을 인증하지 않는다. ps lifetime-average와 top interval-average는 구분한다.
- PostgreSQL 18.0 형태 합성 JSON: plain EXPLAIN의 estimate와 EXPLAIN ANALYZE의 실제 per-loop row/time/loops를 분리한다. SQL·predicate는 모델에 넣지 않는다. Parent/child 시간을 더하지 않으며 없는 실제 측정을 0으로 만들지 않는다.

모든 API 수치는 현재 한 번의 실제 합성 실행 표본이다. 재생성 시 allocation/GC/event-loop 수치는 바뀔 수 있다. 원본 바이트를 고정한 뒤 앱의 generate-runtime-fixtures.go로 기대 모델을 결정적으로 생성하고 manifest와 임베드 복사본을 함께 갱신한다. Runtime 모델은 32 MiB, 결과는 64 MiB, source는 원본/확장 각 64 MiB이며 개수 전에 byte 한도에 도달할 수 있다.

관계·profile reference는 명시적 선언과 hash 바인딩이며 provenance 인증·인과·요청 귀속을 의미하지 않는다. 실제 format/version 지원은 위 범위로 제한한다. 기술 식별자와 frame/source 경로도 가져오기 전에 민감 정보를 제거한다. Pickle/marshal·힙 객체 그래프·라이브 collector·DB 자동 실행은 포함하지 않는다.
