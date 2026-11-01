package genCert

import (
	"testing"

	"github.com/spf13/pflag"
)

func noopRun(*pflag.FlagSet) (err error, exitCode int) { return }

func TestSingleFlagSetDefaultsAndParse(t *testing.T) {
	values, singleFs := SingleFlagSet("v-test", noopRun)
	for name, want := range map[string]string{
		"replace":          "false",
		"ignoreIfExisting": "false",
		"help":             "false",
	} {
		f := singleFs.Fs().Lookup(name)
		if f == nil {
			t.Fatalf("flag %q not registered", name)
		}
		if f.DefValue != want {
			t.Errorf("%s default = %q, want %q", name, f.DefValue, want)
		}
	}
	if err := singleFs.Fs().Parse([]string{"-r"}); err != nil {
		t.Fatal(err)
	}
	if !values.Replace || values.IgnoreIfExisting {
		t.Fatalf("values = %+v", values)
	}
}
