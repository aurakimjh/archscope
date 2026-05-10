//go:build ios

// ─────────────────────────────────────────────────────────────────────
// [한글] main_ios — iOS c-shared 빌드 진입 hook.
//
// iOS 는 Objective-C 런타임에서 main 진입을 호출하므로, Go 의 main 을
// "C" 로 export 한 WailsIOSMain wrapper 를 노출한다.
// 중요한 점은 runtime.LockOSThread() 를 절대 호출하지 말 것 — iOS 의
// signal 처리 모델과 Go 런타임의 sigaltstack 사용이 충돌해 fatal error 발생.
// ─────────────────────────────────────────────────────────────────────

package main

import (
	"C"
)

// For iOS builds, we need to export a function that can be called from Objective-C
// This wrapper allows us to keep the original main.go unmodified
//
// [한글] WailsIOSMain — Objective-C 측에서 호출하는 진입점.
// 내부에서 main() 을 그대로 호출하여 일반 Wails 앱 동작과 동등하게 동작.
//
//export WailsIOSMain
func WailsIOSMain() {
	// DO NOT lock the goroutine to the current OS thread on iOS!
	// This causes signal handling issues:
	// "signal 16 received on thread with no signal stack"
	// "fatal error: non-Go code disabled sigaltstack"
	// iOS apps run in a sandboxed environment where the Go runtime's
	// signal handling doesn't work the same way as desktop platforms.

	// Call the actual main function from main.go
	// This ensures all the user's code is executed
	main()
}