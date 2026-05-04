"""Tests for the Node.js diagnostic-report parser plugin (T-200 / T-201)."""
from __future__ import annotations

import json

from archscope_engine.models.thread_snapshot import StackFrame, ThreadState
from archscope_engine.parsers.thread_dump.nodejs_report import (
    NodejsDiagnosticReportParserPlugin,
    _normalize_js_frame,
)


_REPORT: dict[str, object] = {
    "header": {
        "event": "JavaScript API",
        "trigger": "writeReport",
        "filename": "report.json",
        "nodejsVersion": "v20.10.0",
    },
    "javascriptStack": {
        "message": "Diagnostic report on demand",
        "stack": [
            "    at handler (/app/server.js:42:5)",
            "    at Layer.handle [as handle_request] (/app/node_modules/express/lib/router/layer.js:95:5)",
            "    at next (/app/node_modules/express/lib/router/route.js:144:13)",
            "    at /app/server.js:88:3",
        ],
    },
    "nativeStack": [
        {"pc": "0xdead", "symbol": "uv_run"},
        {"pc": "0xbeef", "symbol": "node::SpinEventLoop"},
    ],
    "libuv": [
        {"type": "tcp", "is_active": True},
        {"type": "timer", "is_active": False},
    ],
}


def test_can_parse_recognizes_diagnostic_report_json() -> None:
    plugin = NodejsDiagnosticReportParserPlugin()
    assert plugin.can_parse(json.dumps(_REPORT)[:4000])


def test_can_parse_rejects_unrelated_json() -> None:
    plugin = NodejsDiagnosticReportParserPlugin()
    assert not plugin.can_parse('{"foo": 1, "bar": 2}')


def test_parse_extracts_js_main_thread_and_native_pool(tmp_path) -> None:
    path = tmp_path / "report.json"
    path.write_text(json.dumps(_REPORT), encoding="utf-8")

    bundle = NodejsDiagnosticReportParserPlugin().parse(path)

    assert bundle.language == "javascript"
    assert bundle.source_format == "nodejs_diagnostic_report"
    assert [snap.thread_name for snap in bundle.snapshots] == ["main", "native"]
    js_thread = bundle.snapshots[0]
    assert js_thread.stack_frames[0].function == "handler"
    assert js_thread.stack_frames[0].file == "/app/server.js"
    assert js_thread.stack_frames[0].line == 42
    # T-201: Layer.handle [as handle_request] → Layer.handle
    assert js_thread.stack_frames[1].function == "Layer.handle"
    # libuv carries an active TCP handle → main is in NETWORK_WAIT.
    assert js_thread.state is ThreadState.NETWORK_WAIT


def test_native_thread_marked_as_native_language(tmp_path) -> None:
    path = tmp_path / "report.json"
    path.write_text(json.dumps(_REPORT), encoding="utf-8")
    bundle = NodejsDiagnosticReportParserPlugin().parse(path)

    native = bundle.snapshots[1]
    assert native.language == "native"
    assert native.stack_frames[0].language == "native"
    assert native.stack_frames[0].function == "uv_run"


def test_libuv_handles_recorded_in_metadata(tmp_path) -> None:
    path = tmp_path / "report.json"
    path.write_text(json.dumps(_REPORT), encoding="utf-8")
    bundle = NodejsDiagnosticReportParserPlugin().parse(path)
    assert bundle.metadata["libuv_handles"] == _REPORT["libuv"]
    assert bundle.metadata["header"]["nodejsVersion"] == "v20.10.0"


def test_normalize_strips_express_alias() -> None:
    frame = StackFrame(
        function="Layer.handle [as handle_request]",
        file="/x.js",
        line=1,
        language="javascript",
    )
    cleaned = _normalize_js_frame(frame)
    assert cleaned.function == "Layer.handle"


def test_invalid_json_returns_empty_bundle(tmp_path) -> None:
    path = tmp_path / "broken.json"
    path.write_text("{not json", encoding="utf-8")
    bundle = NodejsDiagnosticReportParserPlugin().parse(path)
    assert bundle.snapshots == []
    assert bundle.metadata.get("parse_error") == "INVALID_NODEJS_REPORT_JSON"
