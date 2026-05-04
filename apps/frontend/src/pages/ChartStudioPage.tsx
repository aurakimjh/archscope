import { Check, Pencil, Upload, X } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import type { EChartsOption } from "echarts";

import { createChartOption } from "@/charts/chartFactory";
import {
  chartTemplates,
  getChartTemplate,
  type ChartTemplateId,
} from "@/charts/chartTemplates";
import { sampleAnalysisResult } from "@/charts/sampleCharts";
import { ChartPanel } from "@/components/ChartPanel";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useI18n } from "@/i18n/I18nProvider";

export function ChartStudioPage(): JSX.Element {
  const { t } = useI18n();
  const [templateId, setTemplateId] = useState<ChartTemplateId>(
    "AccessLog.RequestCountTrend",
  );
  const [renderer, setRenderer] = useState<"canvas" | "svg">("canvas");
  const [theme, setTheme] = useState<"archscope" | "archscope-dark">("archscope");
  const [customTitle, setCustomTitle] = useState(t("requestCountTrend"));
  const [jsonEditing, setJsonEditing] = useState(false);
  const [jsonText, setJsonText] = useState("");
  const [jsonError, setJsonError] = useState("");
  const [customOption, setCustomOption] = useState<EChartsOption | null>(null);

  const labels = useMemo(
    () => ({
      requestsAxis: t("requestsAxis"),
      millisecondsAxis: t("millisecondsAxis"),
      statusSeries: t("statusSeries"),
      samplesAxis: t("samplesAxis"),
      p95Series: t("p95Series"),
    }),
    [t],
  );
  const template = getChartTemplate(templateId);
  const baseOption = useMemo(
    () => createChartOption(templateId, sampleAnalysisResult, labels),
    [labels, templateId],
  );
  const option = customOption ?? baseOption;
  const optionPreview = useMemo(() => JSON.stringify(option, null, 2), [option]);

  function handleTemplateChange(nextId: ChartTemplateId): void {
    const nextTemplate = getChartTemplate(nextId);
    setTemplateId(nextId);
    setCustomTitle(t(nextTemplate.titleKey));
    setCustomOption(null);
    setJsonEditing(false);
    setJsonError("");
    if (!nextTemplate.supportedRenderers.includes(renderer)) {
      setRenderer(nextTemplate.supportedRenderers[0]);
    }
  }

  function startEditing(): void {
    setJsonText(optionPreview);
    setJsonEditing(true);
    setJsonError("");
  }

  function applyJson(): void {
    try {
      const parsed = JSON.parse(jsonText) as EChartsOption;
      setCustomOption(parsed);
      setJsonError("");
      setJsonEditing(false);
    } catch (e) {
      setJsonError(e instanceof Error ? e.message : "Invalid JSON");
    }
  }

  function resetJson(): void {
    setCustomOption(null);
    setJsonEditing(false);
    setJsonError("");
  }

  const loadExternalJson = useCallback(async () => {
    const response = await window.archscope?.selectFile?.({
      title: t("selectJsonResultFile"),
      filters: [{ name: t("jsonFilesFilter"), extensions: ["json"] }],
    });
    if (!response || response.canceled || !response.filePath) return;

    try {
      const text = await fetch(`/api/files?path=${encodeURIComponent(response.filePath)}`).then(
        (r) => r.text(),
      );
      const parsed = JSON.parse(text);
      if (parsed && typeof parsed === "object" && parsed.series) {
        const rebuilt = createChartOption(templateId, parsed, labels);
        setCustomOption(rebuilt);
      }
    } catch {
      // silently ignore
    }
  }, [templateId, labels, t]);

  return (
    <div className="grid gap-5 xl:grid-cols-[2fr_3fr]">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t("chartCatalog")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("chartTemplate")}</span>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={templateId}
              onChange={(event) =>
                handleTemplateChange(event.target.value as ChartTemplateId)
              }
            >
              {chartTemplates.map((item) => (
                <option key={item.id} value={item.id}>
                  {t(item.titleKey)}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("chartTitle")}</span>
            <Input
              value={customTitle}
              onChange={(event) => setCustomTitle(event.target.value)}
            />
          </label>

          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("renderer")}</span>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={renderer}
              onChange={(event) => setRenderer(event.target.value as "canvas" | "svg")}
            >
              {template.supportedRenderers.map((item) => (
                <option key={item} value={item}>
                  {item.toUpperCase()}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("defaultChartTheme")}</span>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={theme}
              onChange={(event) =>
                setTheme(event.target.value as "archscope" | "archscope-dark")
              }
            >
              <option value="archscope">{t("lightTheme")}</option>
              <option value="archscope-dark">{t("darkTheme")}</option>
            </select>
          </label>

          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={() => void loadExternalJson()}
            className="w-full"
          >
            <Upload className="h-3.5 w-3.5" />
            Load JSON
          </Button>

          <div className="flex flex-wrap gap-1.5 pt-1 text-[10px]">
            <span className="rounded bg-muted px-1.5 py-0.5 font-mono">
              {template.resultType}
            </span>
            <span className="rounded bg-muted px-1.5 py-0.5 font-mono">
              {template.chartKind}
            </span>
            {template.exportFormats.map((format) => (
              <span
                key={format}
                className="rounded bg-primary/10 px-1.5 py-0.5 font-mono text-primary"
              >
                {format.toUpperCase()}
              </span>
            ))}
          </div>
        </CardContent>
      </Card>

      <div className="flex flex-col gap-4">
        <ChartPanel
          title={customTitle || t(template.titleKey)}
          option={option}
          renderer={renderer}
          theme={theme}
        />
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
            <CardTitle className="text-sm">{t("chartOptionJson")}</CardTitle>
            <div className="flex items-center gap-1.5">
              {!jsonEditing ? (
                <Button type="button" variant="outline" size="sm" onClick={startEditing}>
                  <Pencil className="h-3 w-3" />
                  Edit
                </Button>
              ) : (
                <>
                  <Button type="button" size="sm" onClick={applyJson}>
                    <Check className="h-3 w-3" />
                    Apply
                  </Button>
                  <Button type="button" variant="outline" size="sm" onClick={resetJson}>
                    <X className="h-3 w-3" />
                    Reset
                  </Button>
                </>
              )}
            </div>
          </CardHeader>
          <CardContent>
            {jsonError && (
              <p className="mb-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                {jsonError}
              </p>
            )}
            {jsonEditing ? (
              <textarea
                className="h-72 w-full rounded-md border border-input bg-transparent px-3 py-2 font-mono text-xs leading-relaxed"
                value={jsonText}
                onChange={(e) => setJsonText(e.target.value)}
                spellCheck={false}
              />
            ) : (
              <pre className="max-h-72 overflow-auto rounded-md border border-border bg-muted/30 p-3 font-mono text-[11px] leading-relaxed">
                {optionPreview}
              </pre>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
