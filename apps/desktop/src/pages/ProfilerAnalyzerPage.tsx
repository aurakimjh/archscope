import { FileDropZone } from "../components/FileDropZone";
import { useI18n } from "../i18n/I18nProvider";

export function ProfilerAnalyzerPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <div className="page">
      <section className="workspace-grid">
        <div className="tool-panel">
          <h2>{t("profilerAnalyzer")}</h2>
          <FileDropZone label={t("selectCpuCollapsedFile")} />
          <FileDropZone label={t("selectWallCollapsedFile")} />
          <div className="input-grid">
            <label className="field">
              <span>{t("cpuIntervalMs")}</span>
              <input type="number" defaultValue={10} min={1} />
            </label>
            <label className="field">
              <span>{t("wallIntervalMs")}</span>
              <input type="number" defaultValue={100} min={1} />
            </label>
            <label className="field">
              <span>{t("elapsedSeconds")}</span>
              <input type="number" placeholder="1336.559" min={0} step="0.001" />
            </label>
          </div>
          <button className="primary-button" type="button">
            {t("analyze")}
          </button>
        </div>
        <div>
          <section className="summary-grid compact">
            <MetricCard label={t("totalCpuSamples")} value="-" />
            <MetricCard label={t("totalWallSamples")} value="-" />
            <MetricCard label={t("estimatedCpuTime")} value="-" />
            <MetricCard label={t("estimatedWallTime")} value="-" />
          </section>
          <section className="table-panel">
            <div className="panel-header">
              <h2>{t("topStackTable")}</h2>
            </div>
            <table>
              <thead>
                <tr>
                  <th>{t("stack")}</th>
                  <th>{t("samples")}</th>
                  <th>{t("estimatedSeconds")}</th>
                  <th>{t("ratio")}</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>{t("waitingProfilerResult")}</td>
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
