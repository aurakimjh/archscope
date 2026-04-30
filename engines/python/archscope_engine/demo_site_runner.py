from __future__ import annotations

import json
from dataclasses import dataclass, field
from html import escape
from pathlib import Path
from typing import Any, Callable

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.analyzers.exception_analyzer import analyze_exception_stack
from archscope_engine.analyzers.gc_log_analyzer import analyze_gc_log
from archscope_engine.analyzers.jfr_analyzer import analyze_jfr_print_json
from archscope_engine.analyzers.otel_analyzer import analyze_otel_jsonl
from archscope_engine.analyzers.profiler_analyzer import (
    analyze_collapsed_profile,
    analyze_jennifer_csv_profile,
)
from archscope_engine.analyzers.runtime_analyzer import (
    analyze_dotnet_exception_iis,
    analyze_go_panic,
    analyze_nodejs_stack,
    analyze_python_traceback,
)
from archscope_engine.analyzers.thread_dump_analyzer import analyze_thread_dump
from archscope_engine.exporters.html_exporter import render_html_report, write_html_report
from archscope_engine.exporters.json_exporter import write_json_result
from archscope_engine.exporters.pptx_exporter import write_pptx_report
from archscope_engine.exporters.report_diff import build_comparison_report
from archscope_engine.models.analysis_result import AnalysisResult


ANALYZER_TYPE_COMMANDS: dict[str, tuple[str, ...]] = {
    "access_log": ("access-log", "analyze"),
    "profiler_collapsed": ("profiler", "analyze-collapsed"),
    "jfr_recording": ("jfr", "analyze-json"),
    "gc_log": ("gc-log", "analyze"),
    "thread_dump": ("thread-dump", "analyze"),
    "exception": ("exception", "analyze"),
    "exception_stack": ("exception", "analyze"),
    "nodejs_stack": ("nodejs", "analyze"),
    "python_traceback": ("python-traceback", "analyze"),
    "go_panic": ("go-panic", "analyze"),
    "dotnet_exception_iis": ("dotnet", "analyze"),
    "otel_logs": ("otel", "analyze"),
}


@dataclass(frozen=True)
class DemoAnalyzerRun:
    analyzer_type: str
    file: str
    command: list[str]
    json_path: Path | None = None
    html_path: Path | None = None
    pptx_path: Path | None = None
    skipped_lines: int = 0
    failed: bool = False
    error: str | None = None


@dataclass(frozen=True)
class DemoScenarioRun:
    scenario: str
    data_source: str
    manifest_path: Path
    output_dir: Path
    runs: list[DemoAnalyzerRun] = field(default_factory=list)
    skipped_files: list[dict[str, Any]] = field(default_factory=list)
    reference_files: list[dict[str, Any]] = field(default_factory=list)
    comparison_paths: list[Path] = field(default_factory=list)
    index_path: Path | None = None

    @property
    def failed_runs(self) -> list[DemoAnalyzerRun]:
        return [run for run in self.runs if run.failed]

    @property
    def json_paths(self) -> list[Path]:
        return [run.json_path for run in self.runs if run.json_path is not None]


def discover_demo_manifests(manifest_root: Path) -> list[Path]:
    if manifest_root.is_file():
        return [manifest_root]
    return sorted(manifest_root.glob("*/*/manifest.json"))


