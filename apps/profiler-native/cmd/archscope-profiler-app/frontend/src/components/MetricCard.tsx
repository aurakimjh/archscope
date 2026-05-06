import { cn } from "@/lib/utils";

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
};

export function MetricCard({
  label,
  value,
  className,
}: MetricCardProps): JSX.Element {
  return (
    <div
      className={cn(
        "flex flex-col gap-1 rounded-lg border border-border bg-card p-3 text-card-foreground shadow-sm",
        className,
      )}
    >
      <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </span>
      <strong className="text-base font-semibold tabular-nums">
        {value}
      </strong>
    </div>
  );
}
