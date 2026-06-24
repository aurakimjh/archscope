// ─────────────────────────────────────────────────────────────────────
// [한글] DrilldownPanel.tsx — 프로파일러 드릴다운(필터 누적) 패널.
//
// 책임/목적:
//   - 사용자가 입력한 프레임 패턴(텍스트/정규식 + include/exclude)을
//     필터 chip 으로 누적 적용 → ProfilerService.Drilldown 호출 →
//     stage 별 flamegraph + breadcrumb + 매칭 통계를 받아 화면에 표시.
//   - 필터 추가/제거 시마다 누적 필터 배열로 재호출(중간 stage 캐시 X)
//     → Go 측이 매번 전체 파이프라인을 재실행. 단순하지만 idempotent.
//
// 데이터 흐름 (Go ProfilerService ↔ React):
//   사용자 → handleAdd → DrilldownRequest({...baseRequest, filters})
//   → Wails IPC → ProfilerService.Drilldown(request) → DrilldownStage[]
//   → setStages → 마지막 stage 의 flamegraph 를 CanvasFlameGraph 로 렌더.
//
// 주요 모드:
//   - filterType : include_text / exclude_text / regex_include / regex_exclude
//   - matchMode  : anywhere / ordered / subtree
//   - viewMode   : preserve_full_path / reroot_at_match (매칭 위치를
//                   루트로 재설정할지 전체 호출 경로 보존할지 선택)
//
// 의존성 주의: Wails 자동 생성 bindings 의 DrilldownFilter 클래스를
// `new DrilldownFilter({...})` 형태로 인스턴스화 해야 직렬화 시
// Go 측 struct 와 매칭됩니다 (단순 object literal 은 unmarshal 실패 가능).
// ─────────────────────────────────────────────────────────────────────
import { useMemo, useState } from "react";

import { useI18n } from "../i18n/I18nProvider";
import {
  ProfilerService,
  AnalyzeRequest,
  DrilldownRequest,
} from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app";
import {
  DrilldownFilter,
  type DrilldownStage,
} from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/internal/profiler/models";

import { CanvasFlameGraph, type FlameGraphNode } from "./CanvasFlameGraph";
import { HelpTip, HelpedLabel, HelpedTitle } from "./HelpTip";
import { HorizontalBarChart, type BarRow } from "./HorizontalBarChart";
import { getGenericMetricHelpText, getHelpText } from "@/help/helpCatalog";

type FilterType = "include_text" | "exclude_text" | "regex_include" | "regex_exclude";
type MatchMode = "anywhere" | "ordered" | "subtree";
type ViewMode = "preserve_full_path" | "reroot_at_match";
type DrilldownTimelineMode = "developer" | "expert";
type CompositionGroupId = "database" | "external" | "internal";

type DeveloperTimelineRow = {
  key: string;
  label: string;
  group: CompositionGroupId;
  order: number;
  samples: number;
  estimatedSeconds: number;
  color: string;
  rawSegments: string[];
};

type DeveloperBucketSpec = {
  key: string;
  labelKey: any;
  group: CompositionGroupId;
  order: number;
  color: string;
};

type DraftFilter = {
  pattern: string;
  filterType: FilterType;
  matchMode: MatchMode;
  viewMode: ViewMode;
  caseSensitive: boolean;
};

const DEFAULT_DRAFT: DraftFilter = {
  pattern: "",
  filterType: "include_text",
  matchMode: "anywhere",
  viewMode: "preserve_full_path",
  caseSensitive: false,
};

