import { ChartPanel } from "../components/ChartPanel";
import { FileDropZone } from "../components/FileDropZone";

export function AccessLogAnalyzerPage(): JSX.Element {
  return (
    <div className="page">
      <section className="workspace-grid">
        <div className="tool-panel">
          <h2>Access Log Analyzer</h2>
          <FileDropZone
            label="Select access log file"
            description="NGINX-like sample format is supported by the current engine MVP."
          />
          <label className="field">
            <span>Log format</span>
            <select defaultValue="nginx">
              <option value="nginx">NGINX</option>
              <option value="apache">Apache</option>
              <option value="ohs">OHS</option>
              <option value="weblogic">WebLogic</option>
              <option value="tomcat">Tomcat</option>
              <option value="custom-regex">Custom Regex</option>
            </select>
          </label>
          <button className="primary-button" type="button">
            Analyze
          </button>
        </div>
        <div>
          <section className="summary-grid compact">
            <MetricCard label="Total Requests" value="-" />
            <MetricCard label="Average Response Time" value="-" />
            <MetricCard label="p95 Response Time" value="-" />
            <MetricCard label="Error Rate" value="-" />
          </section>
          <ChartPanel
            title="Access Log Chart Area"
            option={{
              xAxis: { type: "category", data: ["10:00", "10:01", "10:02"] },
              yAxis: { type: "value" },
              series: [{ type: "bar", data: [3, 2, 1] }],
            }}
          />
        </div>
      </section>
    </div>
  );
}

function MetricCard({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <div className="metric-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
