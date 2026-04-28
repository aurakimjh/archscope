type FileDropZoneProps = {
  label: string;
  description?: string;
};

export function FileDropZone({
  label,
  description,
}: FileDropZoneProps): JSX.Element {
  return (
    <label className="file-zone">
      <span>{label}</span>
      {description && <small>{description}</small>}
      <input type="file" />
    </label>
  );
}
