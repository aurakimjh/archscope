import { useEffect, useMemo, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";

import {
  ProfilerService,
  AnalyzeRequest,
  ExportPprofRequest,
} from "../bindings/github.com/aurakimjh/archscope/apps/profiler-native/cmd/archscope-profiler-app";
import type {
  AnalysisResult,
  FlameNode,
} from "../bindings/github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler/models";

import { useI18n } from "./i18n/I18nProvider";
import { localeLabels, locales } from "./i18n/messages";
import { useTheme, type Theme } from "./theme/ThemeProvider";
import { CanvasFlameGraph, type FlameGraphNode } from "./components/CanvasFlameGraph";
import { DrilldownPanel } from "./components/DrilldownPanel";
import { HorizontalBarChart, type BarRow } from "./components/HorizontalBarChart";
import { DiagnosticsPanel } from "./components/DiagnosticsPanel";
import { Tabs, type TabSpec } from "./components/Tabs";
import { DropZone } from "./components/DropZone";
import { Sidebar, type NavKey } from "./components/Sidebar";
import { useRecentFiles } from "./hooks/useRecentFiles";
import { useShortcuts } from "./hooks/useShortcuts";
import { loadDefaults } from "./state/defaults";
import { DiffPage } from "./pages/DiffPage";
import { SettingsPage } from "./pages/SettingsPage";
import { AccessLogAnalyzerPage } from "./pages/AccessLogAnalyzerPage";
import { GcLogAnalyzerPage } from "./pages/GcLogAnalyzerPage";

function adaptFlameNode(node: FlameNode | null | undefined): FlameGraphNode | null {
  if (!node) return null;
  const children = (node.children ?? [])
    .map(adaptFlameNode)
    .filter((child): child is FlameGraphNode => child !== null);
  return {
    name: node.name ?? "",
    value: node.samples ?? 0,
    category: node.category ?? null,
    color: node.color ?? null,
    children,
  };
}

const THEME_OPTIONS: Theme[] = ["light", "dark", "system"];

type TabKey = "summary" | "flamegraph" | "charts" | "drilldown" | "diagnostics";
type ProfileFormat =
  | "collapsed"
  | "jennifer"
  | "flamegraph_svg"
  | "flamegraph_html";

function detectFormat(filePath: string): ProfileFormat {
  const lower = filePath.toLowerCase();
  if (lower.endsWith(".csv")) return "jennifer";
  if (lower.endsWith(".svg")) return "flamegraph_svg";
  if (lower.endsWith(".html") || lower.endsWith(".htm")) return "flamegraph_html";
  return "collapsed";
}

function App() {
  const { locale, setLocale, t } = useI18n();
  const { theme, setTheme } = useTheme();
  const recent = useRecentFiles();

  const [active, setActive] = useState<NavKey>("profiler");

  const defaults = useMemo(loadDefaults, []);
  const [path, setPath] = useState<string>("");
  const [format, setFormat] = useState<ProfileFormat>("collapsed");
  const [intervalMs, setIntervalMs] = useState<number>(defaults.intervalMs);
  const [elapsedSec, setElapsedSec] = useState<string>("");
  const [topN, setTopN] = useState<number>(defaults.topN);
  const [profileKind, setProfileKind] = useState<"wall" | "cpu" | "lock">(defaults.profileKind);
  const [timelineBaseMethod, setTimelineBaseMethod] = useState<string>("");

  const [analyzing, setAnalyzing] = useState<boolean>(false);
  const [exporting, setExporting] = useState<boolean>(false);
  const [exportNotice, setExportNotice] = useState<string>("");
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [error, setError] = useState<string>("");
  const [optionsOpen, setOptionsOpen] = useState<boolean>(true);
  const [activeTab, setActiveTab] = useState<TabKey>("summary");
  const activeTaskRef = useRef<string | null>(null);

  useEffect(() => {
    const offDone = Events.On("analyze:done", (event: any) => {
      const data = event?.data ?? event;
      if (!data || data.taskId !== activeTaskRef.current) return;
      activeTaskRef.current = null;
      setResult(data.result);
      setActiveTab("summary");
      setOptionsOpen(false);
      setAnalyzing(false);
      if (path) recent.push(path);
    });
    const offError = Events.On("analyze:error", (event: any) => {
      const data = event?.data ?? event;
      if (!data || data.taskId !== activeTaskRef.current) return;
      activeTaskRef.current = null;
      setError(data.message ?? "analysis failed");
      setResult(null);
      setAnalyzing(false);
    });
    const offCancelled = Events.On("analyze:cancelled", (event: any) => {
      const data = event?.data ?? event;
      if (!data || data.taskId !== activeTaskRef.current) return;
      activeTaskRef.current = null;
      setAnalyzing(false);
    });
    return () => {
      offDone?.();
      offError?.();
      offCancelled?.();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path]);

  const handlePathChange = (newPath: string) => {
    setPath(newPath);
    setFormat(detectFormat(newPath));
  };

  const handlePickFile = async () => {
    try {
      const picked = await ProfilerService.PickProfileFile();
      if (picked) handlePathChange(picked);
    } catch (err: any) {
      setError(String(err?.message ?? err));
    }
  };

  const handleAnalyze = async () => {
    setError("");
    setExportNotice("");
    setAnalyzing(true);
    try {
      const elapsed = elapsedSec.trim() === "" ? -1 : Number(elapsedSec);
      const request = new AnalyzeRequest({
        path,
        format,
        intervalMs,
        elapsedSec: Number.isFinite(elapsed) ? elapsed : -1,
        topN,
        profileKind,
        timelineBaseMethod,
      });
      const response = await ProfilerService.AnalyzeAsync(request);
      activeTaskRef.current = response.taskId;
      // The result lands via the analyze:done / analyze:error events.
    } catch (err: any) {
      setError(String(err?.message ?? err));
      setResult(null);
      setAnalyzing(false);
    }
  };

  const tabKeys: TabKey[] = ["summary", "flamegraph", "charts", "drilldown", "diagnostics"];
  useShortcuts({
    onOpen: () => {
      if (active === "profiler" && !analyzing) handlePickFile();
    },
    onAnalyze: () => {
      if (active === "profiler" && !analyzing && path) handleAnalyze();
    },
    onCancel: () => {
      if (analyzing) handleCancel();
    },
    onSettings: () => setActive("settings"),
    onTab: (n) => {
      if (active !== "profiler" || !result) return;
      const key = tabKeys[n - 1];
      if (key) setActiveTab(key);
    },
  });

  const handleCancel = async () => {
    const taskId = activeTaskRef.current;
    if (!taskId) return;
    try {
      await ProfilerService.Cancel(taskId);
    } catch {
      // The cancel hook is best-effort; even if the server rejects we still
      // tear down the spinner because the user already asked to stop.
    } finally {
      activeTaskRef.current = null;
      setAnalyzing(false);
    }
  };

  const handleExportPprof = async () => {
    if (!path) return;
    setError("");
    setExportNotice("");
    setExporting(true);
    try {
      const req = new ExportPprofRequest({
        path,
        format,
        intervalMs,
        elapsedSec: -1,
        topN,
        profileKind,
        timelineBaseMethod,
        outputPath: "",
      } as any);
      const res = await ProfilerService.ExportPprof(req);
      if (res?.outputPath) {
        setExportNotice(`${t("exportPprofSaved")} ${res.outputPath}`);
      }
    } catch (err: any) {
      setError(String(err?.message ?? err));
    } finally {
      setExporting(false);
    }
  };

  // Derived view data
  const summary = result?.summary;
  const diagnostics = result?.metadata?.diagnostics ?? null;
  const timelineScope = result?.metadata?.timeline_scope;
  const topStacks = (result?.series?.top_stacks ?? []).slice(0, 15);
  const topChildFrames = result?.tables?.top_child_frames ?? [];
  const flamegraph = useMemo(() => adaptFlameNode(result?.charts?.flamegraph), [result]);
  const exportName = useMemo(() => {
    if (!path) return "archscope-flamegraph";
    const segments = path.split(/[\\/]/);
    const last = segments[segments.length - 1] || "archscope-flamegraph";
    return last.replace(/\.[^.]+$/, "") + "-flamegraph";
  }, [path]);

  const drilldownBase = result
    ? new AnalyzeRequest({
        path,
        format,
        intervalMs,
        elapsedSec: elapsedSec.trim() === "" ? -1 : Number(elapsedSec) || -1,
        topN,
        profileKind,
        timelineBaseMethod,
      })
    : null;

  const breakdownRows: BarRow[] = (result?.series?.execution_breakdown ?? [])
    .filter((row) => (row?.samples ?? 0) > 0)
    .map((row) => ({
      key: row.category,
      label: row.executive_label || row.category || "(unknown)",
      value: row.samples,
      ratio: typeof row.total_ratio === "number" ? row.total_ratio : undefined,
    }));

  const timelineRows: BarRow[] = (result?.series?.timeline_analysis ?? [])
    .filter((row) => (row?.samples ?? 0) > 0)
    .map((row) => ({
      key: `${row.index}-${row.segment}`,
      label: row.label || row.segment || "(unknown)",
      value: row.samples,
      ratio: typeof row.total_ratio === "number" ? row.total_ratio : undefined,
    }));

  const errorBadge = (diagnostics?.error_count ?? 0) + (diagnostics?.warning_count ?? 0);
  const tabs: TabSpec<TabKey>[] = [
    { key: "summary", label: t("tabSummary"), disabled: !result },
    { key: "flamegraph", label: t("tabFlamegraph"), disabled: !flamegraph },
    { key: "charts", label: t("tabCharts"), disabled: !result },
    { key: "drilldown", label: t("tabDrilldown"), disabled: !drilldownBase },
    {
      key: "diagnostics",
      label: t("tabDiagnostics"),
      disabled: !result,
      badge: errorBadge > 0 ? errorBadge : undefined,
    },
  ];

  useEffect(() => {
    if (!result) setActiveTab("summary");
  }, [result]);

  return (
    <DropZone onDrop={handlePathChange}>
      <div className="app-shell with-sidebar">
        <Sidebar active={active} onNavigate={setActive} />
        <div className="app-main">
          <header className="topbar">
            <div className="brand">
              <span className="eyebrow">{t("eyebrow")}</span>
              <h1>{t("appTitle")}</h1>
            </div>
            <div className="topbar-controls">
              <div className="locale-switch" role="group" aria-label={t("themeLabel")}>
                {THEME_OPTIONS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    className={value === theme ? "locale-pill active" : "locale-pill"}
                    onClick={() => setTheme(value)}
                    aria-pressed={value === theme}
                  >
                    {value === "light"
                      ? t("themeLight")
                      : value === "dark"
                      ? t("themeDark")
                      : t("themeSystem")}
                  </button>
                ))}
              </div>
              <div className="locale-switch" role="group" aria-label="Locale">
                {locales.map((value) => (
                  <button
                    key={value}
                    type="button"
                    className={value === locale ? "locale-pill active" : "locale-pill"}
                    onClick={() => setLocale(value)}
                    aria-pressed={value === locale}
                    title={localeLabels[value]}
                  >
                    {value.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
          </header>

          {active === "profiler" && (
            <>
              <section className={optionsOpen ? "file-strip" : "file-strip collapsed"}>
                <div className="file-row">
                  <input
                    type="text"
                    className="path-input"
                    value={path}
                    onChange={(e) => handlePathChange(e.target.value)}
                    placeholder={t("filePathPlaceholder")}
                    disabled={analyzing}
                  />
                  <button
                    type="button"
                    className="primary"
                    onClick={handlePickFile}
                    disabled={analyzing}
                  >
                    {t("pickFile")}
                  </button>
                  {recent.entries.length > 0 && (
                    <details className="recent-dropdown">
                      <summary>{t("recentFiles")}</summary>
                      <ul>
                        {recent.entries.map((entry) => (
                          <li key={entry}>
                            <button
                              type="button"
                              onClick={() => handlePathChange(entry)}
                              title={entry}
                            >
                              {entry.split(/[\\/]/).pop() ?? entry}
                            </button>
                          </li>
                        ))}
                        <li>
                          <button type="button" className="ghost" onClick={recent.clear}>
                            {t("clearRecent")}
                          </button>
                        </li>
                      </ul>
                    </details>
                  )}
                  {result && !optionsOpen && (
                    <button
                      type="button"
                      className="ghost"
                      onClick={() => setOptionsOpen(true)}
                    >
                      {t("editOptions")}
                    </button>
                  )}
                </div>
                {optionsOpen && (
                  <div className="option-row">
                    <label className="field">
                      <span>{t("format")}</span>
                      <select
                        value={format}
                        onChange={(e) => setFormat(e.target.value as ProfileFormat)}
                        disabled={analyzing}
                      >
                        <option value="collapsed">{t("formatCollapsed")}</option>
                        <option value="jennifer">{t("formatJennifer")}</option>
                        <option value="flamegraph_svg">{t("formatFlamegraphSvg")}</option>
                        <option value="flamegraph_html">{t("formatFlamegraphHtml")}</option>
                      </select>
                    </label>
                    <label className="field">
                      <span>{t("intervalMs")}</span>
                      <input
                        type="number"
                        min={1}
                        value={intervalMs}
                        onChange={(e) => setIntervalMs(Number(e.target.value) || 100)}
                        disabled={analyzing}
                      />
                    </label>
                    <label className="field">
                      <span>{t("elapsedSec")}</span>
                      <input
                        type="text"
                        value={elapsedSec}
                        onChange={(e) => setElapsedSec(e.target.value)}
                        placeholder="—"
                        disabled={analyzing}
                      />
                    </label>
                    <label className="field">
                      <span>{t("topN")}</span>
                      <input
                        type="number"
                        min={1}
                        max={100}
                        value={topN}
                        onChange={(e) => setTopN(Number(e.target.value) || 20)}
                        disabled={analyzing}
                      />
                    </label>
                    <label className="field">
                      <span>{t("profileKind")}</span>
                      <select
                        value={profileKind}
                        onChange={(e) =>
                          setProfileKind(e.target.value as "wall" | "cpu" | "lock")
                        }
                        disabled={analyzing}
                      >
                        <option value="wall">wall</option>
                        <option value="cpu">cpu</option>
                        <option value="lock">lock</option>
                      </select>
                    </label>
                    <label className="field grow">
                      <span>{t("timelineBaseMethod")}</span>
                      <input
                        type="text"
                        value={timelineBaseMethod}
                        onChange={(e) => setTimelineBaseMethod(e.target.value)}
                        placeholder="—"
                        disabled={analyzing}
                      />
                    </label>
                    <button
                      type="button"
                      className="primary"
                      onClick={handleAnalyze}
                      disabled={analyzing || !path}
                    >
                      {analyzing ? (
                        <>
                          <span className="spinner" aria-hidden="true" />
                          {t("analyzing")}
                        </>
                      ) : (
                        t("analyze")
                      )}
                    </button>
                    {result && optionsOpen && (
                      <button
                        type="button"
                        className="ghost"
                        onClick={() => setOptionsOpen(false)}
                        disabled={analyzing}
                      >
                        {t("hideOptions")}
                      </button>
                    )}
                  </div>
                )}
              </section>

              {result && (
                <Tabs
                  tabs={tabs}
                  active={activeTab}
                  onChange={(key) => setActiveTab(key)}
                  rightSlot={
                    <button
                      type="button"
                      className="ghost"
                      onClick={handleExportPprof}
                      disabled={exporting || !path}
                    >
                      {exporting ? (
                        <>
                          <span className="spinner" aria-hidden="true" />
                          {t("exportPprofBusy")}
                        </>
                      ) : (
                        t("exportPprof")
                      )}
                    </button>
                  }
                />
              )}

              <main className="content">
                {error && (
                  <div className="alert error">
                    <strong>{t("error")}:</strong> {error}
                  </div>
                )}
                {exportNotice && (
                  <div className="alert info">
                    <strong>✓</strong> {exportNotice}
                  </div>
                )}
                {!result && !error && !analyzing && (
                  <p className="muted">{t("nothingYet")}</p>
                )}
                {analyzing && (
                  <div className="loading-overlay">
                    <span className="spinner large" aria-hidden="true" />
                    <p>{t("analyzing")}</p>
                    <button type="button" className="ghost" onClick={handleCancel}>
                      {t("cancel")}
                    </button>
                  </div>
                )}

                {result && activeTab === "summary" && summary && (
                  <>
                    <section className="card">
                      <h2>{t("summary")}</h2>
                      <div className="metric-grid">
                        <Metric
                          label={t("totalSamples")}
                          value={summary.total_samples.toLocaleString()}
                        />
                        <Metric
                          label={t("estimatedSeconds")}
                          value={summary.estimated_seconds.toFixed(3)}
                        />
                        <Metric
                          label={t("intervalMsLabel")}
                          value={summary.interval_ms.toString()}
                        />
                        <Metric
                          label={t("elapsedLabel")}
                          value={
                            summary.elapsed_seconds == null
                              ? "—"
                              : summary.elapsed_seconds.toFixed(3)
                          }
                        />
                        <Metric
                          label={t("profileKindLabel")}
                          value={summary.profile_kind}
                        />
                        <Metric label={t("parser")} value={result?.metadata?.parser ?? ""} />
                      </div>
                    </section>

                    {timelineScope && (
                      <section className="card">
                        <h2>{t("timelineScope")}</h2>
                        <div className="metric-grid">
                          <Metric
                            label={t("timelineScopeMode")}
                            value={timelineScope.mode}
                          />
                          <Metric
                            label={t("timelineScopeBaseMethod")}
                            value={timelineScope.base_method ?? "—"}
                          />
                          <Metric
                            label={t("timelineScopeMatch")}
                            value={timelineScope.match_mode}
                          />
                          <Metric
                            label={t("timelineScopeView")}
                            value={timelineScope.view_mode}
                          />
                          <Metric
                            label={t("timelineScopeBaseSamples")}
                            value={timelineScope.base_samples.toLocaleString()}
                          />
                          <Metric
                            label={t("timelineScopeBaseRatio")}
                            value={
                              timelineScope.base_ratio_of_total == null
                                ? "—"
                                : `${timelineScope.base_ratio_of_total.toFixed(2)}%`
                            }
                          />
                        </div>
                        {timelineScope.warnings && timelineScope.warnings.length > 0 && (
                          <ul className="scope-warnings">
                            {timelineScope.warnings.map((w, idx) => (
                              <li key={`${w.code}-${idx}`}>
                                <code>{w.code}</code> — {w.message}
                              </li>
                            ))}
                          </ul>
                        )}
                      </section>
                    )}

                    {topChildFrames.length > 0 && (
                      <section className="card">
                        <h2>{t("topChildFrames")}</h2>
                        <table className="data-table">
                          <thead>
                            <tr>
                              <th>{t("framesColumn")}</th>
                              <th className="num">{t("samplesColumn")}</th>
                              <th className="num">{t("ratioColumn")}</th>
                            </tr>
                          </thead>
                          <tbody>
                            {topChildFrames.map((row, idx) => (
                              <tr key={`${row.frame}-${idx}`}>
                                <td className="frame-cell" title={row.frame}>
                                  {row.frame}
                                </td>
                                <td className="num">{row.samples.toLocaleString()}</td>
                                <td className="num">{(row.ratio * 100).toFixed(2)}%</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </section>
                    )}

                    {topStacks.length > 0 && (
                      <section className="card">
                        <h2>{t("topStacks")}</h2>
                        <ol className="top-stacks scrollable">
                          {topStacks.map((row, idx) => (
                            <li key={`${row.stack}-${idx}`}>
                              <div className="stack-name" title={row.stack}>
                                {row.stack}
                              </div>
                              <div className="stack-meta">
                                {row.samples.toLocaleString()} {t("samples")} ·{" "}
                                {row.sample_ratio.toFixed(2)}%
                              </div>
                            </li>
                          ))}
                        </ol>
                      </section>
                    )}
                  </>
                )}

                {result && activeTab === "flamegraph" && flamegraph && (
                  <section className="card flush">
                    <h2>{t("flamegraphTitle")}</h2>
                    <CanvasFlameGraph data={flamegraph} exportName={exportName} />
                  </section>
                )}

                {result && activeTab === "charts" && (
                  <>
                    <section className="card">
                      <h2>{t("breakdownTitle")}</h2>
                      <HorizontalBarChart
                        rows={breakdownRows}
                        emptyLabel={t("breakdownEmpty")}
                        valueSuffix={t("samples")}
                        ratioFractionDigits={2}
                      />
                    </section>
                    <section className="card">
                      <h2>{t("timelineTitle")}</h2>
                      <HorizontalBarChart
                        rows={timelineRows}
                        emptyLabel={t("timelineEmpty")}
                        valueSuffix={t("samples")}
                        ratioFractionDigits={2}
                      />
                    </section>
                  </>
                )}

                {result && activeTab === "drilldown" && drilldownBase && (
                  <DrilldownPanel baseRequest={drilldownBase} onError={setError} />
                )}

                {result && activeTab === "diagnostics" && (
                  <section className="card">
                    <h2>{t("diagnosticsTitle")}</h2>
                    <DiagnosticsPanel
                      diagnostics={diagnostics}
                      baseRequest={drilldownBase}
                    />
                  </section>
                )}
              </main>
            </>
          )}

          {active === "diff" && <DiffPage />}
          {active === "access_log" && <AccessLogAnalyzerPage />}
          {active === "gc_log" && <GcLogAnalyzerPage />}
          {active === "settings" && <SettingsPage />}
        </div>
      </div>
    </DropZone>
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

export default App;
