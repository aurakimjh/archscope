import * as bridge from "./bridge/visuals";
import { rasterizeVisual, type VisualRenderInput } from "./state/visualRaster";
export async function runVisualRenderSurface(token: string): Promise<void> {
  document.body.style.margin = "0"; document.body.style.background = "white";
  const abort = new AbortController(); const timer = setTimeout(() => abort.abort(), 50_000);
  try {
    if (!/^[a-f0-9]{32}$/.test(token)) throw new Error("visual_session_invalid");
    const input = JSON.parse(await bridge.renderInput(token)) as VisualRenderInput;
    await rasterizeVisual(input, async (page, png) => { await bridge.complete(token, page, png, ""); }, abort.signal);
  } catch (error) { await bridge.complete(token, -1, "", String(error).slice(0, 512)).catch(() => {}); }
  finally { clearTimeout(timer); }
}
