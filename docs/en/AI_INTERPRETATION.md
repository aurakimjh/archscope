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
- `evidence_refs` must follow `{source_type}:{entity_type}:{identifier}` and use a registered namespace such as `jfr:event:1`, `access_log:record:42`, `profiler:stack:7`, `otel:log:9`, or `timeline:correlation:3`.
- `confidence` is model confidence in the interpretation, not proof.
- `limitations` must name missing evidence or uncertainty when relevant.
- Prompt inputs should include bounded evidence excerpts, not entire large log files.

## Transport Contract

AI interpretation is transported as a separate `InterpretationResult`, not as a replacement for `AnalysisResult`. This keeps deterministic analyzer output as the source of truth.

```text
{
  schema_version: "0.1.0",
  provider: string,
  model: string,
  prompt_version: string,
  source_result_type: string,
  source_schema_version: string,
  generated_at: string,
  findings: AiFinding[],
  disabled: boolean
}
```

If an analyzer result later embeds AI output under `metadata.ai_interpretation`, the Electron IPC boundary must validate this same contract and surface compatibility warnings when it does not match.

## Runtime Enforcement

The first implementation includes code-level guardrails under `archscope_engine.ai_interpretation`:

- `EvidenceRegistry` collects canonical evidence references from analyzer output.
- `AiFindingValidator` rejects blank, malformed, unsupported, unknown, low-confidence, or quote-mismatched findings.
- `EvidenceSelector` bounds evidence count and character budgets before prompt construction.
- `PromptBuilder` separates system instructions from untrusted diagnostic data inside a delimited JSON block.
- Prompt and response logging are disabled by default.

Low-confidence output is rejected below the initial threshold of `0.3`. Partial invalid responses are treated conservatively: invalid findings are not displayed, and a validation failure should be surfaced as an engine/UI message.

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

## Prompt Injection Defense

Diagnostic inputs are untrusted data. Access logs, OTel bodies, JFR messages, and stack frames may contain attacker-controlled instructions. Prompt construction must:

- place all evidence inside a data-only block;
- explicitly tell the model to treat strings in that block as data, not instructions;
- flag suspicious instruction-like phrases for diagnostics;
- apply privacy redaction before prompt construction;
- require output validation after model response.

## Local Runtime Policy

Ollama and models are user-installed dependencies. ArchScope does not bundle model payloads in the desktop package.

Initial runtime policy:

- provider: `ollama`
- default URL: `http://localhost:11434`
- allowed hosts: `localhost`, `127.0.0.1`, `::1`
- default timeout: 30 seconds
- initial concurrency: 1
- prompt/response logging: disabled
- suggested starter model: `qwen2.5-coder:7b`
- recommended machine class: 16 GB RAM or better for predictable desktop use

If the local provider or configured model is unavailable, AI interpretation is disabled gracefully and deterministic analysis remains usable.

## Evaluation

AI interpretation changes require a small golden diagnostics set and automated checks for:

- 100% evidence reference integrity;
- quote-to-source matching;
- low-confidence filtering behavior;
- prompt/model version regression;
- hallucination and relevance review on known scenarios.

## Phase Entry Criteria

Phase 5 implementation should start only after Phase 4 follow-up contracts are closed:

- common timestamp policy
- correlation evidence model
- JFR command bridge PoC
- OTel evidence retention policy
- schema-version warning path
