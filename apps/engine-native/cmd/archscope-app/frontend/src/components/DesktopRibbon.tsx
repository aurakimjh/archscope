import { useEffect, useRef, useState, type ComponentType, type ReactNode } from "react";
import {
  Activity, ArrowLeft, BarChart3, BookOpen, ChartNoAxesCombined, ChevronDown, ChevronUp,
  CircleHelp, Clock3, Cpu, Download, Eraser, EyeOff, FileSearch, FolderOpen, Gauge,
  GitCompareArrows, Globe, History, Info, Layers, Network, PanelTop, PanelTopClose,
  PanelsTopLeft, Pause, Pin, Play, Route, Save, Search, Settings, Square, TriangleAlert, ClipboardList,
} from "lucide-react";
import { useI18n } from "@/i18n/I18nProvider";
import { localeLabels, locales } from "@/i18n/locales";
import type { Theme } from "@/theme/ThemeProvider";
import type { NavKey } from "@/state/desktopNavigation";
import type { HttpCommand } from "@/state/httpCommands";
import { RIBBON_TABS, type RibbonMode, type RibbonTab } from "@/state/ribbon";
import "@/styles/ribbon.css";

type Icon = ComponentType<{ size?: number; "aria-hidden"?: boolean }>;
export type RibbonRoute = { key: NavKey; label: string; group: string };
type Props = {
  active: NavKey; routes: readonly RibbonRoute[]; mode: RibbonMode; onMode: (mode: RibbonMode) => void;
  onToggle: () => void; onCommand: (command: string) => void; canOpen: boolean; canRecent: boolean;
  captureCommands: readonly HttpCommand[]; theme: Theme;
  fileOpen: boolean; onFileOpen: (open: boolean) => void;
};
const routeIcons: Record<NavKey, Icon> = {
  profiler: Activity, diff: GitCompareArrows, access_log: FileSearch, gc_log: ChartNoAxesCombined,
  jfr: Cpu, exception: TriangleAlert, thread_dump: Layers, msa_profile: Network, trace_import: Route,
  http_capture: Network, browser_cpu: Cpu, browser_audit: Globe, analysis_workspace: PanelsTopLeft,
  export_center: Download, report_diff: GitCompareArrows, chart_studio: BarChart3, incident_timeline: Clock3,
  slo_golden_signals: Gauge, service_flow: Network, evidence_board: ClipboardList, settings: Settings,
};
const captureIcons: Record<HttpCommand, Icon> = {
  start: Play, stop: Square, pause: Pause, clear: Eraser, follow: ChevronDown, find: Search,
  save: Save, collapse: PanelTopClose, expand: PanelTop, help: CircleHelp,
};

