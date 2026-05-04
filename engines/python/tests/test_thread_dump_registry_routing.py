"""Cross-plugin auto-detection sanity check (T-189 contract).

For each language we feed the same head bytes that the runtime would
emit and assert that ``DEFAULT_REGISTRY`` picks exactly the matching
plugin — no overlap between Java/Go/Python/Node.js/.NET headers.
"""
from __future__ import annotations

import json

import pytest

from archscope_engine.parsers.thread_dump import (
    DEFAULT_REGISTRY,
    DotnetClrstackParserPlugin,
    GoGoroutineParserPlugin,
    JavaJstackParserPlugin,
    NodejsDiagnosticReportParserPlugin,
    PythonFaulthandlerParserPlugin,
    PythonPySpyParserPlugin,
)


@pytest.mark.parametrize(
    ("head", "expected_format"),
    [
        (
            "Full thread dump OpenJDK 64-Bit Server VM (17.0.1):\n"
            '"http-nio-1" #25 nid=0x00ab runnable\n',
            "java_jstack",
        ),
        (
            "goroutine 1 [running]:\nmain.main()\n",
            "go_goroutine",
        ),
        (
            "Process 12345: python /app/server.py\n"
            "Python v3.11.7\n\nThread 12345 (idle): \"MainThread\"\n",
            "python_pyspy",
        ),
        (
            "Fatal Python error\nThread 0x00007f12 (most recent call first):\n"
            '  File "/app/x.py", line 1 in main\n',
            "python_faulthandler",
        ),
        (
            json.dumps(
                {
                    "header": {"event": "diag"},
                    "javascriptStack": {"stack": []},
                    "nativeStack": [],
                    "libuv": [],
                }
            ),
            "nodejs_diagnostic_report",
        ),
        (
            "OS Thread Id: 0x1a4 (1)\n"
            "        Child SP               IP Call Site\n"
            "0123 0456 MyApp.Program.Main(System.String[])\n",
            "dotnet_clrstack",
        ),
    ],
)
def test_default_registry_routes_each_format_to_matching_plugin(
    head: str, expected_format: str
) -> None:
    plugin = DEFAULT_REGISTRY.detect_format(head)
    assert plugin is not None, f"No plugin matched for {expected_format}"
    assert plugin.format_id == expected_format


def test_default_registry_rejects_random_text() -> None:
    plugin = DEFAULT_REGISTRY.detect_format(
        "Some unrelated log line\nNothing thread-dumpy here.\n"
    )
    assert plugin is None


def test_default_registry_has_all_expected_plugins() -> None:
    formats = {plugin.format_id for plugin in DEFAULT_REGISTRY.plugins}
    assert formats == {
        "java_jstack",
        "go_goroutine",
        "python_pyspy",
        "python_faulthandler",
        "nodejs_diagnostic_report",
        "dotnet_clrstack",
    }


def test_default_registry_picks_each_plugin_class() -> None:
    """Make sure we can fetch each plugin instance back by format id."""
    by_id = {plugin.format_id: plugin for plugin in DEFAULT_REGISTRY.plugins}
    assert isinstance(by_id["java_jstack"], JavaJstackParserPlugin)
    assert isinstance(by_id["go_goroutine"], GoGoroutineParserPlugin)
    assert isinstance(by_id["python_pyspy"], PythonPySpyParserPlugin)
    assert isinstance(by_id["python_faulthandler"], PythonFaulthandlerParserPlugin)
    assert isinstance(
        by_id["nodejs_diagnostic_report"], NodejsDiagnosticReportParserPlugin
    )
    assert isinstance(by_id["dotnet_clrstack"], DotnetClrstackParserPlugin)
