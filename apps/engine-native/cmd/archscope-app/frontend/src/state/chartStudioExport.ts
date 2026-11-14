export type ChartExportRow = Record<string, string | number | null>;

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