const DEVELOPER_BUCKETS: Record<string, DeveloperBucketSpec> = {
  SQL_TIME: {
    key: "SQL_TIME",
    labelKey: "drilldownDevSqlTime",
    group: "database",
    order: 10,
    color: "#2563eb",
  },
  DB_FETCH: {
    key: "DB_FETCH",
    labelKey: "drilldownDevDbFetchTime",
    group: "database",
    order: 11,
    color: "#38bdf8",
  },
  CONNECTION_POOL_WAIT: {
    key: "CONNECTION_POOL_WAIT",
    labelKey: "drilldownDevConnectionPoolTime",
    group: "database",
    order: 12,
    color: "#ef4444",
  },
  EXTERNAL_TIME: {
    key: "EXTERNAL_TIME",
    labelKey: "drilldownDevExternalTime",
    group: "external",
    order: 20,
    color: "#10b981",
  },
  EXTERNAL_PREP: {
    key: "EXTERNAL_PREP",
    labelKey: "drilldownDevExternalPrepTime",
    group: "external",
    order: 21,
    color: "#a78bfa",
  },
  LOGGING: {
    key: "LOGGING",
    labelKey: "drilldownDevLoggingTime",
    group: "internal",
    order: 30,
    color: "#db2777",
  },
  DTO_MAPPING: {
    key: "DTO_MAPPING",
    labelKey: "drilldownDevDtoTime",
    group: "internal",
    order: 40,
    color: "#0d9488",
  },
  FRAMEWORK_TIME: {
    key: "FRAMEWORK_TIME",
    labelKey: "drilldownDevFrameworkTime",
    group: "internal",
    order: 50,
    color: "#7c3aed",
  },
  INTERNAL_METHOD: {
    key: "INTERNAL_METHOD",
    labelKey: "drilldownDevInternalTime",
    group: "internal",
    order: 60,
    color: "#f97316",
  },
  RUNTIME_OTHER: {
    key: "RUNTIME_OTHER",
    labelKey: "drilldownDevRuntimeTime",
    group: "internal",
    order: 70,
    color: "#64748b",
  },
};

const COMPOSITION_GROUP_ORDER: CompositionGroupId[] = ["database", "external", "internal"];

const COMPOSITION_GROUP_COLORS: Record<CompositionGroupId, string> = {
  database: "#2563eb",
  external: "#059669",
  internal: "#ea580c",
};

function adaptStageFlamegraph(stage: DrilldownStage | undefined): FlameGraphNode | null {
  if (!stage) return null;
  const root = stage.flamegraph;
  function walk(node: any): FlameGraphNode {
    return {
      name: node.name ?? "",
      value: node.samples ?? 0,
      category: node.category ?? null,
      color: node.color ?? null,
      children: (node.children ?? []).map(walk),
    };
  }
  return walk(root);
}

function timelineDisplayRatio(row: any): number | undefined {
  if (typeof row?.stage_ratio === "number") return row.stage_ratio;
  if (typeof row?.total_ratio === "number") return row.total_ratio;
  return undefined;
}

function formatTimelineSeconds(value: unknown): string {
  const n = Number(value);
  if (!Number.isFinite(n)) return "—";
  return `${n.toLocaleString(undefined, {
    maximumFractionDigits: 3,
    minimumFractionDigits: n > 0 && n < 1 ? 3 : 0,
  })} s`;
}

function stageTimelineRows(stage: DrilldownStage | undefined): BarRow[] {
  return ((stage as any)?.timeline_analysis ?? [])
    .filter((row: any) => Number(row?.samples ?? 0) > 0)
    .map((row: any) => ({
      key: row.segment,
      label: row.label || row.segment || "(unknown)",
      value: Number(row.samples ?? 0),
      ratio: timelineDisplayRatio(row),
      detail: formatTimelineSeconds(row.estimated_seconds),
    }));
}

