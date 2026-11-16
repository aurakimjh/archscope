"""Independent original-budget recomputation for the ordinary Windows x86 run."""
import hashlib
import json
import math
import statistics
import struct
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
PUBLIC = Path(__file__).resolve().parent
PRIVATE = ROOT / ".tmp/windows-validation-20261116"
run = PRIVATE / "memory-386"
source = run / "result.json"
report = json.loads(source.read_text(encoding="utf-8"))
protocol = report["protocol"]
assert protocol == {"cycles":36,"warmupCycles":28,"workspaceCapacity":80,"openSizes":[4096,512,32],"settleAfterOpenSeconds":3,"matchedSmallFileIdleSeconds":15}
assert len(report["cycles"]) == 36 and report["sourceFilesUnchanged"] and report["functionalPassed"] and report["desktopAliveAtEnd"]
assert report["buildCommit"] == "10228156"
assert hashlib.sha256((ROOT / ".tmp/windows-final-20261114/import-amd64/synthetic-32.httpsession").read_bytes()).hexdigest() == report["sourceFixtureSha256"]
for number, cycle in enumerate(report["cycles"],1):
    assert cycle["cycle"] == number and cycle["warmup"] == (number <= 28)
    assert [opened["rows"] for opened in cycle["opens"]] == [4096,512,32]
    assert all(opened["selectedIdMatches"] for opened in cycle["opens"])
for fixture in report["files"]:
    data = (run / f"synthetic-{fixture['rows']}.httpsession").read_bytes()
    assert hashlib.sha256(data).hexdigest() == fixture["sha256"]
    parsed = json.loads(data)
    assert len(parsed["transactions"]) == fixture["rows"] and parsed["selectedId"] == fixture["selectedId"]
binary = (PRIVATE / "archscope-production-386.exe").read_bytes()
assert hashlib.sha256(binary).hexdigest() == report["binarySha256"]
assert b"__archscopeWorkspaceRetention" not in binary
assert struct.unpack_from("<H",binary,struct.unpack_from("<I",binary,0x3c)[0]+4)[0] == 0x14c
points = [cycle["settled"] for cycle in report["cycles"][28:]]
identities=[]
for point in points:
    members=point["members"]
    assert point["treePrivateBytes"] == sum(member["privateBytes"] for member in members)
    app, = [member for member in members if member["role"] == "app"]
    assert [point["rootPrivateBytes"],point["rootHandles"],point["rootThreads"]] == [app["privateBytes"],app["handles"],app["threads"]]
    identity=sorted(member["ordinal"] for member in members)
    assert len(identity) == len(set(identity))
    identities.append(identity)
stable=all(identity==identities[0] for identity in identities)
assert stable == report["stableMeasuredProcessIdentities"]
budgets={"treePrivateBytes":(64*2**20,4*2**20),"rootPrivateBytes":(16*2**20,2**20),"rootHandles":(24,2),"rootThreads":(4,0.5)}
metrics={}
for key,(delta_budget,slope_budget) in budgets.items():
    values=[point[key] for point in points]
    assert all(isinstance(value,int) and value>=0 for value in values)
    first,last=statistics.median(values[:3]),statistics.median(values[-3:])
    delta=last-first
    slope=statistics.median((values[j]-values[i])/(j-i) for i in range(8) for j in range(i+1,8))
    original=report["memoryAssessment"]["metrics"][key]
    assert original["budget"] == {"delta":delta_budget,"perCycle":slope_budget}
    assert math.isclose(delta,original["delta"],abs_tol=1e-6) and math.isclose(slope,original["slopePerCycle"],abs_tol=1e-6)
    passed=delta<=delta_budget and slope<=slope_budget
    assert passed == original["withinBudget"]
    metrics[key]={"firstMedian":first,"lastMedian":last,"delta":delta,"slopePerCycle":slope,"withinBudget":passed}
passed=stable and all(metric["withinBudget"] for metric in metrics.values())
assert passed == report["passed"] == report["memoryAssessment"]["withinBudget"]
(PUBLIC / "memory-386.json").write_bytes(source.read_bytes())
validation={"passed":passed,"scope":"One finite ordinary Windows x86 body-omitted repeated-import screen; no forcedGC/trim, unique/body-rich/longer native closure or reinterpretation of historical failures","hostLimit":"Shared interactive host; owned domain tests and containers completed before native memory launch; agent UI bookends only after measurement","metrics":metrics,"stableMeasuredIdentities":stable,"measuredProcessCount":len(identities[0]),"binarySha256":report["binarySha256"],"sourceReportSha256":hashlib.sha256(source.read_bytes()).hexdigest(),"elapsedSeconds":report["elapsedSeconds"]}
(PUBLIC / "memory-validation.json").write_text(json.dumps(validation,indent=2)+"\n",encoding="utf-8")
print(json.dumps({"passed":passed,"cycles":36,"metrics":metrics}))
if not passed:
    raise SystemExit(1)
