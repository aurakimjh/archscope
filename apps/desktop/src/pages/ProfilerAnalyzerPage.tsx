import { FileDropZone } from "../components/FileDropZone";

export function ProfilerAnalyzerPage(): JSX.Element {
  return (
    <div className="page">
      <section className="workspace-grid">
        <div className="tool-panel">
          <h2>Profiler Analyzer</h2>
          <FileDropZone label="Select CPU collapsed file" />
          <FileDropZone label="Select wall collapsed file" />
          <div className="input-grid">
            <label className="field">
              <span>CPU interval ms</span>
              <input type="number" defaultValue={10} min={1} />
            </label>
            <label className="field">
              <span>Wall interval ms</span>
              <input type="number" defaultValue={100} min={1} />
            </label>
            <label className="field">
              <span>Elapsed seconds</span>
              <input type="number" placeholder="1336.559" min={0} step="0.001" />
            </label>
          </div>
          <button className="primary-button" type="button">
            Analyze
          </button>
        </div>
        <div>
          <section className="summary-grid compact">
            <MetricCard label="Total CPU Samples" value="-" />
            <MetricCard label="Total Wall Samples" value="-" />
            <MetricCard label="Estimated CPU Time" value="-" />
            <MetricCard label="Estimated Wall Time" value="-" />
          </section>
          <section className="table-panel">
            <div className="panel-header">
              <h2>Top Stack Table</h2>
            </div>
            <table>
              <thead>
                <tr>
                  <th>Stack</th>
                  <th>Samples</th>
                  <th>Estimated Seconds</th>
                  <th>Ratio</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>Waiting for profiler analysis result</td>
                  <td>-</td>
                  <td>-</td>
                  <td>-</td>
                </tr>
              </tbody>
            </table>
          </section>
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
