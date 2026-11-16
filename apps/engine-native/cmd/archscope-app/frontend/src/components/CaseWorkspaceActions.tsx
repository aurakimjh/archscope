import { useState } from "react";
import { FolderClock, Save, FolderOpen } from "lucide-react";
import { listCases, getCase, cleanupEvidenceCache } from "@/bridge/engine";
import { reopenWorkspaceCase, saveWorkspaceCase, type AnalysisWorkspaceSnapshot, type CaseManifest } from "@/state/analysisWorkspace";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/I18nProvider";

type CaseSummary = { id: string; title: string; state: string; problem?: string };
// T-630 exposes saved snapshots through the existing Workspace. Generic case
// rendering, discovery and case-wide search remain their own backlog tasks.
export function CaseWorkspaceActions({ workspace }: { workspace: AnalysisWorkspaceSnapshot }): React.JSX.Element {
  const { w } = useI18n();
  const [cases, setCases] = useState<CaseSummary[]>([]);
  const [selected, setSelected] = useState<CaseManifest | null>(null);
  const [busy, setBusy] = useState(false); const [message, setMessage] = useState("");
  const active = workspace.entries.find(e => e.id === workspace.active_id);
  async function run(fn: () => Promise<void>): Promise<void> {
    setBusy(true); setMessage(""); try { await fn(); } catch (e) { setMessage(String(e)); } finally { setBusy(false); }
  }
  return <details className="relative">
    <summary className="cursor-pointer list-none rounded-md border border-border p-2" title={w("Saved cases", "저장된 케이스")} aria-label={w("Saved cases", "저장된 케이스")}><FolderClock size={18} /></summary>
    <div className="absolute right-0 z-40 mt-1 flex max-h-96 w-80 max-w-[calc(100vw-2rem)] flex-col gap-2 overflow-auto rounded-md border border-border bg-popover p-3 text-popover-foreground shadow-xl">
      <Button disabled={busy || !active?.backend_result_handle} title={w("Save a backend result without numeric conversion", "백엔드 분석 결과를 정밀도 손실 없이 저장")} onClick={() => void run(async () => { if (!active) return; await saveWorkspaceCase(active.id); setMessage(w("Saved to case.", "케이스에 저장했습니다.")); })}><Save size={16} />{w("Save selected result", "선택한 결과 저장")}</Button>
      <Button variant="outline" disabled={busy} onClick={() => void run(async () => { setCases(JSON.parse(await listCases())); setSelected(null); })}><FolderOpen size={16} />{w("Open case", "케이스 열기")}</Button>
      {cases.map(c => <Button key={c.id} variant="ghost" disabled={busy || !!c.problem || c.state === "deleted"} title={c.problem || c.id} onClick={() => void run(async () => setSelected(JSON.parse(await getCase(c.id))))}>{c.title || c.id.slice(-8)} ({c.state})</Button>)}
      {selected?.results.map(r => <Button key={r.id} variant="outline" disabled={busy} onClick={() => void run(async () => { await reopenWorkspaceCase(selected.id, r.id); setMessage(w("Opened saved snapshot.", "저장된 스냅샷을 열었습니다.")); })}>{r.type} · {r.id.slice(-8)}</Button>)}
      <details><summary>{w("Result cache", "결과 캐시 관리")}</summary>
        <p className="text-xs">{w("Save or export needed results first. Cleanup removes cached results with no screen owner; saved cases and originals remain.", "필요한 결과는 먼저 저장하거나 내보내세요. 정리하면 어느 화면도 소유하지 않은 캐시 결과만 제거합니다. 저장된 케이스와 원본은 유지됩니다.")}</p>
        <Button variant="outline" disabled={busy} onClick={() => void run(async () => { const report = await cleanupEvidenceCache(); setMessage(w("Removed {removed}; retained {retained} owned results", "{removed}개 정리, 사용 중 {retained}개 보존").replace("{removed}", String(report.removed)).replace("{retained}", String(report.retained_owned))); })}>{w("Clean unowned result cache", "미사용 결과 캐시 정리")}</Button>
      </details>
      {message && <p role="status" className="break-words text-xs">{message}</p>}
    </div>
  </details>;
}
