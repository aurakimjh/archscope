import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function ExceptionAnalyzerPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("exceptionAnalyzer")}
      analyzer={{
        fileLabel: t("selectExceptionFile"),
        fileFilters: [
          { name: "Log files", extensions: ["log", "txt"] },
          { name: "All files", extensions: ["*"] },
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
