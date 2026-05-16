import { ClipboardPlus, FileText, Trash2 } from "lucide-react";

import {
  AiInterpretationFindingsPanel,
  AiInterpretationSummary,
} from "@/components/AiInterpretationPanel";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/i18n/I18nProvider";
import {
  clearWorkspaceResults,
  removeWorkspaceResult,
  selectWorkspaceResult,
  useAnalysisWorkspace,
  type AnalysisWorkspaceEntry,
} from "@/state/analysisWorkspace";
import { getReportableAiFindings } from "@/state/aiInterpretation";
import { addEvidenceCard } from "@/state/evidenceBoard";

export function AnalysisWorkspacePage(): JSX.Element {
  const { t } = useI18n();
  const workspace = useAnalysisWorkspace();

  const addEntryEvidence = (entry: AnalysisWorkspaceEntry): void => {
    addEvidenceCard({
      analyzer: entry.result_type,
      source_kind: "source_metadata",
      title: entry.title,
      summary: `${entry.result_type} / ${entry.summary_preview.length} summary fields`,
      severity: "info",
      source_file: entry.source_files[0],
      source_ref: entry.id,
      payload: {
        result_type: entry.result_type,
        source_files: entry.source_files,
        created_at: entry.created_at,
        recorded_at: entry.recorded_at,
        summary: entry.result.summary,
      },
    });
  };

  const addAiEvidence = (entry: AnalysisWorkspaceEntry): void => {
    const reportable = getReportableAiFindings(entry.result);
    if (!reportable.interpretation || reportable.findings.length === 0) return;
    for (const finding of reportable.findings) {
      addEvidenceCard({
        analyzer: `${entry.result_type}:ai`,
        source_kind: "ai_finding",
        title: finding.label,
        summary: finding.summary,
        severity: finding.severity,
        source_file: entry.source_files[0],
        source_ref: finding.evidence_refs.join(", "),
        payload: {
          result_id: entry.id,
          result_type: entry.result_type,
          provider: reportable.interpretation.provider,
          model: finding.model ?? reportable.interpretation.model,
          prompt_version: reportable.interpretation.prompt_version,
          generated_at: reportable.interpretation.generated_at,
          gate: reportable.gate,
          finding,
        },
      });
    }
  };

  return (
    <main className="content">
      <section className="card workspace-header">
        <div>
          <h2>{t("navAnalysisWorkspace")}</h2>
          <p className="muted">{t("workspaceDescription")}</p>
        </div>
        <Button
          type="button"
          variant="outline"
          disabled={workspace.entries.length === 0}
          onClick={clearWorkspaceResults}
        >
          <Trash2 className="nav-lucide-sm" />
          {t("workspaceClear")}
        </Button>
      </section>

      {workspace.entries.length === 0 ? (
        <p className="muted">{t("workspaceEmpty")}</p>
      ) : (
        <div className="workspace-result-list">
          {workspace.entries.map((entry) => (
            <WorkspaceResultCard
              key={entry.id}
              entry={entry}
              active={entry.id === workspace.active_id}
              onSelect={() => selectWorkspaceResult(entry.id)}
              onEvidence={() => addEntryEvidence(entry)}
              onAiEvidence={() => addAiEvidence(entry)}
              onRemove={() => removeWorkspaceResult(entry.id)}
            />
          ))}
        </div>
      )}
    </main>
  );
}

function WorkspaceResultCard({
  entry,
  active,
  onSelect,
  onEvidence,
  onAiEvidence,
  onRemove,
}: {
  entry: AnalysisWorkspaceEntry;
  active: boolean;
  onSelect: () => void;
  onEvidence: () => void;
  onAiEvidence: () => void;
  onRemove: () => void;
}): JSX.Element {
  const { t } = useI18n();
  const reportableAi = getReportableAiFindings(entry.result);
  return (
    <Card className={active ? "workspace-result-card active" : "workspace-result-card"}>
      <CardHeader className="workspace-card-header">
        <CardTitle className="workspace-card-title">
          <FileText className="nav-lucide-sm" />
          {entry.title}
        </CardTitle>
        <div className="workspace-card-actions">
          <Button type="button" size="sm" variant={active ? "default" : "outline"} onClick={onSelect}>
            {active ? t("workspaceActive") : t("workspaceUse")}
          </Button>
          <Button type="button" size="sm" variant="outline" onClick={onEvidence}>
            <ClipboardPlus className="nav-lucide-sm" />
            {t("evidenceAdd")}
          </Button>
          {reportableAi.findings.length > 0 && (
            <Button type="button" size="sm" variant="outline" onClick={onAiEvidence}>
              <ClipboardPlus className="nav-lucide-sm" />
              {t("aiAddEvidence")} ({reportableAi.findings.length})
            </Button>
          )}
          <Button type="button" size="sm" variant="ghost" onClick={onRemove}>
            <Trash2 className="nav-lucide-sm" />
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="workspace-meta-grid">
          <span>{entry.result_type}</span>
          <span>{new Date(entry.recorded_at).toLocaleString()}</span>
          <span title={entry.source_files.join("\n")}>
            {entry.source_files[0] ?? t("workspaceNoSource")}
          </span>
        </div>
        <div className="workspace-summary-chips">
          {entry.summary_preview.length === 0 ? (
            <span>{t("workspaceNoSummary")}</span>
          ) : (
            entry.summary_preview.map((item) => (
              <span key={item.key}>
                {item.key}: <strong>{item.value}</strong>
              </span>
            ))
          )}
        </div>
        <AiInterpretationSummary result={entry.result} />
        <AiInterpretationFindingsPanel result={entry.result} />
      </CardContent>
    </Card>
  );
}
