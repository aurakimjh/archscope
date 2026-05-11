import { useEffect } from "react";

type SortDirection = "asc" | "desc";

type ParsedSortValue =
  | { kind: "empty"; text: string }
  | { kind: "number"; text: string; value: number }
  | { kind: "date"; text: string; value: number }
  | { kind: "text"; text: string };

const SORTABLE_TABLE_SELECTOR = "table:not([data-sortable='false'])";
const SORTABLE_HEADER_CLASS = "sortable-header";
const SORT_ASC_CLASS = "sort-asc";
const SORT_DESC_CLASS = "sort-desc";

export function useSortableTables(): void {
  useEffect(() => {
    const enhanceAllTables = () => {
      document
        .querySelectorAll<HTMLTableElement>(SORTABLE_TABLE_SELECTOR)
        .forEach(enhanceTable);
    };

    const handleClick = (event: MouseEvent) => {
      const header = sortableHeaderFromEvent(event);
      if (!header) return;
      sortByHeader(header);
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Enter" && event.key !== " ") return;
      const header = sortableHeaderFromEvent(event);
      if (!header) return;
      event.preventDefault();
      sortByHeader(header);
    };

    let enhanceFrame = 0;
    const scheduleEnhance = () => {
      if (enhanceFrame) return;
      enhanceFrame = window.requestAnimationFrame(() => {
        enhanceFrame = 0;
        enhanceAllTables();
      });
    };

    enhanceAllTables();
    document.addEventListener("click", handleClick);
    document.addEventListener("keydown", handleKeyDown);

    const observer = new MutationObserver(scheduleEnhance);
    observer.observe(document.body, { childList: true, subtree: true });

    return () => {
      if (enhanceFrame) window.cancelAnimationFrame(enhanceFrame);
      observer.disconnect();
      document.removeEventListener("click", handleClick);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, []);
}

function enhanceTable(table: HTMLTableElement): void {
  const headerRow = table.tHead?.rows[0];
  const body = table.tBodies[0];
  if (!headerRow || !body || body.rows.length < 2) return;

  Array.from(headerRow.cells).forEach((cell) => {
    if (cell.dataset.sortable === "false") return;
    cell.classList.add(SORTABLE_HEADER_CLASS);
    cell.dataset.sortColumn = String(cell.cellIndex);
    cell.setAttribute("role", "button");
    if (!cell.hasAttribute("tabindex")) cell.tabIndex = 0;
    if (!cell.hasAttribute("aria-sort")) cell.setAttribute("aria-sort", "none");
    if (!cell.title) cell.title = "Sort by this column";
  });
}

function sortableHeaderFromEvent(event: Event): HTMLTableCellElement | null {
  const target = event.target;
  if (!(target instanceof Element)) return null;
  const header = target.closest<HTMLTableCellElement>(
    `th.${SORTABLE_HEADER_CLASS}`,
  );
  if (!header || header.dataset.sortable === "false") return null;
  const table = header.closest("table");
  if (!table?.tHead?.contains(header)) return null;
  return header;
}

function sortByHeader(header: HTMLTableCellElement): void {
  const table = header.closest("table");
  const body = table?.tBodies[0];
  if (!table || !body || body.rows.length < 2) return;

  const columnIndex = Number(header.dataset.sortColumn ?? header.cellIndex);
  if (!Number.isInteger(columnIndex) || columnIndex < 0) return;

  const rows = Array.from(body.rows).map((row, index) => ({
    row,
    index,
    value: parseSortValue(cellText(row, columnIndex)),
  }));
  const currentColumn = table.dataset.sortColumn;
  const currentDirection = table.dataset.sortDirection as SortDirection | undefined;
  const direction =
    currentColumn === String(columnIndex) && currentDirection
      ? flipDirection(currentDirection)
      : defaultDirection(rows.map((item) => item.value));

  rows.sort((a, b) => {
    if (a.value.kind === "empty" && b.value.kind === "empty") return 0;
    if (a.value.kind === "empty") return 1;
    if (b.value.kind === "empty") return -1;
    const compared = compareValues(a.value, b.value);
    if (compared !== 0) return direction === "asc" ? compared : -compared;
    return a.index - b.index;
  });

  rows.forEach((item) => body.appendChild(item.row));
  updateHeaderState(table, header, columnIndex, direction);
}

function cellText(row: HTMLTableRowElement, columnIndex: number): string {
  const cell = row.cells[columnIndex];
  return cell?.dataset.sortValue ?? cell?.innerText ?? "";
}

function parseSortValue(raw: string): ParsedSortValue {
  const text = raw.replace(/\s+/g, " ").trim();
  const lower = text.toLowerCase();
  if (
    lower === "" ||
    lower === "-" ||
    lower === "\u2013" ||
    lower === "\u2014" ||
    lower === "n/a" ||
    lower === "na" ||
    lower === "null" ||
    lower === "undefined"
  ) {
    return { kind: "empty", text };
  }

  const numericCandidate = text
    .replace(/,/g, "")
    .replace(/%$/, "")
    .replace(
      /\s*(?:ms|msec|sec|s|bytes|calls|profiles|errors|warnings|\uAC74|\uAC1C)$/i,
      "",
    )
    .trim();
  if (/^[+-]?(?:\d+\.?\d*|\.\d+)$/.test(numericCandidate)) {
    return { kind: "number", text, value: Number(numericCandidate) };
  }

  if (looksDateLike(text)) {
    const timestamp = Date.parse(text);
    if (Number.isFinite(timestamp)) {
      return { kind: "date", text, value: timestamp };
    }
  }

  return { kind: "text", text };
}

function looksDateLike(text: string): boolean {
  return /^\d{4}[-/]\d{1,2}[-/]\d{1,2}/.test(text) || /\d{1,2}:\d{2}/.test(text);
}

function defaultDirection(values: ParsedSortValue[]): SortDirection {
  const filled = values.filter((value) => value.kind !== "empty");
  if (filled.length > 0 && filled.every((value) => value.kind !== "text")) {
    return "desc";
  }
  return "asc";
}

function flipDirection(direction: SortDirection): SortDirection {
  return direction === "asc" ? "desc" : "asc";
}

function compareValues(a: ParsedSortValue, b: ParsedSortValue): number {
  if (a.kind === "number" && b.kind === "number") {
    return a.value - b.value;
  }
  if (a.kind === "date" && b.kind === "date") {
    return a.value - b.value;
  }

  return a.text.localeCompare(b.text, undefined, {
    numeric: true,
    sensitivity: "base",
  });
}

function updateHeaderState(
  table: HTMLTableElement,
  activeHeader: HTMLTableCellElement,
  columnIndex: number,
  direction: SortDirection,
): void {
  const headers = table.tHead?.querySelectorAll<HTMLTableCellElement>(
    `th.${SORTABLE_HEADER_CLASS}`,
  );
  headers?.forEach((header) => {
    header.classList.remove(SORT_ASC_CLASS, SORT_DESC_CLASS);
    header.setAttribute("aria-sort", "none");
  });

  activeHeader.classList.add(direction === "asc" ? SORT_ASC_CLASS : SORT_DESC_CLASS);
  activeHeader.setAttribute(
    "aria-sort",
    direction === "asc" ? "ascending" : "descending",
  );
  table.dataset.sortColumn = String(columnIndex);
  table.dataset.sortDirection = direction;
}
