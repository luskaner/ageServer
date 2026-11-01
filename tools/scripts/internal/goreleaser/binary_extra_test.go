package goreleaser

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/goreleaser/goreleaser/v2/pkg/config"
)

func TestBinaryMain(t *testing.T) {
	b := NewBinary("./launcher", NewBinaryTargets())
	if got := b.Main(); got != "./launcher" {
		t.Fatalf("Main() = %q, want %q", got, "./launcher")
	}
}

func TestBinaryTargetsCloneForOperatingSystems(t *testing.T) {
	bt := NewBinaryTargets()
	bt.AddTarget(OSLinux, ArchAmd64)
	bt.AddTarget(OSMacOS, ArchArm64)
	bt.AddTarget(OSWindowsModern, ArchAmd64)

	osSet := mapset.NewSet[OperatingSystem](OSLinux, OSMacOS)
	clone := bt.CloneForOperatingSystems(osSet)

	if _, ok := (*clone)[OSLinux]; !ok {
		t.Error("linux missing from clone")
	}
	if _, ok := (*clone)[OSMacOS]; !ok {
		t.Error("macOS missing from clone")
	}
	if _, ok := (*clone)[OSWindowsModern]; ok {
		t.Error("windows should not be in clone")
	}
}

func TestBinaryTargetsAddMultipleTargets(t *testing.T) {
	bt1 := NewBinaryTargets()
	bt1.AddTarget(OSLinux, ArchAmd64)
	bt2 := NewBinaryTargets()
	bt2.AddTarget(OSLinux, ArchArm64)

	combined := NewBinaryTargets()
	combined.AddMultipleTargets(bt1, bt2)

	archs := (*combined)[OSLinux]
	if _, ok := archs[ArchAmd64]; !ok {
		t.Error("amd64 missing")
	}
	if _, ok := archs[ArchArm64]; !ok {
		t.Error("arm64 missing")
	}
}

func TestBinaryCloneForOperatingSystems(t *testing.T) {
	bt := NewBinaryTargets()
	bt.AddTarget(OSLinux, ArchAmd64)
	bt.AddTarget(OSMacOS, ArchArm64)

	b := NewBinary("./srv", bt)
	osSet := mapset.NewSet[OperatingSystem](OSLinux)
	clone := b.CloneForOperatingSystems(osSet)

	if clone.Main() != "./srv" {
		t.Errorf("Main() = %q", clone.Main())
	}
	if _, ok := (*clone.targets)[OSLinux]; !ok {
		t.Error("linux missing from cloned binary")
	}
	if _, ok := (*clone.targets)[OSMacOS]; ok {
		t.Error("macOS should not be in cloned binary")
	}
}

func TestBinaryTargetsClonePreservesContent(t *testing.T) {
	bt := NewBinaryTargets()
	bt.AddTarget(OSLinux, ArchAmd64, "v2")
	clone := bt.Clone()
	// Modifying clone must not affect original
	(*clone).AddTarget(OSLinux, ArchArm64)
	if _, ok := (*bt)[OSLinux][ArchArm64]; ok {
		t.Fatal("clone modified original")
	}
}

func TestUniversalBinaries(t *testing.T) {
	builds := []config.Build{
		{ID: "srv_linux_amd64", Goos: []string{"linux"}, Binary: "srv"},
		{ID: "srv_darwin_amd64", Goos: []string{"darwin"}, Binary: "srv"},
		{ID: "srv_windows_amd64", Goos: []string{"windows"}, Binary: "srv"},
	}
	ub := universalBinaries(builds)
	if len(ub) != 1 {
		t.Fatalf("universalBinaries = %d, want 1", len(ub))
	}
	if ub[0].ID != "srv_darwin_amd64" {
		t.Errorf("ID = %q, want %q", ub[0].ID, "srv_darwin_amd64")
	}
	if !ub[0].Replace {
		t.Error("Replace must be true")
	}
}

func TestUniversalBinariesNoneDarwin(t *testing.T) {
	builds := []config.Build{
		{ID: "srv_linux_amd64", Goos: []string{"linux"}},
	}
	ub := universalBinaries(builds)
	if len(ub) != 0 {
		t.Fatalf("universalBinaries = %d, want 0", len(ub))
	}
}
