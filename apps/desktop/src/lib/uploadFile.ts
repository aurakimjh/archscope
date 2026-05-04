/** Upload a File to the FastAPI engine and resolve its server-side path. */
export type UploadedFile = {
  filePath: string;
  originalName: string;
  size: number;
};

export async function uploadFile(file: File): Promise<UploadedFile> {
  const form = new FormData();
  form.append("file", file, file.name);
  const response = await fetch("/api/upload", {
    method: "POST",
    body: form,
  });
  if (!response.ok) {
    const detail = await response.text().catch(() => "");
    throw new Error(`Upload failed (HTTP ${response.status}): ${detail || response.statusText}`);
  }
  const payload = (await response.json()) as {
    ok?: boolean;
    filePath?: string;
    originalName?: string;
  };
  if (!payload.ok || typeof payload.filePath !== "string" || !payload.filePath) {
    throw new Error("Upload failed: server returned unexpected response.");
  }
  return {
    filePath: payload.filePath,
    originalName: payload.originalName ?? file.name,
    size: file.size,
  };
}
