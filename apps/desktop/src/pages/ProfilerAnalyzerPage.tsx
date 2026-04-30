import { useState } from "react";
import type { EChartsOption } from "echarts";

import {
  getAnalyzerClient,
  type BridgeError,
  type ExecutionBreakdownRow,
  type FlameNode,
  type ProfilerCollapsedAnalysisResult,
  type ProfilerTopStackTableRow,
} from "../api/analyzerClient";
import { ChartPanel } from "../components/ChartPanel";
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
type FilterType = "include_text" | "exclude_text" | "regex_include" | "regex_exclude";
type MatchMode = "anywhere" | "ordered" | "subtree";
type ViewMode = "preserve_full_path" | "reroot_at_match";

type TopStackRow = {
  stack: string;
  samples: number;
  estimated_seconds: number;
  sample_ratio: number;
};

type DrilldownStageView = {
  label: string;
  breadcrumb: string[];
  flamegraph: FlameNode;
  metrics: {
    matched_samples: number;
    estimated_seconds: number;
    total_ratio: number;
    parent_stage_ratio: number;
    elapsed_ratio: number | null;
  };
  breakdown: ExecutionBreakdownRow[];
};

export function ProfilerAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [wallPath, setWallPath] = useState("");
  const [wallIntervalMs, setWallIntervalMs] = useState(100);
  const [elapsedSec, setElapsedSec] = useState("");
  const [topN, setTopN] = useState(20);
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<ProfilerCollapsedAnalysisResult | null>(null);
  const [stages, setStages] = useState<DrilldownStageView[]>([]);
  const [filterPattern, setFilterPattern] = useState("");
  const [filterType, setFilterType] = useState<FilterType>("include_text");
  const [matchMode, setMatchMode] = useState<MatchMode>("anywhere");
  const [viewMode, setViewMode] = useState<ViewMode>("preserve_full_path");
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);

  const canAnalyze = Boolean(wallPath) && wallIntervalMs > 0 && state !== "running";
  const summary = result?.summary;
  const topStacks = getTopStackRows(result?.tables.top_stacks);
  const currentStage = stages[stages.length - 1];
  const breakdownRows = currentStage?.breakdown ?? result?.series.execution_breakdown ?? [];

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
      setResult(null);
      setStages([]);
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
    setResult(null);
    setStages([]);
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
        setStages(createInitialStages(response.result));
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

  function applyFilter(): void {
    const pattern = filterPattern.trim();
    if (!pattern || !currentStage) {
      return;
    }
    const next = applyStageFilter(currentStage, {
      pattern,
      filterType,
      matchMode,
      viewMode,
      intervalMs: wallIntervalMs,
      elapsedSec: parseOptionalPositiveNumber(elapsedSec) ?? undefined,
    });
    setStages([...stages, next]);
    setFilterPattern("");
  }

  function resetDrilldown(): void {
    if (result) {
      setStages(createInitialStages(result));
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
          <section className="drilldown-controls">
            <h2>{t("flamegraphDrilldown")}</h2>
            <label className="field">
              <span>{t("drilldownFilter")}</span>
              <input
                type="text"
                value={filterPattern}
                placeholder="oracle.jdbc"
                onChange={(event) => setFilterPattern(event.target.value)}
              />
            </label>
            <label className="field">
              <span>{t("filterType")}</span>
              <select
                value={filterType}
                onChange={(event) => setFilterType(event.target.value as FilterType)}
              >
                <option value="include_text">include text</option>
                <option value="exclude_text">exclude text</option>
                <option value="regex_include">regex include</option>
                <option value="regex_exclude">regex exclude</option>
              </select>
            </label>
            <label className="field">
              <span>{t("matchMode")}</span>
              <select
                value={matchMode}
                onChange={(event) => setMatchMode(event.target.value as MatchMode)}
              >
                <option value="anywhere">anywhere</option>
                <option value="ordered">ordered</option>
                <option value="subtree">subtree</option>
              </select>
            </label>
            <label className="field">
              <span>{t("viewMode")}</span>
              <select
                value={viewMode}
                onChange={(event) => setViewMode(event.target.value as ViewMode)}
              >
                <option value="preserve_full_path">preserve full path</option>
                <option value="reroot_at_match">re-root at matched frame</option>
              </select>
            </label>
            <div className="button-row">
              <button
                className="secondary-button"
                type="button"
                disabled={!currentStage || !filterPattern.trim()}
                onClick={applyFilter}
              >
                {t("applyFilter")}
              </button>
              <button
                className="secondary-button"
                type="button"
                disabled={stages.length <= 1}
                onClick={resetDrilldown}
              >
                {t("resetDrilldown")}
              </button>
            </div>
          </section>
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
          {currentStage && (
            <>
              <section className="table-panel drilldown-panel">
                <div className="panel-header">
                  <h2>{t("stageMetrics")}</h2>
                </div>
                <div className="breadcrumb">
                  {currentStage.breadcrumb.map((item, index) => (
                    <span key={`${item}-${index}`}>{item}</span>
                  ))}
                </div>
                <section className="summary-grid compact">
                  <MetricCard
                    label={t("matchedSamples")}
                    value={formatNumber(currentStage.metrics.matched_samples)}
                  />
                  <MetricCard
                    label={t("estimatedSeconds")}
                    value={formatSeconds(currentStage.metrics.estimated_seconds)}
                  />
                  <MetricCard
                    label={t("totalRatio")}
                    value={formatPercent(currentStage.metrics.total_ratio)}
                  />
                  <MetricCard
                    label={t("parentStageRatio")}
                    value={formatPercent(currentStage.metrics.parent_stage_ratio)}
                  />
                </section>
                <FlameTreeView root={currentStage.flamegraph} />
              </section>
              <section className="chart-grid">
                <ChartPanel
                  title={t("breakdownDonut")}
                  option={breakdownDonutOption(breakdownRows)}
                />
                <ChartPanel
                  title={t("breakdownBars")}
                  option={breakdownBarOption(breakdownRows)}
                />
              </section>
              <BreakdownTable rows={breakdownRows} labels={{
                title: t("categoryTopStacks"),
                category: t("category"),
                samples: t("samples"),
                ratio: t("ratio"),
                waitReason: t("waitReason"),
                topMethods: t("topMethods"),
              }} />
            </>
          )}
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

function createInitialStages(
  result: ProfilerCollapsedAnalysisResult,
): DrilldownStageView[] {
  const flamegraph = result.charts.flamegraph;
  if (!flamegraph) {
    return [];
  }
  return [
    {
      label: "All",
      breadcrumb: ["All"],
      flamegraph,
      metrics: {
        matched_samples: flamegraph.samples,
        estimated_seconds: result.summary.estimated_seconds,
        total_ratio: 100,
        parent_stage_ratio: 100,
        elapsed_ratio: result.summary.elapsed_seconds
          ? result.summary.estimated_seconds / result.summary.elapsed_seconds * 100
          : null,
      },
      breakdown: result.series.execution_breakdown ?? [],
    },
  ];
}

function applyStageFilter(
  stage: DrilldownStageView,
  options: {
    pattern: string;
    filterType: FilterType;
    matchMode: MatchMode;
    viewMode: ViewMode;
    intervalMs: number;
    elapsedSec?: number;
  },
): DrilldownStageView {
  const leaves = extractLeaves(stage.flamegraph);
  const filtered = leaves.flatMap((leaf) => {
    const matchIndex = matchedIndex(leaf.path, options.pattern, options.filterType, options.matchMode);
    const matched = matchIndex >= 0;
    const include = options.filterType.includes("exclude") ? !matched : matched;
    if (!include) {
      return [];
    }
    const path =
      options.viewMode === "reroot_at_match" &&
      matchIndex >= 0 &&
      !options.filterType.includes("exclude")
        ? leaf.path.slice(matchIndex)
        : leaf.path;
    return [{ path, samples: leaf.samples }];
  });
  const flamegraph = buildTree(filtered, options.pattern);
  const estimatedSeconds = flamegraph.samples * (options.intervalMs / 1000);
  return {
    label: options.pattern,
    breadcrumb: [...stage.breadcrumb, options.pattern],
    flamegraph,
    metrics: {
      matched_samples: flamegraph.samples,
      estimated_seconds: estimatedSeconds,
      total_ratio: stage.breadcrumb.length === 1
        ? flamegraph.ratio
        : flamegraph.samples / Math.max(rootSamples(stage), 1) * 100,
      parent_stage_ratio: flamegraph.samples / Math.max(stage.flamegraph.samples, 1) * 100,
      elapsed_ratio: options.elapsedSec ? estimatedSeconds / options.elapsedSec * 100 : null,
    },
    breakdown: buildBreakdown(flamegraph, options.intervalMs, options.elapsedSec),
  };
}

function FlameTreeView({ root }: { root: FlameNode }): JSX.Element {
  return (
    <div className="flame-tree">
      {root.children.slice(0, 16).map((child) => (
        <FlameNodeBar key={child.id} node={child} depth={0} total={Math.max(root.samples, 1)} />
      ))}
    </div>
  );
}

function FlameNodeBar({
  node,
  depth,
  total,
}: {
  node: FlameNode;
  depth: number;
  total: number;
}): JSX.Element {
  return (
    <div className="flame-node-row" style={{ paddingLeft: depth * 14 }}>
      <div className="flame-node-label">
        <span>{node.name}</span>
        <small>{formatNumber(node.samples)} / {formatPercent(node.samples / total * 100)}</small>
      </div>
      <div className="flame-node-track">
        <div
          className="flame-node-fill"
          style={{ width: `${Math.max(node.samples / total * 100, 1)}%` }}
        />
      </div>
      {node.children.slice(0, 8).map((child) => (
        <FlameNodeBar key={child.id} node={child} depth={depth + 1} total={total} />
      ))}
    </div>
  );
}

function BreakdownTable({
  rows,
  labels,
}: {
  rows: ExecutionBreakdownRow[];
  labels: {
    title: string;
    category: string;
    samples: string;
    ratio: string;
    waitReason: string;
    topMethods: string;
  };
}): JSX.Element {
  return (
    <section className="table-panel">
      <div className="panel-header">
        <h2>{labels.title}</h2>
      </div>
      <table>
        <thead>
          <tr>
            <th>{labels.category}</th>
            <th>{labels.samples}</th>
            <th>{labels.ratio}</th>
            <th>{labels.waitReason}</th>
            <th>{labels.topMethods}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.category}>
              <td>{row.executive_label}</td>
              <td>{formatNumber(row.samples)}</td>
              <td>{formatPercent(row.total_ratio)}</td>
              <td>{row.wait_reason ?? "-"}</td>
              <td>{row.top_methods.map((item) => item.name).join(", ") || "-"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function breakdownDonutOption(rows: ExecutionBreakdownRow[]): EChartsOption {
  return {
    tooltip: { trigger: "item" },
    legend: { bottom: 0 },
    series: [
      {
        type: "pie",
        radius: ["48%", "72%"],
        data: rows.map((row) => ({ name: row.executive_label, value: row.samples })),
      },
    ],
  };
}

function breakdownBarOption(rows: ExecutionBreakdownRow[]): EChartsOption {
  return {
    tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
    grid: { left: 144, right: 24, top: 24, bottom: 36 },
    xAxis: { type: "value" },
    yAxis: { type: "category", data: rows.map((row) => row.executive_label) },
    series: [{ type: "bar", data: rows.map((row) => row.samples) }],
  };
}

function extractLeaves(root: FlameNode): Array<{ path: string[]; samples: number }> {
  if (root.children.length === 0 && root.path.length > 0) {
    return [{ path: root.path, samples: root.samples }];
  }
  return root.children.flatMap(extractLeaves);
}

function buildTree(leaves: Array<{ path: string[]; samples: number }>, label: string): FlameNode {
  const root: FlameNode = {
    id: `stage:${label}`,
    parentId: null,
    name: label,
    samples: leaves.reduce((sum, leaf) => sum + leaf.samples, 0),
    ratio: 100,
    category: null,
    color: null,
    children: [],
    path: [],
  };
  for (const leaf of leaves) {
    let current = root;
    leaf.path.forEach((frame, index) => {
      let child = current.children.find((item) => item.name === frame);
      if (!child) {
        child = {
          id: `${current.id}/${index}-${frame}`,
          parentId: current.id,
          name: frame,
          samples: 0,
          ratio: 0,
          category: null,
          color: null,
          children: [],
          path: leaf.path.slice(0, index + 1),
        };
        current.children.push(child);
      }
      child.samples += leaf.samples;
      current = child;
    });
  }
  assignRatios(root, Math.max(root.samples, 1));
  return root;
}

function assignRatios(node: FlameNode, total: number): void {
  node.ratio = node.samples / total * 100;
  node.children.sort((a, b) => b.samples - a.samples);
  node.children.forEach((child) => assignRatios(child, total));
}

function matchedIndex(
  path: string[],
  pattern: string,
  filterType: FilterType,
  matchMode: MatchMode,
): number {
  if (matchMode === "ordered") {
    const terms = pattern.split(/[>;]/).map((term) => term.trim()).filter(Boolean);
    let termIndex = 0;
    let startIndex = -1;
    for (let index = 0; index < path.length; index += 1) {
      if (frameMatches(path[index], terms[termIndex], filterType)) {
        if (startIndex < 0) {
          startIndex = index;
        }
        termIndex += 1;
        if (termIndex === terms.length) {
          return startIndex;
        }
      }
    }
    return -1;
  }
  return path.findIndex((frame) => frameMatches(frame, pattern, filterType));
}

function frameMatches(frame: string, pattern: string, filterType: FilterType): boolean {
  if (filterType.startsWith("regex")) {
    try {
      return new RegExp(pattern).test(frame);
    } catch {
      return false;
    }
  }
  return frame.toLowerCase().includes(pattern.toLowerCase());
}

function buildBreakdown(
  root: FlameNode,
  intervalMs: number,
  elapsedSec?: number,
): ExecutionBreakdownRow[] {
  const categories = new Map<string, ExecutionBreakdownRow>();
  for (const leaf of extractLeaves(root)) {
    const classification = classifyFrames(leaf.path);
    const existing = categories.get(classification.category) ?? {
      category: classification.category,
      executive_label: classification.label,
      primary_category: classification.category,
      wait_reason: classification.waitReason,
      samples: 0,
      estimated_seconds: 0,
      total_ratio: 0,
      parent_stage_ratio: 0,
      elapsed_ratio: null,
      top_methods: [],
      top_stacks: [],
    };
    existing.samples += leaf.samples;
    existing.estimated_seconds = existing.samples * (intervalMs / 1000);
    existing.total_ratio = existing.samples / Math.max(root.samples, 1) * 100;
    existing.parent_stage_ratio = existing.total_ratio;
    existing.elapsed_ratio = elapsedSec ? existing.estimated_seconds / elapsedSec * 100 : null;
    existing.top_methods = [{ name: leaf.path[leaf.path.length - 1] ?? "-", samples: leaf.samples }];
    existing.top_stacks = [{ name: leaf.path.join(";"), samples: leaf.samples }];
    categories.set(classification.category, existing);
  }
  return [...categories.values()].sort((a, b) => b.samples - a.samples);
}

function classifyFrames(path: string[]): { category: string; label: string; waitReason: string | null } {
  const stack = path.join(";").toLowerCase();
  const hasHttp = /resttemplate|webclient|httpclient|okhttp|urlconnection|axios|fetch|requests\.|urllib|net\/http/.test(stack);
  const hasNetwork = /socketinputstream\.socketread|niosocketimpl|socketchannelimpl\.read|socketdispatcher\.read|epollwait|poll|recv|read0/.test(stack);
  if (/hikaripool\.getconnection|concurrentbag|synchronousqueue|datasource\.getconnection/.test(stack)) {
    return { category: "CONNECTION_POOL_WAIT", label: "Lock / Pool Wait", waitReason: null };
  }
  if (/oracle\.jdbc|java\.sql|javax\.sql|executequery|executeupdate|resultset|com\.mysql|org\.postgresql/.test(stack)) {
    return { category: "SQL_DATABASE", label: "SQL / DB", waitReason: hasNetwork ? "NETWORK_IO_WAIT" : null };
  }
  if (hasHttp) {
    return { category: "EXTERNAL_API_HTTP", label: "External Call", waitReason: hasNetwork ? "NETWORK_IO_WAIT" : null };
  }
  if (hasNetwork) {
    return { category: "NETWORK_IO_WAIT", label: "Network / I/O Wait", waitReason: null };
  }
  return { category: "UNKNOWN", label: "Others", waitReason: null };
}

function rootSamples(stage: DrilldownStageView): number {
  return stage.metrics.total_ratio > 0
    ? stage.flamegraph.samples / (stage.metrics.total_ratio / 100)
    : stage.flamegraph.samples;
}
