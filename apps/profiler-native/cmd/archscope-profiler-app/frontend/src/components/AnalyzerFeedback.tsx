import type { BridgeError, ParserDiagnostics } from "@/bridge/types";

// Slim port of apps/frontend/src/components/AnalyzerFeedback.tsx.
// Phase 1 only ports `ErrorPanel`, `EngineMessagesPanel`, and
// `DiagnosticsPanel` — the AI-findings panel and intermediate AI helpers
// are deferred to a later phase since none of the wired-up engine paths
// surface AI interpretations yet.

type ErrorPanelProps = {
  error: BridgeError | null;
  labels: {
    title: string;
    code: string;
  };
};

type EngineMessagesPanelProps = {
  messages: string[];
  title: string;
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
    <section
      role="alert"
      className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive"
    >
      <strong className="block">{labels.title}</strong>
      <p className="mt-1 text-foreground">{error.message}</p>
      <small className="mt-1 block text-xs text-muted-foreground">
        {labels.code}: {error.code}
      </small>
      {error.detail && (
        <pre className="mt-2 overflow-x-auto whitespace-pre-wrap text-xs text-muted-foreground">
          {error.detail}
        </pre>
      )}
    </section>
  );
}

export function EngineMessagesPanel({
  messages,
  title,
}: EngineMessagesPanelProps): JSX.Element | null {
  if (messages.length === 0) {
    return null;
  }

  return (
    <section
      aria-live="polite"
      className="rounded-md border border-border bg-muted/40 p-3 text-sm"
    >
      <strong className="block text-foreground">{title}</strong>
      <pre className="mt-1 overflow-x-auto whitespace-pre-wrap text-xs text-muted-foreground">
        {messages.join("\n")}
      </pre>
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

  const samples = diagnostics.samples ?? [];

  return (
    <section className="rounded-md border border-border bg-card p-4 text-sm shadow-sm">
      <header className="mb-3 flex items-center justify-between">
        <h2 className="text-sm font-semibold">{labels.title}</h2>
      </header>
      <dl className="grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
        <div>
          <dt className="text-muted-foreground">{labels.parsedRecords}</dt>
          <dd className="font-mono tabular-nums">
            {diagnostics.parsed_records.toLocaleString()}
          </dd>
        </div>
        <div>
          <dt className="text-muted-foreground">{labels.skippedLines}</dt>
          <dd className="font-mono tabular-nums">
            {diagnostics.skipped_lines.toLocaleString()}
          </dd>
        </div>
      </dl>
      {samples.length > 0 && (
        <div className="mt-4">
          <strong className="block text-xs font-medium uppercase text-muted-foreground">
            {labels.samples}
          </strong>
          <ul className="mt-2 space-y-1 text-xs">
            {samples.slice(0, 5).map((sample, index) => (
              <li
                key={`${sample.line_number}-${index}`}
                className="rounded bg-muted/40 px-2 py-1 font-mono"
              >
                <span className="text-muted-foreground">
                  {sample.line_number}:
                </span>{" "}
                <span className="text-amber-600 dark:text-amber-400">
                  {sample.reason}
                </span>{" "}
                — {sample.raw_preview}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
