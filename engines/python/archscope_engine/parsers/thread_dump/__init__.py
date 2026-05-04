"""Plugin-based parsers for multi-language thread dumps.

The legacy single-file Java parser
(:mod:`archscope_engine.parsers.thread_dump_parser`) keeps living next
door so the existing single-dump analyzer keeps its byte-for-byte
behavior. This subpackage hosts the new plugin protocol, the parser
registry, and one plugin per supported runtime.
"""
from archscope_engine.parsers.thread_dump.dotnet_clrstack import (
    DotnetClrstackParserPlugin,
)
from archscope_engine.parsers.thread_dump.go_goroutine import GoGoroutineParserPlugin
from archscope_engine.parsers.thread_dump.java_jstack import JavaJstackParserPlugin
from archscope_engine.parsers.thread_dump.nodejs_report import (
    NodejsDiagnosticReportParserPlugin,
)
from archscope_engine.parsers.thread_dump.python_dump import (
    PythonFaulthandlerParserPlugin,
    PythonPySpyParserPlugin,
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
    "GoGoroutineParserPlugin",
    "JavaJstackParserPlugin",
    "MixedFormatError",
    "NodejsDiagnosticReportParserPlugin",
    "ParserRegistry",
    "PythonFaulthandlerParserPlugin",
    "PythonPySpyParserPlugin",
    "ThreadDumpParserPlugin",
    "UnknownFormatError",
]
