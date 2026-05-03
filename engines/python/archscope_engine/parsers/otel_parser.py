from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from archscope_engine.common.debug_log import DebugLogCollector, infer_field_shapes
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.models.otel import OTelLogRecord


def parse_otel_jsonl(
    path: Path,
    *,
    diagnostics: ParserDiagnostics | None = None,
    debug_log: DebugLogCollector | None = None,
) -> list[OTelLogRecord]:
    own_diagnostics = diagnostics or ParserDiagnostics()
    records: list[OTelLogRecord] = []
    for line_number, line in enumerate(iter_text_lines(path), start=1):
        own_diagnostics.total_lines += 1
        if not line.strip():
            continue
        try:
            payload = json.loads(line)
        except json.JSONDecodeError as exc:
            _skip(
                own_diagnostics,
                debug_log,
                line_number=line_number,
                reason="INVALID_OTEL_JSON",
                message=str(exc),
                line=line,
            )
            continue
        record = _record_from_payload(payload, raw_line=line)
        if record is None:
            _skip(
                own_diagnostics,
                debug_log,
                line_number=line_number,
                reason="INVALID_OTEL_RECORD",
                message="JSON object did not contain OTel log fields.",
                line=line,
            )
            continue
        records.append(record)
        own_diagnostics.parsed_records += 1
    return records


def _record_from_payload(payload: Any, *, raw_line: str) -> OTelLogRecord | None:
    if not isinstance(payload, dict):
        return None
    trace_id = _string_value(payload, "trace_id", "traceId", "traceid")
    span_id = _string_value(payload, "span_id", "spanId", "spanid")
    parent_span_id = _parent_span_id(payload)
    body = _body_value(payload.get("body") or payload.get("message"))
    if trace_id is None and span_id is None and body is None:
        return None
    return OTelLogRecord(
        timestamp=_string_value(payload, "timestamp", "time", "observed_time"),
        trace_id=trace_id,
        span_id=span_id,
        parent_span_id=parent_span_id,
        service_name=_service_name(payload),
        severity=(
            _string_value(payload, "severity_text", "severityText", "level")
            or "UNSPECIFIED"
        ),
        body=body or "",
        raw_line=raw_line,
    )


def _service_name(payload: dict[str, Any]) -> str | None:
    direct = _string_value(payload, "service_name", "service.name")
    if direct:
        return direct
    resource = payload.get("resource")
    if isinstance(resource, dict):
        attrs = resource.get("attributes")
        if isinstance(attrs, dict):
            return _string_value(attrs, "service.name", "service_name")
    attributes = payload.get("attributes")
    if isinstance(attributes, dict):
        return _string_value(attributes, "service.name", "service_name")
    return None


def _parent_span_id(payload: dict[str, Any]) -> str | None:
    direct = _string_value(
        payload,
        "parent_span_id",
        "parentSpanId",
        "parent_spanid",
        "parentSpanID",
    )
    if direct:
        return direct
    attributes = payload.get("attributes")
    if isinstance(attributes, dict):
        return _string_value(
            attributes,
            "parent_span_id",
            "parentSpanId",
            "parent.span_id",
            "parentSpanID",
        )
    return None


def _string_value(payload: dict[str, Any], *keys: str) -> str | None:
    for key in keys:
        value = payload.get(key)
        if isinstance(value, str):
            return value
        if isinstance(value, (int, float)):
            return str(value)
        if isinstance(value, dict):
            nested = _body_value(value)
            if nested is not None:
                return nested
    return None


def _body_value(value: Any) -> str | None:
    if isinstance(value, str):
        return value
    if isinstance(value, (int, float, bool)):
        return str(value)
    if isinstance(value, dict):
        for key in ("stringValue", "str", "value", "text"):
            nested = value.get(key)
            if isinstance(nested, str):
                return nested
    return None


def _skip(
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None,
    *,
    line_number: int,
    reason: str,
    message: str,
    line: str,
) -> None:
    diagnostics.add_skipped(
        line_number=line_number,
        reason=reason,
        message=message,
        raw_line=line,
    )
    if debug_log is not None:
        debug_log.add_parse_error(
            line_number=line_number,
            reason=reason,
            message=message,
            raw_context={"before": None, "target": line, "after": None},
            failed_pattern="OTEL_JSONL_LOG_RECORD",
            field_shapes=infer_field_shapes(line),
        )
