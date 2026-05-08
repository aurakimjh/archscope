"""Runtime stack/exception records (Node.js, Python, Go, .NET)."""
# [한글] runtime_stack — 4개 런타임의 stack/exception 정규화 record.
# RuntimeStackRecord: runtime ("nodejs"/"python"/"go"/"dotnet"),
# record_type ("exception"/"panic"/"goroutine"), headline (1행 요약),
# message (예외 메시지), stack (frame 문자열 리스트), signature (
# 같은 stack 식별 키), raw_block (원문).
# IisAccessRecord: .NET 입력에 함께 들어오는 IIS 액세스 라인.
# parity: Go engine-native internal/models 와 동일.
from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class RuntimeStackRecord:
    runtime: str
    record_type: str
    headline: str
    message: str | None
    stack: list[str]
    signature: str
    raw_block: str


@dataclass(frozen=True)
class IisAccessRecord:
    method: str
    uri: str
    status: int
    time_taken_ms: int | None
    raw_line: str
