import { createPortal } from "react-dom";
import { CircleAlert, CircleHelp } from "lucide-react";
import {
  useEffect,
  useLayoutEffect,
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
  const [pinned, setPinned] = useState(false);
  const [position, setPosition] = useState({left:8,top:8});
  const panelRef = useRef<HTMLSpanElement | null>(null);
  const leaveTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const show = () => { clearTimeout(leaveTimer.current);setOpen(true); };
  const leave = () => { leaveTimer.current=setTimeout(() => {if(!pinned) setOpen(false);},180); };
  const close = () => { setOpen(false);setPinned(false); };
  useEffect(() => () => clearTimeout(leaveTimer.current), []);
  useLayoutEffect(() => {
    if (!open) return;
    const place = () => {
      const trigger = rootRef.current?.getBoundingClientRect();
      const panel = panelRef.current?.getBoundingClientRect();
      if (!trigger || !panel) return;
      const desired = align === "end" ? trigger.right-panel.width : align === "center" ? trigger.left+(trigger.width-panel.width)/2 : trigger.left;
      setPosition({left:Math.max(8,Math.min(innerWidth-panel.width-8,desired)),top:trigger.bottom+panel.height+12<innerHeight ? trigger.bottom+6 : Math.max(8,trigger.top-panel.height-6)});
    };
    place();window.addEventListener("resize",place);window.addEventListener("scroll",place,true);
    return () => {window.removeEventListener("resize",place);window.removeEventListener("scroll",place,true);};
  }, [open,text,align]);
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
      if (!rootRef.current?.contains(event.target as Node) && !panelRef.current?.contains(event.target as Node)) {
        close();
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      close();
    }
  };

  return (
    <span
      ref={rootRef}
      className={cn("relative inline-flex shrink-0 items-center", className)}
      onMouseEnter={show}
      onMouseLeave={leave}
      onFocus={show}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          if (!pinned) setOpen(false);
        }
      }}
    >
      <button
        type="button"
        data-help-trigger="true"
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
          setPinned(!pinned);
          setOpen(!pinned);
        }}
        onKeyDown={handleKeyDown}
      >
        <Icon className="h-3.5 w-3.5" aria-hidden="true" />
      </button>
      {open && createPortal(
        <span
          ref={panelRef}
          onMouseEnter={show}
          onMouseLeave={leave}
          style={{position:"fixed",left:position.left,top:position.top}}
          id={tooltipId}
          role="tooltip"
          className={cn(
            "z-[90] max-h-[min(24rem,calc(100dvh-16px))] overflow-y-auto w-72 max-w-[min(18rem,calc(100vw-2rem))] rounded-md border border-border bg-popover px-3 py-2 text-left text-xs font-normal leading-5 text-popover-foreground shadow-xl",
            "normal-case tracking-normal",
            panelClassName,
          )}
        >
          {text}
        </span>, document.body
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
