# -*- mode: python ; coding: utf-8 -*-
"""PyInstaller spec for archscope-engine.

Two build targets:

  onedir  (default) — produces dist/archscope-engine/<files>
                       Faster startup, easier to debug.
  onefile            — produces dist/archscope-engine (single binary)
                       Pass `--onefile` env or edit the EXE below.

Usage:
  cd engines/python
  pyinstaller archscope-engine.spec          # onedir
  ONEFILE=1 pyinstaller archscope-engine.spec  # onefile
"""
import os
import sys

ONEFILE = os.environ.get("ONEFILE", "0") == "1"

a = Analysis(
    ["archscope_engine/cli.py"],
    pathex=["."],
    binaries=[],
    datas=[
        ("archscope_engine/config/*.json", "archscope_engine/config"),
    ],
    hiddenimports=[
        # --- CLI ---
        "typer",
        "rich.console",
        "rich.markdown",
        "rich.syntax",
        "rich.table",
        "rich.progress",
        "click",
        "click.core",
        "click.decorators",
        "click.exceptions",
        "click.types",
        # --- Web / FastAPI ---
        "fastapi",
        "fastapi.middleware.cors",
        "fastapi.responses",
        "fastapi.staticfiles",
        "uvicorn",
        "uvicorn.logging",
        "uvicorn.loops",
        "uvicorn.loops.auto",
        "uvicorn.protocols",
        "uvicorn.protocols.http",
        "uvicorn.protocols.http.auto",
        "uvicorn.protocols.websockets",
        "uvicorn.protocols.websockets.auto",
        "uvicorn.lifespan",
        "uvicorn.lifespan.on",
        "starlette",
        "starlette.routing",
        "starlette.middleware",
        "starlette.responses",
        "starlette.staticfiles",
        "anyio",
        "anyio._backends",
        "anyio._backends._asyncio",
        "multipart",
        "multipart.multipart",
        "python_multipart",
        # --- Engine modules (ensure all analyzers are bundled) ---
        "archscope_engine.analyzers.access_log_analyzer",
        "archscope_engine.analyzers.exception_analyzer",
        "archscope_engine.analyzers.gc_log_analyzer",
        "archscope_engine.analyzers.jfr_analyzer",
        "archscope_engine.analyzers.lock_contention_analyzer",
        "archscope_engine.analyzers.multi_thread_analyzer",
        "archscope_engine.analyzers.profiler_analyzer",
        "archscope_engine.analyzers.thread_dump_analyzer",
        "archscope_engine.analyzers.thread_dump_to_collapsed",
        "archscope_engine.exporters.html_exporter",
        "archscope_engine.exporters.json_exporter",
        "archscope_engine.exporters.pptx_exporter",
        "archscope_engine.exporters.report_diff",
        "archscope_engine.web.server",
        "defusedxml",
    ],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[
        "pytest",
        "_pytest",
        "ruff",
        "setuptools",
        "distutils",
        "lib2to3",
        "pydoc",
        "doctest",
        "unittest",
        "tkinter",
        "_tkinter",
        "turtle",
        "xmlrpc",
        "ftplib",
        "imaplib",
        "mailbox",
        "nntplib",
        "poplib",
        "smtplib",
        "telnetlib",
    ],
    noarchive=False,
)

pyz = PYZ(a.pure)

if ONEFILE:
    exe = EXE(
        pyz,
        a.scripts,
        a.binaries,
        a.zipfiles,
        a.datas,
        name="archscope-engine",
        debug=False,
        bootloader_ignore_signals=False,
        strip=False,
        upx=False,
        console=True,
    )
else:
    exe = EXE(
        pyz,
        a.scripts,
        [],
        exclude_binaries=True,
        name="archscope-engine",
        debug=False,
        bootloader_ignore_signals=False,
        strip=False,
        upx=False,
        console=True,
    )

    coll = COLLECT(
        exe,
        a.binaries,
        a.zipfiles,
        a.datas,
        strip=False,
        upx=False,
        name="archscope-engine",
    )
