// ─────────────────────────────────────────────────────────────────────
// [한글] DashboardPage.tsx — 진입 직후 사용자가 처음 보는 종합 대시보드.
//
// 책임/목적:
//   - 가장 최근 분석 결과(DashboardSampleResult) 를 localStorage 에서
//     복구해 KPI 카드 + 차트 그리드로 한 화면에 모아 보여줍니다.
//   - 차트 그리드는 chartTemplates 의 dashboardChartTemplateIds 순서를
//     따르며, createChartOption(template, data, labels) 로 ECharts 옵션을
//     생성합니다(템플릿 정의는 charts/chartTemplates.ts).
//   - "모든 차트 PNG 일괄 저장" 기능: container DOM 안의 ECharts 인스턴스를
//     순회해 batchExport(lib/batchExport.ts)가 PNG 로 일괄 export.
//
// 데이터 흐름:
//   localStorage("archscope.dashboard.lastResult")
//   → data state (DashboardSampleResult)
//   → useMemo(chartOptions) → ChartPanel/MetricCard 들.
//
// 사용 series/tables/charts 키:
//   - dashboardChartTemplateIds 가 다루는 template.id 들. 각 템플릿이
//     실제로 어떤 series/tables 키를 읽는지는 chartTemplates 정의 참조.
//
// 의존성:
//   - @/charts/chartFactory: ECharts option 생성.
//   - @/lib/batchExport: 컨테이너 내 차트 일괄 PNG.
//   - shadcn Card/Button + lucide 아이콘 + i18n(useI18n).
// ─────────────────────────────────────────────────────────────────────
import { Camera, LayoutDashboard, Loader2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import type { DashboardSampleResult } from "@/api/analyzerClient";
import { ChartPanel } from "@/components/ChartPanel";
import { MetricCard } from "@/components/MetricCard";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useI18n } from "@/i18n/I18nProvider";
import { exportChartsInContainer } from "@/lib/batchExport";
import { createChartOption } from "@/charts/chartFactory";
import {
  dashboardChartTemplateIds,
  getChartTemplate,
  type ChartTemplateId,
} from "@/charts/chartTemplates";

// [한글] DASHBOARD_STORAGE_KEY — 마지막 분석 결과를 캐싱하는
//   localStorage 키. 다른 페이지(예: 액세스 로그 분석)가 분석 직후
//   이 키에 결과를 저장해 두면, 사용자가 대시보드로 돌아왔을 때 자동
//   복구되어 "방금 본 결과" 를 다시 보여 줍니다.
const DASHBOARD_STORAGE_KEY = "archscope.dashboard.lastResult";

// [한글] DashboardPage — 종합 대시보드 페이지 컴포넌트.
//   state: data(분석 결과 또는 null), savingAll(일괄 저장 진행 여부).
//   ref:   chartsContainerRef(차트 컨테이너 DOM, 일괄 export 시 순회용).
//   effect: 마운트 시 localStorage 에서 결과를 1회 복구.
export function DashboardPage(): JSX.Element {
  const [data, setData] = useState<DashboardSampleResult | null>(null);
  const [savingAll, setSavingAll] = useState(false);
  const chartsContainerRef = useRef<HTMLDivElement | null>(null);
  const { t } = useI18n();

  useEffect(() => {
    const saved = window.localStorage.getItem(DASHBOARD_STORAGE_KEY);
    if (!saved) return;
    try {
      setData(JSON.parse(saved) as DashboardSampleResult);
    } catch {
      // ignore corrupt storage
    }
  }, []);

  const chartOptions = useMemo(() => {
    if (!data) return null;
    const labels = {
      requestsAxis: t("requestsAxis"),
      millisecondsAxis: t("millisecondsAxis"),
      statusSeries: t("statusSeries"),
      samplesAxis: t("samplesAxis"),
      p95Series: t("p95Series"),
    };
    const dataType = (data as { type?: string }).type;
    const dashboardSupportedTypes = new Set<string>(
      dashboardChartTemplateIds.map((id) => getChartTemplate(id).resultType),
    );
    if (dataType && !dashboardSupportedTypes.has(dataType)) {
      return null;
    }
    const built: { id: ChartTemplateId; title: string; option: ReturnType<typeof createChartOption> }[] = [];
    for (const templateId of dashboardChartTemplateIds) {
      const template = getChartTemplate(templateId);
      try {
        built.push({
          id: template.id,
          title: t(template.titleKey),
          option: createChartOption(template.id, data, labels),
        });
      } catch (err) {
        console.warn(`[dashboard] skipping chart ${templateId}:`, err);
      }
    }
    return built.length > 0 ? built : null;
  }, [data, t]);

  const handleSaveAllCharts = useCallback(async () => {
    const container = chartsContainerRef.current;
    if (!container || savingAll) return;
    setSavingAll(true);
    try {
      await exportChartsInContainer(container, {
        format: "png",
        multiplier: 2,
        prefix: "dashboard",
      });
    } finally {
      setSavingAll(false);
    }
  }, [savingAll]);

  if (!data || !chartOptions) {
    return (
      <Card className="border-dashed">
        <CardContent className="flex flex-col items-center justify-center gap-3 py-16 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted text-muted-foreground">
            <LayoutDashboard className="h-6 w-6" />
          </div>
          <p className="max-w-md text-sm text-muted-foreground">
            {t("dashboardEmpty")}
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">{t("dashboard")}</h1>
          <p className="text-xs text-muted-foreground">
            {t("brandSubtitle")} · {t("appTagline")}
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={savingAll}
          onClick={() => void handleSaveAllCharts()}
        >
          {savingAll ? (
            <>
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              {t("saveAllChartsBusy")}
            </>
          ) : (
            <>
              <Camera className="h-3.5 w-3.5" />
              {t("saveAllCharts")}
            </>
          )}
        </Button>
      </div>

      <section className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <MetricCard
          label={t("totalRequests")}
          value={String(data.summary.total_requests ?? "—")}
        />
        <MetricCard
          label={t("averageResponseTime")}
          value={`${data.summary.avg_response_ms ?? "—"} ms`}
        />
        <MetricCard
          label={t("p95ResponseTime")}
          value={`${data.summary.p95_response_ms ?? "—"} ms`}
        />
        <MetricCard
          label={t("errorRate")}
          value={`${data.summary.error_rate ?? "—"}%`}
        />
      </section>

      <section
        ref={chartsContainerRef}
        className="grid grid-cols-1 gap-4 lg:grid-cols-2"
      >
        {chartOptions.map((chart) => (
          <ChartPanel key={chart.id} title={chart.title} option={chart.option} />
        ))}
      </section>
    </div>
  );
}

export function saveDashboardResult(result: DashboardSampleResult): void {
  try {
    window.localStorage.setItem(DASHBOARD_STORAGE_KEY, JSON.stringify(result));
  } catch {
    // storage full, ignore
  }
}
