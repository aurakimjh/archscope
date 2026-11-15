"""Independent fixed-protocol arithmetic, identities and immutable-file checks."""
from pathlib import Path
import argparse
import hashlib
import json
import math
import statistics
import struct

parser = argparse.ArgumentParser()
parser.add_argument("--available", action="store_true", help="Archive completed runs without claiming all planned runs finished")
args = parser.parse_args()
root = Path.cwd()
evidence = root / "docs/review/evidence/2026-11-15-t613-memory-diagnosis"
private = root / ".tmp/t613-memory-diagnosis-20261115"
read = lambda p: json.loads(p.read_text(encoding="utf-8-sig"))
sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
names = ["baseline-amd64", "fixed-amd64-1", "fixed-amd64-2"]
plan = read(evidence / "protocol.json")
assert plan["primaryRuns"] == names
budgets = {"treePrivateBytes": (64*1048576, 4*1048576), "rootPrivateBytes": (16*1048576, 1048576), "rootHandles": (24, 2), "rootThreads": (4, .5)}
result = {"scope": "Independent finite Windows import screens; original failures unchanged; no universal leak-free claim", "complete": True, "runs": []}
fixture_sha = None
fixed_binary_sha = None
for name in names:
    path = private / name / "result.json"
    if not path.exists():
        if args.available:
            result["complete"] = False
            continue
        raise ValueError("planned run missing: " + name)
    report = read(path)
    protocol = report["protocol"]
    assert protocol == {"cycles": 36, "warmupCycles": 28, "workspaceCapacity": 80, "openSizes": [4096, 512, 32], "settleAfterOpenSeconds": 3, "matchedSmallFileIdleSeconds": 15}
    assert len(report["cycles"]) == 36
    assert report["functionalPassed"] and report["sourceFilesUnchanged"] and report["desktopAliveAtEnd"]
    for index, cycle in enumerate(report["cycles"], 1):
        assert cycle["cycle"] == index and cycle["warmup"] == (index <= 28)
        assert [opened["rows"] for opened in cycle["opens"]] == [4096, 512, 32]
        assert all(opened["selectedIdMatches"] for opened in cycle["opens"])
    points = [cycle["settled"] for cycle in report["cycles"][28:]]
    identities = []
    for point in points:
        members = point["members"]
        assert point["treePrivateBytes"] == sum(member["privateBytes"] for member in members)
        app, = [member for member in members if member["role"] == "app"]
        assert point["rootPrivateBytes"] == app["privateBytes"] and point["rootHandles"] == app["handles"] and point["rootThreads"] == app["threads"]
        identity = sorted(member["ordinal"] for member in members)
        assert len(identity) == len(set(identity))
        identities.append(identity)
    stable = all(value == identities[0] for value in identities)
    assert stable == report["stableMeasuredProcessIdentities"]
    metrics = {}
    for field, (delta_budget, slope_budget) in budgets.items():
        values = [point[field] for point in points]
        assert all(isinstance(value, int) and value >= 0 for value in values)
        first, last = statistics.median(values[:3]), statistics.median(values[-3:])
        delta = last - first
        slope = statistics.median((values[j]-values[i])/(j-i) for i in range(8) for j in range(i+1, 8))
        got = report["memoryAssessment"]["metrics"][field]
        assert got["budget"] == {"delta": delta_budget, "perCycle": slope_budget}
        assert math.isclose(delta, got["delta"], abs_tol=1e-6) and math.isclose(slope, got["slopePerCycle"], abs_tol=1e-6)
        within = delta <= delta_budget and slope <= slope_budget
        assert within == got["withinBudget"]
        metrics[field] = {"firstMedian": first, "lastMedian": last, "delta": delta, "slopePerCycle": slope, "withinBudget": within}
    qualified = stable and all(metric["withinBudget"] for metric in metrics.values())
    assert qualified == report["passed"] == report["memoryAssessment"]["withinBudget"]
    for fixture in report["files"]:
        source = private / name / f"synthetic-{fixture['rows']}.httpsession"
        assert sha(source) == fixture["sha256"]
        document = read(source)
        assert len(document["transactions"]) == fixture["rows"] and document["selectedId"] == fixture["selectedId"]
    if fixture_sha is None:
        fixture_sha = [item["sha256"] for item in report["files"]]
    assert fixture_sha == [item["sha256"] for item in report["files"]]
    binary = private / ("archscope-baseline-amd64.exe" if name.startswith("baseline") else "archscope-fixed-amd64.exe")
    data = binary.read_bytes()
    assert hashlib.sha256(data).hexdigest() == report["binarySha256"]
    pe = struct.unpack_from("<I", data, 0x3c)[0]
    assert struct.unpack_from("<H", data, pe+4)[0] == 0x8664
    assert b"__archscopeWorkspaceRetention" not in data
    if name.startswith("fixed"):
        assert report["buildCommit"] == "d25581d4"
        if fixed_binary_sha is None:
            fixed_binary_sha = report["binarySha256"]
        assert fixed_binary_sha == report["binarySha256"]
    else:
        assert report["buildCommit"] == "30e5e078-baseline"
    (evidence / (name + ".json")).write_bytes(path.read_bytes())
    result["runs"].append({"name": name, "passed": qualified, "classification": report["memoryAssessment"]["classification"], "metrics": metrics, "measuredProcessCount": len(identities[0]), "stableIdentities": stable, "elapsedSeconds": report["elapsedSeconds"], "binarySha256": report["binarySha256"], "sourceReportSha256": sha(path)})
result["allPlannedRunsQualified"] = result["complete"] and all(run["passed"] for run in result["runs"])
result["historicalLimit"] = "Current baseline may pass; do not relabel old failures or attribute every historical late private-byte fluctuation to duplicate data."
result["hostLimit"] = "Shared interactive Windows host; other applications/user activity are not isolated. Agent UI bookend inspection is after measured cycles."
filename = "native-memory-validation.json" if result["complete"] else "native-memory-validation.partial.json"
(evidence / filename).write_text(json.dumps(result, indent=2)+"\n", encoding="utf-8")
print(json.dumps({"completed": len(result["runs"]), "allPlannedRunsQualified": result["allPlannedRunsQualified"], "file": filename}))
if result["complete"] and not result["allPlannedRunsQualified"]:
    raise SystemExit(1)
