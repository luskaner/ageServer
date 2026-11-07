package models

import (
	"testing"
	"time"
)

func TestCurrentRankedSeasonIsAlwaysActive(t *testing.T) {
	cases := []time.Time{
		rankedSeasonAnchor,                  // anchor
		rankedSeasonAnchor.Add(time.Second), // just after
		time.Date(2025, time.October, 29, 23, 59, 59, 0, time.UTC),
		time.Date(2026, time.November, 7, 1, 0, 0, 0, time.UTC),
		time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, now := range cases {
		season := CurrentRankedSeason(now)
		if season.Number < 1 {
			t.Fatalf("%s: season number = %d", now, season.Number)
		}
		if now.Before(season.Start) || !now.Before(season.End) {
			t.Fatalf("%s is outside its own season %d [%s, %s)", now, season.Number, season.Start, season.End)
		}
		if season.End.Sub(season.Start) != RankedSeasonDuration {
			t.Fatalf("season %d duration = %s", season.Number, season.End.Sub(season.Start))
		}
		if season.Number == 1 {
			continue
		}
		// The previous season must end exactly where this one starts.
		previous := CurrentRankedSeason(season.Start.Add(-time.Second))
		if !previous.End.Equal(season.Start) {
			t.Fatalf("season %d starts at %s but the previous one ends at %s", season.Number, season.Start, previous.End)
		}
		if previous.Number+1 != season.Number {
			t.Fatalf("season numbers are not contiguous: %d then %d", previous.Number, season.Number)
		}
	}
}

func TestCurrentRankedSeasonNumberGrows(t *testing.T) {
	first := CurrentRankedSeason(rankedSeasonAnchor)
	later := CurrentRankedSeason(rankedSeasonAnchor.Add(365 * 24 * time.Hour))
	if later.Number <= first.Number {
		t.Fatalf("season number did not grow: %d then %d", first.Number, later.Number)
	}
}

func TestGameHasRankedSeasons(t *testing.T) {
	if !GameHasRankedSeasons("age4") {
		t.Fatal("age4 must reset its ladders per season")
	}
	for _, gameID := range []string{"age1", "age2", "age3", "athens"} {
		if GameHasRankedSeasons(gameID) {
			t.Fatalf("%s must keep a single permanent ladder", gameID)
		}
	}
}
