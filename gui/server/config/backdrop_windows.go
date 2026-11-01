//go:build windows

package main

import (
	"log"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Windows chooses the window backdrop by OS version. Wails covers the modern
// case by itself: Windows 11 (build 22621 and later) draws the Mica material
// asked for in main.go through DWMWA_SYSTEMBACKDROP_TYPE, while older versions
// fall back to the plain blur-behind accent, which blurs whatever sits behind
// the window without tinting it. Over a light wallpaper that leaves a dark
// interface floating on a bright blur, so the fallback is upgraded here to the
// acrylic variation of the same API, tinted with the surface the application is
// themed for. Windows 7 offers neither Mica nor acrylic, and this application
// cannot run there in any case: Wails builds on WebView2, which requires
// Windows 10 or later.

const (
	// dwmwaSystemBackdropType is DWMWA_SYSTEMBACKDROP_TYPE, an attribute only
	// Windows 11 build 22621 and later understand.
	dwmwaSystemBackdropType = 38

	// wcaAccentPolicy is WCA_ACCENT_POLICY, the SetWindowCompositionAttribute
	// attribute that selects the accent effect drawn behind the window.
	wcaAccentPolicy = 19

	// accentEnableAcrylicBlurBehind draws the acrylic material: a blurred and
	// tinted backdrop inside the window frame.
	accentEnableAcrylicBlurBehind = 4
)

// acrylicTint is the acrylic tint in AABBGGRR layout: 80% opaque #1f1f1f, the
// dark surface the application is themed for.
const acrylicTint = 0xCC1F1F1F

var (
	procDwmGetWindowAttribute         = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmGetWindowAttribute")
	procSetWindowCompositionAttribute = syscall.NewLazyDLL("user32.dll").NewProc("SetWindowCompositionAttribute")
)

// windowCompositionAttributeData mirrors WINDOWCOMPOSITIONATTRIBDATA.
type windowCompositionAttributeData struct {
	attribute  uint32
	data       unsafe.Pointer
	sizeOfData uintptr
}

// accentPolicy mirrors ACCENT_POLICY.
type accentPolicy struct {
	accentState   uint32
	accentFlags   uint32
	gradientColor uint32
	animationID   uint32
}

// setupWindowBackdrop applies the Windows backdrop the window options cannot
// express, once the window exists.
func setupWindowBackdrop(window *application.WebviewWindow) {
	window.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) {
		applyAcrylicBackdrop(window)
	})
}

// applyAcrylicBackdrop leaves the Mica material alone where it is available and
// swaps the untinted blur-behind accent for acrylic everywhere else.
func applyAcrylicBackdrop(window *application.WebviewWindow) {
	hwnd := uintptr(window.NativeWindow())
	if hwnd == 0 {
		return
	}
	if supportsSystemBackdrop(hwnd) {
		return
	}
	if err := procSetWindowCompositionAttribute.Find(); err != nil {
		log.Printf("windows: no window composition support (%v); keeping the window background as configured", err)
		return
	}
	log.Println("windows: Mica is unavailable, drawing the translucent window with the acrylic accent")
	policy := accentPolicy{
		accentState:   accentEnableAcrylicBlurBehind,
		gradientColor: acrylicTint,
	}
	data := windowCompositionAttributeData{
		attribute:  wcaAccentPolicy,
		data:       unsafe.Pointer(&policy),
		sizeOfData: unsafe.Sizeof(policy),
	}
	procSetWindowCompositionAttribute.Call(hwnd, uintptr(unsafe.Pointer(&data)))
}

// supportsSystemBackdrop reports whether the window understands
// DWMWA_SYSTEMBACKDROP_TYPE, which is where Windows 11 build 22621 and later
// keep the Mica backdrop requested through the window options.
func supportsSystemBackdrop(hwnd uintptr) bool {
	if err := procDwmGetWindowAttribute.Find(); err != nil {
		return false
	}
	var backdrop uint32
	ret, _, _ := procDwmGetWindowAttribute.Call(
		hwnd,
		dwmwaSystemBackdropType,
		uintptr(unsafe.Pointer(&backdrop)),
		unsafe.Sizeof(backdrop),
	)
	return ret == 0
}
