//go:build !windows

package goreleaser

func currentWindowsOS() (OperatingSystem, error) {
	return currentOS(), nil
}

func currentWindowsArch() Architecture {
	return currentArch()
}
