"""Independent readback of the 24 files saved through native Chart Studio."""
import csv
import hashlib
import io
import json
import shutil
import sys
import xml.etree.ElementTree as ET
from pathlib import Path
from PIL import Image

source = Path(sys.argv[1])
destination = Path(__file__).parent / "chart-exports"
destination.mkdir(exist_ok=True)
specs = [
    ("comparison-deltas-canvas", "comparison.deltas", "metric-specific", "comparison_report", [-50, 25], 2),
    ("profiler-components-svg", "profiler.components", "samples", "profiler_collapsed", [20100], 1),
    ("archscope-trace.services", "trace.services", "ms", "trace_import", [3960, 840], 1),
    ("archscope-trace.dependencies", "trace.dependencies", "ms", "trace_import", [1080], 1),
    ("archscope-gc.pause", "gc.pause", "ms", "gc_log", [10 + i % 12 for i in range(60)], 1),
    ("archscope-gc.heap", "gc.heap", "MB", "gc_log", [50] * 60, 1),
    ("archscope-access.requests", "access.requests", "count", "access_log", [120], 1),
    ("archscope-access.p95", "access.p95", "ms", "access_log", [1500], 1),
]
receipts = []
for stem, template, unit, result_type, expected, source_count in specs:
    rows = list(csv.DictReader(io.StringIO((source / (stem + ".csv")).read_text(encoding="utf-8-sig"))))
    assert [float(row["value"]) for row in rows] == expected, template
    for row in rows:
        assert row["template_id"] == template
        assert row["value_unit"] == unit
        assert row["result_type"] == result_type
        paths = json.loads(row["source_files"])
        assert len(paths) == source_count and all(paths), template
    files = []
    for extension in ("csv", "png", "svg"):
        path = source / (stem + "." + extension)
        data = path.read_bytes()
        extra = {}
        if extension == "png":
            with Image.open(path) as img:
                img.load()
                assert img.format == "PNG" and img.width > 1000 and img.height > 500
                extrema = img.convert("RGB").getextrema()
                assert any(lo < hi for lo, hi in extrema)
                extra = {"width": img.width, "height": img.height}
        if extension == "svg":
            root = ET.fromstring(data)
            assert root.tag.endswith("svg")
            assert any(e.tag.endswith("path") for e in root.iter())
            assert not any(e.tag.endswith("image") for e in root.iter()), "SVG must remain vector"
            extra = {"width": root.attrib.get("width"), "height": root.attrib.get("height")}
        shutil.copyfile(path, destination / path.name)
        files.append({"file": "chart-exports/" + path.name, "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest(), **extra})
    receipts.append({"template": template, "unit": unit, "resultType": result_type, "rows": len(rows), "sourceFiles": json.loads(rows[0]["source_files"]), "values": expected, "files": files})
report = {"passed": True, "buildCommit": "e50b0f48-plus-chart-csv", "nativeRenderer": {"comparison.deltas": "canvas", "otherSeven": "svg"}, "scope": "8 templates x 3 output formats; OS100/Web100/KO; not every renderer/locale/DPI Cartesian combination", "templates": receipts}
(destination.parent / "chart-export-readback.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print("PASS: 8 templates, 24 native exports, independent numeric/unit/provenance/vector/raster readback")
