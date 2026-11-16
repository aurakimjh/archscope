"""Independently read native Save outputs; never replace them with helper output."""
import csv
import argparse
import hashlib
import io
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
PRIVATE = ROOT / ".tmp/windows-validation-20261116"
PUBLIC = Path(__file__).resolve().parent
UNITS = ["samples", "nanoseconds", "microseconds", "milliseconds", "seconds", "bytes", "unknown"]

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--arch", choices=["amd64", "386"], default="amd64")
arch = parser.parse_args().arch
suffix = "" if arch == "amd64" else "-386"
native_path = PRIVATE / ("chart-amd64-4" if arch == "amd64" else "chart-386") / "layout.json"
native = json.loads(native_path.read_text(encoding="utf-8"))
assert native["report"]["passed"] and native["osScreenScaleFactor"] == native["webviewZoom"] == 1
assert [case["unit"] for case in native["report"]["cases"]] == UNITS
assert all(case["nativeSaveCompleted"] and case["svgAxisObserved"] and case["canvasObserved"] for case in native["report"]["cases"])
fixture = PRIVATE / "units.collapsed"
assert fixture.read_bytes() == b"Root;work 1000000\n"
destination = PUBLIC / ("chart-exports" + suffix)
destination.mkdir(exist_ok=True)
cases = []
for unit in UNITS:
    filename = unit + ".csv"
    data = (PRIVATE / ("chart-exports" + suffix) / filename).read_bytes()
    rows = list(csv.DictReader(io.StringIO(data.decode("utf-8-sig"))))
    assert len(rows) == 1
    row = rows[0]
    assert row["label"] == "Other" and row["value"] == "1000000"
    assert row["value_unit"] == unit and row["template_id"] == "profiler.components" and row["result_type"] == "profiler_collapsed"
    assert json.loads(row["source_files"]) == [fixture.as_posix()]
    if arch == "386":
        assert data == (PUBLIC / "chart-exports" / filename).read_bytes(), "x64/x86 native saved CSV bytes differ"
    (destination / filename).write_bytes(data)
    cases.append({"unit": unit, "value": 1000000, "sourceCount": 1, "csvSha256": hashlib.sha256(data).hexdigest(), "passed": True})
(PUBLIC / ("chart-native" + suffix + ".json")).write_bytes(native_path.read_bytes())
report = {"passed": True, "scope": arch + " Windows actual native profiler/Chart Studio SVG axis and seven CSV Save dialogs; Canvas surface creation only, no physical drop/OS125/PNG/SVG export matrix closure", "cases": cases, "fixtureSha256": hashlib.sha256(fixture.read_bytes()).hexdigest(), "nativeReportSha256": hashlib.sha256(native_path.read_bytes()).hexdigest()}
if arch == "386":
    report["nativeSavedCsvBytesMatchAmd64"] = True
(PUBLIC / ("chart-readback" + suffix + ".json")).write_text(json.dumps(report,indent=2)+"\n",encoding="utf-8")
print(json.dumps({"nativeSaveOutputs":len(cases),"passed":True}))
