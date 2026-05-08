// ─────────────────────────────────────────────────────────────────────
// [한글] lib/utils.ts — shadcn 표준 cn() 헬퍼.
//   clsx 로 조건부 className 을 평탄화한 뒤 tailwind-merge 로 충돌하는
//   유틸리티 클래스(예: px-2 와 px-4)를 뒤쪽 우선으로 정리.
//   shadcn 가이드 레퍼런스 그대로이며, 모든 컴포넌트가 import 합니다.
// ─────────────────────────────────────────────────────────────────────
import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// [한글] cn — Tailwind 유틸리티 클래스를 안전하게 합치는 표준 헬퍼.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
