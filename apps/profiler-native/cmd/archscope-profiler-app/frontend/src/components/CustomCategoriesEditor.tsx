// CustomCategoriesEditor — preset + form for user-supplied timeline
// segment patterns. Used by both the profiler page (collapsed wall
// timeline customization) and the Jennifer page (MSA event-category
// extension). The shape is identical: { [segmentId]: pattern[] }.
//
// Patterns are case-insensitive substrings. First match wins on the
// backend so users can broaden a category without breaking built-in
// matches (those still apply in parallel for the canonical event
// types — see event_classifier.go).

import { Plus, Trash2 } from "lucide-react";

import { Button } from "./ui/button";
import { Input } from "./ui/input";
import { useI18n } from "../i18n/I18nProvider";

export type CategoryRules = Record<string, string[]>;

export type SegmentSpec = {
  id: string;
  label: string;
};

export type Preset = {
  id: string;
  label: string;
  rules: CategoryRules;
};

export type CustomCategoriesEditorProps = {
  segments: SegmentSpec[];
  presets: Preset[];
  value: CategoryRules;
  onChange: (next: CategoryRules) => void;
  disabled?: boolean;
};

export function CustomCategoriesEditor({
  segments,
  presets,
  value,
  onChange,
  disabled,
}: CustomCategoriesEditorProps) {
  const { t } = useI18n();

  const addPattern = (segmentId: string, pattern: string) => {
    const trimmed = pattern.trim();
    if (!trimmed) return;
    const existing = value[segmentId] ?? [];
    if (existing.includes(trimmed)) return;
    onChange({ ...value, [segmentId]: [...existing, trimmed] });
  };

  const removePattern = (segmentId: string, idx: number) => {
    const existing = value[segmentId] ?? [];
    const next = existing.filter((_, i) => i !== idx);
    if (next.length === 0) {
      const { [segmentId]: _drop, ...rest } = value;
      onChange(rest);
    } else {
      onChange({ ...value, [segmentId]: next });
    }
  };

  const applyPreset = (presetId: string) => {
    if (presetId === "") return;
    const preset = presets.find((p) => p.id === presetId);
    if (!preset) return;
    // Merge: preset patterns are appended to existing user patterns
    // (de-duplicated). We don't replace because users typically pick
    // a preset to seed and then refine inline.
    const merged: CategoryRules = { ...value };
    for (const [seg, patterns] of Object.entries(preset.rules)) {
      const existing = merged[seg] ?? [];
      const dedup = Array.from(new Set([...existing, ...patterns]));
      merged[seg] = dedup;
    }
    onChange(merged);
  };

  return (
    <div className="flex flex-col gap-3 rounded-md border border-border bg-muted/20 p-3">
      <div className="flex flex-wrap items-center gap-2">
        <label className="text-xs font-medium text-foreground/80">
          {t("customCategoryPreset")}:
        </label>
        <select
          className="h-8 rounded-md border border-input bg-transparent px-2 text-xs"
          disabled={disabled}
          defaultValue=""
          onChange={(e) => {
            applyPreset(e.target.value);
            e.currentTarget.value = "";
          }}
        >
          <option value="">{t("presetEmpty")}</option>
          {presets.map((p) => (
            <option key={p.id} value={p.id}>
              {p.label}
            </option>
          ))}
        </select>
        <span className="text-xs text-muted-foreground">
          {t("customCategoriesHint")}
        </span>
      </div>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        {segments.map((seg) => (
          <SegmentRow
            key={seg.id}
            segment={seg}
            patterns={value[seg.id] ?? []}
            onAdd={(p) => addPattern(seg.id, p)}
            onRemove={(idx) => removePattern(seg.id, idx)}
            disabled={disabled}
            addLabel={t("customCategoryAdd")}
          />
        ))}
      </div>
    </div>
  );
}

type SegmentRowProps = {
  segment: SegmentSpec;
  patterns: string[];
  onAdd: (pattern: string) => void;
  onRemove: (idx: number) => void;
  addLabel: string;
  disabled?: boolean;
};

function SegmentRow({
  segment,
  patterns,
  onAdd,
  onRemove,
  addLabel,
  disabled,
}: SegmentRowProps) {
  let inputEl: HTMLInputElement | null = null;
  return (
    <div className="flex flex-col gap-1.5 rounded border border-border bg-background/50 p-2">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-semibold uppercase tracking-wide text-foreground/80">
          {segment.label}
        </span>
        <code className="text-[10px] text-muted-foreground">{segment.id}</code>
      </div>
      <div className="flex flex-wrap gap-1">
        {patterns.length === 0 && (
          <span className="text-xs text-muted-foreground">—</span>
        )}
        {patterns.map((p, idx) => (
          <span
            key={`${p}-${idx}`}
            className="inline-flex items-center gap-1 rounded bg-primary/10 px-1.5 py-0.5 text-xs text-primary"
          >
            <code>{p}</code>
            <button
              type="button"
              className="text-primary/60 hover:text-destructive"
              onClick={() => onRemove(idx)}
              disabled={disabled}
              aria-label={`Remove ${p}`}
            >
              <Trash2 className="h-3 w-3" />
            </button>
          </span>
        ))}
      </div>
      <div className="flex items-center gap-1.5">
        <Input
          ref={(el) => {
            inputEl = el;
          }}
          type="text"
          placeholder="substring..."
          disabled={disabled}
          className="h-7 text-xs"
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              const v = e.currentTarget.value;
              onAdd(v);
              e.currentTarget.value = "";
            }
          }}
        />
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={disabled}
          onClick={() => {
            if (!inputEl) return;
            onAdd(inputEl.value);
            inputEl.value = "";
          }}
        >
          <Plus className="h-3 w-3" />
          {addLabel}
        </Button>
      </div>
    </div>
  );
}
