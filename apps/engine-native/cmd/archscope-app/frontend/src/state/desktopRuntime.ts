type NativeHost = {
  chrome?: { webview?: { postMessage?: unknown } };
  webkit?: { messageHandlers?: { external?: { postMessage?: unknown } } };
  navigator?: { userAgent?: string };
};

// Wails 3.1.0 System.invoke가 사용하는 실제 네이티브 전송 채널을 확인한다.
// _wails 객체만으로 판단하면 일반 브라우저에서도 npm 런타임이 이를 생성한다.
// 이 검사는 잘못된 UI 미리보기 방지용이며 보안 인증 경계는 아니다.
export function hasNativeDesktopBridge(host: NativeHost): boolean {
  if (/Electron\//i.test(host.navigator?.userAgent ?? "")) return false;
  return typeof host.chrome?.webview?.postMessage === "function"
    || typeof host.webkit?.messageHandlers?.external?.postMessage === "function";
}
