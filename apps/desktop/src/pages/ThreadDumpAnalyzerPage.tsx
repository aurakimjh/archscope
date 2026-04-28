import { PlaceholderPage } from "./PlaceholderPage";

export function ThreadDumpAnalyzerPage(): JSX.Element {
  return (
    <PlaceholderPage
      title="Thread Dump Analyzer"
      items={["Thread state distribution", "Blocked thread grouping", "Stack signature analysis"]}
    />
  );
}
