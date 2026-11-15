"""Portable checker around the immutable WREV2 source-derived evidence generator."""
import argparse
import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import sys

# Historical generator snapshots use locale-default read_text. Select UTF-8 for
# this process only, preserving the received source and every raw receipt byte.
if not sys.flags.utf8_mode:
    optimization = ["-" + "O" * sys.flags.optimize] if sys.flags.optimize else []
    raise SystemExit(subprocess.call([sys.executable, *optimization, "-X", "utf8", __file__, *sys.argv[1:]]))

root = Path(__file__).resolve().parents[2]
source = root / "docs/review/evidence/2026-11-15-windows-remediation/update_inventory.py"
spec = importlib.util.spec_from_file_location("received_inventory_generator", source)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
expected = module.corrected_inventory()  # validates original hashes/populations
correction = expected["correction"]
# These fields are repository paths, independent of the host filesystem syntax.
correction["originalInventory"]["path"] = correction["originalInventory"]["path"].replace("\\", "/")
correction["sourceSha256"] = {path.replace("\\", "/"): digest for path, digest in correction["sourceSha256"].items()}
correction["relatedIncrementalInventory"]["path"] = correction["relatedIncrementalInventory"]["path"].replace("\\", "/")

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--path", type=Path, default=source.with_name("remaining-acceptance.current.json"))
parser.add_argument("--self-test", action="store_true")
args = parser.parse_args()


def check(document):
    if document != expected:
        raise ValueError("current inventory differs from source-derived populations, paths or acceptance state")


check(json.loads(args.path.read_text(encoding="utf-8")))
results = []
if args.self_test:
    for name in ("valid", "wrong-description", "wrong-count", "promoted-acceptance"):
        candidate = copy.deepcopy(expected)
        jfr, = [row for row in candidate["T-627"]["routes"] if row["route"] == "jfr"]
        if name == "wrong-description":
            jfr["verifiedThisFollowup"] = "153filtered events/29stacks"
        elif name == "wrong-count":
            jfr["verifiedCounts"]["filtered_events"] = 153
        elif name == "promoted-acceptance":
            candidate["T-627"]["status"] = "DONE"
        accepted = True
        try:
            check(candidate)
        except ValueError:
            accepted = False
        if accepted != (name == "valid"):
            raise RuntimeError("unexpected acceptance: " + name)
        results.append({"case": name, "passed": True})
print(json.dumps({"passed": True, "optimized": sys.flags.optimize, "utf8": True, "cases": results}, indent=2))
