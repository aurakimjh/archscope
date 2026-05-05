import { File as FileIcon, Loader2, Upload, X } from "lucide-react";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type DragEvent,
  type ReactNode,
} from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { uploadFile } from "@/lib/uploadFile";
import { cn } from "@/lib/utils";

export type FileDockSelection = {
  filePath: string;
  originalName: string;
  size: number;
};

export type FileDockProps = {
  /** Translated label shown above the dropzone. */
  label: string;
  /** Optional helper text. */
  description?: string;
  /** Comma-separated `accept` value for the underlying input. */
  accept?: string;
  /** Allow selecting / dropping more than one file at a time. The same
   *  ``onSelect`` callback fires once per uploaded file. */
  multiple?: boolean;
  /** Currently selected file (controlled). */
  selected?: FileDockSelection | null;
  /** Called once the upload completes successfully. */
  onSelect: (file: FileDockSelection) => void;
  /** Called when the user clears the selection (X button). */
  onClear?: () => void;
  /** Optional translated copy for the call-to-action button. */
  browseLabel?: string;
  /** Optional translated copy for the drag-overlay text. */
  dropHereLabel?: string;
  /** Translated copy used while an upload is in flight. */
  uploadingLabel?: string;
  /** Translated copy used in the error banner. Defaults to a hard-coded English fallback. */
  errorLabel?: string;
  /** Render extra controls (selectors, etc.) inline next to the dropzone. */
  rightSlot?: ReactNode;
  /** When true the card uses the light "sticky top" treatment (background + shadow). */
  sticky?: boolean;
  className?: string;
};

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value.toFixed(value < 10 && index > 0 ? 1 : 0)} ${units[index]}`;
}

export function FileDock({
  label,
  description,
  accept,
  multiple = false,
  selected,
  onSelect,
  onClear,
  browseLabel = "Browse",
  dropHereLabel = "Drop file to upload",
  uploadingLabel = "Uploading...",
  errorLabel = "Upload failed",
  rightSlot,
  sticky = true,
  className,
}: FileDockProps): JSX.Element {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const dragCounterRef = useRef(0);
  const [isDragOver, setIsDragOver] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!selected) {
      setError(null);
    }
  }, [selected]);

  const handleFiles = useCallback(
    async (fileList: FileList | File[] | null | undefined) => {
      if (!fileList) return;
      const files = Array.from(fileList);
      if (files.length === 0) return;
      setError(null);
      setIsUploading(true);
      try {
        for (const file of files) {
          try {
            const result = await uploadFile(file);
            onSelect(result);
          } catch (caught) {
            setError(caught instanceof Error ? caught.message : String(caught));
          }
        }
      } finally {
        setIsUploading(false);
      }
    },
    [onSelect],
  );

  const onInputChange = useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      void handleFiles(event.currentTarget.files);
      event.currentTarget.value = "";
    },
    [handleFiles],
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

  const onDrop = useCallback(
    (event: DragEvent<HTMLDivElement>) => {
      event.preventDefault();
      event.stopPropagation();
      dragCounterRef.current = 0;
      setIsDragOver(false);
      void handleFiles(event.dataTransfer.files);
    },
    [handleFiles],
  );

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
            onDrop={onDrop}
            onClick={() => inputRef.current?.click()}
            role="button"
            tabIndex={0}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                inputRef.current?.click();
              }
            }}
            className={cn(
              "group relative flex flex-1 cursor-pointer items-center gap-3 rounded-lg border border-dashed border-input/80 bg-muted/30 px-4 py-3 text-sm transition-colors",
              "hover:border-primary/60 hover:bg-muted/50",
              isDragOver && "border-primary bg-primary/5",
              isUploading && "pointer-events-none opacity-80",
            )}
          >
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
              {isUploading ? (
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
              {selected && !isUploading && (
                <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                  <FileIcon className="h-3 w-3 shrink-0" />
                  <span className="truncate font-mono">
                    {selected.originalName}
                  </span>
                  <span className="shrink-0">·</span>
                  <span className="shrink-0 tabular-nums">
                    {formatBytes(selected.size)}
                  </span>
                </div>
              )}
              {isUploading && (
                <p className="mt-1 text-xs text-muted-foreground">
                  {uploadingLabel}
                </p>
              )}
              {isDragOver && (
                <p className="mt-1 text-xs font-medium text-primary">
                  {dropHereLabel}
                </p>
              )}
            </div>
            <input
              ref={inputRef}
              type="file"
              accept={accept}
              multiple={multiple}
              className="hidden"
              onChange={onInputChange}
            />
            <div className="flex shrink-0 items-center gap-2">
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={(event) => {
                  event.stopPropagation();
                  inputRef.current?.click();
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
