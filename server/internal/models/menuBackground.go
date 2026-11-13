package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"time"

	i "github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/pelletier/go-toml/v2"
)

// MenuBackgroundEventIDBase is the first id of the reserved range used when a
// configured period does not name its own event id; the id is Base + image.
//
// The range must stay disjoint from the ranked season ids the AoE4 route
// generates (see RankedSeasonEventIDBase and
// routes/game/communityEvent/season.go). A background event that landed in the
// season range would make the client read it as a season and break ranked play,
// so loadMenuBackgroundSchedule rejects any explicit id above
// RankedSeasonEventIDBase and below this base.
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

// MenuBackgroundWindow is one [[background]] table of a game's
// menuBackgrounds.toml: the image shown from Start until End. Gaps between
// windows mean "no background".
type MenuBackgroundWindow struct {
	Image int       `toml:"image"`
	Start time.Time `toml:"start"`
	End   time.Time `toml:"end"`
	Event int32     `toml:"event,omitempty"`
	Name  string    `toml:"name,omitempty"`
}

// MenuBackgroundPeriod is a validated window with the catalogue data resolved,
// so publishing it is a pure function of the period and the current time.
type MenuBackgroundPeriod struct {
	Event int32
	Name  string
	Start time.Time
	End   time.Time
	// CustomData is the event's customdata field, encoded once at load time
	// because it is sent on every event list request.
	CustomData string
}

// MenuBackgrounds is the configured background schedule of one game.
type MenuBackgrounds struct {
	Periods []MenuBackgroundPeriod
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

// resolvedWindow pairs a validated period with its 1-based position in the file
// so the overlap error can name the entries as the operator wrote them, even
// though the check runs on the start-ordered schedule.
type resolvedWindow struct {
	period MenuBackgroundPeriod
	entry  int
}

// loadMenuBackgroundSchedule reads and validates path against catalog. A missing
// file means the feature is off and is not an error; any other problem is fatal
// at startup, because the catalogue is static and a bad value cannot become
// correct later.
func loadMenuBackgroundSchedule(path string, catalog []MenuBackgroundDefinition) ([]MenuBackgroundPeriod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	decoder := toml.NewDecoder(bytes.NewReader(data))
	// Without this a typo in a table name ([[backgroud]]) or in a key would be
	// ignored and the feature would silently switch itself off.
	decoder.DisallowUnknownFields()
	var file menuBackgroundFile
	if err = decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	byID := make(map[int]MenuBackgroundDefinition, len(catalog))
	for _, definition := range catalog {
		byID[definition.ID] = definition
	}
	windows := make([]resolvedWindow, 0, len(file.Backgrounds))
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
		event := window.Event
		if event == 0 {
			if int64(window.Image) > 1<<31-1-int64(MenuBackgroundEventIDBase) {
				return nil, fmt.Errorf("%s: background %d: image id %d is out of range",
					path, entry, window.Image)
			}
			event = MenuBackgroundEventIDBase + int32(window.Image)
		}
		if event >= RankedSeasonEventIDBase && event < MenuBackgroundEventIDBase {
			return nil, fmt.Errorf(
				"%s: background %d: event id %d is reserved for the ranked season events "+
					"(ids from %d); use an id below %d or at least %d",
				path, entry, event, RankedSeasonEventIDBase, RankedSeasonEventIDBase,
				MenuBackgroundEventIDBase)
		}
		if previous, repeated := eventIDs[event]; repeated {
			return nil, fmt.Errorf(
				"%s: background %d: event id %d is already used by background %d; "+
					"set event explicitly to show the same image in several periods",
				path, entry, event, previous)
		}
		eventIDs[event] = entry
		name := window.Name
		if name == "" {
			name = definition.Name
		}
		customData, err := json.Marshal(map[string]string{
			"frontEndImagePath": definition.FrontEndImagePath,
		})
		if err != nil {
			return nil, fmt.Errorf("cannot encode the customdata of background %d: %w", entry, err)
		}
		windows = append(windows, resolvedWindow{
			entry: entry,
			period: MenuBackgroundPeriod{
				Event:      event,
				Name:       name,
				Start:      window.Start,
				End:        window.End,
				CustomData: string(customData),
			},
		})
	}
	sort.SliceStable(windows, func(left, right int) bool {
		return windows[left].period.Start.Before(windows[right].period.Start)
	})
	for index := 1; index < len(windows); index++ {
		if windows[index].period.Start.Before(windows[index-1].period.End) {
			return nil, fmt.Errorf("%s: background %d overlaps background %d",
				path, windows[index].entry, windows[index-1].entry)
		}
	}
	periods := make([]MenuBackgroundPeriod, len(windows))
	for index, window := range windows {
		periods[index] = window.period
	}
	return periods, nil
}

// warnMenuBackgroundsWithoutCatalog reports a schedule that cannot be used
// because the title ships no catalogue. It is a warning rather than an error:
// titles without menu backgrounds have neither file, and only an operator who
// wrote one has a problem worth reporting.
func warnMenuBackgroundsWithoutCatalog(path string) {
	if _, err := os.Stat(path); err == nil {
		logger.Warn("%s is configured but this title ships no menuBackgrounds.json, "+
			"so no menu background will be shown", path)
	}
}