def run_demo_site_manifest(
    manifest_path: Path,
    output_root: Path,
    *,
    baseline_manifest_path: Path | None = None,
    write_pptx: bool = True,
) -> DemoScenarioRun:
    manifest = _read_manifest(manifest_path)
    scenario = str(manifest.get("scenario") or manifest_path.parent.name)
    data_source = _data_source(manifest, manifest_path)
    scenario_output_dir = output_root / data_source / scenario
    scenario_output_dir.mkdir(parents=True, exist_ok=True)

    runs: list[DemoAnalyzerRun] = []
    skipped_files: list[dict[str, Any]] = []
    reference_files: list[dict[str, Any]] = []
    for index, file_entry in enumerate(_manifest_files(manifest), start=1):
        effective_entry = {
            **_dict(manifest.get("recommended_archscope_options")),
            **file_entry,
        }
        analyzer_type = str(file_entry.get("analyzer_type", "")).strip()
        relative_file = str(file_entry.get("file", "")).strip()
        if not analyzer_type or not relative_file:
            skipped_files.append(
                {
                    "file": relative_file,
                    "analyzer_type": analyzer_type,
                    "reason": "missing file or analyzer_type",
                }
            )
            continue
        if analyzer_type == "reference_only":
            reference_files.append(
                {
                    "file": relative_file,
                    "analyzer_type": analyzer_type,
                    "description": file_entry.get("description"),
                    "path": str(manifest_path.parent / relative_file),
                }
            )
            continue
        if analyzer_type not in ANALYZER_TYPE_COMMANDS:
            skipped_files.append(
                {
                    "file": relative_file,
                    "analyzer_type": analyzer_type,
                    "reason": "unsupported analyzer_type",
                }
            )
            continue

        source_path = manifest_path.parent / relative_file
        output_base = f"{index:02d}-{Path(relative_file).stem}-{analyzer_type}"
        json_path = scenario_output_dir / f"{output_base}.json"
        html_path = scenario_output_dir / f"{output_base}.html"
        pptx_path = scenario_output_dir / f"{output_base}.pptx" if write_pptx else None
        command = _command_for_entry(effective_entry, source_path, json_path)
        try:
            result = _analyze_file(effective_entry, source_path)
            _annotate_demo_metadata(result, manifest, effective_entry, manifest_path)
            write_json_result(result, json_path)
            write_html_report(json_path, html_path, title=f"{scenario} - {analyzer_type}")
            if pptx_path is not None:
                write_pptx_report(json_path, pptx_path, title=f"{scenario} - {analyzer_type}")
            diagnostics = _dict(result.metadata.get("diagnostics"))
            runs.append(
                DemoAnalyzerRun(
                    analyzer_type=analyzer_type,
                    file=relative_file,
                    command=command,
                    json_path=json_path,
                    html_path=html_path,
                    pptx_path=pptx_path,
                    skipped_lines=_int(diagnostics.get("skipped_lines")),
                )
            )
        except Exception as exc:  # noqa: BLE001 - failures are reported per analyzer.
            runs.append(
                DemoAnalyzerRun(
                    analyzer_type=analyzer_type,
                    file=relative_file,
                    command=command,
                    failed=True,
                    error=str(exc),
                )
            )

    comparison_paths = _write_baseline_comparison(
        scenario=scenario,
        scenario_output_dir=scenario_output_dir,
        baseline_manifest_path=baseline_manifest_path,
        output_root=output_root,
        after_json_paths=[run.json_path for run in runs if run.json_path is not None],
    )
    summary_path = scenario_output_dir / "run-summary.json"
    write_json_result(
        _scenario_summary(
            manifest,
            manifest_path,
            runs,
            skipped_files,
            reference_files,
            comparison_paths,
        ),
        summary_path,
    )
    index_path = scenario_output_dir / "index.html"
    index_path.write_text(
        render_demo_index(
            scenario=scenario,
            data_source=data_source,
            manifest=manifest,
            runs=runs,
            skipped_files=skipped_files,
            reference_files=reference_files,
            comparison_paths=comparison_paths,
        ),
        encoding="utf-8",
    )
    return DemoScenarioRun(
        scenario=scenario,
        data_source=data_source,
        manifest_path=manifest_path,
        output_dir=scenario_output_dir,
        runs=runs,
        skipped_files=skipped_files,
        reference_files=reference_files,
        comparison_paths=comparison_paths,
        index_path=index_path,
    )


