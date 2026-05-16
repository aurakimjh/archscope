import type { AnalysisWorkspaceEntry } from "@/state/analysisWorkspace";
import type { EvidenceCard } from "@/state/evidenceBoard";
import { buildIncidentTimelineAnalysisResult } from "@/state/incidentTimeline";
import { buildServiceFlowAnalysis, buildServiceFlowExportPayload } from "@/state/serviceFlow";
import { analyzeSloViolationsFromEntries } from "@/state/sloGoldenSignals";

export type ReportPackPayload = {
  type: "archscope_report_pack";
  schema_version: "0.1.0";
  created_at: string;
  card_count: number;
  source_result_count: number;
  customer_summary: ReportPackCustomerSummary;
  provenance: ReportPackProvenance;
  artifacts: {
    evidence_cards: EvidenceCard[];
    incident_timeline: ReturnType<typeof buildIncidentTimelineAnalysisResult>;
    slo_analysis: ReturnType<typeof analyzeSloViolationsFromEntries>;
    service_flow: ReturnType<typeof buildServiceFlowExportPayload>;
  };
};

export type ReportPackCustomerSummary = {
  generated_at: string;
  overview: string;
  key_observations: ReportPackCustomerObservation[];
  evidence_policy: string;
};

export type ReportPackCustomerObservation = {
  severity?: string;
  title: string;
  summary: string;
  evidence_card_ids: string[];
  evidence_refs: string[];
};

export type ReportPackProvenance = {
  generated_at: string;
  source_results: ReportPackSourceResultProvenance[];
  captured_evidence: ReportPackCapturedEvidenceProvenance[];
  deterministic_findings: ReportPackFindingProvenance[];
  derived_artifacts: Array<{
    artifact_type: string;
    schema_version?: string;
    evidence_refs: string[];
  }>;
  ai_interpretation: {
    present: boolean;
    sources: Array<{
      source_result_id: string;
      source_result_type: string;
      provider?: string;
      model?: string;
      prompt_version?: string;
      generated_at?: string;
    }>;
  };
};

export type ReportPackSourceResultProvenance = {
  id: string;
  title: string;
  result_type: string;
  source_files: string[];
  created_at: string;
  recorded_at: string;
  parser?: string;
  schema_version?: string;
  analyzer_options?: Record<string, unknown>;
  summary_preview: Array<{ key: string; value: string }>;
};

export type ReportPackCapturedEvidenceProvenance = {
  card_id: string;
  captured_at: string;
  analyzer: string;
  source_kind: string;
  source_file?: string;
  source_ref?: string;
  severity?: string;
};

export type ReportPackFindingProvenance = {
  id: string;
  source_result_id: string;
  source_result_type: string;
  severity?: string;
  code: string;
  message: string;
  evidence_refs: string[];
  evidence?: Record<string, unknown>;
};

type ZipFile = {
  path: string;
  content: string;
};

export function buildReportPackPayload(
  cards: EvidenceCard[],
  entries: AnalysisWorkspaceEntry[],
): ReportPackPayload {
  const serviceFlow = buildServiceFlowAnalysis(entries);
  const incidentTimeline = buildIncidentTimelineAnalysisResult(entries);
  const sloAnalysis = analyzeSloViolationsFromEntries(entries);
  const serviceFlowPayload = buildServiceFlowExportPayload(serviceFlow);
  const provenance = buildReportPackProvenance(cards, entries, {
    incidentTimeline,
    sloAnalysis,
    serviceFlow: serviceFlowPayload,
  });
  return {
    type: "archscope_report_pack",
    schema_version: "0.1.0",
    created_at: new Date().toISOString(),
    card_count: cards.length,
    source_result_count: entries.length,
    customer_summary: buildCustomerSummary(cards, provenance, {
      incidentTimeline,
      sloAnalysis,
      serviceFlow: serviceFlowPayload,
    }),
    provenance,
    artifacts: {
      evidence_cards: cards,
      incident_timeline: incidentTimeline,
      slo_analysis: sloAnalysis,
      service_flow: serviceFlowPayload,
    },
  };
}

