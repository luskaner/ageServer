package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestFlagSetToArgsIncludeName(t *testing.T) {
	fs := pflag.NewFlagSet("myapp", pflag.ContinueOnError)
	fs.String("key", "val", "")
	_ = fs.Set("key", "other")

	args := FlagSetToArgs(fs, true)
	if len(args) == 0 || args[0] != "myapp" {
		t.Fatalf("first arg should be flagset name, got %v", args)
	}
}

func TestFlagSetToArgsBoolTrue(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Bool("verbose", false, "")
	_ = fs.Set("verbose", "true")

	args := FlagSetToArgs(fs, false)
	found := false
	for _, a := range args {
		if a == "--verbose" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --verbose, got %v", args)
	}
}

func TestFlagSetToArgsBoolFalse(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Bool("verbose", true, "")
	_ = fs.Set("verbose", "false")

	args := FlagSetToArgs(fs, false)
	found := false
	for _, a := range args {
		if a == "--verbose=false" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --verbose=false, got %v", args)
	}
}

func TestFlagSetToArgsString(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.String("name", "", "")
	_ = fs.Set("name", "hello")

	args := FlagSetToArgs(fs, false)
	found := false
	for _, a := range args {
		if a == "--name=hello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --name=hello, got %v", args)
	}
}

func TestFlagSetToArgsInt(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Int("port", 0, "")
	_ = fs.Set("port", "8080")

	args := FlagSetToArgs(fs, false)
	found := false
	for _, a := range args {
		if a == "--port=8080" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --port=8080, got %v", args)
	}
}

func TestFlagSetToArgsStringSlice(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.StringSlice("tags", nil, "")
	_ = fs.Set("tags", "a,b")

	args := FlagSetToArgs(fs, false)
	hasA, hasB := false, false
	for _, a := range args {
		if a == "--tags=a" {
			hasA = true
		}
		if a == "--tags=b" {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Fatalf("expected --tags=a and --tags=b, got %v", args)
	}
}

func TestFlagSetToArgsStringSliceEmpty(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.StringSlice("tags", nil, "")
	// Don't set — default is empty []

	args := FlagSetToArgs(fs, false)
	for _, a := range args {
		if a == "[]" || a == "--tags=" {
			t.Fatalf("empty slice should be skipped, got %v", args)
		}
	}
}

func TestFlagSetToArgsStringArray(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.StringArray("items", nil, "")
	_ = fs.Set("items", "x")
	_ = fs.Set("items", "y")

	args := FlagSetToArgs(fs, false)
	hasX, hasY := false, false
	for _, a := range args {
		if a == "--items=x" {
			hasX = true
		}
		if a == "--items=y" {
			hasY = true
		}
	}
	if !hasX || !hasY {
		t.Fatalf("expected --items=x and --items=y, got %v", args)
	}
}

func TestFlagSetToArgsDefaultSliceType(t *testing.T) {
	// Custom type ending with "Slice" goes through the default branch
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.StringSlice("custom", nil, "")
	_ = fs.Set("custom", "v1,v2")

	args := FlagSetToArgs(fs, false)
	count := 0
	for _, a := range args {
		if a == "--custom=v1" || a == "--custom=v2" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 custom args, got %d from %v", count, args)
	}
}

func TestFlagSetToArgsGenericType(t *testing.T) {
	// Unknown type falls to default else branch
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.IP("bind", nil, "")
	_ = fs.Set("bind", "1.2.3.4")

	args := FlagSetToArgs(fs, false)
	found := false
	for _, a := range args {
		if a == "--bind=1.2.3.4" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --bind=1.2.3.4, got %v", args)
	}
}

func TestFlagSetToArgsSkipsDefault(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.String("k", "default-val", "")
	// Don't set — should be skipped

	args := FlagSetToArgs(fs, false)
	for _, a := range args {
		if a == "--k=default-val" {
			t.Fatalf("default values should be skipped, got %v", args)
		}
	}
}

func TestFlagSetToArgsPositionalArgs(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.String("k", "", "")
	_ = fs.Set("k", "v")

	args := FlagSetToArgs(fs, false)
	// fs.Args() returns remaining args after parsing
	if len(args) < 1 {
		t.Fatal("expected at least one arg")
	}
}

func TestFlagSetToArgsEmptyFlagSet(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	args := FlagSetToArgs(fs, false)
	if len(args) != 0 {
		t.Fatalf("empty flagset should produce no args, got %v", args)
	}
}

func TestFlagSetToArgsDuration(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	fs.Duration("timeout", 0, "")
	_ = fs.Set("timeout", "5s")

	args := FlagSetToArgs(fs, false)
	found := false
	for _, a := range args {
		if a == "--timeout=5s" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected --timeout=5s, got %v", args)
	}
}

func TestFlagSetToArgsEmptyStringSliceValue(t *testing.T) {
	// Simulate the edge case where a stringSlice flag has a non-default value
	// that trims to empty after [ ] removal. pflag.StringSlice always wraps
	// in brackets, so Set("[]") → "[[]]". We inject a custom Value to bypass this.
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	sv := &emptySliceFlag{val: "[x]"} // DefValue will be "[x]"
	fs.Var(sv, "es", "")
	_ = fs.Set("es", "[]") // value becomes "[]", differs from DefValue "[x]"

	args := FlagSetToArgs(fs, false)
	for _, a := range args {
		if a == "--es=" || a == "--es=[]" {
			t.Fatalf("empty trimmed slice should be skipped, got %v", args)
		}
	}
}

// emptySliceFlag is a pflag.Value whose Type() returns "stringSlice"
// to exercise the empty-val early return in the stringSlice case.
type emptySliceFlag struct {
	val string
}

func (e *emptySliceFlag) String() string     { return e.val }
func (e *emptySliceFlag) Set(s string) error { e.val = s; return nil }
func (e *emptySliceFlag) Type() string       { return "stringSlice" }

// customSliceType wraps a string slice with a custom type name ending in "Slice"
// to exercise the default branch in FlagSetToArgs.
type customSliceType []string

func (c *customSliceType) String() string     { return strings.Join(*c, ",") }
func (c *customSliceType) Set(s string) error { *c = strings.Split(s, ","); return nil }
func (c *customSliceType) Type() string       { return "customSlice" }

func TestFlagSetToArgsCustomSliceType(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	var val customSliceType = []string{"initial"}
	fs.AddFlag(&pflag.Flag{
		Name:     "cslice",
		DefValue: val.String(),
		Value:    &val,
	})
	_ = val.Set("a,b")
	args := FlagSetToArgs(fs, false)
	hasA, hasB := false, false
	for _, a := range args {
		if a == "--cslice=a" {
			hasA = true
		}
		if a == "--cslice=b" {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Fatalf("expected --cslice=a and --cslice=b, got %v", args)
	}
}

func TestFlagSetToArgsCustomSliceTypeEmpty(t *testing.T) {
	fs := pflag.NewFlagSet("t", pflag.ContinueOnError)
	var val customSliceType = []string{"x"}
	fs.AddFlag(&pflag.Flag{
		Name:     "cslice",
		DefValue: val.String(),
		Value:    &val,
	})
	// Set to empty slice → after trim, val == ""
	_ = val.Set("")
	args := FlagSetToArgs(fs, false)
	for _, a := range args {
		if a == "--cslice=" {
			t.Fatalf("empty custom slice should be skipped, got %v", args)
		}
	}
}
