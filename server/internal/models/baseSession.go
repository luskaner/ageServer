package models

import (
	"iter"
	"slices"
	"sync"
	"time"

	"github.com/luskaner/ageServer/server/internal"
)

func newExpiry(duration time.Duration) time.Time {
	return time.Now().UTC().Add(duration)
}

type BaseSession[T comparable, D any] struct {
	id         T
	expiryLock sync.RWMutex
	expiry     time.Time
	data       *D
}

func (session *BaseSession[T, D]) Expiry() time.Time {
	return session.expiry
}

func (session *BaseSession[T, D]) Id() T {
	return session.id
}

func (session *BaseSession[T, D]) Data() *D {
	return session.data
}

type BaseSessions[T comparable, D any] struct {
	expiry             time.Duration
	store              *internal.SafeMap[T, *BaseSession[T, D]]
	sweeperTaskMu      sync.Mutex
	sweeperTaskStarted bool
}

func (sessions *BaseSessions[T, D]) CreateSession(idGenFun func() T, data *D) (stored *BaseSession[T, D]) {
	exp := newExpiry(sessions.expiry)
	for exists := true; exists; {
		info := &BaseSession[T, D]{
			id:     idGenFun(),
			expiry: exp,
			data:   data,
		}
		stored, exists = sessions.store.Store(info.id, info, func(stored *BaseSession[T, D]) bool {
			return false
		})
	}
	sessions.sweeperTaskMu.Lock()
	defer sessions.sweeperTaskMu.Unlock()
	if !sessions.sweeperTaskStarted {
		go sessions.startSweeper()
		sessions.sweeperTaskStarted = true
	}
	return stored
}

func (sessions *BaseSessions[T, D]) Get(id T) (*BaseSession[T, D], bool) {
	return sessions.store.Load(id)
}

func (sessions *BaseSessions[T, D]) Values() iter.Seq[*BaseSession[T, D]] {
	return sessions.store.Values()
}

func (sessions *BaseSessions[T, D]) Delete(id T) {
	sessions.store.Delete(id)
}

func (sessions *BaseSessions[T, D]) startSweeper() {
	go func() {
		var alreadyExpired []T
		_, nextExpiration := sessions.nextExpiration()
		ticker := time.NewTicker(nextExpiration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				alreadyExpired, nextExpiration = sessions.nextExpiration()
				ticker.Reset(nextExpiration)
				for _, expired := range alreadyExpired {
					sessions.store.Delete(expired)
				}
			}
		}
	}()
}

func (sessions *BaseSessions[T, D]) nextExpiration() (alreadyExpired []T, nextExpiration time.Duration) {
	var expirationTimes []time.Time
	now := time.Now().UTC()
	for sess := range sessions.store.Values() {
		sess.expiryLock.RLock()
		if sess.expiry.Before(now) {
			alreadyExpired = append(alreadyExpired, sess.id)
		} else {
			expirationTimes = append(expirationTimes, sess.expiry)
		}
		sess.expiryLock.RUnlock()
	}
	if len(expirationTimes) > 0 {
		slices.SortFunc(expirationTimes, func(a, b time.Time) int {
			switch {
			case a.Before(b):
				return -1
			case a.After(b):
				return 1
			default:
				return 0
			}
		})
		nextExpiration = expirationTimes[0].Sub(now)
	} else {
		nextExpiration = sessions.expiry
	}
	// Clamp to a minimum positive interval to prevent NewTicker/Reset from
	// panicking on zero or negative durations.
	const minInterval = 100 * time.Millisecond
	if nextExpiration < minInterval {
		nextExpiration = minInterval
	}
	return
}

func (sessions *BaseSessions[T, D]) ResetExpiryTimer(id T) {
	if sess, ok := sessions.Get(id); ok {
		sess.expiryLock.Lock()
		defer sess.expiryLock.Unlock()
		sess.expiry = newExpiry(sessions.expiry)
	}
}

func NewBaseSessions[T comparable, D any](expiry time.Duration) *BaseSessions[T, D] {
	return &BaseSessions[T, D]{
		store:  internal.NewSafeMap[T, *BaseSession[T, D]](),
		expiry: expiry,
	}
}
