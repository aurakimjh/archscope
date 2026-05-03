import { useEffect, useRef, useState } from "react";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type AnalysisResult,
  type AnalysisValue,
  type BridgeError,
  type ParserDiagnostics,
  type SelectFileRequest,
} from "../api/analyzerClient";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "../components/AnalyzerFeedback";
import { FileDropZone } from "../components/FileDropZone";
import { useI18n } from "../i18n/I18nProvider";

type AnalyzerState = "idle" | "ready" | "running" | "error";

type PlaceholderAnalyzerProps = {
  fileLabel: string;
  fileDescription?: string;
  fileFilters?: SelectFileRequest["filters"];
  executionType?: "gc_log" | "thread_dump" | "exception_stack";
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
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const mountedRef = useRef(true);
  const currentRequestIdRef = useRef<string | null>(null);

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
            <div className="button-row">
              <button
                className="primary-button"
                type="button"
                disabled={!canAnalyze}
                onClick={() => void analyzePlaceholder()}
              >
                {state === "running" ? t("analyzing") : t("analyze")}
              </button>
              <button
                className="secondary-button"
                type="button"
                disabled={state !== "running"}
                onClick={() => void cancelAnalysis()}
              >
                {t("cancelAnalysis")}
              </button>
            </div>
            <ErrorPanel
              error={error}
              labels={{ title: t("analysisError"), code: t("errorCode") }}
            />
          </div>
          <div className="page">
            <PlaceholderSummary title={title} items={items} result={result} />
            <DiagnosticsPanel
              diagnostics={result?.metadata.diagnostics as ParserDiagnostics | undefined}
              labels={{
                title: t("parserDiagnostics"),
                parsedRecords: t("parsedRecords"),
                skippedLines: t("skippedLines"),
                samples: t("diagnosticSamples"),
              }}
            />
            <EngineMessagesPanel messages={engineMessages} title={t("engineMessages")} />
          </div>
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
    setResult(null);
    setEngineMessages([]);
  }

  async function analyzePlaceholder(): Promise<void> {
    if (!analyzer || !filePath || state === "running") {
      return;
    }

    setState("running");
    setError(null);
    setEngineMessages([]);
    const requestId = createAnalyzerRequestId();
    currentRequestIdRef.current = requestId;
    try {
      await Promise.resolve();
      if (!mountedRef.current) {
        return;
      }

      if (!analyzer.executionType) {
        setError({
          code: "ANALYZER_NOT_IMPLEMENTED",
          message: t("analyzerNotImplemented"),
        });
        setState("error");
        return;
      }

      const response = await getAnalyzerClient().execute({
        requestId,
        type: analyzer.executionType,
        params: { requestId, filePath, topN: 20 },
      });

      if (!mountedRef.current || currentRequestIdRef.current !== requestId) {
        return;
      }
      currentRequestIdRef.current = null;

      if (response.ok) {
        setResult(response.result);
        setEngineMessages(response.engine_messages ?? []);
        setState("ready");
        return;
      }

      if (response.error.code === "ENGINE_CANCELED") {
        setEngineMessages([t("analysisCanceled")]);
        setState("ready");
        return;
      }

      setError(response.error);
      setState("error");
    } catch (caught) {
      setError({
        code: "IPC_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
      setState("error");
    } finally {
      if (currentRequestIdRef.current === requestId) {
        currentRequestIdRef.current = null;
      }
    }
  }

  async function cancelAnalysis(): Promise<void> {
    const requestId = currentRequestIdRef.current;
    if (!requestId) {
      return;
    }
    const response = await getAnalyzerClient().cancel(requestId);
    if (!response.ok) {
      setError(response.error);
      setState("error");
    }
  }
}

function PlaceholderSummary({
  title,
  items,
  result,
}: {
  title: string;
  items: string[];
  result?: AnalysisResult | null;
}): JSX.Element {
  return (
    <section className="placeholder-panel">
      <h2>{title}</h2>
      {result ? (
        <ResultPreview result={result} />
      ) : (
        <div className="placeholder-list">
          {items.map((item) => (
            <span key={item}>{item}</span>
          ))}
        </div>
      )}
    </section>
  );
}

function ResultPreview({ result }: { result: AnalysisResult }): JSX.Element {
  const summaryEntries = Object.entries(result.summary).slice(0, 8);
  const firstTable = Object.values(result.tables).find(Array.isArray);
  const rows = Array.isArray(firstTable) ? firstTable.slice(0, 5) : [];

  return (
    <div className="placeholder-result">
      <dl className="diagnostics-grid">
        {summaryEntries.map(([key, value]) => (
          <div key={key}>
            <dt>{labelize(key)}</dt>
            <dd>{formatValue(value)}</dd>
          </div>
        ))}
      </dl>
      {rows.length > 0 && (
        <table>
          <thead>
            <tr>
              {Object.keys(rows[0] as Record<string, AnalysisValue>)
                .slice(0, 5)
                .map((key) => (
                  <th key={key}>{labelize(key)}</th>
                ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, index) => {
              const rowObject = row as Record<string, AnalysisValue>;
              return (
                <tr key={index}>
                  {Object.keys(rowObject)
                    .slice(0, 5)
                    .map((key) => (
                      <td key={key}>{formatValue(rowObject[key])}</td>
                    ))}
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </div>
  );
}

function labelize(value: string): string {
  return value.replace(/_/g, " ");
}

function formatValue(value: AnalysisValue | undefined): string {
  if (value === undefined || value === null) {
    return "-";
  }
  if (typeof value === "number") {
    return Number.isInteger(value) ? String(value) : value.toFixed(2);
  }
  if (typeof value === "object") {
    return JSON.stringify(value);
  }
  return String(value);
}
