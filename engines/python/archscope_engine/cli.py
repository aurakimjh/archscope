from __future__ import annotations

import json
from datetime import datetime
from pathlib import Path
from typing import Callable, Optional

import typer
from rich.console import Console

from archscope_engine.common.debug_log import DebugLogCollector, default_debug_log_dir
from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.analyzers.exception_analyzer import analyze_exception_stack
from archscope_engine.analyzers.gc_log_analyzer import analyze_gc_log
from archscope_engine.analyzers.jfr_analyzer import analyze_jfr_print_json
from archscope_engine.analyzers.otel_analyzer import analyze_otel_jsonl
from archscope_engine.analyzers.profiler_analyzer import (
    analyze_collapsed_profile,
    analyze_jennifer_csv_profile,
    breakdown_collapsed_profile,
    breakdown_jennifer_csv_profile,
    drilldown_collapsed_profile,
    drilldown_jennifer_csv_profile,
)
from archscope_engine.analyzers.profiler_drilldown import DrilldownFilter
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
from archscope_engine.demo_site_runner import (
    ANALYZER_TYPE_COMMANDS,
    discover_demo_manifests,
    run_demo_site_manifest,
)
from archscope_engine.models.analysis_result import AnalysisResult

console = Console()

app = typer.Typer(
    help="ArchScope analysis engine CLI.",
    no_args_is_help=True,
)
access_log_app = typer.Typer(help="Access log analysis commands.")
profiler_app = typer.Typer(help="Profiler analysis commands.")
jfr_app = typer.Typer(help="JFR recording analysis commands.")
gc_log_app = typer.Typer(help="GC log analysis commands.")
thread_dump_app = typer.Typer(help="Java thread dump analysis commands.")
exception_app = typer.Typer(help="Java exception stack analysis commands.")
nodejs_app = typer.Typer(help="Node.js log and stack analysis commands.")
python_traceback_app = typer.Typer(help="Python traceback analysis commands.")
go_panic_app = typer.Typer(help="Go panic and goroutine analysis commands.")
dotnet_app = typer.Typer(help=".NET exception and IIS log analysis commands.")
otel_app = typer.Typer(help="OpenTelemetry input analysis commands.")
report_app = typer.Typer(help="Report export commands.")
demo_site_app = typer.Typer(help="Demo-site manifest runner commands.")


