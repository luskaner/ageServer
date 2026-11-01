package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/common/game"
)

func TestAppIdAoE1(t *testing.T) {
	if got := appId(game.AoE1); got != "1017900" {
		t.Errorf("appId(AoE1) = %q, want %q", got, "1017900")
	}
}

func TestAppIdAoE2(t *testing.T) {
	if got := appId(game.AoE2); got != "813780" {
		t.Errorf("appId(AoE2) = %q, want %q", got, "813780")
	}
}

func TestAppIdAoE3(t *testing.T) {
	if got := appId(game.AoE3); got != "933110" {
		t.Errorf("appId(AoE3) = %q, want %q", got, "933110")
	}
}

func TestAppIdAoE4(t *testing.T) {
	if got := appId(game.AoE4); got != "1466860" {
		t.Errorf("appId(AoE4) = %q, want %q", got, "1466860")
	}
}

func TestAppIdAoM(t *testing.T) {
	if got := appId(game.AoM); got != "1934680" {
		t.Errorf("appId(AoM) = %q, want %q", got, "1934680")
	}
}

func TestAppIdUnknown(t *testing.T) {
	if got := appId("unknown"); got != "" {
		t.Errorf("appId(unknown) = %q, want empty", got)
	}
}

func TestGameOpenUri(t *testing.T) {
	g := &Game{appId: "813780", libraryPath: "/some/path"}
	want := "steam://rungameid/813780"
	if got := g.OpenUri(); got != want {
		t.Errorf("OpenUri() = %q, want %q", got, want)
	}
}

func TestGamePath(t *testing.T) {
	g := &Game{appId: "813780", libraryPath: "/some/path"}
	if got := g.Path(); got != "" {
		t.Errorf("Path() = %q, want empty (no real steam dir)", got)
	}
}

func TestLibraryFolderEmptyConfigPath(t *testing.T) {
	folder := libraryFolder("813780", func() string { return "" }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder with empty config = %q, want empty", folder)
	}
}

func TestLibraryFolderNonexistentConfig(t *testing.T) {
	folder := libraryFolder("813780", func() string { return "nonexistent" }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder with nonexistent config = %q, want empty", folder)
	}
}

func TestLibraryFolderWithValidVDF(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"C:\\SteamLibrary"
		"apps"
		{
			"813780"		"813780"
		}
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return dir }, func() string { return "" })
	if folder != "C:\\SteamLibrary" {
		t.Errorf("libraryFolder = %q, want %q", folder, "C:\\SteamLibrary")
	}
}

func TestLibraryFolderAppNotFound(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"C:\\SteamLibrary"
		"apps"
		{
			"999999"		"999999"
		}
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return dir }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder = %q, want empty (app not in library)", folder)
	}
}

func TestLibraryFolderFallbackToAlt(t *testing.T) {
	// Primary config has a dir but openLibraryFolder fails; alt has the VDF
	primaryDir := t.TempDir() // exists but no config/libraryfolders.vdf
	altDir := t.TempDir()
	configDir := filepath.Join(altDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"D:\\AltSteam"
		"apps"
		{
			"813780"		"813780"
		}
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return primaryDir }, func() string { return altDir })
	if folder != "D:\\AltSteam" {
		t.Errorf("libraryFolder = %q, want %q", folder, "D:\\AltSteam")
	}
}

func TestLibraryFolderBothEmpty(t *testing.T) {
	folder := libraryFolder("813780", func() string { return "" }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder = %q, want empty", folder)
	}
}

func TestLibraryFolderInvalidVDF(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte("invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return dir }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder = %q, want empty (invalid VDF)", folder)
	}
}

func TestLibraryFolderNoLibraryFoldersKey(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"something"
	{
		"nothelib"		{}
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return dir }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder = %q, want empty (no libraryfolders key)", folder)
	}
}

func TestLibraryFolderNoAppsKey(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"C:\\SteamLibrary"
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return dir }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder = %q, want empty (no apps key)", folder)
	}
}

func TestNewCustomGameNoConfigPath(t *testing.T) {
	g, ok := NewCustomGame(game.AoE2, func() string { return "" }, func() string { return "" }, func(s string) string { return s })
	if ok {
		t.Error("NewCustomGame should return ok=false with empty config")
	}
	if g != nil {
		t.Error("game should be nil")
	}
}

func TestNewCustomGameWithValidVDF(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"C:\SteamLibrary"
		"apps"
		{
			"813780"		"813780"
		}
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	g, ok := NewCustomGame(game.AoE2, func() string { return dir }, func() string { return "" }, func(s string) string { return s })
	if !ok {
		t.Fatal("NewCustomGame should succeed")
	}
	if g == nil {
		t.Fatal("game should not be nil")
	}
	if g.appId != "813780" {
		t.Errorf("appId = %q, want %q", g.appId, "813780")
	}
}

func TestNewCustomGamePathTransform(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"/unix/steam"
		"apps"
		{
			"813780"		"813780"
		}
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	transform := func(s string) string { return strings.Replace(s, "/unix", "Z:\\unix", 1) }
	g, ok := NewCustomGame(game.AoE2, func() string { return dir }, func() string { return "" }, transform)
	if !ok {
		t.Fatal("NewCustomGame should succeed")
	}
	if g.libraryPath != "Z:\\unix/steam" {
		t.Errorf("libraryPath = %q, want %q", g.libraryPath, "Z:\\unix/steam")
	}
}

func TestNewCustomGamePathTransformEmpty(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"/unix/steam"
		"apps"
		{
			"813780"		"813780"
		}
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	transform := func(s string) string { return "" }
	g, ok := NewCustomGame(game.AoE2, func() string { return dir }, func() string { return "" }, transform)
	if ok {
		t.Error("NewCustomGame should fail when transform returns empty")
	}
	if g != nil {
		t.Error("game should be nil")
	}
}

