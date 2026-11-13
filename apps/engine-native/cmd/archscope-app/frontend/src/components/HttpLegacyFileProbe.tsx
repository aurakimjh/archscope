import { Dialogs } from "@/bridge/dialogs";
import { useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import * as CaptureService from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app/captureservice";
export function HttpLegacyFileProbe(): React.JSX.Element {
  const { t } = useI18n();
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [result, setResult] = useState<{
    bytes: number;
    sha256: string;
    container: string;
  } | null>(null);
  const probe = async () => {
    setBusy(true);
    setError("");
    setResult(null);
    try {
      const picked = await Dialogs.OpenFile({
        Title: t("httpHalogProbe"),
        Filters: [
          { DisplayName: "Legacy HTTP Analyzer log", Pattern: "*.halog" },
        ],
      });
      const path = Array.isArray(picked) ? picked[0] : picked;
      if (path) setResult(await CaptureService.ProbeLegacyCapture(path));
    } catch (caught) {
      setError(String(caught));
    } finally {
      setBusy(false);
    }
  };
  return (
    <span className="inline-flex min-w-0 flex-wrap items-center gap-2 text-xs">
      <button
        type="button"
        disabled={busy}
        onClick={() => void probe()}
        className="rounded border px-2 py-1"
      >
        {t("httpHalogProbe")}
      </button>
      {result && (
        <span className="break-all text-amber-700">
          {t("httpHalogUnverified")} · {result.container} · {result.bytes} B ·
          SHA256 {result.sha256}
        </span>
      )}
      {error && <span role="alert">{error}</span>}
    </span>
  );
}
