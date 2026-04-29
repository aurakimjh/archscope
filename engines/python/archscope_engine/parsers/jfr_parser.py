from __future__ import annotations

import json
from pathlib import Path
from typing import Any, TypedDict


class JfrEvent(TypedDict):
    event_type: str
    time: str
    duration_ms: float | None
    thread: str | None
    message: str
    frames: list[str]
    raw_preview: str


def parse_jfr_print_json(path: Path) -> list[JfrEvent]:
    """Parse JSON produced by `jfr print --json`.

    This is a minimal PoC parser for the command-bridge path. It intentionally
    accepts the common `{"recording": {"events": [...]}}` shape and a plain
    top-level events array so tests can run without a local JDK.
    """
    with path.open("r", encoding="utf-8") as file:
        payload = json.load(file)

    events = _extract_events(payload)
    return [_to_jfr_event(event) for event in events if isinstance(event, dict)]


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

    return {
        "event_type": event_type or "unknown",
        "time": time or "",
        "duration_ms": duration_ms,
        "thread": thread,
        "message": message or "",
        "frames": frames,
        "raw_preview": json.dumps(event, ensure_ascii=False, sort_keys=True)[:500],
    }


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
