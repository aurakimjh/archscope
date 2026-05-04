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