def render_demo_index(
    *,
    scenario: str,
    data_source: str,
    manifest: dict[str, Any],
    runs: list[DemoAnalyzerRun],
    skipped_files: list[dict[str, Any]],
    reference_files: list[dict[str, Any]],
    comparison_paths: list[Path],
) -> str:
    analysis_summaries = _analysis_summaries(runs)
    summary_cards = _summary_cards(
        runs=runs,
        skipped_files=skipped_files,
        reference_files=reference_files,
        analysis_summaries=analysis_summaries,
    )
    summary_rows = _analysis_summary_rows(analysis_summaries)
    rows = "\n".join(_run_row(run) for run in runs) or (
        '<tr><td colspan="7">No analyzer outputs.</td></tr>'
    )
    skipped_rows = "\n".join(
        "<tr>"
        f"<td>{escape(str(item.get('file', '')))}</td>"
        f"<td>{escape(str(item.get('analyzer_type', '')))}</td>"
        f"<td>{escape(str(item.get('reason', '')))}</td>"
        "</tr>"
        for item in skipped_files
    ) or '<tr><td colspan="3">No skipped manifest files.</td></tr>'
    reference_rows = "\n".join(
        "<tr>"
        f"<td>{escape(str(item.get('file', '')))}</td>"
        f"<td>{escape(str(item.get('description', '')))}</td>"
        f"<td>{escape(str(item.get('path', '')))}</td>"
        "</tr>"
        for item in reference_files
    ) or '<tr><td colspan="3">No reference-only files.</td></tr>'
    comparison_items = "\n".join(
        f'<li><a href="{escape(path.name)}">{escape(path.name)}</a></li>'
        for path in comparison_paths
    ) or "<li>No baseline comparison generated.</li>"
    expected = manifest.get("expected_key_signals")
    expected_html = (
        f"<pre>{escape(json.dumps(expected, ensure_ascii=False, indent=2))}</pre>"
        if isinstance(expected, dict)
        else "<p>No expected signals in manifest.</p>"
    )
    description = escape(str(manifest.get("description", "")))
    return "\n".join(
        [
            "<!doctype html>",
            '<html lang="en">',
            "<head>",
            '  <meta charset="utf-8">',
            '  <meta name="viewport" content="width=device-width, initial-scale=1">',
            f"  <title>ArchScope Demo Bundle - {escape(scenario)}</title>",
            "  <style>",
            _index_stylesheet(),
            "  </style>",
            "</head>",
            "<body>",
            f"  <h1>{escape(scenario)}</h1>",
            f"  <p class=\"meta\">Data source: {escape(data_source)}</p>",
            f"  <p>{description}</p>",
            "  <h2>Scenario Executive Summary</h2>",
            f"  <div class=\"summary-grid\">{summary_cards}</div>",
            "  <h2>Analyzer Result Summary</h2>",
            "  <table><thead><tr><th>Analyzer</th><th>File</th><th>Key metrics</th>"
            "<th>Findings</th><th>Skipped lines</th></tr></thead>",
            f"  <tbody>{summary_rows}</tbody></table>",
            "  <h2>Analyzer Outputs</h2>",
            "  <table><thead><tr><th>File</th><th>Analyzer</th><th>Status</th>"
            "<th>Skipped lines</th><th>JSON</th><th>HTML</th><th>PPTX</th></tr></thead>",
            f"  <tbody>{rows}</tbody></table>",
            "  <h2>Manifest Files Not Analyzed</h2>",
            "  <table><thead><tr><th>File</th><th>Analyzer</th><th>Reason</th></tr></thead>",
            f"  <tbody>{skipped_rows}</tbody></table>",
            "  <h2>Correlation Context</h2>",
            "  <table><thead><tr><th>File</th><th>Description</th><th>Path</th></tr></thead>",
            f"  <tbody>{reference_rows}</tbody></table>",
            "  <h2>Baseline Comparison</h2>",
            f"  <ul>{comparison_items}</ul>",
            "  <h2>Expected Signals</h2>",
            expected_html,
            "</body>",
            "</html>",
        ]
    )


