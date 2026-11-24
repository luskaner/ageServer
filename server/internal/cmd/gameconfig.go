package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/cmd/server"
	"github.com/luskaner/ageServer/server/internal"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/spf13/pflag"
)

// gameConfigKeys are the sections a config.<game>.toml may contain. Everything
// else is an error: a misspelled key silently ignored is a setting that never
// took effect.
var gameConfigEmbeddedKeys = []string{
	"StartIfMissing", "Region", "Name", "IPv4", "BsPort", "WebSocketPort", "OutOfBandPort", "LogLevel",
}

var gameConfigBattleServerKeys = []string{
	"Region", "Name", "IPv4", "BsPort", "WebSocketPort", "OutOfBandPort",
}

// gameConfigCandidates is the search order of the per-game file: the explicit
// --gameConfig first, then the configured folders, then the folder of the main
// config file that was actually loaded.
func gameConfigCandidates(gameID, usedFile string, values *server.Values) []string {
	var candidates []string
	if values != nil && values.GameConfig != "" {
		candidates = append(candidates, values.GameConfig)
	}
	for _, dir := range configPaths {
		candidates = append(candidates, filepath.Join(dir, "config."+gameID+".toml"))
	}
	if usedFile != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(usedFile), "config."+gameID+".toml"))
	}
	seen := make(map[string]bool, len(candidates))
	out := candidates[:0]
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		out = append(out, candidate)
	}
	return out
}

func asStringMap(value any) (map[string]any, bool) {
	m, ok := value.(map[string]any)
	return m, ok
}

// withoutGameHosts removes the deprecated [Game.Hosts] table before the file
// is merged. It is filtered rather than deleted in place because koanf may hand
// back a copy of its map.
func withoutGameHosts(raw map[string]any) map[string]any {
	clean := make(map[string]any, len(raw))
	for key, value := range raw {
		if key != "Game" {
			clean[key] = value
			continue
		}
		gameTable, ok := value.(map[string]any)
		if !ok {
			clean[key] = value
			continue
		}
		cleanGame := make(map[string]any, len(gameTable))
		for gameKey, gameValue := range gameTable {
			if gameKey == "Hosts" {
				continue
			}
			cleanGame[gameKey] = gameValue
		}
		clean[key] = cleanGame
	}
	return clean
}

func validateGameTableKeys(table map[string]any, allowed []string, path, where string) error {
	for key := range table {
		if !slices.Contains(allowed, key) {
			return fmt.Errorf("%s: unknown key %q in %s", path, key, where)
		}
	}
	return nil
}

// validateGameConfigFile rejects anything the per-game document does not own.
// It returns whether [Game.Hosts] was present, which is deprecated and ignored
// rather than an error so an old file keeps loading.
func validateGameConfigFile(raw map[string]any, path, gameID string) (hasHosts bool, err error) {
	for topKey, topValue := range raw {
		if topKey != "Game" {
			return false, fmt.Errorf("%s: key %q is not allowed here: [Games] belongs to config.toml, not config.%s.toml (only Game.BattleServers and Game.EmbeddedBattleServer are allowed)", path, topKey, gameID)
		}
		gameTable, ok := asStringMap(topValue)
		if !ok {
			return false, fmt.Errorf("%s: Game must be a table", path)
		}
		for key, value := range gameTable {
			switch key {
			case "BattleServers":
				if err = validateBattleServers(value, path); err != nil {
					return false, err
				}
			case "EmbeddedBattleServer":
				table, ok := asStringMap(value)
				if !ok {
					return false, fmt.Errorf("%s: Game.EmbeddedBattleServer must be a table", path)
				}
				if err = validateGameTableKeys(table, gameConfigEmbeddedKeys, path, "Game.EmbeddedBattleServer"); err != nil {
					return false, err
				}
			case "Hosts":
				hasHosts = true
			default:
				return false, fmt.Errorf("%s: unknown key %q in Game (only BattleServers and EmbeddedBattleServer are allowed)", path, key)
			}
		}
	}
	return hasHosts, nil
}

