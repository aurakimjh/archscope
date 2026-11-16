package main

import (
	"encoding/json"
	"github.com/wailsapp/wails/v3/pkg/application"
	"runtime"
	"strings"
)

type desktopMenuRoute struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Group string `json:"group"`
}
type desktopMenuState struct {
	Locale          string             `json:"locale"`
	Theme           string             `json:"theme"`
	Active          string             `json:"active"`
	CanRecent       bool               `json:"canRecent"`
	CanOpen         bool               `json:"canOpen"`
	Routes          []desktopMenuRoute `json:"routes"`
	Labels          map[string]string  `json:"labels"`
	CaptureCommands []string           `json:"captureCommands"`
}

// 네이티브 메뉴는 renderer의 활성 도구/언어와 동기화한다.
// 동일 명령은 기존 화면의 조작 경로로 전달하므로 분석 상태를 복제하지 않는다.
func configureDesktopMenu(app *application.App, window *application.WebviewWindow) {
	configureRibbonKeyBindings(window)
	if runtime.GOOS != "darwin" {
		// The renderer ribbon owns the single menu row. Removing the window menu
		// also removes its accelerators, so explicit window bindings replace them.
		window.SetMenu(nil)
		return
	}
	var signature string
	app.Event.On("desktop:state", func(event *application.CustomEvent) {
		data, err := json.Marshal(event.Data)
		if err != nil {
			return
		}
		var state desktopMenuState
		if json.Unmarshal(data, &state) != nil {
			return
		}
		application.InvokeAsync(func() {
			routes, _ := json.Marshal(state.Routes)
			labels, _ := json.Marshal(state.Labels)
			next := state.Locale + string(routes) + string(labels)
			menu := app.Menu.GetApplicationMenu()
			if signature != next {
				previous := menu
				menu = buildDesktopMenu(state)
				app.Menu.Set(menu)
				window.SetMenu(menu)
				previous.Destroy()
				signature = next
			} else {
				updateDesktopMenu(menu, state)
			}
		})
	})
}

func updateDesktopMenu(menu *application.Menu, state desktopMenuState) {
	label := func(en, kr string) string {
		return desktopMenuLabel(state, en, kr)
	}
	if item := menu.FindByLabel(label("Open…", "열기…")); item != nil {
		item.SetEnabled(state.CanOpen)
	}
	if item := menu.FindByLabel(label("Recent files", "최근 파일")); item != nil {
		item.SetEnabled(state.CanRecent)
	}
	for _, route := range state.Routes {
		if item := menu.FindByLabel(route.Label); item != nil {
			item.SetChecked(route.Key == state.Active)
		}
	}
	for _, c := range desktopCaptureCommands {
		if item := menu.FindByLabel(label(c.en, c.ko)); item != nil {
			item.SetEnabled(desktopCaptureCommandEnabled(state, c.name))
		}
	}
	for _, v := range desktopThemes {
		if item := menu.FindByLabel(label(v.en, v.ko)); item != nil {
			item.SetChecked(state.Theme == v.id)
		}
	}
}

var desktopThemes = []struct{ id, en, ko string }{{"light", "Light", "라이트"}, {"dark", "Dark", "다크"}, {"system", "System theme", "시스템 테마"}}
var desktopCaptureCommands = []struct{ en, ko, name, key string }{
	{"Start", "시작", "start", "F9"}, {"Stop", "중지", "stop", "F2"}, {"Pause / Resume", "일시정지 / 재개", "pause", ""},
	{"Clear", "지우기", "clear", "Ctrl+L"}, {"Auto Scroll", "자동 스크롤", "follow", ""}, {"Find", "찾기", "find", "Ctrl+F"},
	{"Save / Export", "저장 / 내보내기", "save", "Ctrl+S"}, {"Collapse Groups", "그룹 접기", "collapse", "Ctrl+Shift+Left"}, {"Expand Groups", "그룹 펼치기", "expand", "Ctrl+Shift+Right"},
}

func buildDesktopMenu(state desktopMenuState) *application.Menu {
	menu := application.NewMenu()
	if runtime.GOOS == "darwin" {
		menu.AddRole(application.AppMenu)
	}
	populateDesktopMenu(menu, state)
	decorateLayoutAuditMenu(menu)
	return menu
}

