package goreleaser

import "runtime"

func windowsReleaseTarget(majorVersion uint32) OperatingSystem {
	if majorVersion < 10 {
		return OSWindowsLegacy
	}
	return OSWindowsModern
}

func currentOS() OperatingSystem {
	switch runtime.GOOS {
	case "linux":
		return OSLinux
	case "darwin":
		return OSMacOS
	case "windows":
		return nil // handled by host_windows.go
	}
	return nil
}

func currentArch() Architecture {
	switch runtime.GOARCH {
	case "386":
		return Arch386
	case "amd64":
		return ArchAmd64
	case "arm":
		return ArchArm32
	case "arm64":
		return ArchArm64
	}
	return nil
}
