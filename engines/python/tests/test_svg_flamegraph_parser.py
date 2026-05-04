from collections import Counter

from archscope_engine.parsers.svg_flamegraph_parser import (
    parse_svg_flamegraph,
    parse_svg_flamegraph_text,
)


# Brendan Gregg layout: row 0 == ROOT (bottom), row 1 == middle, row 2 == leaves (top).
# Width is proportional to samples (1 unit per sample for easy reading).
_BRENDAN_SVG = """<?xml version="1.0" standalone="no"?>
<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="100" height="48">
  <g class="func_g">
    <title>all (10 samples, 100.00%)</title>
    <rect x="0" y="32" width="100" height="16" fill="#ff8800"/>
  </g>
  <g class="func_g">
    <title>main (6 samples, 60.00%)</title>
    <rect x="0" y="16" width="60" height="16" fill="#ff9900"/>
  </g>
  <g class="func_g">
    <title>worker (4 samples, 40.00%)</title>
    <rect x="60" y="16" width="40" height="16" fill="#ffaa00"/>
  </g>
  <g class="func_g">
    <title>compute (3 samples, 30.00%)</title>
    <rect x="0" y="0" width="30" height="16" fill="#ff7700"/>
  </g>
  <g class="func_g">
    <title>render (3 samples, 30.00%)</title>
    <rect x="30" y="0" width="30" height="16" fill="#ff6600"/>
  </g>
  <g class="func_g">
    <title>poll (4 samples, 40.00%)</title>
    <rect x="60" y="0" width="40" height="16" fill="#ff5500"/>
  </g>
</svg>
"""

# Icicle layout (root at top, leaves at bottom). Same logical tree.
_ICICLE_SVG = """<?xml version="1.0" standalone="no"?>
<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="100" height="48">
  <g><title>all (10 samples, 100.00%)</title>
     <rect x="0" y="0" width="100" height="16"/></g>
  <g><title>main (6 samples, 60.00%)</title>
     <rect x="0" y="16" width="60" height="16"/></g>
  <g><title>worker (4 samples, 40.00%)</title>
     <rect x="60" y="16" width="40" height="16"/></g>
  <g><title>compute (3 samples, 30.00%)</title>
     <rect x="0" y="32" width="30" height="16"/></g>
  <g><title>render (3 samples, 30.00%)</title>
     <rect x="30" y="32" width="30" height="16"/></g>
  <g><title>poll (4 samples, 40.00%)</title>
     <rect x="60" y="32" width="40" height="16"/></g>
</svg>
"""


def test_parse_brendan_default_layout_yields_collapsed_stacks() -> None:
    result = parse_svg_flamegraph_text(_BRENDAN_SVG)

    assert result.stacks == Counter(
        {
            "all;main;compute": 3,
            "all;main;render": 3,
            "all;worker;poll": 4,
        }
    )
    diagnostics = result.diagnostics
    assert diagnostics["total_lines"] == 6
    assert diagnostics["parsed_records"] == 10
    assert diagnostics["skipped_lines"] == 0


def test_parse_icicle_layout_yields_same_stacks() -> None:
    result = parse_svg_flamegraph_text(_ICICLE_SVG)

    assert result.stacks == Counter(
        {
            "all;main;compute": 3,
            "all;main;render": 3,
            "all;worker;poll": 4,
        }
    )


def test_parse_handles_comma_grouped_sample_count() -> None:
    svg = """<?xml version="1.0"?>
    <svg xmlns="http://www.w3.org/2000/svg">
      <g><title>all (12,345 samples, 100.00%)</title>
         <rect x="0" y="16" width="100" height="16"/></g>
      <g><title>main (12,345 samples, 100.00%)</title>
         <rect x="0" y="0" width="100" height="16"/></g>
    </svg>
    """

    result = parse_svg_flamegraph_text(svg)

    assert result.stacks == Counter({"all;main": 12_345})


def test_parse_skips_titles_without_sample_marker() -> None:
    svg = """<?xml version="1.0"?>
    <svg xmlns="http://www.w3.org/2000/svg">
      <g><title>Search</title><rect x="0" y="0" width="50" height="16"/></g>
      <g><title>all (5 samples, 100%)</title>
         <rect x="0" y="32" width="100" height="16"/></g>
      <g><title>leaf (5 samples, 100%)</title>
         <rect x="0" y="16" width="100" height="16"/></g>
    </svg>
    """

    result = parse_svg_flamegraph_text(svg)

    assert result.stacks == Counter({"all;leaf": 5})
    skipped_reasons = result.diagnostics["skipped_by_reason"]
    assert skipped_reasons.get("UNPARSEABLE_TITLE", 0) >= 1


def test_invalid_svg_returns_empty_stacks_and_diagnostics() -> None:
    result = parse_svg_flamegraph_text("<svg><not closed")

    assert result.stacks == Counter()
    assert "INVALID_SVG" in result.diagnostics["skipped_by_reason"]


def test_parse_svg_flamegraph_reads_from_disk(tmp_path) -> None:
    path = tmp_path / "flame.svg"
    path.write_text(_BRENDAN_SVG, encoding="utf-8")

    result = parse_svg_flamegraph(path)

    assert sum(result.stacks.values()) == 10
    assert "all;main;compute" in result.stacks
