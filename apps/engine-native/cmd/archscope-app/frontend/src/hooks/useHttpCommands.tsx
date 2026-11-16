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
      if (document.querySelector(".desktop-shell")?.getAttribute("data-active-route") !== "http_capture") return;
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
    const fromRibbon = (event: Event) => {
      const command = (event as CustomEvent<unknown>).detail;
      if (document.querySelector(".desktop-shell")?.getAttribute("data-active-route") !== "http_capture" ||
        !isHttpCommand(command) || (modalOpen() && command !== "help")) return;
      router.dispatch(command, "ribbon", performance.now());
    };
    const publish = () => window.dispatchEvent(new CustomEvent("desktop:capture-commands", { detail: router.availableCommands() }));
    const unsubscribe = router.subscribe(publish);
    window.addEventListener("desktop:capture-command", fromRibbon);
    publish();
    const keydown = (event: KeyboardEvent) => {
      if (document.querySelector(".desktop-shell")?.getAttribute("data-active-route") !== "http_capture") return;
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
      unsubscribe();
      window.removeEventListener("desktop:capture-command", fromRibbon);
      window.dispatchEvent(new CustomEvent("desktop:capture-commands", { detail: [] }));
      window.removeEventListener("keydown", keydown);
    };
  }, [router]);
  return <Context.Provider value={router}>{children}</Context.Provider>;
}
export function useHttpCommands(
  handlers: Partial<Record<HttpCommand, () => void>>,
  enabled: Partial<Record<HttpCommand, boolean>> = {},
): void {
  const router = useContext(Context);
  const latest = useRef(handlers);
  latest.current = handlers;
  const keys = Object.keys(handlers).filter(isHttpCommand).filter(command => enabled[command] !== false).sort().join(",");
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
