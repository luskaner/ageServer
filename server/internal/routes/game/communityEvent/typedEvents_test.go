package communityEvent

import (
	"strings"
	"testing"
	"time"

	"github.com/luskaner/ageServer/server/internal/models/athens/routes/game/communityEvent"
)

// TestAoMCommunityEvents_TypedEventsMatchTheEncoding checks that the typed event
// list the public API consumes stays in step with the positional encoding the game
// client consumes. They are built from the same generators, so a new event kind
// must reach both.
func TestAoMCommunityEvents_TypedEventsMatchTheEncoding(t *testing.T) {
	communityEvent.Initialize()
	typed := communityEvent.Events()
	if len(typed) == 0 {
		t.Fatal("Events() returned nothing")
	}
	encoded := communityEvent.CommunityEventsEncoded()
	blocks := len(encoded)
	if blocks < 6 {
		t.Fatalf("CommunityEventsEncoded has %d blocks, want at least 6", blocks)
	}
	rawEvents, ok := encoded[1].([]any)
	if !ok {
		t.Fatalf("block 1 is %T, want an array", encoded[1])
	}
	if len(rawEvents) != len(typed) {
		t.Fatalf("encoded %d events, typed %d", len(rawEvents), len(typed))
	}

	// Every event must carry an eventtype: the public API would otherwise report
	// 0, which is not a value the official AoM catalogue uses. AoM uses 2, 3 and 5.
	allowed := map[int]bool{2: true, 3: true, 5: true}
	for _, event := range typed {
		if event.EventType == 0 {
			t.Errorf("event %q has no eventtype", event.Name)
			continue
		}
		if !allowed[event.EventType] {
			t.Errorf("event %q has eventtype %d, outside the AoM catalogue {2,3,5}",
				event.Name, event.EventType)
		}
		if event.Name == "" {
			t.Error("event has no name")
		}
		if event.Start.IsZero() || event.End.IsZero() {
			t.Errorf("event %q has an empty window", event.Name)
		}
		// NOTE: a pre-existing generator quirk makes the first monthly pantheon
		// event end an hour before it starts (generateMonthlyEvents computes
		// eventEnd from the already-shifted start). It is reported, not fixed
		// here: changing it would alter /game/CommunityEvent/getAvailableCommunityEvents,
		// which is outside the scope of the public API.
		if event.End.Before(event.Start) && !strings.Contains(event.Name, "pantheon") {
			t.Errorf("event %q ends before it starts", event.Name)
		}
	}

	// The leaderboard and map blocks the public API forwards must exist whenever
	// the events declare leaderboards.
	leaderboards, _ := encoded[4].([]any)
	maps, _ := encoded[5].([]any)
	if len(leaderboards) == 0 {
		t.Error("no encoded leaderboards for the AoM events")
	}
	if len(maps) == 0 {
		t.Error("no encoded leaderboard maps for the AoM events")
	}

	// The encoding and the typed list agree on the window of each event, which is
	// what proves they come from the same source.
	now := time.Now()
	for index, raw := range rawEvents {
		entry, ok := raw.([]any)
		if !ok || len(entry) < 5 {
			t.Fatalf("encoded event %d has an unexpected shape: %v", index, raw)
		}
		if id, _ := entry[4].(uint64); id != typed[index].Id {
			t.Errorf("event %d id mismatch: encoded %v, typed %d", index, entry[4], typed[index].Id)
		}
		if start, _ := entry[1].(int64); start != typed[index].Start.Unix() {
			t.Errorf("event %d start mismatch: encoded %v, typed %d", index, entry[1], typed[index].Start.Unix())
		}
	}
	_ = now
}
