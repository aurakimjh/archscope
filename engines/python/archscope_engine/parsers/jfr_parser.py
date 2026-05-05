from __future__ import annotations

import json
from json import JSONDecodeError
from pathlib import Path
from typing import Any, TypedDict

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.diagnostics import ParserDiagnostics


class JfrEvent(TypedDict):
    event_type: str
    time: str
    duration_ms: float | None
    thread: str | None
    state: str | None
    address: int | None
    size: int | None
    message: str
    frames: list[str]
    raw_preview: str


def parse_jfr_print_json(
    path: Path,
    *,
    diagnostics: ParserDiagnostics | None = None,
    debug_log: DebugLogCollector | None = None,
) -> list[JfrEvent]:
    """Parse JSON produced by `jfr print --json`.

    This is a minimal PoC parser for the command-bridge path. It intentionally
    accepts the common `{"recording": {"events": [...]}}` shape and a plain
    top-level events array so tests can run without a local JDK.
    """
    own_diagnostics = diagnostics or ParserDiagnostics()
    try:
        with path.open("r", encoding="utf-8") as file:
            payload = json.load(file)
    except JSONDecodeError as exc:
        own_diagnostics.total_lines = 1
        own_diagnostics.add_skipped(
            line_number=exc.lineno,
            reason="INVALID_JFR_JSON",
            message=exc.msg,
            raw_line=f"line={exc.lineno} col={exc.colno}",
        )
        if debug_log is not None:
            debug_log.add_parse_error(
                line_number=exc.lineno,
                reason="INVALID_JFR_JSON",
                message=exc.msg,
                raw_context={
                    "before": None,
                    "target": f"line={exc.lineno} col={exc.colno}",
                    "after": None,
                },
                failed_pattern="JFR_PRINT_JSON",
                field_shapes={"json_error_position": {"line": exc.lineno, "column": exc.colno}},
            )
        raise

    try:
        events = _extract_events(payload)
    except ValueError as exc:
        own_diagnostics.total_lines = 1
        own_diagnostics.add_skipped(
            line_number=1,
            reason="INVALID_JFR_SHAPE",
            message=str(exc),
            raw_line=_preview_json(payload),
        )
        if debug_log is not None:
            debug_log.add_parse_error(
                line_number=1,
                reason="INVALID_JFR_SHAPE",
                message=str(exc),
                raw_context={"before": None, "target": _preview_json(payload), "after": None},
                failed_pattern="JFR_EVENTS_ARRAY",
                field_shapes={"json_top_level_type": type(payload).__name__},
            )
        raise

    parsed: list[JfrEvent] = []
    own_diagnostics.total_lines = len(events)
    for index, event in enumerate(events, start=1):
        if not isinstance(event, dict):
            own_diagnostics.add_skipped(
                line_number=index,
                reason="INVALID_JFR_EVENT",
                message="JFR event entry must be an object.",
                raw_line=_preview_json(event),
            )
            if debug_log is not None:
                debug_log.add_parse_error(
                    line_number=index,
                    reason="INVALID_JFR_EVENT",
                    message="JFR event entry must be an object.",
                    raw_context={"before": None, "target": _preview_json(event), "after": None},
                    failed_pattern="JFR_EVENT_OBJECT",
                    field_shapes={"json_path": f"events[{index - 1}]"},
                )
            continue
        parsed.append(_to_jfr_event(event))
        own_diagnostics.parsed_records += 1
    return parsed


def _extract_events(payload: object) -> list[object]:
    if isinstance(payload, list):
        return payload
    if not isinstance(payload, dict):
        raise ValueError("JFR JSON payload must be an object or array.")

    recording = payload.get("recording")
    if isinstance(recording, dict) and isinstance(recording.get("events"), list):
        return recording["events"]
    if isinstance(payload.get("events"), list):
        return payload["events"]

    raise ValueError("JFR JSON payload does not contain an events array.")


def _to_jfr_event(event: dict[str, Any]) -> JfrEvent:
    values = event.get("values")
    values = values if isinstance(values, dict) else {}
    event_type = _string_value(event.get("type")) or _string_value(event.get("eventType"))
    time = _string_value(values.get("startTime")) or _string_value(event.get("startTime"))
    duration_ms = _duration_to_ms(values.get("duration") or event.get("duration"))
    thread = _extract_thread(values.get("eventThread") or values.get("thread"))
    frames = _extract_frames(values.get("stackTrace"))
    message = _string_value(values.get("message")) or _string_value(values.get("name"))

    state_value = values.get("state")
    if isinstance(state_value, dict):
        state = _string_value(state_value.get("name"))
    else:
        state = _string_value(state_value)
    address = _to_int(values.get("address"))
    size = _to_int(values.get("size") or values.get("allocationSize") or values.get("bytes"))
    return {
        "event_type": event_type or "unknown",
        "time": time or "",
        "duration_ms": duration_ms,
        "thread": thread,
        "state": state,
        "address": address,
        "size": size,
        "message": message or "",
        "frames": frames,
        "raw_preview": json.dumps(event, ensure_ascii=False, sort_keys=True)[:500],
    }


def _to_int(value: object) -> int | None:
    if isinstance(value, bool):  # bool is subclass of int — exclude.
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, float):
        return int(value)
    if isinstance(value, str):
        text = value.strip()
        if not text:
            return None
        try:
            if text.startswith("0x") or text.startswith("0X"):
                return int(text, 16)
            return int(text)
        except ValueError:
            return None
    return None


def _extract_thread(value: object) -> str | None:
    if isinstance(value, str):
        return value
    if isinstance(value, dict):
        for key in ("javaName", "osName", "name"):
            text = _string_value(value.get(key))
            if text:
                return text
    return None


def _extract_frames(value: object) -> list[str]:
    if not isinstance(value, dict):
        return []

    frames = value.get("frames")
    if not isinstance(frames, list):
        return []

    return [_format_frame(frame) for frame in frames if isinstance(frame, dict)]


def _format_frame(frame: dict[str, Any]) -> str:
    method = frame.get("method")
    if isinstance(method, dict):
        method_name = _string_value(method.get("name"))
        type_value = method.get("type")
        type_name = _string_value(type_value.get("name")) if isinstance(type_value, dict) else None
        if type_name and method_name:
            return f"{type_name}.{method_name}"
        if method_name:
            return method_name

    return _string_value(frame.get("name")) or "unknown"


def _duration_to_ms(value: object) -> float | None:
    if isinstance(value, (int, float)):
        return float(value)
    if not isinstance(value, str):
        return None

    text = value.strip()
    if not text:
        return None
    try:
        if text.endswith(" ms"):
            return float(text.removesuffix(" ms"))
        if text.endswith(" ns"):
            return float(text.removesuffix(" ns")) / 1_000_000
        if text.endswith(" us"):
            return float(text.removesuffix(" us")) / 1_000
        if text.endswith(" s"):
            return float(text.removesuffix(" s")) * 1_000
        return float(text)
    except ValueError:
        return None


def _string_value(value: object) -> str | None:
    return value if isinstance(value, str) and value else None


def _preview_json(value: object) -> str:
    try:
        return json.dumps(value, ensure_ascii=False, sort_keys=True)[:500]
    except TypeError:
        return repr(value)[:500]
