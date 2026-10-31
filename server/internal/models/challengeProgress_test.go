package models

import (
	"os"
	"path/filepath"
	"testing"
)

// testChallengeDefinitionsPath points at the resources of the repository, as
// tests run with the package directory as working directory.
var testChallengeDefinitionsPath = filepath.Join("..", "..", "resources", "responses", "age4", "challenges.json")

func useTestChallengeDefinitions(t *testing.T) {
	t.Helper()
	previous := challengeDefinitionsPath
	challengeDefinitionsPath = testChallengeDefinitionsPath
	t.Cleanup(func() { challengeDefinitionsPath = previous })
}

func TestChallengeProgressDefaultsIncludeEveryDefinition(t *testing.T) {
	useTestChallengeDefinitions(t)
	testData, err := os.ReadFile(testChallengeDefinitionsPath)
	if err != nil {
		t.Fatal(err)
	}
	upgrader := NewChallengeProgressUpgradableDefaultData(testData, FormatAoE4)
	ids, err := upgrader.challengeDefinitionIDs()
	if err != nil {
		t.Fatal(err)
	}
	defaults := upgrader.Default()
	if len(*defaults) != len(ids) {
		t.Fatalf("got %d progress records for %d definitions", len(*defaults), len(ids))
	}
	seen := make(map[int]bool, len(*defaults))
	for i, entry := range *defaults {
		if entry["progressID"] != i+1 {
			t.Fatalf("record %d has non-sequential progressID: %#v", i, entry["progressID"])
		}
		if entry["definition.definitionID"] == nil {
			t.Fatalf("record %d has no definition ID", i)
		}
		if entry["status"] != 1 {
			t.Fatalf("record %d is not marked completed", i)
		}
		id := int(entry["definition.definitionID"].(int64))
		if seen[id] {
			t.Fatalf("duplicate definition ID %d", id)
		}
		seen[id] = true
	}
}

// A resource update must add the new challenges without discarding the state
// the client already reported for the existing ones.
func TestReconcileChallengeProgressKeepsSavedState(t *testing.T) {
	useTestChallengeDefinitions(t)
	testData, err := os.ReadFile(testChallengeDefinitionsPath)
	if err != nil {
		t.Fatal(err)
	}
	upgrader := NewChallengeProgressUpgradableDefaultData(testData, FormatAoE4)
	defaults := upgrader.Default()
	if len(*defaults) < 2 {
		t.Skip("not enough challenge definitions")
	}
	first := (*defaults)[0]
	firstID, ok := challengeDefinitionID(first)
	if !ok {
		t.Fatal("default record has no definition ID")
	}
	// Simulate what the client saved: JSON numbers, a completed status and the
	// last definition removed from the current resources.
	stored := []ChallengeProgress{
		{
			"progressID":              float64(1),
			"definition.definitionID": float64(firstID),
			"status":                  float64(2),
			"progress":                float64(7),
		},
		{
			"progressID":              float64(2),
			"definition.definitionID": float64(-1),
			"status":                  float64(2),
		},
	}
	merged := reconcileChallengeProgress(stored, upgrader)
	if len(merged) != len(*defaults)+1 {
		t.Fatalf("got %d records, want %d", len(merged), len(*defaults)+1)
	}
	if merged[0]["status"] != float64(2) || merged[0]["progress"] != float64(7) {
		t.Fatalf("saved state was not kept: %#v", merged[0])
	}
	for i, entry := range merged {
		if entry["progressID"] != i+1 {
			t.Fatalf("record %d has non-sequential progressID: %#v", i, entry["progressID"])
		}
	}
	if last := merged[len(merged)-1]; last["definition.definitionID"] != float64(-1) {
		t.Fatalf("removed challenge was dropped: %#v", merged)
	}
}
