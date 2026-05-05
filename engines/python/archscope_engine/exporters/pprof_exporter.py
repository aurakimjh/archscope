"""Minimal pprof (``profile.proto``) encoder.

The pprof format is a Google-defined protobuf wire format used by ``go tool
pprof``, Pyroscope, Speedscope, and async-profiler. We avoid taking a
``protobuf`` runtime dependency by hand-rolling the bare subset we need.

Spec reference: ``profile.proto`` from
https://github.com/google/pprof/blob/main/proto/profile.proto

Wire-format primer:
- Each field has a tag = (field_number << 3) | wire_type.
- We only use:
    * wire_type 0 — varint (int64, bool, enum)
    * wire_type 2 — length-delimited (string, bytes, embedded message)
- Varints are LEB128 encoded.

The encoder produces a byte string; callers may gzip it (``.pb.gz`` is the
canonical extension) — pprof's CLI auto-detects gzip.
"""
from __future__ import annotations

import gzip
import io
from dataclasses import dataclass, field
from typing import Iterable

from archscope_engine.models.flamegraph import FlameNode

# ── Wire helpers ─────────────────────────────────────────────────────────────


def _varint(value: int) -> bytes:
    """LEB128 encoding for unsigned varints (negatives become huge — only OK
    for the int64 fields we use, which are always non-negative)."""
    out = bytearray()
    if value < 0:
        # Two's-complement to 64 bits — pprof never asks for negatives but
        # be defensive in case future call sites do.
        value &= 0xFFFFFFFFFFFFFFFF
    while value > 0x7F:
        out.append((value & 0x7F) | 0x80)
        value >>= 7
    out.append(value & 0x7F)
    return bytes(out)


def _tag(field_no: int, wire_type: int) -> bytes:
    return _varint((field_no << 3) | wire_type)


def _len_delim(field_no: int, payload: bytes) -> bytes:
    return _tag(field_no, 2) + _varint(len(payload)) + payload


def _field_varint(field_no: int, value: int) -> bytes:
    return _tag(field_no, 0) + _varint(value)


def _field_string(field_no: int, value: str) -> bytes:
    return _len_delim(field_no, value.encode("utf-8"))


def _field_message(field_no: int, payload: bytes) -> bytes:
    return _len_delim(field_no, payload)


def _packed_varints(field_no: int, values: Iterable[int]) -> bytes:
    """``location_id`` in Sample is packed-repeated."""
    body = b"".join(_varint(v) for v in values)
    return _len_delim(field_no, body)


# ── Profile builder ──────────────────────────────────────────────────────────


@dataclass
class _Builder:
    """Accumulates ids while building the profile."""

    string_table: list[str] = field(default_factory=lambda: [""])
    string_index: dict[str, int] = field(default_factory=dict)
    function_index: dict[str, int] = field(default_factory=dict)
    functions: list[bytes] = field(default_factory=list)
    location_index: dict[int, int] = field(default_factory=dict)
    locations: list[bytes] = field(default_factory=list)

    def __post_init__(self) -> None:
        self.string_index[""] = 0

    def intern_string(self, value: str) -> int:
        idx = self.string_index.get(value)
        if idx is not None:
            return idx
        idx = len(self.string_table)
        self.string_table.append(value)
        self.string_index[value] = idx
        return idx

    def function_id(self, name: str) -> int:
        existing = self.function_index.get(name)
        if existing is not None:
            return existing
        fn_id = len(self.function_index) + 1
        name_idx = self.intern_string(name)
        # Function { id=1, name=2, system_name=3, filename=4, start_line=5 }
        body = (
            _field_varint(1, fn_id)
            + _field_varint(2, name_idx)
            + _field_varint(3, name_idx)
        )
        self.functions.append(body)
        self.function_index[name] = fn_id
        return fn_id

    def location_id(self, function_id: int) -> int:
        existing = self.location_index.get(function_id)
        if existing is not None:
            return existing
        loc_id = len(self.location_index) + 1
        # Line { function_id=1, line=2 }
        line = _field_varint(1, function_id)
        # Location { id=1, mapping_id=2, address=3, line=4 (repeated) }
        body = (
            _field_varint(1, loc_id)
            + _field_message(4, line)
        )
        self.locations.append(body)
        self.location_index[function_id] = loc_id
        return loc_id


def encode_pprof(
    flame_root: FlameNode,
    *,
    sample_type: str = "samples",
    sample_unit: str = "count",
    duration_ns: int = 0,
) -> bytes:
    """Serialize a flame tree as a pprof binary message.

    We walk leaf paths so each Sample corresponds to one unique stack with
    its exclusive sample count. Internal nodes contribute only when they
    have positive exclusive samples (matches the flame-tree builder's
    invariants for both collapsed and Jennifer CSV inputs).
    """
    builder = _Builder()
    samples_payload: list[bytes] = []

    sample_type_idx = builder.intern_string(sample_type)
    sample_unit_idx = builder.intern_string(sample_unit)

    for path, count in _iter_leaf_paths(flame_root):
        if count <= 0 or not path:
            continue
        # Sample.location_id is leaf-first per pprof convention.
        location_ids: list[int] = []
        for frame in reversed(path):
            fn_id = builder.function_id(frame)
            loc_id = builder.location_id(fn_id)
            location_ids.append(loc_id)
        sample_body = _packed_varints(1, location_ids) + _packed_varints(2, [count])
        samples_payload.append(_field_message(2, sample_body))

    # ValueType { type=1 (string idx), unit=2 (string idx) }
    sample_type_msg = (
        _field_varint(1, sample_type_idx) + _field_varint(2, sample_unit_idx)
    )

    out = io.BytesIO()
    out.write(_field_message(1, sample_type_msg))
    for s in samples_payload:
        out.write(s)
    for loc in builder.locations:
        out.write(_field_message(4, loc))
    for fn in builder.functions:
        out.write(_field_message(5, fn))
    for s in builder.string_table:
        out.write(_field_string(6, s))
    out.write(_field_varint(9, 0))  # time_nanos (unset)
    out.write(_field_varint(10, max(0, duration_ns)))  # duration_nanos

    return out.getvalue()


def encode_pprof_gzipped(flame_root: FlameNode, **kwargs: object) -> bytes:
    """Convenience helper that gzips the encoded pprof payload."""
    raw = encode_pprof(flame_root, **kwargs)  # type: ignore[arg-type]
    return gzip.compress(raw)


def _iter_leaf_paths(node: FlameNode) -> Iterable[tuple[list[str], int]]:
    """Yield (path, exclusive_samples) per node with positive exclusive
    samples — matches ``flamegraph_builder.iter_leaf_paths`` semantics so
    both collapsed and Jennifer-style inputs round-trip cleanly."""
    stack = [node]
    while stack:
        cur = stack.pop()
        child_total = sum(child.samples for child in cur.children)
        exclusive = cur.samples - child_total
        if cur.path and exclusive > 0:
            yield cur.path, exclusive
        if not cur.children and cur.path and cur.samples > 0 and exclusive <= 0:
            yield cur.path, cur.samples
        for child in reversed(cur.children):
            stack.append(child)
