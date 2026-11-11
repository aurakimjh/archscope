// The backend owns the immutable revision. Opaque JSON remains text across
// Wails and Report Pack; never parse/stringify it through JavaScript numbers.
export type EvidenceScalar = {
  present: boolean;
  availability: string;
  reason?: string;
  value: string | null;
  unit?: string;
  population?: string;
};

export function evidenceScalarText(value: EvidenceScalar): string {
  if (!value.present || value.availability === "unavailable" || value.value === null) return "—";
  return value.value;
}

export type BackendSourceReference = { backend_result_handle?: string };
export async function backendSourceAppendix(
  entries: BackendSourceReference[],
  readJSON: (handle: string) => Promise<string>,
): Promise<Array<{ path: string; content: string }>> {
  const handles = [...new Set(entries.flatMap((entry) => entry.backend_result_handle ? [entry.backend_result_handle] : []))];
  if (handles.length > 32) throw new Error("Report Pack backend source limit exceeded");
  const files: Array<{ path: string; content: string }> = [];
  let bytes = 0;
  for (const handle of handles) {
    if (!/^result-[0-9a-f]{32}$/.test(handle)) throw new Error("Invalid backend result handle");
    const content = await readJSON(handle);
    bytes += new TextEncoder().encode(content).byteLength;
    if (bytes > 64 * 1024 * 1024) throw new Error("Report Pack backend source byte limit exceeded");
    files.push({ path: `sources/${handle}.json`, content });
  }
  return files;
}

let backendReader: ((handle: string) => Promise<string>) | null = null;
export function configureBackendSourceReader(reader: (handle: string) => Promise<string>): void { backendReader = reader; }
export function readBackendSource(handle: string): Promise<string> {
  if (!backendReader) return Promise.reject(new Error("Native evidence bridge is unavailable"));
  return backendReader(handle);
}
