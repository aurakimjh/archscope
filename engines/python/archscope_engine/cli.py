from __future__ import annotations

from pathlib import Path
from typing import Optional

import typer
from rich.console import Console

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.analyzers.profiler_analyzer import analyze_collapsed_profile
from archscope_engine.exporters.json_exporter import write_json_result

console = Console()

app = typer.Typer(
    help="ArchScope analysis engine CLI.",
    no_args_is_help=True,
)
access_log_app = typer.Typer(help="Access log analysis commands.")
profiler_app = typer.Typer(help="Profiler analysis commands.")


@access_log_app.command("analyze")
def access_log_analyze(
    file: Path = typer.Option(..., "--file", exists=True, readable=True),
    format: str = typer.Option("nginx", "--format"),
    out: Path = typer.Option(..., "--out"),
) -> None:
    """Analyze an access log and write an AnalysisResult JSON file."""
    result = analyze_access_log(file, log_format=format)
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


app.add_typer(access_log_app, name="access-log")
app.add_typer(profiler_app, name="profiler")


def main() -> None:
    app()


if __name__ == "__main__":
    main()
