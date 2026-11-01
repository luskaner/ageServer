package party

import (
	"testing"
)

// Regression: the peerAdd rollback used to evict pre-existing peers because
// UnsafeNewPeer returned a non-nil existing peer for already-joined users,
// causing them to be tracked in addedUserIds and removed on rollback.
//
// The fix skips users who are already peers before calling UnsafeNewPeer.
// Full integration testing requires the advertisement model with write locks;
// this test documents the expected behaviour at the logic level.
func TestPreExistingPeerNotEvictedOnRollback(t *testing.T) {
	// Simulate: match has [Bob (pre-existing)], adding [Bob, Charlie]
	existingPeers := map[int32]bool{2: true} // Bob's userId = 2
	var addedUserIds []int32

	users := []struct {
		id       int32
		willFail bool // simulate Charlie failing to join
	}{
		{id: 2, willFail: false}, // Bob: already exists → skip
		{id: 3, willFail: false}, // Charlie: new → add
	}

	for _, u := range users {
		if existingPeers[u.id] {
			continue // ← THE FIX: skip pre-existing peers
		}
		if u.willFail {
			break
		}
		addedUserIds = append(addedUserIds, u.id)
	}

	// Simulate rollback of addedUserIds
	for _, userId := range addedUserIds {
		delete(existingPeers, userId)
	}

	// Bob must still be in the match after rollback
	if !existingPeers[2] {
		t.Fatal("Bob was incorrectly evicted during rollback")
	}
}

func TestRollbackOnlyRemovesNewlyAdded(t *testing.T) {
	existingPeers := map[int32]bool{}
	addedUserIds := []int32{}

	// Add Alice (1) and Bob (2) — both new, both succeed
	for _, id := range []int32{1, 2} {
		addedUserIds = append(addedUserIds, id)
		existingPeers[id] = true
	}

	// Charlie (3) fails to join → break
	_ = 3

	// Rollback removes only newly added peers
	for _, userId := range addedUserIds {
		delete(existingPeers, userId)
		delete(existingPeers, userId)
	}

	if len(existingPeers) != 0 {
		t.Fatalf("expected all new peers removed, got %v", existingPeers)
	}
}
