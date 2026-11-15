#!/usr/bin/env python3
"""Read-only audit of committed Windows receipts; never opens private stores."""
import collections
import csv
import hashlib
import io
import json
import math
from pathlib import Path
import statistics
import subprocess
import struct
import xml.etree.ElementTree as ET
import zlib

ROOT = Path(__file__).resolve().parents[4]
PREFIX = "docs/review/evidence/"
DIRS = [
    "2026-11-14-t613-readback-v2",
    "2026-11-14-t613-failures",
    "2026-11-14-t613-memory",
    "2026-11-14-windows-final",
]
HEAD = subprocess.check_output(["git", "rev-parse", "fbf2ce2c"], cwd=ROOT).decode().strip()

def blob(path):
    return subprocess.check_output(["git", "show", f"{HEAD}:{path}"], cwd=ROOT)

def read(path):
    return json.loads(blob(path))

report = {"reviewed_head": HEAD, "artifact_manifests": [], "memory": []}
for directory in DIRS:
    prefix = PREFIX + directory + "/"
    manifest = read(prefix + "artifact-sha256.json")
    files = manifest["files"]
    records = files if isinstance(files, list) else [dict(v, file=k) for k, v in files.items()]
    normalized = []
    for entry in records:
        path = prefix + entry["file"]
        data = blob(path)
        assert len(data) == entry["bytes"], (path, "length")
        assert hashlib.sha256(data).hexdigest() == entry["sha256"], (path, "digest")
        if entry.get("gitBlob"):
            oid = subprocess.check_output(["git", "rev-parse", f"{HEAD}:{path}"], cwd=ROOT).decode().strip()
            assert oid == entry["gitBlob"], (path, "Git blob")
        if (ROOT / path).read_bytes() != data:
            normalized.append(entry["file"])
    report["artifact_manifests"].append({"directory": directory, "verified_files": len(records), "worktree_byte_differences": normalized})

final = PREFIX + DIRS[-1] + "/"
validation = read(final + "final-validation.json")
for entry in validation["changedProductSource"]:
    assert hashlib.sha256(blob(entry["path"])).hexdigest() == entry["gitIndexSha256"], entry["path"]
report["pinned_product_sources"] = len(validation["changedProductSource"])
cases = read(final + "native-cases.json")
counts = dict(collections.Counter(case["result"] for case in cases))
assert len(cases) == validation["nativeCases"]
assert counts == validation["nativeCaseResultsIncludingPreservedBeforeFixFailures"]
report["native_cases"] = {"total": len(cases), "historical_results": counts}
remaining = read(final + "remaining-acceptance.json")
routes = remaining["T-627"]["routes"]
assert len(routes) == 21 and len({r["route"] for r in routes}) == 21
assert remaining["T-613"]["status"] == "PENDING"
assert remaining["T-627"]["status"] == "REVIEW"
report["remaining_inventory"] = {"routes": 21, "T-613": "PENDING", "T-627": "REVIEW"}

specs = [
    ("comparison-deltas-canvas", "comparison.deltas", "metric-specific", [-50, 25]),
    ("profiler-components-svg", "profiler.components", "samples", [20100]),
    ("archscope-trace.services", "trace.services", "ms", [3960, 840]),
    ("archscope-trace.dependencies", "trace.dependencies", "ms", [1080]),
    ("archscope-gc.pause", "gc.pause", "ms", [10+i % 12 for i in range(60)]),
    ("archscope-gc.heap", "gc.heap", "MB", [50]*60),
    ("archscope-access.requests", "access.requests", "count", [120]),
    ("archscope-access.p95", "access.p95", "ms", [1500]),
]
report["chart_exports"] = []
for stem, template, unit, expected in specs:
    prefix = final + "chart-exports/" + stem
    rows = list(csv.DictReader(io.StringIO(blob(prefix+".csv").decode("utf-8-sig"))))
    assert [float(row["value"]) for row in rows] == expected, stem
    assert all(row["value_unit"] == unit and row["template_id"] == template for row in rows)
    assert all(len(json.loads(row["source_files"])) == (2 if template == "comparison.deltas" else 1) for row in rows)
    svg = ET.fromstring(blob(prefix+".svg"))
    assert any(e.tag.endswith("path") for e in svg.iter())
    assert not any(e.tag.endswith("image") for e in svg.iter())
    png = blob(prefix+".png")
    assert png[:8] == b"\x89PNG\r\n\x1a\n"
    offset, compressed, dimensions = 8, [], None
    while offset < len(png):
        size, = struct.unpack(">I", png[offset:offset+4])
        kind = png[offset+4:offset+8]
        data = png[offset+8:offset+8+size]
        crc, = struct.unpack(">I", png[offset+8+size:offset+12+size])
        assert zlib.crc32(kind+data) == crc
        if kind == b"IHDR":
            width, height, depth, color, compression, filtering, interlace = struct.unpack(">IIBBBBB", data)
            assert width > 1000 and height > 500 and depth == 8 and color in (2, 6)
            assert (compression, filtering, interlace) == (0, 0, 0)
            dimensions = width, height, 3 if color == 2 else 4
        elif kind == b"IDAT":
            compressed.append(data)
        offset += size+12
    assert offset == len(png) and dimensions is not None
    width, height, channels = dimensions
    scanlines = zlib.decompress(b"".join(compressed))
    stride = width*channels+1
    assert len(scanlines) == height*stride
    assert all(scanlines[i*stride] <= 4 for i in range(height))
    report["chart_exports"].append({"template": template, "rows": len(rows), "unit": unit, "png_dimensions": [width, height]})
