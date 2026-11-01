//go:build !windows

package hosts

import "math"

var maxHostsPerLine = math.MaxInt // Not an actual limit
var maxCharsPerLine = math.MaxUint8 + 1
