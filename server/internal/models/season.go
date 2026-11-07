package models

import (
	"time"

	"github.com/luskaner/ageServer/common/game"
)

const (
	// RankedSeasonDuration is how long a synthetic ranked season lasts.
	// Official AoE4 seasons have run between 78 and 167 days; 90 keeps the
	// "roughly three months" cadence the official service shows.
	RankedSeasonDuration = 90 * 24 * time.Hour

	// PlacementMatches is how many games a ranked season requires before the
	// visible rating is shown as a rank. It mirrors placementMatch in the
	// season metadata served by getAvailableCommunityEvents.
	PlacementMatches = 5
)

// rankedSeasonAnchor is the start of the first official AoE4 ranked season
// (Season 1 - Festival of Ages, 2022-04-13). Season windows are derived from
// it so the number grows monotonically and the current window always covers
// "now".
var rankedSeasonAnchor = time.Date(2022, time.April, 13, 0, 0, 0, 0, time.UTC)

// RankedSeason is the window the server reports as the active ranked season.
type RankedSeason struct {
	Number int
	Start  time.Time
	End    time.Time
}

// CurrentRankedSeason resolves the season that contains now. Seasons are
// contiguous: each one starts where the previous ended, so there is always
// exactly one active season.
func CurrentRankedSeason(now time.Time) RankedSeason {
	elapsed := now.Sub(rankedSeasonAnchor)
	if elapsed < 0 {
		elapsed = 0
	}
	index := int(elapsed / RankedSeasonDuration)
	start := rankedSeasonAnchor.Add(time.Duration(index) * RankedSeasonDuration)
	return RankedSeason{
		Number: index + 1,
		Start:  start,
		End:    start.Add(RankedSeasonDuration),
	}
}

// Previous returns the season that ended when this one started. Season 1 has
// no previous season, so the result is not meaningful for it.
func (s RankedSeason) Previous() RankedSeason {
	return RankedSeason{
		Number: s.Number - 1,
		Start:  s.Start.Add(-RankedSeasonDuration),
		End:    s.Start,
	}
}

// GameHasRankedSeasons reports whether a title resets its ladders every season.
// AoE4 is the only title whose client exposes a ranked season; the rest keep a
// single, permanent ladder (season 0).
func GameHasRankedSeasons(gameID string) bool {
	return gameID == game.AoE4
}
