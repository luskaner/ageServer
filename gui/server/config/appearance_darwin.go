//go:build darwin

package main

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// accentColours maps the value of the AppleAccentColor default to the colour it
// stands for. -1 is the "multicolour" choice, which macOS draws in the blue it
// also uses when the default has never been written.
var accentColours = map[int]string{
	-1: "#0a84ff",
	0:  "#ff5257",
	1:  "#f7821b",
	2:  "#ffc600",
	3:  "#62ba46",
	4:  "#007aff",
	5:  "#a550a7",
	6:  "#f74f9e",
	7:  "#8e8e93",
}

// accentWatchInterval is how often the accent colour is read to notice a
// change. Reading it here starts a helper tool, so the interval is longer than
// the one Windows can afford.
const accentWatchInterval = 15 * time.Second

// systemAccentColour reads the accent colour chosen in the Appearance pane of
// the system settings, as #rrggbb. It is empty when the setting cannot be read.
func systemAccentColour() string {
	out, err := readDefault("AppleAccentColor")
	if err != nil {
		// Without the setting the desktop follows its wallpaper, which for the
		// default choices means the standard blue.
		return accentColours[-1]
	}
	index, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return ""
	}
	return accentColours[index]
}

// readDefault asks the user defaults for a value, giving up rather than
// delaying startup when the tool does not answer.
func readDefault(key string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "defaults", "read", "-g", key).Output()
	return string(out), err
}
