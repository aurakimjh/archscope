"""Plugin-based parsers for multi-language thread dumps.

The legacy single-file Java parser
(:mod:`archscope_engine.parsers.thread_dump_parser`) keeps living next
door so the existing single-dump analyzer keeps its byte-for-byte
behavior. This subpackage hosts the new plugin protocol, the parser
registry, and one plugin per supported runtime.
"""
# ─────────────────────────────────────────────────────────────────────
# [한글] thread_dump 플러그인 패키지 — 다언어 thread-dump 파서 모음.
#
# 책임/목적
#   다양한 런타임의 thread/stack dump 파일을 동일한 ThreadDumpBundle
#   /ThreadSnapshot 모델로 정규화. 입력 자동 감지 + 라우팅은 registry
#   가 담당.
#
# 등록 플러그인 (생성자 호출 시점에 DEFAULT_REGISTRY 에 자동 등록)
#   - JavaJstackParserPlugin       : Java jstack 텍스트 (가장 복잡).
#   - JavaJcmdJsonParserPlugin     : `jcmd Thread.print -fJson`.
#   - GoGoroutineParserPlugin      : Go goroutine dump.
#   - DotnetClrstackParserPlugin   : WinDbg/SOS clrstack.
#   - DotnetEnvironmentStackTraceParserPlugin : .NET Exception.GetEnvironmentStackTrace.
#   - NodejsDiagnosticReportParserPlugin : Node.js diagnostic report.
#   - NodejsSampleTraceParserPlugin     : `node --inspect` sample trace.
#   - PythonFaulthandlerParserPlugin    : Python `faulthandler.dump_traceback_later`.
#   - PythonPySpyParserPlugin           : `py-spy dump`.
#   - PythonTracebackParserPlugin       : 일반 Python traceback.
#
# 자동 감지 흐름 (registry.py 참조)
#   detect → 가장 자신있는 플러그인 1개 선택 → parse → ThreadDumpBundle.
#
# parity 주의사항: 플러그인 이름, ThreadSnapshot 키, source_format
# 라벨이 Go engine-native internal/threaddump/plugins 와 동일.
# ─────────────────────────────────────────────────────────────────────
from archscope_engine.parsers.thread_dump.dotnet_clrstack import (
    DotnetClrstackParserPlugin,
    DotnetEnvironmentStackTraceParserPlugin,
)
from archscope_engine.parsers.thread_dump.go_goroutine import GoGoroutineParserPlugin
from archscope_engine.parsers.thread_dump.java_jcmd_json import (
    JavaJcmdJsonParserPlugin,
)
from archscope_engine.parsers.thread_dump.java_jstack import JavaJstackParserPlugin
from archscope_engine.parsers.thread_dump.nodejs_report import (
    NodejsDiagnosticReportParserPlugin,
    NodejsSampleTraceParserPlugin,
)
from archscope_engine.parsers.thread_dump.python_dump import (
    PythonFaulthandlerParserPlugin,
    PythonPySpyParserPlugin,
    PythonTracebackParserPlugin,
)
from archscope_engine.parsers.thread_dump.registry import (
    DEFAULT_REGISTRY,
    MixedFormatError,
    ParserRegistry,
    ThreadDumpParserPlugin,
    UnknownFormatError,
)

__all__ = [
    "DEFAULT_REGISTRY",
    "DotnetClrstackParserPlugin",
    "DotnetEnvironmentStackTraceParserPlugin",
    "GoGoroutineParserPlugin",
    "JavaJcmdJsonParserPlugin",
    "JavaJstackParserPlugin",
    "MixedFormatError",
    "NodejsDiagnosticReportParserPlugin",
    "NodejsSampleTraceParserPlugin",
    "ParserRegistry",
    "PythonFaulthandlerParserPlugin",
    "PythonPySpyParserPlugin",
    "PythonTracebackParserPlugin",
    "ThreadDumpParserPlugin",
    "UnknownFormatError",
]
