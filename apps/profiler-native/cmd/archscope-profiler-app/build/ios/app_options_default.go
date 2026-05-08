//go:build ios

// [한글] app_options_default — non-iOS 빌드용 modifyOptionsForIOS no-op.
// (참고: 빌드 태그가 ios 로 되어있는 점은 의도와 어긋날 수 있으니 별도 검토.
//  현 시점에는 코드 동작을 변경하지 않는 정책이므로 그대로 유지.)

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// modifyOptionsForIOS is a no-op on non-iOS platforms
//
// [한글] modifyOptionsForIOS — 다른 플랫폼에서는 옵션 수정 없음.
func modifyOptionsForIOS(opts *application.Options) {
	// No modifications needed for non-iOS platforms
}