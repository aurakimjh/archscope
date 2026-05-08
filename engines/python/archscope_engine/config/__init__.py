"""Packaged configuration resources for the ArchScope Python engine."""
# ─────────────────────────────────────────────────────────────────────
# [한글] archscope_engine/config — 패키지 내부 자원.
#
# 역할
#   Python 패키지로 배포되는 정적 자원의 anchor. 실제 파일은 이 디렉토리
#   안에 직접 두고 importlib.resources 로 접근:
#     • runtime_classification_rules.json — stack 분류기 룰셋.
#     • prompt_templates.json             — AI interpretation prompt.
#
# 왜 패키지로 두는가?
#   pip 로 설치한 환경에서도 자원이 함께 따라가도록. 사용자 cwd 와
#   무관하게 importlib.resources.files("archscope_engine.config") 로
#   확실하게 찾을 수 있음.
#
# Go engine-native parity
#   같은 JSON 룰셋을 Go 측은 //go:embed 로 임베드. 룰 변경 시 양쪽
#   동기화 필요 (분류 결과의 byte parity 유지).
# ─────────────────────────────────────────────────────────────────────
