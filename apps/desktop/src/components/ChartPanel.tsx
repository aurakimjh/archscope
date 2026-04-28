import * as echarts from "echarts";
import type { EChartsOption } from "echarts";
import { useEffect, useRef } from "react";

type ChartPanelProps = {
  title: string;
  option: EChartsOption;
};

export function ChartPanel({ title, option }: ChartPanelProps): JSX.Element {
  const chartRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!chartRef.current) {
      return undefined;
    }

    const chart = echarts.init(chartRef.current, "archscope");
    chart.setOption(option);

    const resize = (): void => chart.resize();
    window.addEventListener("resize", resize);

    return () => {
      window.removeEventListener("resize", resize);
      chart.dispose();
    };
  }, [option]);

  return (
    <section className="chart-panel" aria-label={title}>
      <div className="panel-header">
        <h2>{title}</h2>
      </div>
      <div ref={chartRef} className="chart-canvas" />
    </section>
  );
}
