"""Common utilities for ArchScope engine."""
# [한글] common — 모든 분석기/파서가 공유하는 유틸리티.
# 모듈: debug_log (라인별 진단 컬렉터), diagnostics (parser 진단 카운터),
# file_utils (텍스트 인코딩 감지/라인 iteration), redaction (PII 마스킹),
# statistics (BoundedPercentile reservoir sampling), time_utils
# (nginx/iso 타임스탬프 파싱, minute_bucket).
