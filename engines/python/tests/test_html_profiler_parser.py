from collections import Counter

from archscope_engine.parsers.html_profiler_parser import parse_html_profiler_text


_INLINE_SVG_HTML = """<!doctype html>
<html><head><title>perf flamegraph</title></head>
<body>
<svg xmlns="http://www.w3.org/2000/svg" width="100" height="48">
  <g><title>all (10 samples, 100.00%)</title>
     <rect x="0" y="32" width="100" height="16"/></g>
  <g><title>main (10 samples, 100.00%)</title>
     <rect x="0" y="16" width="100" height="16"/></g>
  <g><title>compute (10 samples, 100.00%)</title>
     <rect x="0" y="0" width="100" height="16"/></g>
</svg>
</body></html>
"""

_ASYNC_PROFILER_HTML = """<!doctype html>
<html><body>
<canvas id='canvas'></canvas>
<script>
  var root = {
    name: "all", samples: 10, children: [
      { name: "main", samples: 6, children: [
        { name: "compute", samples: 3 },
        { name: "render", samples: 3 }
      ]},
      { name: "worker", samples: 4, children: [
        { name: "poll", samples: 4 }
      ]}
    ]
  };
</script>
</body></html>
"""

_ASYNC_PROFILER_HTML_WITH_SELF_TIME = """<!doctype html>
<html><body><script>
  const root = {
    name: "all", samples: 10, children: [
      { name: "main", samples: 6, children: [
        { name: "compute", samples: 3 }
      ]}
    ]
  };
</script></body></html>
"""

_UNSUPPORTED_HTML = """<!doctype html>
<html><body>
<h1>Just a regular dashboard, no flamegraph here.</h1>
<table><tr><th>Stat</th><th>Value</th></tr></table>
</body></html>
"""


def test_inline_svg_html_routes_to_svg_parser() -> None:
    result = parse_html_profiler_text(_INLINE_SVG_HTML)

    assert result.detected_format == "inline_svg"
    assert result.stacks == Counter({"all;main;compute": 10})
    assert result.diagnostics["parsed_records"] == 10


def test_async_profiler_html_extracts_js_tree() -> None:
    result = parse_html_profiler_text(_ASYNC_PROFILER_HTML)

    assert result.detected_format == "async_profiler_js"
    assert result.stacks == Counter(
        {
            "all;main;compute": 3,
            "all;main;render": 3,
            "all;worker;poll": 4,
        }
    )


def test_async_profiler_html_records_parent_self_time() -> None:
    # main has samples=6 but only one child with samples=3 → 3 self-time
    # samples for main, plus the 3 attributed to compute.
    result = parse_html_profiler_text(_ASYNC_PROFILER_HTML_WITH_SELF_TIME)

    assert result.detected_format == "async_profiler_js"
    assert result.stacks == Counter(
        {
            "all;main": 3,
            "all;main;compute": 3,
            # all has samples=10 but children sum to 6 → 4 self-time on root
            "all": 4,
        }
    )


def test_unsupported_html_emits_diagnostic() -> None:
    result = parse_html_profiler_text(_UNSUPPORTED_HTML)

    assert result.detected_format == "unsupported"
    assert result.stacks == Counter()
    assert "UNSUPPORTED_HTML_FORMAT" in result.diagnostics["skipped_by_reason"]
