# ─────────────────────────────────────────────────────────────────────
# [한글] ai_interpretation/privacy — AI 해석 prompt 에 들어가는 텍스트의
# 자격증명/시크릿 자동 마스킹.
#
# 적용 대상
#   • Authorization: Bearer <token>
#   • api_key=... / api-key:...
#   • password=... / password: ...
#   • token=... / token: ...
#
# 동작
#   prompt 직전 단계에서 redact_sensitive_text(text) 호출 → 매치된
#   토큰을 [REDACTED] 로 치환. 분석 evidence 에는 원본이 남아있지만,
#   외부 LLM 으로 보내지는 prompt 에서는 제거.
#
# 보안 정책
#   • 매칭 패턴이 중첩 가능 (Authorization 안에 token=... 같이 사용)
#     → 순서대로 적용해 누적 마스킹.
#   • 정규식만으로는 모든 시크릿을 잡을 수 없으므로 보조 수단일 뿐 —
#     AI interpretation 자체가 evidence 모드 + finding 검증으로 추가 안전장치.
# ─────────────────────────────────────────────────────────────────────
from __future__ import annotations

import re

SECRET_PATTERNS = (
    re.compile(r"(?i)(authorization:\s*bearer\s+)[A-Za-z0-9._~+/=-]+"),
    re.compile(r"(?i)(api[_-]?key\s*[=:]\s*)[A-Za-z0-9._~+/=-]+"),
    re.compile(r"(?i)(password\s*[=:]\s*)[^\s,;]+"),
    re.compile(r"(?i)(token\s*[=:]\s*)[A-Za-z0-9._~+/=-]+"),
)


def redact_sensitive_text(value: str) -> str:
    redacted = value
    for pattern in SECRET_PATTERNS:
        redacted = pattern.sub(r"\1[REDACTED]", redacted)
    return redacted
