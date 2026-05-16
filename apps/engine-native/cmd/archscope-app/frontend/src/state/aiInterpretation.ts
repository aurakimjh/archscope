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

export type AiSeverity = "info" | "warning" | "critical";

export type AiFinding = {
  id: string;
  label: string;
  severity: AiSeverity;
  generated_by: string;
  model?: string;
  summary: string;
  reasoning?: string;
  evidence_refs: string[];
  evidence_quotes?: Record<string, string>;
  confidence?: number;
  limitations: string[];
};

export type AiInterpretationResult = {
  schema_version?: string;
  provider?: string;
  model?: string;
  prompt_version?: string;
  source_result_type?: string;
  source_schema_version?: string;
  generated_at?: string;
  disabled: boolean;
  findings: AiFinding[];
  source_metadata_key: string;
};

type AiInterpretationCandidate = {
  key: (typeof AI_METADATA_KEYS)[number];
  value: Record<string, unknown>;
};

export function extractAiInterpretation(
  result: WorkspaceAnalysisResult | null | undefined,
): AiInterpretationResult | null {
  const candidate = findAiInterpretationCandidate(result);
  if (!candidate) return null;
  const value = candidate.value;
  const fallbackModel = stringValue(value.model);
  return {
    schema_version: stringValue(value.schema_version),
    provider: stringValue(value.provider),
    model: fallbackModel,
    prompt_version: stringValue(value.prompt_version),
    source_result_type: stringValue(value.source_result_type) ?? result?.type,
    source_schema_version: stringValue(value.source_schema_version) ?? stringValue(result?.metadata?.schema_version),
    generated_at: stringValue(value.generated_at),
    disabled: value.disabled === true,
    findings: arrayValue(value.findings)
      .map((finding, index) => normalizeAiFinding(finding, index, fallbackModel))
      .filter((finding): finding is AiFinding => finding !== null),
    source_metadata_key: candidate.key,
  };
}

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

function normalizeAiFinding(value: unknown, index: number, fallbackModel?: string): AiFinding | null {
  if (!isPlainObject(value)) return null;
  const id = stringValue(value.id) ?? `ai-finding-${index + 1}`;
  const label = stringValue(value.label) ?? stringValue(value.code) ?? stringValue(value.title) ?? id;
  const summary = stringValue(value.summary) ?? stringValue(value.message) ?? label;
  const evidenceRefs = arrayValue(value.evidence_refs)
    .map(stringValue)
    .filter((ref): ref is string => Boolean(ref));
  const limitations = arrayValue(value.limitations)
    .map(stringValue)
    .filter((limitation): limitation is string => Boolean(limitation));
  return {
    id,
    label,
    severity: normalizeSeverity(stringValue(value.severity)),
    generated_by: stringValue(value.generated_by) ?? "ai",
    model: stringValue(value.model) ?? fallbackModel,
    summary,
    reasoning: stringValue(value.reasoning),
    evidence_refs: evidenceRefs,
    evidence_quotes: stringRecord(value.evidence_quotes),
    confidence: numberValue(value.confidence),
    limitations,
  };
}

function normalizeSeverity(value: string | undefined): AiSeverity {
  switch (value?.toLowerCase()) {
    case "critical":
    case "fatal":
      return "critical";
    case "warning":
    case "warn":
    case "error":
    case "high":
    case "medium":
      return "warning";
    default:
      return "info";
  }
}

function arrayValue(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown): string | undefined {
  if (typeof value === "string") return value || undefined;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return undefined;
}

function numberValue(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function stringRecord(value: unknown): Record<string, string> | undefined {
  if (!isPlainObject(value)) return undefined;
  const entries = Object.entries(value)
    .map(([key, item]) => [key, stringValue(item)] as const)
    .filter((entry): entry is readonly [string, string] => Boolean(entry[1]));
  return entries.length > 0 ? Object.fromEntries(entries) : undefined;
}
