package models

import (
	"testing"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/game/advertisement/shared"
)

// newStateTestAdvertisement builds a hostless advertisement good enough to
// exercise the state machine.
func newStateTestAdvertisement(t *testing.T, state int8) *MainAdvertisement {
	t.Helper()
	adv := &MainAdvertisement{
		id:    1,
		tags:  Tags{integer: map[string]int32{}, text: map[string]string{}},
		peers: i.NewSafeOrderedMap[int32, Peer](),
	}
	adv.UnsafeUpdate(&shared.AdvertisementUpdateRequest{State: state})
	return adv
}

// startAnnouncedAt moves the advertisement into the started state and returns
// the start time it recorded, so tests can assert it is never rewritten.
func startAnnouncedAt(t *testing.T, adv *MainAdvertisement) int64 {
	t.Helper()
	adv.UnsafeUpdateState(1)
	if adv.startTime == 0 {
		t.Fatalf("startTime was not recorded")
	}
	return adv.startTime
}

// The client keeps sending the state it captured when the lobby was created
// (always 0 in the captured traffic) in /game/advertisement/update, including
// right after announcing the start. The official service ignores it; if the
// state dropped back to 0 the advertisement would also be listed as not started
// and the next state=1 would re-stamp the start time, which is the wrong timer
// reported in issue #403.
func TestUpdateDoesNotUnstartMatch(t *testing.T) {
	i.InitializeRng(42)
	adv := newStateTestAdvertisement(t, 0)
	startTime := startAnnouncedAt(t, adv)

	adv.UnsafeUpdate(&shared.AdvertisementUpdateRequest{State: 0})

	if adv.state != 1 {
		t.Fatalf("state after update with state=0 = %d, want 1", adv.state)
	}
	if adv.startTime != startTime {
		t.Fatalf("startTime changed: %d -> %d, want stable", startTime, adv.startTime)
	}
}

// A later start announcement must not move the timer: the official never
// rewrites startTime once the match started.
func TestRepeatedStartKeepsOriginalStartTime(t *testing.T) {
	i.InitializeRng(42)
	adv := newStateTestAdvertisement(t, 0)
	first := startAnnouncedAt(t, adv)

	// Force the state back to 0 the way a legacy/older client would, then
	// announce the start again.
	adv.state = 0
	adv.UnsafeUpdateState(1)

	if adv.startTime != first {
		t.Fatalf("startTime re-stamped: %d -> %d, want %d", first, adv.startTime, first)
	}
	if adv.state != 1 {
		t.Fatalf("state = %d, want 1", adv.state)
	}
}

// The started state still hides the advertisement from the lobby list and drops
// observers, independently of the timer being kept.
func TestStartHidesAdvertisementAndClearsObservers(t *testing.T) {
	i.InitializeRng(42)
	adv := newStateTestAdvertisement(t, 0)
	adv.visible = true
	adv.joinable = true
	adv.StartObserving(7)

	startAnnouncedAt(t, adv)

	if adv.visible {
		t.Fatal("visible = true after start, want false")
	}
	if adv.joinable {
		t.Fatal("joinable = true after start, want false")
	}
	if adv.observers.userIds.Len() != 0 {
		t.Fatalf("observers = %d after start, want 0", adv.observers.userIds.Len())
	}
}

// Before the match starts the state keeps flowing, so hosting and joining still
// work: only the transition into the started state is one-way.
func TestUpdateKeepsStateBeforeStart(t *testing.T) {
	i.InitializeRng(42)
	adv := newStateTestAdvertisement(t, 0)
	if adv.startTime != 0 {
		t.Fatalf("startTime = %d before start, want 0", adv.startTime)
	}
	if adv.state != 0 {
		t.Fatalf("state = %d before start, want 0", adv.state)
	}
}

// A state=1 arriving through /game/advertisement/update (clients that start the
// match without calling updateState) must still start the match.
func TestUpdateCanStartMatch(t *testing.T) {
	i.InitializeRng(42)
	adv := newStateTestAdvertisement(t, 0)

	adv.UnsafeUpdate(&shared.AdvertisementUpdateRequest{State: 1})

	if adv.state != 1 {
		t.Fatalf("state = %d after update with state=1, want 1", adv.state)
	}
	if adv.startTime == 0 {
		t.Fatal("startTime = 0 after update with state=1, want recorded")
	}
	if adv.startTime > time.Now().UTC().Unix() {
		t.Fatalf("startTime = %d in the future", adv.startTime)
	}
}