def _analyze_file(file_entry: dict[str, Any], source_path: Path) -> AnalysisResult:
    analyzer_type = str(file_entry["analyzer_type"])
    top_n = int(file_entry.get("top_n") or 20)
    if analyzer_type == "access_log":
        return analyze_access_log(
            source_path,
            log_format=str(file_entry.get("format") or "nginx"),
        )
    if analyzer_type == "profiler_collapsed":
        if file_entry.get("format") == "jennifer_csv" or source_path.suffix.lower() == ".csv":
            return analyze_jennifer_csv_profile(
                path=source_path,
                interval_ms=float(file_entry.get("interval_ms") or 100),
                elapsed_sec=_optional_float(file_entry.get("elapsed_sec")),
                top_n=top_n,
            )
        return analyze_collapsed_profile(
            path=source_path,
            interval_ms=float(file_entry.get("wall_interval_ms") or 100),
            elapsed_sec=_optional_float(file_entry.get("elapsed_sec")),
            top_n=top_n,
            profile_kind="wall",
        )
    analyzers: dict[str, Callable[[Path], AnalysisResult]] = {
        "jfr_recording": lambda path: analyze_jfr_print_json(path=path, top_n=top_n),
        "gc_log": lambda path: analyze_gc_log(path=path, top_n=top_n),
        "thread_dump": lambda path: analyze_thread_dump(path=path, top_n=top_n),
        "exception": lambda path: analyze_exception_stack(path=path, top_n=top_n),
        "exception_stack": lambda path: analyze_exception_stack(path=path, top_n=top_n),
        "nodejs_stack": lambda path: analyze_nodejs_stack(path=path, top_n=top_n),
        "python_traceback": lambda path: analyze_python_traceback(path=path, top_n=top_n),
        "go_panic": lambda path: analyze_go_panic(path=path, top_n=top_n),
        "dotnet_exception_iis": lambda path: analyze_dotnet_exception_iis(
            path=path,
            top_n=top_n,
        ),
        "otel_logs": lambda path: analyze_otel_jsonl(path=path, top_n=top_n),
    }
    return analyzers[analyzer_type](source_path)


def _annotate_demo_metadata(
    result: AnalysisResult,
    manifest: dict[str, Any],
    file_entry: dict[str, Any],
    manifest_path: Path,
) -> None:
    data_source = _data_source(manifest, manifest_path)
    scenario = str(manifest.get("scenario") or manifest_path.parent.name)
    result.metadata["demo_site"] = {
        "scenario": scenario,
        "data_source": data_source,
        "manifest": str(manifest_path),
        "manifest_analyzer_type": file_entry.get("analyzer_type"),
        "manifest_file": file_entry.get("file"),
        "expected_categories": manifest.get("expected_categories"),
        "expected_key_signals": manifest.get("expected_key_signals"),
    }
    if (
        result.type == "access_log"
        and data_source == "real"
        and "baseline" in scenario
        and float(result.summary.get("error_rate") or 0) >= 5
    ):
        findings = result.metadata.setdefault("findings", [])
        if isinstance(findings, list):
            findings.append(
                {
                    "severity": "warning",
                    "code": "BASELINE_ANOMALY",
                    "message": "Real baseline data includes elevated 5xx/error-rate signals.",
                    "evidence": {"error_rate": result.summary.get("error_rate")},
                }
            )


