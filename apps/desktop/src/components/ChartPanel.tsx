import * as echarts from "echarts";
import type { EChartsOption } from "echarts";
import { Download, Maximize2, Moon, Palette, Sun, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  ChartCustomizer,
  defaultCustomOptions,
  type ChartCustomOptions,
} from "@/components/ChartCustomizer";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import {
  downloadDataUrl,
  exportNodeImage,
  saveImagePresets,
  type ImageFormat,
  type ImageMultiplier,
} from "@/lib/exportImage";

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
  const { t } = useI18n();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<HTMLDivElement | null>(null);
  const chartInstanceRef = useRef<echarts.ECharts | null>(null);
  const optionRef = useRef(option);
  const [currentTheme, setCurrentTheme] = useState(theme);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showCustomizer, setShowCustomizer] = useState(false);
  const [customOptions, setCustomOptions] = useState<ChartCustomOptions>({
    ...defaultCustomOptions,
  });
  const fullscreenChartRef = useRef<HTMLDivElement | null>(null);
  optionRef.current = option;

  const mergedOption = useMemo<EChartsOption>(() => {
    const merged: EChartsOption = { ...option };
    if (customOptions.legendPosition === "hidden") {
      merged.legend = { show: false };
    } else if (option.legend || option.series) {
      const horizontal =
        customOptions.legendPosition === "top" ||
        customOptions.legendPosition === "bottom";
      merged.legend = {
        ...(typeof option.legend === "object" ? option.legend : {}),
        show: true,
        orient: horizontal ? "horizontal" : "vertical",
        [customOptions.legendPosition]: 0,
      };
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

  const exportImage = useCallback(
    async (format: ImageFormat, multiplier: ImageMultiplier) => {
      const filenameBase = title.replace(/\s+/g, "_") || "archscope-chart";
      const chart = chartInstanceRef.current;
      if (chart && (format === "png" || format === "svg")) {
        const dataUrl = chart.getDataURL({
          type: format,
          backgroundColor: customOptions.backgroundColor || "#ffffff",
          pixelRatio: multiplier,
        });
        downloadDataUrl(dataUrl, `${filenameBase}.${format}`);
        return;
      }
      const node = containerRef.current;
      if (!node) return;
      await exportNodeImage(node, {
        filename: filenameBase,
        format,
        multiplier,
        background: customOptions.backgroundColor || "#ffffff",
      });
    },
    [title, customOptions.backgroundColor],
  );

  const toggleTheme = useCallback(() => {
    setCurrentTheme((prev) =>
      prev === "archscope" ? "archscope-dark" : "archscope",
    );
  }, []);

  return (
    <>
      <section
        ref={containerRef}
        className="chart-panel"
        aria-busy={busy}
        aria-label={title}
      >
        <div className="panel-header">
          <h2>{title}</h2>
          <TooltipProvider delayDuration={150}>
            <div className="flex items-center gap-1">
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    aria-label="Toggle theme"
                    onClick={toggleTheme}
                  >
                    {currentTheme === "archscope" ? <Moon /> : <Sun />}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Toggle chart theme</TooltipContent>
              </Tooltip>

              <DropdownMenu>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8"
                        aria-label={t("saveImage")}
                      >
                        <Download />
                      </Button>
                    </DropdownMenuTrigger>
                  </TooltipTrigger>
                  <TooltipContent>{t("saveImage")}</TooltipContent>
                </Tooltip>
                <DropdownMenuContent align="end">
                  <DropdownMenuLabel>{t("saveImage")}</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  {saveImagePresets.map((preset) => (
                    <DropdownMenuItem
                      key={preset.id}
                      onSelect={() => void exportImage(preset.format, preset.multiplier)}
                    >
                      {t(preset.labelKey as MessageKey)}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    aria-label="Customize"
                    onClick={() => setShowCustomizer((prev) => !prev)}
                  >
                    <Palette />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Customize</TooltipContent>
              </Tooltip>

              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-8 w-8"
                    aria-label="Fullscreen"
                    onClick={() => setIsFullscreen(true)}
                  >
                    <Maximize2 />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Fullscreen</TooltipContent>
              </Tooltip>
            </div>
          </TooltipProvider>
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
        <div
          className="chart-fullscreen-overlay"
          onClick={() => setIsFullscreen(false)}
        >
          <div
            className="chart-fullscreen-content"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="panel-header">
              <h2>{title}</h2>
              <div className="flex items-center gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label="Toggle theme"
                  onClick={toggleTheme}
                >
                  {currentTheme === "archscope" ? <Moon /> : <Sun />}
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label={t("saveImage")}
                  onClick={() => void exportImage("png", 2)}
                >
                  <Download />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  aria-label="Close"
                  onClick={() => setIsFullscreen(false)}
                >
                  <X />
                </Button>
              </div>
            </div>
            <div ref={fullscreenChartRef} className="chart-canvas" />
          </div>
        </div>
      )}
    </>
  );
}
