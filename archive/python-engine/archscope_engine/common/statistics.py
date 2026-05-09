"""Bounded percentile via reservoir sampling."""
# [한글] statistics — 스트리밍 통계 헬퍼.
# average / percentile: 단순 list 기반 평균/백분위.
# BoundedPercentile: Vitter 알고리즘 R 변형 reservoir sampling.
#   - max_samples 까지 채우고, 이후엔 N 개째 들어오는 값을 1/N 확률로
#     reservoir 의 임의 슬롯에 교체. percentile 호출 시 reservoir 만
#     정렬해 근사값 반환. 메모리 사용 일정 → 수억 개 요청 처리 가능.
#   - seed 고정 (12_345) 으로 결과 재현성 보장.
# parity: BoundedPercentile 의 max_samples (10_000), seed (12_345),
# 알고리즘이 Go engine-native internal/statistics 와 동일.
from __future__ import annotations

from dataclasses import dataclass, field
from random import Random


def average(values: list[float]) -> float:
    if not values:
        return 0.0
    return sum(values) / len(values)


def percentile(values: list[float], percent: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    return _percentile_from_sorted(ordered, percent)


def _percentile_from_sorted(ordered: list[float], percent: float) -> float:
    if len(ordered) == 1:
        return ordered[0]
    rank = (len(ordered) - 1) * (percent / 100)
    lower = int(rank)
    upper = min(lower + 1, len(ordered) - 1)
    weight = rank - lower
    return ordered[lower] * (1 - weight) + ordered[upper] * weight


@dataclass
class BoundedPercentile:
    max_samples: int = 10_000
    seed: int = 12_345
    count: int = 0
    _samples: list[float] = field(default_factory=list)
    _rng: Random = field(init=False, repr=False)
    _sorted_cache: list[float] | None = field(default=None, init=False, repr=False)

    def __post_init__(self) -> None:
        if self.max_samples <= 0:
            raise ValueError("max_samples must be a positive integer.")
        if self.seed <= 0:
            raise ValueError("seed must be a positive integer.")
        self._rng = Random(self.seed)

    def add(self, value: float) -> None:
        self.count += 1
        self._sorted_cache = None
        if len(self._samples) < self.max_samples:
            self._samples.append(value)
            return

        replace_at = self._rng.randrange(self.count)
        if replace_at < self.max_samples:
            self._samples[replace_at] = value

    def percentile(self, percent: float) -> float:
        if not self._samples:
            return 0.0
        if self._sorted_cache is None:
            self._sorted_cache = sorted(self._samples)
        return _percentile_from_sorted(self._sorted_cache, percent)

    @property
    def sample_size(self) -> int:
        return len(self._samples)
