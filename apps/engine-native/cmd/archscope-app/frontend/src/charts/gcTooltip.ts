import type { GcEventTiming, GcLogAnalysisResult, GcPauseTimelinePoint } from "../bridge/types.js";
import { gcCoordinate } from "../state/gcClock.js";

type Timing = GcEventTiming & { time?: string };
type Labels = {
  pauseAxis: string;
  eventTime?: string;
  eventUptime?: string;
  eventType?: string;
  eventCause?: string;
};
type TooltipItem = {
  axisValue?: string | number;
  value?: unknown;
  seriesId?: string;
  seriesName?: string;
  data?: { value?: unknown; gc?: Timing };
};

const escape = (value: unknown): string => String(value ?? "—").replace(/[&<>"']/g,
  ch => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[ch]!);
const finite = (value: unknown): value is number => typeof value === "number" && Number.isFinite(value);

export function gcPauseDuration(row: GcPauseTimelinePoint): number | null {
  // Old saved results have value only; new results distinguish missing from zero.
  const value = Object.prototype.hasOwnProperty.call(row, "pause_ms") ? row.pause_ms : row.value;
  return finite(value) ? value : null;
}

// Both charts use the original event clock, rather than a display-rounded axis
// label, and preserve same-time events. Never join by a rounded timestamp.
export function gcEventTooltip(result: GcLogAnalysisResult | null, labels: Labels, heap: boolean) {
  const key = (value: string | number): string => {
    if (result?.metadata.time_axis === "absolute" || typeof value === "string" && /^\d{4}-\d\d-\d\dT/.test(value)) {
      const time = typeof value === "number" ? value : Date.parse(value);
      return Number.isFinite(time) ? String(time) : String(value);
    }
    return String(Number(value));
  };
  const timeline = new Map<string, Timing[]>();
  for (const row of result?.series.pause_timeline ?? []) {
    const coordinate = key(gcCoordinate(result, row.time));
    const list = timeline.get(coordinate) ?? [];
    list.push({ ...row, pause_ms: gcPauseDuration(row) });
    timeline.set(coordinate, list);
  }
  return (params: unknown): string => {
    const items = (Array.isArray(params) ? params : [params]).filter(Boolean) as TooltipItem[];
    const events = new Map<string, Timing>();
    const add = (event: Timing) => {
      const id = event.event_number != null ? String(event.event_number)
        : JSON.stringify([event.timestamp ?? event.time, event.gc_type, event.pause_ms]);
      events.set(id, { ...events.get(id), ...event });
    };
    for (const item of items) {
      const event = item.data?.gc;
      const value = item.data?.value ?? item.value;
      const coordinate = event?.time ?? item.axisValue ?? (Array.isArray(value) ? value[0] : undefined);
      // Include every retained event sharing this exact coordinate.
      const matches = coordinate == null ? [] : timeline.get(key(coordinate));
      for (const match of matches ?? []) add(match);
      if (event && (Object.prototype.hasOwnProperty.call(event, "pause_ms") || !matches?.length)) add(event);
    }
    const parts: string[] = [];
    for (const event of events.values()) {
      const axis = result?.metadata.time_axis;
      if (event.timestamp || axis === "absolute") parts.push(`${escape(labels.eventTime ?? "Timestamp")}: ${escape(event.timestamp ?? event.time)}`);
      else if (!finite(event.uptime_sec) && axis === "uptime_seconds") parts.push(`${escape(labels.eventUptime ?? "Uptime (s)")}: ${escape(event.time)}`);
      else if (!finite(event.uptime_sec)) parts.push(`${escape(labels.eventTime ?? "Timestamp")}: — (#${escape(event.event_number ?? event.time)})`);
      if (finite(event.uptime_sec)) parts.push(`${escape(labels.eventUptime ?? "Uptime (s)")}: ${event.uptime_sec}`);
      parts.push(`${escape(labels.eventType ?? "GC type")}: ${escape(event.gc_type)}`);
      parts.push(`${escape(labels.pauseAxis)}: ${finite(event.pause_ms) ? `${event.pause_ms} ms` : "—"}`);
      if (event.cause) parts.push(`${escape(labels.eventCause ?? "Cause")}: ${escape(event.cause)}`);
      parts.push("");
    }
    if (!events.size && items.length) parts.push(`${escape(labels.eventTime ?? "Timestamp")}: ${escape(items[0].axisValue)}`);
    if (heap) for (const item of items) {
      if (item.seriesId === "gc-event-times" || item.seriesId === "gc-pause-overlay") continue;
      const value = item.data?.value ?? item.value;
      const amount = Array.isArray(value) ? value[1] : value;
      if (finite(amount)) parts.push(`${escape(item.seriesName)}: ${amount} MB`);
    }
    return parts.join("<br/>");
  };
}
