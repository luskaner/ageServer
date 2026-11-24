package launcher

// SafeModeWarning is shown before the launcher changes the machine or starts
// the game. The server is the authority; the launcher only mirrors its mode.
const SafeModeWarning = "The server is running in Safe mode: achievements, challenges and avatar stats will not be granted. If you use the official launcher, progress may not synchronize with it."

// SafeAppliesToClient reports whether the server's [Safe] mode applies to this
// client. mode is the announced value, or "" from a server that predates the
// field. Exceptions are not known to the launcher, so an official client is
// warned even when its account may be listed.
func SafeAppliesToClient(mode string, officialClient bool) bool {
	switch mode {
	case "true":
		return true
	case "false":
		return false
	case "auto":
		return officialClient
	default:
		return false
	}
}
