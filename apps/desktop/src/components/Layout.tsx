import type { PageKey } from "../App";
import type { ReactNode } from "react";
import { Sidebar } from "./Sidebar";

type LayoutProps = {
  activePage: PageKey;
  onNavigate: (page: PageKey) => void;
  children: ReactNode;
};

export function Layout({
  activePage,
  onNavigate,
  children,
}: LayoutProps): JSX.Element {
  return (
    <div className="app-shell">
      <Sidebar activePage={activePage} onNavigate={onNavigate} />
      <main className="main-panel">
        <header className="topbar">
          <div>
            <p className="eyebrow">Application Architecture Diagnostic Toolkit</p>
            <h1>ArchScope</h1>
          </div>
          <div className="pipeline">
            <span>Raw Data</span>
            <span>Parsing</span>
            <span>Analysis</span>
            <span>Visualization</span>
            <span>Export</span>
          </div>
        </header>
        {children}
      </main>
    </div>
  );
}
