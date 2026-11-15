#!/usr/bin/env python3
"""Qualify stable pack bounds with source overlays and isolated synthetic stores."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
ENGINE = ROOT / "apps/engine-native"
MODEL = ENGINE / "internal/packs/model.go"
BOOTSTRAP = ENGINE / "internal/packs/assets/bootstrap.zip"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--docker", action="store_true", help="run cross-compiled Linux/arm64 tests in isolated node:22-alpine")
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=False)
    source = MODEL.read_text(encoding="utf-8")
    current = json.loads((ENGINE / "cmd/archscope-app/build/release.json").read_text(encoding="utf-8"))["version"]
    needle = f'const AppVersion = "{current}"'
    if source.count(needle) != 1:
        raise SystemExit("canonical app version does not match pack source")
    original_hash = hashlib.sha256(BOOTSTRAP.read_bytes()).hexdigest()
    with zipfile.ZipFile(BOOTSTRAP) as archive:
        manifest = json.loads(archive.read("manifest.json"))
    manifest["version"] = "1.0.1"
    manifest["app"]["before"] = "0.4.1"
    (args.out / "manifest-candidate.UNSIGNED.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    reports = []
    with tempfile.TemporaryDirectory(prefix="archscope-stable-pack-") as temporary:
        private = Path(temporary)
        for version, phase in [(current, "seed"), ("0.4.0", "stable"), ("0.4.1", "future")]:
            replacement = private / "model.go"
            replacement.write_text(source.replace(needle, f'const AppVersion = "{version}"'), encoding="utf-8")
            overlay = private / "overlay.json"
            overlay.write_text(json.dumps({"Replace": {str(MODEL): str(replacement)}}), encoding="utf-8")
            env = dict(os.environ, ARCHSCOPE_PACK_EXPECTED_VERSION=version,
                       ARCHSCOPE_PACK_PREPARATION_ROOT=str(private),
                       ARCHSCOPE_PACK_PREPARATION_PHASE=phase)
            command = ["go", "test", "-tags", "releasequalification", "-overlay", str(overlay), "./internal/packs",
                       "-run", "^TestStablePackPreparation$", "-count=1", "-v"]
            if args.docker:
                binary = private / ("probe-" + version)
                subprocess.run(["go", "test", "-tags", "releasequalification", "-overlay", str(overlay), "-c", "-o", str(binary), "./internal/packs"],
                               cwd=ENGINE, env=dict(os.environ, GOOS="linux", GOARCH="arm64", CGO_ENABLED="0"), check=True)
                command = ["docker", "run", "--rm", "--network", "none", "--read-only", "--memory", "1g", "--cpus", "2",
                           "--tmpfs", "/tmp:rw,size=128m", "-v", str(private) + ":/qualification",
                           "-e", "ARCHSCOPE_PACK_PREPARATION_ROOT=/qualification",
                           "-e", "ARCHSCOPE_PACK_PREPARATION_PHASE=" + phase,
                           "-e", "ARCHSCOPE_PACK_EXPECTED_VERSION=" + version,
                           "node:22-alpine", "/qualification/" + binary.name,
                           "-test.run=^TestStablePackPreparation$", "-test.v"]
            result = subprocess.run(command, cwd=ENGINE, env=env, capture_output=True)
            (args.out / (version + ".log")).write_bytes(result.stdout + result.stderr)
            if result.returncode:
                raise SystemExit(f"pack qualification failed for {version}; see owned output log")
            reports.append(json.loads((private / ("result-" + version + ".json")).read_text(encoding="utf-8")))
    if MODEL.read_text(encoding="utf-8") != source or hashlib.sha256(BOOTSTRAP.read_bytes()).hexdigest() != original_hash:
        raise SystemExit("production inputs changed during qualification")
    report = {"passed": True, "productionVersionUnchanged": current,
              "execution": "Docker Linux/arm64 network-none" if args.docker else "native host",
              "originalBootstrapSha256": original_hash, "productionTrustUnchanged": True,
              "officialSignedPack": "PENDING existing authorized signer and release migration",
              "candidateRangeBefore": "0.4.1", "matrix": reports}
    (args.out / "result.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report))


if __name__ == "__main__":
    main()