@access_log_app.command("analyze")
def access_log_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    format: str = typer.Option("nginx", "--format"),
    max_lines: Optional[int] = typer.Option(None, "--max-lines"),
    start_time: Optional[str] = typer.Option(None, "--start-time"),
    end_time: Optional[str] = typer.Option(None, "--end-time"),
    out: Path = typer.Option(..., "--out"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze an access log and write an AnalysisResult JSON file."""
    if max_lines is not None and max_lines <= 0:
        raise typer.BadParameter("--max-lines must be a positive integer.")

    collector = _debug_collector(
        analyzer_type="access_log",
        source_file=file,
        parser="nginx_access_log",
        parser_options={
            "format": format,
            "max_lines": max_lines,
            "start_time": start_time,
            "end_time": end_time,
        },
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_access_log(
            file,
            log_format=format,
            max_lines=max_lines,
            start_time=_parse_optional_datetime(start_time, "--start-time"),
            end_time=_parse_optional_datetime(end_time, "--end-time"),
            debug_log=collector,
        ),
        "access log",
    )


@profiler_app.command("analyze-collapsed")
def profiler_analyze_collapsed(
    wall: Path = typer.Option(..., "--wall", exists=True, readable=True),
    wall_interval_ms: float = typer.Option(100, "--wall-interval-ms"),
    elapsed_sec: Optional[float] = typer.Option(None, "--elapsed-sec"),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze an async-profiler wall-clock collapsed file."""
    collector = _debug_collector(
        analyzer_type="profiler_collapsed",
        source_file=wall,
        parser="async_profiler_collapsed",
        parser_options={
            "wall_interval_ms": wall_interval_ms,
            "elapsed_sec": elapsed_sec,
            "top_n": top_n,
        },
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_collapsed_profile(
            path=wall,
            interval_ms=wall_interval_ms,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
            profile_kind="wall",
            debug_log=collector,
        ),
        "profiler",
    )


@profiler_app.command("analyze-jennifer-csv")
def profiler_analyze_jennifer_csv(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    interval_ms: float = typer.Option(100, "--interval-ms"),
    elapsed_sec: Optional[float] = typer.Option(None, "--elapsed-sec"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze a Jennifer APM flamegraph CSV file."""
    collector = _debug_collector(
        analyzer_type="profiler_collapsed",
        source_file=file,
        parser="jennifer_flamegraph_csv",
        parser_options={
            "interval_ms": interval_ms,
            "elapsed_sec": elapsed_sec,
            "top_n": top_n,
        },
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_jennifer_csv_profile(
            path=file,
            interval_ms=interval_ms,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
            debug_log=collector,
        ),
        "Jennifer profiler",
    )


@profiler_app.command("drilldown")
def profiler_drilldown(
    wall: Optional[Path] = typer.Option(None, "--wall", exists=True, readable=True),
    jennifer_csv: Optional[Path] = typer.Option(
        None,
        "--jennifer-csv",
        exists=True,
        readable=True,
    ),
    out: Path = typer.Option(..., "--out"),
    filter_pattern: list[str] = typer.Option([], "--filter"),
    filter_type: str = typer.Option("include_text", "--filter-type"),
    match_mode: str = typer.Option("anywhere", "--match-mode"),
    view_mode: str = typer.Option("preserve_full_path", "--view-mode"),
    wall_interval_ms: float = typer.Option(100, "--wall-interval-ms"),
    elapsed_sec: Optional[float] = typer.Option(None, "--elapsed-sec"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Apply one or more profiler flamegraph drill-down filters."""
    filters = [
        DrilldownFilter(
            pattern=pattern,
            filter_type=filter_type,  # type: ignore[arg-type]
            match_mode=match_mode,  # type: ignore[arg-type]
            view_mode=view_mode,  # type: ignore[arg-type]
        )
        for pattern in filter_pattern
    ]
    if wall is None and jennifer_csv is None:
        raise typer.BadParameter("Either --wall or --jennifer-csv is required.")
    if wall is not None and jennifer_csv is not None:
        raise typer.BadParameter("Use only one input: --wall or --jennifer-csv.")
    source = jennifer_csv if jennifer_csv is not None else wall
    parser = "jennifer_flamegraph_csv" if jennifer_csv is not None else "async_profiler_collapsed"
    collector = _debug_collector(
        analyzer_type="profiler_collapsed",
        source_file=source,
        parser=parser,
        parser_options={
            "filters": [filter_item.pattern for filter_item in filters],
            "filter_type": filter_type,
            "match_mode": match_mode,
            "view_mode": view_mode,
            "top_n": top_n,
        },
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: drilldown_jennifer_csv_profile(
            path=jennifer_csv,
            interval_ms=wall_interval_ms,
            filters=filters,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
            debug_log=collector,
        )
        if jennifer_csv is not None
        else drilldown_collapsed_profile(
            path=wall,
            interval_ms=wall_interval_ms,
            filters=filters,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
            debug_log=collector,
        ),
        "profiler drill-down",
    )


@profiler_app.command("breakdown")
def profiler_breakdown(
    wall: Optional[Path] = typer.Option(None, "--wall", exists=True, readable=True),
    jennifer_csv: Optional[Path] = typer.Option(
        None,
        "--jennifer-csv",
        exists=True,
        readable=True,
    ),
    out: Path = typer.Option(..., "--out"),
    filter_pattern: list[str] = typer.Option([], "--filter"),
    filter_type: str = typer.Option("include_text", "--filter-type"),
    match_mode: str = typer.Option("anywhere", "--match-mode"),
    view_mode: str = typer.Option("preserve_full_path", "--view-mode"),
    wall_interval_ms: float = typer.Option(100, "--wall-interval-ms"),
    elapsed_sec: Optional[float] = typer.Option(None, "--elapsed-sec"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Calculate execution breakdown for a profiler input and optional filters."""
    filters = [
        DrilldownFilter(
            pattern=pattern,
            filter_type=filter_type,  # type: ignore[arg-type]
            match_mode=match_mode,  # type: ignore[arg-type]
            view_mode=view_mode,  # type: ignore[arg-type]
        )
        for pattern in filter_pattern
    ]
    if wall is None and jennifer_csv is None:
        raise typer.BadParameter("Either --wall or --jennifer-csv is required.")
    if wall is not None and jennifer_csv is not None:
        raise typer.BadParameter("Use only one input: --wall or --jennifer-csv.")
    source = jennifer_csv if jennifer_csv is not None else wall
    parser = "jennifer_flamegraph_csv" if jennifer_csv is not None else "async_profiler_collapsed"
    collector = _debug_collector(
        analyzer_type="profiler_collapsed",
        source_file=source,
        parser=parser,
        parser_options={
            "filters": [filter_item.pattern for filter_item in filters],
            "filter_type": filter_type,
            "match_mode": match_mode,
            "view_mode": view_mode,
            "top_n": top_n,
        },
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: breakdown_jennifer_csv_profile(
            path=jennifer_csv,
            interval_ms=wall_interval_ms,
            filters=filters,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
            debug_log=collector,
        )
        if jennifer_csv is not None
        else breakdown_collapsed_profile(
            path=wall,
            interval_ms=wall_interval_ms,
            filters=filters,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
            debug_log=collector,
        ),
        "profiler breakdown",
    )


@jfr_app.command("analyze-json")
def jfr_analyze_json(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze JSON emitted by `jfr print --json`."""
    collector = _debug_collector(
        analyzer_type="jfr_recording",
        source_file=file,
        parser="jdk_jfr_print_json",
        parser_options={"top_n": top_n},
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_jfr_print_json(path=file, top_n=top_n, debug_log=collector),
        "JFR",
    )


@gc_log_app.command("analyze")
def gc_log_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze a HotSpot unified GC log."""
    collector = _debug_collector(
        analyzer_type="gc_log",
        source_file=file,
        parser="hotspot_unified_gc_log",
        parser_options={"top_n": top_n},
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_gc_log(path=file, top_n=top_n, debug_log=collector),
        "GC log",
    )


@thread_dump_app.command("analyze")
def thread_dump_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze a Java thread dump text file."""
    collector = _debug_collector(
        analyzer_type="thread_dump",
        source_file=file,
        parser="java_thread_dump",
        parser_options={"top_n": top_n},
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_thread_dump(path=file, top_n=top_n, debug_log=collector),
        "thread dump",
    )


@exception_app.command("analyze")
def exception_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze Java exception stack traces."""
    collector = _debug_collector(
        analyzer_type="exception_stack",
        source_file=file,
        parser="java_exception_stack",
        parser_options={"top_n": top_n},
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_exception_stack(path=file, top_n=top_n, debug_log=collector),
        "exception",
    )


@nodejs_app.command("analyze")
def nodejs_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze Node.js error stack traces."""
    collector = _debug_collector(
        analyzer_type="nodejs_stack",
        source_file=file,
        parser="nodejs_stack_trace",
        parser_options={"top_n": top_n},
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_nodejs_stack(path=file, top_n=top_n, debug_log=collector),
        "Node.js stack",
    )


@python_traceback_app.command("analyze")
def python_traceback_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze Python traceback blocks."""
    collector = _debug_collector(
        analyzer_type="python_traceback",
        source_file=file,
        parser="python_traceback",
        parser_options={"top_n": top_n},
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_python_traceback(path=file, top_n=top_n, debug_log=collector),
        "Python traceback",
    )


@go_panic_app.command("analyze")
def go_panic_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze Go panic and goroutine dumps."""
    collector = _debug_collector(
        analyzer_type="go_panic",
        source_file=file,
        parser="go_panic_goroutine",
        parser_options={"top_n": top_n},
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_go_panic(path=file, top_n=top_n, debug_log=collector),
        "Go panic",
    )


@dotnet_app.command("analyze")
def dotnet_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze .NET exception stack traces and IIS W3C access logs."""
    collector = _debug_collector(
        analyzer_type="dotnet_exception_iis",
        source_file=file,
        parser="dotnet_exception_iis_w3c",
        parser_options={"top_n": top_n},
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_dotnet_exception_iis(
            path=file,
            top_n=top_n,
            debug_log=collector,
        ),
        ".NET/IIS",
    )


@otel_app.command("analyze")
def otel_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
    debug_log: bool = typer.Option(False, "--debug-log"),
    debug_log_dir: Optional[Path] = typer.Option(None, "--debug-log-dir"),
) -> None:
    """Analyze line-delimited OpenTelemetry JSON logs."""
    collector = _debug_collector(
        analyzer_type="otel_logs",
        source_file=file,
        parser="otel_jsonl",
        parser_options={"top_n": top_n},
        debug_log=debug_log,
        debug_log_dir=debug_log_dir,
    )
    _write_result_with_debug(
        out,
        collector,
        lambda: analyze_otel_jsonl(path=file, top_n=top_n, debug_log=collector),
        "OpenTelemetry logs",
    )


@report_app.command("html")
def report_html(
    input: Path = typer.Option(
        ...,
        "--input",
        exists=True,
        readable=True,
        help="AnalysisResult JSON or parser debug JSON input.",
    ),
    out: Path = typer.Option(..., "--out", help="HTML report output path."),
    title: Optional[str] = typer.Option(None, "--title"),
) -> None:
    """Render an AnalysisResult or parser debug JSON file as a portable HTML report."""
    write_html_report(input, out, title=title)
    console.print(f"Wrote HTML report: {out}")


@report_app.command("diff")
def report_diff(
    before: Path = typer.Option(..., "--before", exists=True, readable=True),
    after: Path = typer.Option(..., "--after", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    label: Optional[str] = typer.Option(None, "--label"),
    html_out: Optional[Path] = typer.Option(None, "--html-out"),
) -> None:
    """Create a before/after comparison AnalysisResult JSON."""
    result = build_comparison_report(before, after, label=label)
    write_json_result(result, out)
    console.print(f"Wrote comparison result: {out}")
    if html_out is not None:
        html_out.parent.mkdir(parents=True, exist_ok=True)
        html_out.write_text(
            render_html_report(result.to_dict(), source_path=out),
            encoding="utf-8",
        )
        console.print(f"Wrote comparison HTML report: {html_out}")


@report_app.command("pptx")
def report_pptx(
    input: Path = typer.Option(
        ...,
        "--input",
        exists=True,
        readable=True,
        help="AnalysisResult JSON input.",
    ),
    out: Path = typer.Option(..., "--out", help="PowerPoint .pptx output path."),
    title: Optional[str] = typer.Option(None, "--title"),
) -> None:
    """Render an AnalysisResult JSON file as a minimal PowerPoint deck."""
    write_pptx_report(input, out, title=title)
    console.print(f"Wrote PowerPoint report: {out}")


@demo_site_app.command("mapping")
def demo_site_mapping() -> None:
    """Print the manifest analyzer_type to ArchScope CLI command mapping."""
    for analyzer_type, command in sorted(ANALYZER_TYPE_COMMANDS.items()):
        console.print(f"{analyzer_type} -> {' '.join(command)}")


@demo_site_app.command("run")
def demo_site_run(
    manifest_root: Path = typer.Option(
        ...,
        "--manifest-root",
        exists=True,
        readable=True,
        help="Demo-site root directory or a single scenario manifest.json.",
    ),
    out: Path = typer.Option(..., "--out", help="Report bundle output directory."),
    scenario: list[str] = typer.Option(
        [],
        "--scenario",
        help="Scenario name to run. May be provided multiple times.",
    ),
    no_pptx: bool = typer.Option(False, "--no-pptx", help="Skip PowerPoint report output."),
) -> None:
    """Run ArchScope analyzers for demo-site manifests and write report bundles."""
    manifests = discover_demo_manifests(manifest_root)
    if scenario:
        requested = set(scenario)
        manifests = [
            path
            for path in manifests
            if path.parent.name in requested or _manifest_scenario(path) in requested
        ]
    if not manifests:
        raise typer.BadParameter("No demo-site manifests matched the request.")

    baseline_manifest = next(
        (path for path in manifests if _manifest_scenario(path) == "normal-baseline"),
        None,
    )
    if baseline_manifest is None and manifest_root.is_dir():
        baseline_candidate = manifest_root / "synthetic" / "normal-baseline" / "manifest.json"
        baseline_manifest = baseline_candidate if baseline_candidate.exists() else None

    runs = []
    for manifest_path in manifests:
        run = run_demo_site_manifest(
            manifest_path,
            out,
            baseline_manifest_path=baseline_manifest,
            write_pptx=not no_pptx,
        )
        runs.append(run)
        console.print(
            f"Wrote demo bundle: {run.output_dir} "
            f"({len(run.json_paths)} JSON, {len(run.failed_runs)} failed)"
        )
        for failed in run.failed_runs:
            console.print(
                f"[red]FAILED[/red] {run.scenario}/{failed.file} "
                f"({failed.analyzer_type}): {failed.error}"
            )
        for analyzer_run in run.runs:
            if analyzer_run.skipped_lines > 0:
                console.print(
                    f"[yellow]SKIPPED_LINES[/yellow] {run.scenario}/{analyzer_run.file} "
                    f"({analyzer_run.analyzer_type}): {analyzer_run.skipped_lines}"
                )

    index_path = out / "index.html"
    _write_demo_root_index(index_path, runs)
    console.print(f"Wrote demo-site index: {index_path}")


app.add_typer(access_log_app, name="access-log")
app.add_typer(profiler_app, name="profiler")
app.add_typer(jfr_app, name="jfr")
app.add_typer(gc_log_app, name="gc-log")
app.add_typer(thread_dump_app, name="thread-dump")
app.add_typer(exception_app, name="exception")
app.add_typer(nodejs_app, name="nodejs")
app.add_typer(python_traceback_app, name="python-traceback")
app.add_typer(go_panic_app, name="go-panic")
app.add_typer(dotnet_app, name="dotnet")
app.add_typer(otel_app, name="otel")
app.add_typer(report_app, name="report")
app.add_typer(demo_site_app, name="demo-site")


def _manifest_scenario(path: Path) -> str:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except Exception:
        return path.parent.name
    return str(payload.get("scenario") or path.parent.name)


def _write_demo_root_index(index_path: Path, runs: list) -> None:
    rows = "\n".join(
        "<tr>"
        f"<td>{run.data_source}</td>"
        f"<td>{run.scenario}</td>"
        f"<td>{len(run.json_paths)}</td>"
        f"<td>{len(run.failed_runs)}</td>"
        f"<td><a href=\"{run.index_path.relative_to(index_path.parent)}\">index.html</a></td>"
        "</tr>"
        for run in runs
        if run.index_path is not None
    )
    index_path.parent.mkdir(parents=True, exist_ok=True)
    index_path.write_text(
        "\n".join(
            [
                "<!doctype html>",
                '<html lang="en"><head><meta charset="utf-8">',
                "<title>ArchScope Demo Site Bundles</title>",
                "<style>body{max-width:960px;margin:32px auto;font-family:Arial,sans-serif}"
                "table{width:100%;border-collapse:collapse}"
                "th,td{padding:10px;border-bottom:1px solid #ddd;text-align:left}</style>",
                "</head><body>",
                "<h1>ArchScope Demo Site Bundles</h1>",
                "<table><thead><tr><th>Data source</th><th>Scenario</th>"
                "<th>JSON outputs</th><th>Failed</th><th>Bundle</th></tr></thead>",
                f"<tbody>{rows}</tbody></table>",
                "</body></html>",
            ]
        ),
        encoding="utf-8",
    )


def main() -> None:
    app()


def _parse_optional_datetime(value: str | None, option_name: str) -> datetime | None:
    if value is None:
        return None
    try:
        return datetime.fromisoformat(value)
    except ValueError as exc:
        raise typer.BadParameter(
            f"{option_name} must be an ISO 8601 datetime."
        ) from exc


def _debug_collector(
    *,
    analyzer_type: str,
    source_file: Path | None,
    parser: str,
    parser_options: dict[str, object],
    debug_log: bool,
    debug_log_dir: Path | None,
) -> DebugLogCollector:
    if source_file is None:
        raise typer.BadParameter("Analyzer source file is required.")
    return DebugLogCollector(
        analyzer_type=analyzer_type,
        source_file=source_file,
        parser=parser,
        parser_options=parser_options,
        debug_log_dir=debug_log_dir or default_debug_log_dir(),
        force_write=debug_log,
    )


def _write_result_with_debug(
    out: Path,
    collector: DebugLogCollector,
    analyze: Callable[[], AnalysisResult],
    label: str,
) -> None:
    try:
        result = analyze()
    except Exception as exc:
        collector.add_exception(phase="analysis", exception=exc)
        debug_path = collector.write()
        if debug_path is not None:
            console.print(f"Wrote parser debug log: {debug_path}")
        raise

    write_json_result(result, out)
    debug_path = collector.write(diagnostics=_diagnostics_from_result(result))
    console.print(f"Wrote {label} result: {out}")
    if debug_path is not None:
        console.print(f"Wrote parser debug log: {debug_path}")


def _diagnostics_from_result(result: AnalysisResult) -> dict[str, object] | None:
    diagnostics = result.metadata.get("diagnostics")
    return diagnostics if isinstance(diagnostics, dict) else None


if __name__ == "__main__":
    main()
