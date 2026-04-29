# AI 보조 해석 설계

AI-assisted interpretation은 optional이며 evidence-bound로 제한한다. ArchScope는 model output을 근거 없는 conclusion처럼 표시하면 안 된다.

## Evidence Requirement

AI가 생성한 finding, explanation, recommendation은 반드시 다음 중 하나 이상의 raw evidence를 참조해야 한다.

- `evidence_ref`
- `raw_line`
- `raw_block`
- `raw_preview`
- `source_file` plus row identifier

Evidence reference가 없는 AI output은 invalid이며 report에 표시하지 않는다.

권장 AI finding shape:

```text
{
  id: string,
  label: string,
  severity: "info" | "warning" | "critical",
  generated_by: "ai",
  model: string,
  summary: string,
  reasoning: string,
  evidence_refs: string[],
  confidence: number,
  limitations: string[]
}
```

규칙:

- `generated_by`는 AI-generated output임을 명시해야 한다.
- `evidence_refs`는 비어 있으면 안 된다.
- `confidence`는 proof가 아니라 interpretation에 대한 model confidence이다.
- `limitations`는 관련된 missing evidence 또는 uncertainty를 명시해야 한다.
- Prompt input은 전체 대용량 log가 아니라 bounded evidence excerpt로 제한한다.

## Local LLM / Ollama Layer

첫 구현은 Ollama 같은 optional local provider를 대상으로 한다. 정상 동작에 cloud model call이 필수이면 안 된다.

Process:

1. `AnalysisResult.tables`와 `metadata.findings`에서 bounded evidence row를 선택한다.
2. Result metadata, normalized metric, evidence excerpt로 prompt를 만든다.
3. Local model에 AI finding shape를 따르는 structured JSON을 요청한다.
4. JSON shape와 non-empty evidence reference를 검증한다.
5. AI output은 deterministic finding과 분리해서 표시한다.

Provider boundary:

```text
AnalysisResult -> EvidenceSelector -> PromptBuilder -> LocalLlmClient -> AiFindingValidator -> Report/UI
```

Guardrails:

- Deterministic analyzer finding을 source of truth로 유지한다.
- Model이 source file, line number, trace ID, evidence reference를 만들어내게 두지 않는다.
- `evidence_refs`가 input evidence set에 없으면 응답을 거부한다.
- Report reproducibility를 위해 provider/model metadata를 저장한다.
- 사용자는 AI interpretation을 완전히 비활성화할 수 있어야 한다.

## Phase Entry Criteria

Phase 5 implementation은 Phase 4 follow-up contract가 닫힌 뒤 시작한다.

- common timestamp policy
- correlation evidence model
- JFR command bridge PoC
- OTel evidence retention policy
- schema-version warning path
