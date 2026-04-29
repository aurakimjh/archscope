from archscope_engine.common.file_utils import iter_text_lines


def test_iter_text_lines_does_not_duplicate_lines_after_fallback(tmp_path) -> None:
    path = tmp_path / "fallback.log"
    prefix_lines = [f"line-{index}" for index in range(10_000)]
    path.write_bytes(("\n".join(prefix_lines) + "\n").encode("utf-8") + b"\x81\n")

    lines = list(iter_text_lines(path))

    assert lines == prefix_lines + ["\x81"]


def test_iter_text_lines_reads_utf8_lines(tmp_path) -> None:
    path = tmp_path / "utf8.log"
    path.write_text("alpha\nbeta\n", encoding="utf-8")

    assert list(iter_text_lines(path)) == ["alpha", "beta"]
