import { inspectForm } from "@/state/httpPayload";
import { Dialogs } from "@/bridge/dialogs";
import { useEffect, useRef, useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import { HttpAnalyzerView } from "./HttpAnalyzerView";
import { emptyFilter } from "@/state/httpCapture";
import {
  liveAnalyzerRow,
  type LiveCaptureExchange,
} from "@/state/liveHttpCapture";
import type { HttpCaptureTransactionRow } from "@/bridge/types";
import {
  HTTP_BUILDER_COPY,
  HTTP_BUILDER_DRAG,
  builderHeaders,
  parseBuilderHeaders,
  type BuilderCopy,
} from "@/state/httpRequestBuilder";
import * as CaptureService from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app/captureservice";
type Part = {
  name: string;
  value: string;
  path?: string;
  filename?: string;
  contentType?: string;
};
export function HttpRequestBuilder({ onReveal }: { onReveal?: () => void }): React.JSX.Element {
  const { t } = useI18n();
  const panel = useRef<HTMLDetailsElement>(null);
  const [method, setMethod] = useState("GET");
  const [url, setURL] = useState("");
  const [headers, setHeaders] = useState("");
  const [body, setBody] = useState("");
  const [mode, setMode] = useState("raw");
  const [parts, setParts] = useState<Part[]>([]);
  const [timeout, setTimeoutSeconds] = useState(30);
  const [busy, setBusy] = useState(false);
  const [copying, setCopying] = useState(false);
  const [dragActive, setDragActive] = useState(false);
  const [error, setError] = useState("");
  const [hint, setHint] = useState("");
  const [response, setResponse] = useState<HttpCaptureTransactionRow | null>(
    null,
  );
  const active = useRef("");
  const copyAttempt = useRef(0);
  const load = async (value: BuilderCopy) => {
    if (active.current) return;
    const attempt = ++copyAttempt.current;
    // Copy may originate in the inspector while the outer file tools are closed.
    onReveal?.();
    if (panel.current) panel.current.open = true;
    setError("");
    setCopying(true);
    try {
      const row = value.sessionId
        ? liveAnalyzerRow(
            (await CaptureService.GetCaptureTransaction(
              value.sessionId,
              value.row.id,
            )) as unknown as LiveCaptureExchange,
          )
        : value.row;
      if (attempt !== copyAttempt.current) return;
      setMethod(row.method);
      setURL(row.url);
      setHeaders(builderHeaders(row));
      setBody(row.request.bodyPreview ?? "");
      setMode("raw");
      setParts([]);
      const form = inspectForm(row.request);
      if (
        form.state === "available" &&
        row.request.contentType?.startsWith("application/x-www-form-urlencoded")
      ) {
        setMode("form");
        setParts(
          form.fields.map((field) => ({
            name: field.name,
            value: field.value,
          })),
        );
      }
      setResponse(null);
      setHint(t("httpBuilderCopyHint"));
    } catch (caught) {
      if (attempt === copyAttempt.current) setError(String(caught));
    } finally {
      if (attempt === copyAttempt.current) setCopying(false);
    }
  };
  const loadRef = useRef(load);
  loadRef.current = load;
  useEffect(() => {
    const listener = (event: Event) =>
      void loadRef.current((event as CustomEvent<BuilderCopy>).detail);
    window.addEventListener(HTTP_BUILDER_COPY, listener);
    return () => {
      copyAttempt.current++;
      window.removeEventListener(HTTP_BUILDER_COPY, listener);
      if (active.current)
        void CaptureService.CancelBuilderRequest(active.current);
    };
  }, []);
  useEffect(() => {
    const start = (event: DragEvent) => {
      if (event.dataTransfer?.types.includes(HTTP_BUILDER_DRAG))
        setDragActive(true);
    };
    const end = () => setDragActive(false);
    window.addEventListener("dragstart", start);
    window.addEventListener("dragend", end);
    window.addEventListener("drop", end);
    return () => {
      window.removeEventListener("dragstart", start);
      window.removeEventListener("dragend", end);
      window.removeEventListener("drop", end);
    };
  }, []);
  const drop = (event: React.DragEvent) => {
    const data = event.dataTransfer.getData(HTTP_BUILDER_DRAG);
    if (!data) return;
    event.preventDefault();
    event.stopPropagation();
    setDragActive(false);
    try {
      void load(JSON.parse(data) as BuilderCopy);
      panel.current?.scrollIntoView({ block: "start" });
    } catch (caught) {
      setError(String(caught));
    }
  };
  const send = async () => {
    if (active.current || copying) return;
    setError("");
    setResponse(null);
    copyAttempt.current++;
    const id = crypto.randomUUID();
    active.current = id;
    setBusy(true);
    try {
      const exchange = await CaptureService.SendBuilderRequest({
        id,
        method,
        url,
        headers: parseBuilderHeaders(headers),
        bodyMode: mode,
        body,
        parts,
        timeoutSeconds: timeout,
      });
      if (active.current === id)
        setResponse(
          liveAnalyzerRow(exchange as unknown as LiveCaptureExchange),
        );
    } catch (caught) {
      setError(String(caught));
    } finally {
      if (active.current === id) active.current = "";
      setBusy(false);
    }
  };
  const chooseFile = async (index: number) => {
    try {
      const picked = await Dialogs.OpenFile({ Title: t("httpBuilderFile") });
      const path = Array.isArray(picked) ? picked[0] : picked;
      if (path)
        setParts((current) =>
          current.map((part, i) => (i === index ? { ...part, path } : part)),
        );
    } catch (caught) {
      setError(String(caught));
    }
  };
  const patchPart = (index: number, patch: Partial<Part>) =>
    setParts((current) =>
      current.map((part, i) => (i === index ? { ...part, ...patch } : part)),
    );
  return (
    <>
      {dragActive && (
        <div
          className="fixed right-4 top-16 z-50 rounded border-2 border-primary bg-card p-4 shadow-lg"
          onDragOver={(event) => event.preventDefault()}
          onDrop={drop}
          role="region"
          aria-label={t("httpBuilderDrop")}
        >
          {t("httpBuilderDrop")}
        </div>
      )}
      <details
        ref={panel}
        className="rounded border bg-card p-2"
        onDragOver={(event) => {
          if (event.dataTransfer.types.includes(HTTP_BUILDER_DRAG))
            event.preventDefault();
        }}
        onDrop={drop}
      >
        <summary className="cursor-pointer text-sm font-semibold">
          {t("httpBuilderTitle")}
        </summary>
        <p className="my-1 text-xs text-muted-foreground">
          {t("httpBuilderHint")}
        </p>
        <form
          className="flex flex-col gap-2 text-xs"
          onSubmit={(event) => {
            event.preventDefault();
            void send();
          }}
        >
          <fieldset disabled={busy || copying} className="flex flex-col gap-2">
            <div className="flex gap-2">
              <input
                aria-label={t("httpCaptureColMethod")}
                value={method}
                onChange={(event) => setMethod(event.target.value)}
                maxLength={32}
                className="w-24 rounded border px-2 py-1"
              />
              <input
                aria-label={t("httpBuilderURL")}
                type="url"
                required
                value={url}
                onChange={(event) => setURL(event.target.value)}
                className="min-w-0 flex-1 rounded border px-2 py-1"
                maxLength={16384}
              />
            </div>
            <label>
              {t("httpAnalyzerTabHeaders")}
              <textarea
                aria-label={t("httpBuilderHeaders")}
                value={headers}
                onChange={(event) => setHeaders(event.target.value)}
                rows={4}
                maxLength={65536}
                className="block w-full rounded border p-2 font-mono"
                placeholder="Content-Type: application/json"
              />
            </label>
            <label>
              {t("httpBuilderBodyMode")}{" "}
              <select
                aria-label={t("httpBuilderBodyMode")}
                value={mode}
                onChange={(event) => setMode(event.target.value)}
              >
                <option value="raw">Raw</option>
                <option value="form">application/x-www-form-urlencoded</option>
                <option value="multipart">multipart/form-data</option>
              </select>
            </label>
            {mode === "raw" ? (
              <textarea
                aria-label={t("httpBuilderBody")}
                value={body}
                onChange={(event) => setBody(event.target.value)}
                rows={4}
                maxLength={10 * 1024 * 1024}
                className="w-full rounded border p-2 font-mono"
              />
            ) : (
              <div className="flex flex-col gap-1">
                {parts.map((part, index) => (
                  <div className="flex flex-wrap gap-1" key={index}>
                    <input
                      aria-label={`${t("httpBuilderPartName")} ${index + 1}`}
                      value={part.name}
                      onChange={(event) =>
                        patchPart(index, { name: event.target.value })
                      }
                      className="rounded border px-1"
                    />
                    <input
                      aria-label={`${t("httpBuilderPartValue")} ${index + 1}`}
                      value={part.value}
                      onChange={(event) =>
                        patchPart(index, { value: event.target.value })
                      }
                      className="min-w-0 flex-1 rounded border px-1"
                      disabled={Boolean(part.path)}
                    />
                    {mode === "multipart" && (
                      <>
                        <button
                          type="button"
                          onClick={() => void chooseFile(index)}
                        >
                          {t("httpBuilderFile")}
                        </button>
                        {part.path && (
                          <>
                            <span title={part.path}>
                              {part.path.split(/[\\/]/).pop()}
                            </span>
                            <input
                              aria-label={`${t("httpBuilderFilename")} ${index + 1}`}
                              placeholder={t("httpBuilderFilename")}
                              value={part.filename ?? ""}
                              onChange={(event) =>
                                patchPart(index, {
                                  filename: event.target.value,
                                })
                              }
                            />
                            <button
                              type="button"
                              onClick={() =>
                                patchPart(index, { path: undefined })
                              }
                            >
                              {t("httpBuilderClearFile")}
                            </button>
                          </>
                        )}
                      </>
                    )}
                    <button
                      type="button"
                      onClick={() =>
                        setParts((current) =>
                          current.filter((_, i) => i !== index),
                        )
                      }
                    >
                      {t("httpBuilderRemove")}
                    </button>
                  </div>
                ))}
                <button
                  type="button"
                  disabled={parts.length >= 128}
                  className="self-start"
                  onClick={() =>
                    setParts((current) => [...current, { name: "", value: "" }])
                  }
                >
                  {t("httpBuilderAdd")}
                </button>
              </div>
            )}
            <label>
              {t("httpBuilderTimeout")}{" "}
              <input
                aria-label={t("httpBuilderTimeout")}
                type="number"
                min={1}
                max={120}
                value={timeout}
                onChange={(event) =>
                  setTimeoutSeconds(Number(event.target.value))
                }
                className="w-20 rounded border px-1"
              />
            </label>
          </fieldset>
          <div className="flex gap-3">
            <button
              type="submit"
              disabled={busy || copying || !url}
              className="rounded border px-3 py-1"
            >
              {busy ? t("httpBuilderSending") : t("httpBuilderSend")}
            </button>
            {busy && (
              <button
                type="button"
                onClick={() =>
                  void CaptureService.CancelBuilderRequest(
                    active.current,
                  ).catch((caught) => setError(String(caught)))
                }
              >
                {t("httpBuilderCancel")}
              </button>
            )}
          </div>
        </form>
        {hint && <p className="mt-1 text-xs text-amber-700">{hint}</p>}
        {error && (
          <p role="alert" className="text-xs text-red-600">
            {error}
          </p>
        )}
        {response && (
          <div className="mt-2">
            <p className="text-xs">{t("httpBuilderResponse")}</p>
            <HttpAnalyzerView
              commandsEnabled={false}
              transactions={[response]}
              total={1}
              methods={[response.method]}
              mimes={[response.response.contentType ?? ""]}
              fidelities={[response.fidelity]}
              kinds={[]}
              filter={emptyFilter}
              filterActive={false}
              onFilter={() => {}}
              onReset={() => {}}
              selectedId={response.id}
              onSelect={() => {}}
            />
          </div>
        )}
      </details>
    </>
  );
}
