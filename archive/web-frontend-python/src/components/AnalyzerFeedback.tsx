// ─────────────────────────────────────────────────────────────────────
// [한글] AnalyzerFeedback.tsx — 분석 결과 외 부가 정보를 표시하는 작은
//   패널 모음(에러/엔진 메시지/parser diagnostics/AI interpretation 등).
//
// 책임/목적:
//   - 모든 분석 페이지가 공통으로 보여 줘야 하는 정보를 한 파일에 모아
//     "패널 컴포넌트 라이브러리" 처럼 사용.
//   - ErrorPanel: BridgeError(code+message+...) 표시.
//   - DiagnosticsPanel: ParserDiagnostics(skipped lines, samples) 표시.
//   - EngineMessagesPanel: 엔진 stdout/log 라인 모음.
//   - 추가로 AiInterpretation 관련 패널 등(파일 본문 참고).
//
// 데이터 흐름:
//   analyzer.execute 의 응답 → 페이지 → 본 컴포넌트들에 props 로 전달.
//
// UI:
//   - shadcn Card/Alert 류로 통일. labels 는 i18n 으로 외부에서 주입.
// ─────────────────────────────────────────────────────────────────────
import type {
  AiFinding,
  AnalysisValue,
  BridgeError,
  InterpretationResult,
  ParserDiagnostics,
} from "../api/analyzerClient";

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

type EngineMessagesPanelProps = {
  messages: string[];
  title: string;
};

type AiFindingsPanelProps = {
  interpretation?: InterpretationResult | null;
  labels: {
    title: string;
    disabled: string;
    generatedBy: string;
    confidence: string;
    evidence: string;
    limitations: string;
  };
};

export function ErrorPanel({ error, labels }: ErrorPanelProps): JSX.Element | null {
  if (!error) {
    return null;
  }

  return (
    <section className="message-panel error-panel" role="alert">
      <strong>{labels.title}</strong>
      <p>{error.message}</p>
      <small>
        {labels.code}: {error.code}
      </small>
      {error.detail && <pre>{error.detail}</pre>}
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
    <section className="message-panel info-panel" aria-live="polite">
      <strong>{title}</strong>
      <pre>{messages.join("\n")}</pre>
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

export function AiFindingsPanel({
  interpretation,
  labels,
}: AiFindingsPanelProps): JSX.Element | null {
  if (!interpretation) {
    return null;
  }

  if (interpretation.disabled) {
    return (
      <section className="message-panel info-panel" aria-live="polite">
        <strong>{labels.title}</strong>
        <p>{labels.disabled}</p>
      </section>
    );
  }

  return (
    <section className="table-panel ai-findings-panel" aria-label={labels.title}>
      <div className="panel-header">
        <h2>{labels.title}</h2>
        <span className="ai-badge">
          {labels.generatedBy}: {interpretation.provider} / {interpretation.model}
        </span>
      </div>
      <div className="ai-finding-list">
        {interpretation.findings.map((finding) => (
          <AiFindingItem key={finding.id} finding={finding} labels={labels} />
        ))}
      </div>
    </section>
  );
}

function AiFindingItem({
  finding,
  labels,
}: {
  finding: AiFinding;
  labels: AiFindingsPanelProps["labels"];
}): JSX.Element {
  return (
    <article className={`ai-finding severity-${finding.severity}`}>
      <div className="ai-finding-header">
        <strong>{finding.label}</strong>
        <span>
          {labels.confidence}: {Math.round(finding.confidence * 100)}%
        </span>
      </div>
      <p>{finding.summary}</p>
      <small>{finding.reasoning}</small>
      <details>
        <summary>{labels.evidence}</summary>
        <ul>
          {finding.evidence_refs.map((ref) => (
            <li key={ref}>
              <code>{ref}</code>
              {finding.evidence_quotes?.[ref] && (
                <blockquote>{finding.evidence_quotes[ref]}</blockquote>
              )}
            </li>
          ))}
        </ul>
      </details>
      {finding.limitations.length > 0 && (
        <details>
          <summary>{labels.limitations}</summary>
          <ul>
            {finding.limitations.map((limitation) => (
              <li key={limitation}>{limitation}</li>
            ))}
          </ul>
        </details>
      )}
    </article>
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
