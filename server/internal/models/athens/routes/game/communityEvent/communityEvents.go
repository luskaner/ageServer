package communityEvent

import (
	"fmt"
	"math"
	"strings"
	"time"

	_ "time/tzdata"

	i "github.com/luskaner/ageServer/server/internal"
)

const dayDuration = 24 * time.Hour
const leaderboardName = "skirmish_plus_leaderboard_38 "

var dailyCelestialChallengeStart time.Time
var eventMetadata DailyCelestialChallenge

func Initialize() {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		panic(err)
	}
	// Time as stated in https://support.ageofempires.com/hc/en-us/articles/35393447365268-Daily-Celestial-Challenge-FAQ
	dailyCelestialChallengeStart = time.Date(2025, 3, 4, 9, 0, 0, 0, loc)
	score := DailyCelestialChallengeScore{
		A: 100000,
		B: 19681,
		P: 1.5,
	}
	eventMetadata = DailyCelestialChallenge{
		Plus: DailyCelestialChallengePlus{
			DailyCelestialChallengeScore: score,
			ScoreCalcParams:              score,
		},
		Leaderboard: DailyCelestialChallengeLeaderboard{
			Values: []DailyCelestialChallengeLeaderboardValue{
				{
					Name:            leaderboardName,
					ScoringType:     3,
					VisibleToPublic: true,
					MapEntries: []DailyCelestialChallengeLeaderboardValueMapEntry{
						{
							MatchType:     60,
							Race:          -1,
							StatGroupType: 1,
						},
					},
				},
			},
			PointUpdate: DailyCelestialChallengeLeaderboardPointUpdate{
				MaxWinPts:             100000,
				WinDifferenceToApply:  math.MaxInt32,
				LoseDifferenceToApply: math.MaxInt32,
			},
		},
	}
}

func generateMonthlyEvents() (events []CommunityEvent) {
	now := time.Now().In(dailyCelestialChallengeStart.Location())
	currentEvent := time.Date(
		now.Year(),
		now.Month(),
		1,
		7,
		55,
		0,
		0,
		dailyCelestialChallengeStart.Location(),
	)
	// Generate events for 2 months just in case
	for months := range 2 {
		eventStart := currentEvent.AddDate(0, months, 0)
		eventEnd := eventStart.AddDate(0, months, 0).Add(-time.Hour)
		expiryTime := eventStart.AddDate(1, 0, 0)
		eventName := fmt.Sprintf("%s%s", strings.ToLower(eventStart.Month().String()), "_pantheon_pinup")
		event := CommunityEvent{
			Id:         uint64(months),
			Name:       eventName,
			Start:      eventStart,
			End:        eventEnd,
			ExpiryTime: expiryTime,
			CustomData: "",
			EventState: 2,
		}
		for s := 1; s < 17; s++ {
			event.Leaderboards = append(
				event.Leaderboards,
				EventLeaderBoard{
					Id:          uint64(s),
					Name:        fmt.Sprintf("%s_s%d", event.Name, s),
					IsRanked:    true,
					ScoringType: 3,
					Maps: []EventLeaderboardMap{
						{
							MatchtypeId:    60,
							StatgroupType:  1,
							CivilizationId: -1,
						},
					},
				},
			)
		}
		events = append(events, event)
	}
	return
}

func generateDailyChallengeEvents() (events []CommunityEvent) {
	now := time.Now().In(dailyCelestialChallengeStart.Location())
	timeSince := now.Sub(dailyCelestialChallengeStart)
	daysSince := timeSince.Truncate(dayDuration)
	currentEventStart := dailyCelestialChallengeStart.Add(daysSince)
	daysSinceIt := uint64(daysSince / dayDuration)
	// Generate events for a 3 days just in case
	for days := range 3 {
		eventStart := currentEventStart.Add(dayDuration * time.Duration(days))
		eventEnd := eventStart.Add(dayDuration - time.Second)
		expiryTime := eventEnd.Add(dayDuration * 7)
		id := daysSinceIt + uint64(days)
		eventName := fmt.Sprintf("%s%d", "skirmish_plus_", id)
		event := CommunityEvent{
			Id:         id,
			Name:       eventName,
			Start:      eventStart,
			End:        eventEnd,
			ExpiryTime: expiryTime,
			CustomData: eventMetadata,
			EventState: 2,
			Leaderboards: []EventLeaderBoard{
				{
					Id:          id,
					Name:        eventName + "_" + leaderboardName,
					IsRanked:    true,
					ScoringType: 3,
					Maps: []EventLeaderboardMap{
						{
							MatchtypeId:    60,
							StatgroupType:  1,
							CivilizationId: -1,
						},
					},
				},
			},
		}
		events = append(events, event)
	}
	return
}

func CommunityEventsEncoded() i.A {
	events := generateDailyChallengeEvents()
	events = append(events, generateMonthlyEvents()...)
	eventsEncoded := i.A{}
	leaderboardsEncoded := i.A{}
	leaderboardMapsEncoded := i.A{}
	for _, event := range events {
		marshalCustomData := !strings.Contains(event.Name, "pantheon")
		eventsEncoded = append(eventsEncoded, event.Encode(marshalCustomData))
		leaderboardsEncoded = append(leaderboardsEncoded, event.EncodeLeaderboards()...)
		leaderboardMapsEncoded = append(leaderboardMapsEncoded, event.EncodeLeaderboardsMaps()...)
	}
	return i.A{
		0,
		eventsEncoded,
		i.A{},
		i.A{},
		leaderboardsEncoded,
		leaderboardMapsEncoded,
		i.A{},
	}
}