export function exportReportPackHTML(cards: EvidenceCard[], entries: AnalysisWorkspaceEntry[]): void {
  const payload = buildReportPackPayload(cards, entries);
  downloadBlob(
    buildReportPackHTML(payload),
    `archscope-report-pack-${timestampSlug()}.html`,
    "text/html",
  );
}

export async function exportReportPackZip(
  cards: EvidenceCard[],
  entries: AnalysisWorkspaceEntry[],
): Promise<void> {
  const payload = buildReportPackPayload(cards, entries);
  const files: ZipFile[] = [
    { path: "index.html", content: buildReportPackHTML(payload) },
    { path: "report-pack.json", content: JSON.stringify(payload, null, 2) },
    { path: "customer-summary.json", content: JSON.stringify(payload.customer_summary, null, 2) },
    { path: "evidence-cards.json", content: JSON.stringify(payload.artifacts.evidence_cards, null, 2) },
    { path: "incident-timeline.json", content: JSON.stringify(payload.artifacts.incident_timeline, null, 2) },
    { path: "slo-analysis.json", content: JSON.stringify(payload.artifacts.slo_analysis, null, 2) },
    { path: "service-flow.json", content: JSON.stringify(payload.artifacts.service_flow, null, 2) },
  ];
  downloadBlob(
    buildStoredZip(files),
    `archscope-report-pack-${timestampSlug()}.zip`,
    "application/zip",
  );
}

