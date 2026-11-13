package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/pelletier/go-toml/v2"
)

// MenuBackgroundEventIDBase is the first id of the reserved range used when a
// configured period does not name its own event id; the id is Base + image.
//
// The range must stay disjoint from the ranked season ids the AoE4 route
// generates (1000 + baseIndex + index, always >= 1000, see
// routes/game/communityEvent/season.go). A background event that landed in the
// season range would make the client read it as a season and break ranked
// play.
const MenuBackgroundEventIDBase int32 = 900000

// MenuBackgroundDefinition is one entry of the bundled menuBackgrounds.json
// catalogue: an image the AoE4 client can show as its main-menu background.
type MenuBackgroundDefinition struct {
	ID                int      `json:"id"`
	Name              string   `json:"name"`
	FrontEndImagePath string   `json:"frontEndImagePath"`
	SourceEventID     int      `json:"sourceEventId"`
	Tags              []string `json:"tags,omitempty"`
}

// MenuBackgroundWindow is one period of a game's menuBackgrounds.toml: the
// image shown from Start until End. Gaps between windows mean "no background".
type MenuBackgroundWindow struct {
	Image int       `toml:"image"`
	Start time.Time `toml:"start"`
	End   time.Time `toml:"end"`
	Event int32     `toml:"event,omitempty"`
	Name  string    `toml:"name,omitempty"`
}

// MenuBackgrounds holds the parsed catalogue and the configured schedule. The
// catalogue is shared by every player and the schedule is the whole history
// the server publishes.
type MenuBackgrounds struct {
	Images   []MenuBackgroundDefinition
	Schedule []MenuBackgroundWindow
}

// menuBackgroundFile is the TOML document: one [[background]] table per period.
type menuBackgroundFile struct {
	Backgrounds []MenuBackgroundWindow `toml:"background"`
}

// ParseMenuBackgroundCatalog decodes the [0, [definition...]] resource. Entries
// without a positive id, a name or an image path are skipped.
func ParseMenuBackgroundCatalog(resource i.A) []MenuBackgroundDefinition {
	if len(resource) < 2 {
		return nil
	}
	entries, ok := resource[1].([]any)
	if !ok {
		return nil
	}
	catalog := make([]MenuBackgroundDefinition, 0, len(entries))
	for _, raw := range entries {
		encoded, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		var definition MenuBackgroundDefinition
		if json.Unmarshal(encoded, &definition) == nil &&
			definition.ID > 0 && definition.Name != "" && definition.FrontEndImagePath != "" {
			catalog = append(catalog, definition)
		}
	}
	return catalog
}

// loadMenuBackgroundSchedule reads and validates path against catalog. A
// missing file means the feature is off and is not an error; any other problem
// is fatal at startup, because the catalogue is static and a bad value cannot
// become correct later.
func loadMenuBackgroundSchedule(path string, catalog []MenuBackgroundDefinition) ([]MenuBackgroundWindow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var file menuBackgroundFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	byID := make(map[int]MenuBackgroundDefinition, len(catalog))
	for _, definition := range catalog {
		byID[definition.ID] = definition
	}
	schedule := make([]MenuBackgroundWindow, len(file.Backgrounds))
	eventIDs := make(map[int32]int, len(file.Backgrounds))
	for index, window := range file.Backgrounds {
		entry := index + 1
		definition, known := byID[window.Image]
		if !known {
			return nil, fmt.Errorf("%s: background %d: unknown image id %d", path, entry, window.Image)
		}
		if window.Start.IsZero() || window.End.IsZero() {
			return nil, fmt.Errorf("%s: background %d: start and end are required", path, entry)
		}
		if !window.Start.Before(window.End) {
			return nil, fmt.Errorf("%s: background %d: start must be before end", path, entry)
		}
		if window.Name == "" {
			window.Name = definition.Name
		}
		if window.Event == 0 {
			window.Event = MenuBackgroundEventIDBase + int32(window.Image)
		}
		if previous, repeated := eventIDs[window.Event]; repeated {
			return nil, fmt.Errorf("%s: background %d: event id %d is already used by background %d",
				path, entry, window.Event, previous)
		}
		eventIDs[window.Event] = entry
		schedule[index] = window
	}
	sort.SliceStable(schedule, func(left, right int) bool {
		return schedule[left].Start.Before(schedule[right].Start)
	})
	for index := 1; index < len(schedule); index++ {
		if schedule[index].Start.Before(schedule[index-1].End) {
			return nil, fmt.Errorf("%s: background %d overlaps background %d", path, index+1, index)
		}
	}
	return schedule, nil
}
