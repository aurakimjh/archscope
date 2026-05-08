//go:build ios

// [한글] app_options_ios — iOS 빌드용 application.Options 보정.
// iOS 는 Go 런타임의 signal handler 가 시스템 signal 처리와 충돌하기 때문에
// DisableDefaultSignalHandler=true 로 비활성화 필수 (앱 크래시 방지).

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// modifyOptionsForIOS adjusts the application options for iOS
//
// [한글] modifyOptionsForIOS — iOS 전용 옵션 패치. 시그널 핸들러 비활성화.
func modifyOptionsForIOS(opts *application.Options) {
	// Disable signal handlers on iOS to prevent crashes
	opts.DisableDefaultSignalHandler = true
}