function developerBucketFor(segment: string): DeveloperBucketSpec {
  switch (segment) {
    case "SQL_EXECUTION":
    case "DB_NETWORK_WAIT":
      return DEVELOPER_BUCKETS.SQL_TIME;
    case "DB_FETCH":
      return DEVELOPER_BUCKETS.DB_FETCH;
    case "CONNECTION_POOL_WAIT":
      return DEVELOPER_BUCKETS.CONNECTION_POOL_WAIT;
    case "EXTERNAL_CALL":
    case "EXTERNAL_NETWORK_WAIT":
      return DEVELOPER_BUCKETS.EXTERNAL_TIME;
    case "NETWORK_PREP":
      return DEVELOPER_BUCKETS.EXTERNAL_PREP;
    case "LOGGING":
      return DEVELOPER_BUCKETS.LOGGING;
    case "DTO_MAPPING":
      return DEVELOPER_BUCKETS.DTO_MAPPING;
    case "FRAMEWORK_MIDDLEWARE":
    case "STARTUP_FRAMEWORK":
      return DEVELOPER_BUCKETS.FRAMEWORK_TIME;
    case "INTERNAL_METHOD":
      return DEVELOPER_BUCKETS.INTERNAL_METHOD;
    default:
      return DEVELOPER_BUCKETS.RUNTIME_OTHER;
  }
}

function stageDeveloperTimelineRows(
  stage: DrilldownStage | undefined,
  t: (key: any) => string,
): DeveloperTimelineRow[] {
  const rows = ((stage as any)?.timeline_analysis ?? []).filter(
    (row: any) => Number(row?.samples ?? 0) > 0,
  );
  const byKey = new Map<string, DeveloperTimelineRow>();
  for (const row of rows) {
    const segment = String(row?.segment ?? "");
    const bucket = developerBucketFor(segment);
    const current =
      byKey.get(bucket.key) ??
      ({
        key: bucket.key,
        label: t(bucket.labelKey),
        group: bucket.group,
        order: bucket.order,
        samples: 0,
        estimatedSeconds: 0,
        color: bucket.color,
        rawSegments: [],
      } satisfies DeveloperTimelineRow);
    current.samples += Number(row?.samples ?? 0);
    current.estimatedSeconds += Number(row?.estimated_seconds ?? 0);
    const rawLabel = String(row?.label || row?.segment || bucket.key);
    if (!current.rawSegments.includes(rawLabel)) {
      current.rawSegments.push(rawLabel);
    }
    byKey.set(bucket.key, current);
  }
  return [...byKey.values()].sort((a, b) => a.order - b.order);
}

function formatSamples(value: number, samplesLabel: string): string {
  return `${value.toLocaleString()} ${samplesLabel}`;
}

function compositionPct(value: number, base: number): number {
  return base > 0 ? (value / base) * 100 : 0;
}

export type DrilldownPanelProps = {
  baseRequest: AnalyzeRequest | null;
  onError?: (message: string) => void;
};

