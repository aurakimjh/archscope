from __future__ import annotations

import configparser
from pathlib import Path


def test_runtime_dependencies_are_declared() -> None:
    config = configparser.ConfigParser()
    config.read(Path(__file__).parents[1] / "setup.cfg")

    requirements = {
        requirement.strip()
        for requirement in config["options"]["install_requires"].splitlines()
        if requirement.strip()
    }

    assert "typer>=0.12,<1.0" in requirements
    assert "rich>=13,<15" in requirements


def test_long_description_file_exists() -> None:
    root = Path(__file__).parents[1]
    config = configparser.ConfigParser()
    config.read(root / "setup.cfg")

    long_description = config["metadata"]["long_description"]
    assert long_description.startswith("file: ")
    assert (root / long_description.removeprefix("file: ").strip()).is_file()


def test_console_script_is_declared() -> None:
    config = configparser.ConfigParser()
    config.read(Path(__file__).parents[1] / "setup.cfg")

    entry_points = config["options.entry_points"]["console_scripts"]

    assert "archscope-engine = archscope_engine.cli:main" in entry_points