def _write_baseline_comparison(
    *,
    scenario: str,
    scenario_output_dir: Path,
    baseline_manifest_path: Path | None,
    output_root: Path,
    after_json_paths: list[Path],
) -> list[Path]:
    if scenario == "normal-baseline" or baseline_manifest_path is None:
        return []
    baseline_data_source = _data_source(
        _read_manifest(baseline_manifest_path),
        baseline_manifest_path,
    )
    before_dir = output_root / baseline_data_source / "normal-baseline"
    before_json_by_analyzer = _result_json_by_analyzer(before_dir)
    if not before_json_by_analyzer:
        baseline_run = run_demo_site_manifest(
            baseline_manifest_path,
            output_root,
            baseline_manifest_path=None,
            write_pptx=False,
        )
        before_json_by_analyzer = _result_json_by_analyzer(baseline_run.output_dir)

    comparison_paths: list[Path] = []
    after_json_by_analyzer = {
        _analyzer_type_from_output_path(path): path
        for path in after_json_paths
        if _analyzer_type_from_output_path(path)
    }
    for analyzer_type, after_json in sorted(after_json_by_analyzer.items()):
        before_json = before_json_by_analyzer.get(analyzer_type)
        if before_json is None:
            continue
        diff_json = scenario_output_dir / f"normal-baseline-vs-{analyzer_type}.json"
        diff_html = scenario_output_dir / f"normal-baseline-vs-{analyzer_type}.html"
        comparison = build_comparison_report(
            before_json,
            after_json,
            label=f"normal-baseline vs {scenario} ({analyzer_type})",
        )
        write_json_result(comparison, diff_json)
        diff_html.write_text(
            render_html_report(comparison.to_dict(), source_path=diff_json),
            encoding="utf-8",
        )
        comparison_paths.extend([diff_json, diff_html])
    return comparison_paths


def _first_result_json(output_dir: Path, analyzer_type: str) -> Path | None:
    candidates = sorted(output_dir.glob(f"*-{analyzer_type}.json"))
    return candidates[0] if candidates else None


def _scenario_summary(
    manifest: dict[str, Any],
    manifest_path: Path,
    runs: list[DemoAnalyzerRun],
    skipped_files: list[dict[str, Any]],
    reference_files: list[dict[str, Any]],
    comparison_paths: list[Path],
) -> dict[str, Any]:
    analysis_summaries = _analysis_summaries(runs)
    return {
        "scenario": manifest.get("scenario") or manifest_path.parent.name,
        "data_source": _data_source(manifest, manifest_path),
        "manifest": str(manifest_path),
        "summary": {
            "analyzer_outputs": len([run for run in runs if not run.failed]),
            "failed_analyzers": len([run for run in runs if run.failed]),
            "skipped_lines": sum(run.skipped_lines for run in runs),
            "reference_files": len(reference_files),
            "finding_count": sum(
                int(summary.get("finding_count") or 0)
                for summary in analysis_summaries
            ),
            "comparison_reports": len(comparison_paths),
        },
        "analysis_summaries": analysis_summaries,
        "analyzer_runs": [
            {
                "file": run.file,
                "analyzer_type": run.analyzer_type,
                "command": run.command,
                "json_path": str(run.json_path) if run.json_path else None,
                "html_path": str(run.html_path) if run.html_path else None,
                "pptx_path": str(run.pptx_path) if run.pptx_path else None,
                "skipped_lines": run.skipped_lines,
                "failed": run.failed,
                "error": run.error,
            }
            for run in runs
        ],
        "skipped_files": skipped_files,
        "reference_files": reference_files,
        "comparison_paths": [str(path) for path in comparison_paths],
        "failed_analyzers": [
            {
                "file": run.file,
                "analyzer_type": run.analyzer_type,
                "error": run.error,
            }
            for run in runs
            if run.failed
        ],
        "skipped_line_report": [
            {
                "file": run.file,
                "analyzer_type": run.analyzer_type,
                "skipped_lines": run.skipped_lines,
            }
            for run in runs
            if run.skipped_lines > 0
        ],
    }


