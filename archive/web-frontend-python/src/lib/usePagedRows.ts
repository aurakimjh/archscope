// ─────────────────────────────────────────────────────────────────────
// [한글] lib/usePagedRows.ts — "처음 N개 보여주기 + 더 보기" 페이지네이션
//   훅(가상화 없이도 큰 리스트를 안전하게 렌더하기 위한 단순 가드).
//
// 책임/목적:
//   - 가상 스크롤(react-virtual 등)은 의외로 무거우며, 본 앱의 UX 목표는
//     "수만 행이라도 즉시 폭발하지 않고, 사용자가 더 보기로 점진 확장".
//   - rows 배열 참조가 바뀌면(=새 분석 결과) 자동으로 initial 슬라이스로
//     리셋해 사용자가 항상 상단부터 보게 합니다.
//
// 사용처:
//   - ThreadDumpAnalyzerPage(virtual thread / class histogram 등 거대 표).
//   - 일반 분석 페이지의 "이벤트 목록" 표.
// ─────────────────────────────────────────────────────────────────────
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
