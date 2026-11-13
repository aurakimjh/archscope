import { Dialogs } from "@/bridge/dialogs";
import { Download, FileText, Loader2 } from "lucide-react";
import { useMemo, useState } from "react";

import { engine, exportEvidenceResult } from "@/bridge/engine";
import { backendWorkspaceExportSupported, exportWorkspaceResult, type WorkspaceExportMode } from "@/state/workspaceExport";
import { HelpTip, HelpedLabel, HelpedTitle } from "@/components/HelpTip";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { getHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";
import {
  selectWorkspaceResult,
  useAnalysisWorkspace,
  type AnalysisWorkspaceEntry,
} from "@/state/analysisWorkspace";

type ExportMode = WorkspaceExportMode;

const MODE_EXTENSIONS: Record<Exclude<ExportMode, "csv_dir">, string> = {
  json: "json",
  html: "html",
  pptx: "pptx",
  csv: "csv",
};

export function ExportCenterPage(): React.JSX.Element {
  const { locale, t } = useI18n();
  const workspace = useAnalysisWorkspace();
  const [selectedID, setSelectedID] = useState<string>(workspace.active_id ?? "");
  const [mode, setMode] = useState<ExportMode>("html");
  const [outputPath, setOutputPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const selectedEntry = useMemo(
    () => workspace.entries.find((entry) => entry.id === selectedID) ?? workspace.entries[0] ?? null,
    [selectedID, workspace.entries],
  );

  const backendEntry = Boolean(selectedEntry?.backend_result_handle);
  const canExport = Boolean(selectedEntry) && !busy && (!backendEntry || backendWorkspaceExportSupported(mode));

  const chooseOutput = async (): Promise<string> => {
    if (mode === "csv_dir") {
      const picked = await Dialogs.OpenFile({
        Title: t("exportChooseDirectory"),
        CanChooseDirectories: true,
        CanChooseFiles: false,
        CanCreateDirectories: true,
      });
      return Array.isArray(picked) ? picked[0] ?? "" : picked;
    }
    const ext = MODE_EXTENSIONS[mode];
    return Dialogs.SaveFile({
      Title: t("exportChooseOutput"),
      Filename: defaultFilename(selectedEntry, ext),
      Filters: [
        { DisplayName: `${ext.toUpperCase()} files`, Pattern: `*.${ext}` },
        { DisplayName: "All files", Pattern: "*.*" },
      ],
    });
  };

  const runExport = async (): Promise<void> => {
    if (!selectedEntry) return;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const destination = outputPath.trim() || (await chooseOutput());
      if (!destination) {
        setBusy(false);
        return;
      }
      setOutputPath(destination);
      await exportWorkspaceResult(selectedEntry, mode, destination, {
        backend: exportEvidenceResult,
        legacy: { json: engine.exportJSON, html: engine.exportHTML, pptx: engine.exportPPTX, csv: engine.exportCSV, csv_dir: engine.exportCSVDir },
      });
      selectWorkspaceResult(selectedEntry.id);
      setNotice(`${t("exportCompleted")}: ${destination}`);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="content workspace-tool exportcenter">
      <section className="card">
        <h2>
          <HelpedTitle help={getHelpText(locale, "pageExportCenter")}>
            {t("navExportCenter")}
          </HelpedTitle>
        </h2>
        <div className="workspace-form-grid">
          <label className="workspace-field">
            <HelpedLabel help={getHelpText(locale, "optionWorkspaceResult")}>
              {t("workspaceResult")}
            </HelpedLabel>
            <select
              value={selectedEntry?.id ?? ""}
              onChange={(event) => {
                setSelectedID(event.target.value);
                selectWorkspaceResult(event.target.value || null);
              }}
            >
              {workspace.entries.length === 0 ? (
                <option value="">{t("workspaceEmpty")}</option>
              ) : (
                workspace.entries.map((entry) => (
                  <option key={entry.id} value={entry.id}>
                    {entry.title}
                  </option>
                ))
              )}
            </select>
          </label>
          <label className="workspace-field">
            <HelpedLabel help={getHelpText(locale, "optionExportFormat")}>
              {t("exportFormat")}
            </HelpedLabel>
            <select
              value={mode}
              onChange={(event) => {
                setMode(event.target.value as ExportMode);
                setOutputPath("");
                setNotice(null);
                setError(null);
              }}
            >
              <option value="html">{t("htmlReport")}</option>
              <option value="pptx" disabled={backendEntry}>{t("powerPointReport")}</option>
              <option value="csv">{t("csvTableExport")}</option>
              <option value="csv_dir" disabled={backendEntry}>{t("csvDirectoryExport")}</option>
              <option value="json">{backendEntry ? (locale === "ko" ? "JSON (원본)" : "JSON (original)") : t("jsonExport")}</option>
            </select>
            {backendEntry && <p className="text-xs text-muted-foreground">{locale === "ko"
              ? "원본 JSON은 그대로 저장합니다. HTML·CSV는 결과 형식이 지원할 때 제공하며, 지원되지 않는 형식은 거절합니다."
              : "Original JSON is preserved. HTML and CSV require a supported result format; unsupported formats are refused."}</p>}
          </label>
          <label className="workspace-field workspace-field-wide">
            <HelpedLabel help={getHelpText(locale, "optionOutputPath")}>
              {mode === "csv_dir" ? t("exportDirectory") : t("exportOutputPath")}
            </HelpedLabel>
            <div className="workspace-path-row">
              <Input
                value={outputPath}
                onChange={(event) => setOutputPath(event.target.value)}
                placeholder={t("exportOutputPlaceholder")}
              />
              <Button type="button" variant="outline" onClick={() => void chooseOutput().then(setOutputPath)}>
                {t("browseFile")}
              </Button>
            </div>
          </label>
          <Button type="button" className="workspace-run-button" disabled={!canExport} onClick={() => void runExport()}>
            {busy ? (
              <>
                <Loader2 className="nav-lucide-sm animate-spin" />
                {t("exporting")}
              </>
            ) : (
              <>
                <Download className="nav-lucide-sm" />
                {t("runExport")}
              </>
            )}
          </Button>
        </div>
      </section>

      {selectedEntry ? <SelectedResultCard entry={selectedEntry} /> : null}

      {notice ? <div className="alert info">{notice}</div> : null}
      {error ? (
        <div className="alert error">
          <strong>{t("error")}:</strong> {error}
        </div>
      ) : null}
    </main>
  );
}

function SelectedResultCard({ entry }: { entry: AnalysisWorkspaceEntry }): React.JSX.Element {
  const { locale, t } = useI18n();
  const helpText = getHelpText(locale, "sectionExportPreview");
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="flex items-center gap-2 text-sm">
          <FileText className="nav-lucide-sm" />
          <span className="min-w-0 truncate">{entry.title}</span>
          <HelpTip text={helpText} />
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="workspace-meta-grid">
          <span>{entry.result_type}</span>
          <span>{new Date(entry.recorded_at).toLocaleString()}</span>
          <span>{entry.source_files[0] ?? t("workspaceNoSource")}</span>
        </div>
        <div className="workspace-summary-chips">
          {entry.summary_preview.map((item) => (
            <span key={item.key}>
              {item.key}: <strong>{item.value}</strong>
            </span>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

function defaultFilename(entry: AnalysisWorkspaceEntry | null, extension: string): string {
  const type = entry?.result_type || "analysis-result";
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  return `archscope-${type}-${stamp}.${extension}`;
}
