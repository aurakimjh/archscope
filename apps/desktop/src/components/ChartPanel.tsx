import * as echarts from "echarts";
import type { EChartsOption } from "echarts";
import { useEffect, useRef } from "react";

type ChartPanelProps = {
  title: string;
  option: EChartsOption;
  renderer?: "canvas" | "svg";
  theme?: "archscope" | "archscope-dark";
  busy?: boolean;
};

export function ChartPanel({
  title,
  option,
  renderer = "canvas",
  theme = "archscope",
  busy = false,
}: ChartPanelProps): JSX.Element {
  const chartRef = useRef<HTMLDivElement | null>(null);
  const chartInstanceRef = useRef<echarts.ECharts | null>(null);
  const optionRef = useRef(option);
  optionRef.current = option;

  useEffect(() => {
    if (!chartRef.current) {
      return undefined;
    }

    const chart = echarts.init(chartRef.current, theme, { renderer });
    chartInstanceRef.current = chart;
    chart.setOption(optionRef.current, { notMerge: true, lazyUpdate: true });

    const resizeObserver = new ResizeObserver(() => chart.resize());
    resizeObserver.observe(chartRef.current);

    return () => {
      resizeObserver.disconnect();
      chart.dispose();
      if (chartInstanceRef.current === chart) {
        chartInstanceRef.current = null;
      }
    };
  }, [renderer, theme]);

  useEffect(() => {
    chartInstanceRef.current?.setOption(option, {
      notMerge: true,
      lazyUpdate: true,
    });
  }, [option]);

  return (
    <section className="chart-panel" aria-busy={busy} aria-label={title}>
      <div className="panel-header">
        <h2>{title}</h2>
      </div>
      <div ref={chartRef} className="chart-canvas" />
    </section>
  );
}
