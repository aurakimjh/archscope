import type { ParserDiagnostics, DiagnosticSample } from "../../bindings/github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler/models";
import { useI18n } from "../i18n/I18nProvider";

export type DiagnosticsPanelProps = {
  diagnostics?: ParserDiagnostics | null;
};

export function DiagnosticsPanel({ diagnostics }: DiagnosticsPanelProps) {
  const { t } = useI18n();
  if (!diagnostics) {
    return <p className="muted">{t("diagnosticsEmpty")}</p>;
  }

  const reasons = Object.entries(diagnostics.skipped_by_reason ?? {}) as [string, number][];
  const samples = (diagnostics.samples ?? []) as DiagnosticSample[];
  const warnings = (diagnostics.warnings ?? []) as DiagnosticSample[];
  const errors = (diagnostics.errors ?? []) as DiagnosticSample[];

  return (
    <div className="diagnostics-detail">
      <div className="metric-grid">
        <Metric label={t("parsedRecords")} value={diagnostics.parsed_records.toLocaleString()} />
        <Metric label={t("skippedLines")} value={diagnostics.skipped_lines.toLocaleString()} />
        <Metric label={t("warnings")} value={(diagnostics.warning_count ?? 0).toLocaleString()} />
        <Metric label={t("errors")} value={(diagnostics.error_count ?? 0).toLocaleString()} />
      </div>

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
