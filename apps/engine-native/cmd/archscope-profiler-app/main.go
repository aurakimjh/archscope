// ─────────────────────────────────────────────────────────────────────
// [한글] archscope-profiler-app — Wails3 데스크톱 셸 진입점.
//
// 책임/목적
//   통합 engine-native 데스크톱 앱(Wails v3) 진입점. frontend(React/Vite
//   빌드 산출물) 를 embed 하고, ProfilerService/EngineService 두 서비스를
//   등록해 renderer 에서 비동기 분석 호출이 가능하도록 한다.
//
// 핵심 구성
//   - assets_* 파일             : production 에서는 frontend/dist 를 embed,
//                                 dev/test 에서는 컴파일 가능한 DirFS fallback 사용
//   - RegisterEvent[T]              : 타입드 이벤트 등록 (generated JS handler)
//   - ProfilerService / EngineService : 분석 서비스 두 개를 노출
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
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// [한글] init — 패키지 로드 시 frontend 와 주고받을 typed event 6개 등록.
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
}

// [한글] main — Wails 앱 인스턴스 생성, 서비스/윈도우/asset 설정 후 Run.
// Run 은 GUI 이벤트 루프 진입. 에러 시 즉시 log.Fatal.
func main() {
	app := application.New(application.Options{
		Name:        "ArchScope Profiler",
		Description: "Native profiler-first slice of ArchScope",
		Services: []application.Service{
			application.NewService(&ProfilerService{}),
			application.NewService(&EngineService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "ArchScope Profiler",
		Width:  1280,
		Height: 800,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(15, 23, 42),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
