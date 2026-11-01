//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// The Windows accent colour lives in the registry as an ABGR dword. The
// interface one is what the Colours page of the Settings application writes;
// the DWM value is the colour window frames are painted with, and it is used
// when the first one is missing.
var accentValues = []struct{ key, name string }{
	{`Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`, "AccentColorMenu"},
	{`SOFTWARE\Microsoft\Windows\DWM`, "AccentColor"},
}

const (
	hkeyCurrentUser = 0x80000001
	rrfRtRegDword   = 0x00000010
	errorSuccess    = 0
)

var procRegGetValueW = syscall.NewLazyDLL("advapi32.dll").NewProc("RegGetValueW")

// accentWatchInterval is how often the accent colour is read to notice a
// change. Reading it here is a registry lookup inside the process, so it can be
// done often.
const accentWatchInterval = 2 * time.Second

// systemAccentColour reads the accent colour chosen for the Windows interface,
// as #rrggbb. It is empty when the registry holds no usable value.
func systemAccentColour() string {
	if err := procRegGetValueW.Find(); err != nil {
		return ""
	}
	for _, value := range accentValues {
		key, err := syscall.UTF16PtrFromString(value.key)
		if err != nil {
			continue
		}
		name, err := syscall.UTF16PtrFromString(value.name)
		if err != nil {
			continue
		}
		var accent uint32
		size := uint32(unsafe.Sizeof(accent))
		ret, _, _ := procRegGetValueW.Call(
			hkeyCurrentUser,
			uintptr(unsafe.Pointer(key)),
			uintptr(unsafe.Pointer(name)),
			rrfRtRegDword,
			0,
			uintptr(unsafe.Pointer(&accent)),
			uintptr(unsafe.Pointer(&size)),
		)
		if ret != errorSuccess {
			continue
		}
		// ABGR: red sits in the low byte.
		return fmt.Sprintf("#%02x%02x%02x", byte(accent), byte(accent>>8), byte(accent>>16))
	}
	return ""
}
