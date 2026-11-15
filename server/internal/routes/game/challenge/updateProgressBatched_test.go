package challenge

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/luskaner/ageServer/server/internal/models"
)

func TestDecodeBatchedProgressPayloadForm(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/game/challenge/updateProgressBatched",
		strings.NewReader("progressIDs=%5B1875400577%2C1875400553%5D&updateAmounts=%5B1%2C3%5D"),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	progressIDs, updateAmounts, err := decodeBatchedProgressPayload(request)
	if err != nil {
		t.Fatalf("decodeBatchedProgressPayload: %s", err)
	}
	if len(progressIDs) != 2 || progressIDs[0] != 1875400577 || progressIDs[1] != 1875400553 {
		t.Fatalf("progressIDs = %v", progressIDs)
	}
	if len(updateAmounts) != 2 || updateAmounts[0] != 1 || updateAmounts[1] != 3 {
		t.Fatalf("updateAmounts = %v", updateAmounts)
	}
}

func TestDecodeBatchedProgressPayloadJSON(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/game/challenge/updateProgressBatched",
		strings.NewReader(`{"progressIDs":[7],"updateAmounts":[2]}`),
	)
	request.Header.Set("Content-Type", "application/json")
	progressIDs, updateAmounts, err := decodeBatchedProgressPayload(request)
	if err != nil {
		t.Fatalf("decodeBatchedProgressPayload: %s", err)
	}
	if len(progressIDs) != 1 || progressIDs[0] != 7 {
		t.Fatalf("progressIDs = %v", progressIDs)
	}
	if len(updateAmounts) != 1 || updateAmounts[0] != 2 {
		t.Fatalf("updateAmounts = %v", updateAmounts)
	}
}

// A completed challenge needs no write, which is the common case because the
// server unlocks every challenge.
func TestHasPendingProgress(t *testing.T) {
	completed := models.ChallengeProgress{
		"progressID": int64(1),
		models.ChallengeProgressFieldDefinitionID:    int64(847),
		models.ChallengeProgressFieldCompletionCount: int64(1),
	}
	pending := models.ChallengeProgress{
		"progressID": int64(2),
		models.ChallengeProgressFieldDefinitionID:    int64(849),
		models.ChallengeProgressFieldCompletionCount: int64(0),
	}
	records := []models.ChallengeProgress{completed, pending}
	if hasPendingProgress(records, []int64{1}) {
		t.Fatal("a completed challenge was reported as pending")
	}
	if !hasPendingProgress(records, []int64{1, 2}) {
		t.Fatal("a pending challenge was not detected")
	}
	if hasPendingProgress(records, []int64{99}) {
		t.Fatal("an unknown progress ID was reported as pending")
	}
}
