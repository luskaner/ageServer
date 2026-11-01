package session

import (
	"testing"

	"github.com/spf13/pflag"
)

// Regression: the initConfig defaults map used "Client.IsolationProfiles"
// (missing dot) as the key, so Client.Isolation.Profiles was never filled by
// the defaults layer. It was masked in normal operation because the -p flag
// default ("required") always provided a value through the flags layer.
func TestInitConfigDefaultsCoverIsolationProfiles(t *testing.T) {
	oldGameId, oldCfgFile, oldGameCfgFile := gameId, cfgFile, gameCfgFile
	gameId = "age2"
	cfgFile = ""
	gameCfgFile = ""
	defer func() { gameId, cfgFile, gameCfgFile = oldGameId, oldCfgFile, oldGameCfgFile }()

	fs := pflag.NewFlagSet("probe", pflag.ContinueOnError)
	cfg := initConfig(fs)

	if cfg.Client.Isolation.Profiles != "required" {
		t.Fatalf("Isolation.Profiles = %q, want %q by default", cfg.Client.Isolation.Profiles, "required")
	}
}
