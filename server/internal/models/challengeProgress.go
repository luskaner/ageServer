package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luskaner/ageServer/server/internal/logger"
)

// Field names of a stored ChallengeProgress record.
//
// The client sends and receives these fields positionally (see
// EncodeChallengeProgressEntry); the names below are the ones the official
// service uses and are only used to keep the persisted document readable.
const (
	// ChallengeProgressFieldDefinitionID is the challenge definition the record
	// belongs to (root[2] of challenges.json).
	ChallengeProgressFieldDefinitionID = "definition.definitionID"
	// ChallengeProgressFieldCompletionCount is how many times the challenge has been
	// completed. 1 means completed; the official service never sends a higher
	// value because a completed challenge is not completed again.
	ChallengeProgressFieldCompletionCount = "completionCount"
	// ChallengeProgressFieldProgress is the counter towards the target of the
	// definition. The official service leaves it at 0 once the challenge is
	// completed.
	ChallengeProgressFieldProgress = "progress"
	// ChallengeProgressFieldExpiryTime is the expiry of the challenge, or nil when it
	// does not expire.
	ChallengeProgressFieldExpiryTime = "expiryTime"
	// ChallengeProgressFieldLastCompletedTime is when the challenge was last
	// completed, or nil while it is still pending.
	ChallengeProgressFieldLastCompletedTime = "lastCompletedTime"
	// ChallengeProgressFieldAssignedTime is when the challenge was handed to the
	// player.
	ChallengeProgressFieldAssignedTime = "assignedTime"
	// legacyChallengeProgressStatus is the field name older builds of this
	// server persisted the completion state under. It is only read, to migrate
	// those documents.
	legacyChallengeProgressStatus = "status"
)

// ChallengeProgress stores the per-user state returned by the AoE IV challenge service.
// A map preserves fields introduced by other game builds.
type ChallengeProgress map[string]any

// ChallengeFormat describes the JSON layout of a game's challenges.json.
type ChallengeFormat int

const (
	// FormatAoE4 is the wrapper layout every current game uses:
	// [result, groups, definitions, signature].
	FormatAoE4 ChallengeFormat = iota
	// FormatFlatArray is the historical AoE II / AoM layout: the definitions
	// are the root array. Current resources wrap them like AoE IV; the parser
	// accepts both and uses this value only as a hint for which one to try
	// first.
	FormatFlatArray
)

// challengeProgressVersion is the version of the persisted challenge document.
//
// Version 1 stored the completion state under "status" and an expiry of -1 for
// challenges that never expire. Version 2 uses the field names and the nil
// expiry of the official service, and defaults every challenge to completed.
const challengeProgressVersion uint32 = 2

type ChallengeProgressUpgradableDefaultData struct {
	InitialUpgradableData[*[]ChallengeProgress]
	challengesData []byte
	format         ChallengeFormat
}

func NewChallengeProgressUpgradableDefaultData(challengesData []byte, format ChallengeFormat) *ChallengeProgressUpgradableDefaultData {
	return &ChallengeProgressUpgradableDefaultData{
		challengesData: challengesData,
		format:         format,
	}
}

// CurrentVersion reports the version of the documents Default builds.
func (d *ChallengeProgressUpgradableDefaultData) CurrentVersion() uint32 {
	return challengeProgressVersion
}

// ObjectOfVersion returns the document of a stored version, ready to be
// upgraded. Version 1 and 2 share the same layout, so the decoded document is
// used as is.
func (d *ChallengeProgressUpgradableDefaultData) ObjectOfVersion(_ uint32) any {
	empty := make([]ChallengeProgress, 0)
	return &empty
}

// UpgradeToNextVersion migrates a stored document one version forward.
func (d *ChallengeProgressUpgradableDefaultData) UpgradeToNextVersion(oldVersion uint32, oldObject any) *[]ChallengeProgress {
	stored, ok := oldObject.(*[]ChallengeProgress)
	if !ok || stored == nil {
		empty := make([]ChallengeProgress, 0)
		return &empty
	}
	switch oldVersion {
	case 1:
		// Version 1 is the same layout with the completion state under a
		// different name and -1 as "no expiry"; both are rewritten when the
		// document is reconciled, so only the rename is needed here.
		for _, entry := range *stored {
			if _, exists := entry[ChallengeProgressFieldCompletionCount]; !exists {
				if status, hasStatus := entry[legacyChallengeProgressStatus]; hasStatus {
					entry[ChallengeProgressFieldCompletionCount] = status
				}
			}
			delete(entry, legacyChallengeProgressStatus)
		}
	}
	return stored
}

