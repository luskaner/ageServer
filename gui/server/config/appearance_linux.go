//go:build linux

package main

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// accentColours maps the accent names of org.gnome.desktop.interface to the
// colours GNOME paints them with. Desktops without that setting (KDE, and GNOME
// before release 47) have no accent to read, so the interface keeps its own.
var accentColours = map[string]string{
	"blue":   "#3584e4",
	"teal":   "#2190a4",
	"green":  "#3a944a",
	"yellow": "#c88800",
	"orange": "#ed5b00",
	"red":    "#e62d42",
	"pink":   "#d56199",
	"purple": "#9141ac",
	"slate":  "#6f8396",
}

// accentWatchInterval is how often the accent colour is read to notice a
// change. Reading it here starts a helper tool, so the interval is longer than
// the one Windows can afford.
const accentWatchInterval = 15 * time.Second

// systemAccentColour reads the accent colour the desktop highlights controls
// with, as #rrggbb. It is empty when the desktop does not offer one.
func systemAccentColour() string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gsettings", "get", "org.gnome.desktop.interface", "accent-color").Output()
	if err != nil {
		return ""
	}
	return accentColours[strings.Trim(strings.TrimSpace(string(out)), "'\"")]
}
