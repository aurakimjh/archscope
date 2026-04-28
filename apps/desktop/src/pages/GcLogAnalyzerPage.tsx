import { PlaceholderPage } from "./PlaceholderPage";

export function GcLogAnalyzerPage(): JSX.Element {
  return (
    <PlaceholderPage
      title="GC Log Analyzer"
      items={["GC pause timeline", "Heap usage trend", "Collector and cause breakdown"]}
    />
  );
}