// challengeDefinitionsPath points to the challenge definitions the client is
// served. Tests may replace it to read the resources of the repository.
var challengeDefinitionsPath = filepath.Join(ResponsesFolder, "age4", "challenges.json")

// challengeDefinition is the part of a challenge definition the progress
// records need: the definition itself and the counter it must reach to be
// completed.
type challengeDefinition struct {
	id     int64
	target int64
}

// challengeDefinitions reads the source-of-truth challenge list at runtime.
func (d *ChallengeProgressUpgradableDefaultData) challengeDefinitions() ([]challengeDefinition, error) {
	var raw []byte
	if d.challengesData != nil {
		raw = d.challengesData
	} else {
		var err error
		raw, err = os.ReadFile(challengeDefinitionsPath)
		if err != nil {
			return nil, fmt.Errorf("read AoE IV challenges from %s: %w", challengeDefinitionsPath, err)
		}
	}

	var rows [][]json.RawMessage
	var err error
	if rows, err = decodeChallengeRows(raw, d.format); err != nil {
		return nil, err
	}
	return extractChallengeDefinitions(rows)
}

// decodeChallengeRows accepts the two layouts the resources have used: the
// current wrapper [result, groups, definitions, signature] and the historical
// flat array of definition rows. The format is only a hint for which one to try
// first, so a resource update that changes the layout still loads.
func decodeChallengeRows(raw []byte, format ChallengeFormat) ([][]json.RawMessage, error) {
	var root []json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("decode challenges root: %w", err)
	}
	wrapper := func() ([][]json.RawMessage, bool) {
		if len(root) < 3 {
			return nil, false
		}
		var rows [][]json.RawMessage
		if err := json.Unmarshal(root[2], &rows); err != nil {
			return nil, false
		}
		return rows, true
	}
	flat := func() ([][]json.RawMessage, bool) {
		var rows [][]json.RawMessage
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, false
		}
		return rows, true
	}
	if format == FormatFlatArray {
		if rows, ok := flat(); ok {
			return rows, nil
		}
		if rows, ok := wrapper(); ok {
			return rows, nil
		}
	} else {
		if rows, ok := wrapper(); ok {
			return rows, nil
		}
		if rows, ok := flat(); ok {
			return rows, nil
		}
	}
	return nil, fmt.Errorf("decode challenge definitions: unsupported layout")
}

// extractChallengeDefinitions pulls the definition ID and the target of each
// row. A row whose target cannot be read (or is not positive) falls back to 1,
// which is what a single-step challenge requires.
func extractChallengeDefinitions(rows [][]json.RawMessage) ([]challengeDefinition, error) {
	definitions := make([]challengeDefinition, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		var id int64
		if err := json.Unmarshal(row[0], &id); err != nil {
			return nil, fmt.Errorf("challenge definition row %d has invalid ID: %w", i, err)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		definition := challengeDefinition{id: id, target: 1}
		if len(row) > challengeTargetIndex {
			if err := json.Unmarshal(row[challengeTargetIndex], &definition.target); err != nil || definition.target <= 0 {
				definition.target = 1
			}
		}
		definitions = append(definitions, definition)
	}
	return definitions, nil
}

// challengeDefinitionIDs returns only the IDs of the definitions.
func (d *ChallengeProgressUpgradableDefaultData) challengeDefinitionIDs() ([]int64, error) {
	definitions, err := d.challengeDefinitions()
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(definitions))
	for _, definition := range definitions {
		ids = append(ids, definition.id)
	}
	return ids, nil
}

// Default builds one completed progress record per challenge the current
// resources declare, so every challenge and every mastery is unlocked as soon
// as the profile is created. A profile that cannot be read is not fatal: the
// key is created empty and the next load retries, so a missing or broken
// resource does not take the whole user down.
func (d *ChallengeProgressUpgradableDefaultData) Default() *[]ChallengeProgress {
	empty := make([]ChallengeProgress, 0)
	definitions, err := d.challengeDefinitions()
	if err != nil {
		logger.Fail("Challenge progress: %s", err)
		return &empty
	}
	now := time.Now().Unix()
	progress := make([]ChallengeProgress, 0, len(definitions))
	for index, definition := range definitions {
		progress = append(progress, completedChallengeProgress(index+1, definition.id, now))
	}
	return &progress
}

// completedChallengeProgress builds the record of a completed challenge. It
// mirrors the official service: completionCount 1, progress 0, no expiry, and
// a completion time after the assignment time.
func completedChallengeProgress(progressID int, definitionID int64, now int64) ChallengeProgress {
	return ChallengeProgress{
		"progressID":                            progressID,
		ChallengeProgressFieldDefinitionID:      definitionID,
		ChallengeProgressFieldCompletionCount:   1,
		ChallengeProgressFieldProgress:          0,
		ChallengeProgressFieldExpiryTime:        nil,
		ChallengeProgressFieldLastCompletedTime: now,
		ChallengeProgressFieldAssignedTime:      now - 1,
	}
}

