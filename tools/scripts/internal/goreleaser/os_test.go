package goreleaser

import (
	"path/filepath"
	"testing"
)

// Regression: WindowsLegacy.Tool indexed os.Args[1] unconditionally, panicking
// whenever the generator ran without a positional argument.
func TestWindowsLegacyToolWithoutArgumentIsEmpty(t *testing.T) {
	old := osArgs
	osArgs = []string{"generateGoreleaserConfig"}
	defer func() { osArgs = old }()

	if got := (WindowsLegacy{}).Tool(); got != "" {
		t.Fatalf("Tool without positional arg = %q, want empty", got)
	}
}

func TestWindowsLegacyToolJoinsGoroot(t *testing.T) {
	old := osArgs
	osArgs = []string{"generateGoreleaserConfig", "/opt/go-legacy-win7"}
	defer func() { osArgs = old }()

	want := filepath.ToSlash(filepath.Join("/opt/go-legacy-win7", "bin", "go"))
	if got := (WindowsLegacy{}).Tool(); got != want {
		t.Fatalf("Tool = %q, want %q", got, want)
	}
}

func TestModernAndDefaultToolsAreEmpty(t *testing.T) {
	if got := (WindowsModern{}).Tool(); got != "" {
		t.Errorf("WindowsModern tool = %q", got)
	}
	if got := (Linux{}).Tool(); got != "" {
		t.Errorf("Linux tool = %q", got)
	}
	if got := (MacOS{}).Tool(); got != "" {
		t.Errorf("MacOS tool = %q", got)
	}
}

func TestArchitectureMetadata(t *testing.T) {
	if got := ArchAmd64.Goarch(); got != "amd64" {
		t.Errorf("amd64 goarch = %q", got)
	}
	if got := Arch386.Name(); got != "x86-32" {
		t.Errorf("386 name = %q", got)
	}
	if set := ArchArm64.InstructionSet(); !set.Contains("v8.0") || !set.Contains("v9.5,lse,crypto") {
		t.Errorf("arm64 instruction sets incomplete: %v", set)
	}
	if set := ArchArm32.InstructionSet(); !set.Contains("7") {
		t.Errorf("arm32 instruction sets incomplete: %v", set)
	}
}

func TestOperatingSystemNamesAndArchs(t *testing.T) {
	if got := OSWindowsLegacy.Name(); got != "win7" {
		t.Errorf("win7 name = %q", got)
	}
	if got := OSWindowsModern.Goos(); got != "windows" {
		t.Errorf("windows goos = %q", got)
	}
	if archs := OSMacOS.Archs(); !archs.Contains(ArchAmd64) || !archs.Contains(ArchArm64) || archs.Contains(Arch386) {
		t.Errorf("macOS archs wrong: %v", archs)
	}
	if archs := OSLinux.Archs(); !archs.Contains(ArchArm32) {
		t.Errorf("linux must include arm32: %v", archs)
	}
}
