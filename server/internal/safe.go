package internal

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

const (
	// SafeAuto resolves [Safe] per session: official sessions are safe, the
	// rest are not.
	SafeAuto = "auto"
	// SafeTrue makes every session safe.
	SafeTrue = "true"
	// SafeFalse makes no session safe.
	SafeFalse = "false"
)

// ParseSafeMode normalizes the [Safe] Enabled value. It accepts the unquoted
// TOML booleans as "true"/"false" because koanf decodes them before this runs.
func ParseSafeMode(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", SafeAuto:
		return SafeAuto, nil
	case SafeTrue, "1":
		return SafeTrue, nil
	case SafeFalse, "0":
		return SafeFalse, nil
	default:
		return "", fmt.Errorf("invalid Safe.Enabled %q (accepted: auto, true, false)", value)
	}
}

// SafeEnabled resolves [Safe] for one session. PlatformUserIdExceptions
// inverts the result, exactly like [Sign].
func SafeEnabled(official bool, platformUserId uint64) bool {
	enabled := official
	switch SafeConfigRuntime.Enabled {
	case SafeTrue:
		enabled = true
	case SafeFalse:
		enabled = false
	}
	if PlatformUserIdInExceptions(platformUserId, SafeConfigRuntime.PlatformUserIdExceptions) {
		return !enabled
	}
	return enabled
}

// PlatformUserIdInExceptions reports whether platformUserId is listed in
// exceptions. A zero ID never matches: it identifies no account.
func PlatformUserIdInExceptions(platformUserId uint64, exceptions []string) bool {
	if platformUserId == 0 {
		return false
	}
	formatted := strconv.FormatUint(platformUserId, 10)
	return slices.Contains(exceptions, formatted)
}
