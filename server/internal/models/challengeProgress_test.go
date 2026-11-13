package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// testChallengeDefinitionsPath points at the resources of the repository, as
// tests run with the package directory as working directory.
var testChallengeDefinitionsPath = filepath.Join("..", "..", "resources", "responses", "age4", "challenges.json")

// crucibleMasteryDefinitionIDs are the 15 challenges of the Crucible path (846).
var crucibleMasteryDefinitionIDs = []int64{
	847, 849, 851, 529, 531, 541, 543, 545, 549, 551, 553, 555, 557, 559, 561,
}

func useTestChallengeDefinitions(t *testing.T) {
	t.Helper()
	previous := challengeDefinitionsPath
	challengeDefinitionsPath = testChallengeDefinitionsPath
	t.Cleanup(func() { challengeDefinitionsPath = previous })
}

func testChallengeUpgrader(t *testing.T) *ChallengeProgressUpgradableDefaultData {
	t.Helper()
	testData, err := os.ReadFile(testChallengeDefinitionsPath)
	if err != nil {
		t.Fatal(err)
	}
	return NewChallengeProgressUpgradableDefaultData(testData, FormatAoE4)
}

func TestChallengeProgressDefaultsIncludeEveryDefinition(t *testing.T) {
	useTestChallengeDefinitions(t)
	upgrader := testChallengeUpgrader(t)
	ids, err := upgrader.challengeDefinitionIDs()
	if err != nil {
		t.Fatal(err)
	}
	defaults := upgrader.Default()
	if len(*defaults) != len(ids) {
		t.Fatalf("got %d progress records for %d definitions", len(*defaults), len(ids))
	}
	seen := make(map[int64]bool, len(*defaults))
	for i, entry := range *defaults {
		if entry["progressID"] != i+1 {
			t.Fatalf("record %d has non-sequential progressID: %#v", i, entry["progressID"])
		}
		definitionID, ok := challengeDefinitionID(entry)
		if !ok {
			t.Fatalf("record %d has no definition ID", i)
		}
		if seen[definitionID] {
			t.Fatalf("duplicate definition ID %d", definitionID)
		}
		seen[definitionID] = true
		assertCompletedChallenge(t, i, entry)
		assertNoExpiry(t, i, entry)
	}
	// The Crucible masteries must be unlocked too, not only the linear paths.
	for _, definitionID := range crucibleMasteryDefinitionIDs {
		if !seen[definitionID] {
			t.Errorf("Crucible mastery %d has no progress record", definitionID)
		}
	}
}

// assertCompletedChallenge checks a record against the shape the official
// service reports for a completed challenge.
func assertCompletedChallenge(t *testing.T, index int, entry ChallengeProgress) {
	t.Helper()
	if _, exists := entry[legacyChallengeProgressStatus]; exists {
		t.Fatalf("record %d still uses the legacy status field: %#v", index, entry)
	}
	if count, _ := challengeInt64(entry[ChallengeProgressFieldCompletionCount]); count != 1 {
		t.Fatalf("record %d is not marked completed: %#v", index, entry[ChallengeProgressFieldCompletionCount])
	}
	if progress, _ := challengeInt64(entry[ChallengeProgressFieldProgress]); progress != 0 {
		t.Fatalf("record %d has a non-zero progress: %#v", index, entry[ChallengeProgressFieldProgress])
	}
	completed, hasCompleted := challengeInt64(entry[ChallengeProgressFieldLastCompletedTime])
	assigned, hasAssigned := challengeInt64(entry[ChallengeProgressFieldAssignedTime])
	if !hasCompleted || !hasAssigned {
		t.Fatalf("record %d lacks its timestamps: %#v", index, entry)
	}
	if completed <= assigned {
		t.Fatalf("record %d completed (%d) before it was assigned (%d)", index, completed, assigned)
	}
}

