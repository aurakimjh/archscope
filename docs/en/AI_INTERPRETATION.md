# AI-Assisted Interpretation Design

AI-assisted interpretation is optional and evidence-bound. ArchScope must never present model output as an unsupported conclusion.

## Evidence Requirement

Every AI-generated finding, explanation, or recommendation must reference raw evidence through one or more of:

- `evidence_ref`
- `raw_line`
- `raw_block`
- `raw_preview`
- `source_file` plus row identifier

AI output without evidence references is invalid and should not be displayed in reports.

Recommended AI finding shape:

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

Rules:

- `generated_by` must explicitly mark AI-generated output.
- `evidence_refs` must be non-empty.
- `confidence` is model confidence in the interpretation, not proof.
- `limitations` must name missing evidence or uncertainty when relevant.
- Prompt inputs should include bounded evidence excerpts, not entire large log files.

## Local LLM / Ollama Layer

The first implementation should target an optional local provider such as Ollama. No cloud model call should be required for normal operation.

Process:

1. Select bounded evidence rows from `AnalysisResult.tables` and `metadata.findings`.
2. Build a prompt with result metadata, normalized metrics, and evidence excerpts.
3. Ask the local model for structured JSON following the AI finding shape.
4. Validate JSON shape and non-empty evidence references.
5. Display AI output separately from deterministic findings.

Provider boundary:

```text
AnalysisResult -> EvidenceSelector -> PromptBuilder -> LocalLlmClient -> AiFindingValidator -> Report/UI
```

Guardrails:

- Keep deterministic analyzer findings as the source of truth.
- Do not let the model invent source files, line numbers, trace IDs, or evidence references.
- Reject responses whose `evidence_refs` are not present in the input evidence set.
- Store provider/model metadata for report reproducibility.
- Allow users to disable AI interpretation completely.

## Phase Entry Criteria

Phase 5 implementation should start only after Phase 4 follow-up contracts are closed:

- common timestamp policy
- correlation evidence model
- JFR command bridge PoC
- OTel evidence retention policy
- schema-version warning path
