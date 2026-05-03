from __future__ import annotations

import json
import platform
import traceback
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional

from archscope_engine import __version__
from archscope_engine.common.redaction import (
    REDACTION_VERSION,
    merge_redaction_summaries,
    redact_text,
)

MAX_SAMPLES_PER_ERROR_TYPE = 5
MAX_CONTEXT_CHARS = 500
MAX_DEBUG_LOG_BYTES = 1_000_000

RawContext = dict[str, Optional[str]]


@dataclass
class DebugLogCollector:
    analyzer_type: str
    source_file: Path
    parser: str
    parser_options: dict[str, Any] = field(default_factory=dict)
    debug_log_dir: Path | None = None
    force_write: bool = False
    encoding_detected: str | None = None
    redaction_enabled: bool = True
    _errors: dict[str, dict[str, Any]] = field(default_factory=dict)
    _exceptions: list[dict[str, Any]] = field(default_factory=list)
    _redaction_summary: dict[str, int] = field(default_factory=dict)

    def set_redaction(
        self,
        *,
        enabled: bool = True,
        redaction_version: str = REDACTION_VERSION,
    ) -> None:
        self.redaction_enabled = enabled
        # Version is currently fixed but retained to keep the public collector API explicit.
        _ = redaction_version

    def add_parse_error(
        self,
        *,
        line_number: int | None,
        reason: str,
        message: str,
        raw_context: RawContext | None = None,
        partial_match: dict[str, Any] | None = None,
        failed_pattern: str | None = None,
        field_shapes: dict[str, Any] | None = None,
        description: str | None = None,
    ) -> None:
        entry = self._errors.setdefault(
            reason,
            {
                "count": 0,
                "description": description or message,
                "failed_pattern": failed_pattern,
                "samples": [],
            },
        )
        entry["count"] += 1
        if failed_pattern and not entry.get("failed_pattern"):
            entry["failed_pattern"] = failed_pattern
        if len(entry["samples"]) >= MAX_SAMPLES_PER_ERROR_TYPE:
            return

        redacted_context = self._redact_context(raw_context or {})
        sample = {
            "line_number": line_number,
            "raw_context": redacted_context,
            "field_shapes": field_shapes or infer_field_shapes(
                (raw_context or {}).get("target") or ""
            ),
            "partial_match": partial_match,
            "message": message,
        }
        entry["samples"].append(sample)

    def add_exception(
        self,
        *,
        phase: str,
        exception: BaseException,
        line_number: int | None = None,
        raw_context: RawContext | None = None,
    ) -> None:
        self._exceptions.append(
            {
                "phase": phase,
                "line_number": line_number,
                "exception_type": type(exception).__name__,
                "message": redact_text(str(exception)).text,
                "traceback": traceback.format_exc(),
                "raw_context": self._redact_context(raw_context or {}),
            }
        )

    def should_write(self) -> bool:
        return self.force_write or bool(self._errors) or bool(self._exceptions)

    def write(
        self,
        *,
        diagnostics: dict[str, Any] | None = None,
        output_dir: Path | None = None,
    ) -> Path | None:
        if not self.should_write():
            return None

        target_dir = output_dir or self.debug_log_dir or default_debug_log_dir()
        target_dir.mkdir(parents=True, exist_ok=True)
        path = target_dir / self.filename()
        payload = self.to_dict(diagnostics=diagnostics, debug_log_dir=target_dir)
        _write_size_limited_json(path, payload)
        return path

    def filename(self) -> str:
        timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
        return f"archscope-debug-{self.analyzer_type}-{timestamp}.json"

    def to_dict(
        self,
        *,
        diagnostics: dict[str, Any] | None = None,
        debug_log_dir: Path | None = None,
    ) -> dict[str, Any]:
        total_lines = _int_from_diagnostics(diagnostics, "total_lines")
        parsed = _int_from_diagnostics(diagnostics, "parsed_records")
        skipped = _int_from_diagnostics(diagnostics, "skipped_lines")
        error_types = {
            reason: int(entry["count"])
            for reason, entry in sorted(self._errors.items())
        }
        return {
            "environment": {
                "archscope_version": __version__,
                "python_version": platform.python_version(),
                "os": platform.platform(),
                "timestamp": datetime.now(timezone.utc).isoformat(),
            },
            "context": {
                "analyzer_type": self.analyzer_type,
                "source_file": redact_text(str(self.source_file)).text,
                "source_file_name": self.source_file.name,
                "file_size_bytes": _file_size(self.source_file),
                "encoding_detected": self.encoding_detected,
                "parser": self.parser,
                "parser_options": dict(self.parser_options),
                "debug_log_dir": str(
                    debug_log_dir or self.debug_log_dir or default_debug_log_dir()
                ),
            },
            "redaction": {
                "enabled": self.redaction_enabled,
                "redaction_version": REDACTION_VERSION,
                "raw_context_redacted": self.redaction_enabled,
                "summary": dict(sorted(self._redaction_summary.items())),
            },
            "summary": {
                "total_lines": total_lines,
                "parsed_ok": parsed,
                "skipped": skipped or sum(error_types.values()),
                "skip_rate_percent": _skip_rate(total_lines, skipped),
                "error_types": error_types,
                "exceptions": len(self._exceptions),
                "verdict": _verdict(total_lines, skipped, len(self._exceptions)),
            },
            "errors_by_type": {
                reason: _clean_error_entry(entry)
                for reason, entry in sorted(self._errors.items())
            },
            "exceptions": list(self._exceptions),
            "hints": _build_hints(total_lines, skipped, error_types, self._exceptions),
        }

    def _redact_context(self, raw_context: RawContext) -> RawContext:
        redacted: RawContext = {}
        summaries: list[dict[str, int]] = []
        for key in ("before", "target", "after"):
            value = raw_context.get(key)
            if value is None:
                redacted[key] = None
                continue
            clipped = value[:MAX_CONTEXT_CHARS]
            if self.redaction_enabled:
                result = redact_text(clipped)
                redacted[key] = result.text
                summaries.append(result.summary)
            else:
                redacted[key] = clipped
        self._redaction_summary = merge_redaction_summaries(
            self._redaction_summary,
            *summaries,
        )
        return redacted


