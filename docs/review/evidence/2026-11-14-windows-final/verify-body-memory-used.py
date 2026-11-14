"""Independent Python check of the predeclared native body-memory experiment."""
import hashlib
import json
import statistics
import sys
from pathlib import Path
from urllib.parse import urlparse, parse_qs

private = Path(sys.argv[1])
out = Path(__file__).parent
report_bytes = (private / "result.json").read_bytes()
r = json.loads(report_bytes)
protocol = json.loads((out / "body-memory-protocol.json").read_text(encoding="utf-8-sig"))
assert r["passed"] and r["functionalPassed"] and r["desktopAliveAtEnd"]
assert r["requests"] == 5120 and len(r["cycles"]) == 10
assert r["bodyGate"] == {"available": True, "gate": "sec10_verified", "maxBodyBytes": 262144}
assert r["protocol"]["captureBodies"] is True
assert r["binarySha256"] == "5ecff72fb9ce72bc9b07c97addc79de77563e5bd4d122a2c4969ad1001b37289"
points = []
for i, cycle in enumerate(r["cycles"], 1):
    assert cycle["cycle"] == i and cycle["warmup"] == (i <= 2)
    assert cycle["requests"] == 512
    assert cycle["clear"]["empty"].get("retained", 0) == 0
    assert cycle["clear"]["refilled"]["retained"] == 4
    s = cycle["stats"]
    assert s["captured"] == s["persisted"] == 512
    assert s["retained"] == 128 and s["evicted"] == 384 and s["generation"] == i
    assert all(s.get(k, 0) == 0 for k in ("parseFailed", "dropped", "bodyOmitted", "eventSkipped", "kernelDropped", "unattributed"))
    if i > 2:
        points.append(cycle["settled"])
metrics = {}
for key, expected in r["memoryAssessment"]["metrics"].items():
    values = [p[key] for p in points]
    delta = statistics.median(values[-3:]) - statistics.median(values[:3])
    slope = statistics.median([(values[j] - values[i]) / (j-i) for i in range(8) for j in range(i+1, 8)])
    assert delta == expected["delta"] and slope == expected["slopePerCycle"]
    assert delta <= expected["budget"]["delta"] and slope <= expected["budget"]["perCycle"]
    metrics[key] = {"values": values, "delta": delta, "slopePerCycle": slope}
session_bytes = (private / "final.httpsession").read_bytes()
assert hashlib.sha256(session_bytes).hexdigest() == r["savedSessionSha256"]
saved = json.loads(session_bytes)
assert saved["type"] == "archscope-http-session" and saved["scope"] == "all"
assert len(saved["transactions"]) == 128
assert saved["selectedId"] in {t["id"] for t in saved["transactions"]}
sequences = []
body_hashes = set()
for tx in saved["transactions"]:
    assert tx["state"] == "complete" and tx["statusCode"] == 200 and tx["generation"] == 10
    u = urlparse(tx["url"])
    assert u.netloc == "127.0.0.1:11514" and u.path == "/large"
    sequences.append(int(parse_qs(u.query)["sequence"][0]))
    response = tx["response"]
    assert response["bodySize"] == 300020 and response["bodyStorage"] == "truncated"
    body = response["bodyPreview"].encode("utf-8")
    assert body == b"capture-body-prefix:" + b"x" * (262144-20)
    body_hashes.add(hashlib.sha256(body).hexdigest())
assert sorted(sequences) == list(range(4992, 5120)), "producer uses zero-based sequence"
assert len(body_hashes) == 1
(out / "body-memory-386-result.json").write_bytes(report_bytes)
receipt = {"passed": True, "classification": "BOUNDED_IN_THIS_RUN", "elapsedSeconds": r["elapsedSeconds"], "binarySha256": r["binarySha256"], "buildCommit": r["buildCommit"], "independentMetrics": metrics, "savedRows": 128, "allBodyPrefixesExact": True, "retainedBodyBytesEach": 262144, "wireBodyBytesEach": 300020, "savedScope": "all", "selectedIdBelongsToSavedRows": True, "bodyPrefixSha256": next(iter(body_hashes)), "savedSessionSha256": r["savedSessionSha256"], "note": "Controller raw session artifact is not relabeled as the UI redacted-export/policy test. Finite scoped memory screening; other tests share the host; not universal leak freedom."}
(out / "body-memory-independent.json").write_text(json.dumps(receipt, indent=2) + "\n", encoding="utf-8")
print("PASS: independent Clear/counter/8-idle-median/Theil-Sen/all-128-body-prefix readback")
