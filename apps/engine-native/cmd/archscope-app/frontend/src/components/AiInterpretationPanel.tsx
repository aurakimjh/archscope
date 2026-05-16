import { BrainCircuit } from "lucide-react";

import { useI18n } from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";
import type { WorkspaceAnalysisResult } from "@/state/analysisWorkspace";
import { extractAiInterpretationProvenance } from "@/state/aiInterpretation";

type AiInterpretationSummaryProps = {
  result: WorkspaceAnalysisResult | null | undefined;
  className?: string;
};

export function AiInterpretationSummary({
  result,
  className,
}: AiInterpretationSummaryProps): JSX.Element {
  const { t } = useI18n();
  const provenance = extractAiInterpretationProvenance(result);
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

function Pill({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <span className="rounded border border-current/20 bg-background/70 px-1.5 py-0.5">
      {label}: <span className="font-mono">{value}</span>
    </span>
  );
}
