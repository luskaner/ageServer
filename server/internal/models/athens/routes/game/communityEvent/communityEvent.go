package communityEvent

import (
	"encoding/json"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
)

type DailyCelestialChallengeScore struct {
	A int
	B int
	P float64
}

type DailyCelestialChallengePlus struct {
	DailyCelestialChallengeScore
	ScoreCalcParams DailyCelestialChallengeScore `json:"scoreCalcParams"`
}

type DailyCelestialChallengeLeaderboardValueMapEntry struct {
	MatchType     int `json:"matchType"`
	Race          int `json:"race"`
	StatGroupType int `json:"statGroupType"`
}

type DailyCelestialChallengeLeaderboardValue struct {
	Name            string                                            `json:"name"`
	ScoringType     int                                               `json:"scoringType"`
	VisibleToPublic bool                                              `json:"visibleToPublic"`
	MapEntries      []DailyCelestialChallengeLeaderboardValueMapEntry `json:"mapEntries"`
}

type DailyCelestialChallengeLeaderboardPointUpdate struct {
	WinPtsDefault         int `json:"winPtsDefault"`
	LosePtsDefault        int `json:"losePtsDefault"`
	MaxWinPts             int `json:"maxWinPts"`
	MinWinPts             int `json:"minWinPts"`
	MaxLosePts            int `json:"maxLosePts"`
	MinLosePts            int `json:"minLosePts"`
	BonusPtsPerDay        int `json:"bonusPtsPerDay"`
	MaxPtsAllowBonus      int `json:"maxPtsAllowBonus"`
	MaxWinPtsWithBonus    int `json:"maxWinPtsWithBonus"`
	MaxBonusPtsDefault    int `json:"maxBonusPtsDefault"`
	PlacementMatch        int `json:"placementMatch"`
	WinDifferenceToApply  int `json:"winDifferenceToApply"`
	LoseDifferenceToApply int `json:"loseDifferenceToApply"`
}

type DailyCelestialChallengeLeaderboard struct {
	Values      []DailyCelestialChallengeLeaderboardValue     `json:"leaderboards"`
	PointUpdate DailyCelestialChallengeLeaderboardPointUpdate `json:"pointUpdate"`
}

type DailyCelestialChallenge struct {
	Plus        DailyCelestialChallengePlus        `json:"skirmishPlus"`
	Leaderboard DailyCelestialChallengeLeaderboard `json:"leaderboard"`
}

type CommunityEvent struct {
	Id           uint64
	Name         string
	Start        time.Time
	End          time.Time
	ExpiryTime   time.Time
	CustomData   any
	EventState   int
	Leaderboards []EventLeaderBoard
}

type EventLeaderBoard struct {
	Id          uint64
	Name        string
	IsRanked    bool
	ScoringType int
	Maps        []EventLeaderboardMap
}

type EventLeaderboardMap struct {
	MatchtypeId    int
	StatgroupType  int
	CivilizationId int
}

func (c *CommunityEvent) Encode(marshalCustomData bool) i.A {
	res := i.A{
		c.Name,
		c.Start.Unix(),
		c.End.Unix(),
		c.ExpiryTime.Unix(),
		c.Id,
		c.EventState,
		nil,
	}
	if marshalCustomData {
		customDataEncoded, _ := json.Marshal(c.CustomData)
		res = append(res, string(customDataEncoded))
	} else {
		res = append(res, c.CustomData)
	}
	return res
}

func (c *CommunityEvent) EncodeLeaderboards() i.A {
	leaderboards := i.A{}
	for _, leaderboard := range c.Leaderboards {
		var isRankedInt int
		if leaderboard.IsRanked {
			isRankedInt = 1
		}
		leaderboards = append(
			leaderboards,
			i.A{
				c.Id,
				leaderboard.Id,
				leaderboard.Name,
				isRankedInt,
				leaderboard.ScoringType,
			},
		)
	}
	return leaderboards
}

func (c *CommunityEvent) EncodeLeaderboardsMaps() i.A {
	maps := i.A{}
	for _, leaderboard := range c.Leaderboards {
		for _, mp := range leaderboard.Maps {
			maps = append(
				maps,
				i.A{
					leaderboard.Id,
					mp.MatchtypeId,
					mp.StatgroupType,
					mp.CivilizationId,
					c.Id,
				},
			)
		}
	}
	return maps
}
