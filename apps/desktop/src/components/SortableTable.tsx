import { useMemo, useState } from "react";

type Column<T> = {
  key: keyof T & string;
  label: string;
  format?: (value: T[keyof T]) => string;
  align?: "left" | "right";
};

type SortableTableProps<T extends Record<string, unknown>> = {
  columns: Column<T>[];
  data: T[];
  emptyMessage?: string;
};

type SortState = {
  key: string;
  direction: "asc" | "desc";
};

export function SortableTable<T extends Record<string, unknown>>({
  columns,
  data,
  emptyMessage = "-",
}: SortableTableProps<T>): JSX.Element {
  const [sort, setSort] = useState<SortState | null>(null);

  const sorted = useMemo(() => {
    if (!sort) return data;
    return [...data].sort((a, b) => {
      const av = a[sort.key];
      const bv = b[sort.key];
      if (av === bv) return 0;
      if (av == null) return 1;
      if (bv == null) return -1;
      const cmp = typeof av === "number" && typeof bv === "number"
        ? av - bv
        : String(av).localeCompare(String(bv));
      return sort.direction === "asc" ? cmp : -cmp;
    });
  }, [data, sort]);

  function toggleSort(key: string): void {
    setSort((prev) => {
      if (prev?.key === key) {
        return prev.direction === "asc" ? { key, direction: "desc" } : null;
      }
      return { key, direction: "asc" };
    });
  }

  if (data.length === 0) {
    return <p className="empty-table-message">{emptyMessage}</p>;
  }

  return (
    <table>
      <thead>
        <tr>
          {columns.map((col) => (
            <th
              key={col.key}
              className="sortable-th"
              style={{ textAlign: col.align ?? "left" }}
              onClick={() => toggleSort(col.key)}
            >
              {col.label}
              <span className="sort-indicator">
                {sort?.key === col.key ? (sort.direction === "asc" ? " ↑" : " ↓") : ""}
              </span>
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {sorted.map((row, idx) => (
          <tr key={idx}>
            {columns.map((col) => (
              <td key={col.key} style={{ textAlign: col.align ?? "left" }}>
                {col.format ? col.format(row[col.key]) : String(row[col.key] ?? "-")}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
