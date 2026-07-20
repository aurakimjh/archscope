import { Download, Trash2 } from "lucide-react";
import { useEffect, useState } from "react";

import { HelpTip, HelpedTitle } from "@/components/HelpTip";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { getHelpText } from "@/help/helpCatalog";
import { useI18n } from "@/i18n/I18nProvider";
import { useAnalysisWorkspace } from "@/state/analysisWorkspace";
import {
  clearEvidenceCards,
  exportEvidencePack,
  exportEvidenceReport,
  readEvidenceCards,
  removeEvidenceCard,
  type EvidenceCard,
} from "@/state/evidenceBoard";
import { exportReportPackHTML, exportReportPackZip } from "@/state/reportPack";

export function EvidenceBoardPage(): React.JSX.Element {
  const { locale, t } = useI18n();
  const workspace = useAnalysisWorkspace();
  const [cards, setCards] = useState<EvidenceCard[]>(() => readEvidenceCards());

  useEffect(() => {
    const refresh = () => setCards(readEvidenceCards());
    window.addEventListener("archscope:evidence-board-updated", refresh);
    return () => window.removeEventListener("archscope:evidence-board-updated", refresh);
  }, []);

  return (
    <main className="mx-auto max-w-6xl px-6 py-6">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">
            <HelpedTitle help={getHelpText(locale, "pageEvidenceBoard")}>
              {t("navEvidenceBoard")}
            </HelpedTitle>
          </h1>
          <p className="text-sm text-muted-foreground">
            {cards.length.toLocaleString()} {t("evidenceCards")}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={cards.length === 0}
            onClick={() => exportEvidenceReport(cards)}
          >
            <Download className="mr-2 h-4 w-4" />
            {t("evidenceExportReport")}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={cards.length === 0}
            onClick={() => exportEvidencePack(cards)}
          >
            <Download className="mr-2 h-4 w-4" />
            {t("evidenceExportPack")}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={cards.length === 0}
            onClick={() => exportReportPackHTML(cards, workspace.entries)}
          >
            <Download className="mr-2 h-4 w-4" />
            {t("reportPackExportHtml")}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={cards.length === 0}
            onClick={() => {
              void exportReportPackZip(cards, workspace.entries);
            }}
          >
            <Download className="mr-2 h-4 w-4" />
            {t("reportPackExportZip")}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={cards.length === 0}
            onClick={() => {
              clearEvidenceCards();
              setCards([]);
            }}
          >
            <Trash2 className="mr-2 h-4 w-4" />
            {t("evidenceClear")}
          </Button>
        </div>
      </div>

      {cards.length === 0 ? (
        <Card>
          <CardContent className="px-4 py-6 text-sm text-muted-foreground">
            {t("evidenceEmpty")}
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-3">
          {cards.map((card) => (
            <EvidenceCardItem
              key={card.id}
              card={card}
              onRemove={() => setCards(removeEvidenceCard(card.id))}
            />
          ))}
        </div>
      )}
    </main>
  );
}

function EvidenceCardItem({
  card,
  onRemove,
}: {
  card: EvidenceCard;
  onRemove: () => void;
}): React.JSX.Element {
  const { locale } = useI18n();
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle className="inline-flex items-center gap-2 text-sm">
              <span className="min-w-0 truncate">{card.title}</span>
              <HelpTip text={getHelpText(locale, "sectionEvidenceCard")} />
            </CardTitle>
            <div className="mt-1 flex flex-wrap gap-1.5 text-[10px] uppercase tracking-wide text-muted-foreground">
              <span className="rounded bg-muted px-1.5 py-0.5">{card.analyzer}</span>
              <span className="rounded bg-muted px-1.5 py-0.5">{card.source_kind}</span>
              {card.severity && (
                <span className="rounded bg-muted px-1.5 py-0.5">{card.severity}</span>
              )}
            </div>
          </div>
          <Button type="button" variant="ghost" size="sm" onClick={onRemove}>
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-2 pt-0 text-xs">
        {card.summary && <p className="leading-relaxed">{card.summary}</p>}
        <div className="grid gap-1 text-muted-foreground sm:grid-cols-2">
          <div>{card.source_file ?? "-"}</div>
          <div>{card.source_ref ?? card.created_at}</div>
        </div>
        <pre className="max-h-40 overflow-auto rounded bg-muted/50 p-2 font-mono text-[11px]">
          {JSON.stringify(card.payload, null, 2)}
        </pre>
      </CardContent>
    </Card>
  );
}
