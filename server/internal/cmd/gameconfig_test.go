package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common/cmd/server"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/spf13/pflag"
)

func withGameConfigDir(t *testing.T, dir string) {
	t.Helper()
	old := configPaths
	configPaths = []string{dir}
	t.Cleanup(func() { configPaths = old })
}

func writeGameFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func legacyBase() internal.Game {
	return internal.Game{
		BattleServers: []internal.BattleServer{{
			Region: "legacy", Name: "Legacy", IPv4: "192.168.1.5", BsPort: 27012, WebSocketPort: 27013,
		}},
	}
}

func TestLoadGameConfigWithoutFileKeepsLegacyBattleServers(t *testing.T) {
	dir := t.TempDir()
	withGameConfigDir(t, dir)
	cfg, found, err := loadGameConfig(legacyBase(), "age2", "", &server.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("no per-game file was written")
	}
	if len(cfg.BattleServers) != 1 || cfg.BattleServers[0].Region != "legacy" {
		t.Fatalf("legacy battle servers lost: %+v", cfg.BattleServers)
	}
	if cfg.EmbeddedBattleServer.StartIfMissing != "true" {
		t.Fatalf("StartIfMissing = %q, want true", cfg.EmbeddedBattleServer.StartIfMissing)
	}
}

func TestLoadGameConfigFileOverridesBattleServers(t *testing.T) {
	dir := t.TempDir()
	withGameConfigDir(t, dir)
	writeGameFile(t, dir, "config.age2.toml", `
[Game.EmbeddedBattleServer]
StartIfMissing = 'required'
Region = 'embedded'

[[Game.BattleServers]]
Region = 'file'
Name = 'From file'
IPv4 = '192.168.1.6'
BsPort = 27014
WebSocketPort = 27015
`)
	cfg, found, err := loadGameConfig(legacyBase(), "age2", "", &server.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("per-game file was not loaded")
	}
	if len(cfg.BattleServers) != 1 || cfg.BattleServers[0].Region != "file" {
		t.Fatalf("file did not replace the battle servers: %+v", cfg.BattleServers)
	}
	if cfg.EmbeddedBattleServer.StartIfMissing != "required" {
		t.Fatalf("StartIfMissing = %q", cfg.EmbeddedBattleServer.StartIfMissing)
	}
}

func TestLoadGameConfigEmbeddedTableOnlyOverridesNamedFields(t *testing.T) {
	dir := t.TempDir()
	withGameConfigDir(t, dir)
	base := internal.Game{EmbeddedBattleServer: internal.EmbeddedBattleServer{
		Region: "legacy-region", Name: "Legacy", IPv4: "192.168.1.5", BsPort: 30003,
	}}
	writeGameFile(t, dir, "config.age2.toml", "[Game.EmbeddedBattleServer]\nStartIfMissing = 'false'\n")
	cfg, found, err := loadGameConfig(base, "age2", "", &server.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("a file with an effective key is found")
	}
	embedded := cfg.EmbeddedBattleServer
	if embedded.StartIfMissing != "false" {
		t.Fatalf("StartIfMissing = %q, want the file value", embedded.StartIfMissing)
	}
	if embedded.Region != "legacy-region" || embedded.Name != "Legacy" ||
		embedded.IPv4 != "192.168.1.5" || embedded.BsPort != 30003 {
		t.Fatalf("partial table dropped inherited values: %+v", embedded)
	}
}

func TestLoadGameConfigTemplateOnlyCommentsIsNotFound(t *testing.T) {
	dir := t.TempDir()
	withGameConfigDir(t, dir)
	writeGameFile(t, dir, "config.age2.toml", "# Per-game configuration.\n\n[Game.EmbeddedBattleServer]\n# StartIfMissing = 'true'\n")
	cfg, found, err := loadGameConfig(legacyBase(), "age2", "", &server.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("a template with no effective key must count as not found")
	}
	if len(cfg.BattleServers) != 1 || cfg.BattleServers[0].Region != "legacy" {
		t.Fatalf("legacy battle servers lost: %+v", cfg.BattleServers)
	}
}

func TestLoadGameConfigRejectsUnknownAndForeignKeys(t *testing.T) {
	cases := map[string]string{
		"typo":        "[Game.EmbeddedBattleServer]\nStartIfMising = 'true'\n",
		"games":       "[Games.age2]\nHosts = ['1.2.3.4']\n",
		"top-level":   "Hosts = ['1.2.3.4']\n",
		"bad-start":   "[Game.EmbeddedBattleServer]\nStartIfMissing = 'yes'\n",
		"battle-typo": "[[Game.BattleServers]]\nRegoin = 'x'\n",
	}
	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			withGameConfigDir(t, dir)
			path := writeGameFile(t, dir, "config.age2.toml", contents)
			if _, _, err := loadGameConfig(internal.Game{}, "age2", "", &server.Values{}); err == nil {
				t.Fatalf("expected an error for %s", path)
			}
		})
	}
}

func TestLoadGameConfigIgnoresGameHosts(t *testing.T) {
	dir := t.TempDir()
	withGameConfigDir(t, dir)
	writeGameFile(t, dir, "config.age2.toml", "[Game.Hosts]\nHosts = ['1.2.3.4']\n")
	cfg, found, err := loadGameConfig(internal.Game{}, "age2", "", &server.Values{})
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("a file with only [Game.Hosts] carries no effective key")
	}
	if cfg.Hosts != nil {
		t.Fatalf("[Game.Hosts] must be ignored, got %v", cfg.Hosts)
	}
}

func TestLoadGameConfigExplicitPathMustExist(t *testing.T) {
	dir := t.TempDir()
	withGameConfigDir(t, dir)
	missing := filepath.Join(dir, "does-not-exist.toml")
	_, _, err := loadGameConfig(internal.Game{}, "age2", "", &server.Values{GameConfig: missing})
	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("expected an error naming %s, got %v", missing, err)
	}
}

func TestApplyGameOverridesFlagWins(t *testing.T) {
	dir := t.TempDir()
	withGameConfigDir(t, dir)
	writeGameFile(t, dir, "config.age2.toml", "[Game.EmbeddedBattleServer]\nStartIfMissing = 'false'\n")
	cfg := &internal.Configuration{
		Games: internal.Games{Enabled: []string{"age2"}},
	}
	values := &server.Values{StartBsIfMissing: "true"}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("startBsIfMissing", "", "")
	if err := fs.Set("startBsIfMissing", "true"); err != nil {
		t.Fatal(err)
	}
	if err := applyGameOverrides(cfg, "", fs, values, mapset.NewThreadUnsafeSet[string]("age2")); err != nil {
		t.Fatal(err)
	}
	if got := cfg.Games.Age2.EmbeddedBattleServer.StartIfMissing; got != "true" {
		t.Fatalf("StartIfMissing = %q, want the flag value", got)
	}
}

func TestApplyGameOverridesGameConfigNeedsOneGame(t *testing.T) {
	dir := t.TempDir()
	withGameConfigDir(t, dir)
	cfg := &internal.Configuration{Games: internal.Games{Enabled: []string{"age1", "age2"}}}
	err := applyGameOverrides(cfg, "", nil, &server.Values{GameConfig: filepath.Join(dir, "x.toml")},
		mapset.NewThreadUnsafeSet[string]("age1", "age2"))
	if err == nil {
		t.Fatal("--gameConfig with two games must fail")
	}
}
