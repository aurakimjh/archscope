import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function ExceptionAnalyzerPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("exceptionAnalyzer")}
      analyzer={{
        fileLabel: t("selectExceptionFile"),
        executionType: "exception_stack",
        fileFilters: [
          { name: t("logFilesFilter"), extensions: ["log", "txt"] },
          { name: t("allFilesFilter"), extensions: ["*"] },
        ],
      }}
      items={[
        t("exceptionTrend"),
        t("rootCauseGrouping"),
        t("stackSignatureTable"),
      ]}
    />
  );
}
