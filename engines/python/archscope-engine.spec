# -*- mode: python ; coding: utf-8 -*-

a = Analysis(
    ['archscope_engine/cli.py'],
    pathex=['.'],
    binaries=[],
    datas=[
        ('archscope_engine/config/*.json', 'archscope_engine/config'),
    ],
    hiddenimports=[
        'typer',
        'rich.console',
        'rich.markdown',
        'rich.syntax',
        'rich.table',
        'rich.progress',
        'click',
        'click.core',
        'click.decorators',
        'click.exceptions',
        'click.types',
    ],
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=[
        'pytest',
        '_pytest',
        'ruff',
        'setuptools',
        'distutils',
        'lib2to3',
        'pydoc',
        'doctest',
        'unittest',
        'tkinter',
        '_tkinter',
        'turtle',
        'xmlrpc',
        'ftplib',
        'imaplib',
        'mailbox',
        'nntplib',
        'poplib',
        'smtplib',
        'telnetlib',
    ],
    noarchive=False,
)

pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    [],
    exclude_binaries=True,
    name='archscope-engine',
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
    name='archscope-engine',
)
