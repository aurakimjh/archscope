import { useEffect, useState } from "react";
import { Dialogs } from "@wailsio/runtime";
import * as bridge from "@/bridge/engine";
import { useI18n } from "@/i18n/I18nProvider";

export function PackSettings(): React.JSX.Element {
  const { locale } = useI18n(); const text = (en: string, ko: string): string => locale === "ko" ? ko : en;
  const [state, setState] = useState<bridge.PackState | null>(null);
  const [problem, setProblem] = useState(""); const [busy, setBusy] = useState(false);
  const [scope, setScope] = useState("application"); const [unsigned, setUnsigned] = useState(false);
  const [choices, setChoices] = useState("{}"); const [key, setKey] = useState('{"id":"owner-key","public_key":"","manage_trust":false,"revoked":false}');
  const [offline, setOffline] = useState(false); const [attempts, setAttempts] = useState<Record<string, number>>({});
  const [font, setFont] = useState("loading");
  async function refresh(): Promise<void> { setState(await bridge.getPackState()); const status = await bridge.getOfflineStatus(); setOffline(status.offline); setAttempts(status.denied_attempts); }
  useEffect(() => { let alive = true; void Promise.all([bridge.getPackState(), bridge.getOfflineStatus()]).then(([s, o]) => { if (alive) { setState(s); setOffline(o.offline); setAttempts(o.denied_attempts); } }).catch(e => { if (alive) setProblem(String(e)); }); void document.fonts.load('14px "Noto Sans KR"', "한글").then(faces => { if (alive) setFont(faces.length > 0 ? "loaded" : "fallback"); }).catch(() => { if (alive) setFont("fallback"); }); return () => { alive = false; }; }, []);
  async function run(action: () => Promise<unknown>): Promise<void> { if (busy) return; setBusy(true); setProblem(""); try { await action(); await refresh(); } catch (e) { setProblem(String(e)); } finally { setBusy(false); } }
  async function localFile(title: string): Promise<string> { return await Dialogs.OpenFile({ Title: title }); }
  return <section className="card" aria-busy={busy}>
    <h2>{text("Data packs", "데이터 팩")}</h2>
    <p>{text("Install trusted data mappings, then activate a version. Running jobs keep their original version.", "신뢰된 데이터 매핑을 설치한 후 버전을 활성화합니다. 실행 중인 작업은 기존 버전을 유지합니다.")}</p>
    <p>{offline ? text("Offline session", "오프라인 세션") : text("Standard session", "일반 세션")} · {text("Korean font", "한글 글꼴")}: {font === "loaded" ? text("Loaded locally", "로컬 로드됨") : font === "loading" ? text("Loading", "로드 중") : text("System fallback — layout unverified", "시스템 대체 — 레이아웃 미검증")}</p>
    {offline && <p>{text("Denied app operations", "거부된 앱 작업")}: {Object.values(attempts).reduce((a,b) => a+b,0)} · {text("OS/browser observation is separate.", "OS/브라우저 관측은 별도입니다.")}</p>}
    <div className="settings-row"><label>{text("Install scope", "설치 범위")} <input value={scope} onChange={e => setScope(e.target.value)} disabled={busy} placeholder="application / user / case:case-…" /></label>
      <label><input type="checkbox" checked={unsigned} onChange={e => { setUnsigned(e.target.checked); if (e.target.checked && scope === "application") setScope("user"); }} disabled={busy} />{text("Explicitly allow unsigned local preset", "미서명 로컬 프리셋 명시적 허용")}</label>
      <button disabled={busy || (unsigned && scope === "application")} onClick={() => void run(async () => { const path = await localFile(text("Choose a data pack ZIP", "데이터 팩 ZIP 선택")); if (path) await bridge.installDataPack(path, scope, unsigned); })}>{text("Install ZIP", "ZIP 설치")}</button>
      <button disabled={busy} onClick={() => void run(refresh)}>{text("Refresh", "새로고침")}</button>
    </div>
    <details><summary>{text("Resolve alias conflicts", "별칭 충돌 해결")}</summary><p>{text("Choose a canonical mapping for each conflicting alias. No local preset wins automatically.", "충돌한 별칭마다 canonical 매핑을 선택합니다. 로컬 프리셋을 자동 우선하지 않습니다.")}</p><textarea aria-label={text("Alias choices JSON", "별칭 선택 JSON")} value={choices} onChange={e => setChoices(e.target.value)} disabled={busy} /></details>
    {state && <><p>{text("State revision", "상태 리비전")}: {state.revision} · {text("Trust sequence", "신뢰 순서")}: {state.trust_sequence}</p>
      <ul>{Object.values(state.installed).map(ref => { const active = state.active[`${ref.scope}|${ref.id}`] === ref.sha256; return <li key={`${ref.scope}|${ref.sha256}`} className="my-3 break-all"><strong>{ref.id} {ref.version}</strong> · {ref.scope} · {ref.origin} · {active ? text("Active", "활성") : text("Retained", "보관")}<br /><small>SHA-256 {ref.sha256}</small><br />
        <button disabled={busy || active} onClick={() => void run(() => bridge.activateDataPack(state.revision, ref, choices, false))}>{text("Activate", "활성화")}</button>{" "}
        <button disabled={busy || active} onClick={() => void run(() => bridge.activateDataPack(state.revision, ref, choices, true))}>{text("Roll back to this version", "이 버전으로 롤백")}</button>{" "}
        <button disabled={busy || !active} onClick={() => void run(() => bridge.deactivateDataPack(state.revision, ref))}>{text("Deactivate", "비활성화")}</button>{" "}
        <button disabled={busy} onClick={() => void run(async () => { const path = await Dialogs.SaveFile({ Title: text("Export pack ZIP", "팩 ZIP 내보내기"), Filename: `${ref.id}-${ref.version}.zip` }); if (path) await bridge.exportDataPack(ref, path); })}>{text("Export ZIP", "ZIP 내보내기")}</button></li>; })}</ul>
      <details><summary>{text("Public trust keys", "공개 신뢰키")}</summary><ul>{Object.values(state.keys).map(k => <li key={k.id}>{k.id} · {k.revoked ? text("Revoked", "폐기됨") : text("Trusted", "신뢰됨")} · {k.manage_trust ? text("Trust manager", "신뢰 관리") : text("Pack signer", "팩 서명")}</li>)}</ul>
        <p>{text("Register only an owner-verified public key. Never enter a private key.", "소유자가 검증한 공개키만 등록하세요. 개인키를 입력하지 마세요.")}</p><textarea aria-label={text("Public key JSON", "공개키 JSON")} value={key} onChange={e => setKey(e.target.value)} disabled={busy} />
        <button disabled={busy} onClick={() => void run(() => bridge.registerPackTrustKey(state.revision, key))}>{text("Register public key", "공개키 등록")}</button>{" "}
        <button disabled={busy} onClick={() => void run(async () => { const body = await localFile(text("Signed trust update JSON", "서명된 신뢰 갱신 JSON")); if (!body) return; const signature = await localFile(text("Detached signature JSON", "분리 서명 JSON")); if (signature) await bridge.applyOfflinePackTrust(state.revision, body, signature); })}>{text("Apply signed rotation / revocation", "서명된 교체 / 폐기 적용")}</button>
      </details></>}
    {problem && <p role="alert">{problem}</p>}
    <p><a href="/offline-assets/docs/en/OFFLINE_PACKS.md" download>English manual</a> · <a href="/offline-assets/docs/ko/OFFLINE_PACKS.md" download>한국어 매뉴얼</a> · <a href="/offline-assets/fonts/OFL.txt" download>OFL-1.1</a></p>
  </section>;
}
