// usePagedRows — small helper for "show first N, load more" pagination.
//
// T-236 — virtual-thread dumps with 10k+ rows would otherwise force the
// renderer to mount tens of thousands of <tr> elements at once. The
// JVM-signals tab (carrier pinning, SMR unresolved, native methods,
// class histogram) is the worst offender; this hook keeps the visible
// row count bounded while letting the user step up by `step` per click.
//
// Reset to the initial slice whenever `rows` itself changes (i.e. a
// fresh analysis). Intentionally not a virtualised list — that would
// be heavier; the UX target is "show enough to scroll, never explode".

import { useEffect, useState } from "react";

export type UsePagedRowsResult<T> = {
  visibleRows: T[];
  hasMore: boolean;
  loadMore: () => void;
  visibleCount: number;
  totalCount: number;
  reset: () => void;
};

export function usePagedRows<T>(
  rows: ReadonlyArray<T>,
  initial = 50,
  step = 100,
): UsePagedRowsResult<T> {
  const [visibleCount, setVisibleCount] = useState(initial);

  // Whenever the backing array reference changes (= new analysis),
  // reset to the initial window so the user starts at the top.
  useEffect(() => {
    setVisibleCount(initial);
  }, [rows, initial]);

  const totalCount = rows.length;
  const visibleRows = totalCount > visibleCount ? rows.slice(0, visibleCount) : rows.slice();
  const hasMore = totalCount > visibleCount;

  const loadMore = () =>
    setVisibleCount((prev) => Math.min(totalCount, prev + step));
  const reset = () => setVisibleCount(initial);

  return {
    visibleRows: visibleRows as T[],
    hasMore,
    loadMore,
    visibleCount: Math.min(visibleCount, totalCount),
    totalCount,
    reset,
  };
}
