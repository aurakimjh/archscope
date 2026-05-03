from archscope_engine.common.statistics import BoundedPercentile, average, percentile


def test_average_returns_zero_for_empty_values() -> None:
    assert average([]) == 0.0


def test_average_handles_single_repeated_and_negative_values() -> None:
    assert average([42.0]) == 42.0
    assert average([5.0, 5.0, 5.0]) == 5.0
    assert average([-10.0, 0.0, 10.0]) == 0.0


def test_percentile_returns_zero_for_empty_values() -> None:
    assert percentile([], 95) == 0.0


def test_percentile_handles_single_repeated_and_negative_values() -> None:
    assert percentile([42.0], 95) == 42.0
    assert percentile([5.0, 5.0, 5.0], 50) == 5.0
    assert percentile([-10.0, 0.0, 10.0], 50) == 0.0


def test_percentile_interpolates_between_ordered_values() -> None:
    assert percentile([10.0, 20.0, 30.0, 40.0], 25) == 17.5
    assert percentile([10.0, 20.0, 30.0, 40.0], 95) == 38.5


def test_bounded_percentile_keeps_sample_count_under_limit() -> None:
    stats = BoundedPercentile(max_samples=5)

    for value in range(20):
        stats.add(float(value))

    assert stats.count == 20
    assert stats.sample_size == 5
    assert stats.percentile(95) > 0


def test_bounded_percentile_seed_changes_deterministic_sample_stream() -> None:
    first = BoundedPercentile(max_samples=5, seed=1)
    second = BoundedPercentile(max_samples=5, seed=2)
    repeated = BoundedPercentile(max_samples=5, seed=1)

    for value in range(20):
        first.add(float(value))
        second.add(float(value))
        repeated.add(float(value))

    assert first.percentile(25) != second.percentile(25)
    assert first.percentile(25) == repeated.percentile(25)


def test_bounded_percentile_rejects_invalid_seed() -> None:
    try:
        BoundedPercentile(max_samples=5, seed=0)
    except ValueError as error:
        assert str(error) == "seed must be a positive integer."
    else:
        raise AssertionError("Expected BoundedPercentile to reject seed=0")


def test_bounded_percentile_large_input_represents_full_distribution() -> None:
    stats = BoundedPercentile(max_samples=1000, seed=12_345)

    for value in range(100_000):
        stats.add(float(value))

    assert stats.count == 100_000
    assert stats.sample_size == 1000
    assert 45_000 < stats.percentile(50) < 55_000
    assert 90_000 < stats.percentile(95) < 99_000


def test_bounded_percentile_cache_is_invalidated_by_new_samples() -> None:
    stats = BoundedPercentile(max_samples=10)
    for value in range(10):
        stats.add(float(value))

    assert stats.percentile(50) == 4.5
    stats.add(100.0)

    assert stats.percentile(95) >= 8.0
