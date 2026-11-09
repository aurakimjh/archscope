import type { GcLogAnalysisResult } from "../bridge/types.js";

export function gcAxis(result: GcLogAnalysisResult | null) {
  const axis = result?.metadata.time_axis ?? (result?.series?.pause_timeline?.every((p) => /^\d{4}-\d\d-\d\dT/.test(p.time)) ? "absolute" : "event_index");
  return axis === "absolute" ? { type: "time" as const, name: "" }
    : { type: "value" as const, name: axis === "uptime_seconds" ? "s" : "#" };
}

export function gcCoordinate(result: GcLogAnalysisResult | null, time: string): string | number {
  return gcAxis(result).type === "time" ? time : Number(time);
}

export function gcUnavailableReasons(result: GcLogAnalysisResult | null): string[] {
  return [...new Set(Object.values(result?.metadata.measurement_unavailable ?? {}).filter(Boolean))];
}

export function gcReasonKey(reason: string) {
  if (reason === "clock_reset") return "gcClockReset" as const;
  if (reason === "pause_exceeds_window") return "gcInvalidPauseWindow" as const;
  return "gcInsufficientEvidence" as const;
}
