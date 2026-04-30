from __future__ import annotations

import re
from dataclasses import dataclass, field
from urllib.parse import parse_qsl, quote, unquote, urlsplit, urlunsplit

REDACTION_VERSION = "0.1.0"


@dataclass(frozen=True)
class RedactionResult:
    text: str
    summary: dict[str, int] = field(default_factory=dict)


def redact_text(value: str | None) -> RedactionResult:
    if value is None:
        return RedactionResult(text="")

    summary: dict[str, int] = {}
    text = value
    text = _redact_authorization(text, summary)
    text = _redact_cookies(text, summary)
    text = _redact_urls(text, summary)
    text = _redact_email(text, summary)
    text = _redact_absolute_paths(text, summary)
    text = _redact_ipv4(text, summary)
    text = _redact_path_numbers(text, summary)
    text = _redact_long_numbers(text, summary)
    return RedactionResult(text=text, summary=summary)


def merge_redaction_summaries(*summaries: dict[str, int]) -> dict[str, int]:
    merged: dict[str, int] = {}
    for summary in summaries:
        for key, count in summary.items():
            merged[key] = merged.get(key, 0) + count
    return merged


def _count(summary: dict[str, int], key: str) -> None:
    summary[key] = summary.get(key, 0) + 1


def _placeholder(kind: str, value: str) -> str:
    return f"<{kind} len={len(value)}>"


def _redact_authorization(text: str, summary: dict[str, int]) -> str:
    def replace(match: re.Match[str]) -> str:
        _count(summary, "TOKEN")
        scheme = match.group("scheme")
        token = match.group("token")
        return f"{match.group('prefix')}{scheme} {_placeholder('TOKEN', token)}"

    return re.sub(
        r"(?P<prefix>\bAuthorization:\s*)(?P<scheme>Bearer|Basic)\s+(?P<token>[^\s\"']+)",
        replace,
        text,
        flags=re.IGNORECASE,
    )


def _redact_cookies(text: str, summary: dict[str, int]) -> str:
    def replace(match: re.Match[str]) -> str:
        _count(summary, "COOKIE")
        return f"{match.group('prefix')}<COOKIE len={len(match.group('value'))}>"

    return re.sub(
        r"(?P<prefix>\b(?:Cookie|Set-Cookie):\s*)(?P<value>[^\"'\n]+)",
        replace,
        text,
        flags=re.IGNORECASE,
    )


def _redact_urls(text: str, summary: dict[str, int]) -> str:
    return re.sub(
        r"(?P<url>(?:https?://[^\s\"']+)|(?:/[^\s\"']*\?[^\s\"']+))",
        lambda match: _redact_url(match.group("url"), summary),
        text,
    )


def _redact_url(url: str, summary: dict[str, int]) -> str:
    split = urlsplit(url)
    if not split.query:
        return url

    redacted_query: list[str] = []
    for key, raw_value in parse_qsl(split.query, keep_blank_values=True):
        value = unquote(raw_value)
        kind = _classify_query_value(key, value)
        _count(summary, kind)
        redacted_query.append(f"{quote(key)}=<{kind} len={len(value)}>")

    return urlunsplit(
        (split.scheme, split.netloc, split.path, "&".join(redacted_query), split.fragment)
    )


def _classify_query_value(key: str, value: str) -> str:
    lowered = key.lower()
    if any(token in lowered for token in ("token", "secret", "password", "key", "auth")):
        return "TOKEN"
    if value.isdigit():
        return "NUMBER"
    return "QUERY_VALUE"


def _redact_email(text: str, summary: dict[str, int]) -> str:
    def replace(match: re.Match[str]) -> str:
        _count(summary, "EMAIL")
        return "<EMAIL>"

    return re.sub(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b", replace, text)


def _redact_absolute_paths(text: str, summary: dict[str, int]) -> str:
    def replace(match: re.Match[str]) -> str:
        _count(summary, "ABSOLUTE_PATH")
        path = match.group(0)
        leaf = path.rstrip("/").rsplit("/", 1)[-1]
        return f"<PATH>/{leaf}" if leaf else "<PATH>"

    return re.sub(r"/(?:Users|home|var|opt|srv|app|data)/[^\s\"']+", replace, text)


def _redact_ipv4(text: str, summary: dict[str, int]) -> str:
    def replace(match: re.Match[str]) -> str:
        _count(summary, "IPV4")
        return "<IPV4>"

    return re.sub(r"\b(?:\d{1,3}\.){3}\d{1,3}\b", replace, text)


def _redact_path_numbers(text: str, summary: dict[str, int]) -> str:
    def replace(match: re.Match[str]) -> str:
        number = match.group("number")
        _count(summary, "NUMBER")
        return f"/<NUMBER len={len(number)}>"

    return re.sub(r"/(?P<number>\d{4,})(?=[/?\s\"'])", replace, text)


def _redact_long_numbers(text: str, summary: dict[str, int]) -> str:
    def replace(match: re.Match[str]) -> str:
        number = match.group(0)
        _count(summary, "LONG_IDENTIFIER")
        return f"<NUMBER len={len(number)}>"

    return re.sub(r"\b\d{8,}\b", replace, text)
