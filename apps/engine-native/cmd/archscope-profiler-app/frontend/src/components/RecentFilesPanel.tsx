// RecentFilesPanel — quick-pick list of recently analyzed files for
// each analyzer page. Click a row to load the path back into the
// page; click the trash icon to forget a single entry. Designed to
// sit in the file-pick header, just below or beside the picker so
// users can see and re-analyze with one click.
//
// The hook (useRecentFiles) handles persistence + per-category
// namespacing. This component only renders.

import { Clock, Trash2 } from "lucide-react";

import type { RecentFileEntry } from "../hooks/useRecentFiles";
import { Button } from "./ui/button";

export type RecentFilesPanelProps = {
  entries: RecentFileEntry[];
  onSelect: (entry: RecentFileEntry) => void;
  onRemove?: (path: string) => void;
  onClear?: () => void;
  emptyLabel?: string;
  title?: string;
  /** Cap displayed rows (the hook itself caps storage at 8). */
  maxVisible?: number;
};

function formatWhen(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const now = Date.now();
  const diffMs = now - d.getTime();
  const min = Math.round(diffMs / 60_000);
  if (min < 1) return "방금";
  if (min < 60) return `${min}분 전`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `${hr}시간 전`;
  const day = Math.round(hr / 24);
  if (day < 30) return `${day}일 전`;
  return d.toLocaleDateString();
}

export function RecentFilesPanel({
  entries,
  onSelect,
  onRemove,
  onClear,
  emptyLabel = "최근 분석한 파일이 없습니다",
  title = "최근 분석",
  maxVisible = 6,
}: RecentFilesPanelProps): JSX.Element | null {
  if (entries.length === 0) {
    return (
      <div className="rounded-md border border-dashed border-border/60 bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
        <Clock className="mr-1 inline-block h-3.5 w-3.5 align-text-bottom" />
        {emptyLabel}
      </div>
    );
  }

  const visible = entries.slice(0, maxVisible);

  return (
    <div className="rounded-md border border-border bg-muted/20 p-2.5">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-xs font-semibold text-foreground/80">
          <Clock className="mr-1 inline-block h-3.5 w-3.5 align-text-bottom" />
          {title}
        </span>
        {onClear && entries.length > 0 && (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="h-6 px-2 text-[11px]"
            onClick={onClear}
          >
            전체 지우기
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
                {formatWhen(entry.analyzedAt)}
              </span>
            </button>
            {onRemove && (
              <button
                type="button"
                className="text-muted-foreground hover:text-destructive"
                onClick={() => onRemove(entry.path)}
                aria-label="Remove from recent"
              >
                <Trash2 className="h-3.5 w-3.5" />
              </button>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
