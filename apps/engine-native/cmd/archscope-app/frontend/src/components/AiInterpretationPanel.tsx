import { BrainCircuit } from "lucide-react";

import { HelpTip } from "@/components/HelpTip";
import { getHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";
import type { WorkspaceAnalysisResult } from "@/state/analysisWorkspace";
import {
  evaluateAiInterpretation,
  extractAiInterpretation,
  extractAiInterpretationProvenance,
  type AiFinding,
  type AiSeverity,
} from "@/state/aiInterpretation";

type AiInterpretationSummaryProps = {
  result: WorkspaceAnalysisResult | null | undefined;
  className?: string;
};

export function AiInterpretationSummary({
  result,
  className,
}: AiInterpretationSummaryProps): React.JSX.Element {
  const { locale, t } = useI18n();
  const provenance = extractAiInterpretationProvenance(result);
  const helpText = getHelpText(locale, "aiFindings");
  const status = provenance.present
    ? provenance.disabled
      ? t("aiProvenanceDisabled")
      : t("aiProvenancePresent")
    : t("aiProvenanceNone");
  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-2 rounded-md border border-border bg-muted/30 px-3 py-2 text-xs text-muted-foreground",
        provenance.present && "border-sky-300 bg-sky-50 text-sky-950 dark:border-sky-900 dark:bg-sky-950/30 dark:text-sky-100",
        className,
      )}
      title={provenance.source_metadata_key}
    >
      <BrainCircuit className="h-3.5 w-3.5" />
      <span className="font-medium text-foreground">{t("aiProvenanceTitle")}</span>
      <HelpTip text={helpText} />
      <span>{status}</span>
      {provenance.provider && <Pill label={t("aiProvider")} value={provenance.provider} />}
      {provenance.model && <Pill label={t("aiModel")} value={provenance.model} />}
      {provenance.prompt_version && <Pill label={t("aiPromptVersion")} value={provenance.prompt_version} />}
      {provenance.finding_count > 0 && (
        <Pill label={t("aiFindingCount")} value={provenance.finding_count.toLocaleString()} />
      )}
      {provenance.generated_at && <Pill label={t("aiGeneratedAt")} value={provenance.generated_at} />}
      {provenance.compatibility_warnings.length > 0 && (
        <Pill
          label={t("aiCompatibilityWarnings")}
          value={provenance.compatibility_warnings.length.toLocaleString()}
        />
      )}
    </div>
  );
}

export function AiInterpretationFindingsPanel({
  result,
}: {
  result: WorkspaceAnalysisResult | null | undefined;
}): React.JSX.Element | null {
  const { locale, t } = useI18n();
  const interpretation = extractAiInterpretation(result);
  if (!interpretation || interpretation.disabled || interpretation.findings.length === 0) return null;
  const gate = evaluateAiInterpretation(result, interpretation);
  const helpText = getHelpText(locale, "aiFindings");
  return (
    <section className="rounded-md border border-sky-200 bg-sky-50/80 p-3 text-sm dark:border-sky-900 dark:bg-sky-950/20">
      <header className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <BrainCircuit className="h-4 w-4 text-sky-700 dark:text-sky-300" />
          <h3 className="inline-flex items-center gap-2 text-sm font-semibold text-foreground">
            {t("aiFindingsTitle")}
            <HelpTip text={helpText} />
          </h3>
        </div>
        <span className="rounded border border-sky-300 bg-background/80 px-2 py-0.5 text-xs font-medium text-sky-900 dark:border-sky-800 dark:text-sky-100">
          {t("aiAssisted")}
        </span>
      </header>
      <div className="mb-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
        <span
          className={cn(
            "rounded border px-2 py-0.5 font-medium",
            gate.valid
              ? "border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-200"
              : "border-amber-300 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200",
          )}
        >
          {gate.valid ? t("aiGatePassed") : t("aiGateBlocked")}
        </span>
        <span>
          {t("aiEvidenceIntegrity")}: {Math.round(gate.evidence_integrity_ratio * 100)}%
        </span>
        {gate.issue_codes.length > 0 && (
          <span>
            {t("aiGateIssues")}: {gate.issue_codes.join(", ")}
          </span>
        )}
      </div>
      <div className="space-y-2">
        {interpretation.findings.map((finding) => (
          <AiFindingCard key={finding.id} finding={finding} />
        ))}
      </div>
    </section>
  );
}

function Pill({ label, value }: { label: string; value: string }): React.JSX.Element {
  return (
    <span className="rounded border border-current/20 bg-background/70 px-1.5 py-0.5">
      {label}: <span className="font-mono">{value}</span>
    </span>
  );
}

function AiFindingCard({ finding }: { finding: AiFinding }): React.JSX.Element {
  const { t } = useI18n();
  return (
    <article className="rounded border border-sky-200 bg-background p-3 dark:border-sky-900">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <span className={severityClass(finding.severity)}>{finding.severity}</span>
        <strong className="text-sm text-foreground">{finding.label}</strong>
        {finding.confidence !== undefined && (
          <span className="text-xs text-muted-foreground">
            {t("aiConfidence")}: {Math.round(finding.confidence * 100)}%
          </span>
        )}
        {finding.model && (
          <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-muted-foreground">
            {finding.model}
          </span>
        )}
      </div>
      <p className="text-sm text-foreground">{finding.summary}</p>
      {finding.reasoning && (
        <p className="mt-1 text-xs text-muted-foreground">
          {t("aiReasoning")}: {finding.reasoning}
        </p>
      )}
      {finding.evidence_refs.length > 0 && (
        <div className="mt-2">
          <span className="text-xs font-medium uppercase text-muted-foreground">{t("aiEvidenceRefs")}</span>
          <div className="mt-1 flex flex-wrap gap-1">
            {finding.evidence_refs.map((ref) => (
              <code key={ref} className="rounded bg-muted px-1.5 py-0.5 text-xs">
                {ref}
              </code>
            ))}
          </div>
        </div>
      )}
      {finding.limitations.length > 0 && (
        <div className="mt-2 text-xs text-muted-foreground">
          {t("aiLimitations")}: {finding.limitations.join(", ")}
        </div>
      )}
    </article>
  );
}

function severityClass(severity: AiSeverity): string {
  switch (severity) {
    case "critical":
      return "rounded bg-red-100 px-1.5 py-0.5 text-xs font-semibold text-red-800 dark:bg-red-950 dark:text-red-200";
    case "warning":
      return "rounded bg-amber-100 px-1.5 py-0.5 text-xs font-semibold text-amber-800 dark:bg-amber-950 dark:text-amber-200";
    default:
      return "rounded bg-sky-100 px-1.5 py-0.5 text-xs font-semibold text-sky-800 dark:bg-sky-950 dark:text-sky-200";
  }
}
