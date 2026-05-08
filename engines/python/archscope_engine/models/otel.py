"""OpenTelemetry log record."""
# [한글] otel.OTelLogRecord — OTLP-JSONL 로그 한 라인.
# trace_id / span_id / parent_span_id 로 trace 트리 재구성 가능.
# severity 는 OTLP 표준 라벨 (TRACE/DEBUG/INFO/WARN/ERROR/FATAL).
# parity: Go engine-native internal/models 와 동일.
from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class OTelLogRecord:
    timestamp: str | None
    trace_id: str | None
    span_id: str | None
    parent_span_id: str | None
    service_name: str | None
    severity: str
    body: str
    raw_line: str
