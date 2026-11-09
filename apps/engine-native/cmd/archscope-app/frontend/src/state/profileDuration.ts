export const PROFILE_VALUE_UNITS = ["samples", "nanoseconds", "microseconds", "milliseconds", "seconds", "bytes", "unknown"] as const;

export function profileWeightUnit(unit: unknown): string {
  return ({ samples: "samples", nanoseconds: "ns", microseconds: "µs", milliseconds: "ms", seconds: "s", bytes: "bytes" } as Record<string, string>)[String(unit)] ?? "?";
}

export function formatProfileSeconds(value: unknown): string {
  if (value == null || typeof value !== "number" || !Number.isFinite(value)) return "—";
  return `${value.toLocaleString(undefined, { maximumFractionDigits: 9 })} s`;
}
