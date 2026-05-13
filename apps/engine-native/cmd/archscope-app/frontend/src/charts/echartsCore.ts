import * as echarts from "echarts/core";
import {
  BarChart,
  CustomChart,
  GraphChart,
  LineChart,
  TreemapChart,
} from "echarts/charts";
import {
  DataZoomComponent,
  DataZoomInsideComponent,
  DataZoomSliderComponent,
  GridComponent,
  LegendComponent,
  MarkLineComponent,
  MarkPointComponent,
  TooltipComponent,
} from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";

echarts.use([
  BarChart,
  CustomChart,
  GraphChart,
  LineChart,
  TreemapChart,
  DataZoomComponent,
  DataZoomInsideComponent,
  DataZoomSliderComponent,
  GridComponent,
  LegendComponent,
  MarkLineComponent,
  MarkPointComponent,
  TooltipComponent,
  CanvasRenderer,
]);

export { echarts };
export type { ECharts, EChartsCoreOption as EChartsOption } from "echarts/core";
