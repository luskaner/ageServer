//go:build windows

package goreleaser

import (
	"fmt"
	"syscall"
	"unsafe"
)

type osVersionInfoExW struct {
	size             uint32
	majorVersion     uint32
	minorVersion     uint32
	buildNumber      uint32
	platformID       uint32
	servicePack      [128]uint16
	servicePackMajor uint16
	servicePackMinor uint16
	suiteMask        uint16
	productType      byte
	reserved         byte
}

func currentWindowsOS() (OperatingSystem, error) {
	info := osVersionInfoExW{size: uint32(unsafe.Sizeof(osVersionInfoExW{}))}
	rtlGetVersion := syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion")
	status, _, _ := rtlGetVersion.Call(uintptr(unsafe.Pointer(&info)))
	if int32(status) != 0 {
		return nil, fmt.Errorf("RtlGetVersion failed with NTSTATUS %#x", uint32(status))
	}
	target := windowsReleaseTarget(info.majorVersion)
	if target == OSWindowsLegacy && (len(osArgs) < 2 || osArgs[1] == "") {
		return nil, fmt.Errorf("GOROOT_LEGACY must point to a Windows 7-compatible Go installation when building on Windows versions older than 10")
	}
	return target, nil
}

func currentWindowsArch() Architecture {
	return currentArch()
}