// assertNoExpiry checks that a non-daily record does not expire.
func assertNoExpiry(t *testing.T, index int, entry ChallengeProgress) {
	t.Helper()
	if expiry, exists := entry[ChallengeProgressFieldExpiryTime]; !exists || expiry != nil {
		t.Fatalf("record %d has an expiry instead of null: %#v", index, expiry)
	}
}

// The client reads the progress positionally, so the encoded entry must be
// [progressID, definitionID, completionCount, progress, expiryTime,
// lastCompletedTime, assignedTime] with a null expiry.
func TestEncodeChallengeProgressEntryMatchesOfficialShape(t *testing.T) {
	useTestChallengeDefinitions(t)
	upgrader := testChallengeUpgrader(t)
	defaults := upgrader.Default()
	encoded := EncodeChallengeProgress(*defaults)
	if len(encoded) != len(*defaults) {
		t.Fatalf("got %d encoded entries for %d records", len(encoded), len(*defaults))
	}
	body, err := json.Marshal([]any{0, encoded})
	if err != nil {
		t.Fatal(err)
	}
	var decoded []any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	entries, ok := decoded[1].([]any)
	if !ok {
		t.Fatalf("second element is not a list: %T", decoded[1])
	}
	if len(entries) != len(*defaults) {
		t.Fatalf("got %d entries, want %d", len(entries), len(*defaults))
	}
	seenCrucible := make(map[int64]bool, len(crucibleMasteryDefinitionIDs))
	for i, raw := range entries {
		fields, ok := raw.([]any)
		if !ok {
			t.Fatalf("entry %d is not a positional array: %T", i, raw)
		}
		if len(fields) != 7 {
			t.Fatalf("entry %d has %d fields, want 7", i, len(fields))
		}
		if fields[0].(float64) != float64(i+1) {
			t.Fatalf("entry %d has progressID %v", i, fields[0])
		}
		if fields[2].(float64) != 1 {
			t.Fatalf("entry %d is not completed: completionCount=%v", i, fields[2])
		}
		if fields[3].(float64) != 0 {
			t.Fatalf("entry %d has progress %v", i, fields[3])
		}
		if fields[4] != nil {
			t.Fatalf("entry %d has expiryTime %v, want null", i, fields[4])
		}
		if fields[5].(float64) <= fields[6].(float64) {
			t.Fatalf("entry %d completed before it was assigned", i)
		}
		definitionID := int64(fields[1].(float64))
		if slices.Contains(crucibleMasteryDefinitionIDs, definitionID) {
			seenCrucible[definitionID] = true
		}
	}
	for _, definitionID := range crucibleMasteryDefinitionIDs {
		if !seenCrucible[definitionID] {
			t.Errorf("Crucible mastery %d is missing from the encoded progress", definitionID)
		}
	}
}

// A resource update must add the new challenges without discarding the state
// the client already reported for the existing ones, and every record must end
// up completed.
func TestReconcileChallengeProgressKeepsSavedState(t *testing.T) {
	useTestChallengeDefinitions(t)
	upgrader := testChallengeUpgrader(t)
	defaults := upgrader.Default()
	if len(*defaults) < 2 {
		t.Skip("not enough challenge definitions")
	}
	first := (*defaults)[0]
	firstID, ok := challengeDefinitionID(first)
	if !ok {
		t.Fatal("default record has no definition ID")
	}
	// Simulate what an older build saved: JSON numbers, the legacy status
	// field, an expiry of -1 and a challenge the current resources removed.
	stored := []ChallengeProgress{
		{
			"progressID":                            float64(1),
			ChallengeProgressFieldDefinitionID:      float64(firstID),
			legacyChallengeProgressStatus:           float64(1),
			ChallengeProgressFieldProgress:          float64(0),
			ChallengeProgressFieldExpiryTime:        float64(-1),
			ChallengeProgressFieldLastCompletedTime: float64(1700000000),
			ChallengeProgressFieldAssignedTime:      float64(1699999999),
		},
		{
			"progressID":                       float64(2),
			ChallengeProgressFieldDefinitionID: float64(-1),
			legacyChallengeProgressStatus:      float64(2),
		},
	}
	merged := reconcileChallengeProgress(stored, upgrader)
	if len(merged) != len(*defaults)+1 {
		t.Fatalf("got %d records, want %d", len(merged), len(*defaults)+1)
	}
	for i, entry := range merged {
		if entry["progressID"] != i+1 {
			t.Fatalf("record %d has non-sequential progressID: %#v", i, entry["progressID"])
		}
		assertCompletedChallenge(t, i, entry)
	}
	// The saved timestamps of a challenge that was already completed are kept.
	if got, _ := challengeInt64(merged[0][ChallengeProgressFieldLastCompletedTime]); got != 1700000000 {
		t.Fatalf("saved completion time was not kept: %v", got)
	}
	if last := merged[len(merged)-1]; last[ChallengeProgressFieldDefinitionID] != float64(-1) {
		t.Fatalf("removed challenge was dropped: %#v", merged)
	}
}

