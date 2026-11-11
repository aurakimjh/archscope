import type { ProfilerMetadata } from "./types";

export type ProfilerAnalysisResult = Omit<import("../../bindings/github.com/aurakimjh/archscope/apps/engine-native/internal/profiler/models").AnalysisResult, "metadata"> & { metadata: ProfilerMetadata };
export type ProfilerDrilldownStage = Omit<import("../../bindings/github.com/aurakimjh/archscope/apps/engine-native/internal/profiler/models").DrilldownStage, "diagnostics"> & { diagnostics: { message: string; [key: string]: unknown } | null };
