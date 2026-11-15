import { PROFILE_VALUE_UNITS } from "./profileDuration.js";

export type ChartExportRow = Record<string, string | number | null>;

// Component rows retain their historical "samples" field name even when their
// weights measure time or bytes. The result, not that field or template default,
// declares the measurement. Missing/unrecognized units remain unknown.
export function chartValueUnit(
  template: string,
  templateUnit: string,
  summary: Record<string, unknown> | null | undefined,
): string {
  if (template !== "profiler.components") return templateUnit;
  const unit = summary?.value_unit;
  return typeof unit === "string" && (PROFILE_VALUE_UNITS as readonly string[]).includes(unit)
    ? unit
    : "unknown";
}

export function chartRowsToCSV(rows: ChartExportRow[], context: {
  template: string;
  unit: string;
  resultType: string;
  sourceFiles: readonly string[];
}): string {
  if (rows.length === 0) return "";
  const headers = [...Object.keys(rows[0]), "value_unit", "template_id", "result_type", "source_files"];
  const provenance = [context.unit, context.template, context.resultType, JSON.stringify(context.sourceFiles)];
  return `${[
    headers.map(csvCell).join(","),
    ...rows.map(row => [...Object.keys(rows[0]).map(key => row[key]), ...provenance].map(csvCell).join(",")),
  ].join("\r\n")}\r\n`;
}

function csvCell(value: string | number | null | undefined): string {
  const text = value == null ? "" : String(value);
  return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
}
