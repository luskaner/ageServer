package goreleaser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGenerateSuccess(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)
	// Generate should create .goreleaser.yaml
	if err := Generate(); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if _, err := os.Stat(".goreleaser.yaml"); err != nil {
		t.Fatalf("goreleaser.yaml not created: %v", err)
	}
}

func TestGenerateCurrentExcludesWindows7Builds(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	archive := NewArchive("server", Targets3264, nil)
	archive.AddMainBinary(NewBinary("./server", Targets3264))
	if err := GenerateCurrentConfig(OSWindowsModern, archive); err != nil {
		t.Fatalf("GenerateCurrentConfig failed: %v", err)
	}
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		Builds []struct {
			ID   string `yaml:"id"`
			Tool string `yaml:"tool"`
		} `yaml:"builds"`
		UniversalBinaries []struct {
			ID string `yaml:"id"`
		} `yaml:"universal_binaries"`
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}
	hasWindows10Amd64 := false
	for _, build := range project.Builds {
		if strings.Contains(build.ID, "_win7_") {
			t.Errorf("Windows 10 current config includes Windows 7 target %q", build.ID)
		}
		if build.ID == "server_win10_amd64" {
			hasWindows10Amd64 = true
		}
	}
	if !hasWindows10Amd64 {
		t.Error("current build config does not include Windows 10 amd64")
	}
	if len(project.UniversalBinaries) > 0 {
		t.Error("current build config includes universal binaries for other platforms")
	}
}

func TestGenerateCurrentLegacyWindowsBuilds(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	oldArgs := osArgs
	osArgs = []string{"generateGoreleaserConfig", `C:\legacy-go`}
	defer func() { osArgs = oldArgs }()

	archive := NewArchive("server", Targets3264, nil)
	archive.AddMainBinary(NewBinary("./server", Targets3264))
	if err := GenerateCurrentConfig(OSWindowsLegacy, archive); err != nil {
		t.Fatalf("GenerateCurrentConfig failed: %v", err)
	}
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		Builds []struct {
			ID   string `yaml:"id"`
			Main string `yaml:"main"`
			Tool string `yaml:"tool"`
		} `yaml:"builds"`
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}
	hasWindows7Amd64 := false
	hasLegacyConfigHelper := false
	for _, build := range project.Builds {
		if build.ID == "server_win7_amd64" && build.Tool == (WindowsLegacy{}).Tool() {
			hasWindows7Amd64 = true
		}
		if build.Main == "./config-helper" && build.Tool == (WindowsLegacy{}).Tool() {
			hasLegacyConfigHelper = true
		}
		if strings.Contains(build.ID, "_win10_") {
			t.Errorf("legacy Windows current config includes Windows 10 target %q", build.ID)
		}
	}
	if !hasWindows7Amd64 {
		t.Error("legacy Windows current config does not include the Windows 7 amd64 build")
	}
	if !hasLegacyConfigHelper {
		t.Error("legacy Windows current config does not include a legacy config-helper build")
	}
}

func TestWindowsReleaseTarget(t *testing.T) {
	if got := windowsReleaseTarget(6); got != OSWindowsLegacy {
		t.Errorf("Windows 6 target = %v, want Windows 7-compatible target", got)
	}
	if got := windowsReleaseTarget(10); got != OSWindowsModern {
		t.Errorf("Windows 10 target = %v, want Windows 10 target", got)
	}
}

func TestAddSrcFile(t *testing.T) {
	archive := NewArchive("test", Targets64, nil)
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	os.WriteFile(src, []byte("hello"), 0644)
	archive.AddSrcFile(src)
}

func TestAddSrcDstFileWithMode(t *testing.T) {
	archive := NewArchive("test", Targets64, nil)
	archive.AddSrcDstFileWithMode("src.txt", "dst.txt", 0644)
	// Ensure no panic
}

