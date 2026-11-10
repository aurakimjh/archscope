import { useEffect, useRef, useState } from "react";
import type { CaptureHTTPMessage, LegacyPreview } from "@/bridge/types";
import { useI18n } from "@/i18n/I18nProvider";
import { HttpJsonTree } from "./HttpPayloadView";
import * as CaptureService from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app/captureservice";
export function HttpLegacyInspector({
  message,
}: {
  message: CaptureHTTPMessage | undefined;
}): React.JSX.Element {
  const { t } = useI18n();
  const [format, setFormat] = useState("amf0"),
    [encoded, setEncoded] = useState("");
  const [manual, setManual] = useState<LegacyPreview | null>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const attempt = useRef(0);
  useEffect(() => {
    attempt.current++;
    setManual(null);
    setEncoded("");
    setBusy(false);
    setError("");
  }, [message]);
  useEffect(
    () => () => {
      attempt.current++;
    },
    [],
  );
  const decode = async () => {
    const current = ++attempt.current;
    setBusy(true);
    setError("");
    try {
      const result = await CaptureService.InspectLegacyContent(format, encoded);
      if (current === attempt.current)
        setManual(result as LegacyPreview | null);
    } catch (caught) {
      if (current === attempt.current) setError(String(caught));
    } finally {
      if (current === attempt.current) setBusy(false);
    }
  };
  const shown = manual ?? message?.legacyPreview;
  return (
    <div className="space-y-2 text-xs">
      <p className="text-muted-foreground">{t("httpLegacyHint")}</p>
      {shown ? (
        <>
          <p role="status">
            {shown.format} ·{" "}
            {t(
              (
                {
                  available: "httpLegacyAvailable",
                  invalid: "httpLegacyInvalid",
                  limited: "httpLegacyLimited",
                  unsupported: "httpLegacyUnsupported",
                } as const
              )[shown.state as "available"] ?? "httpLegacyUnsupported",
            )}{" "}
            · {shown.consumed} B · {t("httpLegacyTail")}: {shown.trailingBytes}{" "}
            B
          </p>
          {shown.error && <p>{shown.error}</p>}
          {shown.redacted && <p>{t("httpCaptureDetailMessageRedacted")}</p>}
          {shown.root && (
            <>
              <HttpJsonTree value={shown.root} />
              <details>
                <summary>{t("httpLegacyText")}</summary>
                <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all font-mono">
                  {JSON.stringify(shown.root, null, 2).slice(0, 65536)}
                </pre>
              </details>
            </>
          )}
        </>
      ) : (
        <p>{t("httpPayloadUnavailable")}</p>
      )}
      <details>
        <summary>{t("httpLegacyManual")}</summary>
        <select
          disabled={busy}
          aria-label={t("httpLegacyFormat")}
          value={format}
          onChange={(e) => {
            attempt.current++;
            setManual(null);
            setFormat(e.target.value);
            setBusy(false);
          }}
        >
          {["amf0", "amf3", "amf_packet", "viewstate"].map((value) => (
            <option key={value} value={value}>
              {value}
            </option>
          ))}
        </select>
        <textarea
          disabled={busy}
          aria-label={t("httpLegacyInput")}
          value={encoded}
          onChange={(e) => setEncoded(e.target.value)}
          maxLength={1398104}
          rows={3}
          className="block w-full rounded border p-1 font-mono"
        />
        <button
          type="button"
          disabled={busy || !encoded}
          onClick={() => void decode()}
        >
          {t("httpLegacyDecode")}
        </button>
        {error && <p role="alert">{error}</p>}
      </details>
    </div>
  );
}
