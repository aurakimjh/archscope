import { useState } from "react";

import {
  getAnalyzerClient,
  type BridgeError,
  type ProfilerCollapsedAnalysisResult,
  type ProfilerTopStackTableRow,
} from "../api/analyzerClient";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "../components/AnalyzerFeedback";
import { FileDropZone } from "../components/FileDropZone";
import { MetricCard } from "../components/MetricCard";
import { useI18n } from "../i18n/I18nProvider";
import {
  formatMilliseconds,
  formatNumber,
  formatPercent,
  formatSeconds,
} from "../utils/formatters";

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

type TopStackRow = {
  stack: string;
  samples: number;
  estimated_seconds: number;
  sample_ratio: number;
};

export function ProfilerAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [wallPath, setWallPath] = useState("");
  const [wallIntervalMs, setWallIntervalMs] = useState(100);
  const [elapsedSec, setElapsedSec] = useState("");
  const [topN, setTopN] = useState(20);
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<ProfilerCollapsedAnalysisResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);

  const canAnalyze = Boolean(wallPath) && wallIntervalMs > 0 && state !== "running";
  const summary = result?.summary;
  const topStacks = getTopStackRows(result?.tables.top_stacks);

  async function browseWallFile(): Promise<void> {
    const response = await window.archscope?.selectFile?.({
      title: t("selectWallCollapsedFile"),
      filters: [
        { name: t("collapsedStackFilesFilter"), extensions: ["collapsed", "txt"] },
        { name: t("allFilesFilter"), extensions: ["*"] },
      ],
    });

    if (response?.filePath) {
      setWallPath(response.filePath);
      setState("ready");
      setError(null);
      setEngineMessages([]);
    }
  }

  function handleWallFileInput(nextPath: string | undefined): void {
    if (!nextPath) {
      setError({
        code: "FILE_PATH_UNAVAILABLE",
        message: t("filePathUnavailable"),
      });
      setState("error");
      return;
    }

    setWallPath(nextPath);
    setState("ready");
    setError(null);
    setEngineMessages([]);
  }

  async function analyze(): Promise<void> {
    if (!canAnalyze) {
      return;
    }

    setState("running");
    setError(null);
    setEngineMessages([]);

    try {
      const parsedElapsedSec = parseOptionalPositiveNumber(elapsedSec);
      if (parsedElapsedSec === null || !Number.isFinite(topN) || topN <= 0) {
        setError({
          code: "INVALID_OPTION",
          message: t("invalidAnalyzerOptions"),
        });
        setState("error");
        return;
      }

      const response = await getAnalyzerClient().analyzeCollapsedProfile({
        wallPath,
        wallIntervalMs,
        elapsedSec: parsedElapsedSec,
        topN,
      });

      if (response.ok) {
        setResult(response.result);
        setEngineMessages(response.engine_messages ?? []);
        setState("success");
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
    }
  }

  return (
    <div className="page">
      <section className="workspace-grid">
        <div className="tool-panel">
          <h2>{t("profilerAnalyzer")}</h2>
          <FileDropZone
            label={t("selectWallCollapsedFile")}
            selectedPath={wallPath}
            browseLabel={t("browseFile")}
            onBrowse={browseWallFile}
            onFileSelected={handleWallFileInput}
          />
          <div className="input-grid">
            <label className="field">
              <span>{t("wallIntervalMs")}</span>
              <input
                type="number"
                value={wallIntervalMs}
                min={1}
                onChange={(event) => setWallIntervalMs(Number(event.target.value))}
              />
            </label>
            <label className="field">
              <span>{t("elapsedSeconds")}</span>
              <input
                type="number"
                value={elapsedSec}
                placeholder="1336.559"
                min={0}
                step="0.001"
                onChange={(event) => setElapsedSec(event.target.value)}
              />
            </label>
            <label className="field">
              <span>{t("topN")}</span>
              <input
                type="number"
                value={topN}
                min={1}
                onChange={(event) => setTopN(Number(event.target.value))}
              />
            </label>
          </div>
          <button
            className="primary-button"
            type="button"
            disabled={!canAnalyze}
            onClick={() => void analyze()}
          >
            {state === "running" ? t("analyzing") : t("analyze")}
          </button>
          <ErrorPanel
            error={error}
            labels={{ title: t("analysisError"), code: t("errorCode") }}
          />
          <EngineMessagesPanel
            messages={engineMessages}
            title={t("engineMessages")}
          />
        </div>
        <div>
          <section className="summary-grid compact">
            <MetricCard
              label={t("totalWallSamples")}
              value={formatNumber(summary?.total_samples)}
            />
            <MetricCard
              label={t("wallIntervalMs")}
              value={formatMilliseconds(summary?.interval_ms)}
            />
            <MetricCard
              label={t("estimatedWallTime")}
              value={formatSeconds(summary?.estimated_seconds)}
            />
            <MetricCard
              label={t("elapsedSeconds")}
              value={formatSeconds(summary?.elapsed_seconds)}
            />
          </section>
          <section className="table-panel">
            <div className="panel-header">
              <h2>{t("topStackTable")}</h2>
            </div>
            <table>
              <thead>
                <tr>
                  <th>{t("stack")}</th>
                  <th>{t("samples")}</th>
                  <th>{t("estimatedSeconds")}</th>
                  <th>{t("ratio")}</th>
                </tr>
              </thead>
              <tbody>
                {topStacks.length > 0 ? (
                  topStacks.map((row) => (
                    <tr key={row.stack}>
                      <td>{row.stack}</td>
                      <td>{row.samples.toLocaleString()}</td>
                      <td>{formatSeconds(row.estimated_seconds)}</td>
                      <td>{formatPercent(row.sample_ratio)}</td>
                    </tr>
                  ))
                ) : (
                  <tr>
                    <td>{t("waitingProfilerResult")}</td>
                    <td>-</td>
                    <td>-</td>
                    <td>-</td>
                  </tr>
                )}
              </tbody>
            </table>
          </section>
          <DiagnosticsPanel
            diagnostics={result?.metadata.diagnostics}
            labels={{
              title: t("parserDiagnostics"),
              parsedRecords: t("parsedRecords"),
              skippedLines: t("skippedLines"),
              samples: t("diagnosticSamples"),
            }}
          />
        </div>
      </section>
    </div>
  );
}

function getTopStackRows(value: ProfilerTopStackTableRow[] | undefined): TopStackRow[] {
  return value ?? [];
}

function parseOptionalPositiveNumber(value: string): number | undefined | null {
  if (!value.trim()) {
    return undefined;
  }

  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }

  return parsed;
}
