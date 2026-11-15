#!/usr/bin/env python3
"""Check the correction and refusal of altered populations or acceptance state."""
import copy
import json
from pathlib import Path
import subprocess
import sys
import tempfile

HERE = Path(__file__).resolve().parent
SCRIPT = HERE / "update_inventory.py"

def run(file, optimized):
    return subprocess.run([sys.executable, *(['-O'] if optimized else []), str(SCRIPT), "--check", "--out", str(file)], capture_output=True)

results = []
original = json.loads((HERE / "remaining-acceptance.current.json").read_text())
with tempfile.TemporaryDirectory(prefix="archscope-wrev2-inventory-") as directory:
    for name in ["valid", "wrong-description", "wrong-count", "promoted-acceptance"]:
        doc = copy.deepcopy(original)
        jfr, = [r for r in doc["T-627"]["routes"] if r["route"] == "jfr"]
        if name == "wrong-description":
            jfr["verifiedThisFollowup"] = "153filtered events/29stacks"
        elif name == "wrong-count":
            jfr["verifiedCounts"]["filtered_events"] = 153
        elif name == "promoted-acceptance":
            doc["T-627"]["status"] = "DONE"
        file = Path(directory) / (name + ".json")
        file.write_text(json.dumps(doc))
        for optimized in [False, True]:
            process = run(file, optimized)
            expected_success = name == "valid"
            if (process.returncode == 0) != expected_success:
                raise RuntimeError(f"unexpected inventory check outcome: {name}, optimized={optimized}")
            results.append({"case": name, "optimized": optimized, "expectedExitSuccess": expected_success, "passed": True})
print(json.dumps({"passed": True, "cases": results}, indent=2))
