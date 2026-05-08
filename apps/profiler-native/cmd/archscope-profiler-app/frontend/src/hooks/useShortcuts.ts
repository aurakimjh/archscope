// ─────────────────────────────────────────────────────────────────────
// [한글] hooks/useShortcuts.ts — 데스크톱 단축키 라우터.
//
// 책임/목적: window 에 단일 keydown 리스너를 등록하고, props 로 받은
// handler 객체에 매핑된 단축키만 처리. 페이지마다 자체 단축키 핸들러를
// 갖되 같은 키 조합 충돌은 호출자 측에서 해결.
//
// 단축키 매트릭스:
//   ⌘/Ctrl + O  → onOpen     (파일 열기)
//   ⌘/Ctrl + R  → onAnalyze  (마지막 분석 재실행)
//   Esc        → onCancel   (분석 중단)
//   ⌘/Ctrl + , → onSettings (설정 페이지로 점프)
//   ⌘/Ctrl + 1..5 → onTab(n) (프로파일러 탭 전환)
//
// 의존성 주의: macOS 는 metaKey, 그 외는 ctrlKey 를 modifier 로 사용.
// 텍스트 input/textarea/contenteditable 에 포커스가 있으면 비-modifier
// 단축키는 가로채지 않습니다 (사용자 타이핑 방해 방지).
// ─────────────────────────────────────────────────────────────────────
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
