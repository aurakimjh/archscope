import { PlaceholderPage } from "./PlaceholderPage";

export function SettingsPage(): JSX.Element {
  return (
    <PlaceholderPage
      title="Settings"
      items={["Engine path", "Default chart theme", "Locale and report label language"]}
    />
  );
}
