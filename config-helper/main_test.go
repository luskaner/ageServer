package main

import (
	"strings"
	"testing"
)

// Regression: main indexed os.Args[1] unconditionally (panic without
// arguments) and unknown subcommands silently succeeded with empty output.
func TestRunWithoutArgumentsFails(t *testing.T) {
	if err := run([]string{"config-helper"}); err == nil {
		t.Fatal("expected usage error without arguments")
	}
}

func TestRunUnknownCommandFails(t *testing.T) {
	err := run([]string{"config-helper", "made-up-command"})
	if err == nil {
		t.Fatal("unknown command must fail, not silently succeed")
	}
	if !strings.Contains(err.Error(), "made-up-command") {
		t.Fatalf("error must mention the command: %v", err)
	}
}

func TestRunSubcommandsRequireArguments(t *testing.T) {
	for _, cmd := range []string{"windowsToUnixPath", "configPath", "userProfilePath"} {
		if err := run([]string{"config-helper", cmd}); err == nil {
			t.Errorf("%s without arguments must fail", cmd)
		}
	}
}

// Outside a Wine environment the conversion always fails with this error;
// the important contract is that it FAILS (non-zero exit via main) instead of
// printing nothing and succeeding.
func TestRunWindowsToUnixPathOutsideWineFails(t *testing.T) {
	err := run([]string{"config-helper", "windowsToUnixPath", `C:\Users\test`})
	if err == nil {
		t.Skip("wine_get_unix_file_name available: running inside Wine")
	}
	if !strings.Contains(err.Error(), "not a Wine environment") && !strings.Contains(err.Error(), "wine could not resolve") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunEmptyPathFails(t *testing.T) {
	// WindowsToUnixPath rejects empty paths before touching Wine APIs.
	err := run([]string{"config-helper", "windowsToUnixPath", ""})
	if err == nil {
		t.Fatal("empty path must fail")
	}
	if !strings.Contains(err.Error(), "empty path") {
		t.Fatalf("unexpected error: %v", err)
	}
}
