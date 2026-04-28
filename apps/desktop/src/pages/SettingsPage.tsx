import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function SettingsPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("settings")}
      items={[
        t("enginePath"),
        t("defaultChartTheme"),
        t("localeAndReportLanguage"),
      ]}
    />
  );
}