export function DesktopRibbon(props: Props): React.JSX.Element {
  const { w, locale } = useI18n();
  const [tab, setTab] = useState<RibbonTab>("home");
  const [temporary, setTemporary] = useState(false);
  const [compact, setCompact] = useState(false);
  const { fileOpen, onFileOpen: setFileOpen } = props;
  const root = useRef<HTMLDivElement>(null);
  const fileButton = useRef<HTMLButtonElement>(null);
  const backButton = useRef<HTMLButtonElement>(null);
  const moreButton = useRef<HTMLSelectElement>(null);
  const modeMenu = useRef<HTMLDetailsElement>(null);
  const visible = props.mode !== "hidden";
  const commandsVisible = visible && (props.mode === "expanded" || temporary) && !fileOpen;
  const tabLabels: Record<RibbonTab, string> = {
    home: w("Home", "홈"), analyze: w("Analyze", "분석"), http: "HTTP",
    workspace: w("Workspace", "워크스페이스"), view: w("View", "보기"), help: w("Help", "도움말"),
  };
  const captureLabels: Record<HttpCommand, string> = {
    start: w("Start", "시작"), stop: w("Stop", "중지"), pause: w("Pause / Resume", "일시정지 / 재개"),
    clear: w("Clear", "지우기"), follow: w("Auto Scroll", "자동 스크롤"), find: w("Find", "찾기"),
    save: w("Save / Export", "저장 / 내보내기"), collapse: w("Collapse Groups", "그룹 접기"),
    expand: w("Expand Groups", "그룹 펼치기"), help: w("Current tool help", "현재 도구 도움말"),
  };
  function closeFile(): void { setFileOpen(false); requestAnimationFrame(() => fileButton.current?.focus()); }
  function setMode(next: RibbonMode): void {
    setTemporary(false); if (modeMenu.current) modeMenu.current.open = false; props.onMode(next);
  }
  function command(value: string): void {
    root.current?.querySelectorAll<HTMLDetailsElement>("details[open]").forEach(details => { details.open = false; });
    setFileOpen(false); setTemporary(false); props.onCommand(value);
  }
  function selectTab(next: RibbonTab): void {
    setTab(next); setFileOpen(false); setTemporary(props.mode === "tabs");
  }
  useEffect(() => { setTemporary(false); }, [props.mode, props.active]);
  useEffect(() => {
    if (props.mode === "hidden") root.current?.querySelector<HTMLButtonElement>(".ribbon-restore")?.focus();
  }, [props.mode]);
  useEffect(() => {
    if (!root.current) return;
    const observer = new ResizeObserver(entries => setCompact(entries[0].contentRect.width < 1050));
    observer.observe(root.current);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!fileOpen) return;
    backButton.current?.focus();
  }, [fileOpen]);
  useEffect(() => {
    if (!temporary && !fileOpen) return;
    const outside = (event: PointerEvent) => {
      if (temporary && event.target instanceof Node && !root.current?.contains(event.target)) setTemporary(false);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) return;
      if (fileOpen) closeFile();
      else {
        setTemporary(false);
        const button = root.current?.querySelector<HTMLButtonElement>(`[data-ribbon-tab="${tab}"]`);
        if (button?.getClientRects().length) button.focus(); else moreButton.current?.focus();
      }
      event.preventDefault(); event.stopPropagation();
    };
    document.addEventListener("pointerdown", outside);
    window.addEventListener("keydown", escape, true);
    return () => { document.removeEventListener("pointerdown", outside); window.removeEventListener("keydown", escape, true); };
  }, [temporary, fileOpen, tab]);
  function action(label: string, Icon: Icon, value: string, disabled = false, selected = false): ReactNode {
    return <button key={value} type="button" className="ribbon-command" data-ribbon-command={value} disabled={disabled}
      aria-current={selected ? "page" : undefined} onClick={() => command(value)} title={label}>
      <Icon size={24} aria-hidden /><span>{label}</span>
    </button>;
  }
  function routes(keys: NavKey[]): ReactNode[] {
    return keys.flatMap(key => {
      const route = props.routes.find(route => route.key === key);
      return route ? [action(route.label, routeIcons[key], `navigate:${key}`, false, props.active === key)] : [];
    });
  }
  function capture(keys: HttpCommand[]): ReactNode[] {
    return keys.map(key => action(captureLabels[key], captureIcons[key], `capture:${key}`,
      props.active !== "http_capture" || !props.captureCommands.includes(key)));
  }
  function group(label: string, children: ReactNode): ReactNode {
    if (compact) return <details key={label} className="ribbon-group-overflow">
      <summary><PanelTop size={24} aria-hidden /><span>{label}</span><ChevronDown size={12} aria-hidden /></summary>
      <div className="ribbon-group-popup">{children}</div>
    </details>;
    return <section key={label} className="ribbon-group" aria-label={label}>
      <div className="ribbon-group-commands">{children}</div><span className="ribbon-group-label">{label}</span>
    </section>;
  }
  function panel(): ReactNode {
    switch (tab) {
      case "home": return <>
        {group(w("File", "파일"), <>{action(w("Open…", "열기…"), FolderOpen, "open", !props.canOpen)}{action(w("Recent files", "최근 파일"), History, "recent", !props.canRecent)}{action(w("Save / Export", "저장 / 내보내기"), Save, "capture:save", props.active !== "http_capture" || !props.captureCommands.includes("save"))}</>)}
        {group(w("Tools", "도구"), routes(["profiler", "http_capture", "analysis_workspace"]))}
        {group(w("Reports", "보고서"), routes(["chart_studio", "export_center"]))}
      </>;
      case "analyze": return <>
        {group(w("Profiles", "프로파일"), routes(["profiler", "diff"]))}
        {group("JVM", routes(["gc_log", "jfr", "thread_dump"]))}
        {group(w("Logs", "로그"), routes(["access_log", "exception"]))}
        {group(w("Distributed", "분산 분석"), routes(["msa_profile", "trace_import"]))}
        {group(w("Browser", "브라우저"), routes(["browser_cpu", "browser_audit"]))}
      </>;
      case "http": return <>
        {group("HTTP", routes(["http_capture"]))}
        {group(w("Capture", "캡처"), capture(["start", "stop", "pause", "clear"]))}
        {group(w("Results", "결과"), capture(["find", "follow", "save"]))}
        {group(w("Groups", "그룹"), capture(["collapse", "expand"]))}
      </>;
      case "workspace": return <>
        {group(w("Results", "결과"), routes(["analysis_workspace", "evidence_board", "report_diff"]))}
        {group(w("Visualization", "시각화"), routes(["chart_studio", "incident_timeline", "slo_golden_signals", "service_flow"]))}
        {group(w("Export", "내보내기"), routes(["export_center"]))}
      </>;
      case "view": return <>
        {group(w("Ribbon", "리본"), <>
          <ModeCommand label={w("Always show ribbon", "리본 항상 표시")} Icon={PanelTop} mode="expanded" />
          <ModeCommand label={w("Show tabs only", "탭만 표시")} Icon={PanelTopClose} mode="tabs" />
          <ModeCommand label={w("Hide ribbon", "리본 숨김")} Icon={EyeOff} mode="hidden" />
        </>)}
        {group(w("Language", "언어"), <label className="ribbon-field"><Globe size={24} aria-hidden />
          <select aria-label={w("Language", "언어")} value={locale} onChange={event => command(`locale:${event.target.value}`)}>{locales.map(value => <option key={value} value={value}>{localeLabels[value]}</option>)}</select></label>)}
        {group(w("Theme", "테마"), <label className="ribbon-field"><span>{w("Theme", "테마")}</span><select aria-label={w("Theme", "테마")} value={props.theme} onChange={event => command(`theme:${event.target.value}`)}>
          <option value="light">{w("Light", "라이트")}</option><option value="dark">{w("Dark", "다크")}</option><option value="system">{w("System theme", "시스템 테마")}</option></select></label>)}
        {group(w("Settings", "설정"), routes(["settings"]))}
      </>;
      case "help": return group(w("Help", "도움말"), <>{action(w("Current tool help", "현재 도구 도움말"), CircleHelp, "help")}{action(w("User guide", "사용 설명서"), BookOpen, "help")}{action(w("About ArchScope", "ArchScope 정보"), Info, "about")}</>);
    }
  }
  function ModeCommand({ label, Icon, mode }: { label: string; Icon: Icon; mode: RibbonMode }): React.JSX.Element {
    return <button type="button" className="ribbon-command" aria-pressed={props.mode === mode} onClick={() => setMode(mode)}><Icon size={24} aria-hidden /><span>{label}</span></button>;
  }
  return <div className="desktop-ribbon" ref={root} data-mode={props.mode} data-file-open={fileOpen || undefined} data-compact={compact || undefined}>
    {!visible && <button type="button" className="ribbon-restore" onClick={() => setMode("expanded")}><PanelTop size={16} aria-hidden />{w("Show ribbon", "리본 표시")}</button>}
    <nav className="ribbon-tabs" aria-label={w("Menu tabs", "메뉴 탭")} hidden={!visible || fileOpen}
      onKeyDown={event => {
        if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key) || !(event.target instanceof HTMLButtonElement) || !event.target.classList.contains("ribbon-tab")) return;
        const buttons = [...event.currentTarget.querySelectorAll<HTMLButtonElement>(".ribbon-tab")].filter(button => button.getClientRects().length);
        const index = buttons.indexOf(event.target);
        const next = event.key === "Home" ? 0 : event.key === "End" ? buttons.length - 1 : (index + (event.key === "ArrowRight" ? 1 : -1) + buttons.length) % buttons.length;
        buttons[next]?.focus(); event.preventDefault();
      }}>
      <button type="button" ref={fileButton} className="ribbon-tab ribbon-file" aria-expanded={fileOpen} aria-controls="desktop-file-screen" onClick={() => { setTemporary(false); setFileOpen(true); }}>{w("File", "파일")}</button>
      {RIBBON_TABS.map(value => <button key={value} type="button" data-ribbon-tab={value}
        className={`ribbon-tab${["http", "workspace", "view", "help"].includes(value) ? " ribbon-secondary-tab" : ""}`}
        aria-pressed={tab === value} aria-controls="desktop-ribbon-panel" onClick={() => selectTab(value)} onDoubleClick={() => { setTemporary(false); props.onToggle(); }}>{tabLabels[value]}</button>)}
      <select ref={moreButton} className="ribbon-more-tabs" aria-label={w("More tabs", "다른 메뉴 탭")} value={["http", "workspace", "view", "help"].includes(tab) ? tab : ""} onChange={event => { if (event.target.value) selectTab(event.target.value as RibbonTab); }}>
        <option value="">{w("More", "더 보기")}</option>{(["http", "workspace", "view", "help"] as const).map(value => <option key={value} value={value}>{tabLabels[value]}</option>)}
      </select>
      <div className="ribbon-options">
        <details className="ribbon-mode-menu" ref={modeMenu}><summary aria-label={w("Ribbon display options", "리본 표시 옵션")}><PanelTop size={16} aria-hidden /><ChevronDown size={12} aria-hidden /></summary>
          <div className="ribbon-mode-popup">{(["expanded", "tabs", "hidden"] as const).map(value => <button key={value} type="button" aria-pressed={props.mode === value} onClick={() => setMode(value)}>{value === "expanded" ? w("Always show ribbon", "리본 항상 표시") : value === "tabs" ? w("Show tabs only", "탭만 표시") : w("Hide ribbon", "리본 숨김")}</button>)}</div>
        </details>
        <button type="button" className="ribbon-fold" aria-expanded={commandsVisible} aria-label={props.mode === "expanded" ? w("Collapse ribbon", "리본 접기") : w("Pin ribbon", "리본 펼침 고정")} onClick={() => { setTemporary(false); props.onToggle(); }}>{props.mode === "expanded" ? <ChevronUp size={16} aria-hidden /> : <Pin size={16} aria-hidden />}</button>
      </div>
    </nav>
    <div className={`ribbon-panel${props.mode === "tabs" ? " ribbon-panel-overlay" : ""}`} id="desktop-ribbon-panel" aria-label={tabLabels[tab]} hidden={!commandsVisible}>{panel()}</div>
    {fileOpen && <section id="desktop-file-screen" className="ribbon-backstage" aria-label={w("File", "파일")}>
      <nav aria-label={w("File actions", "파일 작업")}><button type="button" ref={backButton} onClick={closeFile}><ArrowLeft size={18} aria-hidden />{w("Back", "돌아가기")}</button>
        <button type="button" disabled={!props.canOpen} onClick={() => command("open")}><FolderOpen size={18} aria-hidden />{w("Open…", "열기…")}</button>
        <button type="button" disabled={!props.canRecent} onClick={() => command("recent")}><History size={18} aria-hidden />{w("Recent files", "최근 파일")}</button>
        <button type="button" disabled={props.active !== "http_capture" || !props.captureCommands.includes("save")} onClick={() => command("capture:save")}><Save size={18} aria-hidden />{w("Save / Export", "저장 / 내보내기")}</button>
        <button type="button" onClick={() => command("navigate:export_center")}><Download size={18} aria-hidden />{w("Export", "내보내기")}</button>
        <button type="button" onClick={() => command("navigate:settings")}><Settings size={18} aria-hidden />{w("Settings", "설정")}</button>
      </nav><div><h2>{w("File", "파일")}</h2><p>{props.routes.find(route => route.key === props.active)?.label}</p><p>{w("Open local files, save results, or export from the workspace.", "로컬 파일을 열거나 결과를 저장하고 워크스페이스에서 내보냅니다.")}</p></div>
    </section>}
  </div>;
}
