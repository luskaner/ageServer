package shared

import (
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// GetChallengeProgressData returns the persisted progress for a specific user,
// encoded the way the client expects it: one positional array per challenge,
// [progressID, definitionID, completionCount, progress, expiryTime,
// lastCompletedTime, assignedTime].
//
// A missing/legacy value is represented as an empty list, never as shared state.
func GetChallengeProgressData(user models.User) i.A {
	if user == nil || user.GetChallengeProgress() == nil {
		return i.A{}
	}
	result := i.A{}
	_ = user.GetChallengeProgress().WithReadOnly(func(data *[]models.ChallengeProgress) error {
		result = models.EncodeChallengeProgress(*data)
		return nil
	})
	return result
}
