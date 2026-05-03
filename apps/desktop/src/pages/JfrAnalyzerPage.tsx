import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function JfrAnalyzerPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("jfrAnalyzer")}
      analyzer={{
        fileLabel: t("selectJfrJsonFile"),
        fileDescription: t("jfrFileDescription"),
        executionType: "jfr_recording",
        fileFilters: [
          { name: t("jsonFilesFilter"), extensions: ["json"] },
          { name: t("allFilesFilter"), extensions: ["*"] },
        ],
      }}
      items={[
        t("jfrEventDistribution"),
        t("jfrTopExecutionSamples"),
        t("jfrGcSummary"),
      ]}
    />
  );
}
