package communityEvent

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

func testMenuBackgroundCatalog() []models.MenuBackgroundDefinition {
	return []models.MenuBackgroundDefinition{
		{ID: 100, Name: "First", FrontEndImagePath: `\Images\backgrounds\first.png`},
		{ID: 101, Name: "Second", FrontEndImagePath: `\Images\backgrounds\second.png`},
		{ID: 102, Name: "Third", FrontEndImagePath: `\Images\backgrounds\third.png`},
	}
}

// TestScheduledMenuBackgroundsPublishesStartedPeriods covers the publish rules:
// every period that already started is published, a period in the future is
// not, the active period comes first and the rest follow by event id.
func TestScheduledMenuBackgroundsPublishesStartedPeriods(t *testing.T) {
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	schedule := []models.MenuBackgroundWindow{
		{Image: 100, Event: 900100, Name: "Ended", Start: now.AddDate(0, 0, -40), End: now.AddDate(0, 0, -20)},
		{Image: 101, Event: 5, Name: "Active", Start: now.Add(-time.Hour), End: now.Add(time.Hour)},
		{Image: 102, Event: 7, Name: "Future", Start: now.Add(time.Hour), End: now.Add(2 * time.Hour)},
	}
	events := scheduledMenuBackgrounds(schedule, testMenuBackgroundCatalog(), now)
	if len(events) != 2 {
		t.Fatalf("events = %d, want the active and the ended period only", len(events))
	}
	if name, _ := events[0][0].(string); name != "Active" {
		t.Fatalf("first event = %q, want the active period first", name)
	}
	if id, _ := events[1][4].(int32); id != 900100 {
		t.Fatalf("second event id = %v, want the ended period", events[1][4])
	}
}

// TestScheduledMenuBackgroundsOrdersTheHistoryByEventID checks that the
// non-active history is served ascending by event id, not in schedule order.
func TestScheduledMenuBackgroundsOrdersTheHistoryByEventID(t *testing.T) {
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	schedule := []models.MenuBackgroundWindow{
		{Image: 102, Event: 900102, Start: now.AddDate(0, 0, -10), End: now.AddDate(0, 0, -5)},
		{Image: 100, Event: 900100, Start: now.AddDate(0, 0, -40), End: now.AddDate(0, 0, -20)},
	}
	events := scheduledMenuBackgrounds(schedule, testMenuBackgroundCatalog(), now)
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if events[0][4] != int32(900100) || events[1][4] != int32(900102) {
		t.Fatalf("event ids = %v, %v; want 900100, 900102", events[0][4], events[1][4])
	}
}

func TestScheduledMenuBackgroundsEmptySchedulePublishesNothing(t *testing.T) {
	if events := scheduledMenuBackgrounds(nil, testMenuBackgroundCatalog(), time.Now()); len(events) != 0 {
		t.Fatalf("events = %#v, want none", events)
	}
}

// TestScheduledMenuBackgroundEventMatchesOfficialShape pins the contract the
// client parses: 8 fields, an empty description, customdata with only the image
// path and the far-future expiry sentinel.
func TestScheduledMenuBackgroundEventMatchesOfficialShape(t *testing.T) {
	now := time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)
	schedule := []models.MenuBackgroundWindow{
		{Image: 100, Event: 290, Name: "Trade Secrets", Start: now.Add(-time.Hour), End: now.Add(time.Hour)},
	}
	events := scheduledMenuBackgrounds(schedule, testMenuBackgroundCatalog(), now)
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	event := events[0]
	want := i.A{
		"Trade Secrets",
		schedule[0].Start.Unix(),
		schedule[0].End.Unix(),
		seasonEventFarFuture,
		int32(290),
		menuBackgroundEventType,
		"",
		`{"frontEndImagePath":"\\Images\\backgrounds\\first.png"}`,
	}
	if len(event) != len(want) {
		t.Fatalf("event has %d fields, want %d", len(event), len(want))
	}
	for index := range want {
		if event[index] != want[index] {
			t.Fatalf("event field %d = %#v, want %#v", index, event[index], want[index])
		}
	}
}

// TestMenuBackgroundsKeepTheRankedSeason asserts the only invariant that can
// break the game: the background events are type 0, the ranked season events are
// still served, and the two id sets are disjoint.
func TestMenuBackgroundsKeepTheRankedSeason(t *testing.T) {
	now := time.Date(2026, time.November, 7, 1, 0, 0, 0, time.UTC)
	response := aoe4CommunityEvents(bundledSeasonDefinitions(t), now)
	backgrounds := &models.MenuBackgrounds{
		Images: testMenuBackgroundCatalog(),
		Schedule: []models.MenuBackgroundWindow{
			{Image: 100, Event: 290, Name: "Current", Start: now.Add(-time.Hour), End: now.Add(time.Hour)},
		},
	}
	response = withMenuBackgrounds(response, backgrounds, now)
	events, ok := response[1].([]any)
	if !ok {
		t.Fatalf("events block = %#v", response[1])
	}
	if first, ok := events[0].(i.A); !ok || first[0] != "Current" {
		t.Fatalf("first event = %#v, want the active background", events[0])
	}
	var backgroundIDs, seasonIDs []int32
	for _, raw := range events {
		event, ok := raw.(i.A)
		if !ok || len(event) != 8 {
			t.Fatalf("event = %#v, want 8 fields", raw)
		}
		id, _ := event[4].(int32)
		switch event[5] {
		case menuBackgroundEventType:
			backgroundIDs = append(backgroundIDs, id)
		case seasonEventType:
			seasonIDs = append(seasonIDs, id)
		default:
			t.Fatalf("unexpected event type %v", event[5])
		}
	}
	if len(backgroundIDs) != 1 || len(seasonIDs) == 0 {
		t.Fatalf("background ids = %v, season ids = %v", backgroundIDs, seasonIDs)
	}
	slices.Sort(seasonIDs)
	for _, id := range backgroundIDs {
		if slices.Contains(seasonIDs, id) {
			t.Fatalf("background id %d collides with a ranked season id", id)
		}
	}
}

// TestWithMenuBackgroundsWithoutConfigurationChangesNothing covers "absent or
// empty configuration means no behaviour change".
func TestWithMenuBackgroundsWithoutConfigurationChangesNothing(t *testing.T) {
	now := time.Now()
	unchanged := aoe4CommunityEvents(bundledSeasonDefinitions(t), now)
	want, _ := json.Marshal(unchanged)
	cases := map[string]*models.MenuBackgrounds{
		"no resources":   nil,
		"empty schedule": {Images: testMenuBackgroundCatalog()},
		"future schedule": {Images: testMenuBackgroundCatalog(), Schedule: []models.MenuBackgroundWindow{{
			Image: 100, Event: 900100, Start: now.Add(time.Hour), End: now.Add(2 * time.Hour),
		}}},
	}
	for name, backgrounds := range cases {
		t.Run(name, func(t *testing.T) {
			got, _ := json.Marshal(withMenuBackgrounds(aoe4CommunityEvents(bundledSeasonDefinitions(t), now), backgrounds, now))
			if string(got) != string(want) {
				t.Fatalf("response changed without a published window:\n got %s\nwant %s", got, want)
			}
		})
	}
}
