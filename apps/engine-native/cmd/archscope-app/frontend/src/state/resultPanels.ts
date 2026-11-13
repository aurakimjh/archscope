import type { ResultCapabilities } from "./resultProviders.js";

// The backend contract test reads this exact inventory. Type/payload dispatch
// belongs to the backend descriptor registry; this is the renderer protocol.
export const RESULT_PANEL_RENDERERS = /* panel-contract:start */ {
  "profile": { "version": "1.0.0", "required_action": "tree" },
  "metric_history": { "version": "1.0.0", "required_action": "intervals" },
  "system": { "version": "1.0.0", "required_action": "details" },
  "inference": { "version": "1.0.0", "required_action": "metric" },
  "llm_trace": { "version": "1.0.0", "required_action": "spans" },
  "runtime": { "version": "1.0.0", "required_action": "details" },
  "execution": { "version": "1.0.0", "required_action": "details" },
  "gpu_execution": { "version": "1.0.0", "required_action": "events" }
} /* panel-contract:end */ as const;

export type ResultPanelID = keyof typeof RESULT_PANEL_RENDERERS;
export type ResultPanel = { id: string; version: string; actions: string[] };

export function resultPanelID(caps: ResultCapabilities | null | undefined): ResultPanelID | null {
  if (!caps?.compatibility.allowed || caps.version !== "1.0.0" || !caps.panel) return null;
  const panel = caps.panel;
  if (!Object.prototype.hasOwnProperty.call(RESULT_PANEL_RENDERERS, panel.id)) return null;
  const id = panel.id as ResultPanelID;
  const renderer = RESULT_PANEL_RENDERERS[id];
  return panel.version === renderer.version && Array.isArray(panel.actions) && panel.actions.includes(renderer.required_action) ? id : null;
}
