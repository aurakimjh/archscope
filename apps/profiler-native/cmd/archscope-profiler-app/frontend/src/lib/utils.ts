// ─────────────────────────────────────────────────────────────────────
// [한글] lib/utils.ts — Tailwind 클래스 머지 헬퍼.
//
// 책임/목적: clsx 로 truthy 클래스를 결합한 뒤, tailwind-merge 로 같은
// 카테고리(예: padding) 의 충돌 클래스를 마지막 것으로 단일화. shadcn
// primitives 가 className prop 을 안전하게 합치는 데 사용.
// ─────────────────────────────────────────────────────────────────────
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// `cn` lifted verbatim from apps/frontend/src/lib/utils.ts. Phase 1 keeps
// the helper minimal so the lifted shadcn primitives compile unchanged.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