report["chart_validation_scope"] = "8 CSV values/units/source counts, 8 vector SVG structures, 8 PNG chunk CRC/zlib/geometry checks; not new Office/native rendering"

budgets = {
    "treePrivateBytes": (64 * 1048576, 4 * 1048576),
    "rootPrivateBytes": (16 * 1048576, 1048576),
    "rootHandles": (24, 2),
    "rootThreads": (4, 0.5),
}
memory_paths = [PREFIX + "2026-11-14-t613-memory/native-" + arch + ".json" for arch in ("amd64", "386")]
memory_paths += [final + name for name in (
    "import-initial-amd64.json", "import-initial-386.json",
    "import-history36-amd64.json", "import-history36-386.json",
    "import-history52-amd64.json", "body-memory-386-result.json",
)]
for path in memory_paths:
    doc = read(path)
    points = [c["settled"] for c in doc["cycles"] if not c["warmup"]]
    assert len(points) == 8, path
    identities = []
    for point in points:
        members = point["members"]
        assert len(members) == 7 and len({m["ordinal"] for m in members}) == 7
        identities.append(sorted((m["ordinal"], m["role"]) for m in members))
        assert sum(m["privateBytes"] for m in members) == point["treePrivateBytes"]
        root, = [m for m in members if m["role"] == "app"]
        for source, target in [("privateBytes", "rootPrivateBytes"), ("handles", "rootHandles"), ("threads", "rootThreads")]:
            assert root[source] == point[target]
    assert all(i == identities[0] for i in identities)
    computed = {}
    for metric, (delta_budget, slope_budget) in budgets.items():
        values = [p[metric] for p in points]
        assert all(isinstance(v, int) and not isinstance(v, bool) and v >= 0 for v in values)
        slopes = [(values[j]-values[i])/(j-i) for i in range(8) for j in range(i+1, 8)]
        first, last = statistics.median(values[:3]), statistics.median(values[-3:])
        delta, slope = last-first, statistics.median(slopes)
        expected = doc["memoryAssessment"]["metrics"][metric]
        for key, value in [("firstMedian", first), ("lastMedian", last), ("delta", delta), ("slopePerCycle", slope)]:
            assert math.isclose(value, expected[key], rel_tol=1e-12, abs_tol=1e-8), (path, metric, key)
        assert expected["budget"] == {"delta": delta_budget, "perCycle": slope_budget}
        within = delta <= delta_budget and slope <= slope_budget
        assert within == expected["withinBudget"]
        computed[metric] = {"delta": delta, "slope_per_cycle": slope, "within_budget": within}
    passed = all(m["within_budget"] for m in computed.values())
    assert passed == doc["memoryAssessment"]["withinBudget"] == doc["passed"]
    assert doc["memoryAssessment"]["classification"] == ("BOUNDED_IN_THIS_RUN" if passed else "GROWTH_REQUIRES_DIAGNOSIS")
    report["memory"].append({"file": path, "passed": passed, "stable_processes": 7, "measured_points": 8, "metrics": computed})

report["verified_artifact_files"] = sum(m["verified_files"] for m in report["artifact_manifests"])
print(json.dumps(report, ensure_ascii=False, indent=2))
