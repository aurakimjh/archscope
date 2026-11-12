package main

import "github.com/wailsapp/wails/v3/pkg/application"

// A dedicated hidden native WebView owns rasterization; the visible analysis
// window's viewport, scroll, theme, hover and devicePixelRatio are irrelevant.
func configureVisualRenderer(app *application.App, service *EngineService) {
	service.visualMu.Lock()
	defer service.visualMu.Unlock()
	service.launchVisual = func(token string, width, height int) (visualRenderWindow, error) {
		return application.InvokeSyncWithResultAndError(func() (visualRenderWindow, error) {
			window := app.Window.NewWithOptions(application.WebviewWindowOptions{Name: "visual-" + token, Title: "ArchScope report renderer", URL: "/?visual-render=" + token, Width: width, Height: height, Hidden: true, DisableResize: true, BackgroundColour: application.NewRGB(255, 255, 255), Windows: application.WindowsWindow{HiddenOnTaskbar: true}})
			return &visualWindow{window}, nil
		})
	}
}

type visualWindow struct{ window *application.WebviewWindow }

func (w *visualWindow) ID() uint     { return w.window.ID() }
func (w *visualWindow) Name() string { return w.window.Name() }
func (w *visualWindow) Close()       { application.InvokeAsync(func() { w.window.Close() }) }