func TestAddScriptFilesAndConfig(t *testing.T) {
	archive := NewArchive("test", Targets64, nil)
	archive.AddScriptFiles("bin", NewTemplate[FileData]("{{.BaseOS}}/{{.Name}}"), nil, nil, false, false)
	archive.AddConfigFiles("config", NewTemplate[FileData]("config.toml"), true)
}

func TestAddAuxiliarBinary(t *testing.T) {
	archive := NewArchive("test", Targets64, nil)
	bin := NewBinary("./test", Targets64)
	archive.AddMainBinary(bin)
	aux := NewBinary("./aux", Targets64)
	archive.AddAuxiliarBinary(aux)
}

func TestCloneWithFilesPrefix(t *testing.T) {
	archive := NewArchive("test", Targets64, nil)
	archive.AddSrcDstFile("src.txt", "dst.txt")
	clone := archive.CloneWithFilesPrefix("prefix")
	if clone == nil {
		t.Fatal("clone nil")
	}
}

func TestNameAndGoarch(t *testing.T) {
	if OSWindowsModern.Name() == "" {
		t.Fatal("name empty")
	}
	if OSWindowsModern.Goos() == "" {
		t.Fatal("goos empty")
	}
	if ArchAmd64.Name() == "" {
		t.Fatal("arch name empty")
	}
	if ArchAmd64.Goarch() == "" {
		t.Fatal("goarch empty")
	}
}

func TestGenerateConfig(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(dir)
	archive := NewArchive("test", Targets64, nil)
	archive.AddMainBinary(NewBinary("./test", Targets64))
	if err := GenerateConfig(archive); err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}
	if _, err := os.Stat(".goreleaser.yaml"); err != nil {
		t.Fatalf("not created: %v", err)
	}
}

// TestGenerateConfigIncludesAllPlatforms guards against the full configuration
// being filtered by the host platform: GenerateConfig must emit every supported
// target no matter where it runs (the release pipeline generates it on Linux).
func TestGenerateConfigIncludesAllPlatforms(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	archive := NewArchive("server", Targets3264, nil)
	archive.AddMainBinary(NewBinary("./server", Targets3264))
	want := []string{
		"server_win7_amd64",
		"server_win10_amd64",
		"server_win10_arm64",
		"server_linux_amd64",
		"server_linux_arm64",
		"server_mac",
	}
	if err := GenerateConfig(archive); err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		Builds []struct {
			ID string `yaml:"id"`
		} `yaml:"builds"`
		UniversalBinaries []struct {
			ID string `yaml:"id"`
		} `yaml:"universal_binaries"`
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}

	found := make(map[string]bool, len(project.Builds))
	for _, build := range project.Builds {
		found[build.ID] = true
	}
	for _, id := range want {
		if !found[id] {
			t.Errorf("full config is missing build %q (builds: %v)", id, found)
		}
	}
	if len(project.UniversalBinaries) == 0 {
		t.Error("full config should still declare the macOS universal binaries")
	}
}