// A profile saved by an older build with pending challenges must be unlocked
// on load, not only when it is created.
func TestReconcileChallengeProgressUnlocksPendingChallenges(t *testing.T) {
	useTestChallengeDefinitions(t)
	upgrader := testChallengeUpgrader(t)
	defaults := upgrader.Default()
	if len(*defaults) == 0 {
		t.Skip("no challenge definitions")
	}
	definitionID, _ := challengeDefinitionID((*defaults)[0])
	stored := []ChallengeProgress{
		{
			"progressID":                          float64(7),
			ChallengeProgressFieldDefinitionID:    float64(definitionID),
			ChallengeProgressFieldCompletionCount: float64(0),
			ChallengeProgressFieldProgress:        float64(3),
			ChallengeProgressFieldExpiryTime:      float64(-1),
		},
	}
	merged := reconcileChallengeProgress(stored, upgrader)
	if len(merged) != len(*defaults) {
		t.Fatalf("got %d records, want %d", len(merged), len(*defaults))
	}
	assertCompletedChallenge(t, 0, merged[0])
}

// The version bump must migrate a document saved with the legacy field name.
func TestChallengeProgressUpgradeRenamesLegacyStatus(t *testing.T) {
	upgrader := NewChallengeProgressUpgradableDefaultData(nil, FormatAoE4)
	if upgrader.CurrentVersion() < 2 {
		t.Fatalf("unexpected version %d", upgrader.CurrentVersion())
	}
	stored := &[]ChallengeProgress{{
		"progressID":                       float64(1),
		ChallengeProgressFieldDefinitionID: float64(561),
		legacyChallengeProgressStatus:      float64(1),
	}}
	upgraded := upgrader.UpgradeToNextVersion(1, stored)
	if len(*upgraded) != 1 {
		t.Fatalf("got %d records, want 1", len(*upgraded))
	}
	entry := (*upgraded)[0]
	if _, exists := entry[legacyChallengeProgressStatus]; exists {
		t.Fatalf("legacy field was not removed: %#v", entry)
	}
	if count, _ := challengeInt64(entry[ChallengeProgressFieldCompletionCount]); count != 1 {
		t.Fatalf("legacy status was not migrated: %#v", entry)
	}
}

