import { File as FileIcon, Loader2, Upload, X } from "lucide-react";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type DragEvent,
  type ReactNode,
} from "react";
import { Dialogs } from "@wailsio/runtime";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

// Wails-flavoured replacement for `apps/frontend/src/components/FileDock.tsx`.
//
// The web app's FileDock uploads bytes through the FastAPI multipart
// endpoint. The desktop build instead hands the engine an absolute file
// PATH — the analyzer reads the file directly from disk. Two affordances:
//
//   1. Click → `Dialogs.OpenFile` (Wails 3 native open dialog)
//   2. Drag-drop → Wails patches `File.path` onto dropped files so we can
//      forward the absolute path without an upload round-trip.
//
// The selection shape (`{ filePath, originalName, size }`) matches the
// web FileDock so the AccessLog page (and any later analyzer page that
// gets ported) can stay format-agnostic.

export type FileDockSelection = {
  filePath: string;
  originalName: string;
  size: number;
};

export type WailsFileDockProps = {
  /** Translated label shown above the dropzone. */
  label: string;
  /** Optional helper text. */
  description?: string;
  /** Comma-separated `accept` value for the underlying input. */
  accept?: string;
  /** Currently selected file (controlled). */
  selected?: FileDockSelection | null;
  /** Called once the user picks or drops a file. */
  onSelect: (file: FileDockSelection) => void;
  /** Called when the user clears the selection (X button). */
  onClear?: () => void;
  /** Translated copy for the call-to-action button. */
  browseLabel?: string;
  /** Translated copy for the drag-overlay text. */
  dropHereLabel?: string;
  /** Translated copy for an error banner. */
  errorLabel?: string;
  /** Render extra controls (selectors, etc.) inline next to the dropzone. */
  rightSlot?: ReactNode;
  /** When true the card uses the light "sticky top" treatment. */
  sticky?: boolean;
  className?: string;
  /** Wails dialog filters (DisplayName + Pattern). */
  fileFilters?: { displayName: string; pattern: string }[];
};

function basenameOf(filePath: string): string {
  const segments = filePath.split(/[\\/]/);
  return segments[segments.length - 1] ?? filePath;
}

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value.toFixed(value < 10 && index > 0 ? 1 : 0)} ${units[index]}`;
}

export function WailsFileDock({
  label,
  description,
  accept,
  selected,
  onSelect,
  onClear,
  browseLabel = "Browse",
  dropHereLabel = "Drop file to open",
  errorLabel = "Could not open file",
  rightSlot,
  sticky = true,
  className,
  fileFilters,
}: WailsFileDockProps): JSX.Element {
  const dragCounterRef = useRef(0);
  const [isDragOver, setIsDragOver] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!selected) {
      setError(null);
    }
  }, [selected]);

  const handleNativeFile = useCallback(
    (filePath: string, size: number, originalName?: string) => {
      onSelect({
        filePath,
        size,
        originalName: originalName ?? basenameOf(filePath),
      });
    },
    [onSelect],
  );

  const openDialog = useCallback(async () => {
    if (busy) return;
    setError(null);
    setBusy(true);
    try {
      const filters = (fileFilters ?? []).map((f) => ({
        DisplayName: f.displayName,
        Pattern: f.pattern,
      }));
      const result = await Dialogs.OpenFile({
        Title: label,
        Filters: filters.length > 0 ? filters : undefined,
        AllowsMultipleSelection: false,
      });
      const filePath = typeof result === "string" ? result : "";
      if (!filePath) return;
      handleNativeFile(filePath, 0);
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught));
    } finally {
      setBusy(false);
    }
  }, [busy, fileFilters, handleNativeFile, label]);

  const handleDrop = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      event.preventDefault();
      event.stopPropagation();
      dragCounterRef.current = 0;
      setIsDragOver(false);
      const file = event.dataTransfer?.files?.[0];
      if (!file) return;
      // Wails v3 patches the standard File interface with `path`; if a
      // browser environment ever rendered this component the drop would
      // gracefully no-op (no absolute path means we can't analyze).
      const filePath = (file as File & { path?: string }).path;
      if (!filePath) {
        setError("Drop did not include an absolute path. Use Browse instead.");
        return;
      }
      handleNativeFile(filePath, file.size, file.name);
    },
    [handleNativeFile],
  );

  const onDragEnter = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    dragCounterRef.current += 1;
    setIsDragOver(true);
  }, []);

  const onDragLeave = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    dragCounterRef.current -= 1;
    if (dragCounterRef.current <= 0) {
      dragCounterRef.current = 0;
      setIsDragOver(false);
    }
  }, []);

  const onDragOver = useCallback((event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    event.stopPropagation();
    if (event.dataTransfer) event.dataTransfer.dropEffect = "copy";
  }, []);

  return (
    <Card
      className={cn(
        "border-dashed",
        sticky &&
          "sticky top-0 z-30 border-solid bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80",
        className,
      )}
    >
      <CardContent className="p-4">
        <div className="flex flex-wrap items-stretch gap-3">
          <div
            onDragEnter={onDragEnter}
            onDragOver={onDragOver}
            onDragLeave={onDragLeave}
            onDrop={handleDrop}
            onClick={() => void openDialog()}
            role="button"
            tabIndex={0}
            aria-label={label}
            data-accept={accept}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                void openDialog();
              }
            }}
            className={cn(
              "group relative flex flex-1 cursor-pointer items-center gap-3 rounded-lg border border-dashed border-input/80 bg-muted/30 px-4 py-3 text-sm transition-colors",
              "hover:border-primary/60 hover:bg-muted/50",
              isDragOver && "border-primary bg-primary/5",
              busy && "pointer-events-none opacity-80",
            )}
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
              {busy ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Upload className="h-4 w-4" />
              )}
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium leading-tight">{label}</p>
              {description && (
                <p className="truncate text-xs text-muted-foreground">
                  {description}
                </p>
              )}
              {selected && (
                <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                  <FileIcon className="h-3 w-3 shrink-0" />
                  <span className="truncate font-mono" title={selected.filePath}>
                    {selected.originalName}
                  </span>
                  {selected.size > 0 && (
                    <>
                      <span className="shrink-0">·</span>
                      <span className="shrink-0 tabular-nums">
                        {formatBytes(selected.size)}
                      </span>
                    </>
                  )}
                </div>
              )}
              {isDragOver && (
                <p className="mt-1 text-xs font-medium text-primary">
                  {dropHereLabel}
                </p>
              )}
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Button
                type="button"
                variant="secondary"
                size="sm"
                disabled={busy}
                onClick={(event) => {
                  event.stopPropagation();
                  void openDialog();
                }}
              >
                <Upload className="h-3.5 w-3.5" />
                {browseLabel}
              </Button>
              {selected && onClear && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label="Clear file"
                  onClick={(event) => {
                    event.stopPropagation();
                    setError(null);
                    onClear();
                  }}
                >
                  <X />
                </Button>
              )}
            </div>
          </div>
          {rightSlot && (
            <div className="flex flex-shrink-0 items-stretch gap-2">
              {rightSlot}
            </div>
          )}
        </div>
        {error && (
          <p className="mt-2 rounded-md bg-destructive/10 px-3 py-2 text-xs text-destructive">
            {errorLabel}: {error}
          </p>
        )}
      </CardContent>
    </Card>
  );
}
