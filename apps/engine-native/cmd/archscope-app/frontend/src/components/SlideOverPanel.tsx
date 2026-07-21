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
import { useEffect, useRef, type ReactNode } from "react";

import { HelpTip } from "./HelpTip";
import { Button } from "./ui/button";

const FOCUSABLE =
  'a[href],area[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

function focusableWithin(root: HTMLElement | null): HTMLElement[] {
  if (!root) return [];
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => el.offsetParent !== null || el === document.activeElement,
  );
}

export type SlideOverPanelProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  /** Defaults to 480 — fits a stack trace + meta neatly. */
  width?: number;
  /** Extra footer content (e.g. action buttons). */
  footer?: ReactNode;
  helpText?: string | null;
};

export function SlideOverPanel({
  open,
  onClose,
  title,
  children,
  width = 480,
  footer,
  helpText,
}: SlideOverPanelProps): React.JSX.Element {
  const panelRef = useRef<HTMLElement | null>(null);
  const restoreRef = useRef<HTMLElement | null>(null);

  // Close on Escape and keep Tab focus inside the dialog so keyboard and
  // assistive-tech users can't tab out into the (aria-hidden) background.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
        return;
      }
      if (e.key !== "Tab") return;
      const focusables = focusableWithin(panelRef.current);
      if (focusables.length === 0) {
        e.preventDefault();
        panelRef.current?.focus();
        return;
      }
      const first = focusables[0]!;
      const last = focusables[focusables.length - 1]!;
      const active = document.activeElement as HTMLElement | null;
      if (e.shiftKey && (active === first || !panelRef.current?.contains(active))) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && active === last) {
        e.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  // On open, remember the trigger and move focus into the panel; on close,
  // restore focus to the element that opened it.
  useEffect(() => {
    if (!open) return;
    restoreRef.current = (document.activeElement as HTMLElement) ?? null;
    const focusables = focusableWithin(panelRef.current);
    (focusables[0] ?? panelRef.current)?.focus();
    return () => {
      restoreRef.current?.focus?.();
    };
  }, [open]);

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
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-hidden={!open}
        inert={!open}
        tabIndex={-1}
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
          <h3 className="inline-flex min-w-0 items-center gap-2 text-sm font-semibold">
            <span className="min-w-0 truncate">{title}</span>
            {helpText && <HelpTip text={helpText} />}
          </h3>
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
