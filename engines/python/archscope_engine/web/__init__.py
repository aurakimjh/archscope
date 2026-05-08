"""HTTP/web layer for the ArchScope analysis engine."""
# [한글] web — FastAPI 기반 HTTP API 레이어. server.py 가 라우트 정의,
# progress.py 가 SSE 진행률 스트림. CLI 의 archscope serve 가 진입점.
# parity: 분석 결과 응답은 Go engine-native 와 동일 JSON 응답.
from archscope_engine.web.server import create_app, run

__all__ = ["create_app", "run"]
