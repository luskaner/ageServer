package shared

import (
	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/models"
)

// GetChallengeProgressData returns the persisted progress for a specific user.
// A missing/legacy value is represented as an empty list, never as shared state.
func GetChallengeProgressData(user models.User) i.A {
	if user == nil || user.GetChallengeProgress() == nil {
		return i.A{}
	}
	var result i.A = i.A{}
	_ = user.GetChallengeProgress().WithReadOnly(func(data *[]models.ChallengeProgress) error {
		for _, entry := range *data {
			result = append(result, entry)
		}
		return nil
	})
	return result
}
