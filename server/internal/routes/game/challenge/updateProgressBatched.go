package challenge

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// decodeBatchedProgressPayload reads the progressIDs/updateAmounts document of
// a client. AoE IV posts it as form data (progressIDs=[1,2]&updateAmounts=[1,1]),
// while other builds send the same fields as JSON, so both shapes are accepted.
func decodeBatchedProgressPayload(r *http.Request) (progressIDs []int64, updateAmounts []int64, err error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, err
	}
	fields := make(map[string]json.RawMessage)
	if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && trimmed[0] == '{' && json.Unmarshal(trimmed, &fields) == nil {
		// JSON body: read the fields from it.
	} else {
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err = r.ParseForm(); err != nil {
			return nil, nil, err
		}
		for key, values := range r.PostForm {
			if len(values) == 0 {
				continue
			}
			fields[key] = json.RawMessage(values[0])
		}
	}
	progressIDs, _ = decodeInt64List(fields["progressIDs"])
	updateAmounts, _ = decodeInt64List(fields["updateAmounts"])
	return progressIDs, updateAmounts, nil
}

// decodeInt64List decodes a JSON array of numbers, ignoring the entries that
// are not numbers.
func decodeInt64List(raw json.RawMessage) ([]int64, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var values []json.Number
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, false
	}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if parsed, err := value.Int64(); err == nil {
			result = append(result, parsed)
		}
	}
	return result, true
}

// challengeTargets returns the counter each challenge definition of the game
// must reach to be completed.
func challengeTargets(game models.Game) map[int64]int64 {
	if game == nil || game.Resources() == nil {
		return nil
	}
	challengesData := models.ChallengeDefinitionsData(game.Resources())
	if challengesData == nil {
		return nil
	}
	upgrader := models.NewChallengeProgressUpgradableDefaultData(challengesData, models.ChallengeFormatForGame(game.Title()))
	return upgrader.ChallengeTargets()
}

// hasPendingProgress reports whether any of the given progress IDs belongs to a
// challenge that is not completed yet. The server unlocks everything, so this
// is normally false and the update can be acknowledged without writing the
// user document.
func hasPendingProgress(data []models.ChallengeProgress, progressIDs []int64) bool {
	wanted := make(map[int64]struct{}, len(progressIDs))
	for _, progressID := range progressIDs {
		wanted[progressID] = struct{}{}
	}
	for _, entry := range data {
		progressID, found := models.ChallengeProgressID(entry)
		if !found {
			continue
		}
		if _, ok := wanted[progressID]; !ok {
			continue
		}
		if !models.ChallengeProgressCompleted(entry) {
			return true
		}
	}
	return false
}

// UpdateProgressBatched applies the progress the client reports for a batch of
// challenges.
//
// The server runs with every challenge completed, so an update can only confirm
// what is already stored: the amount is added to the counter of the challenge
// and, once it reaches the target of the definition, the challenge stays
// completed. A completed challenge is never reverted to pending by an update,
// and an unknown progress ID is ignored. The client expects a bare [0] answer.
func UpdateProgressBatched(w http.ResponseWriter, r *http.Request) {
	progressIDs, updateAmounts, err := decodeBatchedProgressPayload(r)
	if err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	if len(progressIDs) == 0 {
		i.JSON(&w, i.A{0})
		return
	}
	game := models.G(r)
	sess := models.SessionOrPanic(r)
	user, ok := game.Users().GetUserById(sess.GetUserId())
	if !ok || user.GetChallengeProgress() == nil {
		i.JSON(&w, i.A{2})
		return
	}
	// Every challenge is completed by default, so the common case has nothing
	// to apply. Checking it read-only avoids rewriting the user document on
	// every batch the client sends during a match.
	pending := false
	if err = user.GetChallengeProgress().WithReadOnly(func(data *[]models.ChallengeProgress) error {
		if data != nil {
			pending = hasPendingProgress(*data, progressIDs)
		}
		return nil
	}); err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	if !pending {
		i.JSON(&w, i.A{0})
		return
	}
	var targets map[int64]int64
	err = user.GetChallengeProgress().WithReadWrite(func(data *[]models.ChallengeProgress) error {
		if data == nil {
			return nil
		}
		byID := make(map[int64]int, len(*data))
		for index, entry := range *data {
			if progressID, found := models.ChallengeProgressID(entry); found {
				byID[progressID] = index
			}
		}
		for position, progressID := range progressIDs {
			index, found := byID[progressID]
			if !found {
				continue
			}
			entry := (*data)[index]
			// The server runs with every challenge completed, so a completed
			// record is already in its final state. Skipping it here also
			// avoids parsing the challenge definitions for the common case.
			if models.ChallengeProgressCompleted(entry) {
				continue
			}
			var amount int64
			if position < len(updateAmounts) {
				amount = updateAmounts[position]
			}
			definitionID, hasDefinition := models.ChallengeProgressDefinitionID(entry)
			target := int64(1)
			if hasDefinition {
				if targets == nil {
					targets = challengeTargets(game)
				}
				if value, ok := targets[definitionID]; ok {
					target = value
				}
			}
			models.ApplyChallengeProgressUpdate(entry, amount, target)
		}
		return nil
	})
	if err != nil {
		i.JSON(&w, i.A{2})
		return
	}
	i.JSON(&w, i.A{0})
}
