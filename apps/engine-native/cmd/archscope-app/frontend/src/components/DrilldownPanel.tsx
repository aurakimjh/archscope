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

type FilterType = "include_text" | "exclude_text" | "regex_include" | "regex_exclude";
type MatchMode = "anywhere" | "ordered" | "subtree";
type ViewMode = "preserve_full_path" | "reroot_at_match";

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

export type DrilldownPanelProps = {
  baseRequest: AnalyzeRequest | null;
  onError?: (message: string) => void;
};

export function DrilldownPanel({ baseRequest, onError }: DrilldownPanelProps) {
  const { t } = useI18n();
  const [filters, setFilters] = useState<DrilldownFilter[]>([]);
  const [draft, setDraft] = useState<DraftFilter>(DEFAULT_DRAFT);
  const [stages, setStages] = useState<DrilldownStage[]>([]);
  const [busy, setBusy] = useState(false);

  const lastStage = stages[stages.length - 1];
  const stageFlamegraph = useMemo(() => adaptStageFlamegraph(lastStage), [lastStage]);

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
        <h2>{t("drilldownTitle")}</h2>
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
          <span>{t("drilldownFilterType")}</span>
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
          <span>{t("drilldownMatchMode")}</span>
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
          <span>{t("drilldownViewMode")}</span>
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
          <span>{t("drilldownCaseSensitive")}</span>
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
            />
            <Metric
              label={t("drilldownStageRatio")}
              value={`${formatNumber(lastStage?.metrics?.total_ratio)}%`}
            />
            <Metric
              label={t("drilldownStageParent")}
              value={`${formatNumber(lastStage?.metrics?.parent_stage_ratio)}%`}
            />
          </div>
          {stageFlamegraph && (
            <CanvasFlameGraph
              data={stageFlamegraph}
              exportName={`drilldown-${stages.length - 1}`}
            />
          )}
        </>
      )}
    </section>
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

function formatNumber(value: any): string {
  const n = Number(value);
  return Number.isFinite(n) ? n.toFixed(2) : "—";
}
