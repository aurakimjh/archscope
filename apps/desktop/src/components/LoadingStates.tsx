export function SkeletonCard(): JSX.Element {
  return <div className="skeleton skeleton-card" />;
}

export function SkeletonChart(): JSX.Element {
  return (
    <section className="chart-panel">
      <div className="panel-header">
        <div className="skeleton" style={{ width: 120, height: 18 }} />
      </div>
      <div className="skeleton skeleton-chart" />
    </section>
  );
}

export function LoadingSpinner({ label }: { label?: string }): JSX.Element {
  return (
    <div className="empty-state">
      <div className="spinner" />
      {label && <p>{label}</p>}
    </div>
  );
}

export function EmptyState({
  message,
  icon,
}: {
  message: string;
  icon?: "chart" | "file" | "search";
}): JSX.Element {
  return (
    <div className="empty-state">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
        {icon === "file" && (
          <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
        )}
        {icon === "chart" && (
          <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
        )}
        {(!icon || icon === "search") && (
          <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
        )}
      </svg>
      <p>{message}</p>
    </div>
  );
}
