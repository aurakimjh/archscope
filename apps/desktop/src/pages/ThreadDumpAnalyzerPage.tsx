import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function ThreadDumpAnalyzerPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("threadDumpAnalyzer")}
      analyzer={{
        fileLabel: t("selectThreadDumpFile"),
        fileFilters: [
          { name: "Thread dump files", extensions: ["txt", "log", "dump"] },
          { name: "All files", extensions: ["*"] },
        ],
      }}
      items={[
        t("threadStateDistribution"),
        t("blockedThreadGrouping"),
        t("stackSignatureAnalysis"),
      ]}
    />
  );
}
