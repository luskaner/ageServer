package agent

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestSingleFlagSetReturnsValuesAndFlags(t *testing.T) {
	values, singleFs := SingleFlagSet("1.0", func(fs *pflag.FlagSet) (err error, exitCode int) {
		return nil, 0
	})
	if values == nil || singleFs == nil {
		t.Fatal("SingleFlagSet returned nil values or flags")
	}
	if values.GameIdValues == nil || values.LogRootValues == nil {
		t.Fatal("embedded values should be initialized")
	}
}

func TestSingleFlagSetRegistersFlags(t *testing.T) {
	_, singleFs := SingleFlagSet("2.0", func(fs *pflag.FlagSet) (err error, exitCode int) {
		return nil, 0
	})
	fs := singleFs.Fs()
	for _, name := range []string{"serverExecutable", "bsManagerExecutable", "bsRegion", "processes", "baseDataPath", "logRoot", "game"} {
		if f := fs.Lookup(name); f == nil {
			t.Errorf("flag %q not registered", name)
		}
	}
}

func TestSingleFlagSetParsesFlags(t *testing.T) {
	values, singleFs := SingleFlagSet("1.0", func(fs *pflag.FlagSet) (err error, exitCode int) {
		return nil, 0
	})
	fs := singleFs.Fs()
	err := fs.Parse([]string{"--serverExecutable", "/path/to/server", "--game", "aoe2de"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if values.ServerExecutable != "/path/to/server" {
		t.Errorf("ServerExecutable = %q, want /path/to/server", values.ServerExecutable)
	}
	if values.GameId != "aoe2de" {
		t.Errorf("GameId = %q, want aoe2de", values.GameId)
	}
}
