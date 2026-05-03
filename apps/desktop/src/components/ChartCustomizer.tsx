import { useState } from "react";

export type ChartCustomOptions = {
  legendPosition: "top" | "bottom" | "left" | "right" | "hidden";
  backgroundColor: string;
  titleFontSize: number;
  seriesColors: string[];
};

const DEFAULT_COLORS = ["#4f46e5", "#06b6d4", "#10b981", "#f59e0b", "#ef4444", "#8b5cf6", "#ec4899"];
const BG_PRESETS = ["#ffffff", "transparent", "#f8fafc", "#1e1b4b", "#0f172a"];

type ChartCustomizerProps = {
  options: ChartCustomOptions;
  onChange: (options: ChartCustomOptions) => void;
  onClose: () => void;
};

export function ChartCustomizer({ options, onChange, onClose }: ChartCustomizerProps): JSX.Element {
  const [local, setLocal] = useState<ChartCustomOptions>({ ...options });

  function update(patch: Partial<ChartCustomOptions>): void {
    const next = { ...local, ...patch };
    setLocal(next);
    onChange(next);
  }

  function updateColor(index: number, color: string): void {
    const colors = [...local.seriesColors];
    colors[index] = color;
    update({ seriesColors: colors });
  }

  return (
    <div className="chart-customizer">
      <div className="chart-customizer-header">
        <strong>Chart Options</strong>
        <button type="button" className="chart-customizer-close" onClick={onClose}>
          <svg viewBox="0 0 24 24" width="16" height="16">
            <path d="M6 18L18 6M6 6l12 12" stroke="currentColor" fill="none" strokeWidth="2" strokeLinecap="round" />
          </svg>
        </button>
      </div>

      <label className="chart-customizer-field">
        Legend Position
        <select value={local.legendPosition} onChange={(e) => update({ legendPosition: e.target.value as ChartCustomOptions["legendPosition"] })}>
          <option value="top">Top</option>
          <option value="bottom">Bottom</option>
          <option value="left">Left</option>
          <option value="right">Right</option>
          <option value="hidden">Hidden</option>
        </select>
      </label>

      <label className="chart-customizer-field">
        Background
        <div className="color-preset-row">
          {BG_PRESETS.map((bg) => (
            <button
              key={bg}
              type="button"
              className={`color-swatch ${local.backgroundColor === bg ? "active" : ""}`}
              style={{ background: bg === "transparent" ? "repeating-conic-gradient(#ccc 0% 25%, #fff 0% 50%) 50% / 12px 12px" : bg }}
              onClick={() => update({ backgroundColor: bg })}
              title={bg}
            />
          ))}
        </div>
      </label>

      <label className="chart-customizer-field">
        Title Font Size
        <input
          type="range"
          min="12"
          max="24"
          value={local.titleFontSize}
          onChange={(e) => update({ titleFontSize: Number(e.target.value) })}
        />
        <span className="range-value">{local.titleFontSize}px</span>
      </label>

      <label className="chart-customizer-field">
        Series Colors
        <div className="color-preset-row">
          {local.seriesColors.slice(0, 6).map((color, i) => (
            <input
              key={i}
              type="color"
              className="color-input"
              value={color}
              onChange={(e) => updateColor(i, e.target.value)}
            />
          ))}
        </div>
      </label>

      <button type="button" className="secondary-button" onClick={() => update({ seriesColors: [...DEFAULT_COLORS] })}>
        Reset Colors
      </button>
    </div>
  );
}

export const defaultCustomOptions: ChartCustomOptions = {
  legendPosition: "bottom",
  backgroundColor: "#ffffff",
  titleFontSize: 16,
  seriesColors: [...DEFAULT_COLORS],
};
