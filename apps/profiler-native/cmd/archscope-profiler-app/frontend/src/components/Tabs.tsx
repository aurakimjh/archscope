// Lightweight tab strip — no shadcn/Radix dependency. Mirrors the visual
// language of the Phase 2 web shell (`apps/frontend`) so users feel at
// home moving between the two.

import type { ReactNode } from "react";

export type TabSpec<TKey extends string = string> = {
  key: TKey;
  label: string;
  /** Optional small badge shown next to the label (e.g. error counts). */
  badge?: number | string;
  /** Disable the tab when no data is available. */
  disabled?: boolean;
};

export type TabsProps<TKey extends string = string> = {
  tabs: TabSpec<TKey>[];
  active: TKey;
  onChange: (key: TKey) => void;
  rightSlot?: ReactNode;
};

export function Tabs<TKey extends string>({
  tabs,
  active,
  onChange,
  rightSlot,
}: TabsProps<TKey>) {
  return (
    <div className="tab-strip" role="tablist">
      <div className="tab-list">
        {tabs.map((tab) => {
          const isActive = tab.key === active;
          const className = [
            "tab-button",
            isActive ? "active" : "",
            tab.disabled ? "disabled" : "",
          ]
            .filter(Boolean)
            .join(" ");
          return (
            <button
              key={tab.key}
              type="button"
              role="tab"
              aria-selected={isActive}
              disabled={tab.disabled}
              className={className}
              onClick={() => !tab.disabled && onChange(tab.key)}
            >
              <span>{tab.label}</span>
              {tab.badge != null && tab.badge !== 0 && (
                <span className="tab-badge">{tab.badge}</span>
              )}
            </button>
          );
        })}
      </div>
      {rightSlot && <div className="tab-actions">{rightSlot}</div>}
    </div>
  );
}
