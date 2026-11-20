package communityEvent

import (
	"slices"
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
func scheduledMenuBackgrounds(periods []models.MenuBackgroundPeriod, now time.Time) i.A {
	if len(periods) == 0 {
		return nil
	}
	started := make([]models.MenuBackgroundPeriod, 0, len(periods))
	for _, period := range periods {
		// A period is published as soon as it starts and stays published after
		// it ends, exactly like the official service keeps its past periods.
		if period.Start.After(now) {
			continue
		}
		started = append(started, period)
	}
	sort.SliceStable(started, func(left, right int) bool {
		leftActive := menuBackgroundActive(started[left], now)
		rightActive := menuBackgroundActive(started[right], now)
		if leftActive != rightActive {
			return leftActive
		}
		return started[left].Event < started[right].Event
	})
	events := make(i.A, 0, len(started))
	for _, period := range started {
		events = append(events, i.A{
			period.Name,
			period.Start.Unix(),
			period.End.Unix(),
			seasonEventFarFuture,
			period.Event,
			menuBackgroundEventType,
			"",
			period.CustomData,
		})
	}
	return events
}

// menuBackgroundActive reports whether now falls inside the half-open window
// [start, end).
func menuBackgroundActive(period models.MenuBackgroundPeriod, now time.Time) bool {
	return !now.Before(period.Start) && now.Before(period.End)
}

// withMenuBackgrounds prepends the configured background events to an AoE4
// community-event response, leaving the ranked season events untouched.
func withMenuBackgrounds(response i.A, backgrounds *models.MenuBackgrounds, now time.Time) i.A {
	if backgrounds == nil || len(response) < 2 {
		return response
	}
	events := scheduledMenuBackgrounds(backgrounds.Periods, now)
	if len(events) == 0 {
		return response
	}
	existing, ok := response[1].([]any)
	if !ok {
		return response
	}
	// existing is sized by whatever the caller put in the response, so adding
	// len(events)+len(existing) for the capacity is an allocation CodeQL reads
	// as overflow-prone. slices.Concat concatenates without that arithmetic.
	response[1] = slices.Concat(events, existing)
	return response
}