// A document persisted by the previous build (version 1, legacy field name)
// must be migrated through the persistent store, not only by calling the
// upgrader directly.
func TestChallengeProgressStoredVersion1IsMigrated(t *testing.T) {
	useTestChallengeDefinitions(t)
	store := &memoryUserStore{values: map[string]StoredValue{
		"challenges": {
			Version: 1,
			Data:    json.RawMessage(`[{"progressID":1,"definition.definitionID":847,"status":1}]`),
		},
	}}
	persistentMap, err := NewPersistentStringMap(store)
	if err != nil {
		t.Fatalf("NewPersistentStringMap: %s", err)
	}
	data, err := NewPersistentJsonData[*[]ChallengeProgress](persistentMap, "challenges", testChallengeUpgrader(t))
	if err != nil {
		t.Fatalf("opening stored progress: %s", err)
	}
	if err = data.WithReadOnly(func(progress *[]ChallengeProgress) error {
		if len(*progress) != 1 {
			t.Fatalf("got %d entries, want 1", len(*progress))
		}
		entry := (*progress)[0]
		if _, exists := entry[legacyChallengeProgressStatus]; exists {
			t.Fatalf("legacy field was not removed: %#v", entry)
		}
		if count, _ := challengeInt64(entry[ChallengeProgressFieldCompletionCount]); count != 1 {
			t.Fatalf("legacy status was not migrated: %#v", entry)
		}
		return nil
	}); err != nil {
		t.Fatalf("WithReadOnly: %s", err)
	}
	stored, exists := store.values["challenges"]
	if !exists {
		t.Fatal("the migrated document was not persisted")
	}
	if stored.Version != challengeProgressVersion {
		t.Fatalf("stored version %d, want %d", stored.Version, challengeProgressVersion)
	}
	var persisted []map[string]any
	if err = json.Unmarshal(stored.Data, &persisted); err != nil {
		t.Fatalf("stored document is not JSON: %s", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("stored %d entries, want 1", len(persisted))
	}
	if _, exists := persisted[0][legacyChallengeProgressStatus]; exists {
		t.Fatalf("stored document still has the legacy field: %s", stored.Data)
	}
	if count, _ := persisted[0][ChallengeProgressFieldCompletionCount].(float64); count != 1 {
		t.Fatalf("stored document has completionCount %v", persisted[0][ChallengeProgressFieldCompletionCount])
	}
}

// A completed challenge must stay completed when the client reports progress,
// and a pending one must be completed once it reaches its target.
func TestApplyChallengeProgressUpdate(t *testing.T) {
	completed := ChallengeProgress{
		"progressID":                            int64(1),
		ChallengeProgressFieldDefinitionID:      int64(561),
		ChallengeProgressFieldCompletionCount:   int64(1),
		ChallengeProgressFieldProgress:          int64(0),
		ChallengeProgressFieldExpiryTime:        nil,
		ChallengeProgressFieldLastCompletedTime: int64(1700000000),
		ChallengeProgressFieldAssignedTime:      int64(1699999999),
	}
	ApplyChallengeProgressUpdate(completed, 1, 1)
	if count, _ := challengeInt64(completed[ChallengeProgressFieldCompletionCount]); count != 1 {
		t.Fatalf("completed challenge was reverted: %#v", completed)
	}
	if progress, _ := challengeInt64(completed[ChallengeProgressFieldProgress]); progress != 0 {
		t.Fatalf("completed challenge got a counter: %#v", completed)
	}
	if got, _ := challengeInt64(completed[ChallengeProgressFieldLastCompletedTime]); got != 1700000000 {
		t.Fatalf("completed challenge lost its completion time: %#v", completed)
	}

	pending := ChallengeProgress{
		"progressID":                       int64(2),
		ChallengeProgressFieldDefinitionID: int64(1428475),
		ChallengeProgressFieldProgress:     int64(0),
	}
	ApplyChallengeProgressUpdate(pending, 1, 5)
	if count, _ := challengeInt64(pending[ChallengeProgressFieldCompletionCount]); count != 0 {
		t.Fatalf("challenge completed before reaching its target: %#v", pending)
	}
	if progress, _ := challengeInt64(pending[ChallengeProgressFieldProgress]); progress != 1 {
		t.Fatalf("counter was not updated: %#v", pending)
	}
	ApplyChallengeProgressUpdate(pending, 4, 5)
	assertCompletedChallenge(t, 0, pending)
}

// The targets read from the resources must match the target field of each
// definition.
func TestChallengeTargets(t *testing.T) {
	useTestChallengeDefinitions(t)
	upgrader := testChallengeUpgrader(t)
	targets := upgrader.ChallengeTargets()
	if len(targets) == 0 {
		t.Fatal("no targets were read")
	}
	// Every Crucible mastery is a single-step challenge.
	for _, definitionID := range crucibleMasteryDefinitionIDs {
		if target, ok := targets[definitionID]; !ok || target != 1 {
			t.Errorf("Crucible mastery %d has target %d (found=%v)", definitionID, target, ok)
		}
	}
	// The first HRE mastery is a single-step challenge; mastery_hre_11 (1440417)
	// needs five wins, so this also checks the target field is read from the
	// right position of the definition.
	if target, ok := targets[1440397]; !ok || target != 1 {
		t.Errorf("mastery_hre_01 has target %d (found=%v)", target, ok)
	}
	if target, ok := targets[1440417]; !ok || target != 5 {
		t.Errorf("mastery_hre_11 has target %d (found=%v)", target, ok)
	}
}

// The current resources wrap the definitions, while older AoE II / AoM saves
// used a flat root array. The parser must accept both regardless of the hint.
func TestDecodeChallengeRowsAcceptsWrapperAndFlat(t *testing.T) {
	wrapper := []byte(`[0,[],[[1,1,"c","","C","D","",1,"{}",-1,-1,-1,0]],"sig"]`)
	flat := []byte(`[[1,1,"c","","C","D","",1,"{}",-1,-1,-1,0]]`)
	cases := []struct {
		name   string
		data   []byte
		format ChallengeFormat
	}{
		{"wrapper", wrapper, FormatAoE4},
		{"wrapper with flat hint", wrapper, FormatFlatArray},
		{"flat", flat, FormatFlatArray},
		{"flat with wrapper hint", flat, FormatAoE4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := decodeChallengeRows(tc.data, tc.format)
			if err != nil {
				t.Fatalf("decodeChallengeRows: %s", err)
			}
			if len(rows) != 1 {
				t.Fatalf("got %d rows, want 1", len(rows))
			}
		})
	}
}

