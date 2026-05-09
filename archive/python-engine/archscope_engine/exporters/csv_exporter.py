"""CSV exporter (placeholder)."""
# [한글] csv_exporter — placeholder. Python 측 CSV export 는 후속
# 단계에서 구현 예정. 현재는 Go engine-native 의 internal/exporters/csv
# 가 캐노니컬. CLI 의 csv export 옵션은 Go 경로로 라우팅됨.
from __future__ import annotations


def write_csv_table() -> None:
    raise NotImplementedError("CSV export is planned for a later phase.")
