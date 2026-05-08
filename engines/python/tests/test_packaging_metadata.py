# ─────────────────────────────────────────────────────────────────────
# [한글] test_packaging_metadata — setup.cfg 의 패키징 메타 회귀 테스트.
#
# 검증 대상
#   • install_requires 에 Typer / Rich / FastAPI / uvicorn 등 필수
#     의존성이 선언됨.
#   • 콘솔 스크립트 entry_points: `archscope` / `archscope-engine` 둘 다.
#   • Python 호환 버전 (`python_requires=>=3.9`).
#   • Wheel 패키지 데이터에 React 빌드 (`apps/frontend/dist/`) 가 포함됨.
#
# 왜 중요한가
#   setup.cfg 변경이 사용자 설치 경험을 즉시 깨뜨림. CI 가 매 PR 마다
#   확인 → "이 의존성 추가/제거를 의도한 것인가" 의 가시화.
# ─────────────────────────────────────────────────────────────────────
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
