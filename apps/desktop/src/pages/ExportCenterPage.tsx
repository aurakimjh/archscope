import { PlaceholderPage } from "./PlaceholderPage";

export function ExportCenterPage(): JSX.Element {
  return (
    <PlaceholderPage
      title="Export Center"
      items={["JSON result archive", "CSV table export", "Report-ready chart export"]}
    />
  );
}
