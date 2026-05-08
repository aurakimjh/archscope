"""Collapsed profiler stack record."""
# [한글] profile_stack.ProfileStack — 프로파일러 collapsed 스택의
# top_n 표용 record. stack 은 ";" join 된 문자열, frames 는 분리 리스트,
# samples 는 정수 카운트, estimated_seconds 는 interval_ms × samples
# 환산 시간, sample_ratio 는 전체 대비 백분율, elapsed_ratio 는
# elapsed_sec 가 주어졌을 때 대비 백분율 (없으면 None).
# parity: Go engine-native 의 동일 구조와 byte 일치.
from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class ProfileStack:
    stack: str
    frames: list[str]
    samples: int
    estimated_seconds: float
    sample_ratio: float
    elapsed_ratio: float | None
    category: str | None = None
