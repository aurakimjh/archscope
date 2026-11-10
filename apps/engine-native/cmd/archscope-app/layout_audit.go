//go:build layoutaudit

package main

import (
	"embed"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
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
	app.Event.On("layout:audit", func(event *application.CustomEvent) {
		x, y := window.Position()
		width, height := window.Size()
		data, err := json.MarshalIndent(map[string]any{"window": map[string]int{"x": x, "y": y, "width": width, "height": height}, "report": event.Data}, "", "  ")
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
			window.ExecJS("if (document.readyState === 'complete' && !window.__layoutAuditRunning) {window.__layoutAuditRunning=true;" + string(script) + "}")
		}
	}()

	go func() {
		limit := 180 * time.Second
		if passive {
			limit = 30 * time.Minute
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
	}{{"1280 x 800", 1280, 800}, {"1024 x 768", 1024, 768}} {
		audit.Add(size.label).OnClick(func(_ *application.Context) { layoutAuditWindow.SetSize(size.width, size.height) })
	}
	audit.Add("100% zoom").OnClick(func(_ *application.Context) { layoutAuditWindow.SetZoom(1) })
	audit.Add("125% zoom").OnClick(func(_ *application.Context) { layoutAuditWindow.SetZoom(1.25) })
}