func validateBattleServers(value any, path string) error {
	var rows []any
	switch typed := value.(type) {
	case []any:
		rows = typed
	case []map[string]any:
		for _, row := range typed {
			rows = append(rows, row)
		}
	default:
		return fmt.Errorf("%s: Game.BattleServers must be a list of tables", path)
	}
	for i, row := range rows {
		table, ok := asStringMap(row)
		if !ok {
			return fmt.Errorf("%s: Game.BattleServers[%d] must be a table", path, i)
		}
		if err := validateGameTableKeys(table, gameConfigBattleServerKeys, path, fmt.Sprintf("Game.BattleServers[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

// seedEmbedded seeds the loader leaf by leaf. A struct handed to
// confmap.Provider is a leaf of its own: a partial [Game.EmbeddedBattleServer]
// table in the per-game file would replace it as a whole and drop every value
// inherited from config.toml. BattleServers is seeded whole on purpose: the
// file replaces that list entirely.
func seedEmbedded(seed map[string]any, eb internal.EmbeddedBattleServer) {
	seed["Game.EmbeddedBattleServer.Region"] = eb.Region
	seed["Game.EmbeddedBattleServer.Name"] = eb.Name
	seed["Game.EmbeddedBattleServer.IPv4"] = eb.IPv4
	seed["Game.EmbeddedBattleServer.BsPort"] = eb.BsPort
	seed["Game.EmbeddedBattleServer.WebSocketPort"] = eb.WebSocketPort
	seed["Game.EmbeddedBattleServer.OutOfBandPort"] = eb.OutOfBandPort
	seed["Game.EmbeddedBattleServer.LogLevel"] = eb.LogLevel
	// An empty value is not seeded so the server default applies as it does
	// when the key is absent.
	if eb.StartIfMissing != "" {
		seed["Game.EmbeddedBattleServer.StartIfMissing"] = eb.StartIfMissing
	}
}

// hasGameConfiguration reports whether a per-game document carries an effective
// setting. The installed template is all comments: counting it as a used file
// would hide the [Games.<game>] migration warnings of every generated install.
func hasGameConfiguration(raw map[string]any) bool {
	gameTable, ok := asStringMap(raw["Game"])
	if !ok {
		return false
	}
	for _, key := range []string{"BattleServers", "EmbeddedBattleServer"} {
		value, ok := gameTable[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []any:
			if len(typed) > 0 {
				return true
			}
		case []map[string]any:
			if len(typed) > 0 {
				return true
			}
		case map[string]any:
			if len(typed) > 0 {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// embeddedForCompare normalises the values the server resolves on its own, so a
// warning about the file overriding config.toml does not fire on spelling only.
func embeddedForCompare(eb internal.EmbeddedBattleServer) internal.EmbeddedBattleServer {
	if strings.TrimSpace(eb.StartIfMissing) == "" {
		eb.StartIfMissing = "true"
	}
	if strings.TrimSpace(eb.IPv4) == "" {
		eb.IPv4 = "auto"
	}
	return eb
}

func validateStartIfMissing(value string) error {
	switch value {
	case "true", "false", "required":
		return nil
	}
	return fmt.Errorf("StartIfMissing: got %q, want true, false or required", value)
}

// loadGameConfig reads one game's battle servers from config.<game>.toml on top
// of the legacy values of config.toml. Libraries are per-game state, not global:
// the file is decoded in its own koanf instead of as a prefix of the main one.
func loadGameConfig(base internal.Game, gameID string, usedFile string, values *server.Values) (gameCfg internal.Game, found bool, err error) {
	gk := koanf.New(".")
	seed := map[string]any{"Game.BattleServers": base.BattleServers}
	seedEmbedded(seed, base.EmbeddedBattleServer)
	_ = gk.Load(confmap.Provider(seed, "."), nil)

	perGameHosts := false
	for _, candidate := range gameConfigCandidates(gameID, usedFile, values) {
		if candidate == "" {
			continue
		}
		fk := koanf.New(".")
		if err = fk.Load(file.Provider(candidate), toml.Parser()); err != nil {
			if os.IsNotExist(err) {
				if values != nil && values.GameConfig != "" && candidate == values.GameConfig {
					return internal.Game{}, false, fmt.Errorf("--gameConfig %s: %w", candidate, err)
				}
				continue
			}
			return internal.Game{}, false, &common.KoanfFileLoadError{Path: candidate, Err: err}
		}
		var hasHosts bool
		hasHosts, err = validateGameConfigFile(fk.Raw(), candidate, gameID)
		if err != nil {
			return internal.Game{}, false, err
		}
		if hasHosts {
			logger.Warn("%s: [Game.Hosts] is deprecated: hosts are global in [Hosts] (ignored here)", candidate)
			perGameHosts = true
		}
		clean := withoutGameHosts(fk.Raw())
		if err = gk.Load(confmap.Provider(clean, "."), nil); err != nil {
			return internal.Game{}, false, err
		}
		found = hasGameConfiguration(clean)
		// Only the first existing candidate is used, like LoadKoanfLayers.
		break
	}

	if err = gk.Unmarshal("Game", &gameCfg); err != nil {
		return internal.Game{}, false, err
	}
	if perGameHosts {
		gameCfg.Hosts = nil
	}
	gameCfg.EmbeddedBattleServer.StartIfMissing =
		strings.ToLower(strings.TrimSpace(gameCfg.EmbeddedBattleServer.StartIfMissing))
	if gameCfg.EmbeddedBattleServer.StartIfMissing == "" {
		gameCfg.EmbeddedBattleServer.StartIfMissing = "true"
	}
	if err = validateStartIfMissing(gameCfg.EmbeddedBattleServer.StartIfMissing); err != nil {
		return internal.Game{}, false, err
	}
	return gameCfg, found, nil
}

// applyGameOverrides loads each enabled game's config.<id>.toml and applies the
// startBsIfMissing flag. It runs after initConfig, so the global document is
// already decoded and the flag values have already been consumed.
func applyGameOverrides(cfg *internal.Configuration, usedFile string, fs *pflag.FlagSet,
	values *server.Values, gameSet mapset.Set[string]) error {

	if values != nil && values.GameConfig != "" && gameSet.Cardinality() != 1 {
		return fmt.Errorf("--gameConfig is only valid with a single enabled game")
	}
	flagStart := fs != nil && fs.Changed("startBsIfMissing")
	gameIDs := gameSet.ToSlice()
	slices.Sort(gameIDs)
	for _, gameID := range gameIDs {
		basePtr := cfg.Games.Get(gameID)
		if basePtr == nil {
			continue
		}
		base := *basePtr
		gameCfg, found, err := loadGameConfig(base, gameID, usedFile, values)
		if err != nil {
			return fmt.Errorf("%s: %w", gameID, err)
		}
		if found && len(base.BattleServers) > 0 && !slices.Equal(base.BattleServers, gameCfg.BattleServers) {
			logger.Warn("config.%s.toml replaces the legacy [Games.%s] battle servers of config.toml", gameID, gameID)
		} else if !found && len(base.BattleServers) > 0 {
			logger.Warn("[Games.%s] in config.toml is deprecated; migrate to config.%s.toml", gameID, gameID)
		}
		if found && base.EmbeddedBattleServer != (internal.EmbeddedBattleServer{}) &&
			embeddedForCompare(base.EmbeddedBattleServer) != embeddedForCompare(gameCfg.EmbeddedBattleServer) {
			logger.Warn("config.%s.toml overrides [Games.%s].EmbeddedBattleServer of config.toml field by field", gameID, gameID)
		}
		if flagStart {
			newStart := strings.ToLower(strings.TrimSpace(values.StartBsIfMissing))
			if err = validateStartIfMissing(newStart); err != nil {
				return err
			}
			if gameCfg.EmbeddedBattleServer.StartIfMissing != newStart {
				logger.Warn("--startBsIfMissing overrides config.%s.toml for %s", gameID, gameID)
			}
			gameCfg.EmbeddedBattleServer.StartIfMissing = newStart
		}
		cfg.Games.Set(gameID, gameCfg)
	}
	return nil
}
