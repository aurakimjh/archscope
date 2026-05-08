// ─────────────────────────────────────────────────────────────────────
// [한글] components/ui/input.tsx — shadcn/ui Input 컴포넌트.
//
// 책임/목적: HTMLInputElement 를 forwardRef 로 래핑하고 표준 input 클래스
// 셋(테두리/포커스링/disabled 스타일)을 적용. 페이지에서 file path,
// 숫자, 검색어 등 텍스트 입력 모두에 공통 사용.
// ─────────────────────────────────────────────────────────────────────
import * as React from "react";

import { cn } from "@/lib/utils";

// Lifted from apps/frontend/src/components/ui/input.tsx.

const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, type, ...props }, ref) => (
    <input
      type={type}
      ref={ref}
      className={cn(
        "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  ),
);
Input.displayName = "Input";

export { Input };
