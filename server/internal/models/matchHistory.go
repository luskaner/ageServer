package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/luskaner/ageServer/server/internal/logger"

	_ "modernc.org/sqlite"
)

const (
	// DefaultRating is the rating assigned to a profile that has never played
	// a rated match.
	DefaultRating int32 = 1000

	// SoloRMRanked is the AoE2:DE 1v1 random map ranked ladder. The automatch
	// queue id (6) is not the ladder id; the client queries the ladder with
	// leaderboard_id=3.
	SoloRMRanked int32 = 3

	// eloK controls how much a single result can move a rating.
	eloK = 32.0

	// MaxLeaderboardPageSize bounds how many ladder entries a single request
	// may ask for, so a client cannot force an arbitrarily large allocation.
	MaxLeaderboardPageSize = 1000

	// initialLeaderboardPageCapacity is the fixed preallocation of a page.
	initialLeaderboardPageCapacity = 100
)

// MatchResult is one reported game. Only the fields that the server can
// derive from the client payload are filled in; everything else stays at its
// zero value.
type MatchResult struct {
	MatchID        int64  `json:"matchId"`
	MapID          int32  `json:"mapId"`
	MapName        string `json:"mapName"`
	MaxPlayers     int32  `json:"maxPlayers"`
	Description    string `json:"description"`
	EncodedOptions string `json:"encodedOptions"`
	EncodedSlots   string `json:"encodedSlots"`
	LeaderboardID  int32  `json:"leaderboard"`
	QueueType      string `json:"queueType"`
	// MatchType is the client pool/queue id that was paired (automatch2-<id>).
	// QueueID is kept as an explicit alias because the plan distinguishes the
	// catalog queue from the legacy match type.
	MatchType     int32         `json:"matchType"`
	QueueID       int32         `json:"queueId"`
	Players       []int32       `json:"players"`
	Teams         []int32       `json:"teams"`
	Results       []int32       `json:"results"`
	Races         []int32       `json:"races"`
	XPGained      []int32       `json:"xpGained"`
	Counters      []string      `json:"counters"`
	CustomUpdates []string      `json:"customUpdates"`
	WinnerTeam    int32         `json:"winnerTeam"`
	StartedAt     time.Time     `json:"startedAt"`
	FinishedAt    time.Time     `json:"finishedAt"`
	Rated         bool          `json:"rated"`
	PlayerStats   []MatchPlayer `json:"playerStats,omitempty"`
}

// MatchPlayer is the per-profile part of a reported match.
type MatchPlayer struct {
	ProfileID     int32  `json:"profileId"`
	TeamID        int32  `json:"teamId"`
	Result        int32  `json:"result"`
	RaceID        int32  `json:"raceId"`
	XPGained      int32  `json:"xpGained"`
	ReportOrder   int32  `json:"reportOrder"`
	Counters      string `json:"counters"`
	CustomUpdates string `json:"customUpdates"`
	RatingBefore  int32  `json:"ratingBefore"`
	RatingAfter   int32  `json:"ratingAfter"`
	RatingDelta   int32  `json:"ratingDelta"`
}

// PlayerRating is a profile's standing on one ladder.
type PlayerRating struct {
	ProfileID     int32     `json:"profileId"`
	LeaderboardID int32     `json:"leaderboardId"`
	StatID        int32     `json:"statId"`
	Rating        int32     `json:"rating"`
	HighestRating int32     `json:"highestRating"`
	RatingDelta   int32     `json:"ratingDelta"`
	Games         int32     `json:"games"`
	Wins          int32     `json:"wins"`
	Losses        int32     `json:"losses"`
	Streak        int32     `json:"streak"`
	Rank          int64     `json:"rank"`
	RankTotal     int64     `json:"rankTotal"`
	LastMatch     time.Time `json:"lastMatch"`
}

// StoredProfile is the identity of a profile as it was last seen by the
// server. Ratings only keep IDs, so this table is what lets the ladder show
// players that are not connected right now.
type StoredProfile struct {
	ProfileID      int32
	StatGroupID    int32
	Alias          string
	PlatformUserID uint64
	IsXbox         bool
	AvatarMetadata string
	XP             uint32
	Level          uint16
	Region         uint8
}

// ResolveStatGroupID picks the stat group id used to render a match member.
// The connected user wins, then the profile stored in the database, then the
// ladder rating row. Sharing this precedence keeps the match notification and
// the recent-match history consistent, especially for offline players.
func ResolveStatGroupID(user, stored, ratingStatID int32) int32 {
	if user != 0 {
		return user
	}
	if stored != 0 {
		return stored
	}
	return ratingStatID
}

// MatchHistoryStore persists matches and ladder ratings in an SQLite database
// that lives next to the per-profile JSON files
// (resources/userData/<gameId>/automatch2.db). The database and its schema are
// created on first use and reused on every later start.
type MatchHistoryStore struct {
	mu     sync.RWMutex
	db     *sql.DB
	path   string
	gameID string
	// now resolves the active ranked season. Tests replace it to move the
	// clock across a season boundary.
	now func() time.Time
}

// NewMatchHistoryStore opens (and creates when missing) the database in
// dataDir.
func NewMatchHistoryStore(dataDir string) *MatchHistoryStore {
	s := &MatchHistoryStore{
		path: filepath.Join(dataDir, "automatch2.db"),
		now:  time.Now,
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		logger.Printf("matchHistory: cannot create data dir %s: %s\n", dataDir, err)
		return s
	}
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		logger.Printf("matchHistory: cannot open %s: %s\n", s.path, err)
		return s
	}
	// The server does very little concurrent database work. A single
	// connection keeps the connection-local pragmas effective and avoids
	// SQLITE_BUSY between pooled connections.
	db.SetMaxOpenConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err = db.Exec(pragma); err != nil {
			logger.Printf("matchHistory: %s failed: %s\n", pragma, err)
		}
	}
	if err = migrate(db); err != nil {
		logger.Printf("matchHistory: schema migration failed: %s\n", err)
		_ = db.Close()
		return s
	}
	s.db = db
	return s
}

// NewMatchHistoryStoreForGame keeps the match history next to the rest of the
// server's per-game user data.
func NewMatchHistoryStoreForGame(gameId string) *MatchHistoryStore {
	store := NewMatchHistoryStore(filepath.Join(userDataFolder, gameId))
	store.gameID = gameId
	return store
}

