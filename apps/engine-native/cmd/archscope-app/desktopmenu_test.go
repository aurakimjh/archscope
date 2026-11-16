package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Same menu instance must react to route/input changes without leaving stale
// accelerators or an enabled Open command on tools without a file picker.
func TestDesktopMenuUpdatesContextAndLocale(t *testing.T) {
	for _, locale := range []string{"en", "ko"} {
		t.Run(locale, func(t *testing.T) {
			state := desktopMenuState{Locale: locale, Active: "profiler", Theme: "system", Routes: []desktopMenuRoute{{Key: "profiler", Label: "Profiler", Group: "analyze"}, {Key: "http_capture", Label: "HTTP", Group: "analyze"}}}
			menu := application.NewMenu()
			populateDesktopMenu(menu, state)
			defer menu.Destroy()
			open, start := "Open…", "Start"
			if locale == "ko" {
				open, start = "열기…", "시작"
			}
			if menu.FindByLabel(open).Enabled() || menu.FindByLabel(start).Enabled() {
				t.Fatal("inactive actions enabled")
			}
			state.CanOpen = true
			state.Active = "http_capture"
			state.CaptureCommands = []string{"start"}
			updateDesktopMenu(menu, state)
			if !menu.FindByLabel(open).Enabled() || !menu.FindByLabel(start).Enabled() {
				t.Fatal("available actions disabled")
			}
			if !strings.EqualFold(menu.FindByLabel(start).GetAccelerator(), "F9") {
				t.Fatal("capture shortcut lost")
			}
			state.CanOpen = false
			state.Active = "settings"
			updateDesktopMenu(menu, state)
			if menu.FindByLabel(open).Enabled() || menu.FindByLabel(start).Enabled() {
				t.Fatal("stale enabled action")
			}
		})
	}
}

func TestDesktopMenuUsesPublishedLocaleCatalogs(t *testing.T) {
	for _, locale := range []string{"ja", "zh-CN", "zh-TW"} {
		t.Run(locale, func(t *testing.T) {
			data, err := os.ReadFile("frontend/src/i18n/" + locale + ".ts")
			if err != nil {
				t.Fatal(err)
			}
			start, end := strings.IndexByte(string(data), '{'), strings.LastIndexByte(string(data), '}')
			var labels map[string]string
			if start < 0 || end < start {
				t.Fatal("invalid catalog")
			}
			if err := json.Unmarshal(data[start:end+1], &labels); err != nil {
				t.Fatal(err)
			}
			state := desktopMenuState{Locale: locale, Active: "http_capture", Labels: labels, CanOpen: true, CaptureCommands: []string{"stop"}}
			menu := application.NewMenu()
			populateDesktopMenu(menu, state)
			defer menu.Destroy()
			for _, name := range []string{"File", "Analyze", "Workspace", "Capture", "View", "Help", "Open…", "Start", "Stop"} {
				if labels[name] == "" || labels[name] == name {
					t.Fatalf("missing localized label %q", name)
				}
				if menu.FindByLabel(labels[name]) == nil {
					t.Fatalf("native menu did not use translation for %q", name)
				}
			}
			if menu.FindByLabel(labels["Start"]).Enabled() || !menu.FindByLabel(labels["Stop"]).Enabled() {
				t.Fatal("capture availability differs from renderer")
			}
			for _, language := range []struct{ id, label string }{{"en", "English"}, {"ko", "한국어"}, {"ja", "日本語"}, {"zh-CN", "简体中文"}, {"zh-TW", "繁體中文（台灣）"}} {
				item := menu.FindByLabel(language.label)
				if item == nil || item.Checked() != (language.id == locale) {
					t.Fatal("wrong language selection", language.id)
				}
			}
		})
	}
}
