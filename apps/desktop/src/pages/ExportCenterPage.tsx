import { useMemo, useState } from "react";

import type { ExportFormat, ExportResponse } from "../api/analyzerClient";
import { useI18n } from "../i18n/I18nProvider";

type ExportMode = ExportFormat;

export function ExportCenterPage(): JSX.Element {
  const { t } = useI18n();
  const [mode, setMode] = useState<ExportMode>("html");
  const [inputPath, setInputPath] = useState("");
  const [beforePath, setBeforePath] = useState("");
  const [afterPath, setAfterPath] = useState("");
  const [isExporting, setIsExporting] = useState(false);
  const [response, setResponse] = useState<ExportResponse | null>(null);

  const canExport = useMemo(() => {
    if (isExporting || !window.archscope?.exporter) {
      return false;
    }
    return mode === "diff" ? Boolean(beforePath && afterPath) : Boolean(inputPath);
  }, [afterPath, beforePath, inputPath, isExporting, mode]);

  async function chooseJson(target: "input" | "before" | "after"): Promise<void> {
    const selected = await window.archscope?.selectFile?.({
      title: t("selectJsonResultFile"),
      filters: [
        { name: t("jsonFilesFilter"), extensions: ["json"] },
        { name: t("allFilesFilter"), extensions: ["*"] },
      ],
    });
    if (!selected || selected.canceled || !selected.filePath) {
      return;
    }
    if (target === "before") {
      setBeforePath(selected.filePath);
    } else if (target === "after") {
      setAfterPath(selected.filePath);
    } else {
      setInputPath(selected.filePath);
    }
    setResponse(null);
  }

  async function runExport(): Promise<void> {
    if (!window.archscope?.exporter || !canExport) {
      return;
    }
    setIsExporting(true);
    setResponse(null);
    try {
      const result =
        mode === "diff"
          ? await window.archscope.exporter.execute({
              format: "diff",
              beforePath,
              afterPath,
            })
          : await window.archscope.exporter.execute({
              format: mode,
              inputPath,
            });
      setResponse(result);
    } finally {
      setIsExporting(false);
    }
  }

  return (
    <div className="page">
      <div className="workspace-grid export-center-grid">
        <section className="tool-panel">
          <h2>{t("exportCenter")}</h2>
          <label className="field">
            {t("exportFormat")}
            <select
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
            </>
          ) : (
            <JsonPathPicker
              label={t("inputJson")}
              path={inputPath}
              onBrowse={() => void chooseJson("input")}
            />
          )}

          <button
            className="primary-button"
            type="button"
            disabled={!canExport}
            onClick={() => void runExport()}
          >
            {isExporting ? t("exporting") : t("runExport")}
          </button>
        </section>

        <section className="table-panel">
          <div className="panel-header">
            <h2>{t("exportResult")}</h2>
          </div>
          {!window.archscope?.exporter ? (
            <div className="message-panel info-panel">
              <p>{t("exporterNotConnected")}</p>
            </div>
          ) : response?.ok ? (
            <div className="message-panel info-panel">
              <p>{t("exportCompleted")}</p>
              <ul className="output-path-list">
                {response.outputPaths.map((path) => (
                  <li key={path}>{path}</li>
                ))}
              </ul>
              {response.engine_messages && response.engine_messages.length > 0 ? (
                <pre>{response.engine_messages.join("\n")}</pre>
              ) : null}
            </div>
          ) : response ? (
            <div className="message-panel error-panel" role="alert">
              <p>{response.error.message}</p>
              <small>{response.error.code}</small>
              {response.error.detail ? <pre>{response.error.detail}</pre> : null}
            </div>
          ) : (
            <div className="message-panel info-panel">
              <p>{t("waitingForExport")}</p>
            </div>
          )}
        </section>
      </div>
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
    <div className="export-path-picker">
      <span>{label}</span>
      <div className="selected-file">{path || t("noFileSelected")}</div>
      <button className="secondary-button" type="button" onClick={onBrowse}>
        {t("browseFile")}
      </button>
    </div>
  );
}
