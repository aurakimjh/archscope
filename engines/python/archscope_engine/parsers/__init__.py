# ─────────────────────────────────────────────────────────────────────
# [한글] parsers 패키지 — archscope_engine 의 모든 파서 진입점.
#
# 책임 범위
#   raw 파일/입력을 도메인별 typed record 로 변환하는 단계.
#   분석 (집계/통계/finding 산출) 은 analyzers/ 가 담당 — parser 는
#   "표현 형식의 차이를 흡수해서 단일 record 모델로 정규화" 만 함.
#
# 패키지 레이아웃 (각 파일 헤더의 [한글] 블록 참고)
#   • access_log_parser     : nginx/apache/IIS access log.
#   • collapsed_parser      : async-profiler / FlameGraph collapsed.
#   • jennifer_csv_parser   : Jennifer APM CSV (legacy 경로).
#   • jfr_parser            : `jfr print --json` JSON 출력.
#   • jfr_recording         : 바이너리 .jfr → JSON 변환 (JDK CLI 브리지).
#   • exception_parser      : Java 예외 스택.
#   • gc_log_parser         : HotSpot Unified/G1 Legacy/Legacy GC log.
#   • gc_log_header         : GC log 의 JVM Info 헤더 추출.
#   • otel_parser           : OpenTelemetry JSONL.
#   • svg_flamegraph_parser : FlameGraph.pl/async-profiler SVG.
#   • html_profiler_parser  : async-profiler HTML wrapper.
#   • dotnet_parser / go_panic_parser / nodejs_stack_parser /
#     python_traceback_parser : 4개 런타임 stack trace.
#   • thread_dump_parser     : 레거시 단일 thread-dump 진입점.
#   • thread_dump/           : 5개 런타임 thread-dump plugin (자동
#     감지 registry + 각 plugin).
#
# Go engine-native parity
#   같은 입력에 대해 Go 측 (apps/engine-native/internal/parsers/) 과
#   동일한 typed record 를 emit 해야 합니다. parity gate 가 두 엔진의
#   결과 JSON 을 byte 단위로 비교하므로, 정렬/키 이름/skip 사유 모두
#   동기화 필요.
#
# 명시적 export (__all__)
#   가장 자주 쓰이는 3개만 top-level export. 나머지는 모듈 경로로
#   직접 import.
# ─────────────────────────────────────────────────────────────────────
from archscope_engine.parsers.access_log_parser import parse_access_log
from archscope_engine.parsers.collapsed_parser import parse_collapsed_file
from archscope_engine.parsers.jfr_parser import parse_jfr_print_json

__all__ = ["parse_access_log", "parse_collapsed_file", "parse_jfr_print_json"]