export function DrilldownPanel({ baseRequest, onError }: DrilldownPanelProps) {
  const { locale, t } = useI18n();
  const [filters, setFilters] = useState<DrilldownFilter[]>([]);
  const [draft, setDraft] = useState<DraftFilter>(DEFAULT_DRAFT);
  const [stages, setStages] = useState<DrilldownStage[]>([]);
  const [busy, setBusy] = useState(false);
  const [timelineMode, setTimelineMode] = useState<DrilldownTimelineMode>("developer");

  const lastStage = stages[stages.length - 1];
  const stageFlamegraph = useMemo(() => adaptStageFlamegraph(lastStage), [lastStage]);
  const timelineRows = useMemo(() => stageTimelineRows(lastStage), [lastStage]);
  const developerTimelineRows = useMemo(
    () => stageDeveloperTimelineRows(lastStage, t),
    [lastStage, t],
  );

  const runDrilldown = async (nextFilters: DrilldownFilter[]) => {
    if (!baseRequest) return;
    setBusy(true);
    try {
      const request = new DrilldownRequest({
        ...baseRequest,
        filters: nextFilters,
      } as any);
      const result = await ProfilerService.Drilldown(request);
      setStages(result);
    } catch (err: any) {
      onError?.(String(err?.message ?? err));
    } finally {
      setBusy(false);
    }
  };

  const handleAdd = () => {
    if (!draft.pattern.trim()) return;
    const next: DrilldownFilter = new DrilldownFilter({
      pattern: draft.pattern.trim(),
      filter_type: draft.filterType,
      match_mode: draft.matchMode,
      view_mode: draft.viewMode,
      case_sensitive: draft.caseSensitive,
      label: "",
    });
    const updated = [...filters, next];
    setFilters(updated);
    setDraft(DEFAULT_DRAFT);
    runDrilldown(updated);
  };

  const handleRemove = (index: number) => {
    const updated = filters.filter((_, i) => i !== index);
    setFilters(updated);
    if (updated.length === 0) {
      setStages([]);
      return;
    }
    runDrilldown(updated);
  };

  const handleClear = () => {
    setFilters([]);
    setStages([]);
  };

  if (!baseRequest) {
    return null;
  }

  return (
    <section className="card drilldown">
      <div className="drilldown-header">
        <h2>
          <HelpedTitle help={getHelpText(locale, "sectionFilters")}>
            {t("drilldownTitle")}
          </HelpedTitle>
        </h2>
        {filters.length > 0 && (
          <button type="button" className="ghost" onClick={handleClear} disabled={busy}>
            {t("drilldownClear")}
          </button>
        )}
      </div>

      <div className="drilldown-form">
        <input
          type="text"
          className="path-input"
          value={draft.pattern}
          placeholder={t("drilldownPattern")}
          onChange={(e) => setDraft({ ...draft, pattern: e.target.value })}
          onKeyDown={(e) => {
            if (e.key === "Enter") handleAdd();
          }}
        />
        <label className="field compact">
          <HelpedLabel help={getHelpText(locale, "optionDrillFilterType")}>
            {t("drilldownFilterType")}
          </HelpedLabel>
          <select
            value={draft.filterType}
            onChange={(e) => setDraft({ ...draft, filterType: e.target.value as FilterType })}
          >
            <option value="include_text">{t("drilldownInclude")}</option>
            <option value="exclude_text">{t("drilldownExclude")}</option>
            <option value="regex_include">{t("drilldownRegexInclude")}</option>
            <option value="regex_exclude">{t("drilldownRegexExclude")}</option>
          </select>
        </label>
        <label className="field compact">
          <HelpedLabel help={getHelpText(locale, "optionDrillMatchMode")}>
            {t("drilldownMatchMode")}
          </HelpedLabel>
          <select
            value={draft.matchMode}
            onChange={(e) => setDraft({ ...draft, matchMode: e.target.value as MatchMode })}
          >
            <option value="anywhere">{t("drilldownAnywhere")}</option>
            <option value="ordered">{t("drilldownOrdered")}</option>
            <option value="subtree">{t("drilldownSubtree")}</option>
          </select>
        </label>
        <label className="field compact">
          <HelpedLabel help={getHelpText(locale, "optionDrillViewMode")}>
            {t("drilldownViewMode")}
          </HelpedLabel>
          <select
            value={draft.viewMode}
            onChange={(e) => setDraft({ ...draft, viewMode: e.target.value as ViewMode })}
          >
            <option value="preserve_full_path">{t("drilldownPreservePath")}</option>
            <option value="reroot_at_match">{t("drilldownReroot")}</option>
          </select>
        </label>
        <label className="checkbox">
          <input
            type="checkbox"
            checked={draft.caseSensitive}
            onChange={(e) => setDraft({ ...draft, caseSensitive: e.target.checked })}
          />
          <HelpedLabel help={getHelpText(locale, "optionCaseSensitive")}>
            {t("drilldownCaseSensitive")}
          </HelpedLabel>
        </label>
        <button
          type="button"
          className="primary"
          onClick={handleAdd}
          disabled={busy || !draft.pattern.trim()}
        >
          {busy ? t("analyzing") : t("drilldownApply")}
        </button>
      </div>

      {stages.length === 0 ? (
        <p className="muted">{t("drilldownEmpty")}</p>
      ) : (
        <>
          <ol className="breadcrumb">
            {(lastStage?.breadcrumb ?? []).map((crumb, idx) => (
              <li key={`${crumb}-${idx}`}>{crumb}</li>
            ))}
          </ol>
          <ul className="filter-chips">
            {filters.map((filter, idx) => (
              <li key={`${filter.pattern}-${idx}`}>
                <span className="chip-type">{filter.filter_type}</span>
                <code>{filter.pattern}</code>
                <span className="chip-mode">{filter.match_mode}</span>
                <button
                  type="button"
                  className="chip-remove"
                  onClick={() => handleRemove(idx)}
                  disabled={busy}
                  aria-label={t("drilldownRemoveFilter")}
                >
                  ×
                </button>
              </li>
            ))}
          </ul>
          {lastStage?.diagnostics ? (
            <div className="alert error">
              <strong>{t("drilldownStageDiagnostic")}:</strong>{" "}
              {(lastStage.diagnostics as any)?.message ?? String(lastStage.diagnostics)}
            </div>
          ) : null}
          <div className="metric-grid">
            <Metric
              label={t("drilldownStageMatched")}
              value={(lastStage?.flamegraph?.samples ?? 0).toLocaleString()}
              help={getGenericMetricHelpText(locale, t("drilldownStageMatched"))}
            />
            <Metric
              label={t("drilldownStageRatio")}
              value={`${formatNumber(lastStage?.metrics?.total_ratio)}%`}
              help={getGenericMetricHelpText(locale, t("drilldownStageRatio"))}
            />
            <Metric
              label={t("drilldownStageParent")}
              value={`${formatNumber(lastStage?.metrics?.parent_stage_ratio)}%`}
              help={getGenericMetricHelpText(locale, t("drilldownStageParent"))}
            />
          </div>
          <div className="drilldown-result-section">
            <div className="drilldown-result-heading">
              <h3 className="inline-flex items-center gap-2 text-sm">
                {timelineMode === "developer" ? t("executionTimeComposition") : t("timelineTitle")}
                <HelpTip text={getHelpText(locale, "sectionTimeline")} />
              </h3>
              <div className="drilldown-mode-toggle" aria-label={t("drilldownTimelineMode")}>
                <button
                  type="button"
                  className={timelineMode === "developer" ? "active" : ""}
                  onClick={() => setTimelineMode("developer")}
                >
                  {t("drilldownTimelineDeveloper")}
                </button>
                <button
                  type="button"
                  className={timelineMode === "expert" ? "active" : ""}
                  onClick={() => setTimelineMode("expert")}
                >
                  {t("drilldownTimelineExpert")}
                </button>
              </div>
            </div>
            {timelineMode === "developer" ? (
              <DrilldownTimelineComposition
                rows={developerTimelineRows}
                totalSamples={lastStage?.flamegraph?.samples ?? 0}
                emptyLabel={t("timelineEmpty")}
                samplesLabel={t("samples")}
                groupLabels={{
                  database: t("drilldownDevGroupDatabase"),
                  external: t("drilldownDevGroupExternal"),
                  internal: t("drilldownDevGroupInternal"),
                }}
              />
            ) : (
              <HorizontalBarChart
                rows={timelineRows}
                emptyLabel={t("timelineEmpty")}
                valueSuffix={t("samples")}
                ratioFractionDigits={2}
              />
            )}
          </div>
          {stageFlamegraph && (
            <div className="drilldown-result-section">
              <h3 className="inline-flex items-center gap-2 text-sm">
                {t("flamegraphTitle")}
                <HelpTip text={getHelpText(locale, "sectionFlamegraph")} />
              </h3>
              <CanvasFlameGraph
                data={stageFlamegraph}
                exportName={`drilldown-${stages.length - 1}`}
              />
            </div>
          )}
        </>
      )}
    </section>
  );
}

