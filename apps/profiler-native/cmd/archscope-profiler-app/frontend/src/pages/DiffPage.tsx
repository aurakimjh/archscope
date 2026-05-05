import { useMemo, useState } from "react";

import {
  ProfilerService,
  DiffRequest,
} from "../../bindings/github.com/aurakimjh/archscope/apps/profiler-native/cmd/archscope-profiler-app";
import type { AnalysisResult } from "../../bindings/github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler/models";
import type { FlameNode } from "../../bindings/github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler/models";

import { useI18n } from "../i18n/I18nProvider";
import { CanvasFlameGraph, type FlameGraphNode } from "../components/CanvasFlameGraph";

type DiffFormat = "collapsed" | "jennifer" | "flamegraph_svg" | "flamegraph_html";

function detectFormat(path: string): DiffFormat {
  const lower = path.toLowerCase();
  if (lower.endsWith(".csv")) return "jennifer";
  if (lower.endsWith(".svg")) return "flamegraph_svg";
  if (lower.endsWith(".html") || lower.endsWith(".htm")) return "flamegraph_html";
  return "collapsed";
}

function adaptDiffNode(node: FlameNode | null | undefined): FlameGraphNode | null {
  if (!node) return null;
  const meta = (node as any).metadata as { delta?: number } | undefined;
  const delta = meta && typeof meta.delta === "number" ? meta.delta : 0;
  // Color the diff with a divergent palette: red (regression), green
  // (improvement), gray (unchanged).
  let color: string | null = null;
  if (delta > 0.001) {
    color = "#ef4444";
  } else if (delta < -0.001) {
    color = "#22c55e";
  } else {
    color = "#94a3b8";
  }
  const children = (node.children ?? [])
    .map(adaptDiffNode)
    .filter((child): child is FlameGraphNode => child !== null);
  return {
    name: node.name ?? "",
    value: node.samples ?? 0,
    color,
    category: null,
    children,
  };
}

