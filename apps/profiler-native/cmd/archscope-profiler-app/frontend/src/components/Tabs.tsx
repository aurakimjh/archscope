// ─────────────────────────────────────────────────────────────────────
// [한글] Tabs.tsx — Radix/shadcn 의존성 없는 가벼운 탭 스트립.
//
// 책임/목적: TabSpec[] 와 active key 를 받아 role="tablist" 시멘틱과
// 함께 버튼 그룹 렌더. 페이지 내부에서 분석 결과의 sub-view (예:
// flamegraph / breakdown / drilldown / diagnostics) 를 전환하는 데 사용.
// 같은 워크스페이스의 web 셸(apps/frontend) 시각 언어와 동일하게 유지.
// ─────────────────────────────────────────────────────────────────────
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
