// ─────────────────────────────────────────────────────────────────────
// [한글] lib/uploadFile.ts — 브라우저에서 선택한 File 을 FastAPI 엔진으로
//   업로드하고, 서버측 임시 경로(filePath) 를 받는 헬퍼.
//
// 책임/목적:
//   - 웹 환경에서는 파일 시스템 절대 경로를 알 수 없으므로, 파일을
//     /api/upload 로 multipart POST 한 뒤 서버가 만든 임시 경로를 받아서
//     이후 분석 호출에서 그 경로를 사용합니다.
//   - 데스크톱(Electron) 환경에서는 preload 가 절대 경로를 직접 주므로
//     이 함수를 거치지 않습니다.
//
// 응답 계약:
//   { ok: true, filePath: string, originalName: string }
//   서버 측 코드는 engines/python/.../web/server.py 의 /upload 핸들러.
// ─────────────────────────────────────────────────────────────────────
import { getApiBase } from "@/api/apiBase";

/** Upload a File to the FastAPI engine and resolve its server-side path. */
export type UploadedFile = {
  filePath: string;
  originalName: string;
  size: number;
};

export async function uploadFile(file: File): Promise<UploadedFile> {
  const form = new FormData();
  form.append("file", file, file.name);
  const response = await fetch(`${getApiBase()}/upload`, {
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
