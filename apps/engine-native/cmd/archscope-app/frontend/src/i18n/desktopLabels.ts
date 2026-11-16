import type { Locale } from "./locales";
import { translateText } from "./translations";

const labels = {
  "Open…": "열기…",
  "Recent files": "최근 파일",
  "File": "파일",
  "Export…": "내보내기…",
  "Edit": "편집",
  "Analyze": "분석",
  "Workspace": "워크스페이스",
  "Capture": "캡처",
  "HTTP Capture": "HTTP 캡처",
  "View": "보기",
  "Settings…": "설정…",
  "Help": "도움말",
  "Current tool help": "현재 도구 도움말",
  "About ArchScope": "ArchScope 정보",
  "Start": "시작",
  "Stop": "중지",
  "Pause / Resume": "일시정지 / 재개",
  "Clear": "지우기",
  "Auto Scroll": "자동 스크롤",
  "Find": "찾기",
  "Save / Export": "저장 / 내보내기",
  "Collapse Groups": "그룹 접기",
  "Expand Groups": "그룹 펼치기",
  "Light": "라이트",
  "Dark": "다크",
  "System theme": "시스템 테마"
};

export function desktopMenuLabels(locale: Locale): Record<string, string> {
  return Object.fromEntries(Object.entries(labels).map(([en, ko]) => [en, translateText(locale, en, ko)]));
}