// season is the ranked season the store reads and writes. Titles without
// seasons keep a single permanent ladder (season 0).
func (s *MatchHistoryStore) season() int {
	if !GameHasRankedSeasons(s.gameID) {
		return 0
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	return CurrentRankedSeason(now()).Number
}

// defaultLeaderboard is the ladder used when a caller does not provide one.
// age1 queries ladder 1, age2 ladder 3 and the rest use their documented
// default (D13). A store created without a game id keeps the historical 1v1
// Random Map fallback.
func (s *MatchHistoryStore) defaultLeaderboard() int32 {
	if s.gameID == "" {
		return SoloRMRanked
	}
	return DefaultLeaderboardFor(s.gameID)
}

func (s *MatchHistoryStore) GetStorePath() string { return s.path }

func (s *MatchHistoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS matches (
			match_id       INTEGER PRIMARY KEY,
			map_id         INTEGER NOT NULL DEFAULT 0,
			map_name       TEXT    NOT NULL DEFAULT '',
			leaderboard_id INTEGER NOT NULL DEFAULT 0,
			queue_type     TEXT    NOT NULL DEFAULT '',
			winner_team    INTEGER NOT NULL DEFAULT -1,
			reported       INTEGER NOT NULL DEFAULT 0,
			rated          INTEGER NOT NULL DEFAULT 0,
			started_at     INTEGER NOT NULL DEFAULT 0,
			finished_at    INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS match_players (
			match_id      INTEGER NOT NULL,
			profile_id    INTEGER NOT NULL,
			team_id       INTEGER NOT NULL DEFAULT 0,
			result        INTEGER NOT NULL DEFAULT -1,
			race_id       INTEGER NOT NULL DEFAULT 0,
			xp_gained     INTEGER NOT NULL DEFAULT 0,
			player_order  INTEGER NOT NULL DEFAULT 0,
			counters      TEXT    NOT NULL DEFAULT '',
			custom_updates TEXT   NOT NULL DEFAULT '',
			rating_before INTEGER NOT NULL DEFAULT 0,
			rating_after  INTEGER NOT NULL DEFAULT 0,
			rating_delta  INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (match_id, profile_id),
			FOREIGN KEY (match_id) REFERENCES matches(match_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS ratings (
			leaderboard_id INTEGER NOT NULL,
			season         INTEGER NOT NULL DEFAULT 0,
			profile_id     INTEGER NOT NULL,
			stat_id        INTEGER NOT NULL DEFAULT 0,
			rating         INTEGER NOT NULL DEFAULT 1000,
			highest_rating INTEGER NOT NULL DEFAULT 1000,
			games          INTEGER NOT NULL DEFAULT 0,
			wins           INTEGER NOT NULL DEFAULT 0,
			losses         INTEGER NOT NULL DEFAULT 0,
			streak         INTEGER NOT NULL DEFAULT 0,
			last_match     INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (leaderboard_id, season, profile_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ratings_ladder ON ratings(leaderboard_id, season, rating DESC)`,
		// mmr is the hidden matchmaking rating. Unlike ratings it is not
		// scoped by season: the official service keeps it across seasons and
		// only resets the visible ladder. It is keyed by queue because Quick
		// Match has one too, even without a visible ladder.
		`CREATE TABLE IF NOT EXISTS mmr (
			queue_id   INTEGER NOT NULL,
			profile_id INTEGER NOT NULL,
			mmr        INTEGER NOT NULL DEFAULT 1000,
			updated_at INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (queue_id, profile_id)
		)`,
		`CREATE TABLE IF NOT EXISTS profiles (
			profile_id       INTEGER PRIMARY KEY,
			stat_group_id    INTEGER NOT NULL DEFAULT 0,
			alias            TEXT    NOT NULL DEFAULT '',
			platform_user_id INTEGER NOT NULL DEFAULT 0,
			is_xbox          INTEGER NOT NULL DEFAULT 0,
			avatar_metadata  TEXT    NOT NULL DEFAULT '',
			xp               INTEGER NOT NULL DEFAULT 0,
			level            INTEGER NOT NULL DEFAULT 1,
			region           INTEGER NOT NULL DEFAULT 0,
			updated_at       INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_match_players_profile ON match_players(profile_id)`,
		`CREATE INDEX IF NOT EXISTS idx_matches_finished ON matches(finished_at DESC)`,
		`CREATE TABLE IF NOT EXISTS single_player_matches (
			match_key  TEXT PRIMARY KEY,
			match_id   INTEGER NOT NULL,
			created_at INTEGER NOT NULL DEFAULT 0
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	for _, column := range []struct {
		name       string
		definition string
	}{
		{name: "player_order", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "counters", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "custom_updates", definition: "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := ensureColumn(db, "match_players", column.name, column.definition); err != nil {
			return err
		}
	}
	// The lobby metadata the client sends when hosting is echoed back in the
	// match history, so it has to survive the match itself.
	for _, column := range []struct {
		name       string
		definition string
	}{
		{name: "max_players", definition: "INTEGER NOT NULL DEFAULT 0"},
		{name: "description", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "encoded_options", definition: "TEXT NOT NULL DEFAULT ''"},
		{name: "encoded_slots", definition: "TEXT NOT NULL DEFAULT ''"},
		// mmr_applied marks that the hidden MMR of the queue was already
		// updated for this match. It is separate from rated because Quick
		// Match updates the MMR without moving any visible ladder.
		{name: "mmr_applied", definition: "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := ensureColumn(db, "matches", column.name, column.definition); err != nil {
			return err
		}
	}
	return nil
}

func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	found := false
	for rows.Next() {
		var (
			cid          int
			name         string
			columnType   string
			notNull      int
			defaultValue sql.NullString
			primaryKey   int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition)
	return err
}

// RecordPendingMatch stores the pairing information as soon as the map has
// been chosen, so the map/queue are still known when the clients report the
// result after the matchmaking entry has already been dropped.
func (s *MatchHistoryStore) RecordPendingMatch(m MatchResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil || m.MatchID == 0 {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		logger.Printf("matchHistory: pending match %d: %s\n", m.MatchID, err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	started := m.StartedAt
	if started.IsZero() {
		started = time.Now()
	}
	if _, err = tx.Exec(
		`INSERT INTO matches (match_id, map_id, map_name, leaderboard_id, queue_type, started_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(match_id) DO NOTHING`,
		m.MatchID, m.MapID, m.MapName, m.LeaderboardID, m.QueueType, started.Unix(),
	); err != nil {
		logger.Printf("matchHistory: pending match %d: %s\n", m.MatchID, err)
		return
	}
	for i, profile := range m.Players {
		team, result := int32(0), int32(-1)
		if i < len(m.Teams) {
			team = m.Teams[i]
		}
		if i < len(m.Results) {
			result = m.Results[i]
		}
		if _, err = tx.Exec(
			`INSERT INTO match_players (match_id, profile_id, team_id, result, player_order)
			 VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(match_id, profile_id) DO NOTHING`,
			m.MatchID, profile, team, result, i,
		); err != nil {
			logger.Printf("matchHistory: pending player %d/%d: %s\n", m.MatchID, profile, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		logger.Printf("matchHistory: pending match %d commit: %s\n", m.MatchID, err)
	}
}

// GetMatch returns the metadata of a stored match without loading its player
// rows. The queue type and ladder are recorded when the match is paired, so
// this is what lets reportMatch score the result on the right ladder (D4/D13).
func (s *MatchHistoryStore) GetMatch(matchID int64) (MatchResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil || matchID == 0 {
		return MatchResult{}, false
	}
	var (
		m                     MatchResult
		startedAt, finishedAt int64
		rated                 int
	)
	err := s.db.QueryRow(
		`SELECT match_id, map_id, map_name, leaderboard_id, queue_type, winner_team,
		        started_at, finished_at, max_players, description, encoded_options,
		        encoded_slots, rated
		 FROM matches WHERE match_id = ?`, matchID,
	).Scan(&m.MatchID, &m.MapID, &m.MapName, &m.LeaderboardID, &m.QueueType,
		&m.WinnerTeam, &startedAt, &finishedAt, &m.MaxPlayers, &m.Description,
		&m.EncodedOptions, &m.EncodedSlots, &rated)
	if err != nil {
		return MatchResult{}, false
	}
	m.StartedAt = time.Unix(startedAt, 0)
	m.FinishedAt = time.Unix(finishedAt, 0)
	m.Rated = rated != 0
	m.QueueID, _ = QueueIDFromQueueType(m.QueueType)
	m.MatchType = m.QueueID
	return m, true
}

// QueueIDFromQueueType extracts the pool id from the "automatch2-<id>" queue
// type the server stores. It returns false for non-automatch matches.
func QueueIDFromQueueType(queueType string) (int32, bool) {
	const prefix = "automatch2-"
	if len(queueType) <= len(prefix) || queueType[:len(prefix)] != prefix {
		return 0, false
	}
	value, err := strconv.ParseInt(queueType[len(prefix):], 10, 32)
	if err != nil || value <= 0 {
		return 0, false
	}
	return int32(value), true
}

// ApplyResult stores a reported result and, the first time it is seen, moves
// the ladder ratings. It returns the final per-profile ratings and whether the
// result changed the ratings.
func (s *MatchHistoryStore) ApplyResult(m MatchResult) (map[int32]*PlayerRating, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return map[int32]*PlayerRating{}, false
	}
	if m.MatchID == 0 || len(m.Players) == 0 {
		return map[int32]*PlayerRating{}, false
	}
	tx, err := s.db.Begin()
	if err != nil {
		logger.Printf("matchHistory: apply %d: %s\n", m.MatchID, err)
		return map[int32]*PlayerRating{}, false
	}
	defer func() { _ = tx.Rollback() }()

	var (
		existingLeaderboard int32
		existingRated       bool
		existingMMRApplied  bool
		existingReached     bool
	)
	if err = tx.QueryRow(
		`SELECT leaderboard_id, rated, mmr_applied FROM matches WHERE match_id = ?`, m.MatchID,
	).Scan(&existingLeaderboard, &existingRated, &existingMMRApplied); err == nil {
		existingReached = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		logger.Printf("matchHistory: lookup %d: %s\n", m.MatchID, err)
		return map[int32]*PlayerRating{}, false
	}

	leaderboard := m.LeaderboardID
	if existingReached && existingLeaderboard != 0 {
		leaderboard = existingLeaderboard
	}
	winner, ok := winnerTeam(m)
	finished := m.FinishedAt
	if finished.IsZero() {
		finished = time.Now()
	}
	if _, err = tx.Exec(
		`INSERT INTO matches (match_id, map_id, map_name, leaderboard_id, queue_type, winner_team,
		                      reported, rated, started_at, finished_at,
		                      max_players, description, encoded_options, encoded_slots)
		 VALUES (?, ?, ?, ?, ?, ?, 1, 0, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(match_id) DO UPDATE SET
		   map_id         = CASE WHEN excluded.map_id <> 0 THEN excluded.map_id ELSE matches.map_id END,
		   map_name       = CASE WHEN excluded.map_name <> '' THEN excluded.map_name ELSE matches.map_name END,
		   leaderboard_id = CASE WHEN excluded.leaderboard_id <> 0 THEN excluded.leaderboard_id ELSE matches.leaderboard_id END,
		   queue_type     = CASE WHEN excluded.queue_type <> '' THEN excluded.queue_type ELSE matches.queue_type END,
		   winner_team    = CASE WHEN excluded.winner_team >= 0 THEN excluded.winner_team ELSE matches.winner_team END,
		   reported       = 1,
		   finished_at    = CASE WHEN excluded.finished_at <> 0 THEN excluded.finished_at ELSE matches.finished_at END,
		   max_players    = CASE WHEN excluded.max_players <> 0 THEN excluded.max_players ELSE matches.max_players END,
		   description    = CASE WHEN excluded.description <> '' THEN excluded.description ELSE matches.description END,
		   encoded_options = CASE WHEN excluded.encoded_options <> '' THEN excluded.encoded_options ELSE matches.encoded_options END,
		   encoded_slots  = CASE WHEN excluded.encoded_slots <> '' THEN excluded.encoded_slots ELSE matches.encoded_slots END`,
		m.MatchID, m.MapID, m.MapName, leaderboard, m.QueueType, winner,
		finished.Unix(), finished.Unix(),
		m.MaxPlayers, m.Description, m.EncodedOptions, m.EncodedSlots,
	); err != nil {
		logger.Printf("matchHistory: store %d: %s\n", m.MatchID, err)
		return map[int32]*PlayerRating{}, false
	}

	for i, profile := range m.Players {
		team, result, race, xp := int32(0), int32(-1), int32(0), int32(0)
		if i < len(m.Teams) {
			team = m.Teams[i]
		}
		if i < len(m.Results) {
			result = m.Results[i]
		}
		if i < len(m.Races) {
			race = m.Races[i]
		}
		if i < len(m.XPGained) {
			xp = m.XPGained[i]
		}
		counters, customUpdates := "", ""
		if i < len(m.Counters) {
			counters = m.Counters[i]
		}
		if i < len(m.CustomUpdates) {
			customUpdates = m.CustomUpdates[i]
		}
		if _, err = tx.Exec(
			`INSERT INTO match_players (match_id, profile_id, team_id, result, race_id, xp_gained,
			                            player_order, counters, custom_updates)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(match_id, profile_id) DO UPDATE SET
			   team_id   = excluded.team_id,
			   result    = excluded.result,
			   race_id   = excluded.race_id,
			   xp_gained = excluded.xp_gained,
			   player_order = excluded.player_order,
			   counters = CASE WHEN excluded.counters <> '' THEN excluded.counters ELSE match_players.counters END,
			   custom_updates = CASE WHEN excluded.custom_updates <> '' THEN excluded.custom_updates ELSE match_players.custom_updates END`,
			m.MatchID, profile, team, result, race, xp, i, counters, customUpdates,
		); err != nil {
			logger.Printf("matchHistory: player %d/%d: %s\n", m.MatchID, profile, err)
			return map[int32]*PlayerRating{}, false
		}
	}

	resultRatings := map[int32]*PlayerRating{}
	rated := false
	// Hidden MMR: every automatch queue of a seasonal title has one, including
	// Quick Match, which shows no visible rating but is still matched by skill.
	// It is applied once per match (mmr_applied) and never reset by a season.
	if !existingMMRApplied && ok && GameHasRankedSeasons(s.gameID) && m.QueueID > 0 &&
		len(m.Players) == len(m.Teams) {
		if s.applyMMRTx(tx, m.QueueID, m, winner) {
			if _, err = tx.Exec(`UPDATE matches SET mmr_applied = 1 WHERE match_id = ?`, m.MatchID); err != nil {
				logger.Printf("matchHistory: mark mmr applied %d: %s\n", m.MatchID, err)
			}
		}
	}
	// Visible ladder: only rated queues have one.
	if !existingRated && ok && leaderboard > 0 && len(m.Players) == len(m.Teams) && len(m.Players) == len(m.Results) {
		if updated, applied := s.applyEloTx(tx, leaderboard, m, winner); applied {
			for id, r := range updated {
				resultRatings[id] = r
			}
			rated = true
			if _, err = tx.Exec(`UPDATE matches SET rated = 1 WHERE match_id = ?`, m.MatchID); err != nil {
				logger.Printf("matchHistory: mark rated %d: %s\n", m.MatchID, err)
			}
		}
	}

	if err = tx.Commit(); err != nil {
		logger.Printf("matchHistory: commit %d: %s\n", m.MatchID, err)
		return map[int32]*PlayerRating{}, false
	}
	if len(resultRatings) == 0 {
		resultRatings = s.getRatingsLocked(leaderboard, m.Players)
	}
	return resultRatings, rated
}

func winnerTeam(m MatchResult) (int32, bool) {
	if len(m.Teams) == len(m.Results) && len(m.Results) > 0 {
		winner := int32(-1)
		for i, result := range m.Results {
			if result != 1 {
				continue
			}
			if winner >= 0 && m.Teams[i] != winner {
				return -1, false
			}
			winner = m.Teams[i]
		}
		if winner >= 0 {
			return winner, true
		}
	}
	if m.WinnerTeam >= 0 {
		return m.WinnerTeam, true
	}
	return -1, false
}

// eloDeltas returns the rating change of each side for a two-team result.
func eloDeltas(ratingA, ratingB float64, winnerIsA bool) (int32, int32) {
	expectedA := 1.0 / (1.0 + math.Pow(10, (ratingB-ratingA)/400.0))
	expectedB := 1.0 - expectedA
	scoreA, scoreB := 0.0, 1.0
	if winnerIsA {
		scoreA, scoreB = 1.0, 0.0
	}
	return int32(math.Round(eloK * (scoreA - expectedA))), int32(math.Round(eloK * (scoreB - expectedB)))
}

// twoTeams splits a reported result into its two teams and validates the
// winner. It returns false when the result is not a two-team game.
func twoTeams(m MatchResult, winner int32) (teams map[int32][]int32, teamA, teamB int32, ok bool) {
	if len(m.Teams) < len(m.Players) {
		return nil, 0, 0, false
	}
	teams = map[int32][]int32{}
	for i, profile := range m.Players {
		team := m.Teams[i]
		teams[team] = append(teams[team], profile)
	}
	if len(teams) != 2 {
		return nil, 0, 0, false
	}
	teamIDs := make([]int32, 0, 2)
	for team := range teams {
		teamIDs = append(teamIDs, team)
	}
	sort.Slice(teamIDs, func(i, j int) bool { return teamIDs[i] < teamIDs[j] })
	teamA, teamB = teamIDs[0], teamIDs[1]
	if winner != teamA && winner != teamB {
		return nil, 0, 0, false
	}
	return teams, teamA, teamB, true
}

// applyMMRTx updates the hidden matchmaking rating of one queue. It is not
// scoped by season: the official service keeps it across seasons, and Quick
// Match has one too even though it shows no visible rating.
func (s *MatchHistoryStore) applyMMRTx(tx *sql.Tx, queueID int32, m MatchResult, winner int32) bool {
	teams, teamA, teamB, ok := twoTeams(m, winner)
	if !ok {
		return false
	}
	mmrOf := func(profile int32) (int32, error) {
		var mmr int32
		err := tx.QueryRow(
			`SELECT mmr FROM mmr WHERE queue_id = ? AND profile_id = ?`,
			queueID, profile,
		).Scan(&mmr)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err = tx.Exec(
				`INSERT INTO mmr (queue_id, profile_id, mmr, updated_at)
				 VALUES (?, ?, ?, ?) ON CONFLICT(queue_id, profile_id) DO NOTHING`,
				queueID, profile, DefaultRating, time.Now().Unix(),
			); err != nil {
				return 0, err
			}
			return DefaultRating, nil
		}
		return mmr, err
	}
	before := make(map[int32]int32, len(m.Players))
	for _, profile := range m.Players {
		mmr, err := mmrOf(profile)
		if err != nil {
			logger.Printf("matchHistory: mmr %d/%d: %s\n", m.MatchID, profile, err)
			return false
		}
		before[profile] = mmr
	}
	average := func(team int32) float64 {
		var total float64
		for _, profile := range teams[team] {
			total += float64(before[profile])
		}
		return total / float64(len(teams[team]))
	}
	deltaA, deltaB := eloDeltas(average(teamA), average(teamB), winner == teamA)
	for team, delta := range map[int32]int32{teamA: deltaA, teamB: deltaB} {
		for _, profile := range teams[team] {
			next := before[profile] + delta
			if next < 1 {
				next = 1
			}
			if _, err := tx.Exec(
				`UPDATE mmr SET mmr = ?, updated_at = ? WHERE queue_id = ? AND profile_id = ?`,
				next, time.Now().Unix(), queueID, profile,
			); err != nil {
				logger.Printf("matchHistory: mmr update %d/%d: %s\n", m.MatchID, profile, err)
				return false
			}
		}
	}
	return true
}

// applyEloTx must be called with s.mu held and inside tx. It updates the
// visible, season-scoped ladder: every rated match scores the season that is
// active when the result is applied, so a new season starts from the default
// rating. During the season's placement matches the rating is calibrated
// towards the hidden MMR (already updated by applyMMRTx), so a returning
// player is placed at their skill level instead of climbing from scratch.
func (s *MatchHistoryStore) applyEloTx(tx *sql.Tx, leaderboard int32, m MatchResult, winner int32) (map[int32]*PlayerRating, bool) {
	season := s.season()
	seasonal := GameHasRankedSeasons(s.gameID)
	teams, teamA, teamB, ok := twoTeams(m, winner)
	if !ok {
		return nil, false
	}

	ratingOf := func(profile int32) (int32, error) {
		var rating int32
		err := tx.QueryRow(
			`SELECT rating FROM ratings WHERE leaderboard_id = ? AND season = ? AND profile_id = ?`,
			leaderboard, season, profile,
		).Scan(&rating)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err = tx.Exec(
				`INSERT INTO ratings (leaderboard_id, season, profile_id, rating, highest_rating)
				 VALUES (?, ?, ?, ?, ?) ON CONFLICT(leaderboard_id, season, profile_id) DO NOTHING`,
				leaderboard, season, profile, DefaultRating, DefaultRating,
			); err != nil {
				return 0, err
			}
			return DefaultRating, nil
		}
		return rating, err
	}
	averageRating := func(team int32) (float64, error) {
		var total float64
		for _, profile := range teams[team] {
			rating, err := ratingOf(profile)
			if err != nil {
				return 0, err
			}
			total += float64(rating)
		}
		return total / float64(len(teams[team])), nil
	}
	ratingA, err := averageRating(teamA)
	if err != nil {
		return nil, false
	}
	ratingB, err := averageRating(teamB)
	if err != nil {
		return nil, false
	}
	deltaA, deltaB := eloDeltas(ratingA, ratingB, winner == teamA)

	// Placement target: the hidden MMR of the queue, already updated for this
	// match. It is what the visible rating converges to while the season is
	// still in placement.
	placementTarget := map[int32]int32{}
	if seasonal && m.QueueID > 0 {
		for _, profile := range m.Players {
			var mmr int32
			if err := tx.QueryRow(
				`SELECT mmr FROM mmr WHERE queue_id = ? AND profile_id = ?`, m.QueueID, profile,
			).Scan(&mmr); err == nil && mmr > 0 {
				placementTarget[profile] = mmr
			}
		}
	}

	updated := make(map[int32]*PlayerRating, len(m.Players))
	apply := func(team int32, delta int32, won bool) error {
		for _, profile := range teams[team] {
			r := &PlayerRating{
				ProfileID:     profile,
				LeaderboardID: leaderboard,
				RatingDelta:   delta,
			}
			var lastMatch int64
			if err := tx.QueryRow(
				`SELECT rating, highest_rating, games, wins, losses, streak, stat_id, last_match
				 FROM ratings WHERE leaderboard_id = ? AND season = ? AND profile_id = ?`,
				leaderboard, season, profile,
			).Scan(&r.Rating, &r.HighestRating, &r.Games, &r.Wins, &r.Losses, &r.Streak, &r.StatID, &lastMatch); err != nil {
				return err
			}
			r.LastMatch = time.Unix(lastMatch, 0)
			before := r.Rating
			after := before + delta
			// Placement calibration: the first PlacementMatches games move the
			// visible rating towards the hidden MMR, so the season's ladder is
			// reset without throwing away the player's real skill.
			if seasonal && r.Games < PlacementMatches {
				if target, ok := placementTarget[profile]; ok {
					if remaining := int32(PlacementMatches) - r.Games; remaining > 0 {
						after = before + int32(math.Round(float64(target-before)/float64(remaining)))
					}
				}
			}
			if after < 1 {
				after = 1
			}
			r.Rating = after
			r.Games++
			if won {
				r.Wins++
				if r.Streak >= 0 {
					r.Streak++
				} else {
					r.Streak = 1
				}
			} else {
				r.Losses++
				if r.Streak <= 0 {
					r.Streak--
				} else {
					r.Streak = -1
				}
			}
			if r.Rating > r.HighestRating {
				r.HighestRating = r.Rating
			}
			r.RatingDelta = r.Rating - before
			r.LastMatch = time.Now()
			if _, err := tx.Exec(
				`UPDATE ratings SET rating = ?, highest_rating = ?, games = ?, wins = ?, losses = ?,
				        streak = ?, last_match = ? WHERE leaderboard_id = ? AND season = ? AND profile_id = ?`,
				r.Rating, r.HighestRating, r.Games, r.Wins, r.Losses, r.Streak, r.LastMatch.Unix(),
				leaderboard, season, profile,
			); err != nil {
				return err
			}
			if _, err := tx.Exec(
				`UPDATE match_players SET rating_before = ?, rating_after = ?, rating_delta = ?
				 WHERE match_id = ? AND profile_id = ?`,
				before, r.Rating, r.RatingDelta, m.MatchID, profile,
			); err != nil {
				return err
			}
			updated[profile] = r
		}
		return nil
	}
	if err := apply(teamA, deltaA, winner == teamA); err != nil {
		logger.Printf("matchHistory: elo team A %d: %s\n", m.MatchID, err)
		return nil, false
	}
	if err := apply(teamB, deltaB, winner == teamB); err != nil {
		logger.Printf("matchHistory: elo team B %d: %s\n", m.MatchID, err)
		return nil, false
	}
	return updated, true
}

// CreateSinglePlayerMatch maps the opaque matchKey of a single-player match to
// the match id the server generated, so the later report can resolve it.
func (s *MatchHistoryStore) CreateSinglePlayerMatch(matchKey string, matchID int64) error {
	if matchKey == "" || matchID == 0 {
		return errors.New("single-player matchKey and matchID are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return errors.New("match history database is not available")
	}
	_, err := s.db.Exec(
		`INSERT INTO single_player_matches (match_key, match_id, created_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(match_key) DO UPDATE SET match_id = excluded.match_id, created_at = excluded.created_at`,
		matchKey, matchID, time.Now().Unix(),
	)
	return err
}

// ResolveSinglePlayerMatch returns the match id stored for a matchKey.
func (s *MatchHistoryStore) ResolveSinglePlayerMatch(matchKey string) (int64, bool) {
	if matchKey == "" {
		return 0, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return 0, false
	}
	var matchID int64
	if err := s.db.QueryRow(
		`SELECT match_id FROM single_player_matches WHERE match_key = ?`, matchKey,
	).Scan(&matchID); err != nil {
		return 0, false
	}
	return matchID, true
}

func (s *MatchHistoryStore) HasMatch(matchID int64) bool {
	if matchID == 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return false
	}
	var reported int
	if err := s.db.QueryRow(`SELECT reported FROM matches WHERE match_id = ?`, matchID).Scan(&reported); err != nil {
		return false
	}
	return reported == 1
}

// RecordMatch keeps the old interface: store a reported match without moving
// ratings. Prefer ApplyResult when the result has just been received.
func (s *MatchHistoryStore) RecordMatch(m MatchResult) {
	if s.HasMatch(m.MatchID) {
		return
	}
	s.ApplyResult(m)
}

// UpdateRatings is kept for callers that only want the ladder update.
func (s *MatchHistoryStore) UpdateRatings(m MatchResult) {
	s.ApplyResult(m)
}

func (s *MatchHistoryStore) GetRatings(profiles []int32) map[int32]*PlayerRating {
	return s.GetRatingsForLeaderboard(s.defaultLeaderboard(), profiles)
}

func (s *MatchHistoryStore) GetRatingsForLeaderboard(leaderboardID int32, profiles []int32) map[int32]*PlayerRating {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getRatingsLocked(leaderboardID, profiles)
}

// getRatingsLocked is the lock-free body of GetRatingsForLeaderboard. Callers
// must hold at least the read lock.
func (s *MatchHistoryStore) getRatingsLocked(leaderboardID int32, profiles []int32) map[int32]*PlayerRating {
	out := make(map[int32]*PlayerRating, len(profiles))
	if leaderboardID <= 0 {
		leaderboardID = s.defaultLeaderboard()
	}
	for _, profile := range profiles {
		out[profile] = &PlayerRating{
			ProfileID:     profile,
			LeaderboardID: leaderboardID,
			Rating:        DefaultRating,
			HighestRating: DefaultRating,
		}
	}
	if s.db == nil {
		return out
	}
	rows, err := s.db.Query(
		`SELECT profile_id, rating, highest_rating, games, wins, losses, streak, stat_id, last_match
		 FROM ratings WHERE leaderboard_id = ? AND season = ?`, leaderboardID, s.season(),
	)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		r := &PlayerRating{LeaderboardID: leaderboardID}
		var lastMatch int64
		if err = rows.Scan(&r.ProfileID, &r.Rating, &r.HighestRating, &r.Games, &r.Wins, &r.Losses, &r.Streak, &r.StatID, &lastMatch); err != nil {
			continue
		}
		r.LastMatch = time.Unix(lastMatch, 0)
		if _, wanted := out[r.ProfileID]; wanted {
			out[r.ProfileID] = r
		}
	}
	return out
}

// GetRecentForProfiles returns the newest reported matches that include any of
// the requested profiles, newest first.
func (s *MatchHistoryStore) GetRecentForProfiles(profiles []int32, limit int) []MatchResult {
	if len(profiles) == 0 {
		return nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// The profile list travels as a single JSON parameter and is expanded by
	// json_each, so the SQL text stays constant for any number of profiles.
	wanted, err := json.Marshal(profiles)
	if err != nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil
	}
	query := `SELECT DISTINCT m.match_id, m.map_id, m.map_name, m.leaderboard_id, m.queue_type,
	                 m.winner_team, m.started_at, m.finished_at,
	                 m.max_players, m.description, m.encoded_options, m.encoded_slots
	          FROM matches m
	          JOIN match_players p ON p.match_id = m.match_id
	          WHERE m.reported = 1
	            AND p.profile_id IN (SELECT value FROM json_each(?))
	          ORDER BY m.finished_at DESC, m.match_id DESC
	          LIMIT ?`
	rows, err := s.db.Query(query, string(wanted), limit)
	if err != nil {
		return nil
	}
	var matches []MatchResult
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			continue
		}
		matches = append(matches, m)
	}
	// Close the cursor before loading the per-match players: the pool has a
	// single connection, so a nested query would wait for this cursor forever.
	_ = rows.Close()
	for i := range matches {
		matches[i] = s.loadPlayersLocked(matches[i])
	}
	return matches
}

func (s *MatchHistoryStore) GetRecentForProfile(profileID int32, limit int) []MatchResult {
	return s.GetRecentForProfiles([]int32{profileID}, limit)
}

func scanMatch(rows *sql.Rows) (MatchResult, error) {
	var (
		m                     MatchResult
		startedAt, finishedAt int64
	)
	err := rows.Scan(&m.MatchID, &m.MapID, &m.MapName, &m.LeaderboardID, &m.QueueType,
		&m.WinnerTeam, &startedAt, &finishedAt,
		&m.MaxPlayers, &m.Description, &m.EncodedOptions, &m.EncodedSlots)
	if err != nil {
		return m, err
	}
	m.StartedAt = time.Unix(startedAt, 0)
	m.FinishedAt = time.Unix(finishedAt, 0)
	return m, nil
}

func (s *MatchHistoryStore) loadPlayersLocked(m MatchResult) MatchResult {
	rows, err := s.db.Query(
		`SELECT profile_id, team_id, result, race_id, xp_gained, player_order, counters, custom_updates,
		        rating_before, rating_after, rating_delta
		 FROM match_players WHERE match_id = ? ORDER BY player_order, team_id, profile_id`, m.MatchID,
	)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var p MatchPlayer
		if err = rows.Scan(&p.ProfileID, &p.TeamID, &p.Result, &p.RaceID, &p.XPGained,
			&p.ReportOrder, &p.Counters, &p.CustomUpdates,
			&p.RatingBefore, &p.RatingAfter, &p.RatingDelta); err != nil {
			continue
		}
		m.Players = append(m.Players, p.ProfileID)
		m.Teams = append(m.Teams, p.TeamID)
		m.Results = append(m.Results, p.Result)
		m.Races = append(m.Races, p.RaceID)
		m.XPGained = append(m.XPGained, p.XPGained)
		m.PlayerStats = append(m.PlayerStats, p)
	}
	return m
}

func (s *MatchHistoryStore) GetLeaderboard(limit int) []*PlayerRating {
	list, _ := s.GetLeaderboardPage(s.defaultLeaderboard(), 0, limit)
	return list
}

// GetLeaderboardPage returns one page of a ladder ordered exactly like the
// client's sortBy=1 query (rating descending) and the total number of ranked
// profiles.
func (s *MatchHistoryStore) GetLeaderboardPage(leaderboardID int32, start, count int) ([]*PlayerRating, int64) {
	if leaderboardID <= 0 {
		leaderboardID = s.defaultLeaderboard()
	}
	if start < 0 {
		start = 0
	}
	if count <= 0 {
		count = 100
	}
	if count > MaxLeaderboardPageSize {
		count = MaxLeaderboardPageSize
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, 0
	}
	var total int64
	season := s.season()
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM ratings WHERE leaderboard_id = ? AND season = ?`, leaderboardID, season,
	).Scan(&total); err != nil {
		return nil, 0
	}
	rows, err := s.db.Query(
		`SELECT profile_id, rating, highest_rating, games, wins, losses, streak, stat_id, last_match
		 FROM ratings WHERE leaderboard_id = ? AND season = ?
		 ORDER BY rating DESC, profile_id ASC LIMIT ? OFFSET ?`,
		leaderboardID, season, count, start,
	)
	if err != nil {
		return nil, total
	}
	defer rows.Close()
	// Preallocate a bounded capacity: appending grows it for larger pages, and
	// the allocation never depends on the requested (user-provided) size.
	list := make([]*PlayerRating, 0, initialLeaderboardPageCapacity)
	for rows.Next() {
		r := &PlayerRating{LeaderboardID: leaderboardID, RankTotal: total}
		var lastMatch int64
		if err = rows.Scan(&r.ProfileID, &r.Rating, &r.HighestRating, &r.Games, &r.Wins, &r.Losses, &r.Streak, &r.StatID, &lastMatch); err != nil {
			continue
		}
		r.LastMatch = time.Unix(lastMatch, 0)
		r.Rank = int64(start + len(list) + 1)
		list = append(list, r)
	}
	return list, total
}

// GetMMR returns the hidden matchmaking rating of the requested profiles for a
// queue. It is never reset by a season boundary: the official service keeps it
// so players keep being matched by skill after the visible ladder resets, and
// Quick Match has one even though it shows no visible rating. Titles without
// seasons use the visible ladder as their only skill estimate.
func (s *MatchHistoryStore) GetMMR(queueID int32, leaderboardID int32, profiles []int32) map[int32]int32 {
	out := make(map[int32]int32, len(profiles))
	for _, profile := range profiles {
		out[profile] = DefaultRating
	}
	if queueID <= 0 || len(profiles) == 0 {
		return out
	}
	if !GameHasRankedSeasons(s.gameID) {
		if leaderboardID <= 0 {
			return out
		}
		for profile, rating := range s.GetRatingsForLeaderboard(leaderboardID, profiles) {
			if rating != nil && rating.Rating > 0 {
				out[profile] = rating.Rating
			}
		}
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return out
	}
	for _, profile := range profiles {
		var mmr int32
		err := s.db.QueryRow(
			`SELECT mmr FROM mmr WHERE queue_id = ? AND profile_id = ?`,
			queueID, profile,
		).Scan(&mmr)
		if err == nil && mmr > 0 {
			out[profile] = mmr
		}
	}
	return out
}

func (s *MatchHistoryStore) GetDefaultRating() int32 { return DefaultRating }

func (s *MatchHistoryStore) EnsureRating(profileID int32, statID int32) *PlayerRating {
	s.mu.Lock()
	defer s.mu.Unlock()
	leaderboardID := s.defaultLeaderboard()
	season := s.season()
	if s.db == nil {
		return &PlayerRating{ProfileID: profileID, LeaderboardID: s.defaultLeaderboard(), StatID: statID, Rating: DefaultRating, HighestRating: DefaultRating}
	}
	if _, err := s.db.Exec(
		`INSERT INTO ratings (leaderboard_id, season, profile_id, stat_id, rating, highest_rating)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(leaderboard_id, season, profile_id) DO UPDATE SET stat_id = CASE WHEN excluded.stat_id <> 0 THEN excluded.stat_id ELSE ratings.stat_id END`,
		leaderboardID, season, profileID, statID, DefaultRating, DefaultRating,
	); err != nil {
		logger.Printf("matchHistory: ensure rating %d: %s\n", profileID, err)
	}
	ratings := s.getRatingsLocked(leaderboardID, []int32{profileID})
	return ratings[profileID]
}

// UpsertProfile remembers who a profile ID belongs to. It is called whenever
// the server builds a user, so the ladder can still render a player that is
// offline.
func (s *MatchHistoryStore) UpsertProfile(p StoredProfile) {
	if p.ProfileID == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return
	}
	isXbox := 0
	if p.IsXbox {
		isXbox = 1
	}
	if _, err := s.db.Exec(
		`INSERT INTO profiles (profile_id, stat_group_id, alias, platform_user_id, is_xbox, avatar_metadata, xp, level, region, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(profile_id) DO UPDATE SET
		     stat_group_id    = CASE WHEN excluded.stat_group_id <> 0 THEN excluded.stat_group_id ELSE profiles.stat_group_id END,
		     alias            = CASE WHEN excluded.alias <> '' THEN excluded.alias ELSE profiles.alias END,
		     platform_user_id = CASE WHEN excluded.platform_user_id <> 0 THEN excluded.platform_user_id ELSE profiles.platform_user_id END,
		     is_xbox          = excluded.is_xbox,
		     avatar_metadata  = CASE WHEN excluded.avatar_metadata <> '' THEN excluded.avatar_metadata ELSE profiles.avatar_metadata END,
		     xp               = excluded.xp,
		     level            = CASE WHEN excluded.level <> 0 THEN excluded.level ELSE profiles.level END,
		     region           = excluded.region,
		     updated_at       = excluded.updated_at`,
		p.ProfileID,
		p.StatGroupID,
		p.Alias,
		int64(p.PlatformUserID),
		isXbox,
		p.AvatarMetadata,
		int64(p.XP),
		int64(p.Level),
		int64(p.Region),
		time.Now().Unix(),
	); err != nil {
		logger.Printf("matchHistory: upsert profile %d: %s\n", p.ProfileID, err)
	}
}

// BackfillProfiles scans user data files and populates the profiles table
// for players that have ratings but no stored profile yet.
func (s *MatchHistoryStore) BackfillProfiles(userDataDir string) {
	if userDataDir == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return
	}

	// Find all user directories/files
	entries, err := os.ReadDir(userDataDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		// Skip non-user entries
		if strings.HasSuffix(name, ".db") || strings.HasSuffix(name, ".db-shm") || strings.HasSuffix(name, ".db-wal") || strings.HasSuffix(name, ".migrated") {
			continue
		}
		if !strings.HasPrefix(name, "STEAM_") && !strings.HasPrefix(name, "XBOX_") {
			continue
		}

		var userDir string
		if entry.IsDir() {
			userDir = filepath.Join(userDataDir, name)
		} else {
			// single file user data, skip for now
			continue
		}

		// Try to read profile from files
		alias := ""
		avatarMeta := ""
		statGroupID := int32(0)
		profileID := int32(0)

		// Try to find profile.json or read from stored data
		// Check for profile in common files
		if data, err := os.ReadFile(filepath.Join(userDir, "profile.json")); err == nil {
			var p map[string]interface{}
			if json.Unmarshal(data, &p) == nil {
				if v, ok := p["alias"].(string); ok {
					alias = v
				}
				if v, ok := p["profileId"].(float64); ok {
					profileID = int32(v)
				} else if v, ok := p["id"].(float64); ok {
					profileID = int32(v)
				}
				if v, ok := p["statGroupId"].(float64); ok {
					statGroupID = int32(v)
				}
			}
		}

		// Try avatarMetadata
		if data, err := os.ReadFile(filepath.Join(userDir, "avatarMetadata.json")); err == nil {
			var v struct {
				Data string `json:"data"`
			}
			if json.Unmarshal(data, &v) == nil {
				avatarMeta = v.Data
			} else {
				var raw string
				if json.Unmarshal(data, &raw) == nil {
					avatarMeta = raw
				}
			}
		}

		// Try to extract profile ID from directory name or from files
		if profileID == 0 {
			// Try to find in any json file
			files, _ := os.ReadDir(userDir)
			for _, f := range files {
				if strings.HasSuffix(f.Name(), ".json") {
					data, err := os.ReadFile(filepath.Join(userDir, f.Name()))
					if err != nil {
						continue
					}
					var obj map[string]interface{}
					if json.Unmarshal(data, &obj) == nil {
						if v, ok := obj["profileId"].(float64); ok && profileID == 0 {
							profileID = int32(v)
						}
						if v, ok := obj["id"].(float64); ok && profileID == 0 {
							profileID = int32(v)
						}
						if v, ok := obj["statGroupId"].(float64); ok && statGroupID == 0 {
							statGroupID = int32(v)
						}
						if v, ok := obj["alias"].(string); ok && alias == "" {
							alias = v
						}
					}
				}
			}
		}

		if profileID == 0 || alias == "" {
			continue
		}

		platformUserID := int64(0)
		isXbox := 0
		if strings.HasPrefix(name, "XBOX_") {
			isXbox = 1
		} else if strings.HasPrefix(name, "STEAM_") {
			// extract steam id
			if idStr := strings.TrimPrefix(name, "STEAM_"); idStr != "" {
				if id, err := strconv.ParseUint(idStr, 10, 64); err == nil && id <= math.MaxInt64 {
					platformUserID = int64(id)
				}
			}
		}

		if _, err := s.db.Exec(
			`INSERT INTO profiles (profile_id, stat_group_id, alias, platform_user_id, is_xbox, avatar_metadata, xp, level, region, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(profile_id) DO UPDATE SET
			     alias = CASE WHEN excluded.alias <> '' THEN excluded.alias ELSE profiles.alias END,
			     stat_group_id = CASE WHEN excluded.stat_group_id <> 0 THEN excluded.stat_group_id ELSE profiles.stat_group_id END,
			     avatar_metadata = CASE WHEN excluded.avatar_metadata <> '' THEN excluded.avatar_metadata ELSE profiles.avatar_metadata END,
			     platform_user_id = CASE WHEN excluded.platform_user_id <> 0 THEN excluded.platform_user_id ELSE profiles.platform_user_id END,
			     is_xbox = excluded.is_xbox,
			     updated_at = excluded.updated_at`,
			profileID,
			statGroupID,
			alias,
			platformUserID,
			isXbox,
			avatarMeta,
			0, 1, 0,
			time.Now().Unix(),
		); err != nil {
			logger.Printf("matchHistory: backfill profile %d: %s\n", profileID, err)
		}
	}
	logger.Printf("matchHistory: backfilled profiles from %s\n", userDataDir)
}

// GetProfiles returns the stored identity of the requested profiles. Profiles
// that were never seen are absent from the map.
func (s *MatchHistoryStore) GetProfiles(profileIDs []int32) map[int32]*StoredProfile {
	out := make(map[int32]*StoredProfile, len(profileIDs))
	if len(profileIDs) == 0 {
		return out
	}
	wanted := make(map[int32]struct{}, len(profileIDs))
	for _, id := range profileIDs {
		wanted[id] = struct{}{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return out
	}
	rows, err := s.db.Query(
		`SELECT profile_id, stat_group_id, alias, platform_user_id, is_xbox, avatar_metadata, xp, level, region
		 FROM profiles`,
	)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		p := &StoredProfile{}
		var platformUserID, xp, level, region int64
		var isXbox int
		if err = rows.Scan(&p.ProfileID, &p.StatGroupID, &p.Alias, &platformUserID, &isXbox, &p.AvatarMetadata, &xp, &level, &region); err != nil {
			continue
		}
		if _, ok := wanted[p.ProfileID]; !ok {
			continue
		}
		p.PlatformUserID = uint64(platformUserID)
		p.IsXbox = isXbox != 0
		p.XP = uint32(xp)
		p.Level = uint16(level)
		p.Region = uint8(region)
		out[p.ProfileID] = p
	}
	return out
}
