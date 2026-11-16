#!/usr/bin/env python3
"""Check desktop message/help/legacy-label coverage without running a WebView."""
import ast
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1] / "apps/engine-native/cmd/archscope-app/frontend/src"
STRING = r'''(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*')'''


def decode(value):
    return json.loads(value) if value.startswith('"') else ast.literal_eval(value)


def required_sources():
    sources = set()
    source = (ROOT / "i18n/messages.ts").read_text()
    en = source[source.index("  en: {"):source.index("  ko: {")]
    sources.update(decode(value) for _, value in re.findall(r"^    (\w+):\s*(" + STRING + r"),", en, re.M))
    for path in sorted(ROOT.rglob("*.tsx")) + sorted(ROOT.rglob("*.ts")):
        if path.name in {"messages.ts", "ja.ts", "zh-CN.ts", "zh-TW.ts"}:
            continue
        source = path.read_text()
        for match in re.finditer(r"\b(?:w|words|text)\(\s*(" + STRING + r")\s*,\s*" + STRING, source):
            sources.add(decode(match[1]))
        for match in re.finditer(r"\btranslateText\(\s*\w+\s*,\s*(" + STRING + r")\s*,", source):
            sources.add(decode(match[1]))
        if path.name == "helpCatalog.ts":
            sources.update(decode(value) for _, value in re.findall(r"^  (\w+):\s*(" + STRING + r"),", source[:source.index("export type HelpKey")], re.M))
        if path.name == "liveHttpCapture.ts":
            sources.update(decode(value) for value in re.findall(r"^    en:\s*(" + STRING + r"),", source, re.M))
        if path.name == "desktopLabels.ts":
            sources.update(json.loads(source[source.index("const labels = ") + 15:source.index(";\n\nexport function")]))
    return sources


def main():
    required = required_sources()
    for locale in ("ja", "zh-CN", "zh-TW"):
        source = (ROOT / "i18n" / (locale + ".ts")).read_text()
        entries = json.loads(source[source.index("{"):source.rindex("}") + 1], object_pairs_hook=list)
        catalog = dict(entries)
        if len(catalog) != len(entries):
            raise SystemExit(f"{locale}: duplicate translation source")
        missing = sorted(required - catalog.keys())
        if missing:
            raise SystemExit(f"{locale}: missing {len(missing)} messages: {missing[:8]}")
        for en, translation in catalog.items():
            if not isinstance(translation, str) or not translation.strip():
                raise SystemExit(f"{locale}: empty translation: {en}")
            placeholders = lambda text: sorted(re.findall(r"\{[A-Za-z_]\w*\}", text))
            if placeholders(en) != placeholders(translation):
                raise SystemExit(f"{locale}: changed placeholders: {en}")
        print(f"{locale}: {len(required)} required phrases covered; {len(catalog)} entries; placeholders preserved")


if __name__ == "__main__":
    main()
