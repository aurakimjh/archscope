import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/I18nProvider";
import { safeDisplayText } from "@/state/resultProviders";
import { filterMetricSeries, indexMetricSeries, pageMetricSeries, updateMetricSelection, type MetricSeriesChoice } from "@/state/metricSeriesSelection";

export function MetricSeriesPicker({ series, selected, onChange }: { series: MetricSeriesChoice[]; selected: string[]; onChange: (ids: string[]) => void }): React.JSX.Element {
  const { locale } = useI18n(); const w = (en: string, ko: string): string => locale === "ko" ? ko : en;
  const [search, setSearch] = useState(""); const [requestedPage, setPage] = useState(0);
  useEffect(() => { setSearch(""); setPage(0); }, [series]);
  const index = useMemo(() => indexMetricSeries(series), [series]);
  const matches = useMemo(() => filterMetricSeries(index, search), [index, search]);
  const selectedSet = useMemo(() => new Set(selected), [selected]);
  const matchingSelected = matches.reduce((count, value) => count + Number(selectedSet.has(value.id)), 0);
  const page = pageMetricSeries(matches, requestedPage);
  return <section className="space-y-2" aria-label={w("Metric series selection", "메트릭 시계열 선택")}>
    <label className="block">{w("Search metric series", "메트릭 시계열 검색")} <input className="rounded border border-border bg-background p-1" maxLength={256} value={search} onChange={event => { setSearch(event.target.value); setPage(0); }} placeholder={w("Name, unit, resource or attributes", "이름, 단위, resource 또는 속성")} /></label>
    <p role="status">{w("Matches", "검색 결과")} {matches.length} / {series.length} · {w("Selected", "선택")} {selected.length} · {w("Selected outside filter", "필터 밖 선택")} {selected.length - matchingSelected}</p>
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" disabled={!matches.length || matchingSelected === matches.length} onClick={() => onChange(updateMetricSelection(selected, matches, true))}>{w("Select all matches", "검색 결과 전체 선택")}</Button>
      <Button variant="outline" disabled={!matchingSelected} onClick={() => onChange(updateMetricSelection(selected, matches, false))}>{w("Clear matches", "검색 결과 선택 해제")}</Button>
      <Button variant="outline" disabled={!selected.length} onClick={() => onChange([])}>{w("Clear selection", "전체 선택 해제")}</Button>
    </div>
    <p className="text-xs">{w("Filter actions include every matching page. Other selections are retained. Large selections may exceed the request limit; narrow the selection before saving or querying.", "검색 결과 선택은 일치하는 모든 페이지에 적용되며 다른 선택은 유지합니다. 선택이 요청 한도를 넘으면 범위를 줄인 뒤 저장·조회하세요.")}</p>
    <div className="max-h-80 overflow-auto rounded border border-border p-2">
      {page.rows.map(value => <div key={value.id}><label><input type="checkbox" checked={selectedSet.has(value.id)} onChange={event => onChange(updateMetricSelection(selected, [value], event.target.checked))} /> {safeDisplayText(`${value.name} · ${value.unit} · ${value.kind}/${value.temporality}`)}</label><details><summary>{w("Resource / scope / attributes", "Resource / scope / 속성")}</summary><pre className="overflow-auto text-xs">{safeDisplayText(value.identity_json)}</pre></details></div>)}
      {!matches.length && <p>{w("No matching series", "일치하는 시계열 없음")}</p>}
    </div>
    <div className="flex items-center gap-2"><Button variant="outline" disabled={page.page === 0} onClick={() => setPage(page.page - 1)}>{w("Previous series page", "이전 시계열 페이지")}</Button><span>{page.page + 1} / {page.pages} · 50 {w("per page", "개씩")}</span><Button variant="outline" disabled={page.page + 1 === page.pages} onClick={() => setPage(page.page + 1)}>{w("Next series page", "다음 시계열 페이지")}</Button></div>
  </section>;
}
