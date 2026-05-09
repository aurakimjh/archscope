"""ArchScope Python analysis engine."""
# ─────────────────────────────────────────────────────────────────────
# [한글] archscope_engine — Python 측 분석 엔진 패키지의 최상위.
#
# 핵심 역할
#   • parsers / analyzers / exporters / models / common / web / cli /
#     ai_interpretation / config 서브패키지를 묶는 진입점.
#   • __version__ 만 노출 (0.2.0-rc1) — pip distribution 의 일관 식별자.
#
# 두 엔진 트랙 (병렬 운용)
#   • 본 패키지 (engines/python/archscope_engine) : Python 측 엔진.
#   • Go 측 (apps/engine-native)                  : 동등한 기능을 Go 로 포팅.
#   둘 다 같은 AnalysisResult JSON envelope 를 emit. parity gate 가
#   양쪽 결과를 byte 단위로 비교.
#
# 외부 진입점
#   • CLI:    `archscope-engine <subcommand>` (cli.py + Typer)
//   • Web:    `archscope serve` (web/server.py + FastAPI)
//   • Library: 직접 import — `from archscope_engine.parsers import ...`
# ─────────────────────────────────────────────────────────────────────

__version__ = "0.2.0-rc1"
