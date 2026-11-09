import { useState } from "react";
import type { HttpParameter } from "@/state/httpPayload";
import { useI18n } from "@/i18n/I18nProvider";

export function HttpParameterTable({ fields }: { fields: HttpParameter[] }): React.JSX.Element {
  const { t } = useI18n();
  if (!fields.length) return <p className="text-xs text-muted-foreground">{t("httpPayloadEmpty")}</p>;
  return (
    <div className="overflow-auto">
      <table className="w-full text-left text-xs">
        <thead><tr>
          <th>{t("httpPayloadName")}</th><th>{t("httpPayloadValue")}</th>
          <th>{t("httpPayloadEncoded")}</th>
        </tr></thead>
        <tbody>{fields.map((field, index) => <tr key={index} className="border-t align-top">
          <td className="break-all py-1 pr-2 font-mono">{field.name}</td>
          <td className="break-all py-1 pr-2 font-mono">
            {field.value}
            {field.filename !== undefined && <p>{field.filename} · {field.contentType}</p>}
            {field.decodeError && <p className="text-amber-700">{t("httpPayloadDecodeError")}</p>}
          </td>
          <td className="break-all py-1 font-mono text-muted-foreground">{field.rawName}={field.rawValue}</td>
        </tr>)}</tbody>
      </table>
    </div>
  );
}

// Bound both depth and sibling counts. Nested structures remain on demand in
// native details elements, with no HTML evaluation or object construction.
export function HttpJsonTree({ value, depth = 0 }: { value: unknown; depth?: number }): React.JSX.Element {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(depth === 0);
  if (value === null || typeof value !== "object") return <span className="break-all font-mono">{JSON.stringify(value)}</span>;
  if (depth >= 16) return <span>{t("httpPayloadLimited")}</span>;
  const entries = Object.entries(value);
  return (
    <details open={expanded} onToggle={(event) => setExpanded(event.currentTarget.open)} className="pl-3 font-mono text-xs">
      <summary className="cursor-pointer">{Array.isArray(value) ? "[]" : "{}"} · {entries.length}</summary>
      {expanded && entries.slice(0, 100).map(([key, child]) => (
        <div key={key} className="border-l pl-2">
          <span className="break-all">{key} </span>
          <HttpJsonTree value={child} depth={depth + 1} />
        </div>
      ))}
      {expanded && entries.length > 100 && <p>{t("httpPayloadLimited")}</p>}
    </details>
  );
}
