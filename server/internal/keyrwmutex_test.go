package internal

import (
	"sync"
	"testing"
)

// Regression: getOrCreateLock had a TOCTOU race — two goroutines could create
// separate mutexes for the same key, breaking mutual exclusion.
func TestKeyRWMutexConcurrentSameKey(t *testing.T) {
	kl := NewKeyRWMutex[string]()
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	var mu sync.Mutex // Protects the counter itself
	counter := 0

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				kl.Lock("shared-key")
				mu.Lock()
				counter++
				mu.Unlock()
				kl.Unlock("shared-key")
			}
		}()
	}
	wg.Wait()

	if counter != goroutines*iterations {
		t.Fatalf("counter = %d, want %d (mutual exclusion broken)", counter, goroutines*iterations)
	}
}

func TestKeyRWMutexDifferentKeysIndependent(t *testing.T) {
	kl := NewKeyRWMutex[string]()
	kl.Lock("a")
	kl.Lock("b") // Must not block (different key)
	kl.Unlock("b")
	kl.Unlock("a")
}

func TestKeyRWMutexSequentialLockUnlock(t *testing.T) {
	kl := NewKeyRWMutex[int]()
	kl.Lock(42)
	kl.Unlock(42)
	// Re-lock after unlock must work (not deadlock).
	kl.Lock(42)
	kl.Unlock(42)
}
