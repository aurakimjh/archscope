import * as echarts from "echarts";
import type { EChartsOption } from "echarts";
import { useEffect, useRef } from "react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
};

export function ChartPanel({
  title,
  option,
  busy = false,
  className,
  height = 320,
}: ChartPanelProps): JSX.Element {
  const chartRef = useRef<HTMLDivElement | null>(null);
  const chartInstanceRef = useRef<echarts.ECharts | null>(null);

  useEffect(() => {
    if (!chartRef.current) return;
    const chart = echarts.init(chartRef.current);
    chartInstanceRef.current = chart;
    chart.setOption(option, { notMerge: true, lazyUpdate: true });
    const ro = new ResizeObserver(() => chart.resize());
    ro.observe(chartRef.current);
    return () => {
      ro.disconnect();
      chart.dispose();
      if (chartInstanceRef.current === chart) {
        chartInstanceRef.current = null;
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    chartInstanceRef.current?.setOption(option, {
      notMerge: true,
      lazyUpdate: true,
    });
  }, [option]);

  return (
    <Card className={cn(className)} aria-busy={busy}>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="p-3">
        <div ref={chartRef} style={{ width: "100%", height }} />
      </CardContent>
    </Card>
  );
}
