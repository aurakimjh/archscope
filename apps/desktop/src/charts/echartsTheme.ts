import * as echarts from "echarts";

export function registerArchScopeTheme(): void {
  const baseTheme = {
    color: ["#2563eb", "#16a34a", "#f59e0b", "#dc2626", "#7c3aed", "#0891b2"],
    backgroundColor: "transparent",
    textStyle: {
      color: "#111827",
      fontFamily:
        'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
    },
    grid: {
      left: 48,
      right: 24,
      top: 48,
      bottom: 42,
    },
  };

  echarts.registerTheme("archscope", baseTheme);
  echarts.registerTheme("archscope-dark", {
    ...baseTheme,
    color: ["#60a5fa", "#34d399", "#fbbf24", "#f87171", "#a78bfa", "#22d3ee"],
    textStyle: {
      ...baseTheme.textStyle,
      color: "#f8fafc",
    },
  });
}
