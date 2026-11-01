package goreleaser

import (
	"testing"

	"github.com/goreleaser/goreleaser/v2/pkg/config"
)

func TestExtChangeWithoutExt(t *testing.T) {
	fn := extChange("")
	got := fn("src/main.go").Render(FileData{})
	if got != "src/main" {
		t.Fatalf("extChange(\"\") = %q, want %q", got, "src/main")
	}
}

func TestExtChangeWithExt(t *testing.T) {
	fn := extChange("command")
	got := fn("start.sh").Render(FileData{})
	if got != "start.command" {
		t.Fatalf("extChange(\"command\") = %q, want %q", got, "start.command")
	}
}

func TestBuildNameNoArchNoIS(t *testing.T) {
	got := buildName("launcher", OSMacOS, nil, "")
	want := "launcher_mac"
	if got != want {
		t.Fatalf("buildName = %q, want %q", got, want)
	}
}

func TestBuildNameWithArch(t *testing.T) {
	arch := ArchAmd64
	got := buildName("launcher", OSWindowsModern, &arch, "")
	want := "launcher_win10_amd64"
	if got != want {
		t.Fatalf("buildName = %q, want %q", got, want)
	}
}

func TestBuildNameWithArchAndIS(t *testing.T) {
	arch := ArchAmd64
	got := buildName("srv", OSMacOS, &arch, "v2")
	want := "srv_mac_amd64_v2"
	if got != want {
		t.Fatalf("buildName = %q, want %q", got, want)
	}
}

func TestArchToValues(t *testing.T) {
	tests := []struct {
		arch Architecture
		b    config.Build
		want int
	}{
		{Arch386, config.Build{Go386: []string{"softfloat"}}, 1},
		{ArchAmd64, config.Build{Goamd64: []string{"v1", "v2"}}, 2},
		{ArchArm32, config.Build{Goarm: []string{"7"}}, 1},
		{ArchArm64, config.Build{Goarm64: []string{"v8.0"}}, 1},
		{ArchAmd64, config.Build{}, 0},
	}
	for _, tt := range tests {
		got := archToValues(tt.arch, tt.b)
		if got.Cardinality() != tt.want {
			t.Errorf("archToValues(%v) cardinality = %d, want %d (set: %v)", tt.arch, got.Cardinality(), tt.want, got)
		}
	}
}

func TestKeyFromStringsSorted(t *testing.T) {
	got := keyFromStrings([]string{"c", "a", "b"})
	want := "a,b,c"
	if got != want {
		t.Fatalf("keyFromStrings = %q, want %q", got, want)
	}
}

func TestKeyFromStringsEmpty(t *testing.T) {
	got := keyFromStrings([]string{})
	if got != "" {
		t.Fatalf("keyFromStrings(empty) = %q, want empty", got)
	}
}

func TestArchiveToUniversal(t *testing.T) {
	a := config.Archive{
		ID:           "launcher_mac_amd64_arm64",
		IDs:          []string{"id1"},
		NameTemplate: "proj_launcher_1.0_mac_x86-64",
		Formats:      []string{"tar.gz"},
	}
	got := archiveToUniversal(a)
	if got.ID != "launcher_mac" {
		t.Errorf("ID = %q, want %q", got.ID, "launcher_mac")
	}
	wantTemplate := "{{ .ProjectName }}_launcher_{{ .RawVersion }}_mac"
	if got.NameTemplate != wantTemplate {
		t.Errorf("NameTemplate = %q, want %q", got.NameTemplate, wantTemplate)
	}
	if len(got.IDs) != 1 || got.IDs[0] != "id1" {
		t.Errorf("IDs = %v, want [id1]", got.IDs)
	}
	if len(got.Formats) != 1 || got.Formats[0] != "tar.gz" {
		t.Errorf("Formats = %v, want [tar.gz]", got.Formats)
	}
}

func TestOverrideWindowsNameDefault(t *testing.T) {
	got := overrideWindowsName("full", OSWindowsModern, ArchAmd64)
	if got != "win10" {
		t.Fatalf("overrideWindowsName = %q, want %q", got, "win10")
	}
}

func TestOverrideWindowsNameArm64(t *testing.T) {
	got := overrideWindowsName("full", OSWindowsModern, ArchArm64)
	if got != "win11" {
		t.Fatalf("overrideWindowsName = %q, want %q", got, "win11")
	}
}

func TestOverrideWindowsNameNonMatchingName(t *testing.T) {
	got := overrideWindowsName("server", OSWindowsModern, ArchArm64)
	if got != "win10" {
		t.Fatalf("overrideWindowsName = %q, want %q", got, "win10")
	}
}

func TestMergeBuildsSingleBuild(t *testing.T) {
	b := mergeBuilds("./srv", "./srv", OSMacOS,
		config.Build{Goarch: []string{"amd64"}, Goamd64: []string{"v2"}},
	)
	if b.ID != "./srv_mac" {
		t.Errorf("ID = %q", b.ID)
	}
	if len(b.Goarch) != 1 || b.Goarch[0] != "amd64" {
		t.Errorf("Goarch = %v", b.Goarch)
	}
	if len(b.Goamd64) != 1 || b.Goamd64[0] != "v2" {
		t.Errorf("Goamd64 = %v", b.Goamd64)
	}
}

func TestMergeBuildsMultipleBuilds(t *testing.T) {
	b := mergeBuilds("./srv", "./srv", OSLinux,
		config.Build{Goarch: []string{"amd64"}, Goamd64: []string{"v1"}},
		config.Build{Goarch: []string{"arm64"}, Goarm64: []string{"v8.0"}},
	)
	if len(b.Goarch) != 2 {
		t.Errorf("Goarch = %v, want 2 entries", b.Goarch)
	}
	if len(b.Goamd64) != 1 || len(b.Goarm64) != 1 {
		t.Errorf("Goamd64 = %v, Goarm64 = %v", b.Goamd64, b.Goarm64)
	}
}

func TestMergeBuildsLegacyTool(t *testing.T) {
	b := mergeBuilds("./srv", "./srv", OSWindowsLegacy,
		config.Build{Goarch: []string{"amd64"}},
	)
	if b.Tool == "" {
		t.Error("legacy tool must propagate")
	}
}
