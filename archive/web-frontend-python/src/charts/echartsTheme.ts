// ─────────────────────────────────────────────────────────────────────
// [한글] echartsTheme.ts — ArchScope 전용 ECharts 테마 두 개("archscope",
//   "archscope-dark") 를 글로벌에 등록.
//
// 책임/목적:
//   - 라이트/다크 시 색 팔레트, 폰트, 기본 grid margin 을 일관되게 통일.
//   - main.tsx 의 부트스트랩에서 한 번 호출되며, 이후 echarts.init(node,
//     "archscope") 처럼 이름으로 참조.
//
// 주의:
//   - 폰트는 Pretendard(한글) + Inter(영문) 조합. main.tsx 에서 CSS import.
//   - color 배열 순서는 Tailwind palette(blue/green/amber/red/violet/cyan)
//     와 정렬되어 있어 다른 컴포넌트 색상과 시각적으로 어울리도록 함.
// ─────────────────────────────────────────────────────────────────────
import * as echarts from "echarts";

// [한글] registerArchScopeTheme — 앱 부트스트랩 시 1회 호출.
//   echarts 는 동일 이름 재등록을 허용하지만, 비용을 줄이기 위해 1회만
//   호출하도록 main.tsx 가 보장.
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
