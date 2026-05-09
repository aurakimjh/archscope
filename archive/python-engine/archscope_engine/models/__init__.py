# ─────────────────────────────────────────────────────────────────────
# [한글] models 패키지 — 모든 record / contract 의 단일 진실 원천.
#
# 책임/목적
#   parser → analyzer → exporter 사이를 흐르는 데이터의 shape 정의.
#   AnalysisResult 가 최상위 컨테이너이고, 도메인별 record (AccessLog,
#   GcEvent, ThreadDumpRecord 등) 가 그 안의 series/tables/metadata
#   에 들어간다.
#
# 주요 모델
#   - AnalysisResult     : 모든 분석기의 공통 응답 (type/source_files
#                          /summary/series/tables/metadata).
#   - AccessLogRecord    : nginx/apache 로그 한 줄.
#   - GcEvent            : GC 로그 한 이벤트.
#   - ThreadSnapshot     : 한 스레드의 한 시점 상태 (다언어 통합).
#   - ThreadDumpBundle   : 한 파일의 ThreadSnapshot 모음 + 메타.
#   - ProfileStack       : collapsed 프로파일의 스택+샘플.
#   - FlameNode          : flame tree 노드.
#   - 결과 contract TypedDict: AccessLog* / ProfilerCollapsed* / ...
#
# Go engine-native parity
#   apps/engine-native/internal/models/*.go 와 동일한 필드 이름/순서/
#   타입. JSON 직렬화 결과가 byte 단위 일치해야 parity 게이트 통과.
# ─────────────────────────────────────────────────────────────────────
from archscope_engine.models.access_log import AccessLogRecord
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.gc_event import GcEvent
from archscope_engine.models.profile_stack import ProfileStack
from archscope_engine.models.result_contracts import (
    AccessLogAnalysisOptions,
    AccessLogFinding,
    AccessLogMetadata,
    AccessLogSeries,
    AccessLogSummary,
    AccessLogTables,
    ParserDiagnostics,
    ProfilerCollapsedMetadata,
    ProfilerCollapsedSeries,
    ProfilerCollapsedSummary,
    ProfilerCollapsedTables,
    ProfilerTimelineScope,
)
from archscope_engine.models.thread_dump import ExceptionRecord, ThreadDumpRecord

__all__ = [
    "AccessLogRecord",
    "AccessLogAnalysisOptions",
    "AccessLogFinding",
    "AccessLogMetadata",
    "AccessLogSeries",
    "AccessLogSummary",
    "AccessLogTables",
    "AnalysisResult",
    "ExceptionRecord",
    "GcEvent",
    "ParserDiagnostics",
    "ProfileStack",
    "ProfilerCollapsedMetadata",
    "ProfilerCollapsedSeries",
    "ProfilerCollapsedSummary",
    "ProfilerCollapsedTables",
    "ProfilerTimelineScope",
    "ThreadDumpRecord",
]
