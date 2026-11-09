import { useEffect, useMemo, useState } from "react";
import type { HttpCaptureTransactionRow } from "@/bridge/types";
import { HttpAnalyzerView } from "./HttpAnalyzerView";
import { useI18n } from "@/i18n/I18nProvider";
import {
  availableFidelities, availableMethods, availableMimeTypes, emptyFilter,
  filterTransactions, isFilterActive, type TransactionFilter,
} from "@/state/httpCapture";
import { availableResourceKinds } from "@/state/httpAnalyzer";
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
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [detail, setDetail] = useState<HttpCaptureTransactionRow | null>(null);
  const [error, setError] = useState<string | null>(null);
  const all = useMemo(() => transactions.map(liveAnalyzerRow), [transactions]);
  const rows = useMemo(() => filterTransactions(all, filter), [all, filter]);
  const selected = all.find((row) => row.id === selectedId);
  useEffect(() => {
    setDetail(null);
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
        bounded boundedNote={t("liveCaptureRowCapSuffix")}
      />
    </>
  );
}
