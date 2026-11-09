import { Events } from "@wailsio/runtime";
import { useEffect, useMemo, useRef, useState } from "react";
import type { HttpCaptureTransactionRow } from "@/bridge/types";
import { HttpAnalyzerView } from "./HttpAnalyzerView";
import { useI18n } from "@/i18n/I18nProvider";
import {
  availableFidelities, availableMethods, availableMimeTypes, emptyFilter,
  filterTransactions, isFilterActive, type TransactionFilter,
} from "@/state/httpCapture";
import { httpProcessKey, availableResourceKinds } from "@/state/httpAnalyzer";
import { liveAnalyzerRow, type LiveCaptureExchange } from "@/state/liveHttpCapture";
import * as CaptureService from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app/captureservice";

export function LiveHttpAnalyzer({
  sessionId, transactions, follow, persisted,
}: {
  sessionId: string;
  transactions: LiveCaptureExchange[];
  follow: boolean;
  persisted: number;
}): React.JSX.Element {
  const { t } = useI18n();
  const [filter, setFilter] = useState<TransactionFilter>(emptyFilter);
  const [processKey, setProcessKey] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<HttpCaptureTransactionRow | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [searchText,setSearchText] = useState("");
  const [searchFields,setSearchFields] = useState(["url","headers","body","cookies","query","post"]);
  const [searchRows,setSearchRows] = useState<LiveCaptureExchange[]|null>(null);
  const [searchTotal,setSearchTotal] = useState(0);
  const [cursor,setCursor] = useState("");
  const [searchBusy,setSearchBusy] = useState(false);
  const attempt = useRef(0);
  const findRef = useRef<HTMLDetailsElement|null>(null);
  const queryRef = useRef<HTMLInputElement|null>(null);
  useEffect(() => () => { attempt.current++; },[]);
  useEffect(() => {
    const open = () => { if(findRef.current) findRef.current.open=true; queryRef.current?.focus(); };
    const off=Events.On("capture:command",(event:unknown) => {
      if((event as {data?:{command?:string}}).data?.command==="find") open();
    });
    const key=(event:KeyboardEvent) => { if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==="f") { event.preventDefault();open(); } };
    window.addEventListener("keydown",key);
    return () => { off();window.removeEventListener("keydown",key); };
  },[]);
  const search = async (nextCursor="") => {
    const current=++attempt.current;
    setSearchBusy(true);
    setError(null);
    try {
      const page=await CaptureService.SearchCaptureTransactions({
        sessionId,filter:{query:searchText,searchFields},cursor:nextCursor,limit:25,
      });
      if(current!==attempt.current) return;
      setSearchRows(page.items.map(item=>item.transaction as unknown as LiveCaptureExchange));
      setSearchTotal(page.total);
      setCursor(page.nextCursor??"");
      setSelectedId(null);
      setDetail(null);
      setProcessKey("");
    } catch(caught) { if(current===attempt.current) setError(String(caught)); }
    finally { if(current===attempt.current) setSearchBusy(false); }
  };
  const all = useMemo(() => (searchRows??transactions).map(liveAnalyzerRow), [transactions,searchRows]);
  const processes = useMemo(() => [...new Map(all.map((row) => [httpProcessKey(row),
    row.process ? `${row.process.name} [${row.process.pid}] · ${row.process.start_time}` : t("liveCaptureAttributionUnknown")
  ])).entries()], [all, t]);
  const scoped = useMemo(() => processKey ? all.filter((row) => httpProcessKey(row) === processKey) : all, [all, processKey]);
  const rows = useMemo(() => filterTransactions(scoped, filter), [scoped, filter]);
  const selected = all.find((row) => row.id === selectedId);
  useEffect(() => {
    setError(null);
    if (!selectedId || !sessionId) return;
    let disposed = false;
    void CaptureService.GetCaptureTransaction(sessionId, selectedId).then((exchange) => {
      if (!disposed && exchange) setDetail(liveAnalyzerRow(exchange as unknown as LiveCaptureExchange));
    }).catch((caught: unknown) => {
      if (!disposed) setError(String(caught));
    });
    return () => { disposed = true; };
  }, [sessionId, selectedId, selected?.state, persisted]);
  return (
    <>
      <details ref={findRef} className="shrink-0 text-xs">
        <summary className="cursor-pointer">{t("httpSearchStored")}</summary>
        <form className="flex flex-wrap items-center gap-2 py-1" onSubmit={(event)=>{event.preventDefault();void search();}}>
          <input ref={queryRef} aria-label={t("httpSearchQuery")} value={searchText} onChange={(event)=>{setSearchText(event.target.value);setCursor("");}} className="rounded border px-2 py-1" maxLength={4096} />
          {(["url","headers","body","cookies","query","post","stream"] as const).map(field=><label key={field} className="inline-flex gap-1">
            <input type="checkbox" checked={searchFields.includes(field)} onChange={(event)=>{setSearchFields(current=>event.target.checked?[...current,field]:current.filter(item=>item!==field));setCursor("");}} />
            {t(({url:"httpCaptureColUrl",headers:"httpAnalyzerTabHeaders",body:"httpAnalyzerTabBody",cookies:"httpAnalyzerTabCookies",query:"httpPayloadQuery",post:"httpPayloadForm",stream:"httpAnalyzerTabRaw"} as const)[field])}
          </label>)}
          <button type="submit" disabled={!sessionId||searchBusy||!searchFields.length}>{t("httpSearchRun")}</button>
          {searchRows!==null && <>
            <span>{searchTotal} {t("httpSearchMatches")}</span>
            <button type="button" disabled={!cursor||searchBusy} onClick={()=>void search(cursor)}>{t("httpSearchNext")}</button>
            <button type="button" onClick={()=>{attempt.current++;setSearchBusy(false);setSearchRows(null);setSelectedId(null);setDetail(null);}}>{t("httpSearchLive")}</button>
          </>}
        </form>
      </details>
      <label className="flex items-center gap-2 text-xs">
        {t("httpAnalyzerTabProcess")}
        <select aria-label={t("httpAnalyzerTabProcess")} value={processKey} className="h-7 min-w-0 rounded border px-2"
          onChange={(event) => { setProcessKey(event.target.value); setSelectedId(null); setDetail(null); }}>
          <option value="">{t("httpLiveAllProcesses")}</option>
          {processes.map(([key,label]) => <option key={key} value={key}>{label}</option>)}
        </select>
        <span className="font-mono">{rows.length} / {all.length}</span>
      </label>
      {error && <p role="alert" className="text-xs text-red-600">{error}</p>}
      <HttpAnalyzerView
        transactions={rows} total={all.length}
        methods={availableMethods(all)} mimes={availableMimeTypes(all)}
        fidelities={availableFidelities(all)} kinds={availableResourceKinds(all)}
        filter={filter} filterActive={isFilterActive(filter)}
        onFilter={(patch) => setFilter((current) => ({ ...current, ...patch }))}
        onReset={() => setFilter(emptyFilter)}
        selectedId={selectedId} selectedDetail={detail}
        onSelect={setSelectedId} follow={follow}
        bounded boundedNote={searchRows===null?t("liveCaptureRowCapSuffix"):`${searchRows.length} / ${searchTotal} ${t("httpSearchMatches")}`}
      />
    </>
  );
}