// TestGenerateConfigUniversalMacOS guards the macOS packaging: a release ships
// one universal archive per package (ageServer_server_2.0.0_mac.tar.gz) instead
// of one archive per architecture.
func TestGenerateConfigUniversalMacOS(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	macTargets := NewBinaryTargets()
	macTargets.AddTarget(OSMacOS, ArchAmd64)
	macTargets.AddTarget(OSMacOS, ArchArm64)
	archive := NewArchive("server", macTargets, nil)
	archive.AddMainBinary(NewBinary("./server", macTargets))
	if err := GenerateConfig(archive); err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		Builds []struct {
			ID     string   `yaml:"id"`
			Goos   []string `yaml:"goos"`
			Goarch []string `yaml:"goarch"`
		} `yaml:"builds"`
		Archives []struct {
			ID           string   `yaml:"id"`
			IDs          []string `yaml:"ids"`
			NameTemplate string   `yaml:"name_template"`
			Builds       []string `yaml:"builds"`
		} `yaml:"archives"`
		UniversalBinaries []struct {
			ID string `yaml:"id"`
		} `yaml:"universal_binaries"`
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}

	var macBuilds []string
	for _, build := range project.Builds {
		if !strings.Contains(build.ID, "_mac") {
			continue
		}
		macBuilds = append(macBuilds, build.ID)
		if build.ID != "server_mac" {
			t.Errorf("macOS build %q is not the merged universal build", build.ID)
			continue
		}
		if strings.Join(build.Goos, ",") != "darwin" {
			t.Errorf("merged macOS build goos = %v, want [darwin]", build.Goos)
		}
		if strings.Join(build.Goarch, ",") != "amd64,arm64" {
			t.Errorf("merged macOS build goarch = %v, want [amd64 arm64]", build.Goarch)
		}
	}
	if strings.Join(macBuilds, ",") != "server_mac" {
		t.Errorf("macOS builds = %v, want [server_mac]", macBuilds)
	}

	if len(project.Archives) != 1 {
		t.Fatalf("archives = %d, want 1 universal macOS archive", len(project.Archives))
	}
	gotArchive := project.Archives[0]
	if gotArchive.ID != "server_mac" {
		t.Errorf("archive id = %q, want %q", gotArchive.ID, "server_mac")
	}
	wantName := `{{ .ProjectName }}_server_{{ .RawVersion }}_mac`
	if gotArchive.NameTemplate != wantName {
		t.Errorf("archive name_template = %q, want %q", gotArchive.NameTemplate, wantName)
	}
	if strings.Join(gotArchive.Builds, ",") != "" {
		t.Errorf("archive builds = %v, want none (universal archives use ids)", gotArchive.Builds)
	}
	archiveIDs := make(map[string]bool, len(gotArchive.IDs))
	for _, id := range gotArchive.IDs {
		archiveIDs[id] = true
	}
	if !archiveIDs["server_mac"] || len(archiveIDs) != 1 {
		t.Errorf("archive ids = %v, want exactly server_mac", gotArchive.IDs)
	}

	if len(project.UniversalBinaries) != 1 || project.UniversalBinaries[0].ID != "server_mac" {
		t.Errorf("universal_binaries = %+v, want a single server_mac entry", project.UniversalBinaries)
	}
}

// TestGenerateConfigReplacesDraft guards release idempotency: re-running the
// workflow for a tag must not reuse a draft left behind by a previous run,
// otherwise GitHub answers 422 already_exists for every asset.
func TestGenerateConfigReplacesDraft(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	archive := NewArchive("server", Targets3264, nil)
	archive.AddMainBinary(NewBinary("./server", Targets3264))
	if err := GenerateConfig(archive); err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		Release struct {
			Draft                    bool `yaml:"draft"`
			ReplaceExistingDraft     bool `yaml:"replace_existing_draft"`
			UseExistingDraft         bool `yaml:"use_existing_draft"`
			ReplaceExistingArtifacts bool `yaml:"replace_existing_artifacts"`
		} `yaml:"release"`
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}

	release := project.Release
	if !release.Draft {
		t.Error("release.draft = false, want true")
	}
	if !release.ReplaceExistingDraft {
		t.Error("release.replace_existing_draft = false, want true")
	}
	if release.UseExistingDraft {
		t.Error("release.use_existing_draft = true, want false")
	}
	if !release.ReplaceExistingArtifacts {
		t.Error("release.replace_existing_artifacts = false, want true")
	}
}

func TestOverrideWindowsName(t *testing.T) {
	name := overrideWindowsName("full", OSWindowsModern, ArchArm64)
	if name != "win11" {
		t.Fatalf("got %q want win11", name)
	}
	name2 := overrideWindowsName("other", OSWindowsModern, ArchAmd64)
	if name2 != OSWindowsModern.Name() {
		t.Fatalf("got %q", name2)
	}
}

