from archscope_engine.common.statistics import average, percentile


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
