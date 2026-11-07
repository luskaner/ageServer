package models

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/luskaner/ageServer/common/game"
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/routes/game/advertisement/shared"
)

// encodeObservation is one advertisement as the observable/general listing
// renders it, with the fields the timer depends on.
type encodeObservation struct {
	ID        int32
	State     int8
	StartTime *int64
	Visible   bool
}

func observeEncoded(adv Advertisement) encodeObservation {
	encoded := adv.UnsafeEncode(game.AoE2, 0, nil)
	raw, err := json.Marshal(encoded)
	if err != nil {
		panic(fmt.Sprintf("encode advertisement: %v", err))
	}
	var fields []json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		panic(fmt.Sprintf("decode advertisement: %v", err))
	}
	// Slot layout checked against the official captures: 7 = match state,
	// 24 = match start time (null until the match starts).
	var obs encodeObservation
	_ = json.Unmarshal(fields[0], &obs.ID)
	_ = json.Unmarshal(fields[7], &obs.State)
	if string(fields[24]) != "null" {
		var start int64
		if err = json.Unmarshal(fields[24], &start); err == nil {
			obs.StartTime = &start
		}
	}
	return obs
}

// TestObservableNeedsStartedState documents the rule taken from the official
// traffic: findObservableAdvertisements only lists advertisements whose match
// has already started.
//
// Every advertisement captured from that route carried state=1 and a start time
// (2523 in AoE4, 1602 in AoM, 76 in AoE3), while /findAdvertisements only ever
// returned state=0 rows with a null start time. The route keeps its own filter
// on top of the generic one; this test pins the property that filter relies on.
func TestObservableNeedsStartedState(t *testing.T) {
	i.InitializeRng(42)
	observableAndStarted := func(adv Advertisement) bool {
		return adv.UnsafeGetObserversEnabled() && adv.UnsafeGetState() == 1
	}

	adv := &MainAdvertisement{
		id:       1,
		tags:     Tags{integer: map[string]int32{}, text: map[string]string{}},
		peers:    i.NewSafeOrderedMap[int32, Peer](),
		lan:      true,
		visible:  true,
		joinable: true,
	}
	adv.UnsafeUpdate(&shared.AdvertisementUpdateRequest{
		Observable: true,
		Visible:    true,
		Joinable:   true,
		State:      0,
	})

	// A lobby that has not started is not listed as observable.
	if observableAndStarted(adv) {
		t.Fatal("state=0 lobby listed as observable, want filtered out")
	}
	if obs := observeEncoded(adv); obs.StartTime != nil {
		t.Fatalf("state=0 advertisement encoded startTime=%d, want null", *obs.StartTime)
	}

	// Once the host announces the start it is listed, with a start time.
	adv.UnsafeUpdateState(1)
	if !observableAndStarted(adv) {
		t.Fatal("started observable match not listed")
	}
	obs := observeEncoded(adv)
	if obs.State != 1 {
		t.Fatalf("encoded state = %d, want 1", obs.State)
	}
	if obs.StartTime == nil || *obs.StartTime <= 0 {
		t.Fatal("started match encoded without a start time")
	}

	// The client keeps sending state=0 in /advertisement/update afterwards; the
	// match must stay listed and keep the same start time (issue #403).
	adv.UnsafeUpdate(&shared.AdvertisementUpdateRequest{
		Observable: true,
		Visible:    false,
		Joinable:   false,
		State:      0,
	})
	if !observableAndStarted(adv) {
		t.Fatal("started match dropped from the observable list after /advertisement/update")
	}
	after := observeEncoded(adv)
	if after.StartTime == nil || *after.StartTime != *obs.StartTime {
		t.Fatalf("startTime changed after update: %v -> %v", obs.StartTime, after.StartTime)
	}
}

// TestNotObservableNeverListed keeps the observer flag as a hard requirement.
func TestNotObservableNeverListed(t *testing.T) {
	i.InitializeRng(42)
	adv := &MainAdvertisement{
		id:    2,
		tags:  Tags{integer: map[string]int32{}, text: map[string]string{}},
		peers: i.NewSafeOrderedMap[int32, Peer](),
		lan:   true,
	}
	adv.UnsafeUpdate(&shared.AdvertisementUpdateRequest{Observable: false, State: 0})
	adv.UnsafeUpdateState(1)

	if adv.UnsafeGetObserversEnabled() {
		t.Fatal("observers enabled, want disabled")
	}
	if adv.UnsafeGetState() != 1 {
		t.Fatalf("state = %d, want 1", adv.UnsafeGetState())
	}
}
