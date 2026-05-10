//go:build android

// ─────────────────────────────────────────────────────────────────────
// [한글] main_android — Android c-shared 빌드 진입 hook.
//
// Android 빌드는 c-shared 모드라 일반적인 main() 자동 호출이 일어나지 않는다.
// 따라서 패키지 init 단계에서 application.RegisterAndroidMain(main) 으로
// 진입점을 등록하면 Wails 안드로이드 런타임이 적절한 시점에 main() 을 호출.
// 이 파일은 //go:build android 태그가 있어 안드로이드 빌드에서만 컴파일된다.
// ─────────────────────────────────────────────────────────────────────

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// [한글] init — c-shared 모드에서 main() 자동 호출 안 되므로 명시적 등록.
func init() {
	// Register main function to be called when the Android app initializes
	// This is necessary because in c-shared build mode, main() is not automatically called
	application.RegisterAndroidMain(main)
}
