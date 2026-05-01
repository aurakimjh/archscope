import { useEffect, useMemo, useState } from "react";

import type {
  DemoOutputArtifact,
  DemoRunResponse,
  DemoRunScenarioResult,
  DemoScenarioManifest,
} from "../api/analyzerClient";
import { useI18n } from "../i18n/I18nProvider";

type DemoDataCenterPageProps = {
  onOpenExportCenter: (inputPath: string) => void;
};

type SourceFilter = "all" | "synthetic" | "real";

const ALL_SCENARIOS = "__all__";

export function DemoDataCenterPage({
  onOpenExportCenter,
}: DemoDataCenterPageProps): JSX.Element {
  const { t } = useI18n();
  const [manifestRoot, setManifestRoot] = useState("");
  const [scenarios, setScenarios] = useState<DemoScenarioManifest[]>([]);
  const [sourceFilter, setSourceFilter] = useState<SourceFilter>("all");
  const [selectedScenario, setSelectedScenario] = useState(ALL_SCENARIOS);
  const [isLoading, setIsLoading] = useState(false);
  const [isRunning, setIsRunning] = useState(false);
  const [response, setResponse] = useState<DemoRunResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  const filteredScenarios = useMemo(
    () =>
      scenarios.filter(
        (scenario) => sourceFilter === "all" || scenario.dataSource === sourceFilter,
      ),
    [scenarios, sourceFilter],
  );
  const selectedManifest = useMemo(
    () => scenarios.find((scenario) => scenario.scenario === selectedScenario),
    [scenarios, selectedScenario],
  );
  const canRun = Boolean(
    window.archscope?.demo &&
      manifestRoot &&
      !isRunning &&
      (selectedScenario === ALL_SCENARIOS || selectedManifest),
  );

  useEffect(() => {
    void loadScenarios();
  }, []);

  useEffect(() => {
    if (
      selectedScenario !== ALL_SCENARIOS &&
      !filteredScenarios.some((scenario) => scenario.scenario === selectedScenario)
    ) {
      setSelectedScenario(ALL_SCENARIOS);
    }
  }, [filteredScenarios, selectedScenario]);

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
      setSelectedScenario(ALL_SCENARIOS);
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
        scenario: selectedScenario === ALL_SCENARIOS ? undefined : selectedScenario,
        dataSource: sourceFilter === "all" ? undefined : sourceFilter,
      });
      setResponse(result);
    } finally {
      setIsRunning(false);
    }
  }

  async function openPath(path: string): Promise<void> {
    const result = await window.archscope?.demo?.openPath(path);
    if (result && !result.ok) {
      setError(result.error.message);
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
            {t("demoDataSource")}
            <select
              value={sourceFilter}
              onChange={(event) => {
                setSourceFilter(event.target.value as SourceFilter);
                setResponse(null);
              }}
            >
              <option value="all">{t("allDemoData")}</option>
              <option value="synthetic">{t("syntheticDemoData")}</option>
              <option value="real">{t("realDemoData")}</option>
            </select>
          </label>
          <label className="field">
            {t("demoScenario")}
            <select
              value={selectedScenario}
              onChange={(event) => {
                setSelectedScenario(event.target.value);
                setResponse(null);
              }}
              disabled={filteredScenarios.length === 0}
            >
              <option value={ALL_SCENARIOS}>{t("allScenarios")}</option>
              {filteredScenarios.map((scenario) => (
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
          ) : (
            <div className="demo-scenario-summary">
              <strong>{t("allScenarios")}</strong>
              <small>{t("allScenariosDescription")}</small>
              <span>{filteredScenarios.length} scenarios</span>
            </div>
          )}
          <button
            className="primary-button"
            type="button"
            data-testid="demo-run-button"
            disabled={!canRun}
            onClick={() => void runScenario()}
          >
            {isRunning ? t("runningDemoData") : t("runDemoData")}
          </button>
        </section>

        <section className="table-panel">
          <div className="panel-header">
            <h2>{t("demoRunResult")}</h2>
            <button
              className="secondary-button"
              type="button"
              onClick={() => void loadScenarios(manifestRoot)}
            >
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
            <DemoRunResults
              response={response}
              onOpenPath={(path) => void openPath(path)}
              onOpenExportCenter={onOpenExportCenter}
            />
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

function DemoRunResults({
  response,
  onOpenPath,
  onOpenExportCenter,
}: {
  response: Extract<DemoRunResponse, { ok: true }>;
  onOpenPath: (path: string) => void;
  onOpenExportCenter: (path: string) => void;
}): JSX.Element {
  const { t } = useI18n();
  const totals = response.scenarios.reduce(
    (accumulator, scenario) => ({
      analyzerOutputs: accumulator.analyzerOutputs + scenario.summary.analyzerOutputs,
      failedAnalyzers: accumulator.failedAnalyzers + scenario.summary.failedAnalyzers,
      skippedLines: accumulator.skippedLines + scenario.summary.skippedLines,
      referenceFiles: accumulator.referenceFiles + scenario.summary.referenceFiles,
      findingCount: accumulator.findingCount + scenario.summary.findingCount,
    }),
    {
      analyzerOutputs: 0,
      failedAnalyzers: 0,
      skippedLines: 0,
      referenceFiles: 0,
      findingCount: 0,
    },
  );

  return (
    <div className="demo-result-stack">
      <div className="message-panel info-panel">
        <p>{t("demoRunCompleted")}</p>
        <div className="demo-summary-strip">
          <Metric label={t("analyzerOutputs")} value={totals.analyzerOutputs} />
          <Metric label={t("failedAnalyzers")} value={totals.failedAnalyzers} />
          <Metric label={t("skippedLines")} value={totals.skippedLines} />
          <Metric label={t("referenceFiles")} value={totals.referenceFiles} />
          <Metric label={t("findingCount")} value={totals.findingCount} />
        </div>
        {response.engine_messages && response.engine_messages.length > 0 ? (
          <pre>{response.engine_messages.join("\n")}</pre>
        ) : null}
      </div>

      {response.scenarios.map((scenario) => (
        <ScenarioResult
          key={`${scenario.dataSource}/${scenario.scenario}`}
          scenario={scenario}
          onOpenPath={onOpenPath}
          onOpenExportCenter={onOpenExportCenter}
        />
      ))}
    </div>
  );
}

function ScenarioResult({
  scenario,
  onOpenPath,
  onOpenExportCenter,
}: {
  scenario: DemoRunScenarioResult;
  onOpenPath: (path: string) => void;
  onOpenExportCenter: (path: string) => void;
}): JSX.Element {
  const { t } = useI18n();
  return (
    <article className="demo-scenario-result">
      <div className="demo-scenario-result-header">
        <div>
          <h3>
            {scenario.dataSource}/{scenario.scenario}
          </h3>
          <small>{scenario.bundleIndexPath}</small>
        </div>
        <button
          className="secondary-button"
          type="button"
          onClick={() => onOpenPath(scenario.bundleIndexPath)}
        >
          {t("openIndex")}
        </button>
      </div>
      <div className="demo-summary-strip">
        <Metric label={t("analyzerOutputs")} value={scenario.summary.analyzerOutputs} />
        <Metric label={t("failedAnalyzers")} value={scenario.summary.failedAnalyzers} />
        <Metric label={t("skippedLines")} value={scenario.summary.skippedLines} />
        <Metric label={t("referenceFiles")} value={scenario.summary.referenceFiles} />
        <Metric label={t("findingCount")} value={scenario.summary.findingCount} />
      </div>
      <ArtifactTable
        artifacts={scenario.artifacts}
        onOpenPath={onOpenPath}
        onOpenExportCenter={onOpenExportCenter}
      />
      {scenario.failedAnalyzers.length > 0 ? (
        <CompactList
          title={t("failedAnalyzers")}
          items={scenario.failedAnalyzers.map(
            (item) => `${item.file} (${item.analyzerType}) ${item.error ?? ""}`,
          )}
        />
      ) : null}
      {scenario.skippedLineReport.length > 0 ? (
        <CompactList
          title={t("skippedLineReport")}
          items={scenario.skippedLineReport.map(
            (item) => `${item.file} (${item.analyzerType}): ${item.skippedLines}`,
          )}
        />
      ) : null}
      {scenario.referenceFiles.length > 0 ? (
        <div className="demo-reference-panel">
          <h4>{t("correlationContext")}</h4>
          <ul>
            {scenario.referenceFiles.map((file) => (
              <li key={file.path}>
                <span>{file.file}</span>
                <small>{file.description}</small>
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => onOpenPath(file.path)}
                >
                  {t("openFile")}
                </button>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </article>
  );
}

function ArtifactTable({
  artifacts,
  onOpenPath,
  onOpenExportCenter,
}: {
  artifacts: DemoOutputArtifact[];
  onOpenPath: (path: string) => void;
  onOpenExportCenter: (path: string) => void;
}): JSX.Element {
  const { t } = useI18n();
  return (
    <table className="demo-artifact-table">
      <thead>
        <tr>
          <th>{t("artifactType")}</th>
          <th>{t("artifact")}</th>
          <th>{t("actions")}</th>
        </tr>
      </thead>
      <tbody>
        {artifacts.map((artifact) => (
          <tr key={`${artifact.kind}:${artifact.path}`}>
            <td>{artifact.kind}</td>
            <td>
              <span>{artifact.label}</span>
              <small>{artifact.path}</small>
            </td>
            <td>
              <div className="button-row">
                <button
                  className="secondary-button"
                  type="button"
                  onClick={() => onOpenPath(artifact.path)}
                >
                  {t("openFile")}
                </button>
                {artifact.exportable ? (
                  <button
                    className="secondary-button"
                    type="button"
                    data-testid="demo-send-export"
                    onClick={() => onOpenExportCenter(artifact.path)}
                  >
                    {t("sendToExportCenter")}
                  </button>
                ) : null}
              </div>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function Metric({ label, value }: { label: string; value: number }): JSX.Element {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function CompactList({ title, items }: { title: string; items: string[] }): JSX.Element {
  return (
    <div className="demo-compact-list">
      <h4>{title}</h4>
      <ul>
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </div>
  );
}
