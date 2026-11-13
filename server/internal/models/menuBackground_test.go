package models

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// TestLoadMenuBackgroundScheduleAppliesDefaults checks the fields a period may
// omit: the name comes from the catalogue and the event id from the reserved
// range. It also checks that the schedule is returned in start order.
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
	schedule, err := loadMenuBackgroundSchedule(path, testMenuBackgroundCatalog())
	if err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	if len(schedule) != 2 {
		t.Fatalf("schedule = %#v, want 2 windows", schedule)
	}
	if schedule[0].Image != 129 {
		t.Fatalf("first window image = %d, want 129 (start order)", schedule[0].Image)
	}
	if schedule[0].Name != "Raiders of the North" {
		t.Fatalf("name = %q, want the catalogue name", schedule[0].Name)
	}
	if schedule[0].Event != MenuBackgroundEventIDBase+129 {
		t.Fatalf("event id = %d, want %d", schedule[0].Event, MenuBackgroundEventIDBase+129)
	}
	if want := time.Date(2026, time.November, 1, 17, 0, 0, 0, time.UTC); !schedule[0].Start.Equal(want) {
		t.Fatalf("start = %s, want %s", schedule[0].Start, want)
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
	schedule, err := loadMenuBackgroundSchedule(path, testMenuBackgroundCatalog())
	if err != nil {
		t.Fatalf("load schedule: %v", err)
	}
	if len(schedule) != 1 {
		t.Fatalf("schedule = %#v, want 1 window", schedule)
	}
	if schedule[0].Event != 290 || schedule[0].Name != "Trade Secrets - Challenges 13" {
		t.Fatalf("window = %+v, want the configured event and name", schedule[0])
	}
}

func TestLoadMenuBackgroundScheduleAbsentFileIsNotAnError(t *testing.T) {
	schedule, err := loadMenuBackgroundSchedule(
		filepath.Join(t.TempDir(), "menuBackgrounds.toml"), testMenuBackgroundCatalog())
	if err != nil {
		t.Fatalf("absent file returned %v", err)
	}
	if len(schedule) != 0 {
		t.Fatalf("schedule = %#v, want none", schedule)
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
			want: "background 2: event id 290 is already used by background 1",
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