def default_debug_log_dir() -> Path:
    return Path.cwd() / "archscope-debug"


def infer_field_shapes(text: str) -> dict[str, Any]:
    quote_count = text.count('"')
    shapes: dict[str, Any] = {
        "target_token_count": len(text.split()),
        "quote_count": quote_count,
        "bracket_count": text.count("[") + text.count("]"),
    }
    if " " in text and "=" in text.rsplit(" ", 1)[-1]:
        shapes["suffix_shape"] = "key=value"
    request = _extract_request_shape(text)
    if request:
        shapes.update(request)
    timestamp_shape = _timestamp_shape(text)
    if timestamp_shape:
        shapes["timestamp_shape"] = timestamp_shape
    return shapes


def _extract_request_shape(text: str) -> dict[str, Any] | None:
    import re

    match = re.search(r'"(?P<method>[A-Z]+)\s+(?P<path>\S+)\s+(?P<protocol>[^"]+)"', text)
    if not match:
        return None
    path = match.group("path")
    query_keys: list[str] = []
    if "?" in path:
        query = path.split("?", 1)[1]
        query_keys = [part.split("=", 1)[0] for part in query.split("&") if part]
    return {
        "request_shape": (
            "METHOD PATH_WITH_QUERY PROTOCOL" if query_keys else "METHOD PATH PROTOCOL"
        ),
        "path_shape": _path_shape(path.split("?", 1)[0]),
        "query_keys": query_keys,
    }


def _path_shape(path: str) -> str:
    import re

    return re.sub(r"/\d+", "/<NUMBER>", path)


