package config

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestSetUpFlagSetRegistersFlags(t *testing.T) {
	values, flags := SetUpFlagSet()
	if values == nil || flags == nil {
		t.Fatal("SetUpFlagSet returned nil")
	}
	for _, name := range []string{"ip", "localCert", "macExclusiveDomain", "caStoreCert", "agentEndOnError", "gamePath", "dataPath", "hostFilePath", "certFilePath", "metadata", "profiles", "logRoot", "game", "internet"} {
		if f := flags.Lookup(name); f == nil {
			t.Errorf("flag %q not registered", name)
		}
	}
}

func TestSetUpFlagSetParsesFlags(t *testing.T) {
	values, flags := SetUpFlagSet()
	err := flags.Parse([]string{"--gamePath", "/game", "--metadata", "--caStoreCert", "dGVzdA=="})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if values.GamePath != "/game" {
		t.Errorf("GamePath = %q, want /game", values.GamePath)
	}
	if !values.Metadata {
		t.Error("Metadata should be true")
	}
}

func TestInitSetUp(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	values := InitSetUp(flags)
	if values == nil {
		t.Fatal("InitSetUp returned nil")
	}
	err := flags.Parse([]string{"--ip", "1.2.3.4"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if values.MapIp.String() != "1.2.3.4" {
		t.Errorf("MapIp = %v, want 1.2.3.4", values.MapIp)
	}
	if !values.CanUseInternet {
		t.Error("CanUseInternet should default to true")
	}
}

func TestInitSetUpCanUseInternetFalse(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	values := InitSetUp(flags)
	if values == nil {
		t.Fatal("InitSetUp returned nil")
	}
	err := flags.Parse([]string{"--internet=false"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if values.CanUseInternet {
		t.Error("CanUseInternet should be false when flag is false")
	}
}

func TestRevertFlagSetRegistersFlags(t *testing.T) {
	values, flags := RevertFlagSet()
	if values == nil || flags == nil {
		t.Fatal("RevertFlagSet returned nil")
	}
	for _, name := range []string{"ip", "localCert", "all", "gamePath", "dataPath", "hostFilePath", "certFilePath", "metadata", "profiles", "caStoreCert", "logRoot", "game"} {
		if f := flags.Lookup(name); f == nil {
			t.Errorf("flag %q not registered", name)
		}
	}
}

func TestRevertFlagSetParsesFlags(t *testing.T) {
	values, flags := RevertFlagSet()
	err := flags.Parse([]string{"--ip", "--localCert", "--gamePath", "/g"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !values.IPs {
		t.Error("IPs should be true")
	}
	if !values.Certs {
		t.Error("Certs should be true")
	}
	if values.GamePath != "/g" {
		t.Errorf("GamePath = %q, want /g", values.GamePath)
	}
}

func TestFlushCacheFlagSetRegistersFlags(t *testing.T) {
	values, flags := FlushCacheFlagSet()
	if values == nil || flags == nil {
		t.Fatal("FlushCacheFlagSet returned nil")
	}
	for _, name := range []string{"flushIpCache", "flushCertsCache", "logRoot"} {
		if f := flags.Lookup(name); f == nil {
			t.Errorf("flag %q not registered", name)
		}
	}
	if values.IPs || values.Certs {
		t.Error("bool flags should default to false")
	}
}

func TestAddCommonFlags(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	values := AddCommonFlags(flags, "hostSuffix", "certSuffix", "metaDesc", "profDesc")
	if values == nil {
		t.Fatal("AddCommonFlags returned nil")
	}
	err := flags.Parse([]string{"--gamePath", "/g", "--dataPath", "/d", "--metadata", "--profiles"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if values.GamePath != "/g" || values.DataPath != "/d" {
		t.Errorf("GamePath=%q DataPath=%q", values.GamePath, values.DataPath)
	}
	if !values.Metadata || !values.Profiles {
		t.Error("Metadata and Profiles should be true")
	}
}

func TestNewRevertMinimalValues(t *testing.T) {
	v := NewRevertMinimalValues()
	if v == nil {
		t.Fatal("NewRevertMinimalValues returned nil")
	}
	if v.IPs || v.Certs {
		t.Error("defaults should be false")
	}
}

func TestNewRevertValues(t *testing.T) {
	v := NewRevertValues()
	if v.CommonBaseValues == nil || v.RevertBaseValues == nil {
		t.Fatal("NewRevertValues should initialize embedded structs")
	}
}

func TestNewCommonBaseValues(t *testing.T) {
	v := NewCommonBaseValues()
	if v.GameIdValues == nil || v.LogRootValues == nil {
		t.Fatal("NewCommonBaseValues should initialize embedded structs")
	}
}

func TestInitBaseRevert(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	values := InitBaseRevert(flags)
	if values == nil {
		t.Fatal("InitBaseRevert returned nil")
	}
	err := flags.Parse([]string{"--ip", "--all"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !values.IPs || !values.RemoveAll {
		t.Error("IPs and RemoveAll should be true")
	}
}

func TestFlushCacheSingleFlagSet(t *testing.T) {
	values, singleFs := FlushCacheSingleFlagSet("3.0", func(fs *pflag.FlagSet) (err error, exitCode int) {
		return nil, 0
	})
	if values == nil || singleFs == nil {
		t.Fatal("FlushCacheSingleFlagSet returned nil")
	}
	fs := singleFs.Fs()
	if f := fs.Lookup("flushIpCache"); f == nil {
		t.Error("flushIpCache flag not registered")
	}
}