// challengeDefinitionID returns the challenge definition a stored record
// belongs to. Records are decoded from JSON, so the ID is usually a float64
// even though Default builds it as an int64.
func challengeDefinitionID(entry ChallengeProgress) (int64, bool) {
	return challengeInt64(entry[ChallengeProgressFieldDefinitionID])
}

// challengeInt64 reads a whole number from a stored record, which may hold it
// as any of the numeric types JSON decoding and Default produce.
func challengeInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint64:
		return int64(v), true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	case json.Number:
		parsed, err := v.Int64()
		return parsed, err == nil
	}
	return 0, false
}

// ChallengeProgressID returns the progress ID of a stored record.
func ChallengeProgressID(entry ChallengeProgress) (int64, bool) {
	return challengeInt64(entry["progressID"])
}

// ChallengeProgressDefinitionID returns the definition a stored record belongs
// to.
func ChallengeProgressDefinitionID(entry ChallengeProgress) (int64, bool) {
	return challengeDefinitionID(entry)
}

// challengeCompletionCount reports how many times a stored record has been
// completed, reading the field name of older builds when the current one is
// missing.
func challengeCompletionCount(entry ChallengeProgress) int64 {
	if count, ok := challengeInt64(entry[ChallengeProgressFieldCompletionCount]); ok {
		return count
	}
	if count, ok := challengeInt64(entry[legacyChallengeProgressStatus]); ok {
		return count
	}
	return 0
}

// reconcileChallengeProgress keeps the saved state of every challenge that is
// still declared by the current resources and adds the ones introduced since
// the save, so a resource update is picked up without discarding progress.
// progressID is renumbered to stay sequential, as the client expects.
//
// The server is meant to run with everything unlocked, so every record is
// brought to the completed state the official service reports: a saved record
// that is not completed is completed, the legacy field names are migrated and
// an expiry of -1 (which the client reads as an expired challenge) becomes nil.
func reconcileChallengeProgress(stored []ChallengeProgress, defaults *ChallengeProgressUpgradableDefaultData) []ChallengeProgress {
	defs := defaults.Default()
	now := time.Now().Unix()
	saved := make(map[int64]ChallengeProgress, len(stored))
	others := make([]ChallengeProgress, 0, len(stored))
	for _, entry := range stored {
		if id, ok := challengeDefinitionID(entry); ok {
			saved[id] = entry
		} else {
			others = append(others, entry)
		}
	}
	merged := make([]ChallengeProgress, 0, len(*defs)+len(others)+len(saved))
	for _, definition := range *defs {
		id, ok := challengeDefinitionID(definition)
		if !ok {
			continue
		}
		if entry, exists := saved[id]; exists {
			delete(saved, id)
			merged = append(merged, unlockChallengeProgress(entry, now))
			continue
		}
		merged = append(merged, definition)
	}
	// Challenges the current resources no longer declare keep their saved
	// state instead of being dropped.
	for _, entry := range stored {
		id, ok := challengeDefinitionID(entry)
		if !ok {
			continue
		}
		if kept, exists := saved[id]; exists {
			merged = append(merged, unlockChallengeProgress(kept, now))
			delete(saved, id)
		}
	}
	for _, entry := range others {
		merged = append(merged, unlockChallengeProgress(entry, now))
	}
	for index, entry := range merged {
		entry["progressID"] = index + 1
	}
	return merged
}

// unlockChallengeProgress rewrites a stored record as a completed one, keeping
// the timestamps it already had when they are consistent. Every challenge is
// completed, so none of them expires: an expiry of -1 (which the client reads
// as an expired challenge) and any stored window are dropped.
func unlockChallengeProgress(entry ChallengeProgress, now int64) ChallengeProgress {
	delete(entry, legacyChallengeProgressStatus)
	entry[ChallengeProgressFieldCompletionCount] = 1
	entry[ChallengeProgressFieldProgress] = 0
	entry[ChallengeProgressFieldExpiryTime] = nil
	completed, hasCompleted := challengeInt64(entry[ChallengeProgressFieldLastCompletedTime])
	assignedTime, hasAssigned := challengeInt64(entry[ChallengeProgressFieldAssignedTime])
	if !hasCompleted {
		completed = now
		entry[ChallengeProgressFieldLastCompletedTime] = completed
	}
	if !hasAssigned {
		assignedTime = completed - 1
		entry[ChallengeProgressFieldAssignedTime] = assignedTime
	}
	if completed <= assignedTime {
		// The official service completes every challenge after assigning it.
		completed = assignedTime + 1
		entry[ChallengeProgressFieldLastCompletedTime] = completed
	}
	return entry
}

