package ops

import (
	"io"
	osExec "os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
	launcherCommon "github.com/luskaner/ageServer/launcher-common"
	"github.com/luskaner/ageServer/launcher-common/launcher"
)

// The launcher-config process is faked out because what these tests pin down is
// the wiring between the pass the session chose and the elevated side: the
// single "--api"/"--rootOnly" flag each pass sends, and which half of the
// domains gets cached for the later pass to resolve against. Nothing here runs
// a real process.

type fakeExecLaunch struct {
	calls []([]string)
}

func fakeLauncherConfigExec(t *testing.T) *fakeExecLaunch {
	t.Helper()
	fake := &fakeExecLaunch{}
	previous := exec.SetExecCustomExecutableFn(
		func(executable string, gui bool, wait bool, show bool, executableWorkingPath bool, stdout io.Writer, stderr io.Writer, arg ...string) (error, *osExec.Cmd) {
			fake.calls = append(fake.calls, arg)
			// standardExec reads cmd.ProcessState when ExitCode is set, so the
			// command must be non-nil even though nothing ran.
			return nil, &osExec.Cmd{}
		},
	)
	t.Cleanup(func() { exec.SetExecCustomExecutableFn(previous) })
	// The revert store is process-global and shared with the real revert flow:
	// point it at a private file so the runs under test cannot leave revert
	// arguments behind for another test.
	originalStore := launcherCommon.RevertConfigStore
	launcherCommon.RevertConfigStore = launcherCommon.NewArgsStore(filepath.Join(t.TempDir(), "config_revert.txt"))
	t.Cleanup(func() { launcherCommon.RevertConfigStore = originalStore })
	common.ClearDNSCache()
	t.Cleanup(common.ClearDNSCache)
	return fake
}

// The server pass of a session with a separate API server must tell the
// elevated side to map only the root's half: sending the whole set would write
// the API domains back at the root's address, overwriting the API pass.
func TestMapHostsSeparateApiSendsRootOnly(t *testing.T) {
	fake := fakeLauncherConfigExec(t)
	ip := "192.0.2.10"
	c := &Config{Report: launcher.Discard{}}
	if exitCode := c.MapHosts(game.AoE2, ip, false, true, true, true); exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("launcher-config ran %d times, want 1", len(fake.calls))
	}
	args := fake.calls[0]
	if !slices.Contains(args, "--rootOnly") {
		t.Errorf("args = %v, want --rootOnly", args)
	}
	if slices.Contains(args, "--api") {
		t.Errorf("args = %v, want no --api (this pass maps the root's half, not the API's)", args)
	}
	// The half it caches is the root's: the later AddApiCACertToGame decision
	// and any other resolver must see the root domains at this address.
	rootDomain := common.RootHosts(game.AoE2, false)[0]
	if got := common.HostOrIpToIps(rootDomain); !slices.Equal(got, []string{ip}) {
		t.Errorf("HostOrIpToIps(%s) = %v, want [%s]", rootDomain, got, ip)
	}
}

// The API pass of a session with a separate API server must tell the elevated
// side to map only the API domains: it runs before the server even exists, so
// mapping the whole set would reach for domains nothing answers yet.
func TestMapApiHostsSendsApiOnly(t *testing.T) {
	fake := fakeLauncherConfigExec(t)
	ip := "192.0.2.10"
	c := &Config{Report: launcher.Discard{}}
	if exitCode := c.MapApiHosts(game.AoE2, ip, false, true, true); exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("launcher-config ran %d times, want 1", len(fake.calls))
	}
	args := fake.calls[0]
	if !slices.Contains(args, "--api") {
		t.Errorf("args = %v, want --api", args)
	}
	if slices.Contains(args, "--rootOnly") {
		t.Errorf("args = %v, want no --rootOnly (this pass maps the API's half)", args)
	}
	apiDomain := common.ApiHosts(game.AoE2, false)[0]
	if got := common.HostOrIpToIps(apiDomain); !slices.Equal(got, []string{ip}) {
		t.Errorf("HostOrIpToIps(%s) = %v, want [%s]", apiDomain, got, ip)
	}
}

// A session without a separate API server maps every domain in one pass: no
// half-set flag may reach the elevated side, or it would map only part of the
// game's domains.
func TestMapHostsTogetherSendsNeitherHalfFlag(t *testing.T) {
	fake := fakeLauncherConfigExec(t)
	ip := "192.0.2.10"
	c := &Config{Report: launcher.Discard{}}
	if exitCode := c.MapHosts(game.AoE2, ip, false, true, true, false); exitCode != common.ErrSuccess {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("launcher-config ran %d times, want 1", len(fake.calls))
	}
	args := fake.calls[0]
	if slices.Contains(args, "--api") || slices.Contains(args, "--rootOnly") {
		t.Errorf("args = %v, want neither --api nor --rootOnly", args)
	}
	for _, domain := range []string{
		common.ApiHosts(game.AoE2, false)[0],
		common.RootHosts(game.AoE2, false)[0],
	} {
		if got := common.HostOrIpToIps(domain); !slices.Equal(got, []string{ip}) {
			t.Errorf("HostOrIpToIps(%s) = %v, want [%s]", domain, got, ip)
		}
	}
}
