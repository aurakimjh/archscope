import { useEffect, useRef, useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import * as Service from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app/captureservice";
import type { HttpFileSource } from "./HttpSessionFiles";
import type { Plan } from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/internal/capture/browsercache/models";
export function HttpCacheTools({
  source,
}: {
  source: HttpFileSource;
}): React.JSX.Element {
  const { t } = useI18n();
  const [available, setAvailable] = useState(false),
    [kind, setKind] = useState("cache"),
    [scope, setScope] = useState("recorded"),
    [plan, setPlan] = useState<Plan | null>(null),
    [busy, setBusy] = useState(false),
    [message, setMessage] = useState("");
  const identity = JSON.stringify([source.sessionId, source.path, kind, scope]);
  const sourceRef = useRef(identity);
  sourceRef.current = identity;
  useEffect(
    () => () => {
      sourceRef.current = "";
    },
    [],
  );
  useEffect(() => {
    let disposed = false;
    void Service.GetCaptureCacheCapability()
      .then((value) => {
        if (!disposed) setAvailable(value.available);
      })
      .catch(() => {});
    return () => {
      disposed = true;
    };
  }, []);
  useEffect(() => {
    setPlan(null);
    setBusy(false);
    setMessage("");
  }, [kind, scope, source.path, source.sessionId]);
  const preview = async () => {
    const expected = identity;
    setBusy(true);
    setMessage("");
    setPlan(null);
    try {
      const value = await Service.PlanCaptureCacheCleanup({
        source,
        kind,
        scope,
      });
      if (sourceRef.current === expected) setPlan(value);
    } catch (error) {
      if (sourceRef.current === expected) setMessage(String(error));
    } finally {
      if (sourceRef.current === expected) setBusy(false);
    }
  };
  const remove = async () => {
    const expected = identity;
    if (
      !plan ||
      !window.confirm(
        t("httpCacheConfirm").replace("{count}", String(plan.count)),
      )
    )
      return;
    setBusy(true);
    try {
      const result = await Service.ExecuteCaptureCacheCleanup(plan.id);
      if (sourceRef.current !== expected) return;
      setMessage(
        `${t("httpCacheDeleted")}: ${result.deleted} · ${t("httpCacheChanged")}: ${result.changed} · ${t("httpCacheFailed")}: ${result.failed}`,
      );
      setPlan(null);
    } catch (error) {
      if (sourceRef.current === expected) setMessage(String(error));
    } finally {
      if (sourceRef.current === expected) setBusy(false);
    }
  };
  return (
    <details className="shrink-0 border-b px-2 py-1 text-xs">
      <summary>{t("httpCacheTitle")}</summary>
      <p className="my-1 text-muted-foreground">{t("httpCacheScopeNote")}</p>
      <div className="flex flex-wrap gap-2">
        <select
          aria-label={t("httpCacheTitle")}
          disabled={busy}
          value={kind}
          onChange={(e) => setKind(e.target.value)}
        >
          <option value="cache">{t("httpCacheCache")}</option>
          <option value="cookies">{t("httpCacheCookies")}</option>
        </select>
        <select
          aria-label={t("httpFilesScope")}
          disabled={busy}
          value={scope}
          onChange={(e) => setScope(e.target.value)}
        >
          <option value="recorded">{t("httpCacheRecorded")}</option>
          <option value="all">{t("httpCacheAll")}</option>
        </select>
        <button disabled={!available || busy} onClick={() => void preview()}>
          {t("httpCachePreview")}
        </button>
        {plan && (
          <>
            <span>{plan.count}</span>
            <button
              disabled={busy || plan.count === 0}
              onClick={() => void remove()}
            >
              {t("httpCacheDelete")}
            </button>
          </>
        )}
      </div>
      {!available && <p>{t("httpCacheWindows")}</p>}
      {plan && (
        <ul className="max-h-32 overflow-auto">
          {plan.items.map((item) => (
            <li key={item.id}>{item.url}</li>
          ))}
        </ul>
      )}
      {message && <p role="status">{message}</p>}
    </details>
  );
}