def _command_for_entry(
    file_entry: dict[str, Any],
    source_path: Path,
    output_path: Path,
) -> list[str]:
    analyzer_type = str(file_entry["analyzer_type"])
    command = list(ANALYZER_TYPE_COMMANDS[analyzer_type])
    is_jennifer = (
        analyzer_type == "profiler_collapsed"
        and (
            file_entry.get("format") == "jennifer_csv"
            or source_path.suffix.lower() == ".csv"
        )
    )
    if is_jennifer:
        command = ["profiler", "analyze-jennifer-csv", "--file", str(source_path)]
        if "interval_ms" in file_entry:
            command.extend(["--interval-ms", str(file_entry["interval_ms"])])
    elif analyzer_type == "profiler_collapsed":
        command.extend(["--wall", str(source_path)])
        if "wall_interval_ms" in file_entry:
            command.extend(["--wall-interval-ms", str(file_entry["wall_interval_ms"])])
    else:
        command.extend(["--file", str(source_path)])
    if analyzer_type == "profiler_collapsed" and "elapsed_sec" in file_entry:
        command.extend(["--elapsed-sec", str(file_entry["elapsed_sec"])])
    if file_entry.get("format") and analyzer_type == "access_log":
        command.extend(["--format", str(file_entry["format"])])
    if file_entry.get("top_n"):
        command.extend(["--top-n", str(file_entry["top_n"])])
    command.extend(["--out", str(output_path)])
    return command


def _run_row(run: DemoAnalyzerRun) -> str:
    status = "failed" if run.failed else "ok"
    json_link = _link(run.json_path)
    html_link = _link(run.html_path)
    pptx_link = _link(run.pptx_path)
    return (
        "<tr>"
        f"<td>{escape(run.file)}</td>"
        f"<td>{escape(run.analyzer_type)}</td>"
        f"<td>{escape(status if run.error is None else status + ': ' + run.error)}</td>"
        f"<td>{run.skipped_lines}</td>"
        f"<td>{json_link}</td>"
        f"<td>{html_link}</td>"
        f"<td>{pptx_link}</td>"
        "</tr>"
    )


def _link(path: Path | None) -> str:
    if path is None:
        return ""
    return f'<a href="{escape(path.name)}">{escape(path.name)}</a>'


