// Theme provider mirrored from apps/frontend/src/components/theme-provider.tsx.
// Storage key is namespaced to the native app so toggling here does not
// stomp the web app's preference if both run on the same machine.

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type Theme = "light" | "dark" | "system";

type ThemeContextValue = {
  theme: Theme;
  resolvedTheme: "light" | "dark";
  setTheme: (theme: Theme) => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

const STORAGE_KEY = "archscope.profiler.theme";

function readInitialTheme(): Theme {
  if (typeof window === "undefined") return "system";
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (saved === "light" || saved === "dark" || saved === "system") return saved;
  } catch {
    // ignore storage failures
  }
  return "system";
}

function resolveTheme(theme: Theme): "light" | "dark" {
  if (theme !== "system") return theme;
  if (typeof window === "undefined") return "dark";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readInitialTheme);
  const [resolved, setResolved] = useState<"light" | "dark">(() => resolveTheme(theme));

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle("dark", resolved === "dark");
    root.dataset.theme = resolved;
  }, [resolved]);

  useEffect(() => {
    setResolved(resolveTheme(theme));
    if (theme !== "system") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => setResolved(mq.matches ? "dark" : "light");
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, [theme]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      theme,
      resolvedTheme: resolved,
      setTheme: (next) => {
        try {
          window.localStorage.setItem(STORAGE_KEY, next);
        } catch {
          // ignore
        }
        setThemeState(next);
      },
    }),
    [theme, resolved],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used within ThemeProvider");
  return ctx;
}
