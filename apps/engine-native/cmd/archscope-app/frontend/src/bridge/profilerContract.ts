import type { ProfilerMetadata } from "./types";
type CheckedResult<T> = Omit<T, "metadata"> & { metadata: ProfilerMetadata };
type CheckedStage<T> = Omit<T, "diagnostics"> & { diagnostics: { message: string; [key: string]: unknown } | null };

type ObjectValue = Record<string, unknown>;
function object(value: unknown): ObjectValue {
  if (value == null || typeof value !== "object" || Array.isArray(value)) throw new Error("Invalid profiler object");
  return value as ObjectValue;
}
function strings(value: ObjectValue, keys: string[]) {
  for (const key of keys) if (typeof value[key] !== "string") throw new Error(`Invalid profiler ${key}`);
}
function numbers(value: ObjectValue, keys: string[]) {
  for (const key of keys) if (typeof value[key] !== "number" || !Number.isFinite(value[key])) throw new Error(`Invalid profiler ${key}`);
}
function diffRow(value: unknown) { const row=object(value); strings(row,["stack"]); numbers(row,["delta","delta_ratio"]); }
function diagnostics(value: unknown) {
  if (value === null) return;
  const d=object(value);
  numbers(d,["total_lines","parsed_records","skipped_lines"]);
  for (const count of Object.values(d.skipped_by_reason===null ? {} : object(d.skipped_by_reason))) if (typeof count!=="number" || !Number.isFinite(count)) throw new Error("Invalid profiler diagnostic count");
  for (const key of ["samples","warnings","errors"]) {
    if (d[key]===null || (d[key]===undefined && key!=="samples")) continue;
    if (!Array.isArray(d[key])) throw new Error("Invalid profiler diagnostic samples");
    for (const entry of d[key]) { const sample=object(entry); numbers(sample,["line_number"]); strings(sample,["reason","message","raw_preview"]); }
  }
  for (const key of ["warning_count","error_count","dropped_stacks","over_depth_records","max_observed_depth","bytes_read"]) if (d[key]!==undefined) numbers(d,[key]);
  for (const key of ["format","source_file","dropped_stack_reason"]) if (d[key]!=null) strings(d,[key]);
}

// Validate the metadata fields consumed by the UI before exposing the typed
// facade. Unknown optional metadata remains opaque on the original object.
export function checkedProfilerResult<T>(value: T): CheckedResult<T> {
  const result=object(value); const metadata=object(result.metadata);
  strings(metadata,["parser","schema_version"]);
  diagnostics(metadata.diagnostics);
  if (metadata.progress_log_path!==undefined) strings(metadata,["progress_log_path"]);
  const scope=object(metadata.timeline_scope);
  strings(scope,["mode","match_mode","view_mode"]); numbers(scope,["base_samples","total_samples"]);
  if (scope.base_method!==null) strings(scope,["base_method"]);
  if (scope.base_ratio_of_total!==null) numbers(scope,["base_ratio_of_total"]);
  if (scope.warnings!==null) {
    if (!Array.isArray(scope.warnings)) throw new Error("Invalid profiler timeline warnings");
    for (const warning of scope.warnings) strings(object(warning),["code","message"]);
  }
  if (metadata.diff_summary!==undefined) {
    const summary=object(metadata.diff_summary);
    numbers(summary,["baseline_total","target_total","common_paths","added_paths","removed_paths"]);
    for (const key of ["max_increase","max_decrease"]) if (summary[key]!=null) diffRow(summary[key]);
  }
  if (metadata.diff_tables!==undefined) {
    const tables=object(metadata.diff_tables);
    for (const key of ["biggest_increases","biggest_decreases"]) {
      if (!Array.isArray(tables[key])) throw new Error("Invalid profiler diff rows");
      tables[key].forEach(diffRow);
    }
  }
  return value as CheckedResult<T>;
}
export function checkedProfilerStages<T>(value: T[]): CheckedStage<T>[] {
  if (!Array.isArray(value)) throw new Error("Invalid profiler drilldown stages");
  for (const stage of value) {
    const d=object(stage).diagnostics;
    if (d!==null) strings(object(d),["message"]);
  }
  return value as CheckedStage<T>[];
}