def _read_manifest(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def _analysis_summaries(runs: list[DemoAnalyzerRun]) -> list[dict[str, Any]]:
    summaries: list[dict[str, Any]] = []
    for run in runs:
        if run.json_path is None or run.failed:
            continue
        try:
            payload = json.loads(run.json_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        metadata = _dict(payload.get("metadata"))
        diagnostics = _dict(metadata.get("diagnostics"))
        findings = metadata.get("findings")
        summaries.append(
            {
                "file": run.file,
                "analyzer_type": run.analyzer_type,
                "result_type": payload.get("type"),
                "json_path": str(run.json_path),
                "html_path": str(run.html_path) if run.html_path else None,
                "pptx_path": str(run.pptx_path) if run.pptx_path else None,
                "key_metrics": _key_metrics(_dict(payload.get("summary"))),
                "finding_count": len(findings) if isinstance(findings, list) else 0,
                "top_findings": [
                    _finding_label(finding)
                    for finding in findings[:5]
                    if isinstance(finding, dict)
                ]
                if isinstance(findings, list)
                else [],
                "skipped_lines": _int(diagnostics.get("skipped_lines")),
            }
        )
    return summaries


def _key_metrics(summary: dict[str, Any]) -> dict[str, Any]:
    preferred_keys = [
        "total_requests",
        "error_rate",
        "p95_response_ms",
        "total_records",
        "unique_traces",
        "failed_traces",
        "total_samples",
        "estimated_seconds",
        "total_events",
        "max_pause_ms",
        "total_threads",
        "blocked_threads",
        "total_exceptions",
        "unique_exception_types",
    ]
    metrics = {
        key: summary[key]
        for key in preferred_keys
        if key in summary and not isinstance(summary[key], (dict, list))
    }
    if metrics:
        return metrics
    return {
        key: value
        for key, value in list(summary.items())[:5]
        if not isinstance(value, (dict, list))
    }


def _finding_label(finding: dict[str, Any]) -> str:
    code = finding.get("code")
    severity = finding.get("severity")
    message = finding.get("message")
    parts = [str(item) for item in (severity, code, message) if item]
    return " - ".join(parts)


def _summary_cards(
    *,
    runs: list[DemoAnalyzerRun],
    skipped_files: list[dict[str, Any]],
    reference_files: list[dict[str, Any]],
    analysis_summaries: list[dict[str, Any]],
) -> str:
    cards = {
        "Analyzer outputs": len([run for run in runs if not run.failed]),
        "Failed analyzers": len([run for run in runs if run.failed]),
        "Skipped lines": sum(run.skipped_lines for run in runs),
        "Finding count": sum(int(item.get("finding_count") or 0) for item in analysis_summaries),
        "Skipped manifest files": len(skipped_files),
        "Reference files": len(reference_files),
    }
    return "".join(
        f"<div><span>{escape(label)}</span><strong>{escape(str(value))}</strong></div>"
        for label, value in cards.items()
    )


def _analysis_summary_rows(summaries: list[dict[str, Any]]) -> str:
    if not summaries:
        return '<tr><td colspan="5">No analyzer summaries.</td></tr>'
    rows: list[str] = []
    for summary in summaries:
        metrics = ", ".join(
            f"{key}: {value}" for key, value in _dict(summary.get("key_metrics")).items()
        )
        findings = summary.get("top_findings")
        finding_text = (
            "; ".join(str(item) for item in findings)
            if isinstance(findings, list)
            else ""
        )
        rows.append(
            "<tr>"
            f"<td>{escape(str(summary.get('analyzer_type', '')))}</td>"
            f"<td>{escape(str(summary.get('file', '')))}</td>"
            f"<td>{escape(metrics)}</td>"
            f"<td>{escape(finding_text or str(summary.get('finding_count', 0)))}</td>"
            f"<td>{escape(str(summary.get('skipped_lines', 0)))}</td>"
            "</tr>"
        )
    return "\n".join(rows)


def _result_json_by_analyzer(output_dir: Path) -> dict[str, Path]:
    return {
        analyzer_type: path
        for path in sorted(output_dir.glob("*.json"))
        if (analyzer_type := _analyzer_type_from_output_path(path))
    }


def _analyzer_type_from_output_path(path: Path) -> str | None:
    if path.name.startswith("normal-baseline-vs-") or path.name == "run-summary.json":
        return None
    for analyzer_type in sorted(ANALYZER_TYPE_COMMANDS, key=len, reverse=True):
        if path.name.endswith(f"-{analyzer_type}.json"):
            return analyzer_type
    return None


def _manifest_files(manifest: dict[str, Any]) -> list[dict[str, Any]]:
    files = manifest.get("files")
    if not isinstance(files, list):
        return []
    return [item for item in files if isinstance(item, dict)]


def _data_source(manifest: dict[str, Any], manifest_path: Path) -> str:
    data_source = manifest.get("data_source")
    if data_source in {"real", "synthetic"}:
        return str(data_source)
    parts = set(manifest_path.parts)
    if "real" in parts:
        return "real"
    if "synthetic" in parts:
        return "synthetic"
    return "unknown"


def _dict(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def _int(value: Any) -> int:
    return value if isinstance(value, int) else 0


def _optional_float(value: Any) -> float | None:
    if value is None:
        return None
    return float(value)


def _index_stylesheet() -> str:
    return """
body{max-width:1180px;margin:32px auto;padding:0 24px;color:#111827;background:#f8fafc}
body{font-family:Inter,Arial,sans-serif}
h1{margin-bottom:4px}h2{margin-top:28px}.meta{color:#475569;font-weight:700}
.summary-grid{display:grid;grid-template-columns:repeat(6,1fr);gap:12px}
.summary-grid div{padding:12px;border:1px solid #dbe3ef;background:#fff}
.summary-grid span{display:block;color:#64748b;font-size:12px;font-weight:700}
.summary-grid strong{display:block;margin-top:6px;font-size:22px}
table{width:100%;border-collapse:collapse;background:#fff;border:1px solid #dbe3ef}
th,td{padding:10px 12px;border-bottom:1px solid #e5e7eb;text-align:left;vertical-align:top}
th{color:#334155;background:#eef2f7}a{color:#1d4ed8;font-weight:700}
pre{overflow:auto;padding:12px;border:1px solid #dbe3ef;background:#fff}
"""
