from pathlib import Path

from archscope_engine.analyzers.profiler_analyzer import analyze_collapsed_profile
from archscope_engine.parsers.collapsed_parser import (
    parse_collapsed_file_with_diagnostics,
    parse_collapsed_line,
)


def test_parse_collapsed_line() -> None:
    stack, samples = parse_collapsed_line("frame1;frame2;frame3 123")

    assert stack == "frame1;frame2;frame3"
    assert samples == 123


def test_analyze_collapsed_merges_duplicate_stacks() -> None:
    sample = Path(__file__).parents[3] / "examples/profiler/sample-wall.collapsed"

    result = analyze_collapsed_profile(sample, interval_ms=100, elapsed_sec=1336.559)

    assert result.type == "profiler_collapsed"
    assert result.summary["total_samples"] == 32629
    assert result.summary["estimated_seconds"] == 3262.9


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
    assert result.diagnostics == {
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
    }


def test_analyze_collapsed_profile_includes_diagnostics_metadata(tmp_path) -> None:
    path = tmp_path / "profile.collapsed"
    path.write_text("frame1;frame2 10\nbad-line\n", encoding="utf-8")

    result = analyze_collapsed_profile(path, interval_ms=100)

    assert result.summary["total_samples"] == 10
    assert result.metadata["diagnostics"]["total_lines"] == 2
    assert result.metadata["diagnostics"]["parsed_records"] == 1
    assert result.metadata["diagnostics"]["skipped_lines"] == 1
    assert result.metadata["diagnostics"]["skipped_by_reason"] == {
        "MISSING_SAMPLE_COUNT": 1
    }
