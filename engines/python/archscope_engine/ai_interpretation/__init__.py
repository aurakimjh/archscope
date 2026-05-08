"""Evidence-bound AI interpretation guardrails."""
# ─────────────────────────────────────────────────────────────────────
# [한글] ai_interpretation — Evidence 기반 AI 해석 가드레일.
#
# 책임/목적
#   분석 결과(AnalysisResult)에서 추출한 "검증 가능한 evidence" 만을
#   프롬프트에 넣고, LLM 응답이 evidence 와 일치하는지 검증해 거짓/
#   환각(hallucination) 을 차단. 로컬 LLM (Ollama) 만 호출하며 외부
#   네트워크 미사용.
#
# 모듈 구성
#   - evidence    : EvidenceItem / EvidenceRegistry, ref 파싱.
#   - prompting   : PromptTemplate, PromptBuilder, evidence 선택.
#   - runtime     : LocalLlmClient (Ollama), 가용성 확인.
#   - validation  : AI 응답이 evidence 셋에 포함된 finding 만 사용
#                    하는지 검증. 위반시 AiFindingValidationError.
#   - evaluation  : 평가 메트릭 (정확도/recall) 산출.
#   - privacy     : redaction 통과 / off-by-default 정책.
#
# 정책: 외부 클라우드 호출 금지, evidence 외 추론 금지, 결과 finding
# 은 항상 evidence_ref 를 포함해 트레이서블.
# ─────────────────────────────────────────────────────────────────────

from archscope_engine.ai_interpretation.evidence import (
    EvidenceItem,
    EvidenceRegistry,
    EvidenceRef,
    collect_evidence,
    parse_evidence_ref,
)
from archscope_engine.ai_interpretation.evaluation import (
    InterpretationEvaluation,
    evaluate_interpretation,
)
from archscope_engine.ai_interpretation.prompting import (
    EvidenceSelection,
    EvidenceSelector,
    PromptBuilder,
    PromptPayload,
    PromptTemplate,
    load_packaged_prompt_templates,
    parse_prompt_templates,
    select_language_template,
    select_prompt_template,
)
from archscope_engine.ai_interpretation.runtime import (
    LocalLlmAvailability,
    LocalLlmConfig,
    LocalLlmClient,
    LocalLlmExecutionError,
    LocalLlmPolicyError,
    OllamaClient,
    validate_local_llm_config,
)
from archscope_engine.ai_interpretation.validation import (
    AiFindingValidationError,
    AiFindingValidator,
    ValidationIssue,
)

__all__ = [
    "AiFindingValidationError",
    "AiFindingValidator",
    "EvidenceItem",
    "EvidenceRef",
    "EvidenceRegistry",
    "EvidenceSelection",
    "EvidenceSelector",
    "InterpretationEvaluation",
    "LocalLlmAvailability",
    "LocalLlmClient",
    "LocalLlmConfig",
    "LocalLlmExecutionError",
    "LocalLlmPolicyError",
    "OllamaClient",
    "PromptBuilder",
    "PromptPayload",
    "PromptTemplate",
    "ValidationIssue",
    "collect_evidence",
    "evaluate_interpretation",
    "load_packaged_prompt_templates",
    "parse_evidence_ref",
    "parse_prompt_templates",
    "select_language_template",
    "select_prompt_template",
    "validate_local_llm_config",
]
