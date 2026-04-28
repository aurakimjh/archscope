import type { PageKey } from "../App";

type NavItem = {
  key: PageKey;
  label: string;
};

const navItems: NavItem[] = [
  { key: "dashboard", label: "Dashboard" },
  { key: "access-log", label: "Access Log Analyzer" },
  { key: "gc-log", label: "GC Log Analyzer" },
  { key: "profiler", label: "Profiler Analyzer" },
  { key: "thread-dump", label: "Thread Dump Analyzer" },
  { key: "exception", label: "Exception Analyzer" },
  { key: "chart-studio", label: "Chart Studio" },
  { key: "export-center", label: "Export Center" },
  { key: "settings", label: "Settings" },
];

type SidebarProps = {
  activePage: PageKey;
  onNavigate: (page: PageKey) => void;
};

export function Sidebar({ activePage, onNavigate }: SidebarProps): JSX.Element {
  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-mark">AS</div>
        <div>
          <strong>ArchScope</strong>
          <span>Evidence Builder</span>
        </div>
      </div>
      <nav className="nav-list" aria-label="Primary navigation">
        {navItems.map((item) => (
          <button
            key={item.key}
            type="button"
            className={item.key === activePage ? "nav-item active" : "nav-item"}
            onClick={() => onNavigate(item.key)}
          >
            {item.label}
          </button>
        ))}
      </nav>
    </aside>
  );
}