export function buildReportPackHTML(payload: ReportPackPayload): string {
  const timeline = payload.artifacts.incident_timeline;
  const slo = payload.artifacts.slo_analysis;
  const serviceFlow = payload.artifacts.service_flow;
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>ArchScope Report Pack</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 32px; color: #111827; background: #f9fafb; }
    h1 { margin: 0 0 4px; font-size: 24px; }
    h2 { margin: 28px 0 12px; font-size: 18px; }
    h3 { margin: 0; font-size: 15px; }
    .subtle { color: #6b7280; margin: 0 0 24px; font-size: 13px; }
    .grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
    .metric, .card { border: 1px solid #d1d5db; border-radius: 8px; background: white; padding: 14px; }
    .metric span { display: block; color: #6b7280; font-size: 11px; text-transform: uppercase; letter-spacing: .04em; }
    .metric strong { display: block; margin-top: 4px; font-size: 20px; }
    .card { break-inside: avoid; margin: 0 0 12px; }
    .observation { margin: 0 0 12px; }
    .meta { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; color: #4b5563; font-size: 11px; text-transform: uppercase; }
    .meta span { background: #f3f4f6; border-radius: 4px; padding: 2px 6px; }
    table { width: 100%; border-collapse: collapse; background: white; border: 1px solid #d1d5db; border-radius: 8px; overflow: hidden; font-size: 12px; }
    th, td { border-bottom: 1px solid #e5e7eb; padding: 8px; text-align: left; vertical-align: top; }
    th { background: #f3f4f6; color: #4b5563; font-size: 10px; text-transform: uppercase; letter-spacing: .04em; }
    td.num { text-align: right; font-variant-numeric: tabular-nums; }
    pre { max-height: 360px; overflow: auto; background: #f3f4f6; border-radius: 6px; padding: 12px; font-size: 11px; line-height: 1.45; }
    a { color: #2563eb; text-decoration: none; }
  </style>
</head>
<body>
  <h1>ArchScope Report Pack</h1>
  <p class="subtle">Generated ${escapeHTML(payload.created_at)} · ${payload.card_count.toLocaleString()} evidence cards · ${payload.source_result_count.toLocaleString()} source results</p>

  <section class="grid">
    <div class="metric"><span>Incident events</span><strong>${timeline.summary.event_count.toLocaleString()}</strong></div>
    <div class="metric"><span>SLO violations</span><strong>${slo.violation_count.toLocaleString()}</strong></div>
    <div class="metric"><span>Service edges</span><strong>${serviceFlow.summary.edge_count.toLocaleString()}</strong></div>
    <div class="metric"><span>Evidence cards</span><strong>${payload.card_count.toLocaleString()}</strong></div>
  </section>

  <h2>Customer Summary</h2>
  ${renderCustomerSummary(payload.customer_summary)}

  <h2>Provenance</h2>
  ${renderProvenance(payload.provenance)}

  <h2>Incident Timeline Preview</h2>
  ${renderTimelineTable(timeline.tables.events.slice(0, 50))}

  <h2>SLO Violations Preview</h2>
  ${renderSloTable(slo.violations.slice(0, 50))}

  <h2>Service Flow Preview</h2>
  ${renderServiceEdgeTable(serviceFlow.edges.slice(0, 50))}

  <h2>Raw Evidence Appendix</h2>
  ${payload.artifacts.evidence_cards.map(renderEvidenceCard).join("\n") || "<p>No evidence cards collected.</p>"}
</body>
</html>`;
}

function buildReportPackProvenance(
  cards: EvidenceCard[],
  entries: AnalysisWorkspaceEntry[],
  artifacts: {
    incidentTimeline: ReportPackPayload["artifacts"]["incident_timeline"];
    sloAnalysis: ReportPackPayload["artifacts"]["slo_analysis"];
    serviceFlow: ReportPackPayload["artifacts"]["service_flow"];
  },
): ReportPackProvenance {
  return {
    generated_at: new Date().toISOString(),
    source_results: entries.map(sourceResultProvenance),
    captured_evidence: cards.map((card) => ({
      card_id: card.id,
      captured_at: card.created_at,
      analyzer: card.analyzer,
      source_kind: card.source_kind,
      source_file: card.source_file,
      source_ref: card.source_ref,
      severity: card.severity,
    })),
    deterministic_findings: [
      ...entries.flatMap(entryFindings),
      ...artifacts.sloAnalysis.violations.map((violation) => ({
        id: violation.id,
        source_result_id: "slo_analysis",
        source_result_type: "slo_analysis",
        severity: violation.severity,
        code: violation.target_id,
        message: violation.target_name,
        evidence_refs: violation.evidence_refs,
        evidence: {
          metric: violation.metric_name,
          actual: violation.actual,
          threshold: violation.threshold,
          affected_scope: violation.affected_scope,
        },
      })),
      ...artifacts.serviceFlow.findings.map((finding) => ({
        id: finding.id,
        source_result_id: "service_flow",
        source_result_type: "service_flow",
        severity: finding.severity,
        code: finding.code,
        message: finding.message,
        evidence_refs: finding.evidence_refs,
        evidence: finding.payload,
      })),
    ],
    derived_artifacts: [
      {
        artifact_type: artifacts.incidentTimeline.type,
        schema_version: artifacts.incidentTimeline.metadata.schema_version,
        evidence_refs: uniqueStrings(artifacts.incidentTimeline.tables.events.map((event) => event.evidence_ref)),
      },
      {
        artifact_type: "slo_analysis",
        evidence_refs: uniqueStrings(artifacts.sloAnalysis.violations.flatMap((violation) => violation.evidence_refs)),
      },
      {
        artifact_type: artifacts.serviceFlow.type,
        schema_version: artifacts.serviceFlow.schema_version,
        evidence_refs: uniqueStrings(artifacts.serviceFlow.findings.flatMap((finding) => finding.evidence_refs)),
      },
    ],
    ai_interpretation: collectAIInterpretationProvenance(entries),
  };
}

function buildCustomerSummary(
  cards: EvidenceCard[],
  provenance: ReportPackProvenance,
  artifacts: {
    incidentTimeline: ReportPackPayload["artifacts"]["incident_timeline"];
    sloAnalysis: ReportPackPayload["artifacts"]["slo_analysis"];
    serviceFlow: ReportPackPayload["artifacts"]["service_flow"];
  },
): ReportPackCustomerSummary {
  const cardObservations = cards
    .filter((card) => card.summary || card.severity || card.impact || card.recommendation || card.hypothesis)
    .sort(compareEvidenceCards)
    .slice(0, 10)
    .map((card) => ({
      severity: card.severity,
      title: card.title,
      summary:
        card.summary ??
        card.impact ??
        card.recommendation ??
        card.hypothesis ??
        "Captured evidence requires review.",
      evidence_card_ids: [card.id],
      evidence_refs: uniqueStrings([card.source_ref, card.source_file].filter((value): value is string => Boolean(value))),
    }));
  const remainingSlots = Math.max(0, 10 - cardObservations.length);
  const findingObservations = provenance.deterministic_findings
    .filter((finding) => finding.evidence_refs.length > 0)
    .sort(compareFindingProvenance)
    .slice(0, remainingSlots)
    .map((finding) => ({
      severity: finding.severity,
      title: finding.code,
      summary: finding.message,
      evidence_card_ids: [],
      evidence_refs: finding.evidence_refs,
    }));
  return {
    generated_at: new Date().toISOString(),
    overview: `This report pack contains ${cards.length.toLocaleString()} captured evidence cards, ${artifacts.incidentTimeline.summary.event_count.toLocaleString()} incident timeline events, ${artifacts.sloAnalysis.violation_count.toLocaleString()} SLO violations, and ${artifacts.serviceFlow.summary.edge_count.toLocaleString()} service-flow edges.`,
    key_observations: [...cardObservations, ...findingObservations],
    evidence_policy:
      "Every summary item lists evidence card IDs or source evidence references. The raw evidence appendix remains part of the report pack.",
  };
}

function sourceResultProvenance(entry: AnalysisWorkspaceEntry): ReportPackSourceResultProvenance {
  const metadata = objectOrEmpty(entry.result.metadata);
  return {
    id: entry.id,
    title: entry.title,
    result_type: entry.result_type,
    source_files: entry.source_files,
    created_at: entry.created_at,
    recorded_at: entry.recorded_at,
    parser: stringValue(metadata.parser),
    schema_version: stringValue(metadata.schema_version),
    analyzer_options: objectOrUndefined(metadata.analysis_options),
    summary_preview: entry.summary_preview,
  };
}

function entryFindings(entry: AnalysisWorkspaceEntry): ReportPackFindingProvenance[] {
  return arrayOfObjects(entry.result.metadata?.findings).map((finding, index) => {
    const code = stringValue(finding.code) || "FINDING";
    const evidenceRef = stringValue(finding.evidence_ref);
    return {
      id: `${entry.id}-finding-${code || index}`,
      source_result_id: entry.id,
      source_result_type: entry.result_type,
      severity: stringValue(finding.severity),
      code,
      message: stringValue(finding.message) || code,
      evidence_refs: evidenceRef ? [evidenceRef] : [],
      evidence: objectOrUndefined(finding.evidence),
    };
  });
}

function collectAIInterpretationProvenance(
  entries: AnalysisWorkspaceEntry[],
): ReportPackProvenance["ai_interpretation"] {
  const sources = entries.flatMap((entry) => {
    const metadata = objectOrEmpty(entry.result.metadata);
    const candidates = [
      objectOrUndefined(metadata.ai_interpretation),
      objectOrUndefined(metadata.interpretation),
      objectOrUndefined(metadata.ai_provenance),
    ].filter((value): value is Record<string, unknown> => value !== undefined);
    return candidates.map((candidate) => ({
      source_result_id: entry.id,
      source_result_type: entry.result_type,
      provider: stringValue(candidate.provider),
      model: stringValue(candidate.model),
      prompt_version: stringValue(candidate.prompt_version),
      generated_at: stringValue(candidate.generated_at),
    }));
  });
  return {
    present: sources.length > 0,
    sources,
  };
}

function renderCustomerSummary(summary: ReportPackCustomerSummary): string {
  const observations =
    summary.key_observations.length > 0
      ? `<ol>${summary.key_observations.map(renderCustomerObservation).join("\n")}</ol>`
      : "<p>No customer summary observations yet.</p>";
  return `<section>
    <p>${escapeHTML(summary.overview)}</p>
    ${observations}
    <p class="subtle">${escapeHTML(summary.evidence_policy)}</p>
  </section>`;
}

function renderCustomerObservation(observation: ReportPackCustomerObservation): string {
  const cardRefs = observation.evidence_card_ids.map(
    (cardId) => `<a href="#${escapeHTML(cardId)}">${escapeHTML(cardId)}</a>`,
  );
  const evidenceRefs = observation.evidence_refs.map((ref) => escapeHTML(ref));
  const evidenceLabels = [
    cardRefs.length > 0 ? `Cards: ${cardRefs.join(", ")}` : "",
    evidenceRefs.length > 0 ? `Refs: ${evidenceRefs.join(", ")}` : "",
  ].filter(Boolean);
  return `<li class="observation">
    <strong>${escapeHTML(observation.title)}</strong>
    ${observation.summary ? `<p>${escapeHTML(observation.summary)}</p>` : ""}
    <div class="meta">
      ${observation.severity ? `<span>${escapeHTML(observation.severity)}</span>` : ""}
      ${evidenceLabels.map((label) => `<span>${label}</span>`).join("")}
    </div>
  </li>`;
}

function renderProvenance(provenance: ReportPackProvenance): string {
  return `<section>
    <div class="grid">
      <div class="metric"><span>Source results</span><strong>${provenance.source_results.length.toLocaleString()}</strong></div>
      <div class="metric"><span>Captured evidence</span><strong>${provenance.captured_evidence.length.toLocaleString()}</strong></div>
      <div class="metric"><span>Deterministic findings</span><strong>${provenance.deterministic_findings.length.toLocaleString()}</strong></div>
      <div class="metric"><span>AI provenance</span><strong>${provenance.ai_interpretation.present ? "Present" : "None"}</strong></div>
    </div>
    ${renderSourceResultTable(provenance.source_results)}
  </section>`;
}

function renderSourceResultTable(results: ReportPackSourceResultProvenance[]): string {
  if (results.length === 0) return "<p>No source results in this session.</p>";
  return `<table><thead><tr><th>Type</th><th>Title</th><th>Parser</th><th>Sources</th></tr></thead><tbody>${results
    .map(
      (result) =>
        `<tr><td>${escapeHTML(result.result_type)}</td><td>${escapeHTML(result.title)}</td><td>${escapeHTML(
          result.parser ?? "-",
        )}</td><td>${escapeHTML(result.source_files.join(", ") || "-")}</td></tr>`,
    )
    .join("")}</tbody></table>`;
}

function renderEvidenceCard(card: EvidenceCard): string {
  return `<article class="card" id="${escapeHTML(card.id)}">
  <h3>${escapeHTML(card.title)}</h3>
  <div class="meta">
    <span>${escapeHTML(card.id)}</span>
    <span>${escapeHTML(card.analyzer)}</span>
    <span>${escapeHTML(card.source_kind)}</span>
    ${card.severity ? `<span>${escapeHTML(card.severity)}</span>` : ""}
  </div>
  ${card.summary ? `<p>${escapeHTML(card.summary)}</p>` : ""}
  <p class="subtle">${escapeHTML(card.source_ref ?? card.created_at)}</p>
  <pre>${escapeHTML(JSON.stringify(card.payload, null, 2))}</pre>
</article>`;
}

function renderTimelineTable(events: ReportPackPayload["artifacts"]["incident_timeline"]["tables"]["events"]): string {
  if (events.length === 0) return "<p>No incident timeline events.</p>";
  return `<table><thead><tr><th>Time</th><th>Severity</th><th>Source</th><th>Event</th><th>Evidence</th></tr></thead><tbody>${events
    .map(
      (event) =>
        `<tr><td>${escapeHTML(event.time_label)}</td><td>${escapeHTML(event.severity)}</td><td>${escapeHTML(
          event.source_analyzer,
        )}</td><td>${escapeHTML(event.label)}<br>${escapeHTML(event.description)}</td><td>${escapeHTML(
          event.evidence_ref,
        )}</td></tr>`,
    )
    .join("")}</tbody></table>`;
}

function renderSloTable(violations: ReportPackPayload["artifacts"]["slo_analysis"]["violations"]): string {
  if (violations.length === 0) return "<p>No SLO violations detected.</p>";
  return `<table><thead><tr><th>Severity</th><th>Target</th><th>Metric</th><th>Scope</th><th>Actual</th><th>Threshold</th></tr></thead><tbody>${violations
    .map(
      (violation) =>
        `<tr><td>${escapeHTML(violation.severity)}</td><td>${escapeHTML(violation.target_name)}</td><td>${escapeHTML(
          violation.metric_name,
        )}</td><td>${escapeHTML(violation.affected_scope)}</td><td class="num">${formatNumber(
          violation.actual,
        )} ${escapeHTML(violation.unit)}</td><td class="num">${escapeHTML(violation.comparator)} ${formatNumber(
          violation.threshold,
        )}</td></tr>`,
    )
    .join("")}</tbody></table>`;
}

function renderServiceEdgeTable(edges: ReportPackPayload["artifacts"]["service_flow"]["edges"]): string {
  if (edges.length === 0) return "<p>No service-flow edges.</p>";
  return `<table><thead><tr><th>Caller</th><th>Callee</th><th>Calls</th><th>Avg latency</th><th>Errors</th><th>Network gap</th></tr></thead><tbody>${edges
    .map(
      (edge) =>
        `<tr><td>${escapeHTML(edge.caller)}</td><td>${escapeHTML(edge.callee)}</td><td class="num">${edge.call_count.toLocaleString()}</td><td class="num">${formatOptionalMs(
          edge.avg_latency_ms,
        )}</td><td class="num">${edge.error_count.toLocaleString()}</td><td class="num">${formatOptionalMs(
          edge.avg_network_gap_ms,
        )}</td></tr>`,
    )
    .join("")}</tbody></table>`;
}

function buildStoredZip(files: ZipFile[]): Blob {
  const encoder = new TextEncoder();
  const localParts: Uint8Array[] = [];
  const centralParts: Uint8Array[] = [];
  let offset = 0;
  for (const file of files) {
    const nameBytes = encoder.encode(file.path);
    const contentBytes = encoder.encode(file.content);
    const crc = crc32(contentBytes);
    const local = buildLocalHeader(nameBytes, contentBytes, crc);
    localParts.push(local, contentBytes);
    centralParts.push(buildCentralHeader(nameBytes, contentBytes, crc, offset));
    offset += local.byteLength + contentBytes.byteLength;
  }
  const centralSize = byteLength(centralParts);
  const end = buildEndRecord(files.length, centralSize, offset);
  return new Blob(blobParts([...localParts, ...centralParts, end]), { type: "application/zip" });
}

function buildLocalHeader(nameBytes: Uint8Array, contentBytes: Uint8Array, crc: number): Uint8Array {
  const header = new Uint8Array(30 + nameBytes.byteLength);
  const view = new DataView(header.buffer);
  view.setUint32(0, 0x04034b50, true);
  view.setUint16(4, 20, true);
  view.setUint16(6, 0, true);
  view.setUint16(8, 0, true);
  setDosDateTime(view, 10);
  view.setUint32(14, crc, true);
  view.setUint32(18, contentBytes.byteLength, true);
  view.setUint32(22, contentBytes.byteLength, true);
  view.setUint16(26, nameBytes.byteLength, true);
  view.setUint16(28, 0, true);
  header.set(nameBytes, 30);
  return header;
}

function buildCentralHeader(
  nameBytes: Uint8Array,
  contentBytes: Uint8Array,
  crc: number,
  offset: number,
): Uint8Array {
  const header = new Uint8Array(46 + nameBytes.byteLength);
  const view = new DataView(header.buffer);
  view.setUint32(0, 0x02014b50, true);
  view.setUint16(4, 20, true);
  view.setUint16(6, 20, true);
  view.setUint16(8, 0, true);
  view.setUint16(10, 0, true);
  setDosDateTime(view, 12);
  view.setUint32(16, crc, true);
  view.setUint32(20, contentBytes.byteLength, true);
  view.setUint32(24, contentBytes.byteLength, true);
  view.setUint16(28, nameBytes.byteLength, true);
  view.setUint16(30, 0, true);
  view.setUint16(32, 0, true);
  view.setUint16(34, 0, true);
  view.setUint16(36, 0, true);
  view.setUint32(38, 0, true);
  view.setUint32(42, offset, true);
  header.set(nameBytes, 46);
  return header;
}

function buildEndRecord(fileCount: number, centralSize: number, centralOffset: number): Uint8Array {
  const end = new Uint8Array(22);
  const view = new DataView(end.buffer);
  view.setUint32(0, 0x06054b50, true);
  view.setUint16(4, 0, true);
  view.setUint16(6, 0, true);
  view.setUint16(8, fileCount, true);
  view.setUint16(10, fileCount, true);
  view.setUint32(12, centralSize, true);
  view.setUint32(16, centralOffset, true);
  view.setUint16(20, 0, true);
  return end;
}

function setDosDateTime(view: DataView, offset: number): void {
  const now = new Date();
  const time = (now.getHours() << 11) | (now.getMinutes() << 5) | Math.floor(now.getSeconds() / 2);
  const date = ((now.getFullYear() - 1980) << 9) | ((now.getMonth() + 1) << 5) | now.getDate();
  view.setUint16(offset, time, true);
  view.setUint16(offset + 2, date, true);
}

function crc32(bytes: Uint8Array): number {
  let crc = 0xffffffff;
  for (const byte of bytes) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit += 1) {
      crc = crc & 1 ? (crc >>> 1) ^ 0xedb88320 : crc >>> 1;
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function byteLength(parts: Uint8Array[]): number {
  return parts.reduce((total, part) => total + part.byteLength, 0);
}

function blobParts(parts: Uint8Array[]): BlobPart[] {
  return parts.map((part) => part.buffer.slice(part.byteOffset, part.byteOffset + part.byteLength) as ArrayBuffer);
}

function arrayOfObjects(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value)
    ? value.filter((item): item is Record<string, unknown> => isPlainObject(item))
    : [];
}

function objectOrEmpty(value: unknown): Record<string, unknown> {
  return isPlainObject(value) ? value : {};
}

function objectOrUndefined(value: unknown): Record<string, unknown> | undefined {
  return isPlainObject(value) ? value : undefined;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown): string | undefined {
  if (typeof value === "string") return value || undefined;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return undefined;
}

function uniqueStrings(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((left, right) => left.localeCompare(right));
}

function compareEvidenceCards(left: EvidenceCard, right: EvidenceCard): number {
  return (
    severityRank(left.severity) - severityRank(right.severity) ||
    timestampValue(right.created_at) - timestampValue(left.created_at) ||
    left.title.localeCompare(right.title)
  );
}

function compareFindingProvenance(
  left: ReportPackFindingProvenance,
  right: ReportPackFindingProvenance,
): number {
  return (
    severityRank(left.severity) - severityRank(right.severity) ||
    left.source_result_id.localeCompare(right.source_result_id) ||
    left.code.localeCompare(right.code) ||
    left.id.localeCompare(right.id)
  );
}

function severityRank(severity?: string): number {
  switch (severity?.toLowerCase()) {
    case "critical":
    case "fatal":
      return 0;
    case "error":
    case "high":
      return 1;
    case "warn":
    case "warning":
    case "medium":
      return 2;
    case "info":
    case "low":
      return 3;
    default:
      return 4;
  }
}

function timestampValue(value: string): number {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function downloadBlob(content: string | Blob, filename: string, type: string): void {
  const blob = content instanceof Blob ? content : new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function formatNumber(value: number): string {
  return Number.isFinite(value)
    ? value.toLocaleString(undefined, { maximumFractionDigits: 2 })
    : String(value);
}

function formatOptionalMs(value: number | undefined): string {
  return value === undefined ? "-" : `${formatNumber(value)} ms`;
}

function timestampSlug(): string {
  return new Date().toISOString().replace(/[:.]/g, "-");
}

function escapeHTML(value: string): string {
  return value.replace(/[&<>"']/g, (char) => {
    switch (char) {
      case "&":
        return "&amp;";
      case "<":
        return "&lt;";
      case ">":
        return "&gt;";
      case "\"":
        return "&quot;";
      default:
        return "&#39;";
    }
  });
}