// challengeTargetIndex is the position of the target of a challenge definition
// row:
//
//	[definitionID, seasonID, name, "", displayName, description,
//	 extraDescription, target, metadataJSON, achievementID, nextDefinitionID,
//	 rewardOrderID, rewardXP]
const challengeTargetIndex = 7

// EncodeChallengeProgressEntry returns the positional form of a record the
// client expects:
//
//	[progressID, definitionID, completionCount, progress, expiryTime,
//	 lastCompletedTime, assignedTime]
//
// A challenge that does not expire is sent as nil, which is what the official
// service does; an expiry of -1 is translated because the client reads it as an
// already expired challenge.
func EncodeChallengeProgressEntry(entry ChallengeProgress) []any {
	expiry := entry[ChallengeProgressFieldExpiryTime]
	if value, ok := challengeInt64(expiry); ok && value < 0 {
		expiry = nil
	}
	completionCount := entry[ChallengeProgressFieldCompletionCount]
	if completionCount == nil {
		// A document that could not be reconciled yet may still use the name
		// of older builds; send the value under the field the client expects.
		completionCount = entry[legacyChallengeProgressStatus]
	}
	progress := entry[ChallengeProgressFieldProgress]
	if progress == nil {
		progress = int64(0)
	}
	return []any{
		entry["progressID"],
		entry[ChallengeProgressFieldDefinitionID],
		completionCount,
		progress,
		expiry,
		entry[ChallengeProgressFieldLastCompletedTime],
		entry[ChallengeProgressFieldAssignedTime],
	}
}

// EncodeChallengeProgress returns the positional form of every record, which is
// what the challenge endpoints answer with. Records without a definition ID are
// skipped: they cannot be matched to a challenge by the client.
func EncodeChallengeProgress(progress []ChallengeProgress) []any {
	encoded := make([]any, 0, len(progress))
	for _, entry := range progress {
		if _, ok := ChallengeProgressDefinitionID(entry); !ok {
			continue
		}
		encoded = append(encoded, EncodeChallengeProgressEntry(entry))
	}
	return encoded
}

// ChallengeTargets returns the counter each challenge definition must reach to
// be completed, keyed by definition ID.
func (d *ChallengeProgressUpgradableDefaultData) ChallengeTargets() map[int64]int64 {
	definitions, err := d.challengeDefinitions()
	if err != nil {
		logger.Fail("Challenge progress: %s", err)
		return nil
	}
	targets := make(map[int64]int64, len(definitions))
	for _, definition := range definitions {
		targets[definition.id] = definition.target
	}
	return targets
}

// ChallengeProgressCompleted reports whether a stored record is completed.
func ChallengeProgressCompleted(entry ChallengeProgress) bool {
	return challengeCompletionCount(entry) >= 1
}

// ApplyChallengeProgressUpdate adds amount to the counter of a challenge.
//
// The server runs with every challenge completed, so an update can only
// confirm what is already stored: a completed challenge keeps its completion
// and its counter is left at 0, while a pending one is completed once its
// counter reaches target. A negative amount never rewinds a pending challenge.
func ApplyChallengeProgressUpdate(entry ChallengeProgress, amount int64, target int64) {
	if target <= 0 {
		target = 1
	}
	if challengeCompletionCount(entry) >= 1 {
		entry[ChallengeProgressFieldCompletionCount] = 1
		entry[ChallengeProgressFieldProgress] = 0
		return
	}
	current, _ := challengeInt64(entry[ChallengeProgressFieldProgress])
	current = max(current+max(amount, 0), 0)
	if current < target {
		entry[ChallengeProgressFieldProgress] = current
		return
	}
	now := time.Now().Unix()
	entry[ChallengeProgressFieldCompletionCount] = 1
	entry[ChallengeProgressFieldProgress] = 0
	entry[ChallengeProgressFieldLastCompletedTime] = now
	if expiry, ok := challengeInt64(entry[ChallengeProgressFieldExpiryTime]); !ok || expiry < 0 {
		entry[ChallengeProgressFieldExpiryTime] = nil
	}
	if assigned, ok := challengeInt64(entry[ChallengeProgressFieldAssignedTime]); !ok || assigned >= now {
		entry[ChallengeProgressFieldAssignedTime] = now - 1
	}
}
