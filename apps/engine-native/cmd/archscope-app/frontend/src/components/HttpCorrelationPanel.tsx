// ─────────────────────────────────────────────────────────────────────
// [한글] HttpCorrelationPanel.tsx — HTTP 증거 교차 분석(X-RG1 / T-583)
// 패널. HttpCapturePage 와 Analysis Workspace 양쪽에서 마운트되는 공유
// 표면으로, state/httpCorrelation.ts 의 module store 로 선택/결과를
// 공유한다.
//
// 책임/목적:
//   - Workspace 의 http_capture(필수) + profile_evidence / jennifer_profile /
//     access_log(선택, 최소 1개) 결과를 골라 `AnalyzeHttpEvidenceCorrelation`
//     을 실행.
//   - V8 프로파일 오버레이는 명시적 RFC3339 wall-clock anchor 없이는
//     backend 가 `none` 등급으로 fail-closed 하므로, anchor 입력과 그 의미를
//     실행 전에 공지한다.
//   - 결과 렌더: per-source 정렬 진단(등급/신뢰도/overlay 허용/사유/행
//     경계), CPU 겹침·Jennifer network-gap·access-log 매치 테이블, findings,
//     행 클릭 드릴다운. 시간 오버레이는 해당 source 진단의
//     `overlay_allowed`+`aligned` 판정이 있을 때만 렌더한다.
//   - 인과관계 금지: 모든 표면에 "시간 상관이며 인과 증거가 아니다" 공지를
//     유지한다 (X-RG1 리뷰 기준).
//
// 의존성 주의: 파생 로직은 전부 state/httpCorrelation.ts 순수 함수 사용.
// ─────────────────────────────────────────────────────────────────────
import { Link2, Loader2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { engine } from "@/bridge/engine";
import type {
  AnalysisResult,
  HttpCaptureAnalysisResult,
  HttpCorrelationAccessRow,
  HttpCorrelationJenniferRow,
  HttpCorrelationProfileOverlapRow,
  HttpCorrelationSourceDiagnostic,
} from "@/bridge/types";
import { ErrorPanel } from "@/components/AnalyzerFeedback";
import { SlideOverPanel } from "@/components/SlideOverPanel";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import {
  getWorkspaceEntry,
  useAnalysisWorkspace,
  type AnalysisWorkspaceEntry,
} from "@/state/analysisWorkspace";
import {
  CORRELATION_ALIGNMENT_LABEL_KEYS,
  CORRELATION_CONFIDENCE_LABEL_KEYS,
  CORRELATION_MATCH_BASIS_LABEL_KEYS,
  CORRELATION_SOURCE_LABEL_KEYS,
  correlationAnchorState,
  correlationCandidates,
  correlationInputsOf,
  correlationOverlayAllowed,
  diagnosticForSource,
  dispatchHttpCorrelation,
  extractCorrelationEnvelope,
  hasCorrelationSecondary,
  resolveCorrelationAlignment,
  resolveCorrelationConfidence,
  resolveCorrelationMatchBasis,
  resolveCorrelationSource,
  selectCorrelationDiagnostics,
  selectCorrelationFindings,
  selectCorrelationSummary,
  selectCorrelationTable,
  useHttpCorrelation,
  type CorrelationSlot,
} from "@/state/httpCorrelation";
import { formatMilliseconds, formatNumber } from "@/utils/formatters";

type Translate = (key: MessageKey) => string;

type DrillState =
  | { kind: "profile"; row: HttpCorrelationProfileOverlapRow; overlay: boolean }
  | { kind: "jennifer"; row: HttpCorrelationJenniferRow }
  | { kind: "access"; row: HttpCorrelationAccessRow }
  | null;

export function HttpCorrelationPanel(): React.JSX.Element {
  const { t } = useI18n();
  const workspace = useAnalysisWorkspace();
  const state = useHttpCorrelation();
  const [drill, setDrill] = useState<DrillState>(null);

  useEffect(() => {
    if (state.contract !== null || state.contractMismatch) return;
    let cancelled = false;
    engine
      .getHttpEvidenceCorrelationContract()
      .then((contract) => {
        if (!cancelled) dispatchHttpCorrelation({ type: "contractLoaded", contract });
      })
      .catch(() => {
        if (!cancelled) dispatchHttpCorrelation({ type: "contractUnavailable" });
      });
    return () => {
      cancelled = true;
    };
  }, [state.contract, state.contractMismatch]);

  const httpCandidates = useMemo(
    () => correlationCandidates(workspace.entries, "http"),
    [workspace.entries],
  );
  const profileCandidates = useMemo(
    () => correlationCandidates(workspace.entries, "profile"),
    [workspace.entries],
  );
  const jenniferCandidates = useMemo(
    () => correlationCandidates(workspace.entries, "jennifer"),
    [workspace.entries],
  );
  const accessCandidates = useMemo(
    () => correlationCandidates(workspace.entries, "accessLog"),
    [workspace.entries],
  );

  const inputs = correlationInputsOf(state);
  const anchorState = correlationAnchorState(state.anchor);
  const canRun =
    state.contractSupported &&
    !state.running &&
    !!state.httpId &&
    hasCorrelationSecondary(inputs) &&
    anchorState !== "invalid";

  const run = useCallback(async () => {
    const http = getWorkspaceEntry(state.httpId);
    if (!http || state.running) return;
    const profile = getWorkspaceEntry(state.profileId);
    const jennifer = getWorkspaceEntry(state.jenniferId);
    const accessLog = getWorkspaceEntry(state.accessLogId);
    const snapshot = correlationInputsOf(state);
    dispatchHttpCorrelation({ type: "runStart" });
    try {
      const result = await engine.analyzeHttpEvidenceCorrelation({
        http: http.result as unknown as HttpCaptureAnalysisResult,
        profile: profile ? (profile.result as unknown as AnalysisResult) : undefined,
        jennifer: jennifer ? (jennifer.result as unknown as AnalysisResult) : undefined,
        accessLog: accessLog ? (accessLog.result as unknown as AnalysisResult) : undefined,
        timeToleranceMs: snapshot.toleranceMs ?? undefined,
        profileWallClockStart: snapshot.anchor.trim() || undefined,
      });
      dispatchHttpCorrelation({ type: "runSuccess", result, inputs: snapshot });
    } catch (caught) {
      dispatchHttpCorrelation({
        type: "runError",
        error: {
          code: "HTTP_CORRELATION_FAILED",
          message: caught instanceof Error ? caught.message : String(caught),
        },
      });
    }
  }, [state]);

  const summary = useMemo(() => selectCorrelationSummary(state.result), [state.result]);
  const envelope = useMemo(() => extractCorrelationEnvelope(state.result), [state.result]);
  const diagnostics = useMemo(() => selectCorrelationDiagnostics(state.result), [state.result]);
  const findings = useMemo(() => selectCorrelationFindings(state.result), [state.result]);
  const profileDiag = diagnosticForSource(diagnostics, "profile_evidence");
  const jenniferDiag = diagnosticForSource(diagnostics, "jennifer_profile");
  const accessDiag = diagnosticForSource(diagnostics, "access_log");
  const profileOverlayAllowed = correlationOverlayAllowed(profileDiag);
  const profileRows = useMemo(
    () =>
      selectCorrelationTable<HttpCorrelationProfileOverlapRow>(state.result, "http_profile_overlaps"),
    [state.result],
  );
  const jenniferRows = useMemo(
    () =>
      selectCorrelationTable<HttpCorrelationJenniferRow>(state.result, "jennifer_network_gap_checks"),
    [state.result],
  );
  const accessRows = useMemo(
    () => selectCorrelationTable<HttpCorrelationAccessRow>(state.result, "access_log_matches"),
    [state.result],
  );

  return (
    <section className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle className="inline-flex items-center gap-2">
            <Link2 className="nav-lucide-sm" />
            {t("httpCorrelationTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-xs text-muted-foreground">{t("httpCorrelationDescription")}</p>
          {state.contractMismatch && (
            <p
              className="rounded-md border border-amber-500/50 bg-amber-500/10 p-2 text-xs text-amber-700 dark:text-amber-400"
              role="status"
            >
              {t("httpCorrelationContractMismatch")}
            </p>
          )}
          {httpCandidates.length === 0 ? (
            <p className="text-sm text-muted-foreground" role="status">
              {t("httpCorrelationNoHttp")}
            </p>
          ) : (
            <>
              <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
                <SlotSelect
                  slot="http"
                  label={t("httpCorrelationSlotHttp")}
                  required
                  value={state.httpId}
                  candidates={httpCandidates}
                  t={t}
                />
                <SlotSelect
                  slot="profile"
                  label={t("httpCorrelationSlotProfile")}
                  value={state.profileId}
                  candidates={profileCandidates}
                  t={t}
                />
                <SlotSelect
                  slot="jennifer"
                  label={t("httpCorrelationSlotJennifer")}
                  value={state.jenniferId}
                  candidates={jenniferCandidates}
                  t={t}
                />
                <SlotSelect
                  slot="accessLog"
                  label={t("httpCorrelationSlotAccessLog")}
                  value={state.accessLogId}
                  candidates={accessCandidates}
                  t={t}
                />
              </div>
              <div className="flex flex-wrap items-end gap-2">
                {state.profileId && (
                  <div className="flex min-w-64 flex-1 flex-col gap-1">
                    <label className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                      {t("httpCorrelationAnchorLabel")}
                      <Input
                        className="mt-1 font-mono text-xs"
                        value={state.anchor}
                        onChange={(event) =>
                          dispatchHttpCorrelation({ type: "setAnchor", anchor: event.target.value })
                        }
                        placeholder="2026-07-30T12:00:00.000Z"
                        aria-label={t("httpCorrelationAnchorLabel")}
                      />
                    </label>
                    <p
                      className={
                        anchorState === "invalid"
                          ? "text-[11px] text-amber-700 dark:text-amber-400"
                          : "text-[11px] text-muted-foreground"
                      }
                      role={anchorState === "invalid" ? "status" : undefined}
                    >
                      {anchorState === "invalid"
                        ? t("httpCorrelationAnchorInvalid")
                        : t("httpCorrelationAnchorHint")}
                    </p>
                  </div>
                )}
                <label className="flex flex-col gap-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                  {t("httpCorrelationToleranceLabel")}
                  <input
                    type="number"
                    min={1}
                    max={60000}
                    inputMode="numeric"
                    className="h-9 w-28 rounded-md border border-input bg-transparent px-2 text-sm normal-case tracking-normal focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                    value={state.toleranceMs ?? ""}
                    onChange={(event) =>
                      dispatchHttpCorrelation({
                        type: "setTolerance",
                        toleranceMs: event.target.value === "" ? null : Number(event.target.value),
                      })
                    }
                    placeholder="1000"
                    aria-label={t("httpCorrelationToleranceLabel")}
                  />
                </label>
                <Button type="button" size="sm" disabled={!canRun} onClick={() => void run()}>
                  {state.running ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      {t("analyzing")}
                    </>
                  ) : (
                    t("httpCorrelationRun")
                  )}
                </Button>
              </div>
              {!hasCorrelationSecondary(inputs) && (
                <p className="text-xs text-muted-foreground">{t("httpCorrelationNeedSecondary")}</p>
              )}
            </>
          )}
        </CardContent>
      </Card>

      <ErrorPanel
        error={state.error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />

      {state.result && summary && (
        <>
          <p
            className="rounded-md border border-sky-500/40 bg-sky-500/10 p-2 text-xs text-sky-800 dark:text-sky-300"
            role="note"
          >
            {t("httpCorrelationNoCausalityNote")}
          </p>

          <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-7">
            <Tile label={t("httpCorrelationMetricHttpRows")} value={formatNumber(summary.http_transaction_rows)} />
            <Tile label={t("httpCorrelationMetricProfileOverlaps")} value={formatNumber(summary.profile_overlap_count)} />
            <Tile label={t("httpCorrelationMetricJenniferChecks")} value={formatNumber(summary.jennifer_check_count)} />
            <Tile label={t("httpCorrelationMetricAccessMatches")} value={formatNumber(summary.access_log_match_count)} />
            <Tile label={t("httpCorrelationMetricAligned")} value={formatNumber(summary.aligned_source_count)} />
            <Tile label={t("httpCorrelationMetricDurationOnly")} value={formatNumber(summary.duration_only_source_count)} />
            <Tile label={t("httpCorrelationMetricIncompatible")} value={formatNumber(summary.incompatible_source_count)} />
          </div>

          <DiagnosticsCard diagnostics={diagnostics} t={t} />

          {profileDiag && (
            <ProfileOverlapCard
              rows={profileRows}
              diagnostic={profileDiag}
              overlayAllowed={profileOverlayAllowed}
              onOpen={(row) => setDrill({ kind: "profile", row, overlay: profileOverlayAllowed })}
              t={t}
            />
          )}
          {jenniferDiag && (
            <JenniferCard
              rows={jenniferRows}
              diagnostic={jenniferDiag}
              onOpen={(row) => setDrill({ kind: "jennifer", row })}
              t={t}
            />
          )}
          {accessDiag && (
            <AccessCard
              rows={accessRows}
              diagnostic={accessDiag}
              onOpen={(row) => setDrill({ kind: "access", row })}
              t={t}
            />
          )}

          {findings.length > 0 && (
            <Card>
              <CardHeader>
                <CardTitle>{t("httpCorrelationFindingsTitle")}</CardTitle>
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
              {t("httpCorrelationBoundsNote")
                .replace("{n}", formatNumber(envelope.top_n))
                .replace("{tol}", formatNumber(envelope.time_tolerance_ms))}
            </p>
          )}
        </>
      )}

      <CorrelationDetail drill={drill} onClose={() => setDrill(null)} t={t} />
    </section>
  );
}

// ── Selection ───────────────────────────────────────────────────────

function SlotSelect({
  slot,
  label,
  required,
  value,
  candidates,
  t,
}: {
  slot: CorrelationSlot;
  label: string;
  required?: boolean;
  value: string | null;
  candidates: AnalysisWorkspaceEntry[];
  t: Translate;
}): React.JSX.Element {
  return (
    <label className="flex min-w-0 flex-col gap-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
      <span>
        {label}
        {required ? " *" : ""}
      </span>
      <select
        className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm normal-case tracking-normal focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
        value={value ?? ""}
        onChange={(event) =>
          dispatchHttpCorrelation({ type: "setSlot", slot, id: event.target.value || null })
        }
        aria-label={label}
      >
        <option value="">
          {candidates.length === 0
            ? t("httpCorrelationSlotEmpty")
            : t("httpCorrelationSlotPlaceholder")}
        </option>
        {candidates.map((entry) => (
          <option key={entry.id} value={entry.id}>
            {entry.title} · {new Date(entry.recorded_at).toLocaleString()}
          </option>
        ))}
      </select>
    </label>
  );
}

function Tile({ label, value }: { label: string; value: string }): React.JSX.Element {
  return (
    <div className="rounded-md border border-border bg-muted/20 p-2">
      <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">{label}</p>
      <p className="mt-0.5 text-sm font-semibold tabular-nums">{value}</p>
    </div>
  );
}

// ── Alignment diagnostics ───────────────────────────────────────────

function alignmentTone(grade: string): string {
  switch (resolveCorrelationAlignment(grade)) {
    case "aligned":
      return "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400";
    case "duration_only":
      return "bg-amber-500/15 text-amber-700 dark:text-amber-400";
    default:
      return "bg-red-500/15 text-red-700 dark:text-red-400";
  }
}

function DiagnosticsCard({
  diagnostics,
  t,
}: {
  diagnostics: HttpCorrelationSourceDiagnostic[];
  t: Translate;
}): React.JSX.Element | null {
  if (diagnostics.length === 0) return null;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("httpCorrelationDiagnosticsTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        {diagnostics.map((row) => {
          const grade = resolveCorrelationAlignment(row.alignment_grade);
          const overlayOK = correlationOverlayAllowed(row);
          return (
            <div key={row.source} className="rounded-md border border-border bg-muted/20 p-2 text-xs">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium" title={row.source}>
                  {t(CORRELATION_SOURCE_LABEL_KEYS[resolveCorrelationSource(row.source)])}
                </span>
                <span className={`rounded-full px-2 py-0.5 ${alignmentTone(row.alignment_grade)}`} title={row.alignment_grade}>
                  {t(CORRELATION_ALIGNMENT_LABEL_KEYS[grade])}
                </span>
                <span className="text-muted-foreground" title={row.confidence}>
                  {t("httpCorrelationConfidenceLabel")}:{" "}
                  {t(CORRELATION_CONFIDENCE_LABEL_KEYS[resolveCorrelationConfidence(row.confidence)])}
                </span>
                <span
                  className={
                    overlayOK
                      ? "text-emerald-700 dark:text-emerald-400"
                      : "text-amber-700 dark:text-amber-400"
                  }
                >
                  {overlayOK
                    ? t("httpCorrelationOverlayEnabled")
                    : t("httpCorrelationOverlaySuppressed")}
                </span>
              </div>
              <p className="mt-1 text-muted-foreground">{row.reason}</p>
              <p className="mt-1 font-mono text-[11px] text-muted-foreground">
                {t("httpCorrelationRowsUsed")}: {formatNumber(row.rows_used)}/{formatNumber(row.input_rows)}
                {row.source !== "http_capture" &&
                  ` · ${t("httpCorrelationOutputRows")}: ${formatNumber(row.output_rows)}`}
                {row.source_truncated && ` · ${t("httpCorrelationSourceTruncated")}`}
                {row.output_truncated && ` · ${t("httpCorrelationOutputTruncated")}`}
              </p>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}

// ── Match tables ────────────────────────────────────────────────────

function MatchTableShell({
  title,
  count,
  emptyText,
  children,
}: {
  title: string;
  count: number;
  emptyText: string;
  children: React.ReactNode;
}): React.JSX.Element {
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2">
        <CardTitle>{title}</CardTitle>
        <span className="tabular-nums text-xs text-muted-foreground">{formatNumber(count)}</span>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        {count === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground" role="status">
            {emptyText}
          </p>
        ) : (
          children
        )}
      </CardContent>
    </Card>
  );
}

const rowClass =
  "cursor-pointer border-t hover:bg-accent focus-visible:bg-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring";

function rowInteraction(onOpen: () => void, label: string) {
  return {
    role: "button" as const,
    tabIndex: 0,
    "aria-label": label,
    onClick: onOpen,
    onKeyDown: (event: React.KeyboardEvent) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        onOpen();
      }
    },
  };
}

function ProfileOverlapCard({
  rows,
  diagnostic,
  overlayAllowed,
  onOpen,
  t,
}: {
  rows: HttpCorrelationProfileOverlapRow[];
  diagnostic: HttpCorrelationSourceDiagnostic;
  overlayAllowed: boolean;
  onOpen: (row: HttpCorrelationProfileOverlapRow) => void;
  t: Translate;
}): React.JSX.Element {
  return (
    <MatchTableShell
      title={t("httpCorrelationProfileTitle")}
      count={rows.length}
      emptyText={
        resolveCorrelationAlignment(diagnostic.alignment_grade) === "none"
          ? `${t("httpCorrelationProfileSuppressed")} ${diagnostic.reason}`
          : t("httpCorrelationNoMatches")
      }
    >
      <table className="w-full text-left text-xs">
        <caption className="sr-only">{t("httpCorrelationProfileTitle")}</caption>
        <thead>
          <tr className="border-b border-border text-muted-foreground">
            <th scope="col" className="p-2 font-medium">{t("httpCorrelationColEndpoint")}</th>
            <th scope="col" className="p-2 font-medium">{t("httpCorrelationColStack")}</th>
            <th scope="col" className="p-2 text-right font-medium">{t("httpCorrelationColOverlapMs")}</th>
            <th scope="col" className="p-2 text-right font-medium">{t("httpCorrelationColOverlapRatio")}</th>
            <th scope="col" className="p-2 font-medium">{t("httpCorrelationColConfidence")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr
              key={`${row.http_id}-${index}`}
              className={rowClass}
              {...rowInteraction(() => onOpen(row), `${t("httpCorrelationRowOpen")}: ${row.endpoint}`)}
            >
              <td className="max-w-xs truncate p-2 font-mono" title={row.endpoint}>{row.endpoint}</td>
              <td className="max-w-xs truncate p-2 font-mono" title={row.cpu_stack}>{row.cpu_stack}</td>
              <td className="p-2 text-right tabular-nums">{row.overlap_ms.toFixed(1)}</td>
              <td className="p-2 text-right tabular-nums">{(row.overlap_ratio * 100).toFixed(1)}%</td>
              <td className="p-2" title={row.confidence}>
                {t(CORRELATION_CONFIDENCE_LABEL_KEYS[resolveCorrelationConfidence(row.confidence)])}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {!overlayAllowed && rows.length > 0 && (
        <p className="mt-2 text-[11px] text-amber-700 dark:text-amber-400" role="status">
          {t("httpCorrelationOverlaySuppressed")}
        </p>
      )}
    </MatchTableShell>
  );
}

function JenniferCard({
  rows,
  diagnostic,
  onOpen,
  t,
}: {
  rows: HttpCorrelationJenniferRow[];
  diagnostic: HttpCorrelationSourceDiagnostic;
  onOpen: (row: HttpCorrelationJenniferRow) => void;
  t: Translate;
}): React.JSX.Element {
  void diagnostic;
  return (
    <MatchTableShell
      title={t("httpCorrelationJenniferTitle")}
      count={rows.length}
      emptyText={t("httpCorrelationNoMatches")}
    >
      <p className="mb-2 text-[11px] text-amber-700 dark:text-amber-400">
        {t("httpCorrelationJenniferDurationOnlyNote")}
      </p>
      <table className="w-full text-left text-xs">
        <caption className="sr-only">{t("httpCorrelationJenniferTitle")}</caption>
        <thead>
          <tr className="border-b border-border text-muted-foreground">
            <th scope="col" className="p-2 font-medium">{t("httpCorrelationColEndpoint")}</th>
            <th scope="col" className="p-2 font-medium">{t("httpCorrelationColTargetHost")}</th>
            <th scope="col" className="p-2 text-right font-medium">{t("httpCorrelationColDurations")}</th>
            <th scope="col" className="p-2 text-right font-medium">{t("httpCorrelationColGapDelta")}</th>
            <th scope="col" className="p-2 font-medium">{t("httpCorrelationColConfidence")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr
              key={`${row.http_id}-${index}`}
              className={rowClass}
              {...rowInteraction(() => onOpen(row), `${t("httpCorrelationRowOpen")}: ${row.endpoint}`)}
            >
              <td className="max-w-xs truncate p-2 font-mono" title={row.endpoint}>{row.endpoint}</td>
              <td className="max-w-40 truncate p-2 font-mono" title={row.target_host}>{row.target_host}</td>
              <td className="p-2 text-right tabular-nums">
                {formatMilliseconds(Math.round(row.external_call_elapsed_ms))} ↔{" "}
                {formatMilliseconds(Math.round(row.http_duration_ms))}
              </td>
              <td className="p-2 text-right tabular-nums">
                {typeof row.network_gap_delta_ms === "number"
                  ? `${row.network_gap_delta_ms > 0 ? "+" : ""}${row.network_gap_delta_ms.toFixed(1)}ms`
                  : "—"}
              </td>
              <td className="p-2" title={row.confidence}>
                {t(CORRELATION_CONFIDENCE_LABEL_KEYS[resolveCorrelationConfidence(row.confidence)])}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </MatchTableShell>
  );
}

function AccessCard({
  rows,
  diagnostic,
  onOpen,
  t,
}: {
  rows: HttpCorrelationAccessRow[];
  diagnostic: HttpCorrelationSourceDiagnostic;
  onOpen: (row: HttpCorrelationAccessRow) => void;
  t: Translate;
}): React.JSX.Element {
  void diagnostic;
  return (
    <MatchTableShell
      title={t("httpCorrelationAccessTitle")}
      count={rows.length}
      emptyText={t("httpCorrelationNoMatches")}
    >
      <table className="w-full text-left text-xs">
        <caption className="sr-only">{t("httpCorrelationAccessTitle")}</caption>
        <thead>
          <tr className="border-b border-border text-muted-foreground">
            <th scope="col" className="p-2 font-medium">{t("httpCorrelationColEndpoint")}</th>
            <th scope="col" className="p-2 text-right font-medium">{t("httpCorrelationColClientServer")}</th>
            <th scope="col" className="p-2 text-right font-medium">{t("httpCorrelationColOutsideServer")}</th>
            <th scope="col" className="p-2 font-medium">{t("httpCorrelationColBasis")}</th>
            <th scope="col" className="p-2 font-medium">{t("httpCorrelationColConfidence")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr
              key={`${row.http_id}-${index}`}
              className={rowClass}
              {...rowInteraction(() => onOpen(row), `${t("httpCorrelationRowOpen")}: ${row.endpoint}`)}
            >
              <td className="max-w-xs truncate p-2 font-mono" title={row.endpoint}>{row.endpoint}</td>
              <td className="p-2 text-right tabular-nums">
                {formatMilliseconds(Math.round(row.http_duration_ms))} ↔{" "}
                {formatMilliseconds(Math.round(row.server_response_ms))}
              </td>
              <td className="p-2 text-right tabular-nums">
                {row.outside_server_ms > 0 ? "+" : ""}
                {row.outside_server_ms.toFixed(1)}ms
              </td>
              <td className="p-2" title={row.match_basis}>
                {t(CORRELATION_MATCH_BASIS_LABEL_KEYS[resolveCorrelationMatchBasis(row.match_basis)])}
              </td>
              <td className="p-2" title={row.confidence}>
                {t(CORRELATION_CONFIDENCE_LABEL_KEYS[resolveCorrelationConfidence(row.confidence)])}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </MatchTableShell>
  );
}

// ── Drilldown ───────────────────────────────────────────────────────

function CorrelationDetail({
  drill,
  onClose,
  t,
}: {
  drill: DrillState;
  onClose: () => void;
  t: Translate;
}): React.JSX.Element {
  return (
    <SlideOverPanel
      open={drill !== null}
      onClose={onClose}
      title={drill ? drill.row.endpoint : t("httpCorrelationDetailTitle")}
      width={560}
    >
      {drill && (
        <div className="flex flex-col gap-4 text-sm">
          <p
            className="rounded-md border border-sky-500/40 bg-sky-500/10 p-2 text-[11px] text-sky-800 dark:text-sky-300"
            role="note"
          >
            {t("httpCorrelationNoCausalityNote")}
          </p>
          <dl className="grid grid-cols-2 gap-x-2 gap-y-1 text-xs tabular-nums">
            <DetailField label={t("httpCorrelationColBasis")} value={basisLabel(drill.row.match_basis, t)} title={drill.row.match_basis} />
            <DetailField
              label={t("httpCorrelationColConfidence")}
              value={t(CORRELATION_CONFIDENCE_LABEL_KEYS[resolveCorrelationConfidence(drill.row.confidence)])}
              title={drill.row.confidence}
            />
            <DetailField
              label={t("httpCorrelationDetailAlignment")}
              value={t(CORRELATION_ALIGNMENT_LABEL_KEYS[resolveCorrelationAlignment(drill.row.alignment_grade)])}
              title={drill.row.alignment_grade}
            />
            {drill.kind === "profile" && (
              <>
                <DetailField label={t("httpCorrelationColOverlapMs")} value={`${drill.row.overlap_ms.toFixed(1)}ms`} />
                <DetailField label={t("httpCorrelationColOverlapRatio")} value={`${(drill.row.overlap_ratio * 100).toFixed(1)}%`} />
                <DetailField label={t("httpCorrelationColStack")} value={drill.row.cpu_stack} wide />
              </>
            )}
            {drill.kind === "jennifer" && (
              <>
                <DetailField label={t("httpCorrelationColTargetHost")} value={drill.row.target_host} />
                <DetailField
                  label={t("httpCorrelationColDurations")}
                  value={`${drill.row.external_call_elapsed_ms.toFixed(1)}ms ↔ ${drill.row.http_duration_ms.toFixed(1)}ms (Δ ${drill.row.duration_delta_ms.toFixed(1)}ms)`}
                />
                <DetailField
                  label={t("httpCorrelationDetailJenniferGap")}
                  value={`${drill.row.jennifer_network_gap_ms.toFixed(1)}ms`}
                />
                <DetailField
                  label={t("httpCorrelationDetailObservedNetwork")}
                  value={
                    typeof drill.row.http_observed_network_ms === "number"
                      ? `${drill.row.http_observed_network_ms.toFixed(1)}ms (Δ ${(drill.row.network_gap_delta_ms ?? 0).toFixed(1)}ms)`
                      : drill.row.network_gap_unavailable_reason || t("httpCorrelationDetailObservedUnavailable")
                  }
                  wide
                />
              </>
            )}
            {drill.kind === "access" && (
              <>
                <DetailField label={t("httpCorrelationDetailAccessUri")} value={drill.row.access_uri} wide />
                <DetailField
                  label={t("httpCorrelationDetailRequestId")}
                  value={drill.row.request_id || "—"}
                />
                <DetailField
                  label={t("httpCorrelationColClientServer")}
                  value={`${drill.row.http_duration_ms.toFixed(1)}ms ↔ ${drill.row.server_response_ms.toFixed(1)}ms`}
                />
                <DetailField
                  label={t("httpCorrelationColOutsideServer")}
                  value={`${drill.row.outside_server_ms.toFixed(1)}ms`}
                />
                <DetailField
                  label={t("httpCorrelationDetailTimestampDelta")}
                  value={`${drill.row.timestamp_delta_ms.toFixed(1)}ms`}
                />
              </>
            )}
          </dl>
          {drill.kind === "profile" && (
            <TimeOverlapBlock
              overlayAllowed={drill.overlay}
              httpStart={drill.row.http_started_at}
              httpEnd={drill.row.http_ended_at}
              cpuStart={drill.row.cpu_started_at}
              cpuEnd={drill.row.cpu_ended_at}
              t={t}
            />
          )}
        </div>
      )}
    </SlideOverPanel>
  );
}

function basisLabel(value: string, t: Translate): string {
  return t(CORRELATION_MATCH_BASIS_LABEL_KEYS[resolveCorrelationMatchBasis(value)]);
}

function DetailField({
  label,
  value,
  title,
  wide,
}: {
  label: string;
  value: string;
  title?: string;
  wide?: boolean;
}): React.JSX.Element {
  return (
    <>
      <dt className={wide ? "col-span-2 text-muted-foreground" : "text-muted-foreground"}>{label}</dt>
      <dd
        className={
          wide ? "col-span-2 break-all font-mono" : "break-all text-right font-mono"
        }
        title={title ?? value}
      >
        {value}
      </dd>
    </>
  );
}

/**
 * Time-window overlay for one HTTP transaction and one CPU run. Renders only
 * when the profile source diagnostic certified an aligned overlay — otherwise
 * the suppression note is shown instead of a chart that would imply
 * comparable clocks.
 */
function TimeOverlapBlock({
  overlayAllowed,
  httpStart,
  httpEnd,
  cpuStart,
  cpuEnd,
  t,
}: {
  overlayAllowed: boolean;
  httpStart: string;
  httpEnd: string;
  cpuStart: string;
  cpuEnd: string;
  t: Translate;
}): React.JSX.Element {
  if (!overlayAllowed) {
    return (
      <p className="text-[11px] text-amber-700 dark:text-amber-400" role="status">
        {t("httpCorrelationOverlaySuppressed")}
      </p>
    );
  }
  const hs = Date.parse(httpStart);
  const he = Date.parse(httpEnd);
  const cs = Date.parse(cpuStart);
  const ce = Date.parse(cpuEnd);
  if ([hs, he, cs, ce].some(Number.isNaN) || he <= hs) {
    return (
      <p className="text-[11px] text-muted-foreground">{t("httpCorrelationOverlayUnavailable")}</p>
    );
  }
  const min = Math.min(hs, cs);
  const max = Math.max(he, ce);
  const span = Math.max(1, max - min);
  const pct = (from: number, to: number) => ({
    left: `${((from - min) / span) * 100}%`,
    width: `${Math.max(1, ((to - from) / span) * 100)}%`,
  });
  return (
    <div className="flex flex-col gap-1">
      <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {t("httpCorrelationOverlayTitle")}
      </p>
      <div className="flex items-center gap-2 text-[10px]">
        <span className="w-10 shrink-0 text-muted-foreground">HTTP</span>
        <div className="relative h-2.5 flex-1 rounded-sm bg-muted">
          <div className="absolute inset-y-0 rounded-sm bg-primary/70" style={pct(hs, he)} />
        </div>
      </div>
      <div className="flex items-center gap-2 text-[10px]">
        <span className="w-10 shrink-0 text-muted-foreground">CPU</span>
        <div className="relative h-2.5 flex-1 rounded-sm bg-muted">
          <div className="absolute inset-y-0 rounded-sm bg-amber-500/70" style={pct(cs, ce)} />
        </div>
      </div>
      <p className="font-mono text-[10px] text-muted-foreground">
        {httpStart} — {httpEnd}
      </p>
    </div>
  );
}
