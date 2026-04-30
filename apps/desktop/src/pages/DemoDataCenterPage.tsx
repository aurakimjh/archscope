import { useEffect, useMemo, useState } from "react";

import type {
  DemoRunResponse,
  DemoScenarioManifest,
} from "../api/analyzerClient";
import { useI18n } from "../i18n/I18nProvider";

type DemoDataCenterPageProps = {
  onOpenExportCenter: (inputPath: string) => void;
};

export function DemoDataCenterPage({
  onOpenExportCenter,
}: DemoDataCenterPageProps): JSX.Element {
  const { t } = useI18n();
  const [manifestRoot, setManifestRoot] = useState("");
  const [scenarios, setScenarios] = useState<DemoScenarioManifest[]>([]);
  const [selectedScenario, setSelectedScenario] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isRunning, setIsRunning] = useState(false);
  const [response, setResponse] = useState<DemoRunResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const selectedManifest = useMemo(
    () => scenarios.find((scenario) => scenario.scenario === selectedScenario),
    [scenarios, selectedScenario],
  );
  const canRun = Boolean(window.archscope?.demo && manifestRoot && selectedScenario && !isRunning);

  useEffect(() => {
    void loadScenarios();
  }, []);

  async function loadScenarios(root?: string): Promise<void> {
    if (!window.archscope?.demo) {
      return;
    }
    setIsLoading(true);
    setError(null);
    setResponse(null);
    try {
      const result = await window.archscope.demo.list(root);
      if (!result.ok) {
        setError(result.error.message);
        return;
      }
      setManifestRoot(result.manifestRoot);
      setScenarios(result.scenarios);
      setSelectedScenario(result.scenarios[0]?.scenario ?? "");
    } finally {
      setIsLoading(false);
    }
  }

  async function chooseRoot(): Promise<void> {
    const selected = await window.archscope?.selectDirectory?.({
      title: t("selectDemoManifestRoot"),
    });
    if (!selected || selected.canceled || !selected.directoryPath) {
      return;
    }
    await loadScenarios(selected.directoryPath);
  }

  async function runScenario(): Promise<void> {
    if (!window.archscope?.demo || !canRun) {
      return;
    }
    setIsRunning(true);
    setError(null);
    setResponse(null);
    try {
      const result = await window.archscope.demo.run({
        manifestRoot,
        scenario: selectedScenario,
      });
      setResponse(result);
      if (result.ok && result.exportInputPaths.length > 0) {
        onOpenExportCenter(result.exportInputPaths[0]);
      }
    } finally {
      setIsRunning(false);
    }
  }

  return (
    <div className="page">
      <div className="workspace-grid demo-data-grid">
        <section className="tool-panel">
          <h2>{t("demoDataCenter")}</h2>
          <div className="export-path-picker">
            <span>{t("demoManifestRoot")}</span>
            <div className="selected-file">{manifestRoot || t("noFileSelected")}</div>
            <button className="secondary-button" type="button" onClick={() => void chooseRoot()}>
              {t("browseFile")}
            </button>
          </div>
          <label className="field">
            {t("demoScenario")}
            <select
              value={selectedScenario}
              onChange={(event) => {
                setSelectedScenario(event.target.value);
                setResponse(null);
              }}
              disabled={scenarios.length === 0}
            >
              {scenarios.map((scenario) => (
                <option key={scenario.manifestPath} value={scenario.scenario}>
                  {scenario.dataSource}/{scenario.scenario}
                </option>
              ))}
            </select>
          </label>
          {selectedManifest ? (
            <div className="demo-scenario-summary">
              <strong>{selectedManifest.scenario}</strong>
              <small>{selectedManifest.description}</small>
              <span>{selectedManifest.analyzers.join(", ")}</span>
            </div>
          ) : null}
          <button
            className="primary-button"
            type="button"
            disabled={!canRun}
            onClick={() => void runScenario()}
          >
            {isRunning ? t("runningDemoData") : t("runDemoData")}
          </button>
        </section>

        <section className="table-panel">
          <div className="panel-header">
            <h2>{t("demoRunResult")}</h2>
            <button className="secondary-button" type="button" onClick={() => void loadScenarios(manifestRoot)}>
              {isLoading ? t("loadingDemoData") : t("refresh")}
            </button>
          </div>
          {!window.archscope?.demo ? (
            <div className="message-panel info-panel">
              <p>{t("demoBridgeNotConnected")}</p>
            </div>
          ) : error ? (
            <div className="message-panel error-panel" role="alert">
              <p>{error}</p>
            </div>
          ) : response?.ok ? (
            <div className="message-panel info-panel">
              <p>{t("demoRunCompleted")}</p>
              <ul className="output-path-list">
                {response.outputPaths.map((path) => (
                  <li key={path}>{path}</li>
                ))}
              </ul>
              {response.engine_messages && response.engine_messages.length > 0 ? (
                <pre>{response.engine_messages.join("\n")}</pre>
              ) : null}
            </div>
          ) : response ? (
            <div className="message-panel error-panel" role="alert">
              <p>{response.error.message}</p>
              <small>{response.error.code}</small>
              {response.error.detail ? <pre>{response.error.detail}</pre> : null}
            </div>
          ) : (
            <div className="message-panel info-panel">
              <p>{t("waitingForDemoData")}</p>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
