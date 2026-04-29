import { useEffect, useRef, useState } from "react";

import type { BridgeError, SelectFileRequest } from "../api/analyzerClient";
import { ErrorPanel } from "../components/AnalyzerFeedback";
import { FileDropZone } from "../components/FileDropZone";
import { useI18n } from "../i18n/I18nProvider";

type AnalyzerState = "idle" | "ready" | "running" | "error";

type PlaceholderAnalyzerProps = {
  fileLabel: string;
  fileDescription?: string;
  fileFilters?: SelectFileRequest["filters"];
};

type PlaceholderPageProps = {
  title: string;
  items: string[];
  analyzer?: PlaceholderAnalyzerProps;
};

export function PlaceholderPage({
  title,
  items,
  analyzer,
}: PlaceholderPageProps): JSX.Element {
  const { t } = useI18n();
  const [filePath, setFilePath] = useState("");
  const [state, setState] = useState<AnalyzerState>("idle");
  const [error, setError] = useState<BridgeError | null>(null);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  if (analyzer) {
    const canAnalyze = Boolean(filePath) && state !== "running";

    return (
      <div className="page">
        <section className="workspace-grid">
          <div className="tool-panel">
            <h2>{title}</h2>
            <FileDropZone
              label={analyzer.fileLabel}
              description={analyzer.fileDescription}
              selectedPath={filePath}
              browseLabel={t("browseFile")}
              onBrowse={() => void browseFile(analyzer)}
              onFileSelected={handleFileInput}
            />
            <button
              className="primary-button"
              type="button"
              disabled={!canAnalyze}
              onClick={() => void analyzePlaceholder()}
            >
              {state === "running" ? t("analyzing") : t("analyze")}
            </button>
            <ErrorPanel
              error={error}
              labels={{ title: t("analysisError"), code: t("errorCode") }}
            />
          </div>
          <PlaceholderSummary title={title} items={items} />
        </section>
      </div>
    );
  }

  return (
    <div className="page">
      <PlaceholderSummary title={title} items={items} />
    </div>
  );

  async function browseFile(nextAnalyzer: PlaceholderAnalyzerProps): Promise<void> {
    const response = await window.archscope?.selectFile?.({
      title: nextAnalyzer.fileLabel,
      filters: nextAnalyzer.fileFilters,
    });

    if (response?.filePath) {
      setFilePath(response.filePath);
      setState("ready");
      setError(null);
    }
  }

  function handleFileInput(nextPath: string | undefined): void {
    if (!nextPath) {
      setError({
        code: "FILE_PATH_UNAVAILABLE",
        message: t("filePathUnavailable"),
      });
      setState("error");
      return;
    }

    setFilePath(nextPath);
    setState("ready");
    setError(null);
  }

  async function analyzePlaceholder(): Promise<void> {
    if (!filePath || state === "running") {
      return;
    }

    setState("running");
    setError(null);
    await Promise.resolve();
    if (!mountedRef.current) {
      return;
    }

    setError({
      code: "ANALYZER_NOT_IMPLEMENTED",
      message: t("analyzerNotImplemented"),
    });
    setState("error");
  }
}

function PlaceholderSummary({
  title,
  items,
}: {
  title: string;
  items: string[];
}): JSX.Element {
  return (
    <section className="placeholder-panel">
      <h2>{title}</h2>
      <div className="placeholder-list">
        {items.map((item) => (
          <span key={item}>{item}</span>
        ))}
      </div>
    </section>
  );
}
