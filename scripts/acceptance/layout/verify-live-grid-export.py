"""Independently check the synthetic 680-request live-grid fixture's export.

Reads only an explicitly supplied private harness run. Public output contains
counts and hashes, never controller credentials, process keys or request IDs.
This verifies stored data; it does not generate a native UI verdict.
"""
import argparse
import hashlib
import json
from pathlib import Path
from urllib.parse import urlsplit


def read(path):
    return json.loads(path.read_text(encoding="utf-8-sig"))


def digest(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True,
                      separators=(",", ":")).encode("utf-8")


def verify(run):
    session = read(run / ".private-session.json")
    store = Path(session["storePath"]).resolve()
    assert store.is_relative_to((run / ".archscope-captures").resolve())
    files = sorted((store / "rolling").glob("*.json"))
    originals = {p.name: p.read_bytes() for p in files}
    source = [json.loads(raw)["transaction"] for raw in originals.values()]
    export_path = run / "live-after.httpsession"
    exported_bytes = export_path.read_bytes()
    doc = json.loads(exported_bytes)
    exported = doc["transactions"]
    assert len(source) == len(exported) == 640
    assert sorted(tx["sequence"] for tx in source) == list(range(41, 681))
    assert sorted(tx["sequence"] for tx in exported) == list(range(41, 681))
    source_by_id = {tx["id"]: tx for tx in source}
    export_by_id = {tx["id"]: tx for tx in exported}
    assert len(source_by_id) == len(export_by_id) == 640
    assert source_by_id.keys() == export_by_id.keys()
    changed_fields = {}
    for ident, original in source_by_id.items():
        actual = export_by_id[ident]
        for key in original.keys() | actual.keys():
            if original.get(key) != actual.get(key):
                changed_fields[key] = changed_fields.get(key, 0) + 1
        # Portable session Sanitize intentionally derives these fields from the
        # retained redacted URL. Assert that transformation, not false equality.
        normalized = dict(original)
        parsed = urlsplit(original["url"])
        normalized.update(host=parsed.netloc, query=parsed.query)
        assert {k: v for k, v in normalized.items() if k not in ("starred", "comment")} == {
            k: v for k, v in actual.items() if k not in ("starred", "comment")}
    assert set(changed_fields) == {"host", "query", "starred", "comment"}
    annotated = [tx for tx in exported if tx.get("starred") or tx.get("comment")]
    assert len(annotated) == 1
    note = annotated[0]
    assert note["sequence"] == 50 and note["starred"] is True
    assert note["comment"] == "라이브 주석 검증 / live-note-50"
    assert doc["selectedId"] == note["id"]
    annotations = read(store / "annotations.json")
    assert len(annotations) == 1
    assert annotations[note["id"]]["starred"] is True
    assert annotations[note["id"]]["comment"] == note["comment"]
    assert doc["retention"] == {"limitRows": 640, "persisted": 680,
                                "retained": 640, "evicted": 40}
    assert not list(store.glob(".annotations-*"))
    assert all((store / "rolling" / name).read_bytes() == raw
               for name, raw in originals.items())
    return {
        "schemaVersion": 1, "scope": "Independent stored/export data check; no UI verdict",
        "rows": 640, "sequenceRange": [41, 680], "allIdsUniqueAndEqual": True,
        "idSetSha256": digest(canonical(sorted(source_by_id))),
        "exportSha256": digest(exported_bytes), "exportBytes": len(exported_bytes),
        "sourceRollingSetSha256": digest(canonical({k: digest(v) for k, v in originals.items()})),
        "annotationFileSha256": digest((store / "annotations.json").read_bytes()),
        "selectedSequence": 50, "exactStarAndUnicodeComment": True,
        "changedFields": changed_fields,
        "normalization": "host includes port and query derives from retained redacted URL; only star/comment additionally change",
        "allOtherFieldsEqual": True, "sourceUnchangedDuringReadback": True,
        "stagingFiles": 0, "retention": doc["retention"], "passed": True,
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-dir", type=Path, required=True)
    parser.add_argument("--report", type=Path, required=True)
    args = parser.parse_args()
    result = verify(args.run_dir.resolve())
    args.report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print("PASS: 640 retained IDs, selected annotation, URL normalization and all other fields")
