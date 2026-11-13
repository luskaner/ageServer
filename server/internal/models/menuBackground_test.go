package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
)

func testMenuBackgroundCatalog() []MenuBackgroundDefinition {
	return []MenuBackgroundDefinition{
		{ID: 125, Name: "Season's Feast", FrontEndImagePath: `\Images\backgrounds\s9_winterbanquet_keyart.png`},
		{ID: 129, Name: "Raiders of the North", FrontEndImagePath: `\Images\backgrounds\xp7_keyart.png`},
	}
}

func writeMenuBackgroundConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "menuBackgrounds.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// bundledMenuBackgroundCatalog loads the catalogue the same way the server does,
// so a malformed resource fails here instead of at startup.
func bundledMenuBackgroundCatalog(t *testing.T) []MenuBackgroundDefinition {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "resources", "responses", "age4", "menuBackgrounds.json"))
	if err != nil {
		t.Fatalf("read menuBackgrounds.json: %v", err)
	}
	var decoded i.A
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode menuBackgrounds.json: %v", err)
	}
	return ParseMenuBackgroundCatalog(decoded)
}

// TestParseMenuBackgroundCatalogReadsTheShippedCatalogue pins the resource the
// whole feature depends on: 30 entries, ids 100-129, every one usable.
func TestParseMenuBackgroundCatalogReadsTheShippedCatalogue(t *testing.T) {
	catalog := bundledMenuBackgroundCatalog(t)
	if len(catalog) != 30 {
		t.Fatalf("catalogue has %d entries, want 30", len(catalog))
	}
	seen := make(map[int]bool, len(catalog))
	for _, definition := range catalog {
		if definition.ID < 100 || definition.ID > 129 {
			t.Errorf("id %d is outside the shipped range 100-129", definition.ID)
		}
		if seen[definition.ID] {
			t.Errorf("id %d appears twice", definition.ID)
		}
		seen[definition.ID] = true
		if definition.Name == "" || definition.FrontEndImagePath == "" {
			t.Errorf("id %d has an empty name or path", definition.ID)
		}
	}
}

// TestLoadMenuBackgroundScheduleAppliesDefaults checks the fields a period may
// omit: the name comes from the catalogue, the event id from the reserved range
// and the customdata is encoded once. It also checks the start order.
func TestLoadMenuBackgroundScheduleAppliesDefaults(t *testing.T) {
	path := writeMenuBackgroundConfig(t, `
[[background]]
image = 125
start = 2026-12-05T18:00:00Z
end   = 2027-02-05T18:00:00Z

[[background]]
image = 129
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z
`)
	periods, err := loadMenuBackgroundSchedule(path, testMenuBackgroundCatalog())
	if err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	if len(periods) != 2 {
		t.Fatalf("schedule = %#v, want 2 periods", periods)
	}
	if periods[0].Event != MenuBackgroundEventIDBase+129 {
		t.Fatalf("first event id = %d, want %d", periods[0].Event, MenuBackgroundEventIDBase+129)
	}
	if periods[0].Name != "Raiders of the North" {
		t.Fatalf("name = %q, want the catalogue name", periods[0].Name)
	}
	if periods[0].CustomData != `{"frontEndImagePath":"\\Images\\backgrounds\\xp7_keyart.png"}` {
		t.Fatalf("customdata = %q, want the pre-encoded image path", periods[0].CustomData)
	}
	if want := time.Date(2026, time.November, 1, 17, 0, 0, 0, time.UTC); !periods[0].Start.Equal(want) {
		t.Fatalf("start = %s, want %s", periods[0].Start, want)
	}
}

func TestLoadMenuBackgroundScheduleKeepsExplicitFields(t *testing.T) {
	path := writeMenuBackgroundConfig(t, `
[[background]]
event = 290
name  = "Trade Secrets - Challenges 13"
image = 129
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z
`)
	periods, err := loadMenuBackgroundSchedule(path, testMenuBackgroundCatalog())
	if err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	if len(periods) != 1 || periods[0].Event != 290 || periods[0].Name != "Trade Secrets - Challenges 13" {
		t.Fatalf("periods = %#v, want the configured event and name", periods)
	}
}

func TestLoadMenuBackgroundScheduleAbsentFileIsNotAnError(t *testing.T) {
	periods, err := loadMenuBackgroundSchedule(
		filepath.Join(t.TempDir(), "menuBackgrounds.toml"), testMenuBackgroundCatalog())
	if err != nil {
		t.Fatalf("absent file returned %v", err)
	}
	if len(periods) != 0 {
		t.Fatalf("periods = %#v, want none", periods)
	}
}

