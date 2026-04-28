import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function ExportCenterPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("exportCenter")}
      items={[
        t("jsonResultArchive"),
        t("csvTableExport"),
        t("reportReadyChartExport"),
      ]}
    />
  );
}
