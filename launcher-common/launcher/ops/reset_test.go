package ops

import (
	"path/filepath"
	"testing"

	"github.com/luskaner/ageServer/launcher-common/launcher"
)

// A second session in the same process must not inherit what the first one
// changed.
//
// The fields a Config carries record what a run changed, and that is what makes a
// failure reversible. A frontend that performs one session after another would
// otherwise start the second one believing a server was already running under
// its control, and its teardown would go looking for something to stop that it
// never started.
func TestResetClearsWhatTheRunChanged(t *testing.T) {
	c := &Config{Report: launcher.Discard{}}
	c.gameId = "age2"
	c.serverExe = "server.exe"
	c.setupCommandRan = true
	c.hostFilePath = "/tmp/hosts"
	c.certFilePath = "/tmp/cert.pem"
	c.sigKeyFilePath = "/tmp/sigkey.der"
	c.battleServerRegion = "eu"
	c.battleServerExe = "bsm.exe"

	c.Reset()

	if c.serverExe != "" {
		t.Errorf("serverExe = %q, want empty", c.serverExe)
	}
	if c.setupCommandRan {
		t.Error("setupCommandRan is still set, so this run would undo a setup it did not do")
	}
	if c.hostFilePath != "" || c.certFilePath != "" || c.sigKeyFilePath != "" {
		t.Errorf("paths survived: hostFilePath=%q certFilePath=%q sigKeyFilePath=%q",
			c.hostFilePath, c.certFilePath, c.sigKeyFilePath)
	}
	if c.battleServerRegion != "" || c.battleServerExe != "" {
		t.Errorf("battle server survived: region=%q exe=%q", c.battleServerRegion, c.battleServerExe)
	}
	if c.gameId != "" {
		t.Errorf("gameId = %q, want empty until the run sets it", c.gameId)
	}
}

// Reset is about the run, not the frontend: clearing it must not silence the run.
func TestResetKeepsTheReporter(t *testing.T) {
	r := &countingReporter{}
	c := &Config{Report: r}
	c.Reset()
	if c.report() != launcher.Reporter(r) {
		t.Error("Reset replaced the reporter, which belongs to the frontend")
	}
}

// Teardown decides whether it has anything to do from what the run recorded, so a
// run that changed nothing must not announce work it will not do.
func TestResetMakesTeardownQuiet(t *testing.T) {
	tempStores(t)
	noAgent(t)
	c := &Config{Report: launcher.Discard{}}
	c.serverExe = "server.exe"
	if !c.HasTeardownWork() {
		t.Fatal("a run that started a server should have teardown work")
	}
	c.Reset()
	if c.HasTeardownWork() {
		t.Error("a run that changed nothing still claims teardown work")
	}
}

// The revert command runs through the agent when the agent is launched, and
// through the Config when it is not. A setup command marks that a revert may be
// needed; without the mark, a run that failed before the agent existed skipped it
// silently, which is the case the option documents itself for.
func TestRunningASetupCommandMarksThatARevertMayBeNeeded(t *testing.T) {
	tempStores(t)
	c := &Config{Report: launcher.Discard{}}
	if c.RequiresRunningRevertCommand() {
		t.Fatal("nothing has run, so nothing needs reverting")
	}

	// A command that cannot be executed still counts: it may have failed
	// halfway, and a revert is exactly what undoes halfway.
	c.RunSetupCommand([]string{commandThatDoesNotExist(t)})

	if !c.setupCommandRan {
		t.Error("the setup command ran and the Config does not know it")
	}
	// With no revert command configured there is nothing to require, which is the
	// other half of the same condition.
	if c.RequiresRunningRevertCommand() {
		t.Error("no revert command is configured, so none should be required")
	}
}

// commandThatDoesNotExist returns a path that is not there, so the exec fails
// fast instead of running whatever happens to be on the machine.
func commandThatDoesNotExist(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "no-such-setup-command")
}

type countingReporter struct {
	launcher.Discard
}
