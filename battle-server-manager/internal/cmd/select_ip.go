package cmd

import "net"

// selectNonLoopbackIP returns the FIRST usable (parseable, non-loopback) IP
// from the list, or an empty string when there is none.
//
// Regression: the previous inline loop kept overwriting the result without
// breaking, so the LAST suitable IP won — a nondeterministic choice that
// depended on interface enumeration order.
func selectNonLoopbackIP(ips []string) string {
	for _, currentIP := range ips {
		if parsed := net.ParseIP(currentIP); parsed != nil && !parsed.IsLoopback() {
			return currentIP
		}
	}
	return ""
}
