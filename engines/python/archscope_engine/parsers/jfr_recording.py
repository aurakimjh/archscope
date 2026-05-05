"""High-level JFR ingestion that accepts both binary `.jfr` and the JSON
output of ``jfr print --json``.

This module sits *above* :mod:`archscope_engine.parsers.jfr_parser` (which only
understands the JSON shape) and dispatches based on the file's magic bytes.

Binary JFR support strategy
---------------------------

A from-scratch Python parser of the JFR binary format is non-trivial — the
format has a chunked layout with constant pools, type metadata, and packed
varints. Until we ship a native Python reader, we shell out to the JDK's
``jfr`` CLI (which every JDK 11+ installation provides). Detection priority:

1. ``ARCHSCOPE_JFR_CLI`` env var (explicit override).
2. ``jfr`` on ``PATH``.
3. ``$JAVA_HOME/bin/jfr`` if ``JAVA_HOME`` is set.

When no CLI is found we raise a structured ``JfrCliMissingError`` so the API
layer can surface a clear instruction to the user instead of a stack trace.
"""
from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
from pathlib import Path

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.parsers.jfr_parser import JfrEvent, parse_jfr_print_json

# JFR chunk magic — first 4 bytes of any binary recording.
JFR_MAGIC = b"FLR\x00"


class JfrCliMissingError(RuntimeError):
    """Raised when a binary `.jfr` file is provided but no `jfr` CLI is
    discoverable on the host. The web layer surfaces a structured error
    with installation hints instead of bubbling this up.
    """


def is_binary_jfr(path: Path) -> bool:
    """Return True if *path* starts with the JFR magic bytes."""
    try:
        with path.open("rb") as fh:
            return fh.read(4) == JFR_MAGIC
    except OSError:
        return False


def discover_jfr_cli() -> str | None:
    """Return an absolute path to a `jfr` executable, or None if not found."""
    explicit = os.environ.get("ARCHSCOPE_JFR_CLI")
    if explicit and Path(explicit).is_file():
        return explicit

    on_path = shutil.which("jfr")
    if on_path:
        return on_path

    java_home = os.environ.get("JAVA_HOME")
    if java_home:
        candidate = Path(java_home) / "bin" / ("jfr.exe" if os.name == "nt" else "jfr")
        if candidate.is_file():
            return str(candidate)
    return None


def convert_jfr_to_json(jfr_path: Path, *, cli: str | None = None) -> Path:
    """Run ``jfr print --json <path>`` and return a temporary JSON path.

    The temp file lives under the system temp dir; callers are responsible
    for deleting it (we do not auto-clean so the FastAPI layer can stash it
    for inspection if anything goes wrong).
    """
    resolved = cli or discover_jfr_cli()
    if resolved is None:
        raise JfrCliMissingError(
            "No `jfr` CLI is available on PATH or under JAVA_HOME. Install a "
            "JDK 11+ (or set ARCHSCOPE_JFR_CLI) so binary .jfr recordings can "
            "be converted to JSON."
        )

    out_fd, out_name = tempfile.mkstemp(prefix="archscope_jfr_", suffix=".json")
    os.close(out_fd)
    out_path = Path(out_name)

    try:
        completed = subprocess.run(
            [resolved, "print", "--json", str(jfr_path)],
            capture_output=True,
            check=False,
        )
    except OSError as exc:
        out_path.unlink(missing_ok=True)
        raise JfrCliMissingError(f"Failed to invoke jfr CLI: {exc}") from exc

    if completed.returncode != 0:
        stderr = completed.stderr.decode("utf-8", errors="replace").strip()
        out_path.unlink(missing_ok=True)
        raise RuntimeError(
            f"jfr print --json failed (exit {completed.returncode}): "
            f"{stderr or 'no stderr output'}"
        )

    out_path.write_bytes(completed.stdout)
    return out_path


def parse_jfr_recording(
    path: Path,
    *,
    diagnostics: ParserDiagnostics | None = None,
    debug_log: DebugLogCollector | None = None,
) -> tuple[list[JfrEvent], dict[str, object]]:
    """Parse a binary `.jfr` or JSON file and return ``(events, source_info)``.

    ``source_info`` captures provenance for the analyzer to expose in
    ``metadata`` — whether the input was already JSON, which CLI was used
    for binary→JSON conversion, etc.
    """
    own_diagnostics = diagnostics or ParserDiagnostics()
    info: dict[str, object] = {"source_format": "json"}

    if is_binary_jfr(path):
        cli = discover_jfr_cli()
        info["source_format"] = "binary_jfr"
        info["jfr_cli"] = cli
        if cli is None:
            raise JfrCliMissingError(
                "Binary .jfr files require a JDK `jfr` CLI to convert to JSON. "
                "Install JDK 11+, set ARCHSCOPE_JFR_CLI, or pre-convert with "
                "`jfr print --json recording.jfr > recording.json`."
            )
        json_path = convert_jfr_to_json(path, cli=cli)
        try:
            events = parse_jfr_print_json(
                json_path, diagnostics=own_diagnostics, debug_log=debug_log
            )
        finally:
            json_path.unlink(missing_ok=True)
        return events, info

    events = parse_jfr_print_json(
        path, diagnostics=own_diagnostics, debug_log=debug_log
    )
    return events, info
