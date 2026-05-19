// AnalyzerOptionsDock — shared bookmark tab for analyzer settings.
// Analyzer pages place it beside the file selector so the settings
// affordance is consistent without floating over result cards.

import { SlidersHorizontal } from "lucide-react";
import { useState, type ReactNode } from "react";

import { getHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";

import { HelpTip } from "./HelpTip";
import { SlideOverPanel } from "./SlideOverPanel";

export type AnalyzerOptionsDockProps = {
  title: string;
  children: ReactNode;
  label?: string;
  width?: number;
  footer?: ReactNode;
  disabled?: boolean;
  hasChanges?: boolean;
  className?: string;
  placement?: "inline" | "fixed";
  helpText?: string | null;
};

export function AnalyzerOptionsDock({
  title,
  children,
  label = title,
  width = 560,
  footer,
  disabled = false,
  hasChanges = false,
  className,
  placement = "inline",
  helpText,
}: AnalyzerOptionsDockProps): JSX.Element {
  const { locale } = useI18n();
  const [open, setOpen] = useState(false);
  const fixed = placement === "fixed";
  const resolvedHelp =
    helpText === null ? null : helpText ?? getHelpText(locale, "analyzerOptions");

  return (
    <>
      <button
        type="button"
        className={cn(
          "relative flex shrink-0 items-center gap-1.5 border border-border bg-background px-2 py-3 text-xs font-medium text-foreground shadow-md transition-colors",
          fixed
            ? "fixed right-0 top-[34vh] z-40 rounded-l-md border-r-0"
            : "self-stretch rounded-md",
          "hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          disabled && "pointer-events-none opacity-50",
          className,
        )}
        aria-label={label}
        onClick={() => setOpen(true)}
        disabled={disabled}
      >
        <SlidersHorizontal className="h-3.5 w-3.5 shrink-0" />
        <span style={{ writingMode: "vertical-rl" }}>{label}</span>
        {resolvedHelp && !fixed && <HelpTip text={resolvedHelp} align="end" />}
        {hasChanges && (
          <span
            className="absolute left-1 top-1 h-2 w-2 rounded-full bg-primary"
            aria-hidden="true"
          />
        )}
      </button>
      <SlideOverPanel
        open={open}
        onClose={() => setOpen(false)}
        title={title}
        width={width}
        footer={footer}
        helpText={resolvedHelp}
      >
        {children}
      </SlideOverPanel>
    </>
  );
}
