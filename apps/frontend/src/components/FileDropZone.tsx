// ─────────────────────────────────────────────────────────────────────
// [한글] FileDropZone.tsx — Electron 환경 전용 드롭존(File.path 사용).
//   웹 환경에서는 File.path 가 undefined 이므로 FileDock 을 사용해야 하며,
//   본 컴포넌트는 데스크톱 빌드의 절대경로를 직접 받을 수 있는 단순 zone.
//   레거시/특정 페이지에서만 사용되며 신규 코드는 FileDock 권장.
// ─────────────────────────────────────────────────────────────────────
type FileDropZoneProps = {
  label: string;
  description?: string;
  selectedPath?: string;
  browseLabel?: string;
  onBrowse?: () => void;
  onFileSelected?: (filePath: string | undefined) => void;
};

export function FileDropZone({
  label,
  description,
  selectedPath,
  browseLabel,
  onBrowse,
  onFileSelected,
}: FileDropZoneProps): JSX.Element {
  return (
    <div
      className="file-zone"
      onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => {
        event.preventDefault();
        const file = event.dataTransfer.files[0] as
          | (File & { path?: string })
          | undefined;
        onFileSelected?.(file?.path);
      }}
    >
      <span>{label}</span>
      {description && <small>{description}</small>}
      {selectedPath && <small className="selected-file">{selectedPath}</small>}
      <input
        type="file"
        onChange={(event) => {
          const file = event.currentTarget.files?.[0] as
            | (File & { path?: string })
            | undefined;
          onFileSelected?.(file?.path);
        }}
      />
      {onBrowse && (
        <button className="secondary-button" type="button" onClick={onBrowse}>
          {browseLabel}
        </button>
      )}
    </div>
  );
}
