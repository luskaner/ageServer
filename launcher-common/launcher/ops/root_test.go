package ops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	"github.com/luskaner/ageServer/launcher-common/ui"
)

// tempStores reemplaza los ArgsStore globales por ficheros temporales para que
// los tests de Config no ensucien el estado real y se restauren automáticamente.
func tempStores(t *testing.T) {
	t.Helper()
	origConfig := launcherCommon.RevertConfigStore
	origCommand := launcherCommon.RevertCommandStore
	launcherCommon.RevertConfigStore = launcherCommon.NewArgsStore(filepath.Join(t.TempDir(), "revert_config.txt"))
	launcherCommon.RevertCommandStore = launcherCommon.NewArgsStore(filepath.Join(t.TempDir(), "revert_command.txt"))
	t.Cleanup(func() {
		launcherCommon.RevertConfigStore = origConfig
		launcherCommon.RevertCommandStore = origCommand
	})
}

// noAgent makes the agent probe answer "not running". The default probe reads
// the machine's process table, so an agent left running by a launcher answered
// for it and a Config with no state of its own still reported teardown work.
func noAgent(t *testing.T) {
	t.Helper()
	orig := processFn
	processFn = func(string) (string, *os.Process, error) { return "", nil, errors.New("not running") }
	t.Cleanup(func() { processFn = orig })
}

func TestConfigSetGameId(t *testing.T) {
	c := &Config{}
	c.SetGameId("age2")
	if c.gameId != "age2" {
		t.Fatalf("gameId = %q, want age2", c.gameId)
	}
}

func TestConfigRequiresConfigRevert(t *testing.T) {
	tempStores(t)
	c := &Config{}

	// Sin store no requiere revert.
	if c.RequiresConfigRevert() {
		t.Fatal("empty store should not require revert")
	}

	if err := launcherCommon.RevertConfigStore.Store([]string{"--ip"}); err != nil {
		t.Fatal(err)
	}
	if !c.RequiresConfigRevert() {
		t.Fatal("store with args should require revert")
	}
}

func TestConfigRevertCommand(t *testing.T) {
	tempStores(t)
	c := &Config{}

	// Sin setupCommandRan siempre devuelve vacío.
	if got := c.RevertCommand(); len(got) != 0 {
		t.Fatalf("RevertCommand() = %v, want empty", got)
	}

	if err := launcherCommon.RevertCommandStore.Store([]string{"echo", "hi"}); err != nil {
		t.Fatal(err)
	}
	c.setupCommandRan = true
	got := c.RevertCommand()
	if len(got) != 2 || got[0] != "echo" || got[1] != "hi" {
		t.Fatalf("RevertCommand() = %v", got)
	}

	// Volver a desactivar setupCommandRan devuelve vacío aunque haya store.
	c.setupCommandRan = false
	if got := c.RevertCommand(); len(got) != 0 {
		t.Fatalf("RevertCommand() with setupCommandRan=false = %v, want empty", got)
	}
}

func TestConfigRequiresRunningRevertCommand(t *testing.T) {
	tempStores(t)

	// setupCommandRan false: nunca requiere.
	c := &Config{setupCommandRan: false}
	if c.RequiresRunningRevertCommand() {
		t.Fatal("setupCommandRan=false should not require running revert")
	}

	// setupCommandRan true pero store vacío: no requiere.
	c.setupCommandRan = true
	if c.RequiresRunningRevertCommand() {
		t.Fatal("setupCommandRan=true with empty store should not require")
	}

	// setupCommandRan true y store con args: requiere.
	if err := launcherCommon.RevertCommandStore.Store([]string{"run"}); err != nil {
		t.Fatal(err)
	}
	if !c.RequiresRunningRevertCommand() {
		t.Fatal("setupCommandRan=true with store should require")
	}
}

// The heading is only honest when there is something to undo. Revert runs on every
// exit, but on a clean run it has nothing left to print, so asking for the phase
// is what keeps a heading over an empty phase off the screen.
func TestHasTeardownWork(t *testing.T) {
	t.Cleanup(func() { _ = launcherCommon.RevertConfigStore.Delete() })
	// Pinned, so the answer does not depend on whether an agent happens to be
	// running on the machine that runs the tests.
	orig := processFn
	t.Cleanup(func() { processFn = orig })
	processFn = func(string) (string, *os.Process, error) { return "", nil, errors.New("not running") }
	if err := launcherCommon.RevertConfigStore.Store([]string{"--revert"}); err != nil {
		t.Fatalf("storing revert args: %v", err)
	}
	c := &Config{}
	if !c.HasTeardownWork() {
		t.Error("a stored revert argument is work")
	}
	if err := launcherCommon.RevertConfigStore.Delete(); err != nil {
		t.Fatalf("clearing the store: %v", err)
	}
	if c.HasTeardownWork() {
		t.Error("a Config that touched nothing must report no work")
	}
	c.serverExe = "server.exe"
	if !c.HasTeardownWork() {
		t.Error("a started server is work")
	}
	c.serverExe = ""
	c.battleServerExe = "bsm.exe"
	if c.HasTeardownWork() {
		t.Error("a battle server executable with no region is not work")
	}
	c.battleServerRegion = "eu"
	if !c.HasTeardownWork() {
		t.Error("a battle server region is work")
	}
}

// The teardown announces the agent only when there is one to stop. A clean run has
// no agent by then, and "Stopping config-admin-agent..." over nothing is a step
// that did not happen.
func TestKillAgentOnlySpeaksWhenThereIsAnAgent(t *testing.T) {
	orig := processFn
	t.Cleanup(func() { processFn = orig })
	c := &Config{}
	var out strings.Builder
	console := ui.SetOutput(&out)
	defer console()

	processFn = func(string) (string, *os.Process, error) { return "", nil, errors.New("not running") }
	if c.AgentRunning() {
		t.Error("no agent must not report one running")
	}
	out.Reset()
	c.KillAgent()
	if out.Len() != 0 {
		t.Errorf("KillAgent printed %q with no agent running", out.String())
	}

	processFn = func(string) (string, *os.Process, error) { return "pid", &os.Process{}, nil }
	if !c.AgentRunning() {
		t.Error("a running agent must be reported")
	}
	if !c.HasTeardownWork() {
		t.Error("a running agent is teardown work, and needs its heading")
	}
}
