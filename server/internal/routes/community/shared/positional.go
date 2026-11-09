package shared

import (
	"strconv"
	"strings"

	i "github.com/luskaner/ageServer/server/internal"
)

// The positional resources this API reads (leaderboards.json, achievements.json,
// news entries) are arrays of arrays whose slots are untyped: encoding/json gives
// a float64 for every number and the in-memory literals add int and int64. These
// four helpers are the only place that knows how to poke at those slots, so the
// endpoints do not each carry their own type switch.

// At returns the slot at index, or nil when the row is shorter.
func At(row i.A, index int) any {
	if index < 0 || index >= len(row) {
		return nil
	}
	return row[index]
}

// StringAt returns the slot at index as a string. A missing slot, or one holding
// another type, reads as "".
func StringAt(row i.A, index int) string {
	text, _ := At(row, index).(string)
	return text
}

// Int64At returns the slot at index as a number. The bool is false when the slot
// is missing or holds something that is not numeric.
//
// A decimal string is accepted too: models.initializeNews stores the news
// timestamps as strings (newsTimestampString forwards what news.toml declares),
// so the same reader has to serve both a JSON number and that string form.
func Int64At(row i.A, index int) (int64, bool) {
	value := At(row, index)
	if number, ok := i.NumberToInt64(value); ok {
		return number, true
	}
	if text, isText := value.(string); isText {
		number, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err == nil {
			return number, true
		}
	}
	return 0, false
}

// RowsAt returns the positional block at index as a list of rows, which is how
// every resource nests its tables. A missing or non-array slot yields nil so a
// shorter resource simply contributes nothing.
func RowsAt(container i.A, index int) i.A {
	rows, _ := At(container, index).(i.A)
	return rows
}
