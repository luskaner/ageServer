package shared

import "github.com/luskaner/ageServer/server/internal/models"

// MatchURL is one uploaded artifact of a match, as matchurls[] carries it
// (§3.4, C15). The official service sends one entry per player that uploaded a
// replay, not a positional tuple.
type MatchURL struct {
	ProfileID int32  `json:"profile_id"`
	URL       string `json:"url"`
	Size      int64  `json:"size"`
	DataType  int    `json:"datatype"`
}

// ReplayFile is the getReplayFiles entry: the same artifact plus the match it
// belongs to, because that endpoint is not scoped to a match.
type ReplayFile struct {
	ProfileID      int32  `json:"profile_id"`
	MatchHistoryID int64  `json:"matchhistory_id"`
	URL            string `json:"url"`
	Size           int64  `json:"size"`
	DataType       int    `json:"datatype"`
}

// ReportResult is the matchhistoryreportresults[] entry (8 fields).
//
// CivilizationID, not race_id: the live API renamed the field (C15).
type ReportResult struct {
	MatchHistoryID int64  `json:"matchhistory_id"`
	ProfileID      int32  `json:"profile_id"`
	ResultType     int32  `json:"resulttype"`
	TeamID         int32  `json:"teamid"`
	CivilizationID int32  `json:"civilization_id"`
	XPGained       int32  `json:"xpgained"`
	Counters       string `json:"counters"`
	MatchStartDate int64  `json:"matchstartdate"`
}

// Member is the matchhistorymember[] entry (13 fields). It reproduces the
// defensive fill of the game route: the official service never reports a zero
// rating or a zero arbitration, so an unrated match still carries the default
// rating and arbitration 1 (§3.4).
type Member struct {
	MatchHistoryID int64 `json:"matchhistory_id"`
	ProfileID      int32 `json:"profile_id"`
	CivilizationID int32 `json:"civilization_id"`
	StatGroupID    int32 `json:"statgroup_id"`
	TeamID         int32 `json:"teamid"`
	Wins           int32 `json:"wins"`
	Losses         int32 `json:"losses"`
	Streak         int32 `json:"streak"`
	Arbitration    int32 `json:"arbitration"`
	Outcome        int32 `json:"outcome"`
	OldRating      int32 `json:"oldrating"`
	NewRating      int32 `json:"newrating"`
	ReportType     int32 `json:"reporttype"`
}

// Match is the match object shared by getRecentMatchHistory and getMatchHistory.
// The two endpoints differ only in the top-level key and in gamemod_id, which
// the official service reports on getMatchHistory alone (§2.e).
type Match struct {
	ID               int64  `json:"id"`
	CreatorProfileID int32  `json:"creator_profile_id"`
	MapName          string `json:"mapname"`
	MaxPlayers       int32  `json:"maxplayers"`
	MatchTypeID      int32  `json:"matchtype_id"`
	// Options and SlotInfo travel verbatim (base64+zlib); decoding them is the
	// client's job and the server must not re-encode them.
	Options        string         `json:"options"`
	SlotInfo       string         `json:"slotinfo"`
	Description    string         `json:"description"`
	StartGameTime  int64          `json:"startgametime"`
	CompletionTime int64          `json:"completiontime"`
	ObserverTotal  int32          `json:"observertotal"`
	ReportResults  []ReportResult `json:"matchhistoryreportresults"`
	Items          []any          `json:"matchhistoryitems"`
	MatchURLs      []MatchURL     `json:"matchurls"`
	Members        []Member       `json:"matchhistorymember"`
	GameModID      int32          `json:"gamemod_id"`
}

// MatchHistoryMetadata resolves the lobby metadata the client sent when hosting.
// The official service echoes it back verbatim so the client can rebuild the
// post-game slot layout, and a rated match reports "AUTOMATCH" with the paired
// queue id. It is a copy of the game route's helper on purpose: §D1 keeps the
// two encodings independent so /game is never touched.
func MatchHistoryMetadata(match models.MatchResult) (mapName string, maxPlayers int32, description string, matchTypeID int32) {
	mapName = match.MapName
	description = match.Description
	maxPlayers = match.MaxPlayers
	if maxPlayers == 0 {
		maxPlayers = int32(len(match.PlayerStats))
	}
	if match.QueueType != "" || match.Rated {
		matchTypeID = match.MatchType
		if queueID, ok := models.QueueIDFromQueueType(match.QueueType); ok {
			matchTypeID = queueID
		}
		if matchTypeID == 0 {
			matchTypeID = 26
		}
		description = "AUTOMATCH"
	}
	return mapName, maxPlayers, description, matchTypeID
}

// NewReportResult builds one report result from the stored player row.
func NewReportResult(match models.MatchResult, player models.MatchPlayer) ReportResult {
	counters := player.Counters
	if counters == "" {
		counters = "{}"
	}
	return ReportResult{
		MatchHistoryID: match.MatchID,
		ProfileID:      player.ProfileID,
		ResultType:     player.Result,
		TeamID:         player.TeamID,
		CivilizationID: player.RaceID,
		XPGained:       player.XPGained,
		Counters:       counters,
		MatchStartDate: match.StartedAt.Unix(),
	}
}

// NewMember builds one match member, defaulting the ratings the same way the
// game route does.
func NewMember(match models.MatchResult, player models.MatchPlayer, statGroupID int32, rating *models.PlayerRating) Member {
	member := Member{
		MatchHistoryID: match.MatchID,
		ProfileID:      player.ProfileID,
		CivilizationID: player.RaceID,
		StatGroupID:    statGroupID,
		TeamID:         player.TeamID,
		Arbitration:    1,
		Outcome:        player.Result,
		ReportType:     1,
	}
	if rating != nil {
		member.Wins = rating.Wins
		member.Losses = rating.Losses
		member.Streak = rating.Streak
	}
	member.OldRating = player.RatingBefore
	if member.OldRating <= 0 {
		member.OldRating = models.DefaultRating
	}
	member.NewRating = player.RatingAfter
	if member.NewRating <= 0 {
		member.NewRating = member.OldRating
	}
	return member
}

// NewMatch builds the match object from its stored rows. gamemodID is a
// parameter because the official service only reports it on getMatchHistory
// (1800); getRecentMatchHistory sends 0.
func NewMatch(match models.MatchResult, reportResults []ReportResult, matchURLs []MatchURL, members []Member, gamemodID int32) Match {
	mapName, maxPlayers, description, matchTypeID := MatchHistoryMetadata(match)
	var creator int32
	if len(match.PlayerStats) > 0 {
		creator = match.PlayerStats[0].ProfileID
	}
	return Match{
		ID:               match.MatchID,
		CreatorProfileID: creator,
		MapName:          mapName,
		MaxPlayers:       maxPlayers,
		MatchTypeID:      matchTypeID,
		Options:          match.EncodedOptions,
		SlotInfo:         match.EncodedSlots,
		Description:      description,
		StartGameTime:    match.StartedAt.Unix(),
		CompletionTime:   match.FinishedAt.Unix(),
		// ObserverTotal is not recorded per match; the official service sends the
		// real number (documented deviation).
		ObserverTotal: 0,
		ReportResults: reportResults,
		Items:         []any{},
		MatchURLs:     matchURLs,
		Members:       members,
		GameModID:     gamemodID,
	}
}
