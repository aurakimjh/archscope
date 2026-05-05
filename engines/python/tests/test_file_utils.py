from archscope_engine.common.file_utils import detect_text_encoding, iter_text_lines


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


def test_detect_text_encoding_uses_bounded_probe(tmp_path) -> None:
    path = tmp_path / "late-invalid.log"
    path.write_bytes(b"alpha\nbeta\n" + "안녕\n".encode("cp949"))

    assert detect_text_encoding(path, probe_bytes=8) == "utf-8"
    assert detect_text_encoding(path) == "cp949"


def test_detect_text_encoding_handles_utf16le_bom(tmp_path) -> None:
    path = tmp_path / "jstack-utf16le.txt"
    path.write_text("Full thread dump\n", encoding="utf-16")

    assert detect_text_encoding(path) == "utf-16"
    assert list(iter_text_lines(path)) == ["Full thread dump"]


def test_detect_text_encoding_handles_utf16le_without_bom(tmp_path) -> None:
    path = tmp_path / "jstack-utf16le-nobom.txt"
    path.write_bytes("Full thread dump\n".encode("utf-16-le"))

    assert detect_text_encoding(path) == "utf-16-le"
    assert list(iter_text_lines(path)) == ["Full thread dump"]


def test_detect_text_encoding_rejects_invalid_probe_size(tmp_path) -> None:
    path = tmp_path / "utf8.log"
    path.write_text("alpha\n", encoding="utf-8")

    try:
        detect_text_encoding(path, probe_bytes=0)
    except ValueError as error:
        assert str(error) == "probe_bytes must be a positive integer."
    else:
        raise AssertionError("Expected invalid probe size to be rejected.")
