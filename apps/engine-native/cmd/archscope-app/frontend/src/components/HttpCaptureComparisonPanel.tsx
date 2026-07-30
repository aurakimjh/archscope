// ─────────────────────────────────────────────────────────────────────
// [한글] HttpCaptureComparisonPanel.tsx — HTTP 세션 Diff(H-RG5 / T-582)
// 비교 패널. HttpCapturePage 와 Analysis Workspace 양쪽에서 마운트되는
// 공유 표면으로, state/httpCaptureDiff.ts 의 module store 를 통해 선택과
// 결과를 공유한다.
//
// 책임/목적:
//   - Workspace 에 등록된 http_capture 결과 중 baseline(A)/target(B) 을
//     선택해 backend `ResolveWorkspaceComparison` 라우팅 판정을 거쳐
//     `AnalyzeHttpCaptureDiff` 를 실행.
//   - 결과 렌더: 세션 참조, 시간 정렬 등급, before/after/Δ 요약(명시적
//     분자/분모), 등급 게이트 오버레이, 변경/추가/제거 endpoint 테이블,
//     host/process 테이블, findings, 커서 드릴다운(행 클릭 → 상세).
//   - 억제 규칙: overlay 는 backend 의 `overlay_allowed`+grade 판정을
//     그대로 따르고, HAR pseudo-process 쌍의 process 차원은 이유와 함께
//     비활성 상태로 노출한다 (숨기지 않고 왜 없는지 말한다).
//
// 의존성 주의: Wails 호출은 이 컴포넌트에서만 수행하고 파생 로직은 전부
// state/httpCaptureDiff.ts 의 순수 함수를 사용한다.
// ─────────────────────────────────────────────────────────────────────
import { ArrowLeftRight, GitCompareArrows, Loader2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { engine } from "@/bridge/engine";
import type {
  HttpCaptureAnalysisResult,
  HttpCaptureDiffComparisonRow,
  HttpCaptureDiffSessionRef,
  HttpCaptureDiffSideMetrics,
} from "@/bridge/types";
import { ErrorPanel } from "@/components/AnalyzerFeedback";
import { SlideOverPanel } from "@/components/SlideOverPanel";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import {
  getWorkspaceEntry,
  useAnalysisWorkspace,
  type AnalysisWorkspaceEntry,
} from "@/state/analysisWorkspace";
import { formatBytes } from "@/state/httpCapture";
import {
  DIFF_ALIGNMENT_LABEL_KEYS,
  DIFF_CHANGE_LABEL_KEYS,
  DIFF_RATE_UNAVAILABLE_LABEL_KEYS,
  DIFF_SOURCE_KIND_LABEL_KEYS,
  diffCandidateEntries,
  diffHasChanges,
  diffOverlayPolicy,
  dispatchHttpCaptureDiff,
  extractDiffEnvelope,
  hasDiffSourceProjection,
  resolveDiffAlignmentGrade,
  resolveDiffChange,
  resolveDiffRateUnavailable,
  resolveDiffSourceKind,
  selectDiffFindings,
  selectDiffSummary,
  selectDiffTable,
  useHttpCaptureDiff,
  type DiffTableKey,
} from "@/state/httpCaptureDiff";
import { formatMilliseconds, formatNumber } from "@/utils/formatters";

type Translate = (key: MessageKey) => string;

export function HttpCaptureComparisonPanel(): React.JSX.Element {
  const { t } = useI18n();
  const workspace = useAnalysisWorkspace();
  const state = useHttpCaptureDiff();
  const [drill, setDrill] = useState<{ row: HttpCaptureDiffComparisonRow; dimension: string } | null>(null);

  const candidates = useMemo(
    () => diffCandidateEntries(workspace.entries),
    [workspace.entries],
  );

  // Adopt the versioned diff contract once (H-RG4 R8 pattern): a schema this
  // build does not implement disables comparison with a visible notice
  // instead of being half-honored.
  useEffect(() => {
    if (state.contract !== null || state.contractMismatch) return;
    let cancelled = false;
    engine
      .getHttpCaptureDiffContract()
      .then((contract) => {
        if (!cancelled) dispatchHttpCaptureDiff({ type: "contractLoaded", contract });
      })
      .catch(() => {
        if (!cancelled) dispatchHttpCaptureDiff({ type: "contractUnavailable" });
      });
    return () => {
      cancelled = true;
    };
  }, [state.contract, state.contractMismatch]);

  const beforeEntry = getWorkspaceEntry(state.beforeId);
  const afterEntry = getWorkspaceEntry(state.afterId);
  const beforeMissingProjection = !!beforeEntry && !hasDiffSourceProjection(beforeEntry.result);
  const afterMissingProjection = !!afterEntry && !hasDiffSourceProjection(afterEntry.result);

  const compare = useCallback(async () => {
    const before = getWorkspaceEntry(state.beforeId);
    const after = getWorkspaceEntry(state.afterId);
    if (!before || !after || state.running) return;
    dispatchHttpCaptureDiff({ type: "compareStart" });
    try {
      // The backend owns the routing decision; the renderer never assumes a
      // pair is comparable, even after filtering candidates by type.
      const route = await engine.resolveWorkspaceComparison({
        beforeType: before.result_type,
        afterType: after.result_type,
      });
      if (!route.supported) {
        dispatchHttpCaptureDiff({
          type: "compareError",
          error: {
            code: "HTTP_DIFF_ROUTE_UNSUPPORTED",
            message: route.reason || "comparison route unsupported",
          },
        });
        return;
      }
      const result = await engine.analyzeHttpCaptureDiff({
        before: before.result as unknown as HttpCaptureAnalysisResult,
        after: after.result as unknown as HttpCaptureAnalysisResult,
      });
      dispatchHttpCaptureDiff({
        type: "compareSuccess",
        result,
        pair: { beforeId: before.id, afterId: after.id },
      });
    } catch (caught) {
      dispatchHttpCaptureDiff({
        type: "compareError",
        error: {
          code: "HTTP_DIFF_COMPARE_FAILED",
          message: caught instanceof Error ? caught.message : String(caught),
        },
      });
    }
  }, [state.beforeId, state.afterId, state.running]);

  const summary = useMemo(() => selectDiffSummary(state.result), [state.result]);
  const envelope = useMemo(() => extractDiffEnvelope(state.result), [state.result]);
  const findings = useMemo(() => selectDiffFindings(state.result), [state.result]);
  const alignment = envelope?.time_alignment ?? summary?.time_alignment ?? null;
  const overlay = diffOverlayPolicy(alignment);
  const hasChanges = diffHasChanges(state.result);

  const canCompare =
    state.contractSupported &&
    !state.running &&
    !!beforeEntry &&
    !!afterEntry &&
    !beforeMissingProjection &&
    !afterMissingProjection;

  return (
    <section className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle className="inline-flex items-center gap-2">
            <GitCompareArrows className="nav-lucide-sm" />
            {t("httpCaptureDiffTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-xs text-muted-foreground">{t("httpCaptureDiffDescription")}</p>
          {state.contractMismatch && (
            <p
              className="rounded-md border border-amber-500/50 bg-amber-500/10 p-2 text-xs text-amber-700 dark:text-amber-400"
              role="status"
            >
              {t("httpCaptureDiffContractMismatch")}
            </p>
          )}
          {candidates.length === 0 ? (
            <p className="text-sm text-muted-foreground" role="status">
              {t("httpCaptureDiffNoCandidates")}
            </p>
          ) : (
            <div className="flex flex-wrap items-end gap-2">
              <SelectionColumn
                label={t("httpCaptureDiffBeforeLabel")}
                value={state.beforeId}
                candidates={candidates}
                missingProjection={beforeMissingProjection}
                onChange={(id) => dispatchHttpCaptureDiff({ type: "setBefore", id })}
                t={t}
              />
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => dispatchHttpCaptureDiff({ type: "swap" })}
                disabled={!state.beforeId && !state.afterId}
                aria-label={t("httpCaptureDiffSwap")}
                title={t("httpCaptureDiffSwap")}
              >
                <ArrowLeftRight className="h-3.5 w-3.5" />
              </Button>
              <SelectionColumn
                label={t("httpCaptureDiffAfterLabel")}
                value={state.afterId}
                candidates={candidates}
                missingProjection={afterMissingProjection}
                onChange={(id) => dispatchHttpCaptureDiff({ type: "setAfter", id })}
                t={t}
              />
              <Button type="button" size="sm" disabled={!canCompare} onClick={() => void compare()}>
                {state.running ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    {t("analyzing")}
                  </>
                ) : (
                  t("httpCaptureDiffRun")
                )}
              </Button>
            </div>
          )}
          {candidates.length > 0 && (!state.beforeId || !state.afterId) && (
            <p className="text-xs text-muted-foreground">{t("httpCaptureDiffNeedTwo")}</p>
          )}
        </CardContent>
      </Card>

      <ErrorPanel
        error={state.error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />

      {state.result && summary && (
        <>
          <SessionsCard summary={summary} t={t} />
          <AlignmentCard
            alignment={alignment}
            overlay={overlay}
            t={t}
          />
          <SummaryDeltaCard summary={summary} perMinuteAllowed={overlay.perMinute} t={t} />
          <DurationOverlayCard
            summary={summary}
            enabled={overlay.durations}
            reason={alignment?.reason ?? ""}
            t={t}
          />
          {!hasChanges && (
            <Card>
              <CardContent className="py-6 text-center text-sm text-muted-foreground">
                {t("httpCaptureDiffNoChanges")}
              </CardContent>
            </Card>
          )}
          <ComparisonTable
            titleKey="httpCaptureDiffTableChangedTitle"
            rows={selectDiffTable(state.result, "endpoints_changed")}
            dimension="endpoints_changed"
            onOpen={setDrill}
            t={t}
          />
          <ComparisonTable
            titleKey="httpCaptureDiffTableAddedTitle"
            rows={selectDiffTable(state.result, "endpoints_added")}
            dimension="endpoints_added"
            onOpen={setDrill}
            t={t}
          />
          <ComparisonTable
            titleKey="httpCaptureDiffTableRemovedTitle"
            rows={selectDiffTable(state.result, "endpoints_removed")}
            dimension="endpoints_removed"
            onOpen={setDrill}
            t={t}
          />
          <ComparisonTable
            titleKey="httpCaptureDiffTableHostsTitle"
            rows={selectDiffTable(state.result, "hosts_changed")}
            dimension="hosts_changed"
            onOpen={setDrill}
            t={t}
          />
          {envelope?.process_dimension.available ? (
            <ComparisonTable
              titleKey="httpCaptureDiffTableProcessesTitle"
              rows={selectDiffTable(state.result, "processes_changed")}
              dimension="processes_changed"
              onOpen={setDrill}
              t={t}
            />
          ) : (
            <Card>
              <CardHeader>
                <CardTitle>{t("httpCaptureDiffTableProcessesTitle")}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-xs text-muted-foreground" role="status">
                  {t("httpCaptureDiffProcessUnavailable")}
                  {envelope?.process_dimension.reason && (
                    <span className="mt-1 block font-mono text-[11px]">
                      {envelope.process_dimension.reason}
                    </span>
                  )}
                </p>
              </CardContent>
            </Card>
          )}
          {findings.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("httpCaptureDiffFindingsTitle")}</CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-2">
                {findings.map((finding, index) => (
                  <div key={`${finding.code}-${index}`} className="flex items-start gap-2 text-sm">
                    <span
                      className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${
                        finding.severity === "warning" ? "bg-amber-500" : "bg-sky-500"
                      }`}
                      aria-hidden="true"
                    />
                    <div className="min-w-0">
                      <code className="text-xs font-semibold">{finding.code}</code>
                      <p className="text-muted-foreground">{finding.message}</p>
                    </div>
                  </div>
                ))}
              </CardContent>
            </Card>
          )}
          {envelope && (
            <p className="text-[11px] text-muted-foreground">
              {t("httpCaptureDiffBoundsNote").replace("{n}", formatNumber(envelope.table_limit))}
              {" · "}
              {`url_template_version ${envelope.url_template_version}`}
              {!envelope.dimension_totals.before.cross_check_passed ||
              !envelope.dimension_totals.after.cross_check_passed ? (
                <span className="ml-1 text-amber-700 dark:text-amber-400">
                  {t("httpCaptureDiffCrossCheckFailed")}
                </span>
              ) : null}
            </p>
          )}
        </>
      )}

      <ComparisonDetail drill={drill} onClose={() => setDrill(null)} t={t} />
    </section>
  );
}

// ── Selection ───────────────────────────────────────────────────────

function SelectionColumn({
  label,
  value,
  candidates,
  missingProjection,
  onChange,
  t,
}: {
  label: string;
  value: string | null;
  candidates: AnalysisWorkspaceEntry[];
  missingProjection: boolean;
  onChange: (id: string | null) => void;
  t: Translate;
}): React.JSX.Element {
  return (
    <div className="flex min-w-52 flex-1 flex-col gap-1">
      <label className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
        <select
          className="mt-1 block h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          value={value ?? ""}
          onChange={(event) => onChange(event.target.value || null)}
        >
          <option value="">{t("httpCaptureDiffSelectPlaceholder")}</option>
          {candidates.map((entry) => (
            <option key={entry.id} value={entry.id}>
              {entry.title} · {new Date(entry.recorded_at).toLocaleString()}
            </option>
          ))}
        </select>
      </label>
      {missingProjection && (
        <p className="text-[11px] text-amber-700 dark:text-amber-400" role="status">
          {t("httpCaptureDiffMissingProjection")}
        </p>
      )}
    </div>
  );
}

// ── Sessions ────────────────────────────────────────────────────────

function SessionsCard({
  summary,
  t,
}: {
  summary: NonNullable<ReturnType<typeof selectDiffSummary>>;
  t: Translate;
}): React.JSX.Element {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("httpCaptureDiffSessionsTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3 sm:grid-cols-2">
        <SessionRef label={t("httpCaptureDiffSessionBefore")} session={summary.before_session} t={t} />
        <SessionRef label={t("httpCaptureDiffSessionAfter")} session={summary.after_session} t={t} />
      </CardContent>
    </Card>
  );
}

function SessionRef({
  label,
  session,
  t,
}: {
  label: string;
  session: HttpCaptureDiffSessionRef;
  t: Translate;
}): React.JSX.Element {
  const kind = resolveDiffSourceKind(session.source_kind);
  return (
    <div className="rounded-md border border-border bg-muted/20 p-3 text-xs">
      <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</p>
      <p className="mt-1 break-all font-mono" title={session.session_id}>
        {session.session_id}
      </p>
      <p className="mt-1 text-muted-foreground">
        {/* Raw engine token stays reachable on hover (S4 precedent). */}
        <span title={session.source_kind}>{t(DIFF_SOURCE_KIND_LABEL_KEYS[kind])}</span>
        {session.source_format ? ` · ${session.source_format}` : ""}
        {" · "}
        {formatNumber(session.transactions)} {t("httpCaptureDiffTransactions")}
      </p>
    </div>
  );
}

// ── Alignment ───────────────────────────────────────────────────────

function AlignmentCard({
  alignment,
  overlay,
  t,
}: {
  alignment: { grade: string; overlay_allowed: boolean; reason: string } | null;
  overlay: { durations: boolean; perMinute: boolean };
  t: Translate;
}): React.JSX.Element | null {
  if (!alignment) return null;
  const grade = resolveDiffAlignmentGrade(alignment.grade);
  const tone =
    grade === "aligned"
      ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400"
      : grade === "duration_only"
        ? "bg-amber-500/15 text-amber-700 dark:text-amber-400"
        : "bg-red-500/15 text-red-700 dark:text-red-400";
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("httpCaptureDiffAlignmentTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-2 text-xs">
        <p>
          <span className={`rounded-full px-2 py-0.5 font-medium ${tone}`} title={alignment.grade}>
            {t(DIFF_ALIGNMENT_LABEL_KEYS[grade])}
          </span>
        </p>
        <p className="text-muted-foreground">{alignment.reason}</p>
        {!overlay.durations && (
          <p className="text-amber-700 dark:text-amber-400" role="status">
            {t("httpCaptureDiffOverlaySuppressed")}
          </p>
        )}
        {overlay.durations && !overlay.perMinute && (
          <p className="text-amber-700 dark:text-amber-400" role="status">
            {t("httpCaptureDiffOverlayPerMinuteSuppressed")}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

// ── Before/after summary with explicit denominators ─────────────────

function SummaryDeltaCard({
  summary,
  perMinuteAllowed,
  t,
}: {
  summary: NonNullable<ReturnType<typeof selectDiffSummary>>;
  perMinuteAllowed: boolean;
  t: Translate;
}): React.JSX.Element {
  const { before, after, delta } = summary;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("httpCaptureDiffSummaryTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <caption className="sr-only">{t("httpCaptureDiffSummaryTitle")}</caption>
          <thead>
            <tr className="border-b border-border text-muted-foreground">
              <th scope="col" className="p-2 font-medium" />
              <th scope="col" className="p-2 text-right font-medium">
                {t("httpCaptureDiffSessionBefore")}
              </th>
              <th scope="col" className="p-2 text-right font-medium">
                {t("httpCaptureDiffSessionAfter")}
              </th>
              <th scope="col" className="p-2 text-right font-medium">
                {t("httpCaptureDiffDelta")}
              </th>
            </tr>
          </thead>
          <tbody className="tabular-nums">
            <SummaryRow
              label={t("httpCaptureDiffMetricCount")}
              before={formatNumber(before.count)}
              after={formatNumber(after.count)}
              delta={signedNumber(delta.count)}
            />
            <SummaryRow
              label={t("httpCaptureDiffMetricErrors")}
              before={formatNumber(before.errors)}
              after={formatNumber(after.errors)}
              delta={signedNumber(delta.errors)}
            />
            <SummaryRow
              label={t("httpCaptureDiffMetricErrorRate")}
              before={explicitRateText(before.error_rate)}
              after={explicitRateText(after.error_rate)}
              delta={signedPct(delta.error_rate)}
            />
            <SummaryRow
              label={t("httpCaptureDiffMetricPerMinute")}
              before={perMinuteText(before, perMinuteAllowed, t)}
              after={perMinuteText(after, perMinuteAllowed, t)}
              delta={
                perMinuteAllowed && delta.count_per_minute !== null
                  ? signedFixed(delta.count_per_minute)
                  : "—"
              }
            />
            <SummaryRow
              label={t("httpCaptureDiffMetricP50")}
              before={formatMilliseconds(Math.round(before.duration_p50_ms))}
              after={formatMilliseconds(Math.round(after.duration_p50_ms))}
              delta={signedMs(delta.duration_p50_ms)}
            />
            <SummaryRow
              label={t("httpCaptureDiffMetricP95")}
              before={formatMilliseconds(Math.round(before.duration_p95_ms))}
              after={formatMilliseconds(Math.round(after.duration_p95_ms))}
              delta={signedMs(delta.duration_p95_ms)}
            />
            <SummaryRow
              label={t("httpCaptureDiffMetricP99")}
              before={formatMilliseconds(Math.round(before.duration_p99_ms))}
              after={formatMilliseconds(Math.round(after.duration_p99_ms))}
              delta={signedMs(delta.duration_p99_ms)}
            />
            <SummaryRow
              label={t("httpCaptureDiffDurationSamples")}
              before={formatNumber(before.duration_samples)}
              after={formatNumber(after.duration_samples)}
              delta={signedNumber(after.duration_samples - before.duration_samples)}
            />
            <SummaryRow
              label={t("httpCaptureDiffMetricRequestBytes")}
              before={formatBytes(before.request_bytes)}
              after={formatBytes(after.request_bytes)}
              delta={signedBytes(delta.request_bytes)}
            />
            <SummaryRow
              label={t("httpCaptureDiffMetricResponseBytes")}
              before={formatBytes(before.response_bytes)}
              after={formatBytes(after.response_bytes)}
              delta={signedBytes(delta.response_bytes)}
            />
          </tbody>
        </table>
        <p className="mt-2 text-[11px] text-muted-foreground">
          {t("httpCaptureDiffDetailSamplesNote")}
        </p>
      </CardContent>
    </Card>
  );
}

function SummaryRow({
  label,
  before,
  after,
  delta,
}: {
  label: string;
  before: string;
  after: string;
  delta: string;
}): React.JSX.Element {
  return (
    <tr className="border-t border-border/50">
      <th scope="row" className="p-2 text-left font-medium">
        {label}
      </th>
      <td className="p-2 text-right">{before}</td>
      <td className="p-2 text-right">{after}</td>
      <td className="p-2 text-right font-mono">{delta}</td>
    </tr>
  );
}

// ── Duration overlay (grade-gated) ──────────────────────────────────

function DurationOverlayCard({
  summary,
  enabled,
  reason,
  t,
}: {
  summary: NonNullable<ReturnType<typeof selectDiffSummary>>;
  enabled: boolean;
  reason: string;
  t: Translate;
}): React.JSX.Element {
  const rows: Array<{ label: string; before: number; after: number }> = [
    { label: t("httpCaptureDiffMetricP50"), before: summary.before.duration_p50_ms, after: summary.after.duration_p50_ms },
    { label: t("httpCaptureDiffMetricP95"), before: summary.before.duration_p95_ms, after: summary.after.duration_p95_ms },
    { label: t("httpCaptureDiffMetricP99"), before: summary.before.duration_p99_ms, after: summary.after.duration_p99_ms },
  ];
  const max = rows.reduce((acc, row) => Math.max(acc, row.before, row.after), 0);
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("httpCaptureDiffDurationOverlayTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {!enabled ? (
          <p className="text-xs text-muted-foreground" role="status">
            {t("httpCaptureDiffOverlaySuppressed")}
            {reason && <span className="mt-1 block">{reason}</span>}
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {rows.map((row) => (
              <li key={row.label} className="flex items-center gap-2 text-[11px]">
                <span className="w-10 shrink-0 font-mono text-muted-foreground">{row.label}</span>
                <div className="flex flex-1 flex-col gap-0.5">
                  <OverlayBar
                    label={t("httpCaptureDiffSessionBefore")}
                    ms={row.before}
                    max={max}
                    className="bg-muted-foreground/40"
                  />
                  <OverlayBar
                    label={t("httpCaptureDiffSessionAfter")}
                    ms={row.after}
                    max={max}
                    className="bg-primary/70"
                  />
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

function OverlayBar({
  label,
  ms,
  max,
  className,
}: {
  label: string;
  ms: number;
  max: number;
  className: string;
}): React.JSX.Element {
  return (
    <div className="flex items-center gap-2" title={`${label}: ${ms.toFixed(1)} ms`}>
      <div className="h-2.5 flex-1 overflow-hidden rounded-sm bg-muted">
        <div className={`h-full ${className}`} style={{ width: `${max > 0 ? (ms / max) * 100 : 0}%` }} />
      </div>
      <span className="w-16 shrink-0 text-right font-mono tabular-nums">{ms.toFixed(1)}ms</span>
    </div>
  );
}

// ── Comparison tables ───────────────────────────────────────────────

function ComparisonTable({
  titleKey,
  rows,
  dimension,
  onOpen,
  t,
}: {
  titleKey: MessageKey;
  rows: HttpCaptureDiffComparisonRow[];
  dimension: DiffTableKey;
  onOpen: (drill: { row: HttpCaptureDiffComparisonRow; dimension: string }) => void;
  t: Translate;
}): React.JSX.Element | null {
  if (rows.length === 0) return null;
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2">
        <CardTitle>{t(titleKey)}</CardTitle>
        <span className="tabular-nums text-xs text-muted-foreground">{formatNumber(rows.length)}</span>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <caption className="sr-only">{t(titleKey)}</caption>
          <thead>
            <tr className="border-b border-border text-muted-foreground">
              <th scope="col" className="p-2 font-medium">
                {t("httpCaptureDiffColKey")}
              </th>
              <th scope="col" className="p-2 font-medium">
                {t("httpCaptureDiffColChange")}
              </th>
              <th scope="col" className="p-2 text-right font-medium">
                {t("httpCaptureDiffColCountAB")}
              </th>
              <th scope="col" className="p-2 text-right font-medium">
                {t("httpCaptureDiffColErrorRate")}
              </th>
              <th scope="col" className="p-2 text-right font-medium">
                {t("httpCaptureDiffColP95")}
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => {
              const change = resolveDiffChange(row.change);
              return (
                <tr
                  key={`${dimension}-${row.key}`}
                  className="cursor-pointer border-t hover:bg-accent focus-visible:bg-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
                  role="button"
                  tabIndex={0}
                  aria-label={`${t("httpCaptureDiffRowOpen")}: ${row.key}`}
                  onClick={() => onOpen({ row, dimension })}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      onOpen({ row, dimension });
                    }
                  }}
                >
                  <td className="max-w-md truncate p-2 font-mono" title={row.key}>
                    {row.key}
                  </td>
                  <td className="p-2">
                    <span title={row.change}>{t(DIFF_CHANGE_LABEL_KEYS[change])}</span>
                  </td>
                  <td className="p-2 text-right tabular-nums">
                    {sideText(change, "added", formatNumber(row.before.count))} →{" "}
                    {sideText(change, "removed", formatNumber(row.after.count))}
                  </td>
                  <td className="p-2 text-right tabular-nums">
                    {sideText(change, "added", explicitRateText(row.before.error_rate))} →{" "}
                    {sideText(change, "removed", explicitRateText(row.after.error_rate))}
                  </td>
                  <td className="p-2 text-right tabular-nums">
                    {sideText(change, "added", formatMilliseconds(Math.round(row.before.duration_p95_ms)))} →{" "}
                    {sideText(change, "removed", formatMilliseconds(Math.round(row.after.duration_p95_ms)))}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

// ── Cursor drilldown ────────────────────────────────────────────────

function ComparisonDetail({
  drill,
  onClose,
  t,
}: {
  drill: { row: HttpCaptureDiffComparisonRow; dimension: string } | null;
  onClose: () => void;
  t: Translate;
}): React.JSX.Element {
  return (
    <SlideOverPanel
      open={drill !== null}
      onClose={onClose}
      title={drill ? drill.row.key : t("httpCaptureDiffDetailTitle")}
      width={560}
    >
      {drill && (
        <div className="flex flex-col gap-4 text-sm">
          <p className="text-xs text-muted-foreground">
            <span title={drill.row.change}>
              {t(DIFF_CHANGE_LABEL_KEYS[resolveDiffChange(drill.row.change)])}
            </span>
            {" · "}
            <span className="font-mono">{drill.dimension}</span>
          </p>
          <div className="grid gap-3 sm:grid-cols-2">
            <SideMetricsPanel
              label={t("httpCaptureDiffSessionBefore")}
              metrics={drill.row.before}
              absent={resolveDiffChange(drill.row.change) === "added"}
              t={t}
            />
            <SideMetricsPanel
              label={t("httpCaptureDiffSessionAfter")}
              metrics={drill.row.after}
              absent={resolveDiffChange(drill.row.change) === "removed"}
              t={t}
            />
          </div>
          <p className="text-[11px] text-muted-foreground">{t("httpCaptureDiffDetailSamplesNote")}</p>
        </div>
      )}
    </SlideOverPanel>
  );
}

function SideMetricsPanel({
  label,
  metrics,
  absent,
  t,
}: {
  label: string;
  metrics: HttpCaptureDiffSideMetrics;
  absent: boolean;
  t: Translate;
}): React.JSX.Element {
  if (absent) {
    return (
      <div className="rounded-md border border-border bg-muted/20 p-3 text-xs">
        <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</p>
        <p className="mt-2 text-muted-foreground">{t("httpCaptureDiffSideAbsent")}</p>
      </div>
    );
  }
  return (
    <div className="rounded-md border border-border bg-muted/20 p-3">
      <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">{label}</p>
      <dl className="mt-2 grid grid-cols-2 gap-x-2 gap-y-1 text-xs tabular-nums">
        <DetailField label={t("httpCaptureDiffMetricCount")} value={formatNumber(metrics.count)} />
        <DetailField label={t("httpCaptureDiffMetricErrors")} value={formatNumber(metrics.errors)} />
        <DetailField
          label={t("httpCaptureDiffMetricErrorRate")}
          value={`${explicitRateText(metrics.error_rate)} (${pctText(metrics.error_rate.value)})`}
        />
        <DetailField
          label={t("httpCaptureDiffTrafficShare")}
          value={`${explicitRateText(metrics.traffic_share)} (${pctText(metrics.traffic_share.value)})`}
        />
        <DetailField
          label={t("httpCaptureDiffMetricPerMinute")}
          value={
            metrics.count_per_minute
              ? `${metrics.count_per_minute.value_per_minute.toFixed(2)} (${formatNumber(metrics.count_per_minute.numerator)} / ${metrics.count_per_minute.denominator_minutes.toFixed(2)} min)`
              : rateUnavailableText(metrics, t)
          }
        />
        <DetailField
          label={t("httpCaptureDiffDurationSamples")}
          value={formatNumber(metrics.duration_samples)}
        />
        <DetailField
          label={t("httpCaptureDiffMetricP50")}
          value={formatMilliseconds(Math.round(metrics.duration_p50_ms))}
        />
        <DetailField
          label={t("httpCaptureDiffMetricP95")}
          value={formatMilliseconds(Math.round(metrics.duration_p95_ms))}
        />
        <DetailField
          label={t("httpCaptureDiffMetricP99")}
          value={formatMilliseconds(Math.round(metrics.duration_p99_ms))}
        />
        <DetailField
          label={t("httpCaptureDiffMetricRequestBytes")}
          value={formatBytes(metrics.request_bytes)}
        />
        <DetailField
          label={t("httpCaptureDiffMetricResponseBytes")}
          value={formatBytes(metrics.response_bytes)}
        />
      </dl>
    </div>
  );
}

function DetailField({ label, value }: { label: string; value: string }): React.JSX.Element {
  return (
    <>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right font-mono">{value}</dd>
    </>
  );
}

// ── Formatting helpers (presentation only) ──────────────────────────

function explicitRateText(rate: { numerator: number; denominator: number }): string {
  return `${formatNumber(rate.numerator)}/${formatNumber(rate.denominator)}`;
}

function pctText(value: number): string {
  return `${(value * 100).toFixed(1)}%`;
}

function signedNumber(value: number): string {
  return `${value > 0 ? "+" : ""}${formatNumber(value)}`;
}

function signedPct(value: number): string {
  return `${value > 0 ? "+" : ""}${(value * 100).toFixed(1)}%p`;
}

function signedMs(value: number): string {
  return `${value > 0 ? "+" : ""}${value.toFixed(1)}ms`;
}

function signedFixed(value: number): string {
  return `${value > 0 ? "+" : ""}${value.toFixed(2)}`;
}

function signedBytes(value: number): string {
  const magnitude = formatBytes(Math.abs(value));
  return value < 0 ? `-${magnitude}` : `+${magnitude}`;
}

function perMinuteText(
  metrics: HttpCaptureDiffSideMetrics,
  allowed: boolean,
  t: Translate,
): string {
  if (!allowed || !metrics.count_per_minute) {
    return rateUnavailableText(metrics, t);
  }
  return metrics.count_per_minute.value_per_minute.toFixed(2);
}

function rateUnavailableText(metrics: HttpCaptureDiffSideMetrics, t: Translate): string {
  return t(
    DIFF_RATE_UNAVAILABLE_LABEL_KEYS[
      resolveDiffRateUnavailable(metrics.rate_unavailable_code ?? "")
    ],
  );
}

/** One-sided rows print `—` for the side that has no data, not `0`. */
function sideText(change: string, absentWhen: string, text: string): string {
  return change === absentWhen ? "—" : text;
}
