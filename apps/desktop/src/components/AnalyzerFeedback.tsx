import type { AnalysisValue, BridgeError, ParserDiagnostics } from "../api/analyzerClient";

type ErrorPanelProps = {
  error: BridgeError | null;
  labels: {
    title: string;
    code: string;
  };
};

type DiagnosticsPanelProps = {
  diagnostics?: ParserDiagnostics;
  labels: {
    title: string;
    parsedRecords: string;
    skippedLines: string;
    samples: string;
  };
};

export function ErrorPanel({ error, labels }: ErrorPanelProps): JSX.Element | null {
  if (!error) {
    return null;
  }

  return (
    <section className="message-panel error-panel">
      <strong>{labels.title}</strong>
      <p>{error.message}</p>
      <small>
        {labels.code}: {error.code}
      </small>
    </section>
  );
}

export function DiagnosticsPanel({
  diagnostics,
  labels,
}: DiagnosticsPanelProps): JSX.Element | null {
  if (!diagnostics) {
    return null;
  }

  const parsedRecords = diagnostics.parsed_records;
  const skippedLines = diagnostics.skipped_lines;
  const samples = diagnostics.samples;

  return (
    <section className="table-panel diagnostics-panel">
      <div className="panel-header">
        <h2>{labels.title}</h2>
      </div>
      <dl className="diagnostics-grid">
        <div>
          <dt>{labels.parsedRecords}</dt>
          <dd>{formatValue(parsedRecords)}</dd>
        </div>
        <div>
          <dt>{labels.skippedLines}</dt>
          <dd>{formatValue(skippedLines)}</dd>
        </div>
      </dl>
      {samples.length > 0 && (
        <div className="diagnostic-samples">
          <strong>{labels.samples}</strong>
          <ul>
            {samples.slice(0, 5).map((sample, index) => (
              <li key={`${sample.line_number}-${index}`}>
                {sample.line_number}: {sample.reason} - {sample.raw_preview}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

function formatValue(value: AnalysisValue | undefined): string {
  if (value === undefined || value === null) {
    return "-";
  }

  if (typeof value === "object") {
    return JSON.stringify(value);
  }

  return String(value);
}
