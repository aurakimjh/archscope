import { useState } from "react";

import type { ParserDiagnostics, DiagnosticSample } from "../../bindings/github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler/models";
import { ProfilerService, AnalyzeRequest } from "../../bindings/github.com/aurakimjh/archscope/apps/profiler-native/cmd/archscope-profiler-app";
import { useI18n } from "../i18n/I18nProvider";

export type DiagnosticsPanelProps = {
  diagnostics?: ParserDiagnostics | null;
  baseRequest?: AnalyzeRequest | null;
};

export function DiagnosticsPanel({ diagnostics, baseRequest }: DiagnosticsPanelProps) {
  const { t } = useI18n();
  const [saving, setSaving] = useState(false);
  const [savedNotice, setSavedNotice] = useState<string>("");
  const [error, setError] = useState<string>("");

  const handleSaveDebugLog = async () => {
    if (!baseRequest) return;
    setSaving(true);
    setSavedNotice("");
    setError("");
    try {
      const result = await ProfilerService.WriteDebugLog(
        new AnalyzeRequest({ ...baseRequest, debugLog: true } as any),
      );
      if (!result.outputPath) {
        setSavedNotice(t("debugLogClean"));
      } else {
        setSavedNotice(`${t("debugLogSaved")} ${result.outputPath} (${t("debugLogVerdict")}: ${result.verdict})`);
      }
    } catch (err: any) {
      setError(String(err?.message ?? err));
    } finally {
      setSaving(false);
    }
  };
  if (!diagnostics) {
    return <p className="muted">{t("diagnosticsEmpty")}</p>;
  }

  const reasons = Object.entries(diagnostics.skipped_by_reason ?? {}) as [string, number][];
  const samples = (diagnostics.samples ?? []) as DiagnosticSample[];
  const warnings = (diagnostics.warnings ?? []) as DiagnosticSample[];
  const errors = (diagnostics.errors ?? []) as DiagnosticSample[];

  return (
    <div className="diagnostics-detail">
      {baseRequest && (
        <div className="diagnostics-actions">
          <button
            type="button"
            className="ghost"
            onClick={handleSaveDebugLog}
            disabled={saving}
          >
            {saving ? <><span className="spinner" aria-hidden="true" />…</> : t("saveDebugLog")}
          </button>
          {savedNotice && <span className="diagnostics-notice success">{savedNotice}</span>}
          {error && <span className="diagnostics-notice error">{error}</span>}
        </div>
      )}
      <div className="metric-grid">
        <Metric label={t("parsedRecords")} value={diagnostics.parsed_records.toLocaleString()} />
        <Metric label={t("skippedLines")} value={diagnostics.skipped_lines.toLocaleString()} />
        <Metric label={t("warnings")} value={(diagnostics.warning_count ?? 0).toLocaleString()} />
        <Metric label={t("errors")} value={(diagnostics.error_count ?? 0).toLocaleString()} />
        {(() => {
          const d = diagnostics as any;
          const cards: JSX.Element[] = [];
          if (d.bytes_read) {
            cards.push(<Metric key="bytes" label="Bytes read" value={Number(d.bytes_read).toLocaleString()} />);
          }
          if (d.dropped_stacks) {
            cards.push(<Metric key="dropped" label="Dropped stacks (cap hit)" value={Number(d.dropped_stacks).toLocaleString()} />);
          }
          if (d.over_depth_records) {
            cards.push(<Metric key="overdepth" label="Depth-truncated records" value={Number(d.over_depth_records).toLocaleString()} />);
          }
          if (d.max_observed_depth) {
            cards.push(<Metric key="maxdepth" label="Max observed depth" value={Number(d.max_observed_depth).toLocaleString()} />);
          }
          return cards;
        })()}
      </div>
      {(() => {
        const d = diagnostics as any;
        if (!d.dropped_stack_reason) return null;
        return (
          <p className="diagnostics-notice warn" style={{ marginTop: 8, padding: 8, borderRadius: 6 }}>
            {String(d.dropped_stack_reason)}
          </p>
        );
      })()}

      {reasons.length > 0 && (
        <div className="diagnostics-block">
          <h3>{t("diagnosticsBreakdown")}</h3>
          <ul className="reason-list">
            {reasons
              .sort((a, b) => (b[1] ?? 0) - (a[1] ?? 0))
              .map(([reason, count]) => (
                <li key={reason}>
                  <code>{reason}</code>
                  <span className="reason-count">{(count ?? 0).toLocaleString()}</span>
                </li>
              ))}
          </ul>
        </div>
      )}

      {errors.length > 0 && (
        <SampleSection title={t("diagnosticsErrors")} severity="error" samples={errors} />
      )}
      {warnings.length > 0 && (
        <SampleSection title={t("diagnosticsWarnings")} severity="warn" samples={warnings} />
      )}
      {samples.length > 0 && errors.length === 0 && warnings.length === 0 && (
        <SampleSection title={t("diagnosticsSamples")} severity="info" samples={samples} />
      )}
    </div>
  );
}

type SampleSectionProps = {
  title: string;
  severity: "error" | "warn" | "info";
  samples: DiagnosticSample[];
};

function SampleSection({ title, severity, samples }: SampleSectionProps) {
  return (
    <div className="diagnostics-block">
      <h3>{title}</h3>
      <ol className={`sample-list severity-${severity}`}>
        {samples.slice(0, 30).map((sample, idx) => (
          <li key={`${sample.line_number}-${sample.reason}-${idx}`}>
            <div className="sample-header">
              <code className="sample-reason">{sample.reason}</code>
              {sample.line_number > 0 && (
                <span className="sample-line">L{sample.line_number}</span>
              )}
            </div>
            <p className="sample-message">{sample.message}</p>
            {sample.raw_preview && (
              <pre className="sample-raw">{sample.raw_preview}</pre>
            )}
          </li>
        ))}
      </ol>
    </div>
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
