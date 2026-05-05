// Lightweight drop overlay. Wails v3 webview exposes the dropped file via the
// browser DragEvent's `dataTransfer.files[0].path` (Wails injects the absolute
// path on top of the standard File interface). When that's unavailable, fall
// back to the file's `name` so the path-input still gets a usable hint.

import { useEffect, useRef, useState, type ReactNode } from "react";

import { useI18n } from "../i18n/I18nProvider";

export type DropZoneProps = {
  onDrop: (path: string) => void;
  children: ReactNode;
};

export function DropZone({ onDrop, children }: DropZoneProps) {
  const { t } = useI18n();
  const [hovering, setHovering] = useState(false);
  const counterRef = useRef(0);
  const overlayRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const handleDragEnter = (event: DragEvent) => {
      if (!event.dataTransfer?.types?.includes("Files")) return;
      event.preventDefault();
      counterRef.current += 1;
      setHovering(true);
    };
    const handleDragOver = (event: DragEvent) => {
      if (!event.dataTransfer?.types?.includes("Files")) return;
      event.preventDefault();
      event.dataTransfer.dropEffect = "copy";
    };
    const handleDragLeave = () => {
      counterRef.current = Math.max(0, counterRef.current - 1);
      if (counterRef.current === 0) setHovering(false);
    };
    const handleDrop = (event: DragEvent) => {
      if (!event.dataTransfer?.types?.includes("Files")) return;
      event.preventDefault();
      counterRef.current = 0;
      setHovering(false);
      const file = event.dataTransfer.files?.[0];
      if (!file) return;
      // Wails v3 patches File with `path`; standard browsers do not.
      const path = (file as any).path || file.name;
      if (path) onDrop(path);
    };

    window.addEventListener("dragenter", handleDragEnter);
    window.addEventListener("dragover", handleDragOver);
    window.addEventListener("dragleave", handleDragLeave);
    window.addEventListener("drop", handleDrop);
    return () => {
      window.removeEventListener("dragenter", handleDragEnter);
      window.removeEventListener("dragover", handleDragOver);
      window.removeEventListener("dragleave", handleDragLeave);
      window.removeEventListener("drop", handleDrop);
    };
  }, [onDrop]);

  return (
    <>
      {children}
      {hovering && (
        <div className="drop-overlay" ref={overlayRef}>
          <div className="drop-overlay-inner">
            <div className="drop-icon" aria-hidden="true">
              ⤓
            </div>
            <div className="drop-title">{t("dropHere")}</div>
            <div className="drop-hint">{t("dropHereHint")}</div>
          </div>
        </div>
      )}
    </>
  );
}
