// ─────────────────────────────────────────────────────────────────────
// [한글] ChartPanel.tsx — ECharts 차트를 카드 컨테이너로 감싼 컴포넌트.
//
// 책임/목적: title + EChartsOption 을 받아 카드 헤더 + chart container
// 로 렌더. 마운트 시 echarts.init, ResizeObserver 로 컨테이너 크기 변화
// 추적, unmount 시 dispose 로 메모리 누수 방지.
//
// 의존성 주의: web 버전(ChartPanel.tsx) 의 SVG/PNG 내보내기, 풀스크린,
// 테마 토글 같은 추가 기능은 Phase-2 에서 별도 batch exporter 와 함께
// 도입 예정이라 본 슬림 버전에는 포함하지 않습니다.
// ─────────────────────────────────────────────────────────────────────
import { echarts, type ECharts, type EChartsOption } from "@/charts/echartsCore";
import { useEffect, useMemo, useRef, type ReactNode } from "react";

import { HelpTip } from "@/components/HelpTip";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { getGenericChartHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";

// Simplified ChartPanel for the Wails app. The web ChartPanel
// (apps/frontend/src/components/ChartPanel.tsx) layers on top:
//   - light/dark theme toggle
//   - SVG / PNG export with html-to-image
//   - fullscreen overlay
//   - per-chart customizer dialog
// We deliberately leave those as Phase-2+ scope. Phase 1 only needs
// "render an EChartsOption inside a card" — the rest can land alongside
// the Save-All-Charts batch exporter later.

export type ChartPanelProps = {
  title: string;
  option: EChartsOption;
  busy?: boolean;
  className?: string;
  /** Container height, defaults to 320px to give axis labels room. */
  height?: number;
  renderer?: "canvas" | "svg";
  actions?: ReactNode;
  onReady?: (chart: ECharts | null) => void;
  helpText?: string | null;
};

export function ChartPanel({
  title,
  option,
  busy = false,
  className,
  height = 320,
  renderer = "canvas",
  actions,
  onReady,
  helpText,
}: ChartPanelProps): JSX.Element {
  const { locale } = useI18n();
  const chartRef = useRef<HTMLDivElement | null>(null);
  const chartInstanceRef = useRef<ECharts | null>(null);
  const resolvedHelp =
    helpText === null ? null : helpText ?? getGenericChartHelpText(locale, title);
  const optimizedOption = useMemo<EChartsOption>(
    () => ({ animation: false, ...option }),
    [option],
  );

  useEffect(() => {
    if (!chartRef.current) return;
    const chart = echarts.init(chartRef.current, undefined, {
      renderer,
    });
    chartInstanceRef.current = chart;
    onReady?.(chart);
    chart.setOption(optimizedOption, { notMerge: true, lazyUpdate: true });
    let resizeFrame = 0;
    const ro = new ResizeObserver(() => {
      if (resizeFrame) cancelAnimationFrame(resizeFrame);
      resizeFrame = requestAnimationFrame(() => chart.resize());
    });
    ro.observe(chartRef.current);
    return () => {
      if (resizeFrame) cancelAnimationFrame(resizeFrame);
      ro.disconnect();
      chart.dispose();
      if (chartInstanceRef.current === chart) {
        chartInstanceRef.current = null;
      }
      onReady?.(null);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [renderer]);

  useEffect(() => {
    chartInstanceRef.current?.setOption(optimizedOption, {
      notMerge: true,
      lazyUpdate: true,
    });
  }, [optimizedOption]);

  return (
    <Card className={cn(className)} aria-busy={busy}>
      <CardHeader className="flex flex-row items-center justify-between gap-3 pb-2">
        <CardTitle className="flex min-w-0 items-center gap-2 text-sm">
          <span className="min-w-0 truncate">{title}</span>
          {resolvedHelp && <HelpTip text={resolvedHelp} />}
        </CardTitle>
        {actions}
      </CardHeader>
      <CardContent className="p-3">
        <div ref={chartRef} style={{ width: "100%", height }} />
      </CardContent>
    </Card>
  );
}
