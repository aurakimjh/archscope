"""``archscope`` console script — the user-facing wrapper for T-208.

Goal: a single ``pip install archscope`` (the engine wheel renames to
this short distribution name in T-208) installs the entire stack
(engine + bundled React frontend) and exposes one binary that does the
right thing without flags::

    archscope serve            # uvicorn on 127.0.0.1:8765, opens browser
    archscope <subcommand>     # any analyzer/report/demo CLI command

This file builds a thin :class:`typer.Typer` wrapper around
:mod:`archscope_engine.cli` so the renamed binary inherits every
subcommand the existing ``archscope-engine`` already has, and overrides
``serve`` with a UX-friendly variant (open the browser by default,
auto-resolve the bundled React static dir).
"""
from __future__ import annotations

import os
import threading
import time
import webbrowser
from pathlib import Path
from typing import Optional

import typer

from archscope_engine.cli import app as engine_app


def _open_browser_after_delay(url: str, delay_seconds: float = 1.5) -> None:
    """Open the default browser ``delay_seconds`` after uvicorn starts.

    Run in a daemon thread so a failure to launch the browser never
    keeps the server alive on shutdown.
    """

    def _open() -> None:
        try:
            time.sleep(delay_seconds)
            webbrowser.open(url, new=2)
        except Exception:  # noqa: BLE001 — best-effort; failure is non-fatal.
            return

    thread = threading.Thread(target=_open, daemon=True, name="archscope-open-browser")
    thread.start()


# Build a fresh Typer app that inherits every command group from the
# engine CLI, then attaches our friendlier ``serve``. Using add_typer
# keeps every subcommand discoverable under ``archscope --help``.
app = typer.Typer(
    name="archscope",
    help=(
        "ArchScope — local-first application architecture diagnostic toolkit. "
        "Runs the FastAPI server with the bundled React UI by default; every "
        "subcommand from the analysis engine is also reachable here."
    ),
    no_args_is_help=False,
    invoke_without_command=True,
)


@app.callback()
def _root(ctx: typer.Context) -> None:
    """When invoked with no subcommand, fall through to ``serve``."""
    if ctx.invoked_subcommand is None:
        ctx.invoke(serve)


# Inherit every command / typer-group from the engine CLI without
# duplicating their definitions. The engine's `serve` typer group is
# intentionally skipped so the renamed binary's friendlier `serve`
# command (auto-opens the browser) is the only one bound at this level.
for command in engine_app.registered_commands:
    app.registered_commands.append(command)
for group in engine_app.registered_groups:
    if group.name == "serve":
        continue
    app.registered_groups.append(group)


@app.command(
    "serve",
    context_settings={"allow_extra_args": False, "ignore_unknown_options": False},
)
def serve(
    host: str = typer.Option("127.0.0.1", "--host", help="Bind address."),
    port: int = typer.Option(8765, "--port", help="Bind port."),
    static_dir: Optional[Path] = typer.Option(
        None,
        "--static-dir",
        help=(
            "Override the React static bundle directory. By default the "
            "wheel-bundled assets at archscope_engine.web.static are used; "
            "the dev-tree fallback resolves to apps/frontend/dist."
        ),
    ),
    no_dev_cors: bool = typer.Option(
        False,
        "--no-dev-cors",
        help="Disable the development CORS allow-list for the Vite origin.",
    ),
    reload: bool = typer.Option(
        False,
        "--reload",
        help="Enable uvicorn auto-reload for development.",
    ),
    no_browser: bool = typer.Option(
        False,
        "--no-browser",
        help="Do not open the default browser on startup.",
    ),
) -> None:
    """Start the FastAPI server and open the bundled React UI in the browser."""
    from archscope_engine.web import run as run_web_server

    url = f"http://{host}:{port}"
    if not no_browser and not _is_truthy(os.environ.get("ARCHSCOPE_NO_BROWSER")):
        _open_browser_after_delay(url)
    typer.echo(f"ArchScope serving on {url} — Ctrl+C to stop.")
    run_web_server(
        host=host,
        port=port,
        static_dir=static_dir,
        dev_cors=not no_dev_cors,
        reload=reload,
    )


def _is_truthy(value: Optional[str]) -> bool:
    if not value:
        return False
    return value.strip().lower() in {"1", "true", "yes", "on"}


def main() -> None:
    """Console-script entry point for the ``archscope`` binary."""
    app()


__all__ = ["app", "main"]
