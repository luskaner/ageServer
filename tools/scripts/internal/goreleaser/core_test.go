package goreleaser

import (
	"strings"
	"testing"
)

func newSingleBinaryArchive(name string, os OperatingSystem, arch Architecture, instructionSets ...string) *Archive {
	targets := NewBinaryTargets()
	targets.AddTarget(os, arch, instructionSets...)
	a := NewArchive(name, targets, nil)
	a.AddMainBinary(NewBinary("./"+name, targets))
	a.AddSrcDstFile("README.md", "docs/README.md")
	return a
}

func TestBuildsPerArchitectureWithInstructionSets(t *testing.T) {
	a := newSingleBinaryArchive("srv", OSMacOS, ArchAmd64, "v2")
	builds := a.Builds()
	if len(builds) != 1 {
		t.Fatalf("builds = %d, want 1", len(builds))
	}
	b := builds[0]
	if b.ID != "srv_mac_amd64_v2" {
		t.Errorf("ID = %q", b.ID)
	}
	if b.Tool != "" {
		t.Errorf("macOS must not pin a tool, got %q", b.Tool)
	}
}

func TestLegacyToolPropagatesToBuild(t *testing.T) {
	old := osArgs
	osArgs = []string{"generateGoreleaserConfig", "/legacy"}
	defer func() { osArgs = old }()

	a := newSingleBinaryArchive("srv", OSWindowsLegacy, ArchAmd64)
	builds := a.Builds()
	if len(builds) != 1 {
		t.Fatalf("builds = %d", len(builds))
	}
	want := "/legacy/bin/go"
	if builds[0].Tool != want {
		t.Fatalf("tool = %q, want %q", builds[0].Tool, want)
	}
}

func TestArchivesMergesSameKeyIntoUniversal(t *testing.T) {
	targets := NewBinaryTargets()
	targets.AddTarget(OSMacOS, ArchAmd64)
	targets.AddTarget(OSMacOS, ArchArm64)
	a := NewArchive("launcher", targets, nil)
	a.AddMainBinary(NewBinary("./launcher", targets))

	// Production shape (GenerateConfig): per-OS merged builds feed Archives.
	builds := a.Builds(OSMacOS)
	if len(builds) != 1 {
		t.Fatalf("merged builds = %d, want 1 (both archs in one build)", len(builds))
	}
	archives := a.Archives(builds)

	if len(archives) != 1 {
		t.Fatalf("expected exactly one universal macOS archive, got %d (%+v)", len(archives), archives)
	}
	if !strings.HasSuffix(archives[0].NameTemplate, "_mac") {
		t.Errorf("name template = %q", archives[0].NameTemplate)
	}
	if formats := archives[0].Formats; len(formats) == 0 || formats[0] != "tar.gz" {
		t.Errorf("formats = %v", formats)
	}
}

// Without the merged-OS build list, each architecture yields its own archive.
func TestArchivesPerArchWithoutMergedOSes(t *testing.T) {
	targets := NewBinaryTargets()
	targets.AddTarget(OSMacOS, ArchAmd64)
	targets.AddTarget(OSMacOS, ArchArm64)
	a := NewArchive("launcher", targets, nil)
	a.AddMainBinary(NewBinary("./launcher", targets))

	archives := a.Archives(a.Builds())
	if len(archives) != 2 {
		t.Fatalf("archives = %d, want 2 (one per arch)", len(archives))
	}
}

func TestArchivesWindowsUsesZip(t *testing.T) {
	a := newSingleBinaryArchive("srv", OSWindowsModern, ArchAmd64, "v1")
	archives := a.Archives(a.Builds())
	if len(archives) != 1 {
		t.Fatalf("archives = %d", len(archives))
	}
	if formats := archives[0].Formats; len(formats) != 1 || formats[0] != "zip" {
		t.Fatalf("windows formats = %v, want [zip]", formats)
	}
}

func TestArchivesSkipsWhenNoMatchingBuild(t *testing.T) {
	targets := NewBinaryTargets()
	targets.AddTarget(OSLinux, ArchAmd64)
	a := NewArchive("srv", targets, nil)
	// No binaries added -> no builds -> no archives.
	if archives := a.Archives(a.Builds()); len(archives) != 0 {
		t.Fatalf("archives = %d, want 0", len(archives))
	}
}
