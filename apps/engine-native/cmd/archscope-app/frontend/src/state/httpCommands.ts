// One command vocabulary for native menus and the active HTTP workspace.
export const HTTP_COMMANDS = [
  "start",
  "stop",
  "pause",
  "clear",
  "follow",
  "find",
  "save",
  "help",
  "collapse",
  "expand",
] as const;
export type HttpCommand = (typeof HTTP_COMMANDS)[number];
export function isHttpCommand(value: unknown): value is HttpCommand {
  return (
    typeof value === "string" &&
    (HTTP_COMMANDS as readonly string[]).includes(value)
  );
}
type Key = {
  key: string;
  ctrlKey: boolean;
  metaKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
  repeat: boolean;
  defaultPrevented: boolean;
  isComposing: boolean;
};
export function httpCommandForKey(
  event: Key,
  typing: boolean,
): HttpCommand | null {
  if (
    event.defaultPrevented ||
    event.repeat ||
    event.isComposing ||
    event.altKey
  )
    return null;
  const modifier = event.ctrlKey || event.metaKey;
  const key = event.key.toLowerCase();
  if (!modifier && !event.shiftKey) {
    return (
      ({ f9: "start", f2: "stop", f1: "help" } as const)[
        key as "f9" | "f2" | "f1"
      ] ?? null
    );
  }
  if (!modifier) return null;
  if (event.shiftKey) {
    if (typing) return null; // Preserve text selection and word-navigation keys.
    return key === "arrowleft"
      ? "collapse"
      : key === "arrowright"
        ? "expand"
        : null;
  }
  return (
    ({ s: "save", f: "find", l: "clear" } as const)[key as "s" | "f" | "l"] ??
    null
  );
}

export class HttpCommandRouter {
  private handlers = new Map<HttpCommand, () => void>();
  private last: { command: HttpCommand; source: string; time: number } | null =
    null;
  register(command: HttpCommand, handler: () => void): () => void {
    this.handlers.set(command, handler);
    return () => {
      if (this.handlers.get(command) === handler) {
        this.handlers.delete(command);
        this.last = null;
      }
    };
  }
  dispatch(
    command: HttpCommand,
    source: "native" | "keyboard",
    time: number,
  ): boolean {
    const handler = this.handlers.get(command);
    if (!handler) return false;
    // Some WebViews deliver both the menu accelerator and its DOM key event.
    // Coalesce only that cross-source echo; separate clicks/keystrokes still work.
    if (
      this.last?.command === command &&
      this.last.source !== source &&
      time - this.last.time >= 0 &&
      time - this.last.time < 100
    )
      return true;
    this.last = { command, source, time };
    handler();
    return true;
  }
}
