//go:build layoutaudit

package main

import (
	"embed"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed layout_observer.js
var layoutObserver embed.FS
var layoutAuditWindow *application.WebviewWindow

// 실제 네이티브 WebView에서만 실행하는 명시적 검증 빌드이다.
// 스크립트와 결과 경로는 검증자가 지정하며 일반 제품 빌드에서는 제외된다.
func configureLayoutAudit(app *application.App, window *application.WebviewWindow) {
	layoutAuditWindow = window
	desiredZoom := 0.0
	if value := os.Getenv("ARCHSCOPE_LAYOUT_VIEWPORT"); value != "" {
		var viewport struct {
			Width  int     `json:"width"`
			Height int     `json:"height"`
			Zoom   float64 `json:"zoom"`
		}
		if err := json.Unmarshal([]byte(value), &viewport); err != nil {
			log.Fatal(err)
		}
		if viewport.Width < 640 || viewport.Width > 3840 || viewport.Height < 480 || viewport.Height > 2160 || viewport.Zoom < 0.5 || viewport.Zoom > 2 {
			log.Fatal("ARCHSCOPE_LAYOUT_VIEWPORT requires width640..3840, height480..2160, zoom0.5..2")
		}
		window.SetSize(viewport.Width, viewport.Height)
		desiredZoom = viewport.Zoom
	}
	decorateLayoutAuditMenu(app.Menu.GetApplicationMenu())
	path := os.Getenv("ARCHSCOPE_LAYOUT_SCRIPT")
	passive := path == ""
	var script []byte
	var err error
	if passive {
		script, err = layoutObserver.ReadFile("layout_observer.js")
		window.SetTitle("ArchScope Layout QA")
	} else {
		script, err = os.ReadFile(path)
	}
	if err != nil {
		log.Fatal(err)
	}
	output := os.Getenv("ARCHSCOPE_LAYOUT_OUT")
	if output == "" {
		output = filepath.Join(os.TempDir(), "archscope-layout-observations.json")
	}
	app.Event.On("layout:progress", func(event *application.CustomEvent) {
		if data, err := json.MarshalIndent(event.Data, "", "  "); err == nil {
			if err := os.WriteFile(output+".progress.json", data, 0600); err != nil {
				log.Print(err)
			}
		}
	})
	app.Event.On("layout:audit", func(event *application.CustomEvent) {
		// Retain the renderer result even if optional native geometry collection stalls.
		if data, err := json.MarshalIndent(event.Data, "", "  "); err == nil {
			if err := os.WriteFile(output+".renderer.json", data, 0600); err != nil {
				log.Print(err)
			}
		}
		x, y := window.Position()
		width, height := window.Size()
		receipt := map[string]any{"window": map[string]int{"x": x, "y": y, "width": width, "height": height}, "webviewZoom": window.GetZoom(), "report": event.Data}
		if screen, screenErr := window.GetScreen(); screenErr == nil && screen != nil {
			receipt["osScreenScaleFactor"] = screen.ScaleFactor
			receipt["screenLogicalBounds"] = screen.Bounds
			receipt["screenPhysicalBounds"] = screen.PhysicalBounds
		}
		data, err := json.MarshalIndent(receipt, "", "  ")
		if err == nil {
			err = os.WriteFile(output, data, 0600)
		}
		if err != nil {
			log.Print(err)
		}
		if !passive {
			app.Quit()
		}
	})
	go func() {
		for range 90 {
			time.Sleep(time.Second)
			if desiredZoom > 0 && window.GetZoom() != desiredZoom {
				window.SetZoom(desiredZoom)
			}
			window.ExecJS("if (document.readyState === 'complete' && !window.__layoutAuditRunning) {window.__layoutAuditRunning=true;" + string(script) + "}")
		}
	}()

	go func() {
		limit := 180 * time.Second
		if passive {
			limit = 30 * time.Minute
		}
		if value := os.Getenv("ARCHSCOPE_LAYOUT_MAX_SECONDS"); value != "" {
			seconds, err := strconv.Atoi(value)
			if err != nil || seconds < 30 || seconds > 21600 {
				log.Fatal("ARCHSCOPE_LAYOUT_MAX_SECONDS must be 30..21600")
			}
			limit = time.Duration(seconds) * time.Second
		}
		time.Sleep(limit)
		app.Quit()
	}()
}

func desktopApplicationName() string { return "ArchScope Layout QA" }

// 검증 창 크기와 배율도 네이티브 메뉴에서 선택한다. 일반 제품 빌드에서는 제외된다.
func decorateLayoutAuditMenu(menu *application.Menu) {
	if layoutAuditWindow == nil {
		return
	}
	audit := menu.AddSubmenu("Layout QA")
	for _, size := range []struct {
		label         string
		width, height int
	}{{"1280 x 800", 1280, 800}, {"1024 x 768", 1024, 768}, {"1280 x 1000 (file dialogs)", 1280, 1000}} {
		audit.Add(size.label).OnClick(func(_ *application.Context) { layoutAuditWindow.SetSize(size.width, size.height) })
	}
	audit.Add("100% zoom").OnClick(func(_ *application.Context) { layoutAuditWindow.SetZoom(1) })
	audit.Add("125% zoom").OnClick(func(_ *application.Context) { layoutAuditWindow.SetZoom(1.25) })
}