// TestLoadMenuBackgroundScheduleRejectsInvalidConfig covers every validation
// rule. The message has to name the offending entry so an operator can fix it
// without reading the source.
func TestLoadMenuBackgroundScheduleRejectsInvalidConfig(t *testing.T) {
	cases := map[string]struct {
		content string
		want    string
	}{
		"unparsable": {
			content: "[[background]\nimage = 129\n",
			want:    "cannot parse",
		},
		// A typo in the table name used to be ignored, which silently switched
		// the whole feature off.
		"unknown table": {
			content: `
[[backgroud]]
image = 129
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z
`,
			want: "cannot parse",
		},
		"unknown field": {
			content: `
[[background]]
image = 129
begins = 2026-11-01T17:00:00Z
end    = 2026-11-15T17:00:00Z
`,
			want: "cannot parse",
		},
		"unknown image": {
			content: `
[[background]]
image = 290
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z
`,
			want: "background 1: unknown image id 290",
		},
		"missing start": {
			content: `
[[background]]
image = 129
end   = 2026-11-15T17:00:00Z
`,
			want: "background 1: start and end are required",
		},
		"missing end": {
			content: `
[[background]]
image = 129
start = 2026-11-01T17:00:00Z
`,
			want: "background 1: start and end are required",
		},
		"empty window": {
			content: `
[[background]]
image = 129
start = 2026-11-01T17:00:00Z
end   = 2026-11-01T17:00:00Z
`,
			want: "background 1: start must be before end",
		},
		"overlap": {
			content: `
[[background]]
image = 129
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z

[[background]]
image = 125
start = 2026-11-10T17:00:00Z
end   = 2026-11-20T17:00:00Z
`,
			want: "background 2 overlaps background 1",
		},
		// The season ids are generated from RankedSeasonEventIDBase upwards, so
		// an explicit id in that range would be read as a season.
		"event id in the season range": {
			content: `
[[background]]
event = 1072
image = 129
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z
`,
			want: "reserved for the ranked season events",
		},
		"duplicate event id": {
			content: `
[[background]]
event = 290
image = 129
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z

[[background]]
event = 290
image = 125
start = 2026-11-15T17:00:00Z
end   = 2026-11-20T17:00:00Z
`,
			want: "set event explicitly to show the same image in several periods",
		},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeMenuBackgroundConfig(t, test.content)
			_, err := loadMenuBackgroundSchedule(path, testMenuBackgroundCatalog())
			if err == nil {
				t.Fatal("load schedule succeeded, want an error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want it to contain %q", err, test.want)
			}
		})
	}
}

// TestLoadMenuBackgroundScheduleOverlapNamesTheFileEntries is the regression for
// an error that named the wrong entries: the overlap check runs on the
// start-ordered schedule, so it has to report the positions the operator wrote.
func TestLoadMenuBackgroundScheduleOverlapNamesTheFileEntries(t *testing.T) {
	path := writeMenuBackgroundConfig(t, `
[[background]]
image = 129
start = 2026-11-01T00:00:00Z
end   = 2026-11-30T00:00:00Z

[[background]]
image = 125
start = 2026-12-01T00:00:00Z
end   = 2026-12-10T00:00:00Z

[[background]]
image = 125
event = 291
start = 2026-11-15T00:00:00Z
end   = 2026-11-20T00:00:00Z
`)
	_, err := loadMenuBackgroundSchedule(path, testMenuBackgroundCatalog())
	if err == nil {
		t.Fatal("load schedule succeeded, want an overlap error")
	}
	// Entry 1 (Nov 1-30) contains entry 3 (Nov 15-20); entry 2 is clean and must
	// not be named.
	if !strings.Contains(err.Error(), "background 3 overlaps background 1") {
		t.Fatalf("error = %q, want it to name entries 3 and 1", err)
	}
}

// TestLoadMenuBackgroundScheduleRejectsTheSeasonIDRange checks the boundary the
// ranking depends on: an id just below the reserved base is refused, because the
// season ids grow towards it.
func TestLoadMenuBackgroundScheduleRejectsTheSeasonIDRange(t *testing.T) {
	for _, event := range []int32{RankedSeasonEventIDBase, RankedSeasonEventIDBase + 72, MenuBackgroundEventIDBase - 1} {
		content := fmt.Sprintf(`
[[background]]
event = %d
image = 129
start = 2026-11-01T17:00:00Z
end   = 2026-11-15T17:00:00Z
`, event)
		path := writeMenuBackgroundConfig(t, content)
		if _, err := loadMenuBackgroundSchedule(path, testMenuBackgroundCatalog()); err == nil {
			t.Errorf("event id %d was accepted, want it refused as a season id", event)
		}
	}
}
