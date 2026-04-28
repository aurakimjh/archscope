import pytest

from archscope_engine.parsers.gc_log_parser import parse_gc_log


def test_gc_log_parser_placeholder() -> None:
    with pytest.raises(NotImplementedError):
        parse_gc_log("sample.log")  # type: ignore[arg-type]