func TestGenerateCurrentLinux(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	archive := NewArchive("server", Targets3264, nil)
	archive.AddMainBinary(NewBinary("./server", Targets3264))
	if err := GenerateCurrentConfig(OSLinux, archive); err != nil {
		t.Fatalf("GenerateCurrentConfig failed: %v", err)
	}
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		Builds []struct {
			ID string `yaml:"id"`
		} `yaml:"builds"`
		UniversalBinaries []struct {
			ID string `yaml:"id"`
		} `yaml:"universal_binaries"`
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}

	// Should only have builds for current Linux arch (amd64 in test env)
	hasLinuxAmd64 := false
	for _, build := range project.Builds {
		if strings.Contains(build.ID, "_linux_") && !strings.Contains(build.ID, "_linux_amd64") {
			t.Errorf("Linux current config includes non-amd64 target %q", build.ID)
		}
		if build.ID == "server_linux_amd64" {
			hasLinuxAmd64 = true
		}
	}
	if !hasLinuxAmd64 {
		t.Error("current build config does not include Linux amd64")
	}
	if len(project.UniversalBinaries) > 0 {
		t.Error("current build config includes universal binaries for other platforms")
	}
}

func TestGenerateCurrentMacOS(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	archive := NewArchive("server", Targets3264, nil)
	archive.AddMainBinary(NewBinary("./server", Targets3264))
	if err := GenerateCurrentConfig(OSMacOS, archive); err != nil {
		t.Fatalf("GenerateCurrentConfig failed: %v", err)
	}
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		Builds []struct {
			ID string `yaml:"id"`
		} `yaml:"builds"`
		UniversalBinaries []struct {
			ID string `yaml:"id"`
		} `yaml:"universal_binaries"`
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}

	// Should only have builds for current macOS arch (amd64 in test env)
	hasMacOSAmd64 := false
	for _, build := range project.Builds {
		if strings.Contains(build.ID, "_mac_") && !strings.Contains(build.ID, "_mac_amd64") {
			t.Errorf("macOS current config includes non-amd64 target %q", build.ID)
		}
		if build.ID == "server_mac_amd64" {
			hasMacOSAmd64 = true
		}
	}
	if !hasMacOSAmd64 {
		t.Error("current build config does not include macOS amd64")
	}
	if len(project.UniversalBinaries) > 0 {
		t.Error("current build config includes universal binaries for other platforms")
	}
}

func TestGenerateCurrentWindowsArm64(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	archive := NewArchive("server", Targets3264, nil)
	archive.AddMainBinary(NewBinary("./server", Targets3264))
	if err := GenerateCurrentConfig(OSWindowsModern, archive); err != nil {
		t.Fatalf("GenerateCurrentConfig failed: %v", err)
	}
	data, err := os.ReadFile(".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var project struct {
		Builds []struct {
			ID string `yaml:"id"`
		} `yaml:"builds"`
		UniversalBinaries []struct {
			ID string `yaml:"id"`
		} `yaml:"universal_binaries"`
	}
	if err := yaml.Unmarshal(data, &project); err != nil {
		t.Fatal(err)
	}

	// Windows Modern supports amd64 and arm64, but current should only build one
	// The test runs on amd64, so it should only have amd64
	hasWindows10Amd64 := false
	for _, build := range project.Builds {
		if strings.Contains(build.ID, "_win10_") && !strings.Contains(build.ID, "_win10_amd64") {
			t.Errorf("Windows 10 current config includes non-amd64 target %q", build.ID)
		}
		if build.ID == "server_win10_amd64" {
			hasWindows10Amd64 = true
		}
	}
	if !hasWindows10Amd64 {
		t.Error("current build config does not include Windows 10 amd64")
	}
	if len(project.UniversalBinaries) > 0 {
		t.Error("current build config includes universal binaries for other platforms")
	}
}
