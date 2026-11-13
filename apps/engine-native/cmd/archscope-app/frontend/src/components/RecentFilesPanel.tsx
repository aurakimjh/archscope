// RecentFilesPanel — quick-pick list of recently analyzed files for
// each analyzer page. Click a row to load the path back into the
// page; click the trash icon to forget a single entry. Designed to
// sit in the file-pick header, just below or beside the picker so
// users can see and re-analyze with one click.
//
// The hook (useRecentFiles) handles persistence + per-category
// namespacing. This component only renders.

import { Clock, Trash2 } from "lucide-react";

import { getHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";

import type { RecentFileEntry } from "../hooks/useRecentFiles";
import { HelpTip } from "./HelpTip";
import { Button } from "./ui/button";

export type RecentFilesPanelProps = {
  entries: RecentFileEntry[];
  onSelect: (entry: RecentFileEntry) => void;
  onRemove?: (path: string) => void;
  onClear?: () => void;
  emptyLabel?: string;
  title?: string;
  helpText?: string | null;
  /** Cap displayed rows (the hook itself caps storage at 8). */
  maxVisible?: number;
};

function formatWhen(iso: string | undefined, locale: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const now = Date.now();
  const diffMs = now - d.getTime();
  const min = Math.round(diffMs / 60_000);
  const relative = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  if (min < 1) return relative.format(0, "second");
  if (min < 60) return relative.format(-min, "minute");
  const hr = Math.round(min / 60);
  if (hr < 24) return relative.format(-hr, "hour");
  const day = Math.round(hr / 24);
  if (day < 30) return relative.format(-day, "day");
  return d.toLocaleDateString(locale);
}

export function RecentFilesPanel({
  entries,
  onSelect,
  onRemove,
  onClear,
  emptyLabel: _emptyLabel,
  title,
  helpText,
  maxVisible = 6,
}: RecentFilesPanelProps): React.JSX.Element | null {
  const { locale, t } = useI18n();
  const displayTitle = title ?? t("recentFiles");
  const resolvedHelp =
    helpText === null ? null : helpText ?? getHelpText(locale, "recentFiles");

  if (entries.length === 0) return null;

  const visible = entries.slice(0, maxVisible);

  return (
    <details className="recent-files rounded-md border border-border bg-muted/20 px-2 py-1">
      <summary className="cursor-pointer text-xs">{displayTitle} ({entries.length})</summary>
      <div className="mb-2 flex items-center justify-between">
        <span className="inline-flex min-w-0 items-center gap-1.5 text-xs font-semibold text-foreground/80">
          <Clock className="mr-1 inline-block h-3.5 w-3.5 align-text-bottom" />
          <span className="truncate">{displayTitle}</span>
          {resolvedHelp && <HelpTip text={resolvedHelp} />}
        </span>
        {onClear && entries.length > 0 && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="h-6 px-2 text-[11px]"
            onClick={onClear}
          >
            {t("recentFilesClear")}
          </Button>
        )}
      </div>
      <ul className="flex flex-col gap-1">
        {visible.map((entry) => (
          <li key={entry.path} className="flex items-center gap-2">
            <button
              type="button"
              className="group flex min-w-0 flex-1 items-center gap-2 rounded px-2 py-1.5 text-left hover:bg-accent/50"
              onClick={() => onSelect(entry)}
              title={entry.path}
            >
              <span className="truncate font-mono text-xs">
                {entry.name ?? entry.path}
              </span>
              {entry.analyzer && (
                <span className="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium uppercase text-primary">
                  {entry.analyzer}
                </span>
              )}
              <span className="ml-auto whitespace-nowrap text-[10px] text-muted-foreground">
                {formatWhen(entry.analyzedAt, locale)}
              </span>
            </button>
            {onRemove && (
              <button
                type="button"
                className="text-muted-foreground hover:text-destructive"
                onClick={() => onRemove(entry.path)}
                aria-label={t("recentFilesRemove")}
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            )}
          </li>
        ))}
      </ul>
    </details>
  );
}
