import type { WorkspaceAnalysisResult } from "@/state/analysisWorkspace";

const AI_METADATA_KEYS = ["ai_interpretation", "interpretation", "ai_provenance"] as const;
export const DEFAULT_AI_MIN_CONFIDENCE = 0.3;

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
  raw_severity?: string;
  generated_by: string;
  model?: string;
  summary: string;
  reasoning?: string;
  evidence_refs: string[];
  evidence_quotes?: Record<string, string>;
  confidence?: number;
  limitations: string[];
};

export type AiEvaluationGate = {
  valid: boolean;
  gate_status: "passed" | "blocked" | "not_present" | "disabled";
  total_findings: number;
  valid_findings: number;
  rejected_findings: number;
  evidence_integrity_ratio: number;
  issue_codes: string[];
  issues: Array<{
    code: string;
    message: string;
    finding_id?: string;
    evidence_ref?: string;
  }>;
};

export type AiEvaluationOptions = {
  minConfidence?: number;
  requireEvidenceQuotes?: boolean;
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
      .map((finding, index) => normalizeAiFinding(finding, index))
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

function normalizeAiFinding(value: unknown, index: number): AiFinding | null {
  if (!isPlainObject(value)) return null;
  const id = stringValue(value.id) ?? `ai-finding-${index + 1}`;
  const label = stringValue(value.label) ?? stringValue(value.code) ?? stringValue(value.title) ?? id;
  const summary = stringValue(value.summary) ?? "";
  const evidenceRefs = arrayValue(value.evidence_refs)
    .map((ref) => stringValue(ref) ?? "");
  const limitations = arrayValue(value.limitations)
    .map(stringValue)
    .filter((limitation): limitation is string => Boolean(limitation));
  return {
    id,
    label,
    severity: normalizeSeverity(stringValue(value.severity)),
    raw_severity: stringValue(value.severity),
    generated_by: stringValue(value.generated_by) ?? "",
    model: stringValue(value.model),
    summary,
    reasoning: stringValue(value.reasoning),
    evidence_refs: evidenceRefs,
    evidence_quotes: stringRecord(value.evidence_quotes),
    confidence: numberValue(value.confidence),
    limitations,
  };
}

export function evaluateAiInterpretation(
  result: WorkspaceAnalysisResult | null | undefined,
  interpretation: AiInterpretationResult | null = extractAiInterpretation(result),
  options: AiEvaluationOptions = {},
): AiEvaluationGate {
  if (!interpretation) {
    return emptyGate("not_present");
  }
  if (interpretation.disabled) {
    return emptyGate("disabled");
  }
  const evidence = buildEvidenceMap(result);
  const issues: AiEvaluationGate["issues"] = [];
  const minConfidence = options.minConfidence ?? DEFAULT_AI_MIN_CONFIDENCE;
  const requireEvidenceQuotes = options.requireEvidenceQuotes ?? true;
  if (interpretation.schema_version !== "0.1.0") {
    issues.push({ code: "SCHEMA_VERSION", message: "schema_version must be 0.1.0" });
  }
  for (const finding of interpretation.findings) {
    for (const [field, value] of [
      ["model", finding.model],
      ["summary", finding.summary],
      ["reasoning", finding.reasoning],
    ] as const) {
      if (!value) {
        issues.push({
          code: `${field.toUpperCase()}_REQUIRED`,
          message: `${field} must be a non-empty string`,
          finding_id: finding.id,
        });
      }
    }
    if (finding.generated_by !== "ai") {
      issues.push({
        code: "GENERATED_BY_REQUIRED",
        message: "generated_by must be ai",
        finding_id: finding.id,
      });
    }
    if (!validSeverity(finding.raw_severity)) {
      issues.push({
        code: "SEVERITY_INVALID",
        message: "severity must be info, warning, or critical",
        finding_id: finding.id,
      });
    }
    if (finding.confidence === undefined || finding.confidence < 0 || finding.confidence > 1) {
      issues.push({
        code: "CONFIDENCE_INVALID",
        message: "confidence must be a number between 0 and 1",
        finding_id: finding.id,
      });
    } else if (finding.confidence < minConfidence) {
      issues.push({
        code: "CONFIDENCE_TOO_LOW",
        message: `confidence must be at least ${minConfidence.toFixed(2)}`,
        finding_id: finding.id,
      });
    }
    if (finding.evidence_refs.length === 0) {
      issues.push({
        code: "EVIDENCE_REFS_REQUIRED",
        message: "evidence_refs must be a non-empty array",
        finding_id: finding.id,
      });
    }
    if (requireEvidenceQuotes && !finding.evidence_quotes) {
      issues.push({
        code: "EVIDENCE_QUOTES_REQUIRED",
        message: "evidence_quotes must be present when quote matching is required",
        finding_id: finding.id,
      });
    }
    for (const ref of finding.evidence_refs) {
      const evidenceRef = ref.trim();
      if (!evidenceRef) {
        issues.push({
          code: "EVIDENCE_REF_BLANK",
          message: "evidence_refs must contain non-empty strings",
          finding_id: finding.id,
        });
        continue;
      }
      if (!parseEvidenceRef(evidenceRef)) {
        issues.push({
          code: "EVIDENCE_REF_GRAMMAR",
          message: "invalid evidence_ref grammar",
          finding_id: finding.id,
          evidence_ref: evidenceRef,
        });
        continue;
      }
      const sourceText = evidence.get(evidenceRef);
      if (!sourceText) {
        issues.push({
          code: "EVIDENCE_REF_UNKNOWN",
          message: "evidence_ref is not present in the input evidence set",
          finding_id: finding.id,
          evidence_ref: evidenceRef,
        });
        continue;
      }
      const quote = finding.evidence_quotes?.[evidenceRef];
      if (requireEvidenceQuotes && !quote) {
        issues.push({
          code: "EVIDENCE_QUOTE_REQUIRED",
          message: "evidence_quotes must include a non-empty quote for every evidence_ref",
          finding_id: finding.id,
          evidence_ref: evidenceRef,
        });
        continue;
      }
      if (quote && !normalizeText(sourceText).includes(normalizeText(quote))) {
        issues.push({
          code: "EVIDENCE_QUOTE_MISMATCH",
          message: "evidence quote is not present in source evidence",
          finding_id: finding.id,
          evidence_ref: evidenceRef,
        });
      }
    }
  }
  const total = interpretation.findings.length;
  const evidenceIssueCount = issues.filter((issue) => isEvidenceIssue(issue.code)).length;
  const evidenceIntegrityRatio = total > 0 ? Math.max(0, 1 - evidenceIssueCount / total) : 1;
  const valid = issues.length === 0;
  return {
    valid,
    gate_status: valid ? "passed" : "blocked",
    total_findings: total,
    valid_findings: valid ? total : 0,
    rejected_findings: valid ? 0 : total,
    evidence_integrity_ratio: evidenceIntegrityRatio,
    issue_codes: Array.from(new Set(issues.map((issue) => issue.code))).sort(),
    issues,
  };
}

export function getReportableAiFindings(
  result: WorkspaceAnalysisResult | null | undefined,
): {
  interpretation: AiInterpretationResult | null;
  gate: AiEvaluationGate;
  findings: AiFinding[];
} {
  const interpretation = extractAiInterpretation(result);
  const gate = evaluateAiInterpretation(result, interpretation);
  return {
    interpretation,
    gate,
    findings: gate.valid && interpretation ? interpretation.findings : [],
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

function validSeverity(value: string | undefined): boolean {
  return value === "info" || value === "warning" || value === "critical";
}

function emptyGate(status: AiEvaluationGate["gate_status"]): AiEvaluationGate {
  return {
    valid: false,
    gate_status: status,
    total_findings: 0,
    valid_findings: 0,
    rejected_findings: 0,
    evidence_integrity_ratio: 0,
    issue_codes: [],
    issues: [],
  };
}

function arrayValue(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown): string | undefined {
  if (typeof value === "string") return value.trim() || undefined;
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

function buildEvidenceMap(result: WorkspaceAnalysisResult | null | undefined): Map<string, string> {
  const evidence = new Map<string, string>();
  collectEvidence(result, evidence);
  return evidence;
}

function collectEvidence(value: unknown, evidence: Map<string, string>): void {
  if (Array.isArray(value)) {
    for (const item of value) collectEvidence(item, evidence);
    return;
  }
  if (!isPlainObject(value)) return;
  const ref = stringValue(value.evidence_ref);
  if (ref) {
    evidence.set(ref, evidenceText(value, ref));
  }
  for (const child of Object.values(value)) collectEvidence(child, evidence);
}

function evidenceText(value: Record<string, unknown>, fallback: string): string {
  for (const key of ["message", "summary", "text", "description", "event_type", "thread", "signature"]) {
    const text = stringValue(value[key]);
    if (text) return text;
  }
  return fallback;
}

function parseEvidenceRef(value: string): boolean {
  return /^[a-z][a-z0-9_]*:[a-z][a-z0-9_]*:[A-Za-z0-9_.-]+$/.test(value.trim());
}

function normalizeText(value: string): string {
  return value.split(/\s+/).filter(Boolean).join(" ").toLowerCase();
}

function isEvidenceIssue(code: string): boolean {
  return (
    code === "EVIDENCE_REFS_REQUIRED" ||
    code === "EVIDENCE_REF_BLANK" ||
    code === "EVIDENCE_REF_GRAMMAR" ||
    code === "EVIDENCE_REF_UNKNOWN" ||
    code === "EVIDENCE_QUOTES_REQUIRED" ||
    code === "EVIDENCE_QUOTE_REQUIRED" ||
    code === "EVIDENCE_QUOTE_MISMATCH"
  );
}