export function DiffPage() {
  const { t } = useI18n();
  const [baseline, setBaseline] = useState("");
  const [target, setTarget] = useState("");
  const [normalize, setNormalize] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<AnalysisResult | null>(null);

  const flame = useMemo(() => adaptDiffNode(result?.charts?.flamegraph), [result]);

  const pickFile = async (setter: (path: string) => void) => {
    try {
      const path = await ProfilerService.PickProfileFile();
      if (path) setter(path);
    } catch (err: any) {
      setError(String(err?.message ?? err));
    }
  };

  const runDiff = async () => {
    setError("");
    setBusy(true);
    try {
      const request = new DiffRequest({
        baselinePath: baseline,
        targetPath: target,
        baselineFormat: detectFormat(baseline),
        targetFormat: detectFormat(target),
        intervalMs: 100,
        topN: 20,
        normalize,
      });
      const res = await ProfilerService.Diff(request);
      setResult(res);
    } catch (err: any) {
      setError(String(err?.message ?? err));
      setResult(null);
    } finally {
      setBusy(false);
    }
  };

  const summary = result?.metadata?.diff_summary as
    | { [key: string]: any }
    | undefined;
  const tables = result?.metadata?.diff_tables as { [key: string]: any } | undefined;
  const increases = (tables?.biggest_increases ?? []) as Array<Record<string, any>>;
  const decreases = (tables?.biggest_decreases ?? []) as Array<Record<string, any>>;

  return (
    <main className="content">
      <section className="card">
        <h2>{t("diffTitle")}</h2>
        <div className="diff-form">
          <DiffFileRow label={t("diffBaseline")} value={baseline} onPick={() => pickFile(setBaseline)} onChange={setBaseline} disabled={busy} />
          <DiffFileRow label={t("diffTarget")} value={target} onPick={() => pickFile(setTarget)} onChange={setTarget} disabled={busy} />
          <label className="checkbox">
            <input
              type="checkbox"
              checked={normalize}
              onChange={(e) => setNormalize(e.target.checked)}
              disabled={busy}
            />
            <span>{t("diffNormalize")}</span>
          </label>
          <button
            type="button"
            className="primary"
            onClick={runDiff}
            disabled={busy || !baseline || !target}
          >
            {busy ? <><span className="spinner" aria-hidden="true" />{t("analyzing")}</> : t("diffRun")}
          </button>
        </div>
      </section>

      {error && (
        <div className="alert error">
          <strong>{t("error")}:</strong> {error}
        </div>
      )}

      {!result && !error && !busy && <p className="muted">{t("diffEmpty")}</p>}

      {summary && (
        <section className="card">
          <h2>{t("diffSummary")}</h2>
          <div className="metric-grid">
            <Metric label={t("diffBaselineTotal")} value={(summary.baseline_total ?? 0).toLocaleString()} />
            <Metric label={t("diffTargetTotal")} value={(summary.target_total ?? 0).toLocaleString()} />
            <Metric label={t("diffCommonPaths")} value={(summary.common_paths ?? 0).toLocaleString()} />
            <Metric label={t("diffAddedPaths")} value={(summary.added_paths ?? 0).toLocaleString()} />
            <Metric label={t("diffRemovedPaths")} value={(summary.removed_paths ?? 0).toLocaleString()} />
          </div>
          {summary.max_increase && (
            <p className="diff-extreme regression">
              <strong>{t("diffMaxIncrease")}:</strong>{" "}
              <code>{summary.max_increase.stack}</code>{" "}
              ({(summary.max_increase.delta_ratio * 100).toFixed(1)}%)
            </p>
          )}
          {summary.max_decrease && (
            <p className="diff-extreme improvement">
              <strong>{t("diffMaxDecrease")}:</strong>{" "}
              <code>{summary.max_decrease.stack}</code>{" "}
              ({(summary.max_decrease.delta_ratio * 100).toFixed(1)}%)
            </p>
          )}
        </section>
      )}

      {flame && (
        <section className="card flush">
          <h2>{t("flamegraphTitle")}</h2>
          <CanvasFlameGraph data={flame} exportName="diff-flamegraph" />
        </section>
      )}

      {(increases.length > 0 || decreases.length > 0) && (
        <section className="card">
          <h2>{t("diffSummary")}</h2>
          <div className="diff-tables">
            {increases.length > 0 && (
              <div>
                <h3 className="regression">{t("diffMaxIncrease")}</h3>
                <DiffRowsTable rows={increases.slice(0, 10)} />
              </div>
            )}
            {decreases.length > 0 && (
              <div>
                <h3 className="improvement">{t("diffMaxDecrease")}</h3>
                <DiffRowsTable rows={decreases.slice(0, 10)} />
              </div>
            )}
          </div>
        </section>
      )}
    </main>
  );
}

function DiffFileRow({
  label,
  value,
  onPick,
  onChange,
  disabled,
}: {
  label: string;
  value: string;
  onPick: () => void;
  onChange: (next: string) => void;
  disabled: boolean;
}) {
  const { t } = useI18n();
  return (
    <div className="file-row">
      <input
        type="text"
        className="path-input"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={label}
        disabled={disabled}
      />
      <button type="button" className="primary" onClick={onPick} disabled={disabled}>
        {t("pickFile")}
      </button>
    </div>
  );
}

function DiffRowsTable({ rows }: { rows: Array<Record<string, any>> }) {
  return (
    <table className="data-table diff-table">
      <thead>
        <tr>
          <th>Stack</th>
          <th className="num">Δ</th>
          <th className="num">Δ%</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row, idx) => (
          <tr key={`${row.stack}-${idx}`}>
            <td className="frame-cell" title={String(row.stack)}>
              {row.stack}
            </td>
            <td className="num">{Number(row.delta).toFixed(4)}</td>
            <td className="num">{(Number(row.delta_ratio) * 100).toFixed(1)}%</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="metric">
      <div className="metric-label">{label}</div>
      <div className="metric-value">{value}</div>
    </div>
  );
}
