package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"strings"
	"testing"
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
