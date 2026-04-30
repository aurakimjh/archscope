import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function ThreadDumpAnalyzerPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("threadDumpAnalyzer")}
      analyzer={{
        fileLabel: t("selectThreadDumpFile"),
        executionType: "thread_dump",
        fileFilters: [
          { name: t("threadDumpFilesFilter"), extensions: ["txt", "log", "dump"] },
          { name: t("allFilesFilter"), extensions: ["*"] },
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
