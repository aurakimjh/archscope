from __future__ import annotations

from datetime import datetime
from pathlib import Path
from typing import Optional

import typer
from rich.console import Console

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.analyzers.exception_analyzer import analyze_exception_stack
from archscope_engine.analyzers.gc_log_analyzer import analyze_gc_log
from archscope_engine.analyzers.jfr_analyzer import analyze_jfr_print_json
from archscope_engine.analyzers.profiler_analyzer import (
    analyze_collapsed_profile,
    analyze_jennifer_csv_profile,
    breakdown_collapsed_profile,
    breakdown_jennifer_csv_profile,
    drilldown_collapsed_profile,
    drilldown_jennifer_csv_profile,
)
from archscope_engine.analyzers.profiler_drilldown import DrilldownFilter
from archscope_engine.analyzers.thread_dump_analyzer import analyze_thread_dump
from archscope_engine.exporters.json_exporter import write_json_result

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


@access_log_app.command("analyze")
def access_log_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    format: str = typer.Option("nginx", "--format"),
    max_lines: Optional[int] = typer.Option(None, "--max-lines"),
    start_time: Optional[str] = typer.Option(None, "--start-time"),
    end_time: Optional[str] = typer.Option(None, "--end-time"),
    out: Path = typer.Option(..., "--out"),
) -> None:
    """Analyze an access log and write an AnalysisResult JSON file."""
    if max_lines is not None and max_lines <= 0:
        raise typer.BadParameter("--max-lines must be a positive integer.")

    result = analyze_access_log(
        file,
        log_format=format,
        max_lines=max_lines,
        start_time=_parse_optional_datetime(start_time, "--start-time"),
        end_time=_parse_optional_datetime(end_time, "--end-time"),
    )
    write_json_result(result, out)
    console.print(f"Wrote access log result: {out}")


@profiler_app.command("analyze-collapsed")
def profiler_analyze_collapsed(
    wall: Path = typer.Option(..., "--wall", exists=True, readable=True),
    wall_interval_ms: float = typer.Option(100, "--wall-interval-ms"),
    elapsed_sec: Optional[float] = typer.Option(None, "--elapsed-sec"),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
) -> None:
    """Analyze an async-profiler wall-clock collapsed file."""
    result = analyze_collapsed_profile(
        path=wall,
        interval_ms=wall_interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        profile_kind="wall",
    )
    write_json_result(result, out)
    console.print(f"Wrote profiler result: {out}")


@profiler_app.command("analyze-jennifer-csv")
def profiler_analyze_jennifer_csv(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    interval_ms: float = typer.Option(100, "--interval-ms"),
    elapsed_sec: Optional[float] = typer.Option(None, "--elapsed-sec"),
    top_n: int = typer.Option(20, "--top-n"),
) -> None:
    """Analyze a Jennifer APM flamegraph CSV file."""
    result = analyze_jennifer_csv_profile(
        path=file,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
    )
    write_json_result(result, out)
    console.print(f"Wrote Jennifer profiler result: {out}")


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
    if jennifer_csv is not None:
        result = drilldown_jennifer_csv_profile(
            path=jennifer_csv,
            interval_ms=wall_interval_ms,
            filters=filters,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
        )
    else:
        result = drilldown_collapsed_profile(
            path=wall,
            interval_ms=wall_interval_ms,
            filters=filters,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
        )
    write_json_result(result, out)
    console.print(f"Wrote profiler drill-down result: {out}")


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
    if jennifer_csv is not None:
        result = breakdown_jennifer_csv_profile(
            path=jennifer_csv,
            interval_ms=wall_interval_ms,
            filters=filters,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
        )
    else:
        result = breakdown_collapsed_profile(
            path=wall,
            interval_ms=wall_interval_ms,
            filters=filters,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
        )
    write_json_result(result, out)
    console.print(f"Wrote profiler breakdown result: {out}")


@jfr_app.command("analyze-json")
def jfr_analyze_json(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
) -> None:
    """Analyze JSON emitted by `jfr print --json`."""
    result = analyze_jfr_print_json(path=file, top_n=top_n)
    write_json_result(result, out)
    console.print(f"Wrote JFR result: {out}")


@gc_log_app.command("analyze")
def gc_log_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
) -> None:
    """Analyze a HotSpot unified GC log."""
    result = analyze_gc_log(path=file, top_n=top_n)
    write_json_result(result, out)
    console.print(f"Wrote GC log result: {out}")


@thread_dump_app.command("analyze")
def thread_dump_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
) -> None:
    """Analyze a Java thread dump text file."""
    result = analyze_thread_dump(path=file, top_n=top_n)
    write_json_result(result, out)
    console.print(f"Wrote thread dump result: {out}")


@exception_app.command("analyze")
def exception_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    out: Path = typer.Option(..., "--out"),
    top_n: int = typer.Option(20, "--top-n"),
) -> None:
    """Analyze Java exception stack traces."""
    result = analyze_exception_stack(path=file, top_n=top_n)
    write_json_result(result, out)
    console.print(f"Wrote exception result: {out}")


app.add_typer(access_log_app, name="access-log")
app.add_typer(profiler_app, name="profiler")
app.add_typer(jfr_app, name="jfr")
app.add_typer(gc_log_app, name="gc-log")
app.add_typer(thread_dump_app, name="thread-dump")
app.add_typer(exception_app, name="exception")


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


if __name__ == "__main__":
    main()
