//go:build !windows

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// setupWindowBackdrop is a no-op away from Windows, where the window options
// already describe the whole effect: macOS draws the vibrancy material asked
// for through `Mac.Backdrop` (`MacBackdropTranslucent`) and Linux asks the
// window manager for a transparent window through `BackgroundTypeTranslucent`,
// which Wails enables only on a composited screen.
func setupWindowBackdrop(*application.WebviewWindow) {}
