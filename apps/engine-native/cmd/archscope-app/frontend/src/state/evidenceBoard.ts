export type EvidenceSourceKind =
  | "finding"
  | "chart_selection"
  | "table_row"
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