function DrilldownTimelineComposition({
  rows,
  totalSamples,
  emptyLabel,
  samplesLabel,
  groupLabels,
}: {
  rows: DeveloperTimelineRow[];
  totalSamples: number;
  emptyLabel: string;
  samplesLabel: string;
  groupLabels: Record<CompositionGroupId, string>;
}) {
  const rowsSamples = rows.reduce((sum, row) => sum + row.samples, 0);
  const base = Math.max(totalSamples, rowsSamples, 1);
  const groups = COMPOSITION_GROUP_ORDER.map((id) => ({
    id,
    label: groupLabels[id],
    color: COMPOSITION_GROUP_COLORS[id],
    samples: rows
      .filter((row) => row.group === id)
      .reduce((sum, row) => sum + row.samples, 0),
    rows: rows.filter((row) => row.group === id),
  })).filter((group) => group.samples > 0);

  if (!rows.length || rowsSamples <= 0) {
    return <p className="muted">{emptyLabel}</p>;
  }

  return (
    <div className="drilldown-composition">
      <div className="drilldown-composition-groups">
        {groups.map((group) => {
          const widthPct = compositionPct(group.samples, base);
          return (
            <div
              key={group.id}
              className="drilldown-composition-group"
              style={{ width: `${widthPct.toFixed(2)}%` }}
              title={`${group.label}: ${formatSamples(group.samples, samplesLabel)} · ${widthPct.toFixed(2)}%`}
            >
              <div
                className="drilldown-composition-group-marker"
                style={{ borderColor: group.color }}
                aria-hidden
              />
              {widthPct >= 10 ? (
                <span style={{ color: group.color }}>
                  {group.label} · {widthPct.toFixed(1)}%
                </span>
              ) : null}
            </div>
          );
        })}
      </div>
      <div className="drilldown-composition-segments">
        {rows.map((row) => {
          const widthPct = compositionPct(row.samples, base);
          return (
            <div
              key={row.key}
              className="drilldown-composition-segment"
              style={{
                width: `${widthPct.toFixed(2)}%`,
                backgroundColor: row.color,
              }}
              title={`${row.label}: ${formatSamples(row.samples, samplesLabel)} · ${widthPct.toFixed(2)}% · ${formatTimelineSeconds(row.estimatedSeconds)}${row.rawSegments.length > 0 ? `\n${row.rawSegments.join(", ")}` : ""}`}
            >
              {widthPct >= 10 ? <span>{row.label}</span> : null}
            </div>
          );
        })}
      </div>
      <div className="drilldown-composition-legend">
        {groups.map((group) => (
          <section key={group.id} className="drilldown-composition-card">
            <header>
              <span
                className="drilldown-composition-dot"
                style={{ backgroundColor: group.color }}
                aria-hidden
              />
              <span>{group.label}</span>
              <strong>{compositionPct(group.samples, base).toFixed(1)}%</strong>
            </header>
            <ul>
              {group.rows.map((row) => {
                const rowPct = compositionPct(row.samples, base);
                return (
                  <li key={row.key} title={row.rawSegments.join(", ")}>
                    <span
                      className="drilldown-composition-dot"
                      style={{ backgroundColor: row.color }}
                      aria-hidden
                    />
                    <span className="drilldown-composition-label">{row.label}</span>
                    <span className="drilldown-composition-value">
                      {formatSamples(row.samples, samplesLabel)} · {rowPct.toFixed(1)}%
                    </span>
                  </li>
                );
              })}
            </ul>
          </section>
        ))}
      </div>
    </div>
  );
}

function Metric({ label, value, help }: { label: string; value: string; help: string }) {
  return (
    <div className="metric">
      <div className="metric-label inline-flex items-center gap-1.5">
        {label}
        <HelpTip text={help} align="end" />
      </div>
      <div className="metric-value">{value}</div>
    </div>
  );
}

function formatNumber(value: any): string {
  const n = Number(value);
  return Number.isFinite(n) ? n.toFixed(2) : "—";
}
