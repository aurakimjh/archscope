"""Independent readback and direct-origin wire comparison; run at repo root."""
from pathlib import Path
import hashlib
import json
import urllib.parse
import urllib.request

root = Path.cwd()
private = root / ".tmp/windows-stream-20261115"
evidence = root / "docs/review/evidence/2026-11-15-windows-stream-boundaries"
read = lambda p: json.loads(p.read_text(encoding="utf-8-sig"))
sha = lambda p: hashlib.sha256(p.read_bytes()).hexdigest()
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
base = "http://127.0.0.1:11516"
with opener.open(base + "/stats", timeout=10) as response:
    events = json.load(response)
result = {"passed": True, "execution": "Actual Windows backend sends; native saved-file observations separately pinned", "architectures": {}}
assert (evidence / "windows-helper-used.go").read_bytes() == (root / "apps/engine-native/.tmp/windows-stream-20261115/main.go").read_bytes()
for arch in ("386", "amd64"):
    session_path = private / f"stream-{arch}.httpsession"
    document = read(session_path)
    receipt = read(private / f"stream-{arch}-receipt.json")
    rows = document["transactions"]
    assert receipt["passed"] and receipt["count"] == len(rows) == 13
    assert [row["sequence"] for row in rows] == list(range(1, 14))
    assert receipt["summary"]["compressionKnown"] == 6 and receipt["summary"]["compressionSaved"] == -20
    at = lambda endpoint: next(row for row in rows if urllib.parse.urlsplit(row["url"]).path == endpoint)
    assert at("/identity")["response"]["bodyPreview"] == '{"marker":"identity-proof"}'
    short = at("/identity-short")
    assert short["state"] == "failed" and short["response"]["bodySize"] == 5
    assert any(h["name"].lower() == "content-length" and h["value"] == "20" for h in short["response"]["headers"])
    stacked = at("/stacked")["response"]
    assert stacked["bodyStorage"] == "encoded" and stacked["bodyDecoded"] == -1
    assert stacked["bodyEncoding"] == "gzip, br" and not stacked.get("bodyPreview")
    for endpoint in ("/binary", "/binary-gzip"):
        message = at(endpoint)["response"]
        assert message["bodyStorage"] == "truncated" and message["bodyDecoded"] == 1048576
        assert not message.get("bodyPreview") and not message.get("imagePreview")
    assert at("/binary-exact")["response"]["bodyStorage"] == "binary"
    assert at("/binary-exact")["response"]["bodySize"] == 1048576
    assert at("/stream")["response"]["bodyPreview"] == "first\nsecond\nthird\n"
    assert at("/stream")["response"]["bodySize"] == 19
    assert at("/head")["method"] == "HEAD" and at("/head")["response"]["bodySize"] == 0
    scoped_events = []
    for endpoint in ("/hold", "/hold-gzip"):
        tx = at(endpoint)
        assert tx["state"] == "failed" and "context canceled" in tx["error"]
        parsed = urllib.parse.urlsplit(tx["url"])
        target = parsed.path + "?" + parsed.query
        found = [event for event in events if event["path"] == target]
        assert sum(event["stage"] == "prefix" for event in found) == 1
        assert any(event["stage"] == "client-closed" for event in found)
        request_id = urllib.parse.parse_qs(parsed.query)["id"][0]
        assert not any("duplicate=" + request_id in event["path"] for event in events)
        scoped_events.extend(found)
    assert at("/hold")["response"]["bodyPreview"] == "prefix\n"
    canceled = at("/hold-gzip")["response"]
    assert canceled["bodyDecoded"] == -1 and canceled["bodyStorage"] == "encoded" and not canceled.get("bodyPreview")
    timeout = at("/timeout")
    assert timeout["state"] == "failed" and "deadline exceeded" in timeout["error"]
    assert timeout["response"]["bodyPreview"].startswith("prefix\n")
    assert timeout["response"]["bodySize"] == len(timeout["response"]["bodyPreview"].encode())
    assert any("after-cancel=1" in row["url"] and row["state"] == "complete" for row in rows)
    result["architectures"][arch] = {"sessionSha256": sha(session_path), "count": 13, "summary": {key: receipt["summary"][key] for key in ("total", "completed", "pending", "compressionKnown", "compressionSaved")}, "rows": [{"path": urllib.parse.urlsplit(row["url"]).path, "state": row["state"], "bodySize": row["response"]["bodySize"], "bodyDecoded": row["response"]["bodyDecoded"], "storage": row["response"]["bodyStorage"]} for row in rows], "independentServerCancellationEvents": scoped_events}
live = read(private / "live-notice-final/result.json")
assert live["passed"] and live["positiveBodyVerified"] and live["gate"]["state"] == "sec10_verified"
wire = {}
for endpoint in ("identity", "x-gzip", "stacked", "binary", "binary-gzip"):
    with opener.open(base + "/" + endpoint, timeout=10) as response:
        data = response.read()
    proof = {"bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()}
    assert proof["bytes"] == live["clientWire"][endpoint]["bytes"]
    assert proof["sha256"] == live["clientWire"][endpoint]["sha256"]
    wire[endpoint] = proof
assert live["summary"]["compressionKnown"] == 2 and live["summary"]["compressionSaved"] == -20
live_file = private / "live-notice-final/live-binary.httpsession"
assert len(read(live_file)["transactions"]) == 5
result["actualProxy"] = {"build": live["build"], "gate": live["gate"], "sessionSha256": sha(live_file), "directOriginMatchesAllFiveClientWireBodies": wire}
latest = read(private / "live-keyboard-final/result.json")
assert latest["passed"] and latest["positiveBodyVerified"] and latest["gate"]["state"] == "sec10_verified"
for endpoint, proof in wire.items():
    assert latest["clientWire"][endpoint]["bytes"] == proof["bytes"]
    assert latest["clientWire"][endpoint]["sha256"] == proof["sha256"]
result["latestNativeBuildActualProxy"] = {"build": latest["build"], "gate": latest["gate"], "sessionSha256": sha(private / "live-keyboard-final/live-binary.httpsession"), "allFiveWireHashesMatch": True}
gate64 = read(private / "live-amd64/result.json")
assert gate64["gated"] and not gate64["positiveBodyVerified"]
result["amd64BodyCapability"] = {"gated": True, "positiveBodyVerified": False, "scope": "Current runtime capability check only; no gate bypass or positive body assertion"}
assert all(event["acceptEncoding"] == "" for event in events if event["method"] == "HEAD")
result["headDefaultNegotiationAbsent"] = True
(evidence / "windows-readback.json").write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
(evidence / "live-proxy-final.json").write_text(json.dumps(live, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print("PASS: Windows x64/x86 13 cases each; actual SEC10 proxy5; direct-origin wire hashes and cancellation events")
