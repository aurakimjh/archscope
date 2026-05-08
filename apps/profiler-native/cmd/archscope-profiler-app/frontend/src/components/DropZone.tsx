// ─────────────────────────────────────────────────────────────────────
// [한글] DropZone.tsx — 화면 전체에서 OS 파일 드롭을 가로채는 오버레이.
//
// 책임/목적: window 의 dragenter/over/leave/drop 이벤트를 listen 하여
// dragenter 카운터(counterRef) 로 nested 진입을 추적, hovering 상태가
// true 일 때만 반투명 안내 오버레이를 렌더. drop 시 첫 번째 파일의
// path (Wails v3 가 주입) 또는 name 을 onDrop 으로 전달합니다.
//
// 의존성 주의: 표준 브라우저 File 객체에는 path 속성이 없습니다.
// Wails v3 webview 가 File interface 위에 절대경로를 덧붙이는 패치를
// 적용해 주기 때문에 `(file as any).path` 로 캐스팅해 사용합니다 —
// 일반 브라우저에서 import 하면 name 만 떨어진다는 점에 유의.
// ─────────────────────────────────────────────────────────────────────
// Lightweight drop overlay. Wails v3 webview exposes the dropped file via the
// browser DragEvent's `dataTransfer.files[0].path` (Wails injects the absolute
// path on top of the standard File interface). When that's unavailable, fall
// back to the file's `name` so the path-input still gets a usable hint.

import { useEffect, useRef, useState, type ReactNode } from "react";

import { useI18n } from "../i18n/I18nProvider";

export type DropZoneProps = {
  onDrop: (path: string) => void;
  children: ReactNode;
};

export function DropZone({ onDrop, children }: DropZoneProps) {
  const { t } = useI18n();
  const [hovering, setHovering] = useState(false);
  const counterRef = useRef(0);
  const overlayRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const handleDragEnter = (event: DragEvent) => {
      if (!event.dataTransfer?.types?.includes("Files")) return;
      event.preventDefault();
      counterRef.current += 1;
      setHovering(true);
    };
    const handleDragOver = (event: DragEvent) => {
      if (!event.dataTransfer?.types?.includes("Files")) return;
      event.preventDefault();
      event.dataTransfer.dropEffect = "copy";
    };
    const handleDragLeave = () => {
      counterRef.current = Math.max(0, counterRef.current - 1);
      if (counterRef.current === 0) setHovering(false);
    };
    const handleDrop = (event: DragEvent) => {
      if (!event.dataTransfer?.types?.includes("Files")) return;
      event.preventDefault();
      counterRef.current = 0;
      setHovering(false);
      const file = event.dataTransfer.files?.[0];
      if (!file) return;
      // Wails v3 patches File with `path`; standard browsers do not.
      const path = (file as any).path || file.name;
      if (path) onDrop(path);
    };

    window.addEventListener("dragenter", handleDragEnter);
    window.addEventListener("dragover", handleDragOver);
    window.addEventListener("dragleave", handleDragLeave);
    window.addEventListener("drop", handleDrop);
    return () => {
      window.removeEventListener("dragenter", handleDragEnter);
      window.removeEventListener("dragover", handleDragOver);
      window.removeEventListener("dragleave", handleDragLeave);
      window.removeEventListener("drop", handleDrop);
    };
  }, [onDrop]);

  return (
    <>
      {children}
      {hovering && (
        <div className="drop-overlay" ref={overlayRef}>
          <div className="drop-overlay-inner">
            <div className="drop-icon" aria-hidden="true">
              ⤓
            </div>
            <div className="drop-title">{t("dropHere")}</div>
            <div className="drop-hint">{t("dropHereHint")}</div>
          </div>
        </div>
      )}
    </>
  );
}
