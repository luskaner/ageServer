package main

import "time"

// watchAccentColour calls changed with the desktop accent colour every time it
// differs from the value read before, so the interface can repaint itself
// without being asked. The desktop reports a light/dark switch on its own, but
// not a change of the accent colour, and that colour is chosen in the system
// settings while the application is open, so it is read again on a timer.
func watchAccentColour(changed func(accent string)) {
	go func() {
		previous := systemAccentColour()
		ticker := time.NewTicker(accentWatchInterval)
		defer ticker.Stop()
		for range ticker.C {
			accent := systemAccentColour()
			if accent == previous {
				continue
			}
			previous = accent
			changed(accent)
		}
	}()
}
