package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luskaner/ageServer/server/internal/logger"
)

// ChallengeProgress stores the per-user state returned by the AoE IV challenge service.
// A map preserves fields introduced by other game builds.
type ChallengeProgress map[string]any

// ChallengeFormat describes the JSON layout of a game's challenges.json.
type ChallengeFormat int

const (
	// FormatAoE4 is AoE IV's wrapper format: [result, seasons, definitions, signature]
	FormatAoE4 ChallengeFormat = iota
	// FormatFlatArray is AoE II / AoM's flat array of 11-field challenge objects
	FormatFlatArray
)

type ChallengeProgressUpgradableDefaultData struct {
	InitialUpgradableDefaultData[*[]ChallengeProgress]
	challengesData []byte
	format         ChallengeFormat
}

func NewChallengeProgressUpgradableDefaultData(challengesData []byte, format ChallengeFormat) *ChallengeProgressUpgradableDefaultData {
	return &ChallengeProgressUpgradableDefaultData{
		challengesData: challengesData,
		format:         format,
	}
}

// challengeDefinitionsPath points to the challenge definitions the client is
// served. Tests may replace it to read the resources of the repository.
var challengeDefinitionsPath = filepath.Join(ResponsesFolder, "age4", "challenges.json")

// challengeDefinitionIDs reads the source-of-truth challenge list at runtime.
func (d *ChallengeProgressUpgradableDefaultData) challengeDefinitionIDs() ([]int64, error) {
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

	switch d.format {
	case FormatAoE4:
		var root []json.RawMessage
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, fmt.Errorf("decode AoE IV challenges root: %w", err)
		}
		if len(root) < 3 {
			return nil, fmt.Errorf("AoE IV challenges JSON has %d root fields; expected definitions at index 2", len(root))
		}
		var rows [][]json.RawMessage
		if err := json.Unmarshal(root[2], &rows); err != nil {
			return nil, fmt.Errorf("decode AoE IV challenge definitions: %w", err)
		}
		return extractDefinitionIDs(rows)
	case FormatFlatArray:
		var rows [][]json.RawMessage
		if err := json.Unmarshal(raw, &rows); err != nil {
			return nil, fmt.Errorf("decode flat challenge array: %w", err)
		}
		return extractDefinitionIDs(rows)
	default:
		return nil, fmt.Errorf("unknown challenge format %d", d.format)
	}
}

// extractDefinitionIDs pulls the first field (definition ID) from each row.
func extractDefinitionIDs(rows [][]json.RawMessage) ([]int64, error) {
	ids := make([]int64, 0, len(rows))
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
		ids = append(ids, id)
	}
	return ids, nil
}

// Default builds one progress record per challenge the current resources
// declare. A profile that cannot be read is not fatal: the key is created
// empty and the next load retries, so a missing or broken resource does not
// take the whole user down.
func (d *ChallengeProgressUpgradableDefaultData) Default() *[]ChallengeProgress {
	empty := make([]ChallengeProgress, 0)
	ids, err := d.challengeDefinitionIDs()
	if err != nil {
		logger.Fail("Challenge progress: %s", err)
		return &empty
	}
	now := time.Now().Unix()
	progress := make([]ChallengeProgress, 0, len(ids))
	for index, definitionID := range ids {
		progress = append(progress, ChallengeProgress{
			"progressID":              index + 1,
			"definition.definitionID": definitionID,
			"status":                  1,
			"progress":                0,
			"expiryTime":              -1,
			"lastCompletedTime":       now,
			"assignedTime":            now,
		})
	}
	return &progress
}

// challengeDefinitionID returns the challenge definition a stored record
// belongs to. Records are decoded from JSON, so the ID is usually a float64
// even though Default builds it as an int64.
func challengeDefinitionID(entry ChallengeProgress) (int64, bool) {
	switch id := entry["definition.definitionID"].(type) {
	case int64:
		return id, true
	case int:
		return int64(id), true
	case float64:
		return int64(id), true
	case json.Number:
		value, err := id.Int64()
		return value, err == nil
	}
	return 0, false
}

// reconcileChallengeProgress keeps the saved state of every challenge that is
// still declared by the current resources and adds the ones introduced since
// the save, so a resource update is picked up without discarding progress.
// progressID is renumbered to stay sequential, as the client expects.
func reconcileChallengeProgress(stored []ChallengeProgress, defaults *ChallengeProgressUpgradableDefaultData) []ChallengeProgress {
	defs := defaults.Default()
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
			merged = append(merged, entry)
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
			merged = append(merged, kept)
			delete(saved, id)
		}
	}
	merged = append(merged, others...)
	for index, entry := range merged {
		entry["progressID"] = index + 1
	}
	return merged
}
