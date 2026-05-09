// ─────────────────────────────────────────────────────────────────────
// [한글] D3ChartFrame.tsx — D3 기반 차트들의 공통 프레임(헤더/툴바/카드).
//
// 책임/목적:
//   - D3BarChart / D3TimelineChart / D3FlameGraph / D3HeatmapStrip 등
//     모든 D3 차트가 동일한 카드 레이아웃과 이미지 저장 UX 를 갖도록
//     공통 wrapper 를 제공.
//   - 자식이 그릴 SVG 영역과 toolbar 를 nested로 두고, exportNodeImage
//     로 카드 전체를 캡처해 PNG/SVG 다운로드.
//   - data-chart-export-name 속성을 부여해 batchExport 가 모든 D3 차트
//     를 일괄 export 할 수 있게 마킹.
//
// UI:
//   - shadcn Card/Button/DropdownMenu + Tailwind v4 토큰.
//   - useImperativeHandle 로 부모가 ref 를 통해 export 를 트리거할 수 있게.
// ─────────────────────────────────────────────────────────────────────
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
