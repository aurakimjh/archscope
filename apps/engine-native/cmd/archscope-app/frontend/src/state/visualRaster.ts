// The dedicated desktop surface consumes fixed SVG, not the visible DOM.
export type VisualRenderInput = { version: string; svg: string[]; width: number; height: number; font_url: string; font_sha256: string };
export async function rasterizeVisual(input: VisualRenderInput, consume: (page: number, png: string) => Promise<void>, signal?: AbortSignal): Promise<void> {
  if (input.version !== "1.0.0" || input.svg.length < 1 || input.svg.length > 5 || input.width * input.height > 8_000_000 || input.font_url !== "/offline-assets/fonts/NotoSansKR.ttf") throw new Error("visual_renderer_contract_invalid");
  const fontResponse = await fetch(input.font_url, { signal, credentials: "same-origin" }); if (!fontResponse.ok) throw new Error("visual_font_unavailable");
  const bytes = await fontResponse.arrayBuffer(); const sum = await crypto.subtle.digest("SHA-256", bytes); const digest = Array.from(new Uint8Array(sum), n => n.toString(16).padStart(2, "0")).join("");
  if (digest !== input.font_sha256) throw new Error("visual_font_hash_mismatch");
  const font = new FontFace("ReportPinned", bytes); await font.load(); document.fonts.add(font); await document.fonts.ready;
  if (!document.fonts.check('26px "ReportPinned"', "보고서 ABC 0123")) throw new Error("visual_font_not_ready");
  try {
    for (let page = 0; page < input.svg.length; page++) {
      if (signal?.aborted) throw new Error("visual_render_cancelled");
      const blob = new Blob([input.svg[page]], { type: "image/svg+xml;charset=utf-8" }); const url = URL.createObjectURL(blob);
      try {
        const image = new Image(); image.src = url; await image.decode();
        const canvas = document.createElement("canvas"); canvas.width = input.width; canvas.height = input.height;
        const context = canvas.getContext("2d"); if (!context) throw new Error("visual_canvas_unavailable"); context.drawImage(image, 0, 0, input.width, input.height);
        const png = canvas.toDataURL("image/png"); if (signal?.aborted) throw new Error("visual_render_cancelled"); await consume(page, png);
        canvas.width = 0; canvas.height = 0;
      } finally { URL.revokeObjectURL(url); }
    }
  } finally { document.fonts.delete(font); }
}
