import { Languages, Moon, Search, Settings as SettingsIcon, Sun } from "lucide-react";
import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
} from "react";

import type { PageKey } from "@/App";
import { useTheme } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import {
  localeLabels,
  locales,
  useI18n,
  type Locale,
  type MessageKey,
} from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";

type TopBarProps = {
  onNavigate: (page: PageKey) => void;
};

type SearchEntry = {
  page: PageKey;
  labelKey: MessageKey;
  groupKey: MessageKey;
};

const SEARCH_INDEX: SearchEntry[] = [
  { page: "dashboard", labelKey: "dashboard", groupKey: "navWorkspace" },
  { page: "access-log", labelKey: "accessLogAnalyzer", groupKey: "navAnalyzers" },
  { page: "gc-log", labelKey: "gcLogAnalyzer", groupKey: "navAnalyzers" },
  { page: "profiler", labelKey: "profilerAnalyzer", groupKey: "navAnalyzers" },
  { page: "thread-dump", labelKey: "threadDumpAnalyzer", groupKey: "navAnalyzers" },
  { page: "exception", labelKey: "exceptionAnalyzer", groupKey: "navAnalyzers" },
  { page: "jfr", labelKey: "jfrAnalyzer", groupKey: "navAnalyzers" },
  { page: "chart-studio", labelKey: "chartStudio", groupKey: "navWorkspace" },
  { page: "demo-data", labelKey: "demoDataCenter", groupKey: "navWorkspace" },
  { page: "export-center", labelKey: "exportCenter", groupKey: "navWorkspace" },
  { page: "settings", labelKey: "settings", groupKey: "navWorkspace" },
];

