#!/usr/bin/env python3
"""Check public RC version and platform metadata without modifying the checkout."""
import argparse
import json
from pathlib import Path
import plistlib
import re

ROOT = Path(__file__).resolve().parents[1]
APP = ROOT / "apps/engine-native/cmd/archscope-app"

def require(condition, message):
    if not condition:
        raise SystemExit("RELEASE_VERSION: " + message)

def check(expected=None, bundle=None):
    release = json.loads((APP / "build/release.json").read_text())
    version, tag = release["version"], release["tag"]
    require(re.fullmatch(r"(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-(?:rc|beta|alpha)\.[1-9]\d*)?", version) is not None, "invalid public version")
    require(tag == "v" + version and (expected is None or expected == tag), "tag/public version mismatch")
    numeric = release["windowsVersion"].split(".")
    require(len(numeric) == 4 and all(v.isdigit() and 0 <= int(v) <= 65535 for v in numeric), "invalid Windows numeric version")
    require(".".join(numeric[:3]) == version.split("-")[0] == release["macosShortVersion"], "platform core version mismatch")
    require(release["macosBuildVersion"].isdigit() and int(release["macosBuildVersion"]) > 0, "invalid macOS build")
    package = json.loads((APP / "frontend/package.json").read_text())
    lock = json.loads((APP / "frontend/package-lock.json").read_text())
    require(package["version"] == lock["version"] == lock["packages"][""]["version"] == version, "frontend/lock version mismatch")
    require(re.search(r'^  version: "' + re.escape(version) + r'"$', (APP / "build/config.yml").read_text(), re.M), "Wails info version mismatch")
    for filename in ["Info.plist", "Info.dev.plist"]:
        with (APP / "build/darwin" / filename).open("rb") as f:
            info = plistlib.load(f)
        require(info["CFBundleShortVersionString"] == release["macosShortVersion"], filename + " short version")
        require(info["CFBundleVersion"] == release["macosBuildVersion"], filename + " build version")
    if bundle is not None:
        with (bundle / "Contents/Info.plist").open("rb") as f:
            info = plistlib.load(f)
        require(info["CFBundleShortVersionString"] == release["macosShortVersion"], "built bundle short version")
        require(info["CFBundleVersion"] == release["macosBuildVersion"], "built bundle build version")
    info = json.loads((APP / "build/windows/info.json").read_text())
    require(all(info["fixed"][field] == release["windowsVersion"] for field in ["file_version", "product_version"]), "Windows numeric version")
    require(all(info["info"]["0409"][field] == version for field in ["FileVersion", "ProductVersion"]), "Windows display version")
    nsis = (APP / "build/windows/nsis/project.nsi").read_text()
    require('!define INFO_PRODUCTVERSION "' + version + '"' in nsis, "NSIS display version")
    require('!define INFO_NUMERICVERSION "' + release["windowsVersion"] + '"' in nsis, "NSIS numeric version")
    require('VIProductVersion "${INFO_NUMERICVERSION}"' in nsis and 'VIFileVersion    "${INFO_NUMERICVERSION}"' in nsis, "NSIS resource version source")
    require('!define INFO_PRODUCTVERSION "' + version + '"' in (APP / "build/windows/nsis/wails_tools.nsh").read_text(), "NSIS fallback version")
    require('version: "' + version + '"' in (APP / "build/linux/nfpm/nfpm.yaml").read_text(), "Linux package version")
    require('const AppVersion = "' + version + '"' in (ROOT / "apps/engine-native/internal/packs/model.go").read_text(), "pack application version")
    notices = json.loads((ROOT / "apps/engine-native/internal/distribution/assets/licenses/manifest.json").read_text())
    require(any(v["ecosystem"] == "project" and v["name"] == "archscope" and v["version"] == version for v in notices["dependencies"]), "bundled project notice version")
    for lang in ["en", "ko"]:
        require((ROOT / "docs" / lang / "releases" / (tag + ".md")).is_file(), lang + " release scope missing")
    require("## [" + version + "]" in (ROOT / "CHANGELOG.md").read_text(), "CHANGELOG release missing")
    print("Release metadata verified: " + tag)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--expected-tag")
    parser.add_argument("--bundle", type=Path)
    args = parser.parse_args()
    check(args.expected_tag, args.bundle)
