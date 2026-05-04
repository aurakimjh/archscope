import {
  ExternalLink,
  FileText,
  Folder,
  Loader2,
  Play,
  RefreshCw,
  Send,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import type {
  DemoOutputArtifact,
  DemoRunResponse,
  DemoRunScenarioResult,
  DemoScenarioManifest,
} from "@/api/analyzerClient";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/i18n/I18nProvider";

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
  const demoBridgeAvailable = Boolean(window.archscope?.demo);
  const canRun = Boolean(
    demoBridgeAvailable &&
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
    if (!window.archscope?.demo) return;
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
    if (!selected || selected.canceled || !selected.directoryPath) return;
    await loadScenarios(selected.directoryPath);
  }

  async function runScenario(): Promise<void> {
    if (!window.archscope?.demo || !canRun) return;
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
    <div className="grid gap-5 lg:grid-cols-[2fr_3fr]">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t("demoDataCenter")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("demoManifestRoot")}</span>
            <div className="flex items-center gap-2">
              <div className="flex h-9 flex-1 items-center truncate rounded-md border border-input bg-muted/20 px-3 font-mono text-xs text-muted-foreground">
                {manifestRoot || t("noFileSelected")}
              </div>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={() => void chooseRoot()}
              >
                <Folder className="h-3.5 w-3.5" />
                {t("browseFile")}
              </Button>
            </div>
          </div>

          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("demoDataSource")}</span>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
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

          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("demoScenario")}</span>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm disabled:opacity-50"
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

          <div className="rounded-md border border-border bg-muted/30 px-3 py-2 text-xs">
            {selectedManifest ? (
              <>
                <p className="font-semibold">{selectedManifest.scenario}</p>
                <p className="mt-0.5 text-muted-foreground">
                  {selectedManifest.description || "—"}
                </p>
                <p className="mt-1 font-mono text-[11px] text-muted-foreground">
                  {selectedManifest.analyzers.join(", ")}
                </p>
              </>
            ) : (
              <>
                <p className="font-semibold">{t("allScenarios")}</p>
                <p className="mt-0.5 text-muted-foreground">
                  {t("allScenariosDescription")}
                </p>
                <p className="mt-1 text-muted-foreground">
                  {filteredScenarios.length} scenarios
                </p>
              </>
            )}
          </div>

          <Button
            type="button"
            data-testid="demo-run-button"
            disabled={!canRun}
            onClick={() => void runScenario()}
            className="w-full"
          >
            {isRunning ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                {t("runningDemoData")}
              </>
            ) : (
              <>
                <Play className="h-3.5 w-3.5" />
                {t("runDemoData")}
              </>
            )}
          </Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
          <CardTitle className="text-sm">{t("demoRunResult")}</CardTitle>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void loadScenarios(manifestRoot)}
          >
            {isLoading ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <RefreshCw className="h-3.5 w-3.5" />
            )}
            {isLoading ? t("loadingDemoData") : t("refresh")}
          </Button>
        </CardHeader>
        <CardContent>
          {!demoBridgeAvailable ? (
            <p className="rounded-md border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
              {t("demoBridgeNotConnected")}
            </p>
          ) : error ? (
            <p
              className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
              role="alert"
            >
              {error}
            </p>
          ) : response?.ok ? (
            <DemoRunResults
              response={response}
              onOpenPath={(path) => void openPath(path)}
              onOpenExportCenter={onOpenExportCenter}
            />
          ) : response ? (
            <div
              className="rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
              role="alert"
            >
              <p>{response.error.message}</p>
              <small className="mt-1 block text-xs opacity-80">
                {response.error.code}
              </small>
              {response.error.detail && (
                <pre className="mt-2 max-h-48 overflow-auto rounded bg-destructive/5 p-2 text-[11px]">
                  {response.error.detail}
                </pre>
              )}
            </div>
          ) : (
            <p className="rounded-md border border-border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
              {t("waitingForDemoData")}
            </p>
          )}
        </CardContent>
      </Card>
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
    { analyzerOutputs: 0, failedAnalyzers: 0, skippedLines: 0, referenceFiles: 0, findingCount: 0 },
  );

  return (
    <div className="space-y-4">
      <div className="rounded-md border border-emerald-500/30 bg-emerald-500/5 p-3">
        <p className="text-sm font-medium text-emerald-700 dark:text-emerald-400">
          {t("demoRunCompleted")}
        </p>
        <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-5">
          <Metric label={t("analyzerOutputs")} value={totals.analyzerOutputs} />
          <Metric label={t("failedAnalyzers")} value={totals.failedAnalyzers} />
          <Metric label={t("skippedLines")} value={totals.skippedLines} />
          <Metric label={t("referenceFiles")} value={totals.referenceFiles} />
          <Metric label={t("findingCount")} value={totals.findingCount} />
        </div>
        {response.engine_messages && response.engine_messages.length > 0 && (
          <pre className="mt-3 max-h-32 overflow-auto rounded bg-background/40 p-2 text-[11px] leading-relaxed">
            {response.engine_messages.join("\n")}
          </pre>
        )}
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
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-3">
        <div className="min-w-0">
          <CardTitle className="text-sm">
            {scenario.dataSource}/{scenario.scenario}
          </CardTitle>
          <p className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
            {scenario.bundleIndexPath}
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => onOpenPath(scenario.bundleIndexPath)}
        >
          <ExternalLink className="h-3.5 w-3.5" />
          {t("openIndex")}
        </Button>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
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

        {scenario.failedAnalyzers.length > 0 && (
          <CompactList
            title={t("failedAnalyzers")}
            items={scenario.failedAnalyzers.map(
              (item) => `${item.file} (${item.analyzerType}) ${item.error ?? ""}`,
            )}
          />
        )}
        {scenario.skippedLineReport.length > 0 && (
          <CompactList
            title={t("skippedLineReport")}
            items={scenario.skippedLineReport.map(
              (item) => `${item.file} (${item.analyzerType}): ${item.skippedLines}`,
            )}
          />
        )}
        {scenario.referenceFiles.length > 0 && (
          <div className="space-y-2">
            <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              {t("correlationContext")}
            </h4>
            <ul className="space-y-1.5">
              {scenario.referenceFiles.map((file) => (
                <li
                  key={file.path}
                  className="flex items-center justify-between gap-2 rounded-md border border-border bg-muted/20 px-3 py-2 text-xs"
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-mono">{file.file}</p>
                    {file.description && (
                      <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
                        {file.description}
                      </p>
                    )}
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => onOpenPath(file.path)}
                  >
                    <ExternalLink className="h-3 w-3" />
                    {t("openFile")}
                  </Button>
                </li>
              ))}
            </ul>
          </div>
        )}
      </CardContent>
    </Card>
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
    <div className="overflow-x-auto rounded-md border border-border">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
            <th className="px-3 py-2 text-left font-medium">{t("artifactType")}</th>
            <th className="px-3 py-2 text-left font-medium">{t("artifact")}</th>
            <th className="px-3 py-2 text-right font-medium">{t("actions")}</th>
          </tr>
        </thead>
        <tbody>
          {artifacts.map((artifact) => (
            <tr
              key={`${artifact.kind}:${artifact.path}`}
              className="border-b border-border last:border-0"
            >
              <td className="px-3 py-2">
                <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase">
                  {artifact.kind}
                </span>
              </td>
              <td className="px-3 py-2">
                <p className="text-xs">{artifact.label}</p>
                <p className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                  {artifact.path}
                </p>
              </td>
              <td className="px-3 py-2">
                <div className="flex items-center justify-end gap-1.5">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => onOpenPath(artifact.path)}
                  >
                    <FileText className="h-3 w-3" />
                    {t("openFile")}
                  </Button>
                  {artifact.exportable && (
                    <Button
                      type="button"
                      size="sm"
                      data-testid="demo-send-export"
                      onClick={() => onOpenExportCenter(artifact.path)}
                    >
                      <Send className="h-3 w-3" />
                      {t("sendToExportCenter")}
                    </Button>
                  )}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: number }): JSX.Element {
  return (
    <div className="rounded-md border border-border bg-background/50 px-2 py-1.5">
      <p className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
        {label}
      </p>
      <p className="mt-0.5 text-sm font-semibold tabular-nums">{value}</p>
    </div>
  );
}

function CompactList({ title, items }: { title: string; items: string[] }): JSX.Element {
  return (
    <div className="space-y-1.5">
      <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
        {title}
      </h4>
      <ul className="space-y-1">
        {items.map((item) => (
          <li
            key={item}
            className="rounded-md border border-border bg-muted/20 px-3 py-1.5 font-mono text-[11px]"
          >
            {item}
          </li>
        ))}
      </ul>
    </div>
  );
}
