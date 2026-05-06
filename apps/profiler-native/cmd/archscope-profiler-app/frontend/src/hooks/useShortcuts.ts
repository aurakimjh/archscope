// Global keyboard shortcut router. Registers a single keydown listener and
// dispatches to the most-recent handler so individual pages can opt into
// shortcuts without each one wiring up window listeners.

import { useEffect } from "react";

export type ShortcutHandlers = {
  /** Cmd/Ctrl + O — pick a profiler file. */
  onOpen?: () => void;
  /** Cmd/Ctrl + R — re-run the last analysis. */
  onAnalyze?: () => void;
  /** Esc — cancel an in-flight analysis. */
  onCancel?: () => void;
  /** Cmd/Ctrl + , — jump to Settings. */
  onSettings?: () => void;
  /** Cmd/Ctrl + 1..5 — jump to a profiler tab. */
  onTab?: (index: number) => void;
};

const isMac =
  typeof navigator !== "undefined" &&
  /Mac|iPhone|iPad|iPod/i.test(navigator.platform || navigator.userAgent || "");

export function useShortcuts(handlers: ShortcutHandlers) {
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const meta = isMac ? event.metaKey : event.ctrlKey;
      // Don't hijack typing inside text inputs.
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName?.toLowerCase();
      const isTyping =
        tag === "input" || tag === "textarea" || target?.isContentEditable;

      if (event.key === "Escape") {
        handlers.onCancel?.();
        return;
      }

      if (!meta) return;

      // Allow Cmd+Letter even when focused on inputs — that's the OS
      // convention. We only block when the user is typing in a non-Cmd
      // shortcut.
      switch (event.key.toLowerCase()) {
        case "o":
          if (isTyping && !meta) return;
          event.preventDefault();
          handlers.onOpen?.();
          break;
        case "r":
          event.preventDefault();
          handlers.onAnalyze?.();
          break;
        case ",":
          event.preventDefault();
          handlers.onSettings?.();
          break;
        case "1":
        case "2":
        case "3":
        case "4":
        case "5":
          event.preventDefault();
          handlers.onTab?.(Number(event.key));
          break;
        default:
          break;
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [handlers]);
}

export const shortcutLabels = {
  open: isMac ? "⌘O" : "Ctrl+O",
  analyze: isMac ? "⌘R" : "Ctrl+R",
  cancel: "Esc",
  settings: isMac ? "⌘," : "Ctrl+,",
  tab: (n: number) => (isMac ? `⌘${n}` : `Ctrl+${n}`),
};
