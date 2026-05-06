"""FastAPI server that exposes the ArchScope engine over HTTP.

Phase 1 of the Electron→Web pivot. The endpoints mirror the IPC contract that
``apps/desktop/electron/main.ts`` (now in ``apps/desktop``) previously implemented so the React frontend
can keep using the same shapes via an HTTP bridge instead of ``window.archscope``.
"""
from __future__ import annotations

import json
import os
import shutil
import uuid
from collections import Counter
from datetime import datetime
from pathlib import Path
from typing import Any, Callable, Optional

from fastapi import FastAPI, File, HTTPException, Query, Request, UploadFile, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, JSONResponse, RedirectResponse
from fastapi.staticfiles import StaticFiles

from archscope_engine.web.progress import progress_registry
from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.analyzers.exception_analyzer import analyze_exception_stack
from archscope_engine.analyzers.gc_log_analyzer import analyze_gc_log
from archscope_engine.analyzers.jfr_analyzer import analyze_jfr_print_json
from archscope_engine.analyzers.lock_contention_analyzer import analyze_lock_contention
from archscope_engine.analyzers.multi_thread_analyzer import analyze_multi_thread_dumps
from archscope_engine.analyzers.thread_dump_to_collapsed import write_collapsed_file
from archscope_engine.analyzers.profiler_analyzer import (
    analyze_collapsed_profile,
    analyze_flamegraph_html_profile,
    analyze_flamegraph_svg_profile,
    analyze_jennifer_csv_profile,
)
from archscope_engine.analyzers.native_memory_analyzer import analyze_native_memory
from archscope_engine.analyzers.profiler_diff import analyze_profiler_diff
from archscope_engine.analyzers.flamegraph_builder import build_flame_tree_from_collapsed
from archscope_engine.parsers.collapsed_parser import parse_collapsed_file_with_diagnostics
from archscope_engine.parsers.html_profiler_parser import parse_html_profiler
from archscope_engine.parsers.jennifer_csv_parser import parse_jennifer_flamegraph_csv
from archscope_engine.parsers.svg_flamegraph_parser import parse_svg_flamegraph
from archscope_engine.analyzers.thread_dump_analyzer import analyze_thread_dump
from archscope_engine.parsers.thread_dump import (
    DEFAULT_REGISTRY as THREAD_DUMP_REGISTRY,
    MixedFormatError,
    UnknownFormatError,
)
from archscope_engine.demo_site_runner import (
    discover_demo_manifests,
    run_demo_site_manifest,
)
from archscope_engine.exporters.html_exporter import render_html_report, write_html_report
from archscope_engine.exporters.json_exporter import write_json_result
from archscope_engine.exporters.pprof_exporter import encode_pprof_gzipped
from archscope_engine.exporters.pptx_exporter import write_pptx_report
from archscope_engine.exporters.report_diff import build_comparison_report
from archscope_engine.models.analysis_result import AnalysisResult


# ---------------------------------------------------------------------------
# Storage helpers
# ---------------------------------------------------------------------------

DEFAULT_SETTINGS: dict[str, Any] = {
    "enginePath": "",
    "chartTheme": "light",
    "locale": "en",
}


def archscope_home() -> Path:
    return Path.home() / ".archscope"


def upload_root() -> Path:
    return archscope_home() / "uploads"


def settings_path() -> Path:
    return archscope_home() / "settings.json"


