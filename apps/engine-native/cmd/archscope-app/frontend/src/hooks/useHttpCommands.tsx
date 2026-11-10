import { Events } from "@wailsio/runtime";
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  type ReactNode,
} from "react";
import {
  HttpCommandRouter,
  httpCommandForKey,
  isHttpCommand,
  type HttpCommand,
} from "@/state/httpCommands";

const Context = createContext<HttpCommandRouter | null>(null);
export function HttpCommandProvider({
  children,
}: {
  children: ReactNode;
}): React.JSX.Element {
  const router = useMemo(() => new HttpCommandRouter(), []);
  useEffect(() => {
    const modalOpen = () =>
      !!document.querySelector(
        '[role="dialog"][aria-modal="true"]:not([aria-hidden="true"])',
      );
    const off = Events.On("capture:command", (event: unknown) => {
      const command = (event as { data?: { command?: unknown } })?.data
        ?.command;
      if (!isHttpCommand(command) || (modalOpen() && command !== "help"))
        return;
      if (
        (command === "collapse" || command === "expand") &&
        document.activeElement?.closest(
          'input,textarea,select,[contenteditable="true"]',
        )
      )
        return;
      router.dispatch(command, "native", performance.now());
    });
    const keydown = (event: KeyboardEvent) => {
      const target = event.target instanceof HTMLElement ? event.target : null;
      const typing = !!target?.closest(
        'input,textarea,select,[contenteditable="true"]',
      );
      const command = httpCommandForKey(event, typing);
      if (!command || (modalOpen() && command !== "help")) return;
      event.preventDefault();
      router.dispatch(command, "keyboard", performance.now());
    };
    window.addEventListener("keydown", keydown);
    return () => {
      off();
      window.removeEventListener("keydown", keydown);
    };
  }, [router]);
  return <Context.Provider value={router}>{children}</Context.Provider>;
}
export function useHttpCommands(
  handlers: Partial<Record<HttpCommand, () => void>>,
): void {
  const router = useContext(Context);
  const latest = useRef(handlers);
  latest.current = handlers;
  const keys = Object.keys(handlers).sort().join(",");
  useEffect(() => {
    if (!router) return;
    const cleanups = keys
      .split(",")
      .filter(isHttpCommand)
      .map((command) =>
        router.register(command, () => latest.current[command]?.()),
      );
    return () => cleanups.forEach((cleanup) => cleanup());
  }, [router, keys]);
}