// A record without a definition ID cannot be matched by the client, so it must
// not be sent even if it is kept in the saved document.
func TestEncodeChallengeProgressSkipsRecordsWithoutDefinition(t *testing.T) {
	progress := []ChallengeProgress{
		{"progressID": int64(1)},
		{"progressID": int64(2), ChallengeProgressFieldDefinitionID: int64(561)},
	}
	encoded := EncodeChallengeProgress(progress)
	if len(encoded) != 1 {
		t.Fatalf("got %d encoded entries, want 1", len(encoded))
	}
	fields, ok := encoded[0].([]any)
	if !ok || len(fields) != 7 {
		t.Fatalf("unexpected encoded entry: %#v", encoded[0])
	}
	if fields[1] != int64(561) {
		t.Fatalf("definition ID = %v, want 561", fields[1])
	}
}

// A record that never went through reconcile must still be encoded with the
// field names the client expects.
func TestEncodeChallengeProgressEntryMigratesLegacyFields(t *testing.T) {
	entry := ChallengeProgress{
		"progressID":                       int64(1),
		ChallengeProgressFieldDefinitionID: int64(561),
		legacyChallengeProgressStatus:      int64(1),
	}
	fields := EncodeChallengeProgressEntry(entry)
	if len(fields) != 7 {
		t.Fatalf("got %d fields, want 7", len(fields))
	}
	if fields[2] != int64(1) {
		t.Fatalf("completionCount = %v, want 1", fields[2])
	}
	if fields[3] != int64(0) {
		t.Fatalf("progress = %v, want 0", fields[3])
	}
	if fields[4] != nil {
		t.Fatalf("expiryTime = %v, want null", fields[4])
	}
}
