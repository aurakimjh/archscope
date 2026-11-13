// ─────────────────────────────────────────────────────────────────────
// [한글] archscope — Wails3 데스크톱 셸 진입점.
//
// 책임/목적
//   통합 engine-native 데스크톱 앱(Wails v3) 진입점. frontend(React/Vite
//   빌드 산출물) 를 embed 하고, ProfilerService/EngineService/CaptureService를
//   등록해 renderer 에서 분석과 실시간 캡처를 호출할 수 있도록 한다.
//
// 핵심 구성
//   - assets_* 파일             : production 에서는 frontend/dist 를 embed,
//                                 dev/test 에서는 컴파일 가능한 DirFS fallback 사용
//   - RegisterEvent[T]              : 타입드 이벤트 등록 (generated JS handler)
//   - ProfilerService / EngineService / CaptureService : 분석·캡처 서비스 노출
//
// 등록 이벤트
//   profiler 측 — analyze:done / analyze:error / analyze:cancelled
//   engine 측   — engine:done  / engine:error  / engine:cancelled
//   (multithread / lockcontention 분석은 비동기 경로라 같은 패턴 적용.)
//
// 윈도우 옵션
//   1280x800 디폴트, 다크 배경(#0F172A), Mac 타이틀바 hidden-inset, 마지막
//   윈도우 종료 시 앱 종료. URL="/" 는 React Router 의 root.
// ─────────────────────────────────────────────────────────────────────

package main

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/comautomation"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// [한글] init — 패키지 로드 시 frontend 와 주고받을 typed event 등록.
// Wails 바인딩 생성기가 이 정보를 보고 강타입 JS 핸들러를 만든다.
func init() {
	// Register typed events so the binding generator can produce strongly
	// typed JS handlers and the renderer doesn't have to guess payloads.
	application.RegisterEvent[AnalyzeDoneEvent]("analyze:done")
	application.RegisterEvent[AnalyzeErrorEvent]("analyze:error")
	application.RegisterEvent[AnalyzeCancelledEvent]("analyze:cancelled")

	// Engine-side typed events (multithread / lockcontention async paths).
	// Same pattern as the profiler events above so the renderer can
	// listen with the generated EngineDoneEvent etc. interfaces.
	application.RegisterEvent[EngineDoneEvent]("engine:done")
	application.RegisterEvent[EngineErrorEvent]("engine:error")
	application.RegisterEvent[EngineCancelledEvent]("engine:cancelled")

	application.RegisterEvent[capture.Session]("capture:started")
	application.RegisterEvent[CaptureProgressEvent]("capture:progress")
	application.RegisterEvent[CaptureTransactionsEvent]("capture:transactions")
	application.RegisterEvent[CaptureAggregateEvent]("capture:aggregate")
	application.RegisterEvent[capture.Stats]("capture:stats")
	application.RegisterEvent[capture.Session]("capture:stopped")
	application.RegisterEvent[CaptureErrorEvent]("capture:error")
}

// [한글] main — Wails 앱 인스턴스 생성, 서비스/윈도우/asset 설정 후 Run.
// Run 은 GUI 이벤트 루프 진입. 에러 시 즉시 log.Fatal.
func main() {
	offline, remaining, err := parseOffline(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	defer writeOfflineReport(offline.Report)
	options, err := automation.ParseOptions(remaining)
	if err != nil {
		log.Fatal(err)
	}
	if options.Help {
		fmt.Print(automation.Usage)
		return
	}
	captureService := NewCaptureService()
	webviewDataPath := ""
	if options.Descriptor != "" {
		root := filepath.Join(filepath.Dir(options.Descriptor), ".archscope-captures")
		captureService = newCaptureService(root, certstore.NewSystem(filepath.Join(root, "ca-trust.json")))
		// Separate automation instances must not share a WebView browser process,
		// local preferences or renderer state with the user's normal application.
		webviewDataPath = filepath.Join(filepath.Dir(options.Descriptor), ".archscope-webview2")
	}
	captureService.launchOptions = options
	engineService := &EngineService{}
	app := application.New(application.Options{
		Name:        desktopApplicationName(),
		Description: "Local-first architecture diagnostics desktop",
		Services: []application.Service{
			application.NewService(&ProfilerService{}),
			application.NewService(engineService),
			application.NewService(captureService),
		},
		Assets: application.AssetOptions{
			Handler: desktopAssets(application.AssetFileServerFS(assets)),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			AdditionalBrowserArgs: windowsAdditionalBrowserArgs(),
			WebviewBrowserPath:    offline.Webview,
			WebviewUserDataPath:   webviewDataPath,
		},
	})

	app.OnShutdown(captureService.shutdownAutomation)
	app.OnShutdown(engineService.closeEvidence)
	if options.Descriptor != "" {
		controller, err := automation.Start(options.Descriptor, captureService.executeAutomation)
		if err != nil {
			log.Fatal(err)
		}
		app.OnShutdown(func() { _ = controller.Close() })
	}
	menu := buildDesktopMenu(desktopMenuState{Locale: "en", Theme: "system", Active: "profiler"})
	app.Menu.Set(menu)
	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Windows: application.WindowsWindow{Menu: menu},
		Title:   "ArchScope",
		Width:   1280,
		Height:  800,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 30,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(15, 23, 42),
		URL:              "/",
	})

	configureVisualRenderer(app, engineService)
	configureDesktopMenu(app, window)
	configureLayoutAudit(app, window)

	var visible atomic.Bool
	visible.Store(true)
	captureService.windowVisibility = func(value *bool) (bool, error) {
		captureService.lifecycleMu.Lock()
		defer captureService.lifecycleMu.Unlock()
		if captureService.closing {
			return false, fmt.Errorf("desktop is shutting down")
		}
		if value != nil {
			requested := *value
			visible.Store(requested)
			application.InvokeAsync(func() {
				if requested {
					window.Show()
				} else {
					window.Hide()
				}
			})
		}
		return visible.Load(), nil
	}
	if options.COMServer {
		server, err := comautomation.Start(captureService.executeAutomation, comautomation.WithAdmission(captureService.manager.Admissions()))
		if err != nil {
			log.Fatal(err)
		}
		app.OnShutdown(func() {
			if err := server.Close(); err != nil {
				log.Print(err)
			}
		})
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
