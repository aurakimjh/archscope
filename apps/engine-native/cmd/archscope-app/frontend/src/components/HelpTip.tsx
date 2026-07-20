import { CircleAlert, CircleHelp } from "lucide-react";
import {
  useEffect,
  useId,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";

import { getHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";

type HelpTipAlign = "start" | "center" | "end";
type HelpTipVariant = "help" | "warning";

export type HelpTipProps = {
  text: string;
  label?: string;
  align?: HelpTipAlign;
  variant?: HelpTipVariant;
  className?: string;
  panelClassName?: string;
};

const alignClasses: Record<HelpTipAlign, string> = {
  start: "left-0",
  center: "left-1/2 -translate-x-1/2",
  end: "right-0",
};

export function HelpTip({
  text,
  label,
  align = "start",
  variant = "help",
  className,
  panelClassName,
}: HelpTipProps): React.JSX.Element {
  const { locale } = useI18n();
  const [open, setOpen] = useState(false);
  const tooltipId = useId();
  const rootRef = useRef<HTMLSpanElement | null>(null);
  const Icon = variant === "warning" ? CircleAlert : CircleHelp;
  const fallbackLabel = getHelpText(
    locale,
    variant === "warning" ? "warningTrigger" : "helpTrigger",
  );

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === "Escape") {
      event.stopPropagation();
      setOpen(false);
    }
  };

  return (
    <span
      ref={rootRef}
      className={cn("relative inline-flex shrink-0 items-center", className)}
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onFocus={() => setOpen(true)}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setOpen(false);
        }
      }}
    >
      <button
        type="button"
        className={cn(
          "inline-flex h-5 w-5 items-center justify-center rounded-full border bg-background text-muted-foreground shadow-sm transition-colors",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1",
          variant === "warning"
            ? "border-amber-400/70 text-amber-600 hover:bg-amber-500/10 hover:text-amber-700 dark:text-amber-300"
            : "border-border hover:bg-accent hover:text-accent-foreground",
        )}
        aria-label={label ?? fallbackLabel}
        aria-describedby={open ? tooltipId : undefined}
        aria-expanded={open}
        onClick={(event) => {
          event.stopPropagation();
          setOpen((value) => !value);
        }}
        onKeyDown={handleKeyDown}
      >
        <Icon className="h-3.5 w-3.5" aria-hidden="true" />
      </button>
      {open && (
        <span
          id={tooltipId}
          role="tooltip"
          className={cn(
            "absolute top-full z-[90] mt-2 w-72 max-w-[min(18rem,calc(100vw-2rem))] rounded-md border border-border bg-popover px-3 py-2 text-left text-xs font-normal leading-5 text-popover-foreground shadow-xl",
            "normal-case tracking-normal",
            alignClasses[align],
            panelClassName,
          )}
        >
          {text}
        </span>
      )}
    </span>
  );
}

export function HelpedTitle({
  children,
  help,
  align = "start",
  className,
}: {
  children: ReactNode;
  help: string;
  align?: HelpTipAlign;
  className?: string;
}): React.JSX.Element {
  return (
    <span className={cn("inline-flex min-w-0 items-center gap-2", className)}>
      <span className="min-w-0 truncate">{children}</span>
      <HelpTip text={help} align={align} />
    </span>
  );
}

export function HelpedLabel({
  children,
  help,
  align = "start",
  className,
}: {
  children: ReactNode;
  help: string;
  align?: HelpTipAlign;
  className?: string;
}): React.JSX.Element {
  return (
    <span className={cn("inline-flex min-w-0 items-center gap-1.5", className)}>
      <span className="min-w-0">{children}</span>
      <HelpTip text={help} align={align} />
    </span>
  );
}
