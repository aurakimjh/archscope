import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function GcLogAnalyzerPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("gcLogAnalyzer")}
      analyzer={{
        fileLabel: t("selectGcLogFile"),
        fileFilters: [
          { name: "Log files", extensions: ["log", "txt"] },
          { name: "All files", extensions: ["*"] },
        ],
      }}
      items={[
        t("gcPauseTimeline"),
        t("heapUsageTrend"),
        t("collectorCauseBreakdown"),
      ]}
    />
  );
}
