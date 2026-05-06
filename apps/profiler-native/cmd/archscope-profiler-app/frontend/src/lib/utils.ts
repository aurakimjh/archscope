import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// `cn` lifted verbatim from apps/frontend/src/lib/utils.ts. Phase 1 keeps
// the helper minimal so the lifted shadcn primitives compile unchanged.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