def load_settings() -> dict[str, Any]:
    try:
        raw = json.loads(settings_path().read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {**DEFAULT_SETTINGS}
    return {**DEFAULT_SETTINGS, **(raw if isinstance(raw, dict) else {})}


def save_settings(settings: dict[str, Any]) -> None:
    archscope_home().mkdir(parents=True, exist_ok=True)
    settings_path().write_text(
        json.dumps(settings, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )


# ---------------------------------------------------------------------------
# Response helpers
# ---------------------------------------------------------------------------


def _stacks_for_format(path: Path, fmt: str) -> Counter[str]:
    """Load the file at *path* using the parser implied by *fmt* and return
    collapsed-style stack counts. Shared between the diff and pprof export
    routes."""
    if fmt == "collapsed":
        return parse_collapsed_file_with_diagnostics(path).stacks
    if fmt == "flamegraph_svg":
        return _stacks_from_paths(parse_svg_flamegraph(path).stacks)
    if fmt == "flamegraph_html":
        return _stacks_from_paths(parse_html_profiler(path).stacks)
    if fmt == "jennifer_csv":
        return _stacks_from_flamegraph(parse_jennifer_flamegraph_csv(path).root)
    raise ValueError(f"Unsupported profile format: {fmt!r}")


def _stacks_from_paths(paths: list[tuple[list[str], int]]) -> Counter[str]:
    """Aggregate ``[(path_frames, samples), ...]`` into a collapsed-style
    ``Counter[stack_string]`` so the diff analyzer can consume any input
    format uniformly."""
    out: Counter[str] = Counter()
    for path, samples in paths:
        if not path or samples <= 0:
            continue
        out[";".join(path)] += int(samples)
    return out


def _stacks_from_flamegraph(root: Any) -> Counter[str]:
    """Walk a FlameNode tree and emit one (path, exclusive_samples) row
    per leaf so it can be diffed."""
    from archscope_engine.analyzers.flamegraph_builder import iter_leaf_paths

    out: Counter[str] = Counter()
    for path, samples in iter_leaf_paths(root):
        if not path or samples <= 0:
            continue
        out[";".join(path)] += int(samples)
    return out


def _failure(code: str, message: str, detail: str | None = None) -> dict[str, Any]:
    err: dict[str, Any] = {"code": code, "message": message}
    if detail is not None:
        err["detail"] = detail
    return {"ok": False, "error": err}


def _require_existing_file(value: Optional[str], label: str):
    if not isinstance(value, str) or not value:
        return None, _failure("INVALID_OPTION", f"{label} is required.")
    path = Path(value)
    if not path.is_file():
        return None, _failure("FILE_NOT_FOUND", f"{label} is not readable.", str(path))
    return path, None


def _parse_optional_dt(value: Optional[str], name: str) -> Optional[datetime]:
    if value is None or value == "":
        return None
    try:
        return datetime.fromisoformat(value)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=f"{name} must be ISO 8601: {exc}")


def _wrap_analyzer(thunk: Callable[[], AnalysisResult]) -> dict[str, Any]:
    try:
        result = thunk()
    except FileNotFoundError as exc:
        return _failure("FILE_NOT_FOUND", "Analyzer source file not found.", str(exc))
    except ValueError as exc:
        return _failure("INVALID_OPTION", str(exc))
    except Exception as exc:  # noqa: BLE001 - bubble engine errors as structured failures
        return _failure("ENGINE_FAILED", "Analyzer execution failed.", repr(exc))
    return {"ok": True, "result": result.to_dict()}


# ---------------------------------------------------------------------------
# Static serving
# ---------------------------------------------------------------------------


def _resolve_static_dir(explicit: Optional[Path]) -> Optional[Path]:
    """Locate the built React bundle for the FastAPI server to serve.

    Resolution order:

    1. Explicit ``--static-dir`` argument from the CLI / programmatic call.
    2. ``ARCHSCOPE_STATIC_DIR`` env var (set by reload mode).
    3. **Wheel-bundled assets** at ``archscope_engine/web/static/`` —
       this is what T-208 ships in the ``archscope`` wheel via the
       ``[options.package_data]`` entry. Resolved through
       ``importlib.resources`` so it works inside zipapps too.
    4. Dev-tree fallback ``apps/frontend/dist`` from the repo root or
       the current working directory.
    """
    if explicit is not None:
        return explicit if explicit.is_dir() else None

    env_value = os.environ.get("ARCHSCOPE_STATIC_DIR")
    if env_value:
        env_path = Path(env_value)
        if env_path.is_dir():
            return env_path

    bundled = _wheel_bundled_static()
    if bundled is not None:
        return bundled

    here = Path(__file__).resolve()
    # engines/python/archscope_engine/web/server.py → repo root is parents[4]
    candidates = [
        here.parents[4] / "apps" / "frontend" / "dist",
        Path.cwd() / "apps" / "frontend" / "dist",
    ]
    for candidate in candidates:
        if candidate.is_dir():
            return candidate
    return None


def _wheel_bundled_static() -> Optional[Path]:
    """Return the wheel-bundled ``static`` directory if it ships any HTML."""
    try:
        from importlib.resources import files as _resource_files
    except ImportError:  # pragma: no cover — Python < 3.9
        return None
    try:
        bundle = _resource_files("archscope_engine.web").joinpath("static")
    except (ModuleNotFoundError, FileNotFoundError):
        return None
    try:
        index_html = bundle.joinpath("index.html")
        if not index_html.is_file():
            return None
        path_str = str(bundle)
    except (FileNotFoundError, AttributeError):
        return None
    candidate = Path(path_str)
    return candidate if candidate.is_dir() else None


# ---------------------------------------------------------------------------
# Analyzer dispatch
# ---------------------------------------------------------------------------


def _execute_analyzer(payload: dict[str, Any]) -> dict[str, Any]:
    request_type = payload.get("type")
    params = payload.get("params") or {}

    if request_type == "access_log":
        path, err = _require_existing_file(params.get("filePath"), "Access log file")
        if err:
            return err
        log_format = params.get("format")
        if not isinstance(log_format, str) or not log_format:
            return _failure("INVALID_OPTION", "Access log format is required.")
        max_lines = params.get("maxLines")
        if max_lines is not None and (not isinstance(max_lines, int) or max_lines <= 0):
            return _failure("INVALID_OPTION", "Max lines must be a positive integer.")
        return _wrap_analyzer(
            lambda: analyze_access_log(
                path,
                log_format=log_format,
                max_lines=max_lines,
                start_time=_parse_optional_dt(params.get("startTime"), "startTime"),
                end_time=_parse_optional_dt(params.get("endTime"), "endTime"),
            )
        )

    if request_type == "profiler_collapsed":
        wall_path, err = _require_existing_file(params.get("wallPath"), "Wall collapsed file")
        if err:
            return err
        interval_ms = params.get("wallIntervalMs")
        if not isinstance(interval_ms, (int, float)) or interval_ms <= 0:
            return _failure("INVALID_OPTION", "Wall interval must be positive.")
        elapsed = params.get("elapsedSec")
        top_n = params.get("topN") or 20
        profile_format = params.get("profileFormat") or "collapsed"
        elapsed_arg = elapsed if isinstance(elapsed, (int, float)) else None
        profile_kind = params.get("profileKind") or "wall"
        if profile_kind not in {"wall", "cpu", "lock"}:
            return _failure("INVALID_OPTION", "profileKind must be wall/cpu/lock.")
        timeline_base_raw = params.get("timelineBaseMethod")
        if timeline_base_raw is not None and not isinstance(timeline_base_raw, str):
            return _failure("INVALID_OPTION", "timelineBaseMethod must be a string.")
        timeline_base_method = (
            timeline_base_raw.strip()
            if isinstance(timeline_base_raw, str) and timeline_base_raw.strip()
            else None
        )

        if profile_format == "jennifer_csv":
            return _wrap_analyzer(
                lambda: analyze_jennifer_csv_profile(
                    path=wall_path,
                    interval_ms=float(interval_ms),
                    elapsed_sec=elapsed_arg,
                    top_n=int(top_n),
                    timeline_base_method=timeline_base_method,
                )
            )
        if profile_format == "flamegraph_svg":
            return _wrap_analyzer(
                lambda: analyze_flamegraph_svg_profile(
                    path=wall_path,
                    interval_ms=float(interval_ms),
                    elapsed_sec=elapsed_arg,
                    top_n=int(top_n),
                    profile_kind=profile_kind,
                    timeline_base_method=timeline_base_method,
                )
            )
        if profile_format == "flamegraph_html":
            return _wrap_analyzer(
                lambda: analyze_flamegraph_html_profile(
                    path=wall_path,
                    interval_ms=float(interval_ms),
                    elapsed_sec=elapsed_arg,
                    top_n=int(top_n),
                    profile_kind=profile_kind,
                    timeline_base_method=timeline_base_method,
                )
            )
        return _wrap_analyzer(
            lambda: analyze_collapsed_profile(
                path=wall_path,
                interval_ms=float(interval_ms),
                elapsed_sec=elapsed_arg,
                top_n=int(top_n),
                profile_kind=profile_kind,
                timeline_base_method=timeline_base_method,
            )
        )

    if request_type == "native_memory":
        path, err = _require_existing_file(params.get("filePath"), "JFR file (.jfr or .json)")
        if err:
            return err
        leak_only = params.get("leakOnly")
        tail_raw = params.get("tailRatio")
        tail_ratio = float(tail_raw) if isinstance(tail_raw, (int, float)) else 0.10
        return _wrap_analyzer(
            lambda: analyze_native_memory(
                path=path,
                leak_only=bool(leak_only) if leak_only is not None else True,
                tail_ratio=tail_ratio,
            )
        )

    if request_type == "profiler_diff":
        baseline_path, err = _require_existing_file(
            params.get("baselinePath"), "Baseline profile file"
        )
        if err:
            return err
        target_path, err = _require_existing_file(
            params.get("targetPath"), "Target profile file"
        )
        if err:
            return err
        baseline_format = params.get("baselineFormat") or "collapsed"
        target_format = params.get("targetFormat") or "collapsed"
        normalize = params.get("normalize")
        normalize_arg = bool(normalize) if normalize is not None else True

        def _load(path: Path, fmt: str) -> Counter[str]:
            if fmt == "collapsed":
                return parse_collapsed_file_with_diagnostics(path).stacks
            if fmt == "flamegraph_svg":
                return _stacks_from_paths(parse_svg_flamegraph(path).stacks)
            if fmt == "flamegraph_html":
                return _stacks_from_paths(parse_html_profiler(path).stacks)
            if fmt == "jennifer_csv":
                return _stacks_from_flamegraph(parse_jennifer_flamegraph_csv(path).root)
            raise ValueError(f"Unsupported profile format for diff: {fmt!r}")

        return _wrap_analyzer(
            lambda: analyze_profiler_diff(
                _load(baseline_path, baseline_format),
                _load(target_path, target_format),
                baseline_path=baseline_path,
                target_path=target_path,
                normalize=normalize_arg,
            )
        )

    if request_type == "gc_log":
        path, err = _require_existing_file(params.get("filePath"), "GC log file")
        if err:
            return err
        return _wrap_analyzer(
            lambda: analyze_gc_log(path=path, top_n=int(params.get("topN") or 20))
        )

    if request_type == "thread_dump":
        path, err = _require_existing_file(params.get("filePath"), "Thread dump file")
        if err:
            return err
        return _wrap_analyzer(
            lambda: analyze_thread_dump(path=path, top_n=int(params.get("topN") or 20))
        )

    if request_type == "thread_dump_multi":
        return _execute_thread_dump_multi(params)

    if request_type == "thread_dump_to_collapsed":
        return _execute_thread_dump_to_collapsed(params)

    if request_type == "thread_dump_locks":
        return _execute_thread_dump_locks(params)

    if request_type == "exception_stack":
        path, err = _require_existing_file(params.get("filePath"), "Exception stack file")
        if err:
            return err
        return _wrap_analyzer(
            lambda: analyze_exception_stack(path=path, top_n=int(params.get("topN") or 20))
        )

    if request_type == "jfr_recording":
        path, err = _require_existing_file(params.get("filePath"), "JFR file (.jfr or .json)")
        if err:
            return err
        mode_param = params.get("mode") or "all"
        if not isinstance(mode_param, str):
            return _failure("INVALID_OPTION", "mode must be a string.")
        from_time = params.get("fromTime")
        to_time = params.get("toTime")
        if from_time is not None and not isinstance(from_time, str):
            return _failure("INVALID_OPTION", "fromTime must be a string.")
        if to_time is not None and not isinstance(to_time, str):
            return _failure("INVALID_OPTION", "toTime must be a string.")
        state_param = params.get("state")
        if state_param is not None and not isinstance(state_param, str):
            return _failure("INVALID_OPTION", "state must be a string.")
        min_duration_raw = params.get("minDurationMs")
        min_duration: float | None = None
        if min_duration_raw is not None:
            if not isinstance(min_duration_raw, (int, float)):
                return _failure("INVALID_OPTION", "minDurationMs must be a number.")
            min_duration = float(min_duration_raw)
        return _wrap_analyzer(
            lambda: analyze_jfr_print_json(
                path=path,
                top_n=int(params.get("topN") or 20),
                mode=mode_param,
                from_time=from_time or None,
                to_time=to_time or None,
                state=state_param or None,
                min_duration_ms=min_duration,
            )
        )

    return _failure("INVALID_OPTION", f"Unsupported analyzer type: {request_type!r}.")


def _execute_thread_dump_to_collapsed(params: dict[str, Any]) -> dict[str, Any]:
    raw_paths = params.get("filePaths")
    if not isinstance(raw_paths, list) or not raw_paths:
        return _failure(
            "INVALID_OPTION",
            "Convert request requires a non-empty 'filePaths' array.",
        )
    paths: list[Path] = []
    for entry in raw_paths:
        if not isinstance(entry, str) or not entry:
            return _failure("INVALID_OPTION", "Every filePaths entry must be a string.")
        candidate = Path(entry)
        if not candidate.is_file():
            return _failure(
                "FILE_NOT_FOUND", "Thread-dump file is not readable.", str(candidate)
            )
        paths.append(candidate)

    format_override = params.get("format")
    if format_override is not None and not isinstance(format_override, str):
        return _failure("INVALID_OPTION", "format override must be a string when set.")
    include_thread_name = bool(params.get("includeThreadName", True))

    output_dir = upload_root() / "collapsed"
    output_dir.mkdir(parents=True, exist_ok=True)
    target = output_dir / f"thread-dump-{uuid.uuid4().hex}.collapsed"

    try:
        written, unique_stacks = write_collapsed_file(
            paths,
            target,
            format_override=format_override or None,
            include_thread_name=include_thread_name,
        )
    except UnknownFormatError as exc:
        return _failure("UNKNOWN_THREAD_DUMP_FORMAT", str(exc), exc.head_preview[:200])
    except MixedFormatError as exc:
        return _failure("MIXED_THREAD_DUMP_FORMATS", str(exc))
    except Exception as exc:  # noqa: BLE001
        return _failure("ENGINE_FAILED", "Conversion failed.", repr(exc))

    return {
        "ok": True,
        "result": {
            "outputPath": str(written),
            "uniqueStacks": unique_stacks,
            "inputCount": len(paths),
        },
    }


def _execute_thread_dump_locks(params: dict[str, Any]) -> dict[str, Any]:
    raw_paths = params.get("filePaths")
    if not isinstance(raw_paths, list) or not raw_paths:
        return _failure(
            "INVALID_OPTION",
            "Lock contention request requires a non-empty 'filePaths' array.",
        )
    paths: list[Path] = []
    for entry in raw_paths:
        if not isinstance(entry, str) or not entry:
            return _failure("INVALID_OPTION", "Every filePaths entry must be a string.")
        candidate = Path(entry)
        if not candidate.is_file():
            return _failure(
                "FILE_NOT_FOUND", "Thread-dump file is not readable.", str(candidate)
            )
        paths.append(candidate)

    top_n = int(params.get("topN") or 20)
    format_override = params.get("format")
    if format_override is not None and not isinstance(format_override, str):
        return _failure("INVALID_OPTION", "format override must be a string when set.")

    try:
        bundles = THREAD_DUMP_REGISTRY.parse_many(
            paths, format_override=format_override or None
        )
    except UnknownFormatError as exc:
        return _failure("UNKNOWN_THREAD_DUMP_FORMAT", str(exc), exc.head_preview[:200])
    except MixedFormatError as exc:
        return _failure("MIXED_THREAD_DUMP_FORMATS", str(exc))
    except Exception as exc:  # noqa: BLE001
        return _failure("ENGINE_FAILED", "Lock contention parsing failed.", repr(exc))

    return _wrap_analyzer(lambda: analyze_lock_contention(bundles, top_n=top_n))


def _execute_thread_dump_multi(params: dict[str, Any]) -> dict[str, Any]:
    raw_paths = params.get("filePaths")
    if not isinstance(raw_paths, list) or not raw_paths:
        return _failure(
            "INVALID_OPTION",
            "Multi-dump request requires a non-empty 'filePaths' array.",
        )
    paths: list[Path] = []
    for entry in raw_paths:
        if not isinstance(entry, str) or not entry:
            return _failure("INVALID_OPTION", "Every filePaths entry must be a string.")
        candidate = Path(entry)
        if not candidate.is_file():
            return _failure(
                "FILE_NOT_FOUND", "Thread-dump file is not readable.", str(candidate)
            )
        paths.append(candidate)

    top_n = int(params.get("topN") or 20)
    class_histogram_limit = int(params.get("classHistogramLimit") or 500)
    if class_histogram_limit < 1:
        return _failure("INVALID_OPTION", "classHistogramLimit must be >= 1.")
    threshold = int(params.get("consecutiveThreshold") or 3)
    if threshold < 1:
        return _failure("INVALID_OPTION", "consecutiveThreshold must be >= 1.")
    format_override = params.get("format")
    if format_override is not None and not isinstance(format_override, str):
        return _failure("INVALID_OPTION", "format override must be a string when set.")

    try:
        bundles = THREAD_DUMP_REGISTRY.parse_many(
            paths,
            format_override=format_override or None,
            parser_options={"class_histogram_limit": class_histogram_limit},
        )
    except UnknownFormatError as exc:
        return _failure("UNKNOWN_THREAD_DUMP_FORMAT", str(exc), exc.head_preview[:200])
    except MixedFormatError as exc:
        return _failure("MIXED_THREAD_DUMP_FORMATS", str(exc))
    except Exception as exc:  # noqa: BLE001
        return _failure("ENGINE_FAILED", "Multi-dump parser failed.", repr(exc))

    return _wrap_analyzer(
        lambda: analyze_multi_thread_dumps(bundles, threshold=threshold, top_n=top_n)
    )


# ---------------------------------------------------------------------------
# Export dispatch
# ---------------------------------------------------------------------------


def _sibling_output(input_path: Path, ext: str) -> Path:
    return input_path.with_suffix(f".{ext}")


def _execute_export(payload: dict[str, Any]) -> dict[str, Any]:
    fmt = payload.get("format")
    title = payload.get("title")

    if fmt in {"html", "pptx"}:
        input_path, err = _require_existing_file(payload.get("inputPath"), "Input JSON file")
        if err:
            return err
        out_path = _sibling_output(input_path, fmt)
        try:
            if fmt == "html":
                write_html_report(input_path, out_path, title=title if isinstance(title, str) else None)
            else:
                write_pptx_report(input_path, out_path, title=title if isinstance(title, str) else None)
        except Exception as exc:  # noqa: BLE001
            return _failure("EXPORT_FAILED", f"{fmt.upper()} export failed.", repr(exc))
        return {"ok": True, "outputPaths": [str(out_path)]}

    if fmt == "diff":
        before, err = _require_existing_file(payload.get("beforePath"), "Before JSON file")
        if err:
            return err
        after, err = _require_existing_file(payload.get("afterPath"), "After JSON file")
        if err:
            return err
        label = payload.get("label")
        base_name = f"{before.stem}-vs-{after.stem}"
        out_dir = after.parent
        json_out = out_dir / f"{base_name}-diff.json"
        html_out = out_dir / f"{base_name}-diff.html"
        try:
            result = build_comparison_report(
                before, after, label=label if isinstance(label, str) else None
            )
            write_json_result(result, json_out)
            html_out.parent.mkdir(parents=True, exist_ok=True)
            html_out.write_text(
                render_html_report(result.to_dict(), source_path=json_out),
                encoding="utf-8",
            )
        except Exception as exc:  # noqa: BLE001
            return _failure("EXPORT_FAILED", "Diff export failed.", repr(exc))
        return {"ok": True, "outputPaths": [str(json_out), str(html_out)]}

    return _failure("INVALID_OPTION", f"Unsupported export format: {fmt!r}.")


# ---------------------------------------------------------------------------
# Demo dispatch
# ---------------------------------------------------------------------------


def _data_source_for_manifest(payload: dict[str, Any], manifest_path: Path) -> str:
    declared = payload.get("data_source")
    if declared in {"real", "synthetic"}:
        return str(declared)
    parts = manifest_path.parts
    if "real" in parts:
        return "real"
    if "synthetic" in parts:
        return "synthetic"
    return "unknown"


def _list_demo_scenarios(manifest_root: str) -> dict[str, Any]:
    if not manifest_root:
        return _failure("INVALID_OPTION", "Demo manifest root is required.")
    root = Path(manifest_root)
    if not root.exists():
        return _failure("FILE_NOT_FOUND", "Demo manifest root is not readable.", str(root))

    manifest_paths: list[Path] = []
    if root.is_file():
        manifest_paths = [root]
    else:
        for source_entry in sorted(p for p in root.iterdir() if p.is_dir()):
            for scenario_entry in sorted(p for p in source_entry.iterdir() if p.is_dir()):
                manifest_path = scenario_entry / "manifest.json"
                if manifest_path.is_file():
                    manifest_paths.append(manifest_path)

    scenarios: list[dict[str, Any]] = []
    for manifest_path in manifest_paths:
        try:
            payload = json.loads(manifest_path.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            continue
        files = payload.get("files") if isinstance(payload, dict) else None
        analyzers = []
        if isinstance(files, list):
            for item in files:
                if isinstance(item, dict) and isinstance(item.get("analyzer_type"), str):
                    analyzers.append(item["analyzer_type"])
        scenarios.append(
            {
                "scenario": (
                    payload.get("scenario")
                    if isinstance(payload, dict) and isinstance(payload.get("scenario"), str)
                    else manifest_path.parent.name
                ),
                "dataSource": _data_source_for_manifest(payload if isinstance(payload, dict) else {}, manifest_path),
                "manifestPath": str(manifest_path),
                "description": (
                    payload.get("description")
                    if isinstance(payload, dict) and isinstance(payload.get("description"), str)
                    else ""
                ),
                "analyzers": analyzers,
            }
        )

    scenarios.sort(key=lambda item: f"{item['dataSource']}/{item['scenario']}")
    return {"ok": True, "manifestRoot": manifest_root, "scenarios": scenarios}


def _read_run_summary(scenario_dir: Path, scenario_meta: dict[str, Any]) -> dict[str, Any]:
    summary_path = scenario_dir / "run-summary.json"
    bundle_index = scenario_dir / "index.html"
    try:
        payload = json.loads(summary_path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {
            "scenario": scenario_meta["scenario"],
            "dataSource": scenario_meta["dataSource"],
            "bundleIndexPath": str(bundle_index),
            "summaryPath": str(summary_path),
            "summary": {
                "analyzerOutputs": 0,
                "failedAnalyzers": 0,
                "skippedLines": 0,
                "referenceFiles": 0,
                "findingCount": 0,
                "comparisonReports": 0,
            },
            "artifacts": [],
            "referenceFiles": [],
            "failedAnalyzers": [],
            "skippedLineReport": [],
        }

    summary = payload.get("summary") if isinstance(payload, dict) else {}
    summary = summary if isinstance(summary, dict) else {}

    artifacts: list[dict[str, Any]] = [
        {"kind": "index", "label": "index.html", "path": str(bundle_index), "exportable": False},
        {"kind": "summary", "label": "run-summary.json", "path": str(summary_path), "exportable": False},
    ]
    for item in payload.get("analyzer_runs", []) if isinstance(payload, dict) else []:
        if not isinstance(item, dict):
            continue
        file_label = item.get("file") if isinstance(item.get("file"), str) else "analyzer result"
        analyzer_type = item.get("analyzer_type") if isinstance(item.get("analyzer_type"), str) else ""
        for key, kind in (("json_path", "json"), ("html_path", "html"), ("pptx_path", "pptx")):
            value = item.get(key)
            if isinstance(value, str) and value:
                artifacts.append(
                    {
                        "kind": kind,
                        "label": f"{file_label} {analyzer_type} {kind.upper()}",
                        "path": value,
                        "exportable": kind == "json",
                    }
                )
    for value in payload.get("comparison_paths", []) if isinstance(payload, dict) else []:
        if isinstance(value, str):
            artifacts.append(
                {
                    "kind": "comparison",
                    "label": Path(value).name,
                    "path": value,
                    "exportable": value.endswith(".json"),
                }
            )

    reference_files = []
    for item in payload.get("reference_files", []) if isinstance(payload, dict) else []:
        if isinstance(item, dict) and isinstance(item.get("file"), str) and isinstance(item.get("path"), str):
            entry: dict[str, Any] = {"file": item["file"], "path": item["path"]}
            if isinstance(item.get("description"), str):
                entry["description"] = item["description"]
            reference_files.append(entry)

    failed_analyzers = []
    for item in payload.get("failed_analyzers", []) if isinstance(payload, dict) else []:
        if isinstance(item, dict) and isinstance(item.get("file"), str):
            failed_analyzers.append(
                {
                    "file": item["file"],
                    "analyzerType": item.get("analyzer_type") if isinstance(item.get("analyzer_type"), str) else "unknown",
                    "error": item.get("error") if isinstance(item.get("error"), str) else None,
                }
            )

    skipped_report = []
    for item in payload.get("skipped_line_report", []) if isinstance(payload, dict) else []:
        if isinstance(item, dict) and isinstance(item.get("file"), str):
            skipped_value = item.get("skipped_lines")
            skipped_report.append(
                {
                    "file": item["file"],
                    "analyzerType": item.get("analyzer_type") if isinstance(item.get("analyzer_type"), str) else "unknown",
                    "skippedLines": int(skipped_value) if isinstance(skipped_value, (int, float)) else 0,
                }
            )

    def _num(key: str) -> int:
        value = summary.get(key)
        return int(value) if isinstance(value, (int, float)) else 0

    return {
        "scenario": scenario_meta["scenario"],
        "dataSource": scenario_meta["dataSource"],
        "bundleIndexPath": str(bundle_index),
        "summaryPath": str(summary_path),
        "summary": {
            "analyzerOutputs": _num("analyzer_outputs"),
            "failedAnalyzers": _num("failed_analyzers"),
            "skippedLines": _num("skipped_lines"),
            "referenceFiles": _num("reference_files"),
            "findingCount": _num("finding_count"),
            "comparisonReports": _num("comparison_reports"),
        },
        "artifacts": artifacts,
        "referenceFiles": reference_files,
        "failedAnalyzers": failed_analyzers,
        "skippedLineReport": skipped_report,
    }


def _run_demo(payload: dict[str, Any]) -> dict[str, Any]:
    manifest_root_str = payload.get("manifestRoot")
    if not isinstance(manifest_root_str, str) or not manifest_root_str:
        return _failure("INVALID_OPTION", "Demo manifest root is required.")
    manifest_root = Path(manifest_root_str)
    if not manifest_root.exists():
        return _failure("FILE_NOT_FOUND", "Demo manifest root is not readable.", str(manifest_root))

    output_root_str = payload.get("outputRoot")
    if isinstance(output_root_str, str) and output_root_str:
        output_root = Path(output_root_str)
    else:
        output_root = manifest_root.parent / "demo-site-report-bundles"

    target_scenario = payload.get("scenario") if isinstance(payload.get("scenario"), str) else None
    target_source = payload.get("dataSource") if payload.get("dataSource") in {"real", "synthetic"} else None

    manifests = discover_demo_manifests(manifest_root)
    if target_source is not None:
        manifests = [m for m in manifests if _data_source_for_manifest(_safe_load_json(m), m) == target_source]
    if target_scenario is not None:
        manifests = [
            m
            for m in manifests
            if m.parent.name == target_scenario
            or (
                isinstance(_safe_load_json(m).get("scenario"), str)
                and _safe_load_json(m).get("scenario") == target_scenario
            )
        ]
    if not manifests:
        return _failure("INVALID_OPTION", "No demo-site manifests matched the request.")

    baseline = next(
        (
            m
            for m in manifests
            if _safe_load_json(m).get("scenario") == "normal-baseline"
        ),
        None,
    )

    try:
        for manifest_path in manifests:
            run_demo_site_manifest(
                manifest_path,
                output_root,
                baseline_manifest_path=baseline,
                write_pptx=True,
            )
    except Exception as exc:  # noqa: BLE001
        return _failure("DEMO_RUN_FAILED", "Demo data execution failed.", repr(exc))

    listing = _list_demo_scenarios(str(manifest_root))
    selected = []
    if listing.get("ok"):
        for scenario in listing["scenarios"]:
            if target_scenario and scenario["scenario"] != target_scenario:
                continue
            if target_source and scenario["dataSource"] != target_source:
                continue
            selected.append(scenario)

    output_paths = [str(output_root / "index.html")]
    output_paths.extend(
        str(output_root / scenario["dataSource"] / scenario["scenario"] / "index.html")
        for scenario in selected
    )

    scenario_results = [
        _read_run_summary(output_root / scenario["dataSource"] / scenario["scenario"], scenario)
        for scenario in selected
    ]

    export_inputs = [
        artifact["path"]
        for scenario in scenario_results
        for artifact in scenario["artifacts"]
        if artifact.get("exportable")
    ]

    return {
        "ok": True,
        "outputPaths": output_paths,
        "exportInputPaths": export_inputs,
        "scenarios": scenario_results,
    }


def _safe_load_json(path: Path) -> dict[str, Any]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}
    return payload if isinstance(payload, dict) else {}


# ---------------------------------------------------------------------------
# FastAPI factory
# ---------------------------------------------------------------------------


def create_app(static_dir: Optional[Path] = None, *, dev_cors: bool = True) -> FastAPI:
    app = FastAPI(title="ArchScope", version="0.2.0-rc1")

    if dev_cors:
        app.add_middleware(
            CORSMiddleware,
            allow_origins=["*"],
            allow_credentials=False,
            allow_methods=["*"],
            allow_headers=["*"],
        )

    upload_root().mkdir(parents=True, exist_ok=True)

    @app.get("/api/health")
    def health() -> dict[str, Any]:
        return {"ok": True, "service": "archscope", "version": "0.2.0-rc1"}

    @app.get("/api/settings")
    def settings_get() -> dict[str, Any]:
        return load_settings()

    @app.put("/api/settings")
    async def settings_put(request: Request) -> dict[str, Any]:
        body = await request.json()
        if not isinstance(body, dict):
            raise HTTPException(status_code=400, detail="Settings body must be an object.")
        merged = {**load_settings(), **body}
        save_settings(merged)
        return {"ok": True}

    @app.post("/api/upload")
    async def upload(file: UploadFile = File(...)) -> dict[str, Any]:
        original = file.filename or "uploaded"
        target_dir = upload_root() / uuid.uuid4().hex
        target_dir.mkdir(parents=True, exist_ok=True)
        target_path = target_dir / original

        # Read in chunks to avoid blocking the event loop on large files.
        chunk_size = 1024 * 1024  # 1 MiB
        with target_path.open("wb") as out:
            while True:
                chunk = await file.read(chunk_size)
                if not chunk:
                    break
                out.write(chunk)

        return {"ok": True, "filePath": str(target_path), "originalName": original}

    @app.post("/api/analyzer/execute")
    async def analyzer_execute(request: Request) -> dict[str, Any]:
        body = await request.json()
        if not isinstance(body, dict):
            return _failure("INVALID_OPTION", "Analyzer request must be an object.")
        return _execute_analyzer(body)

    @app.post("/api/analyzer/cancel")
    async def analyzer_cancel(request: Request) -> dict[str, Any]:
        # T-206: cancellation is task-id keyed; the registry owns the
        # actual signal so the synchronous in-process analyzer can opt
        # in once we add a check-pulse helper. Returns whether the
        # taskId was found in the registry, for renderer feedback.
        body = await request.json() if request.headers.get("content-length") else {}
        task_id = body.get("taskId") if isinstance(body, dict) else None
        if not task_id:
            return {"ok": True, "canceled": False, "reason": "no taskId"}
        canceled = progress_registry.cancel(task_id)
        return {"ok": True, "canceled": canceled}

    @app.post("/api/files/select")
    async def files_select(request: Request) -> dict[str, Any]:
        """T-206 file-selection contract — server-side absolute path resolver.

        The renderer surfaces a path picker (browser File System Access API
        when available, manual absolute-path input otherwise) and POSTs the
        candidate here for normalization + existence check; the engine then
        returns a canonical absolute path that subsequent analyzer requests
        reference. Avoids the multipart-upload round-trip for the common
        case of a localhost engine. The browser-upload fallback at
        ``POST /api/upload`` stays available for non-localhost deployments.
        """
        body = await request.json()
        if not isinstance(body, dict):
            raise HTTPException(status_code=400, detail="Body must be an object.")
        candidate = body.get("path")
        if not isinstance(candidate, str) or not candidate.strip():
            raise HTTPException(status_code=400, detail="path is required.")
        resolved = Path(candidate).expanduser().resolve()
        if not resolved.exists():
            raise HTTPException(status_code=404, detail=f"Path does not exist: {resolved}")
        if not resolved.is_file():
            raise HTTPException(status_code=400, detail=f"Path is not a file: {resolved}")
        return {
            "ok": True,
            "path": str(resolved),
            "size": resolved.stat().st_size,
            "name": resolved.name,
        }

    @app.websocket("/ws/progress")
    async def ws_progress(websocket: WebSocket) -> None:
        """T-206 progress + cancellation channel.

        Renderer protocol::

            {"op": "subscribe", "taskId": "..."}    # opt in to a task
            {"op": "cancel",    "taskId": "..."}    # request cancellation

        Engine -> renderer frames::

            {"event": "ready",     "taskId": "<id>"}
            {"event": "progress",  "taskId": "<id>", "stage": "...", "ratio": 0.42}
            {"event": "done",      "taskId": "<id>"}
            {"event": "cancelled", "taskId": "<id>"}
            {"event": "error",     "taskId": "<id>", "message": "..."}

        Symmetric with the Wails track's ``analyze:done|error|cancelled``
        events (T-240f) so the same renderer hook can serve both shells.
        Phase 1 analyzers are synchronous in-process and finish in ms; the
        WebSocket exists so longer-running analyzers (multi-dump
        correlation, large GC logs) can stream progress without polling.
        """
        await websocket.accept()
        connection_id = uuid.uuid4().hex
        progress_registry.attach(connection_id, websocket)
        try:
            await websocket.send_json({"event": "ready", "connectionId": connection_id})
            while True:
                message = await websocket.receive_json()
                if not isinstance(message, dict):
                    continue
                op = message.get("op")
                task_id = message.get("taskId")
                if op == "subscribe" and task_id:
                    progress_registry.subscribe(connection_id, str(task_id))
                elif op == "cancel" and task_id:
                    canceled = progress_registry.cancel(str(task_id))
                    await websocket.send_json(
                        {"event": "cancel-ack", "taskId": task_id, "canceled": canceled},
                    )
        except WebSocketDisconnect:
            progress_registry.detach(connection_id)
        except Exception:  # noqa: BLE001 — ensure registry cleanup on any failure
            progress_registry.detach(connection_id)
            raise

    @app.post("/api/export/execute")
    async def export_execute(request: Request) -> dict[str, Any]:
        body = await request.json()
        if not isinstance(body, dict):
            return _failure("INVALID_OPTION", "Export request must be an object.")
        return _execute_export(body)

    @app.post("/api/profiler/export-pprof")
    async def profiler_export_pprof(request: Request) -> Any:
        """Encode a profile (collapsed/SVG/HTML/Jennifer CSV) as pprof bytes.

        Returns ``application/x-profile`` (gzipped pprof) so the browser
        downloads ``profile.pb.gz`` directly. Works with Pyroscope,
        Speedscope, ``go tool pprof``, ``pprof.dev``, etc.
        """
        body = await request.json()
        if not isinstance(body, dict):
            raise HTTPException(status_code=400, detail="Body must be an object.")
        file_path = body.get("filePath")
        if not isinstance(file_path, str) or not file_path:
            raise HTTPException(status_code=400, detail="filePath is required.")
        path = Path(file_path)
        if not path.is_file():
            raise HTTPException(status_code=404, detail=f"Not a file: {path}")
        fmt = body.get("format") or "collapsed"
        try:
            stacks = _stacks_for_format(path, fmt)
        except ValueError as exc:
            raise HTTPException(status_code=400, detail=str(exc))
        flame = build_flame_tree_from_collapsed(stacks)
        payload = encode_pprof_gzipped(flame, sample_type="samples", sample_unit="count")
        download_name = path.stem + ".pb.gz"
        from fastapi.responses import Response

        return Response(
            content=payload,
            media_type="application/x-profile",
            headers={"Content-Disposition": f'attachment; filename="{download_name}"'},
        )

    @app.get("/api/demo/list")
    def demo_list(manifestRoot: str = Query(..., description="Demo manifest root path")) -> dict[str, Any]:
        return _list_demo_scenarios(manifestRoot)

    @app.post("/api/demo/run")
    async def demo_run(request: Request) -> dict[str, Any]:
        body = await request.json()
        if not isinstance(body, dict):
            return _failure("INVALID_OPTION", "Demo run request must be an object.")
        return _run_demo(body)

    @app.get("/api/files")
    def files_get(path: str = Query(..., description="Absolute file path to stream")) -> Any:
        target = Path(path)
        if not target.is_file():
            raise HTTPException(status_code=404, detail="File not found.")
        return FileResponse(target)

    @app.get("/api/version")
    def version() -> dict[str, Any]:
        return {"name": "archscope-engine", "version": "0.2.0-rc1"}

    resolved_static = _resolve_static_dir(static_dir)
    if resolved_static is not None:
        app.mount(
            "/",
            StaticFiles(directory=str(resolved_static), html=True),
            name="static",
        )
    else:
        @app.get("/")
        def index_placeholder() -> JSONResponse:
            return JSONResponse(
                {
                    "ok": True,
                    "message": (
                        "ArchScope API is running. Build the React app "
                        "(apps/frontend) or run Vite on :5173 to use the UI."
                    ),
                }
            )

    return app


# ---------------------------------------------------------------------------
# Entrypoint
# ---------------------------------------------------------------------------


def run(
    *,
    host: str = "127.0.0.1",
    port: int = 8765,
    static_dir: Optional[Path] = None,
    dev_cors: bool = True,
    reload: bool = False,
) -> None:
    import uvicorn

    if reload:
        # uvicorn reload requires an import string.
        import os as _os

        if static_dir is not None:
            _os.environ["ARCHSCOPE_STATIC_DIR"] = str(static_dir)
        if not dev_cors:
            _os.environ["ARCHSCOPE_DISABLE_DEV_CORS"] = "1"
        uvicorn.run(
            "archscope_engine.web.server:_factory_for_reload",
            host=host,
            port=port,
            reload=True,
            factory=True,
        )
        return

    uvicorn.run(create_app(static_dir=static_dir, dev_cors=dev_cors), host=host, port=port)


def _factory_for_reload() -> FastAPI:
    import os as _os

    static_env = _os.environ.get("ARCHSCOPE_STATIC_DIR")
    static_dir = Path(static_env) if static_env else None
    dev_cors = _os.environ.get("ARCHSCOPE_DISABLE_DEV_CORS") != "1"
    return create_app(static_dir=static_dir, dev_cors=dev_cors)
