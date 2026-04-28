import { useI18n } from "../i18n/I18nProvider";
import { PlaceholderPage } from "./PlaceholderPage";

export function ExceptionAnalyzerPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <PlaceholderPage
      title={t("exceptionAnalyzer")}
      items={[
        t("exceptionTrend"),
        t("rootCauseGrouping"),
        t("stackSignatureTable"),
      ]}
    />
  );
}
