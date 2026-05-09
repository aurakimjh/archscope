// ─────────────────────────────────────────────────────────────────────
// [한글] theme-provider.tsx — light/dark/system 테마를 관리하는 React
//   Context Provider(shadcn 가 권장하는 단순 구현).
//
// 책임/목적:
//   - 사용자 선택을 localStorage("archscope.theme") 에 영속화.
//   - "system" 인 경우 prefers-color-scheme media query 로 OS 테마 추적.
//   - resolved 테마(light/dark) 가 바뀌면 <html> 에 .dark 클래스 + data-theme
//     속성을 토글해 Tailwind v4 의 dark variant 와 CSS 변수를 활성화.
//
// 데이터 흐름:
//   App 부트스트랩 → ThemeProvider → useTheme() 으로 자식 컴포넌트가
//   theme/resolvedTheme/setTheme 접근.
//
// 주의:
//   - SSR 환경(typeof window === "undefined") 에서 안전한 fallback 제공.
//   - 컨텍스트 외부에서 useTheme() 호출 시 throw — 마운트 누락을 빠르게
//     발견하기 위함.
// ─────────────────────────────────────────────────────────────────────
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

const STORAGE_KEY = "archscope.theme";

function readInitialTheme(): Theme {
  if (typeof window === "undefined") return "system";
  const saved = window.localStorage.getItem(STORAGE_KEY);
  if (saved === "light" || saved === "dark" || saved === "system") return saved;
  return "system";
}

function resolveTheme(theme: Theme): "light" | "dark" {
  if (theme !== "system") return theme;
  if (typeof window === "undefined") return "light";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }): JSX.Element {
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
        window.localStorage.setItem(STORAGE_KEY, next);
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
