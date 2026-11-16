# Desktop ribbon and display languages

The desktop app uses one in-window menu tab row: File, Home, Analyze, HTTP,
Workspace, View and Help. Selecting a command tab immediately changes its
ribbon. Selecting an analyzer command changes the active tool. Merely switching
ribbon tabs preserves the active page and its inputs.

The expanded ribbon uses 24 px command icons and an approximately 122 px total
tab/command area at normal desktop width. Below 1050 CSS px, command groups open
as compact popups. Below 760 CSS px, secondary tabs move into a selector. The
existing native macOS application menu remains in the OS menu bar. Windows and
Linux use the in-window ribbon without a second native window menu row.

## Visibility

- **Always show ribbon:** tabs and commands stay visible.
- **Show tabs only:** selecting a tab temporarily opens its commands. Escape or
  clicking outside closes that panel.
- **Hide ribbon:** a small Show ribbon button restores the expanded view.

Use the display options at the right of the tabs, the View tab, a tab double
click, or Ctrl+F1 to change visibility. The selected mode survives restart.
The File screen keeps the active page mounted; Back or Escape returns to it.
Unavailable HTTP commands are disabled using the active capture page's command
availability. Ribbon actions use the same command dispatcher as existing native
menus and shortcuts.

## Display languages

View → Language and Settings offer English (`en`), 한국어 (`ko`), 日本語 (`ja`),
简体中文 (`zh-CN`) and 繁體中文（台灣） (`zh-TW`). Traditional Chinese uses Taiwan
terminology. Hong Kong and Macao browser language preferences select this same
Traditional Chinese catalog; they do not have separate regional translations.

A saved language choice takes priority over the system language list. Chinese
script tags take priority over region tags: `zh-Hans` selects Simplified and
`zh-Hant` selects Traditional. Without a script, TW/HK/MO select Traditional;
other Chinese regions select Simplified. Unsupported languages fall back to
English. Language selection is stored under the existing
`archscope.profiler.locale` preference; ribbon mode uses
`archscope.desktop.ribbon.mode`.

The three added catalogs each cover 2,197 unique source phrases, including the
existing message keys, inline UI labels, contextual help and native menus.
Technical identifiers and original input/diagnostic data retain their original
values. Markdown manuals remain in English and Korean; links from Japanese and
Chinese UI use the English manual. Translated UI does not imply translated
input data or every engine-generated diagnostic sentence.

Run `python3 scripts/check-desktop-i18n.py` from the repository root to verify
phrase coverage and placeholders. Frontend state tests check language matching,
catalog key parity, ribbon preference fallback and HTTP command routing. The
explicit native `layoutaudit` build can execute
`scripts/verify-ribbon-native.js` using `ARCHSCOPE_LAYOUT_SCRIPT` and write a
receipt using `ARCHSCOPE_LAYOUT_OUT`. Independent language review and actual
Windows WebView2/DPI/keyboard acceptance remain separate from local builds and
cross-compilation. See `work_status.md` for the recorded validation scope.

For native responsive UI testing, the audit build accepts
`ARCHSCOPE_LAYOUT_VIEWPORT='{"width":1024,"height":768,"zoom":1.25}'` and
`ARCHSCOPE_LAYOUT_MAX_SECONDS=300`. These options are excluded from ordinary
product builds. WebView zoom is distinct from changing OS display scaling.
