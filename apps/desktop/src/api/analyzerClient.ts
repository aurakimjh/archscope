import { sampleAnalysisResult } from "../charts/sampleCharts";

export type SampleAnalysisResult = typeof sampleAnalysisResult;

export async function loadSampleAnalysisResult(): Promise<SampleAnalysisResult> {
  return sampleAnalysisResult;
}
