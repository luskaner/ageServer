package models

import "testing"

func TestEncodeChallengeProgressPending(t *testing.T) {
	progress := []ChallengeProgress{
		{
			"progressID":                            int64(7),
			ChallengeProgressFieldDefinitionID:      int64(42),
			ChallengeProgressFieldCompletionCount:   int64(1),
			ChallengeProgressFieldProgress:          int64(3),
			ChallengeProgressFieldExpiryTime:        int64(1234),
			ChallengeProgressFieldLastCompletedTime: int64(999),
			ChallengeProgressFieldAssignedTime:      int64(500),
		},
		{"progressID": int64(8)},
	}
	rows := EncodeChallengeProgressPending(progress)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	row, ok := rows[0].([]any)
	if !ok {
		t.Fatalf("row is %T, want []any", rows[0])
	}
	if row[0] != int64(7) || row[1] != int64(42) {
		t.Fatalf("identifiers = %v, want [7 42]", row[:2])
	}
	if row[2] != int64(0) || row[3] != int64(0) || row[5] != nil {
		t.Fatalf("row is not pending: %v", row)
	}
	if row[4] != int64(1234) || row[6] != int64(500) {
		t.Fatalf("expiry/assigned changed: %v", row)
	}
}
