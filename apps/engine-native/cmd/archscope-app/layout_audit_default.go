//go:build !layoutaudit

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func configureLayoutAudit(_ *application.App, _ *application.WebviewWindow) {}
