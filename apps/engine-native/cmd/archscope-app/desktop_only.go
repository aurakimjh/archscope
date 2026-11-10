//go:build server || android || ios

package main

// 생성 템플릿이나 EXTRA_TAGS로 GUI 없는 서버/모바일 배포 경로가 다시
// 활성화되어도 컴파일을 중단한다. 제품 진입점은 네이티브 Wails 데스크톱뿐이다.
var _ = ArchScope_requires_native_Wails_desktop_server_and_mobile_builds_are_disabled
