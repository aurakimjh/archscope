import { Download, FileText, Loader2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import type { ExportFormat, ExportResponse } from "@/api/analyzerClient";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useI18n } from "@/i18n/I18nProvider";

type ExportMode = ExportFormat;

export function ExportCenterPage({
  initialInputPath = "",
}: {
  initialInputPath?: string;
}): JSX.Element {
  const { t } = useI18n();
  const [mode, setMode] = useState<ExportMode>("html");
  const [inputPath, setInputPath] = useState("");
  const [beforePath, setBeforePath] = useState("");
  const [afterPath, setAfterPath] = useState("");
  const [reportTitle, setReportTitle] = useState("");
  const [diffLabel, setDiffLabel] = useState("");
  const [isExporting, setIsExporting] = useState(false);
  const [response, setResponse] = useState<ExportResponse | null>(null);

  useEffect(() => {
    if (initialInputPath) {
      setMode("html");
      setInputPath(initialInputPath);
      setResponse(null);
    }
  }, [initialInputPath]);

  const exporterAvailable = Boolean(window.archscope?.exporter);
  const canExport = useMemo(() => {
    if (isExporting || !exporterAvailable) return false;
    return mode === "diff" ? Boolean(beforePath && afterPath) : Boolean(inputPath);
  }, [afterPath, beforePath, exporterAvailable, inputPath, isExporting, mode]);

  async function chooseJson(target: "input" | "before" | "after"): Promise<void> {
    const selected = await window.archscope?.selectFile?.({
      title: t("selectJsonResultFile"),
      filters: [
        { name: t("jsonFilesFilter"), extensions: ["json"] },
        { name: t("allFilesFilter"), extensions: ["*"] },
      ],
    });
    if (!selected || selected.canceled || !selected.filePath) return;
    if (target === "before") setBeforePath(selected.filePath);
    else if (target === "after") setAfterPath(selected.filePath);
    else setInputPath(selected.filePath);
    setResponse(null);
  }

  async function runExport(): Promise<void> {
    if (!window.archscope?.exporter || !canExport) return;
    setIsExporting(true);
    setResponse(null);
    try {
      const result =
        mode === "diff"
          ? await window.archscope.exporter.execute({
              format: "diff",
              beforePath,
              afterPath,
              label: diffLabel || undefined,
            })
          : await window.archscope.exporter.execute({
              format: mode,
              inputPath,
              title: reportTitle || undefined,
            });
      setResponse(result);
    } finally {
      setIsExporting(false);
    }
  }

  return (
    <div className="grid gap-5 lg:grid-cols-[2fr_3fr]">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t("exportCenter")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("exportFormat")}</span>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={mode}
              onChange={(event) => {
                setMode(event.target.value as ExportMode);
                setResponse(null);
              }}
            >
              <option value="html">{t("htmlReport")}</option>
              <option value="diff">{t("beforeAfterDiff")}</option>
              <option value="pptx">{t("powerPointReport")}</option>
            </select>
          </label>

          {mode === "diff" ? (
            <>
              <JsonPathPicker
                label={t("beforeJson")}
                path={beforePath}
                onBrowse={() => void chooseJson("before")}
              />
              <JsonPathPicker
                label={t("afterJson")}
                path={afterPath}
                onBrowse={() => void chooseJson("after")}
              />
              <label className="flex flex-col gap-1.5 text-xs">
                <span className="font-medium text-foreground/80">{t("diffLabel")}</span>
                <Input
                  type="text"
                  placeholder={t("diffLabelPlaceholder")}
                  value={diffLabel}
                  onChange={(event) => setDiffLabel(event.target.value)}
                />
              </label>
            </>
          ) : (
            <>
              <JsonPathPicker
                label={t("inputJson")}
                path={inputPath}
                onBrowse={() => void chooseJson("input")}
              />
              <label className="flex flex-col gap-1.5 text-xs">
                <span className="font-medium text-foreground/80">{t("reportTitle")}</span>
                <Input
                  type="text"
                  placeholder={t("reportTitlePlaceholder")}
                  value={reportTitle}
                  onChange={(event) => setReportTitle(event.target.value)}
                />
              </label>
            </>
          )}

          <Button
            type="button"
            disabled={!canExport}
            onClick={() => void runExport()}
            className="w-full"
          >
            {isExporting ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                {t("exporting")}
              </>
            ) : (
              <>
                <Download className="h-3.5 w-3.5" />
                {t("runExport")}
              </>
            )}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t("exportResult")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          {!exporterAvailable ? (
            <p className="rounded-md border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
              {t("exporterNotConnected")}
            </p>
          ) : response?.ok ? (
            <>
              <p className="rounded-md bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700 dark:text-emerald-400">
                {t("exportCompleted")}
              </p>
              <ul className="space-y-1 text-xs">
                {response.outputPaths.map((path) => (
                  <li
                    key={path}
                    className="flex items-center gap-2 rounded border border-border bg-muted/20 px-2 py-1.5 font-mono"
                  >
                    <FileText className="h-3 w-3 text-muted-foreground" />
                    <span className="break-all">{path}</span>
                  </li>
                ))}
              </ul>
              {response.engine_messages && response.engine_messages.length > 0 && (
                <pre className="overflow-x-auto rounded-md border border-border bg-muted/30 p-3 text-[11px] leading-relaxed">
                  {response.engine_messages.join("\n")}
                </pre>
              )}
            </>
          ) : response ? (
            <div
              className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
              role="alert"
            >
              <p>{response.error.message}</p>
              <small className="mt-1 block text-xs opacity-80">
                {response.error.code}
              </small>
              {response.error.detail && (
                <pre className="mt-2 max-h-48 overflow-auto rounded bg-destructive/5 p-2 text-[11px]">
                  {response.error.detail}
                </pre>
              )}
            </div>
          ) : (
            <p className="rounded-md border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
              {t("waitingForExport")}
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function JsonPathPicker({
  label,
  path,
  onBrowse,
}: {
  label: string;
  path: string;
  onBrowse: () => void;
}): JSX.Element {
  const { t } = useI18n();
  return (
    <div className="flex flex-col gap-1.5 text-xs">
      <span className="font-medium text-foreground/80">{label}</span>
      <div className="flex items-center gap-2">
        <div className="flex h-9 flex-1 items-center truncate rounded-md border border-input bg-muted/20 px-3 font-mono text-xs text-muted-foreground">
          {path || t("noFileSelected")}
        </div>
        <Button type="button" variant="secondary" size="sm" onClick={onBrowse}>
          {t("browseFile")}
        </Button>
      </div>
    </div>
  );
}
