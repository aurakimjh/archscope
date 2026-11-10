//go:build layoutaudit

package main

import (
	"encoding/json"
	"github.com/wailsapp/wails/v3/pkg/application"

	"log"
	"os"
	"time"
)

// 실제 네이티브 WebView에서만 실행하는 명시적 검증 빌드이다.
// 스크립트와 결과 경로는 검증자가 지정하며 일반 제품 빌드에서는 제외된다.
func configureLayoutAudit(app *application.App, window *application.WebviewWindow) {
	script, err := os.ReadFile(os.Getenv("ARCHSCOPE_LAYOUT_SCRIPT"))
	if err != nil {
		log.Fatal(err)
	}
	app.Event.On("layout:audit", func(event *application.CustomEvent) {
		data, err := json.MarshalIndent(event.Data, "", "  ")
		if err == nil {
			err = os.WriteFile(os.Getenv("ARCHSCOPE_LAYOUT_OUT"), data, 0600)
		}
		if err != nil {
			log.Print(err)
		}
		app.Quit()
	})
	go func() {
		for range 90 {
			time.Sleep(time.Second)
			window.ExecJS("if (document.readyState === 'complete' && !window.__layoutAuditRunning) {window.__layoutAuditRunning=true;" + string(script) + "}")
		}
	}()

	go func() { time.Sleep(180 * time.Second); app.Quit() }()
}
