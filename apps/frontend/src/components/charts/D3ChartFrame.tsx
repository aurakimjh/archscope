import { Download } from "lucide-react";
import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useRef,
  type ReactNode,
} from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import { exportNodeImage, saveImagePresets } from "@/lib/exportImage";
import { cn } from "@/lib/utils";

export type D3ChartFrameProps = {
  /** Card title (already translated). */
  title: string;
  /** File-name prefix used by the image-export dropdown. */
  exportName?: string;
  /** Optional translated subtitle. */
  description?: string;
  /** Right-aligned controls rendered before the export dropdown. */
  actions?: ReactNode;
  className?: string;
  bodyClassName?: string;
  children: ReactNode;
};

export type D3ChartFrameHandle = {
  /** DOM node that should be rasterized when exporting the chart. */
  exportTarget: HTMLDivElement | null;
};

export const D3ChartFrame = forwardRef<D3ChartFrameHandle, D3ChartFrameProps>(
  function D3ChartFrame(
    { title, exportName, description, actions, className, bodyClassName, children },
    ref,
  ) {
    const { t } = useI18n();
    const targetRef = useRef<HTMLDivElement | null>(null);

    useImperativeHandle(ref, () => ({
      get exportTarget() {
        return targetRef.current;
      },
    }));

    const filenameBase = (exportName ?? title).trim() || "archscope-chart";

    const handleExport = useCallback(
      async (format: "png" | "jpeg" | "svg", multiplier: 1 | 2 | 3) => {
        const node = targetRef.current;
        if (!node) return;
        await exportNodeImage(node, {
          filename: filenameBase,
          format,
          multiplier,
          background: "#ffffff",
        });
      },
      [filenameBase],
    );

    return (
      <Card
        className={cn("overflow-hidden", className)}
        data-chart-export-name={filenameBase}
      >
        <div className="flex items-start justify-between gap-2 border-b border-border px-4 py-3">
          <div className="min-w-0">
            <h3 className="truncate text-sm font-semibold leading-none tracking-tight">
              {title}
            </h3>
            {description && (
              <p className="mt-1 truncate text-xs text-muted-foreground">
                {description}
              </p>
            )}
          </div>
          <div className="flex shrink-0 items-center gap-1">
            {actions}
            <DropdownMenu>
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
              <DropdownMenuContent align="end">
                <DropdownMenuLabel>{t("saveImage")}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                {saveImagePresets.map((preset) => (
                  <DropdownMenuItem
                    key={preset.id}
                    onSelect={() =>
                      void handleExport(preset.format, preset.multiplier)
                    }
                  >
                    {t(preset.labelKey as MessageKey)}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
        <CardContent className={cn("p-0", bodyClassName)}>
          <div
            ref={targetRef}
            data-chart-export-target=""
            className="bg-card"
          >
            {children}
          </div>
        </CardContent>
      </Card>
    );
  },
);
