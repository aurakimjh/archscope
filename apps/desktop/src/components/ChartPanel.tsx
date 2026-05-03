import * as echarts from "echarts";
import type { EChartsOption } from "echarts";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ChartCustomizer, defaultCustomOptions, type ChartCustomOptions } from "./ChartCustomizer";

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
  const [currentTheme, setCurrentTheme] = useState(theme);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showCustomizer, setShowCustomizer] = useState(false);
  const [customOptions, setCustomOptions] = useState<ChartCustomOptions>({ ...defaultCustomOptions });
  const fullscreenChartRef = useRef<HTMLDivElement | null>(null);
  optionRef.current = option;

  const mergedOption = useMemo<EChartsOption>(() => {
    const merged: EChartsOption = { ...option };
    if (customOptions.legendPosition === "hidden") {
      merged.legend = { show: false };
    } else if (option.legend || option.series) {
      merged.legend = { ...(typeof option.legend === "object" ? option.legend : {}), show: true, [customOptions.legendPosition === "left" || customOptions.legendPosition === "right" ? "orient" : "orient"]: customOptions.legendPosition === "left" || customOptions.legendPosition === "right" ? "vertical" : "horizontal", [customOptions.legendPosition]: 0 };
    }
    merged.backgroundColor = customOptions.backgroundColor;
    if (Array.isArray(merged.series)) {
      merged.color = customOptions.seriesColors;
    }
    return merged;
  }, [option, customOptions]);

  useEffect(() => {
    if (!chartRef.current) return undefined;

    const chart = echarts.init(chartRef.current, currentTheme, { renderer });
    chartInstanceRef.current = chart;
    chart.setOption(mergedOption, { notMerge: true, lazyUpdate: true });

    const resizeObserver = new ResizeObserver(() => chart.resize());
    resizeObserver.observe(chartRef.current);

    return () => {
      resizeObserver.disconnect();
      chart.dispose();
      if (chartInstanceRef.current === chart) {
        chartInstanceRef.current = null;
      }
    };
  }, [renderer, currentTheme]);

  useEffect(() => {
    chartInstanceRef.current?.setOption(mergedOption, {
      notMerge: true,
      lazyUpdate: true,
    });
  }, [mergedOption]);

  const downloadImage = useCallback((format: "png" | "svg") => {
    const chart = chartInstanceRef.current;
    if (!chart) return;

    const url = chart.getDataURL({
      type: format,
      backgroundColor: "#ffffff",
      pixelRatio: 2,
    });
    const link = document.createElement("a");
    link.download = `${title.replace(/\s+/g, "_")}.${format}`;
    link.href = url;
    link.click();
  }, [title]);

  const toggleTheme = useCallback(() => {
    setCurrentTheme((prev) =>
      prev === "archscope" ? "archscope-dark" : "archscope"
    );
  }, []);

  const openFullscreen = useCallback(() => {
    setIsFullscreen(true);
  }, []);

  const closeFullscreen = useCallback(() => {
    setIsFullscreen(false);
  }, []);

  useEffect(() => {
    if (!isFullscreen || !fullscreenChartRef.current) return undefined;

    const chart = echarts.init(fullscreenChartRef.current, currentTheme, { renderer });
    chart.setOption(optionRef.current, { notMerge: true, lazyUpdate: true });

    const resizeObserver = new ResizeObserver(() => chart.resize());
    resizeObserver.observe(fullscreenChartRef.current);

    return () => {
      resizeObserver.disconnect();
      chart.dispose();
    };
  }, [isFullscreen, currentTheme, renderer]);

  return (
    <>
      <section className="chart-panel" aria-busy={busy} aria-label={title}>
        <div className="panel-header">
          <h2>{title}</h2>
          <div className="chart-toolbar">
            <button type="button" title="Toggle theme" onClick={toggleTheme}>
              <svg viewBox="0 0 24 24">
                {currentTheme === "archscope" ? (
                  <path d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                ) : (
                  <path d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                )}
              </svg>
            </button>
            <button type="button" title="Download PNG" onClick={() => downloadImage("png")}>
              <svg viewBox="0 0 24 24">
                <path d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
            </button>
            <button type="button" title="Download SVG" onClick={() => downloadImage("svg")}>
              <svg viewBox="0 0 24 24">
                <path d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
              </svg>
            </button>
            <button type="button" title="Customize" onClick={() => setShowCustomizer(!showCustomizer)}>
              <svg viewBox="0 0 24 24">
                <path d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
              </svg>
            </button>
            <button type="button" title="Fullscreen" onClick={openFullscreen}>
              <svg viewBox="0 0 24 24">
                <path d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
              </svg>
            </button>
          </div>
        </div>
        {showCustomizer && (
          <ChartCustomizer
            options={customOptions}
            onChange={setCustomOptions}
            onClose={() => setShowCustomizer(false)}
          />
        )}
        <div ref={chartRef} className="chart-canvas" />
      </section>

      {isFullscreen && (
        <div className="chart-fullscreen-overlay" onClick={closeFullscreen}>
          <div className="chart-fullscreen-content" onClick={(e) => e.stopPropagation()}>
            <div className="panel-header">
              <h2>{title}</h2>
              <div className="chart-toolbar">
                <button type="button" title="Toggle theme" onClick={toggleTheme}>
                  <svg viewBox="0 0 24 24">
                    {currentTheme === "archscope" ? (
                      <path d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                    ) : (
                      <path d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                    )}
                  </svg>
                </button>
                <button type="button" title="Download PNG" onClick={() => downloadImage("png")}>
                  <svg viewBox="0 0 24 24">
                    <path d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                  </svg>
                </button>
                <button type="button" title="Close" onClick={closeFullscreen}>
                  <svg viewBox="0 0 24 24">
                    <path d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>
            <div ref={fullscreenChartRef} className="chart-canvas" />
          </div>
        </div>
      )}
    </>
  );
}
