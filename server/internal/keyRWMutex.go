package internal

import "sync"

type KeyRWMutex[K comparable] struct {
	mu      sync.RWMutex
	mutexes map[K]*sync.RWMutex
}

func NewKeyRWMutex[K comparable]() *KeyRWMutex[K] {
	return &KeyRWMutex[K]{
		mutexes: make(map[K]*sync.RWMutex),
	}
}

func (kl *KeyRWMutex[K]) Lock(key K) {
	kl.getOrCreateLock(key).Lock()
}

func (kl *KeyRWMutex[K]) RLock(key K) {
	kl.getOrCreateLock(key).RLock()
}

func (kl *KeyRWMutex[K]) Unlock(key K) {
	kl.mu.RLock()
	lock, ok := kl.mutexes[key]
	kl.mu.RUnlock()
	if ok {
		lock.Unlock()
	}
}

func (kl *KeyRWMutex[K]) RUnlock(key K) {
	kl.mu.RLock()
	lock, ok := kl.mutexes[key]
	kl.mu.RUnlock()
	if ok {
		lock.RUnlock()
	}
}

func (kl *KeyRWMutex[K]) getOrCreateLock(key K) *sync.RWMutex {
	kl.mu.RLock()
	lock, ok := kl.mutexes[key]
	kl.mu.RUnlock()
	if ok {
		return lock
	}
	kl.mu.Lock()
	defer kl.mu.Unlock()
	// Double-check: another goroutine may have inserted this key while we
	// were waiting for the write lock.
	if lock, ok := kl.mutexes[key]; ok {
		return lock
	}
	newLock := &sync.RWMutex{}
	kl.mutexes[key] = newLock
	return newLock
}
