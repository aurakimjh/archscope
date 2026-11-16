export const RIBBON_MODES = ["expanded", "tabs", "hidden"] as const;
export type RibbonMode = (typeof RIBBON_MODES)[number];
export const RIBBON_STORAGE_KEY = "archscope.desktop.ribbon.mode";
export function ribbonMode(value: unknown): RibbonMode {
  return (RIBBON_MODES as readonly unknown[]).includes(value) ? value as RibbonMode : "expanded";
}
export function toggledRibbonMode(mode: RibbonMode): RibbonMode {
  return mode === "expanded" ? "tabs" : "expanded";
}
export const RIBBON_TABS = ["home", "analyze", "http", "workspace", "view", "help"] as const;
export type RibbonTab = (typeof RIBBON_TABS)[number];
