// ─────────────────────────────────────────────────────────────────────
// [한글] MetricCard.tsx — 라벨 + 큰 숫자 단일 메트릭 카드.
//
// 책임/목적: AnalysisResult.summary 의 단일 키(예: total_threads,
// avg_pause_ms 등)를 큼직하게 보여주는 작은 카드 유닛. Tailwind 클래스
// 만 사용해 web 측 .metric-card 글로벌 CSS 와 의도적으로 분리됨 —
// Wails 셸의 CSS variable 기반 테마와 충돌하지 않도록 self-contained.
// ─────────────────────────────────────────────────────────────────────
import { cn } from "@/lib/utils";
import { getGenericMetricHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";

import { HelpTip } from "./HelpTip";

// Lifted in spirit from apps/frontend/src/components/MetricCard.tsx but
// re-styled with Tailwind utilities so the component stays self-contained
// (the web version relies on a global `.metric-card` rule under
// apps/frontend/src/styles/global.css; we deliberately don't pull that
// stylesheet over because it would clash with the Wails app's existing
// CSS-variable-based theme).

type MetricCardProps = {
  label: string;
  value: string | number;
  className?: string;
  helpText?: string | null;
};

export function MetricCard({
  label,
  value,
  className,
  helpText,
}: MetricCardProps): JSX.Element {
  const { locale } = useI18n();
  const resolvedHelp =
    helpText === null ? null : helpText ?? getGenericMetricHelpText(locale, label);

  return (
    <div
      className={cn(
        "flex flex-col gap-1 rounded-lg border border-border bg-card p-3 text-card-foreground shadow-sm",
        className,
      )}
    >
      <div className="flex min-w-0 items-start justify-between gap-2">
        <span className="min-w-0 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
          {label}
        </span>
        {resolvedHelp && <HelpTip text={resolvedHelp} align="end" />}
      </div>
      <strong className="text-base font-semibold tabular-nums">
        {value}
      </strong>
    </div>
  );
}
