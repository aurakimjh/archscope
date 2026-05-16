export type EvidenceSourceKind =
  | "finding"
  | "chart_selection"
  | "table_row"
  | "timeline_event"
  | "slo_violation"
  | "parser_diagnostic"
  | "source_metadata";

export type EvidenceCard = {
  id: string;
  created_at: string;
  analyzer: string;
  source_kind: EvidenceSourceKind;
  title: string;
  summary?: string;
  severity?: string;
  source_file?: string;
  source_ref?: string;
  payload: Record<string, unknown>;
  comment?: string;
  hypothesis?: string;
  impact?: string;
  recommendation?: string;
};

const STORAGE_KEY = "archscope.evidence_board.cards";

export function readEvidenceCards(): EvidenceCard[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as EvidenceCard[]) : [];
  } catch {
    return [];
  }
}

export function writeEvidenceCards(cards: EvidenceCard[]): void {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(cards));
}

export function createEvidenceCard(
  input: Omit<EvidenceCard, "id" | "created_at">,
): EvidenceCard {
  return {
    ...input,
    id: `ev-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    created_at: new Date().toISOString(),
  };
}

export function addEvidenceCard(
  input: Omit<EvidenceCard, "id" | "created_at">,
): EvidenceCard[] {
  const cards = readEvidenceCards();
  const next = [createEvidenceCard(input), ...cards].slice(0, 500);
  writeEvidenceCards(next);
  window.dispatchEvent(new CustomEvent("archscope:evidence-board-updated"));
  return next;
}

export function removeEvidenceCard(id: string): EvidenceCard[] {
  const next = readEvidenceCards().filter((card) => card.id !== id);
  writeEvidenceCards(next);
  window.dispatchEvent(new CustomEvent("archscope:evidence-board-updated"));
  return next;
}

export function clearEvidenceCards(): void {
  writeEvidenceCards([]);
  window.dispatchEvent(new CustomEvent("archscope:evidence-board-updated"));
}

export function exportEvidencePack(cards: EvidenceCard[]): void {
  const payload = {
    type: "archscope_evidence_pack",
    schema_version: "0.1.0",
    created_at: new Date().toISOString(),
    card_count: cards.length,
    cards,
  };
  downloadBlob(
    JSON.stringify(payload, null, 2),
    `archscope-evidence-pack-${timestampSlug()}.json`,
    "application/json",
  );
}

export function exportEvidenceReport(cards: EvidenceCard[]): void {
  downloadBlob(
    buildEvidenceReportHTML(cards),
    `archscope-evidence-report-${timestampSlug()}.html`,
    "text/html",
  );
}

export function buildEvidenceReportHTML(cards: EvidenceCard[]): string {
  const createdAt = new Date().toISOString();
  const rows = cards
    .map(
      (card) => `<article class="card">
  <header>
    <h2>${escapeHTML(card.title)}</h2>
    <div class="meta">
      <span>${escapeHTML(card.analyzer)}</span>
      <span>${escapeHTML(card.source_kind)}</span>
      ${card.severity ? `<span>${escapeHTML(card.severity)}</span>` : ""}
    </div>
  </header>
  ${card.summary ? `<p>${escapeHTML(card.summary)}</p>` : ""}
  <dl>
    <dt>Source</dt><dd>${escapeHTML(card.source_file ?? "-")}</dd>
    <dt>Reference</dt><dd>${escapeHTML(card.source_ref ?? card.created_at)}</dd>
    <dt>Captured</dt><dd>${escapeHTML(card.created_at)}</dd>
  </dl>
  <pre>${escapeHTML(JSON.stringify(card.payload, null, 2))}</pre>
</article>`,
    )
    .join("\n");

  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>ArchScope Evidence Report</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 32px; color: #111827; background: #f9fafb; }
    h1 { margin: 0 0 4px; font-size: 24px; }
    .subtle { color: #6b7280; margin: 0 0 24px; font-size: 13px; }
    .card { break-inside: avoid; border: 1px solid #d1d5db; border-radius: 8px; background: white; margin: 0 0 16px; padding: 16px; }
    .card h2 { font-size: 16px; margin: 0; }
    .meta { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 6px; color: #4b5563; font-size: 11px; text-transform: uppercase; }
    .meta span { background: #f3f4f6; border-radius: 4px; padding: 2px 6px; }
    dl { display: grid; grid-template-columns: 96px 1fr; gap: 4px 12px; font-size: 12px; color: #4b5563; }
    dt { font-weight: 600; }
    dd { margin: 0; overflow-wrap: anywhere; }
    pre { max-height: 420px; overflow: auto; background: #f3f4f6; border-radius: 6px; padding: 12px; font-size: 11px; line-height: 1.45; }
  </style>
</head>
<body>
  <h1>ArchScope Evidence Report</h1>
  <p class="subtle">Generated ${escapeHTML(createdAt)} · ${cards.length.toLocaleString()} cards</p>
  ${rows || "<p>No evidence cards collected.</p>"}
</body>
</html>`;
}

function downloadBlob(content: string, filename: string, type: string): void {
  const blob = new Blob([content], { type });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
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
