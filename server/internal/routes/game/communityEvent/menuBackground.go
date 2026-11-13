package communityEvent

import (
	"encoding/json"
	"sort"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// menuBackgroundEventType is the community-event type the AoE4 client reads
// the main-menu background from: a type-0 event whose customdata carries a
// frontEndImagePath.
const menuBackgroundEventType = 0

// scheduledMenuBackgrounds returns one event per configured period that has
// already started, mirroring the official service: the full history, never a
// window in the future, active periods first, then the rest by event id.
func scheduledMenuBackgrounds(schedule []models.MenuBackgroundWindow,
	catalog []models.MenuBackgroundDefinition, now time.Time) []i.A {
	if len(schedule) == 0 {
		return nil
	}
	paths := make(map[int]string, len(catalog))
	for _, definition := range catalog {
		paths[definition.ID] = definition.FrontEndImagePath
	}
	started := make([]models.MenuBackgroundWindow, 0, len(schedule))
	for _, window := range schedule {
		// A period is published as soon as it starts and stays published after
		// it ends, exactly like the official service keeps its past periods.
		if window.Start.After(now) {
			continue
		}
		started = append(started, window)
	}
	sort.SliceStable(started, func(left, right int) bool {
		leftActive := menuBackgroundActive(started[left], now)
		rightActive := menuBackgroundActive(started[right], now)
		if leftActive != rightActive {
			return leftActive
		}
		return started[left].Event < started[right].Event
	})
	events := make([]i.A, 0, len(started))
	for _, window := range started {
		metadata, err := json.Marshal(map[string]string{
			"frontEndImagePath": paths[window.Image],
		})
		if err != nil {
			continue
		}
		events = append(events, i.A{
			window.Name,
			window.Start.Unix(),
			window.End.Unix(),
			seasonEventFarFuture,
			window.Event,
			menuBackgroundEventType,
			"",
			string(metadata),
		})
	}
	return events
}

// menuBackgroundActive reports whether now falls inside the half-open window
// [start, end).
func menuBackgroundActive(window models.MenuBackgroundWindow, now time.Time) bool {
	return !now.Before(window.Start) && now.Before(window.End)
}

// withMenuBackgrounds prepends the configured background events to an AoE4
// community-event response, leaving the ranked season events untouched.
func withMenuBackgrounds(response i.A, backgrounds *models.MenuBackgrounds, now time.Time) i.A {
	if backgrounds == nil || len(response) < 2 {
		return response
	}
	events := scheduledMenuBackgrounds(backgrounds.Schedule, backgrounds.Images, now)
	if len(events) == 0 {
		return response
	}
	existing, ok := response[1].([]any)
	if !ok {
		return response
	}
	merged := make(i.A, 0, len(events)+len(existing))
	for _, event := range events {
		merged = append(merged, event)
	}
	merged = append(merged, existing...)
	response[1] = merged
	return response
}
