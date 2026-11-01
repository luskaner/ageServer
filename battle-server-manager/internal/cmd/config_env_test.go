package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/v2"
	"github.com/luskaner/ageServer/common"
	bsManager "github.com/luskaner/ageServer/common/cmd/bsManager"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/spf13/pflag"
)

// Regression: the Docker image ships AGESERVER_BATTLE_SERVER_MANAGER_*
// environment variables written with the same case as the configuration keys
// (e.g. ..._Ports_Bs -> Ports.Bs). The env provider preserves that case, so
// these overrides must reach the capitalized koanf keys.
func TestStartConfigEnvOverridesCapitalizedKeys(t *testing.T) {
	t.Setenv("AGESERVER_BATTLE_SERVER_MANAGER_Ports_Bs", "27012")
	t.Setenv("AGESERVER_BATTLE_SERVER_MANAGER_CertsPath", "/app/server/resources/certificates")

	k := koanf.New(".")
	defaults := map[string]any{
		"Region":               "auto",
		"Name":                 "auto",
		"Host":                 "auto",
		"CertsPath":            "auto",
		"Executable.Path":      "auto",
		"Executable.ExtraArgs": []string{},
		"Ports.Bs":             0,
		"Ports.WebSocket":      0,
		"Ports.OutOfBand":      0,
	}
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	if _, err := common.LoadKoanfLayers(k, defaults, nil, toml.Parser(), fs, nil, executables.BattleServerManager); err != nil {
		t.Fatal(err)
	}

	if got := k.Int("Ports.Bs"); got != 27012 {
		t.Fatalf("env must override Ports.Bs, got %d", got)
	}
	if got := k.String("CertsPath"); got != "/app/server/resources/certificates" {
		t.Fatalf("env must override CertsPath, got %q", got)
	}
}

func TestInitConfigReadsCapitalizedToml(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.age2.toml")
	const contents = `Region = 'eu'
Name = 'auto'
Host = '192.168.1.10'
CertsPath = 'auto'
[Executable]
Path = 'auto'
ExtraArgs = []
[Ports]
Bs = 21001
WebSocket = 21002
OutOfBand = 21003
`
	if err := os.WriteFile(cfgFile, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	values, flags := bsManager.StartFlagSet(configPaths)
	if err := flags.Parse([]string{"--game", "age2", "--gameConfig", cfgFile}); err != nil {
		t.Fatal(err)
	}
	c := initConfig(flags, values)
	if c.Region != "eu" || c.Host != "192.168.1.10" {
		t.Fatalf("scalars wrong: region=%q host=%q", c.Region, c.Host)
	}
	if c.Ports.Bs != 21001 || c.Ports.WebSocket != 21002 || c.Ports.OutOfBand != 21003 {
		t.Fatalf("ports wrong: %+v", c.Ports)
	}
}
