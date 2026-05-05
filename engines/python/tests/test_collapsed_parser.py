import pytest

from archscope_engine.parsers.collapsed_parser import (
    parse_collapsed_file_with_diagnostics,
    parse_collapsed_files_with_diagnostics,
    parse_collapsed_line,
)


def test_parse_collapsed_line() -> None:
    stack, samples = parse_collapsed_line("frame1;frame2;frame3 123")

    assert stack == "frame1;frame2;frame3"
    assert samples == 123


def test_parse_collapsed_file_reports_malformed_line_diagnostics(tmp_path) -> None:
    path = tmp_path / "profile.collapsed"
    path.write_text(
        "\n".join(
            [
                "frame1;frame2 10",
                "",
                "frame1;frame2 15",
                "frame3;frame4",
                "frame5;frame6 abc",
                "frame7;frame8 -1",
            ]
        ),
        encoding="utf-8",
    )

    result = parse_collapsed_file_with_diagnostics(path)

    assert result.stacks == {"frame1;frame2": 25}
    assert result.diagnostics | {
        "source_file": result.diagnostics["source_file"],
        "format": result.diagnostics["format"],
        "warning_count": result.diagnostics["warning_count"],
        "error_count": result.diagnostics["error_count"],
        "warnings": result.diagnostics["warnings"],
        "errors": result.diagnostics["errors"],
    } == {
        "source_file": str(path),
        "format": "async_profiler_collapsed",
        "total_lines": 6,
        "parsed_records": 2,
        "skipped_lines": 3,
        "skipped_by_reason": {
            "MISSING_SAMPLE_COUNT": 1,
            "INVALID_SAMPLE_COUNT": 1,
            "NEGATIVE_SAMPLE_COUNT": 1,
        },
        "samples": [
            {
                "line_number": 4,
                "reason": "MISSING_SAMPLE_COUNT",
                "message": "Line must contain a stack and trailing sample count.",
                "raw_preview": "frame3;frame4",
            },
            {
                "line_number": 5,
                "reason": "INVALID_SAMPLE_COUNT",
                "message": "Sample count must be an integer.",
                "raw_preview": "frame5;frame6 abc",
            },
            {
                "line_number": 6,
                "reason": "NEGATIVE_SAMPLE_COUNT",
                "message": "Sample count must be non-negative.",
                "raw_preview": "frame7;frame8 -1",
            },
        ],
        "warning_count": 0,
        "error_count": 3,
        "warnings": [],
        "errors": result.diagnostics["samples"],
    }


def test_parse_collapsed_accepts_spaces_and_integral_numeric_samples(tmp_path) -> None:
    path = tmp_path / "profile.collapsed"
    path.write_text(
        "frame with spaces;child frame 1e3\n"
        "frame with spaces;child frame 0\n",
        encoding="utf-8",
    )

    result = parse_collapsed_file_with_diagnostics(path)

    assert result.stacks == {"frame with spaces;child frame": 1000}
    assert result.diagnostics["parsed_records"] == 2
    assert result.diagnostics["skipped_lines"] == 0


def test_parse_collapsed_rejects_fractional_samples(tmp_path) -> None:
    path = tmp_path / "profile.collapsed"
    path.write_text("frame;child 1.5\n", encoding="utf-8")

    result = parse_collapsed_file_with_diagnostics(path)

    assert result.stacks == {}
    assert result.diagnostics["skipped_by_reason"] == {"INVALID_SAMPLE_COUNT": 1}
    assert result.diagnostics["samples"][0]["message"] == (
        "Sample count must be a whole number."
    )


def test_parse_collapsed_strict_mode_fails_fast(tmp_path) -> None:
    path = tmp_path / "profile.collapsed"
    path.write_text("frame;child bad\nframe;ok 1\n", encoding="utf-8")

    with pytest.raises(ValueError, match="INVALID_SAMPLE_COUNT"):
        parse_collapsed_file_with_diagnostics(path, strict=True)


def test_parse_collapsed_multiple_files_merge_stacks(tmp_path) -> None:
    first = tmp_path / "one.collapsed"
    second = tmp_path / "two.collapsed"
    first.write_text("a;b 2\n", encoding="utf-8")
    second.write_text("a;b 3\nc;d 4\n", encoding="utf-8")

    result = parse_collapsed_files_with_diagnostics((first, second))

    assert result.stacks == {"a;b": 5, "c;d": 4}
    assert result.diagnostics["total_lines"] == 3
    assert result.diagnostics["parsed_records"] == 3
