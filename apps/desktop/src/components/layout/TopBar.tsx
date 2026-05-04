import { Languages, Moon, PanelLeft, Search, Settings as SettingsIcon, Sun } from "lucide-react";

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
import { localeLabels, locales, useI18n, type Locale } from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";

type TopBarProps = {
  onToggleSidebar: () => void;
  onNavigate: (page: PageKey) => void;
};

export function TopBar({ onToggleSidebar, onNavigate }: TopBarProps): JSX.Element {
  const { theme, setTheme, resolvedTheme } = useTheme();
  const { locale, setLocale, t } = useI18n();

  return (
    <header className="sticky top-0 z-40 flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background/95 px-4 backdrop-blur supports-[backdrop-filter]:bg-background/80">
      <Button
        variant="ghost"
        size="icon"
        aria-label="Toggle sidebar"
        onClick={onToggleSidebar}
      >
        <PanelLeft />
      </Button>

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
          type="search"
          placeholder={t("topbarSearchPlaceholder")}
          className="pl-8"
          aria-label="Search"
        />
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
