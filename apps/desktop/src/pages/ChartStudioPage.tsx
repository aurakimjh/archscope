import { useCallback, useMemo, useState } from "react";

import { createChartOption } from "../charts/chartFactory";
import {
  chartTemplates,
  getChartTemplate,
  type ChartTemplateId,
} from "../charts/chartTemplates";
import { sampleAnalysisResult } from "../charts/sampleCharts";
import { ChartPanel } from "../components/ChartPanel";
import { useI18n } from "../i18n/I18nProvider";
import type { EChartsOption } from "echarts";

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
      const text = await fetch(`file://${response.filePath}`).then((r) => r.text());
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
    <div className="page">
      <div className="workspace-grid chart-studio-grid">
        <section className="tool-panel">
          <h2>{t("chartCatalog")}</h2>
          <label className="field">
            {t("chartTemplate")}
            <select
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
          <label className="field">
            {t("chartTitle")}
            <input
              value={customTitle}
              onChange={(event) => setCustomTitle(event.target.value)}
            />
          </label>
          <label className="field">
            {t("renderer")}
            <select
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
          <label className="field">
            {t("defaultChartTheme")}
            <select
              value={theme}
              onChange={(event) =>
                setTheme(event.target.value as "archscope" | "archscope-dark")
              }
            >
              <option value="archscope">{t("lightTheme")}</option>
              <option value="archscope-dark">{t("darkTheme")}</option>
            </select>
          </label>
          <div className="button-row">
            <button type="button" className="secondary-button" onClick={() => void loadExternalJson()}>
              Load JSON
            </button>
          </div>
          <div className="template-meta">
            <span>{template.resultType}</span>
            <span>{template.chartKind}</span>
            {template.exportFormats.map((format) => (
              <span key={format}>{format.toUpperCase()}</span>
            ))}
          </div>
        </section>

        <div className="chart-studio-preview">
          <ChartPanel
            title={customTitle || t(template.titleKey)}
            option={option}
            renderer={renderer}
            theme={theme}
          />
          <section className="table-panel">
            <div className="panel-header">
              <h2>{t("chartOptionJson")}</h2>
              <div className="button-row">
                {!jsonEditing ? (
                  <button type="button" className="secondary-button" onClick={startEditing}>
                    Edit
                  </button>
                ) : (
                  <>
                    <button type="button" className="primary-button" onClick={applyJson}>
                      Apply
                    </button>
                    <button type="button" className="secondary-button" onClick={resetJson}>
                      Reset
                    </button>
                  </>
                )}
              </div>
            </div>
            {jsonError && <p className="settings-status error">{jsonError}</p>}
            {jsonEditing ? (
              <textarea
                className="json-editor"
                value={jsonText}
                onChange={(e) => setJsonText(e.target.value)}
                spellCheck={false}
              />
            ) : (
              <pre className="json-preview">{optionPreview}</pre>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}
