// Package bytesize parses the human-readable byte sizes used by the
// configuration files.
package bytesize

import (
	"errors"
	"strconv"
	"strings"
)

// Parse parses a plain byte count or an integer followed by B, KB, MB or GB.
// The single-letter forms K, M and G are accepted too, and the suffix is
// case-insensitive.
//
// The server and the configurator both validate these values, so they share
// this parser: a value the editor accepts must be one the server accepts.
func Parse(value string) (int64, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return 0, errors.New("empty size")
	}
	units := []struct {
		suffix     string
		multiplier int64
	}{
		{"GB", 1 << 30},
		{"G", 1 << 30},
		{"MB", 1 << 20},
		{"M", 1 << 20},
		{"KB", 1 << 10},
		{"K", 1 << 10},
		{"B", 1},
	}
	for _, unit := range units {
		if before, ok := strings.CutSuffix(value, unit.suffix); ok {
			number := strings.TrimSpace(before)
			n, err := strconv.ParseInt(number, 10, 64)
			if err != nil {
				return 0, err
			}
			return n * unit.multiplier, nil
		}
	}
	return strconv.ParseInt(value, 10, 64)
}
