package leaderboard

// normalizeProfileIDs drops the placeholder identifiers the client appends to
// pad its request and de-duplicates the result while preserving the order.
func normalizeProfileIDs(ids []int32) []int32 {
	seen := make(map[int32]struct{}, len(ids))
	out := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}