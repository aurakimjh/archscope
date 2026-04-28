import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function ChartStudioPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("chartStudio")}
      items={[
        t("editableTitlesAndLabels"),
        t("topNAndTimeBucketControls"),
        t("exportPresets"),
      ]}
    />
  );
}
