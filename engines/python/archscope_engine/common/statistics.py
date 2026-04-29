from __future__ import annotations

from dataclasses import dataclass, field


def average(values: list[float]) -> float:
    if not values:
        return 0.0
    return sum(values) / len(values)


def percentile(values: list[float], percent: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
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
    count: int = 0
    _samples: list[float] = field(default_factory=list)

    def add(self, value: float) -> None:
        if self.max_samples <= 0:
            raise ValueError("max_samples must be a positive integer.")

        self.count += 1
        if len(self._samples) < self.max_samples:
            self._samples.append(value)
            return

        replace_at = _deterministic_reservoir_index(self.count)
        if replace_at < self.max_samples:
            self._samples[replace_at] = value

    def percentile(self, percent: float) -> float:
        return percentile(self._samples, percent)

    @property
    def sample_size(self) -> int:
        return len(self._samples)


def _deterministic_reservoir_index(count: int) -> int:
    return ((count * 1_103_515_245) + 12_345) % count
