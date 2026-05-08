"""Cross-plugin auto-detection sanity check (T-189 contract).

For each language we feed the same head bytes that the runtime would
emit and assert that ``DEFAULT_REGISTRY`` picks exactly the matching
plugin — no overlap between Java/Go/Python/Node.js/.NET headers.
"""
# ─────────────────────────────────────────────────────────────────────
# [한글] test_thread_dump_registry_routing — registry 자동 감지 sanity
# 체크 (T-189 contract).
#
# 검증 대상
#   각 런타임이 emit 하는 시그니처 head bytes 를 입력으로 줄 때
#   DEFAULT_REGISTRY 가 정확히 해당 plugin 을 채택. 5개 런타임 (Java/
#   Go/Python/Node.js/.NET) 사이에 head 시그니처 중복 / 모호성이 없는지
#   테이블로 검증.
#
# 5개 런타임 시그니처 (head 4 KB)
#   • java_jstack: `Full thread dump OpenJDK 64-Bit Server VM`
#   • java_jcmd_json: `{"recording": ` 또는 `{"threads": [`
#   • go_goroutine: `goroutine 1 [running]:`
#   • python_dump: `Process 12345:` (py-spy) 또는
#                  `Thread 0x7f...` (faulthandler)
#   • nodejs_diagnostic_report: `{"header": {"event":`
#   • dotnet_clrstack: `OS Thread Id: 0x...`
#
# parity 주의
#   Go engine-native registry (apps/engine-native/internal/threaddump/)
#   도 같은 시그니처 표 + 같은 우선순위. 이 테스트가 통과 == 두 엔진의
#   감지 결과 동등.
# ─────────────────────────────────────────────────────────────────────
from __future__ import annotations

import json

import pytest

from archscope_engine.parsers.thread_dump import (
    DEFAULT_REGISTRY,
    DotnetClrstackParserPlugin,
    GoGoroutineParserPlugin,
    JavaJcmdJsonParserPlugin,
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
                    "threadDump": {
                        "threadContainers": [
                            {"threads": [{"name": "main", "state": "RUNNABLE"}]}
                        ]
                    }
                }
            ),
            "java_jcmd_json",
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
        "java_jcmd_json",
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
    assert isinstance(by_id["java_jcmd_json"], JavaJcmdJsonParserPlugin)
    assert isinstance(by_id["go_goroutine"], GoGoroutineParserPlugin)
    assert isinstance(by_id["python_pyspy"], PythonPySpyParserPlugin)
    assert isinstance(by_id["python_faulthandler"], PythonFaulthandlerParserPlugin)
    assert isinstance(
        by_id["nodejs_diagnostic_report"], NodejsDiagnosticReportParserPlugin
    )
    assert isinstance(by_id["dotnet_clrstack"], DotnetClrstackParserPlugin)
