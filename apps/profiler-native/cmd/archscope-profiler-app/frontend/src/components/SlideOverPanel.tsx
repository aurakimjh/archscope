// SlideOverPanel — right-side drawer that slides in over the page
// content. Used for:
//   - Exception event details (so the user doesn't have to scroll
//     past the events table to read a stack trace)
//   - Profiler / Jennifer settings (so the long options form
//     doesn't push everything else off-screen)
//
// We render to the same DOM tree (no portal) because the app is
// fixed-size single-page and a portal would just fight the layout.
// The panel uses position:fixed and a backdrop overlay; clicking
// the backdrop or pressing Escape closes it. Width is configurable;
// the default 480px fits a stack trace comfortably without burying
// the page beneath.

import { X } from "lucide-react";
import { useEffect, type ReactNode } from "react";

import { Button } from "./ui/button";

export type SlideOverPanelProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  /** Defaults to 480 — fits a stack trace + meta neatly. */
  width?: number;
  /** Extra footer content (e.g. action buttons). */
  footer?: ReactNode;
};

export function SlideOverPanel({
  open,
  onClose,
  title,
  children,
  width = 480,
  footer,
}: SlideOverPanelProps): JSX.Element {
  // Close on Escape so keyboard users don't have to reach for the
  // close button. Bound on the document so it works even when focus
  // is on a child input.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  return (
    <>
      <div
        aria-hidden={!open}
        onClick={onClose}
        style={{
          position: "fixed",
          inset: 0,
          background: "rgba(0,0,0,0.35)",
          opacity: open ? 1 : 0,
          pointerEvents: open ? "auto" : "none",
          transition: "opacity 200ms ease",
          zIndex: 50,
        }}
      />
      <aside
        role="dialog"
        aria-modal="true"
        aria-hidden={!open}
        style={{
          position: "fixed",
          top: 0,
          right: 0,
          height: "100vh",
          width,
          maxWidth: "100vw",
          transform: open ? "translateX(0)" : `translateX(${width + 24}px)`,
          transition: "transform 220ms cubic-bezier(0.32, 0.72, 0, 1)",
          zIndex: 51,
          display: "flex",
          flexDirection: "column",
          boxShadow: open ? "-12px 0 32px rgba(0,0,0,0.18)" : "none",
        }}
        className="bg-background"
      >
        <header className="flex items-center justify-between border-b border-border px-4 py-3">
          <h3 className="text-sm font-semibold">{title}</h3>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            className="h-7 w-7 p-0"
            onClick={onClose}
            aria-label="Close panel"
          >
            <X className="h-4 w-4" />
          </Button>
        </header>
        <div className="flex-1 overflow-y-auto p-4">{children}</div>
        {footer && (
          <footer className="border-t border-border bg-muted/20 px-4 py-2.5">
            {footer}
          </footer>
        )}
      </aside>
    </>
  );
}