func populateDesktopMenu(menu *application.Menu, state desktopMenuState) {
	label := func(en, kr string) string {
		return desktopMenuLabel(state, en, kr)
	}
	add := func(parent *application.Menu, text, command, shortcut string) *application.MenuItem {
		item := parent.Add(text).OnClick(func(_ *application.Context) { emitEvent("desktop:command", map[string]string{"command": command}) })
		if shortcut != "" {
			if runtime.GOOS == "darwin" {
				shortcut = strings.Replace(shortcut, "Ctrl+", "Cmd+", 1)
			}
			item.SetAccelerator(shortcut)
		}
		return item
	}
	file := menu.AddSubmenu(label("File", "파일"))
	add(file, label("Open…", "열기…"), "open", "Ctrl+O").SetEnabled(state.CanOpen)
	add(file, label("Recent files", "최근 파일"), "recent", "").SetEnabled(state.CanRecent)
	add(file, label("Export…", "내보내기…"), "navigate:export_center", "")
	menu.AddRole(application.EditMenu)
	menu.FindByLabel("Edit").SetLabel(label("Edit", "편집"))
	analyze := menu.AddSubmenu(label("Analyze", "분석"))
	workspace := menu.AddSubmenu(label("Workspace", "워크스페이스"))
	for _, route := range state.Routes {
		parent := analyze
		if route.Group == "workspace" {
			parent = workspace
		}
		if route.Key == "settings" {
			continue
		}
		item := add(parent, route.Label, "navigate:"+route.Key, "")
		item.SetChecked(route.Key == state.Active)
	}
	capture := menu.AddSubmenu(label("Capture", "캡처"))
	add(capture, label("HTTP Capture", "HTTP 캡처"), "navigate:http_capture", "")
	for _, c := range desktopCaptureCommands {
		name := c.name
		item := capture.Add(label(c.en, c.ko)).OnClick(func(_ *application.Context) { emitEvent("capture:command", map[string]string{"command": name}) })
		item.SetEnabled(desktopCaptureCommandEnabled(state, c.name))
		key := c.key
		if key != "" {
			if runtime.GOOS == "darwin" {
				key = strings.Replace(key, "Ctrl+", "Cmd+", 1)
			}
			item.SetAccelerator(key)
		}
	}
	view := menu.AddSubmenu(label("View", "보기"))
	add(view, label("Settings…", "설정…"), "navigate:settings", "Ctrl+,")
	for _, v := range desktopThemes {
		add(view, label(v.en, v.ko), "theme:"+v.id, "").SetChecked(state.Theme == v.id)
	}
	view.AddSeparator()
	for _, locale := range []struct{ id, label string }{{"en", "English"}, {"ko", "한국어"}, {"ja", "日本語"}, {"zh-CN", "简体中文"}, {"zh-TW", "繁體中文（台灣）"}} {
		add(view, locale.label, "locale:"+locale.id, "").SetChecked(state.Locale == locale.id)
	}
	help := menu.AddSubmenu(label("Help", "도움말"))
	add(help, label("Current tool help", "현재 도구 도움말"), "help", "F1")
	add(help, label("About ArchScope", "ArchScope 정보"), "about", "")
}

func desktopMenuLabel(state desktopMenuState, en, ko string) string {
	if text := state.Labels[en]; text != "" {
		return text
	}
	if state.Locale == "ko" {
		return ko
	}
	return en
}

func desktopCaptureCommandEnabled(state desktopMenuState, command string) bool {
	if state.Active != "http_capture" {
		return false
	}
	for _, available := range state.CaptureCommands {
		if available == command {
			return true
		}
	}
	return false
}

func configureRibbonKeyBindings(window *application.WebviewWindow) {
	window.RegisterKeyBinding("Ctrl+F1", func(application.Window) {
		emitEvent("desktop:command", map[string]string{"command": "ribbon:toggle"})
	})
	if runtime.GOOS == "darwin" {
		return // The system menu keeps its native accelerator ownership on macOS.
	}
	if runtime.GOOS != "windows" {
		window.RegisterKeyBinding("Ctrl+,", func(application.Window) {
			emitEvent("desktop:command", map[string]string{"command": "navigate:settings"})
		})
	}
	// Windows OEM_COMMA is not accepted by Wails' accelerator parser. The
	// renderer owns that logical shortcut; do not register a second dead binding.
	for _, binding := range []struct{ key, command string }{{"Ctrl+O", "open"}, {"F1", "help"}} {
		command := binding.command
		window.RegisterKeyBinding(binding.key, func(application.Window) {
			emitEvent("desktop:command", map[string]string{"command": command})
		})
	}
	for _, binding := range desktopCaptureCommands {
		if binding.key == "" {
			continue
		}
		command := binding.name
		window.RegisterKeyBinding(binding.key, func(application.Window) {
			emitEvent("capture:command", map[string]string{"command": command})
		})
	}
}
