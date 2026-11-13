import { Call } from "@wailsio/runtime";
import { Dialogs } from "@/bridge/dialogs";

export async function saveChartArtifact(format: "png" | "svg" | "csv", data: string, filename: string, title: string): Promise<string | null> {
  const path = await Dialogs.SaveFile({Title:title,Filename:filename,Filters:[{DisplayName:format.toUpperCase(),Pattern:`*.${format}`}]});
  if (!path) return null;
  await Call.ByName("main.EngineService.SaveChartArtifact", {path,format,data});
  return path;
}