def _timestamp_shape(text: str) -> str | None:
    import re

    if re.search(r"\[\d{2}/[A-Za-z]{3}/\d{4}:\d{2}:\d{2}:\d{2} [+-]\d{4}\]", text):
        return "dd/Mon/yyyy:HH:mm:ss Z"
    if re.search(r"\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\]", text):
        return "yyyy-MM-dd HH:mm:ss"
    if re.search(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}", text):
        return "ISO-8601"
    return None


def _write_size_limited_json(path: Path, payload: dict[str, Any]) -> None:
    while True:
        encoded = json.dumps(payload, ensure_ascii=False, indent=2).encode("utf-8")
        if len(encoded) <= MAX_DEBUG_LOG_BYTES:
            path.write_bytes(encoded)
            return
        if _remove_oldest_sample(payload):
            continue
        _shrink_payload(payload)
        encoded = json.dumps(payload, ensure_ascii=False, indent=2).encode("utf-8")
        if len(encoded) <= MAX_DEBUG_LOG_BYTES:
            path.write_bytes(encoded)
            return
        path.write_text(
            json.dumps(_minimal_payload(payload), ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
        return


def _remove_oldest_sample(payload: dict[str, Any]) -> bool:
    errors = payload.get("errors_by_type")
    if not isinstance(errors, dict):
        return False
    for entry in errors.values():
        if isinstance(entry, dict):
            samples = entry.get("samples")
            if isinstance(samples, list) and samples:
                samples.pop(0)
                return True
    return False


def _shrink_payload(payload: dict[str, Any]) -> None:
    payload["truncated"] = True
    for exception in payload.get("exceptions", []):
        if isinstance(exception, dict) and isinstance(exception.get("traceback"), str):
            exception["traceback"] = exception["traceback"][:2000]


def _minimal_payload(payload: dict[str, Any]) -> dict[str, Any]:
    return {
        "environment": payload.get("environment", {}),
        "context": payload.get("context", {}),
        "redaction": payload.get("redaction", {}),
        "summary": payload.get("summary", {}),
        "truncated": True,
        "hints": ["Debug log exceeded size cap; samples were removed."],
    }


def _int_from_diagnostics(diagnostics: dict[str, Any] | None, key: str) -> int:
    if not diagnostics:
        return 0
    value = diagnostics.get(key)
    return value if isinstance(value, int) else 0


def _skip_rate(total_lines: int, skipped: int) -> float:
    if total_lines <= 0:
        return 0.0
    return round(skipped / total_lines * 100, 2)


def _verdict(total_lines: int, skipped: int, exceptions: int) -> str:
    if exceptions > 0:
        return "FATAL_ERROR"
    if skipped <= 0:
        return "CLEAN"
    if total_lines > 0 and skipped / total_lines >= 0.5:
        return "MAJORITY_FAILED"
    return "PARTIAL_SUCCESS"


def _build_hints(
    total_lines: int,
    skipped: int,
    error_types: dict[str, int],
    exceptions: list[dict[str, Any]],
) -> list[str]:
    hints: list[str] = []
    total_errors = sum(error_types.values())
    if total_errors:
        no_format = error_types.get("NO_FORMAT_MATCH", 0)
        if no_format / total_errors >= 0.8:
            hints.append(
                "NO_FORMAT_MATCH dominates. The input may not match the selected parser format."
            )
        if error_types.get("INVALID_TIMESTAMP", 0) > 0:
            hints.append(
                "INVALID_TIMESTAMP is present. Check timestamp shape and parser time format."
            )
    if total_lines > 0 and skipped / total_lines >= 0.5:
        hints.append(
            "More than half of parsed lines failed. The file may use an unsupported format."
        )
    if exceptions:
        hints.append("Parser or analyzer exception captured. Inspect traceback and phase.")
    return hints


def _clean_error_entry(entry: dict[str, Any]) -> dict[str, Any]:
    cleaned = {
        "count": entry["count"],
        "description": entry.get("description"),
        "samples": entry.get("samples", []),
    }
    if entry.get("failed_pattern"):
        cleaned["failed_pattern"] = entry["failed_pattern"]
    return cleaned


def _file_size(path: Path) -> int | None:
    try:
        return path.stat().st_size
    except OSError:
        return None
