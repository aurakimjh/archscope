import type { WorkspaceAnalysisResult } from "@/state/analysisWorkspace";

const AI_METADATA_KEYS = ["ai_interpretation", "interpretation", "ai_provenance"] as const;

export type AiInterpretationProvenance = {
  present: boolean;
  disabled: boolean;
  source_metadata_key?: string;
  provider?: string;
  model?: string;
  prompt_version?: string;
  generated_at?: string;
  source_result_type?: string;
  source_schema_version?: string;
  finding_count: number;
  compatibility_warnings: string[];
};

type AiInterpretationCandidate = {
  key: (typeof AI_METADATA_KEYS)[number];
  value: Record<string, unknown>;
};

export function extractAiInterpretationProvenance(
  result: WorkspaceAnalysisResult | null | undefined,
): AiInterpretationProvenance {
  const candidate = findAiInterpretationCandidate(result);
  if (!candidate) {
    return {
      present: false,
      disabled: false,
      finding_count: 0,
      compatibility_warnings: [],
    };
  }
  const value = candidate.value;
  const findings = Array.isArray(value.findings) ? value.findings : [];
  const warnings: string[] = [];
  if (value.findings !== undefined && !Array.isArray(value.findings)) {
    warnings.push("findings is not an array");
  }
  if (value.provider !== undefined && !stringValue(value.provider)) {
    warnings.push("provider is not a string");
  }
  if (value.model !== undefined && !stringValue(value.model)) {
    warnings.push("model is not a string");
  }
  return {
    present: true,
    disabled: value.disabled === true,
    source_metadata_key: candidate.key,
    provider: stringValue(value.provider),
    model: stringValue(value.model),
    prompt_version: stringValue(value.prompt_version),
    generated_at: stringValue(value.generated_at),
    source_result_type: stringValue(value.source_result_type) ?? result?.type,
    source_schema_version: stringValue(value.source_schema_version) ?? stringValue(result?.metadata?.schema_version),
    finding_count: findings.length,
    compatibility_warnings: warnings,
  };
}

function findAiInterpretationCandidate(
  result: WorkspaceAnalysisResult | null | undefined,
): AiInterpretationCandidate | null {
  const metadata = result?.metadata;
  if (!isPlainObject(metadata)) return null;
  for (const key of AI_METADATA_KEYS) {
    const value = metadata[key];
    if (isPlainObject(value)) return { key, value };
  }
  return null;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown): string | undefined {
  if (typeof value === "string") return value || undefined;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return undefined;
}