export function TopBar({ onNavigate }: TopBarProps): JSX.Element {
  const { theme, setTheme, resolvedTheme } = useTheme();
  const { locale, setLocale, t } = useI18n();
  const inputRef = useRef<HTMLInputElement | null>(null);
  const popoverId = useId();

  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [highlightIndex, setHighlightIndex] = useState(0);

  // Cmd/Ctrl+K focuses + opens the search.
  useEffect(() => {
    const handler = (event: globalThis.KeyboardEvent) => {
      if (event.key.toLowerCase() !== "k") return;
      const meta = event.metaKey || event.ctrlKey;
      if (!meta || event.altKey) return;
      event.preventDefault();
      inputRef.current?.focus();
      inputRef.current?.select();
      setOpen(true);
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  const matches = useMemo(() => {
    const indexed = SEARCH_INDEX.map((entry) => ({
      ...entry,
      label: t(entry.labelKey),
      group: t(entry.groupKey),
    }));
    const trimmed = query.trim().toLowerCase();
    if (!trimmed) return indexed;
    return indexed.filter((entry) =>
      entry.label.toLowerCase().includes(trimmed) ||
      entry.page.toLowerCase().includes(trimmed) ||
      entry.group.toLowerCase().includes(trimmed),
    );
  }, [query, t]);

  // Keep the highlight in range whenever the filtered list changes.
  useEffect(() => {
    if (highlightIndex >= matches.length) {
      setHighlightIndex(matches.length === 0 ? 0 : matches.length - 1);
    }
  }, [highlightIndex, matches.length]);

  const closePopover = () => {
    setOpen(false);
    setQuery("");
    setHighlightIndex(0);
  };

  const navigateTo = (page: PageKey) => {
    onNavigate(page);
    closePopover();
    inputRef.current?.blur();
  };

  const handleKeyDown = (event: ReactKeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setOpen(true);
      setHighlightIndex((idx) => Math.min(matches.length - 1, idx + 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setHighlightIndex((idx) => Math.max(0, idx - 1));
    } else if (event.key === "Enter") {
      event.preventDefault();
      const selected = matches[highlightIndex] ?? matches[0];
      if (selected) navigateTo(selected.page);
    } else if (event.key === "Escape") {
      event.preventDefault();
      closePopover();
      inputRef.current?.blur();
    }
  };

  return (
    <header className="sticky top-0 z-40 flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background/95 px-4 backdrop-blur supports-[backdrop-filter]:bg-background/80">
      <div className="flex items-center gap-2">
        <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-[11px] font-semibold text-primary-foreground">
          AS
        </div>
        <div className="leading-tight">
          <p className="text-sm font-semibold">ArchScope</p>
          <p className="hidden text-[11px] text-muted-foreground sm:block">
            {t("appTagline")}
          </p>
        </div>
      </div>

      <Separator orientation="vertical" className="mx-2 hidden h-6 sm:block" />

      <div className="relative hidden flex-1 max-w-md md:block">
        <Search className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          ref={inputRef}
          type="search"
          placeholder={t("topbarSearchPlaceholder")}
          className="pl-8"
          aria-label={t("topbarSearchPlaceholder")}
          aria-expanded={open}
          aria-controls={popoverId}
          aria-autocomplete="list"
          role="combobox"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            setHighlightIndex(0);
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => {
            // Delay so a click on a result still registers before we hide.
            window.setTimeout(closePopover, 120);
          }}
          onKeyDown={handleKeyDown}
        />
        <kbd className="pointer-events-none absolute right-2 top-2 hidden select-none rounded border border-border bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground sm:inline-block">
          ⌘K
        </kbd>
        {open && (
          <ul
            id={popoverId}
            role="listbox"
            className="absolute left-0 right-0 top-[calc(100%+4px)] z-50 max-h-72 overflow-y-auto rounded-md border border-border bg-popover py-1 shadow-md"
          >
            {matches.length === 0 && (
              <li className="px-3 py-2 text-xs text-muted-foreground">
                {t("topbarSearchNoMatch")}
              </li>
            )}
            {matches.map((entry, idx) => {
              const isActive = idx === highlightIndex;
              return (
                <li
                  key={entry.page}
                  role="option"
                  aria-selected={isActive}
                  className={cn(
                    "flex cursor-pointer items-center justify-between px-3 py-1.5 text-sm",
                    isActive
                      ? "bg-accent text-accent-foreground"
                      : "text-popover-foreground hover:bg-accent/50",
                  )}
                  onMouseDown={(event) => {
                    // Prevent input blur from firing before the click registers.
                    event.preventDefault();
                  }}
                  onMouseEnter={() => setHighlightIndex(idx)}
                  onClick={() => navigateTo(entry.page)}
                >
                  <span>{entry.label}</span>
                  <span className="text-[10px] uppercase tracking-wider text-muted-foreground">
                    {entry.group}
                  </span>
                </li>
              );
            })}
          </ul>
        )}
      </div>

      <div className="ml-auto flex items-center gap-1">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label="Theme">
              <Sun className={cn("h-4 w-4 rotate-0 scale-100 transition-all", resolvedTheme === "dark" && "-rotate-90 scale-0")} />
              <Moon className={cn("absolute h-4 w-4 rotate-90 scale-0 transition-all", resolvedTheme === "dark" && "rotate-0 scale-100")} />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>{t("themeLabel")}</DropdownMenuLabel>
            <DropdownMenuRadioGroup value={theme} onValueChange={(value) => setTheme(value as "light" | "dark" | "system")}>
              <DropdownMenuRadioItem value="light">{t("themeLight")}</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="dark">{t("themeDark")}</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="system">{t("themeSystem")}</DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label="Language">
              <Languages />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>{t("language")}</DropdownMenuLabel>
            <DropdownMenuRadioGroup
              value={locale}
              onValueChange={(value) => setLocale(value as Locale)}
            >
              {locales.map((code) => (
                <DropdownMenuRadioItem key={code} value={code}>
                  {localeLabels[code]}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label={t("settings")}>
              <SettingsIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => onNavigate("settings")}>
              {t("settings")}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => onNavigate("export-center")}>
              {t("exportCenter")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
