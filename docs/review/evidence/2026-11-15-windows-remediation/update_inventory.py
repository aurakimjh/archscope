#!/usr/bin/env python3
"""Build/check a corrected current inventory from immutable public receipts."""
import argparse
import copy
import hashlib
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[4]
ORIGINAL = ROOT / "docs/review/evidence/2026-11-14-windows-final"
OUTPUT = Path(__file__).with_name("remaining-acceptance.current.json")

def require(condition, message):
    if not condition:
        raise ValueError(message)

def corrected_inventory():
    manifest = json.loads((ORIGINAL / "artifact-sha256.json").read_text())
    records = {entry["file"]: entry for entry in manifest["files"]}
    names = ["remaining-acceptance.json", "jfr-notable-dense-fixed.txt", "jfr-binary-counts.json"]
    inputs, hashes = {}, {}
    for name in names:
        data = (ORIGINAL / name).read_bytes()
        digest = hashlib.sha256(data).hexdigest()
        require(digest == records[name]["sha256"] and len(data) == records[name]["bytes"], "original receipt integrity mismatch: " + name)
        inputs[name] = data.decode("utf-8")
        hashes[name] = digest
    native = inputs["jfr-notable-dense-fixed.txt"]
    def number(label):
        found = re.search(r"(?:^|\n)" + re.escape(label) + r"\r?\n도움말 보기\r?\n([0-9,]+)(?:\r?\n|$)", native)
        require(found is not None, "native counter not found: " + label)
        return int(found[1].replace(",", ""))
    counts = {
        "total_events": number("총 이벤트"), "filtered_events": number("필터된 이벤트"),
        "sample_events": number("샘플 이벤트"), "stack_samples": number("스택 샘플"),
        "unique_stacks": number("고유 스택"),
    }
    jdk = json.loads(inputs["jfr-binary-counts.json"])
    require(counts["total_events"] == jdk["totalEvents"] == sum(jdk["eventTypes"].values()), "native/JDK event total mismatch")
    require(counts["filtered_events"] <= counts["total_events"] and counts["stack_samples"] <= counts["sample_events"], "counter population mismatch")
    original = json.loads(inputs["remaining-acceptance.json"])
    result = copy.deepcopy(original)
    routes = [r for r in result["T-627"]["routes"] if r["route"] == "jfr"]
    require(len(routes) == 1, "exactly one JFR inventory row required")
    row = routes[0]
    old_phrase = "153filtered events/29stacks"
    require(old_phrase in row["verifiedThisFollowup"], "expected historical wording not found")
    row["verifiedThisFollowup"] = row["verifiedThisFollowup"].replace(
        old_phrase,
        f'{counts["total_events"]}total/{counts["filtered_events"]}filtered events;'
        f'{counts["sample_events"]}sample events/{counts["stack_samples"]}stack samples/'
        f'{counts["unique_stacks"]}unique stacks',
    )
    row["verifiedCounts"] = counts
    result["correction"] = {
        "finding": "WREV2-02",
        "originalInventory": {"path": str((ORIGINAL / names[0]).relative_to(ROOT)), "sha256": hashes[names[0]]},
        "sourceSha256": {str((ORIGINAL / name).relative_to(ROOT)): hashes[name] for name in names[1:]},
        "scope": "Counter wording only; original evidence and hashes unchanged. No new native run or acceptance status change.",
    }
    # The newer receipt is incremental, not a replacement for the 21-route list.
    later = ROOT / "docs/review/evidence/2026-11-15-windows-conditional/remaining-acceptance.json"
    data = later.read_bytes()
    manifest = json.loads(later.with_name("artifact-sha256.json").read_text())
    entries = manifest["files"]
    entry = next(e for e in entries if e["file"] == later.name) if isinstance(entries, list) else entries[later.name]
    require(hashlib.sha256(data).hexdigest() == entry["sha256"] and len(data) == entry["bytes"], "incremental inventory integrity mismatch")
    incremental = json.loads(data)
    require(incremental["T-613"]["status"] == result["T-613"]["status"] and incremental["T-627"]["status"] == result["T-627"]["status"], "incremental acceptance status mismatch")
    result["correction"]["relatedIncrementalInventory"] = {
        "path": str(later.relative_to(ROOT)),
        "sha256": entry["sha256"],
        "scope": "Read alongside this corrected route inventory; additional Windows conditional follow-up remains separately scoped.",
    }
    return result

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=OUTPUT)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    expected = corrected_inventory()
    if args.check:
        require(json.loads(args.out.read_text()) == expected, "current inventory differs from the source-derived correction")
        print("PASS: current JFR populations and unchanged acceptance scope match original receipts")
    else:
        args.out.write_text(json.dumps(expected, ensure_ascii=False, indent=2) + "\n")
        print("Wrote current inventory; original receipt bytes preserved")

if __name__ == "__main__":
    main()
