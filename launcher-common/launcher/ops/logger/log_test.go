package logger

import (
	"sync"
	"testing"
)

// Regression: allHosts, Cacert, BasePath and MacOsExclusiveMappings were
// package-level variables written by the main flow and read concurrently by
// the signal goroutine via WriteFileLog, causing a data race on Ctrl+C.
func TestConcurrentStateAccess(t *testing.T) {
	var wg sync.WaitGroup

	// Concurrent writers (simulating the setup flow).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			SetBasePath("/some/path")
			SetMacOsExclusiveMappings(i%2 == 0)
			SetCacert(nil)
		}
	}()

	// Concurrent readers (simulating signal-triggered Revert → WriteFileLog).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = GetBasePath()
			state.mu.RLock()
			_ = state.Cacert
			_ = state.MacOsExclusiveMappings
			state.mu.RUnlock()
		}
	}()

	wg.Wait()
}

func TestSettersAndGetters(t *testing.T) {
	SetBasePath("/test/path")
	if got := GetBasePath(); got != "/test/path" {
		t.Fatalf("BasePath = %q, want %q", got, "/test/path")
	}

	SetCacert(nil)
	state.mu.RLock()
	nilCert := state.Cacert == nil
	state.mu.RUnlock()
	if !nilCert {
		t.Fatal("SetCacert(nil) should clear the cert")
	}

	SetMacOsExclusiveMappings(true)
	state.mu.RLock()
	mac := state.MacOsExclusiveMappings
	state.mu.RUnlock()
	if !mac {
		t.Fatal("SetMacOsExclusiveMappings(true) should set the flag")
	}
}
