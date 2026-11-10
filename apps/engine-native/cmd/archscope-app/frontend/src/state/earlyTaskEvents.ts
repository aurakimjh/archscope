// A native task can finish before its IPC start reply supplies the task ID.
// Retain only events during that short handshake, then replay against the bound ID.
export class EarlyTaskEvents {
  private revision = 0;
  private preparing = false;
  private pending: Array<() => void> = [];
  begin(): number { this.cancel();this.preparing=true;return this.revision; }
  isCurrent(run: number): boolean { return this.revision === run; }
  defer(eventID: string, activeID: string | null, replay: () => void): boolean {
    if (!this.preparing || eventID === activeID) return false;
    if (this.pending.length < 16) this.pending.push(replay);
    return true;
  }
  bind(run: number): boolean {
    if (!this.isCurrent(run)) return false;
    this.preparing=false;
    const events=this.pending;this.pending=[];
    for (const replay of events) replay();
    return true;
  }
  cancel(): void { this.revision++;this.preparing=false;this.pending=[]; }
}
