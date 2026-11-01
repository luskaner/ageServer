package session

import (
	"context"
	"testing"

	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/spf13/pflag"
)

// installValues points the session at a frontend's own option storage for the
// duration of a test and puts back whatever was there before.
func installValues(t *testing.T, v launcher.Values) {
	t.Helper()
	orig := setup
	t.Cleanup(func() { setup = orig })
	Configure(Setup{Values: &v})
}

// The run reads the options from the storage the frontend named.
//
// This is the seam that makes a frontend other than the console possible. The
// console hands its words on a command line to BindFlags, which writes them
// here; a window holds them in its controls and never calls BindFlags at all. If
// the run read this package's storage either way, a window would be showing the
// configuration it resolved while running a different one, and the two would
// only disagree once the run touched the machine.
func TestRunReadsItsOptionsFromTheFrontendStorage(t *testing.T) {
	// The package storage says age1 and the frontend's says age2, so the game
	// the run acts on says which one it read.
	restore := applyOverrides(t, runRootOverrides{
		gameId:        "age1",
		isAdmin:       false,
		gameSupported: false,
	})
	defer restore()

	// Rejected here on purpose: the run stops at the game check, having already
	// read the option it was given, which is the only thing this is about.
	installValues(t, launcher.Values{
		ConfigFile:     new(string),
		GameConfigFile: new(string),
		GameID:         ptr("age2"),
		Output:         new(string),
	})
	var asked string
	gameSupportedGamesContainsOneFn = func(id string) bool {
		asked = id
		return false
	}

	if _, exitCode := Run(context.Background(), pflag.NewFlagSet("test", pflag.ContinueOnError)); exitCode != launcherCommon.ErrInvalidGame {
		t.Fatalf("exit code = %d, want %d", exitCode, launcherCommon.ErrInvalidGame)
	}
	if asked != "age2" {
		t.Fatalf("the run asked about game %q, want the one the frontend named (%q)", asked, "age2")
	}
}

// A frontend whose storage is missing one of the options is told what it left
// out, rather than the run reaching through a nil pointer to find out.
func TestRunReportsAMissingGameInsteadOfReadingNothing(t *testing.T) {
	restore := applyOverrides(t, runRootOverrides{
		gameId:        "age1",
		isAdmin:       false,
		gameSupported: true,
	})
	defer restore()

	installValues(t, launcher.Values{ConfigFile: new(string)})

	_, exitCode := Run(context.Background(), pflag.NewFlagSet("test", pflag.ContinueOnError))
	if exitCode == 0 {
		t.Fatal("a run with no game anywhere carried on as if it had one")
	}
}

// Leaving Values out is what the console does, and it keeps reading this
// package's own storage.
func TestRunFallsBackToThisPackagesStorage(t *testing.T) {
	restore := applyOverrides(t, runRootOverrides{
		gameId:        "age2",
		isAdmin:       false,
		gameSupported: false,
	})
	defer restore()

	orig := setup
	t.Cleanup(func() { setup = orig })
	Configure(Setup{})
	if setup.Values != nil {
		t.Fatal("Configure stored option storage nobody gave it")
	}

	var asked string
	gameSupportedGamesContainsOneFn = func(id string) bool {
		asked = id
		return false
	}

	if _, exitCode := Run(context.Background(), pflag.NewFlagSet("test", pflag.ContinueOnError)); exitCode != launcherCommon.ErrInvalidGame {
		t.Fatalf("exit code = %d, want %d", exitCode, launcherCommon.ErrInvalidGame)
	}
	if asked != "age2" {
		t.Fatalf("the run asked about game %q, want the one BindFlags wrote (%q)", asked, "age2")
	}
}

func ptr[T any](v T) *T { return &v }