func TestOpenLibraryFolderNonexistent(t *testing.T) {
	f, err := openLibraryFolder("nonexistent")
	if err == nil {
		f.Close()
		t.Error("expected error for nonexistent path")
	}
}

func TestGamePathNonexistent(t *testing.T) {
	g := &Game{appId: "813780", libraryPath: "nonexistent"}
	if got := g.Path(); got != "" {
		t.Errorf("Path() = %q, want empty", got)
	}
}

func TestGamePathWithValidManifest(t *testing.T) {
	dir := t.TempDir()
	steamappsDir := filepath.Join(dir, "steamapps")
	commonDir := filepath.Join(steamappsDir, "common")
	gameDir := filepath.Join(commonDir, "Age of Empires II DE")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestContent := `"AppState"
{
	"appid"		"813780"
	"Universe"		"1"
	"installdir"		"Age of Empires II DE"
}`
	if err := os.WriteFile(filepath.Join(steamappsDir, "appmanifest_813780.acf"), []byte(manifestContent), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &Game{appId: "813780", libraryPath: dir}
	folder := g.Path()
	if folder != gameDir {
		t.Errorf("Path() = %q, want %q", folder, gameDir)
	}
}

func TestGamePathWithInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	steamappsDir := filepath.Join(dir, "steamapps")
	if err := os.MkdirAll(steamappsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(steamappsDir, "appmanifest_813780.acf"), []byte("invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &Game{appId: "813780", libraryPath: dir}
	if got := g.Path(); got != "" {
		t.Errorf("Path() = %q, want empty (invalid manifest)", got)
	}
}

func TestGamePathWithNonDirInstallDir(t *testing.T) {
	dir := t.TempDir()
	steamappsDir := filepath.Join(dir, "steamapps")
	commonDir := filepath.Join(steamappsDir, "common")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestContent := `"AppState"
{
	"appid"		"813780"
	"Universe"		"1"
	"installdir"		"MissingGame"
}`
	if err := os.WriteFile(filepath.Join(steamappsDir, "appmanifest_813780.acf"), []byte(manifestContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// MissingGame dir doesn't exist
	g := &Game{appId: "813780", libraryPath: dir}
	if got := g.Path(); got != "" {
		t.Errorf("Path() = %q, want empty (installdir doesn't exist)", got)
	}
}

func TestGamePathNonexistentManifest(t *testing.T) {
	dir := t.TempDir()
	steamappsDir := filepath.Join(dir, "steamapps")
	if err := os.MkdirAll(steamappsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	g := &Game{appId: "813780", libraryPath: dir}
	if got := g.Path(); got != "" {
		t.Errorf("Path() = %q, want empty (no manifest file)", got)
	}
}

func TestLibraryFolderMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"C:\\SteamLibrary1"
		"apps"
		{
			"999999"		"999999"
		}
	}
	"1"
	{
		"path"		"C:\\SteamLibrary2"
		"apps"
		{
			"813780"		"813780"
		}
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return dir }, func() string { return "" })
	if folder != "C:\\SteamLibrary2" {
		t.Errorf("libraryFolder = %q, want %q", folder, "C:\\SteamLibrary2")
	}
}

func TestLibraryFolderEntryWithoutApps(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"C:\\SteamLibrary"
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return dir }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder = %q, want empty (no apps key)", folder)
	}
}

func TestLibraryFolderNonMapEntry(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"		"not-a-map"
}"`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return dir }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder = %q, want empty (non-map entry)", folder)
	}
}

func TestConfigPathFn(t *testing.T) {
	// ConfigPathFn is just a type, verify it can be used
	var fn ConfigPathFn = func() string { return "test" }
	if fn() != "test" {
		t.Error("ConfigPathFn should return test")
	}
}

func TestPathTranslateFn(t *testing.T) {
	var fn PathTranslateFn = func(s string) string { return "translated" }
	if fn("input") != "translated" {
		t.Error("PathTranslateFn should return translated")
	}
}

func TestLibraryFolderPathNotString(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"
		{
			"nested"		"value"
		}
		"apps"
		{
			"813780"		"813780"
		}
	}
}`
	if err := os.WriteFile(filepath.Join(configDir, "libraryfolders.vdf"), []byte(vdfContent), 0o644); err != nil {
		t.Fatal(err)
	}
	folder := libraryFolder("813780", func() string { return dir }, func() string { return "" })
	if folder != "" {
		t.Errorf("libraryFolder with non-string path = %q, want empty", folder)
	}
}

func TestGamePathAppStateNotMap(t *testing.T) {
	dir := t.TempDir()
	steamappsDir := filepath.Join(dir, "steamapps")
	if err := os.MkdirAll(steamappsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	invalidContent := `"AppState"		"not-a-map"`
	if err := os.WriteFile(filepath.Join(steamappsDir, "appmanifest_813780.acf"), []byte(invalidContent), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &Game{appId: "813780", libraryPath: dir}
	if got := g.Path(); got != "" {
		t.Errorf("Path with AppState not map = %q, want empty", got)
	}
}

func TestGamePathInstallDirNotString(t *testing.T) {
	dir := t.TempDir()
	steamappsDir := filepath.Join(dir, "steamapps")
	if err := os.MkdirAll(steamappsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifestContent := `"AppState"
{
	"appid"		"813780"
	"installdir"		12345
}`
	if err := os.WriteFile(filepath.Join(steamappsDir, "appmanifest_813780.acf"), []byte(manifestContent), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &Game{appId: "813780", libraryPath: dir}
	if got := g.Path(); got != "" {
		t.Errorf("Path with installdir not string = %q, want empty", got)
	}
	_ = manifestContent
}
