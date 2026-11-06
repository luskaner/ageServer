package internal

import (
	"github.com/luskaner/ageServer/common/battleServer"
)

type Announcement struct {
	Enabled        bool
	Multicast      bool
	MulticastGroup string
	Port           int
}

type BattleServer struct {
	battleServer.Base `koanf:",squash"`
}

// EmbeddedBattleServer configures a battle server that runs inside the age
// server process instead of as a separate BattleServer executable. It mirrors
// the options the external battle-server manager passes on the command line,
// except the simulation period: the game's own default is always used.
type EmbeddedBattleServer struct {
	// StartIfMissing controls when the embedded battle server starts if no
	// other battle server (configured or discovered) exists for this game.
	// Accepted values:
	//   "true" (default): always start if missing
	//   "false": never auto-start
	//   "required": start if missing for age4/athens, otherwise false
	StartIfMissing string
	Region         string
	Name           string
	IPv4           string
	BsPort         int
	WebSocketPort  int
	OutOfBandPort  int
	// LogLevel is one of none, error, warning, notice, info or debug.
	LogLevel string
}

type Game struct {
	Hosts                []string
	BattleServers        []BattleServer
	EmbeddedBattleServer EmbeddedBattleServer
}

type Games struct {
	Enabled []string
	Age1    Game
	Age2    Game
	Age3    Game
	Age4    Game
	Athens  Game
}

type Internet struct {
	Enabled bool
	IP      string
}

// CustomFiles configures the /api/MatchContent/ endpoints (multiplayer
// savegame and replay upload/download). Sizes and the TTL are kept as strings
// and validated once at startup, so a typo fails the boot instead of the first
// upload.
type CustomFiles struct {
	Enabled      bool
	Folder       string
	MaxFileSize  string
	MaxTotalSize string
	Expiry       string
}

// Replays configures the replays the game uploads after a match. They are
// always stored under <resources>/responses/cloud/<game>/replays, so there is
// no folder setting. It mirrors CustomFiles: sizes and the TTL are kept as
// strings and validated once at startup.
type Replays struct {
	Enabled      bool
	MaxFileSize  string
	MaxTotalSize string
	Expiry       string
}

type SignConfig struct {
	// Enabled makes the server re-sign the catalogues it serves instead of
	// replaying the signatures captured alongside them in resources/responses.
	// Off by default: the captured ones are what an unmodified client accepts.
	Enabled bool

	// PlatformUserIdExceptions is a list of platform user IDs for which signing
	// is disabled even when Enabled is true. This allows selective signing:
	// when Enabled is true, all users get signed responses except those listed here.
	PlatformUserIdExceptions []string
}

type Configuration struct {
	Log                    bool
	GeneratePlatformUserId bool
	Authentication         string
	Internet               Internet
	Announcement           Announcement
	Games                  Games
	CustomFiles            CustomFiles
	Replays                Replays
	Sign                   SignConfig
	Automatch              Automatch
}

// Automatch configures the Quick Match engine.
type Automatch struct {
	// AssumeQueue is the opt-in fallback used when a client asks for a pool
	// that the bundled automatchMaps.json does not know. Empty (the default)
	// rejects unknown matchTypes. "rm-solo" pairs them as 1v1 Random Map and
	// forces the resulting match to be unrated. See docs/matchmaking.md §4.1.
	AssumeQueue string
}

func (cfg *Configuration) GetGameHosts(gameId string) []string {
	switch gameId {
	case "age1":
		return cfg.Games.Age1.Hosts
	case "age2":
		return cfg.Games.Age2.Hosts
	case "age3":
		return cfg.Games.Age3.Hosts
	case "age4":
		return cfg.Games.Age4.Hosts
	case "athens":
		return cfg.Games.Athens.Hosts
	default:
		return nil
	}
}

func (cfg *Configuration) GetGameBattleServers(gameId string) []BattleServer {
	switch gameId {
	case "age1":
		return cfg.Games.Age1.BattleServers
	case "age2":
		return cfg.Games.Age2.BattleServers
	case "age3":
		return cfg.Games.Age3.BattleServers
	case "age4":
		return cfg.Games.Age4.BattleServers
	case "athens":
		return cfg.Games.Athens.BattleServers
	default:
		return nil
	}
}

func (cfg *Configuration) GetGameEmbeddedBattleServer(gameId string) EmbeddedBattleServer {
	switch gameId {
	case "age1":
		return cfg.Games.Age1.EmbeddedBattleServer
	case "age2":
		return cfg.Games.Age2.EmbeddedBattleServer
	case "age3":
		return cfg.Games.Age3.EmbeddedBattleServer
	case "age4":
		return cfg.Games.Age4.EmbeddedBattleServer
	case "athens":
		return cfg.Games.Athens.EmbeddedBattleServer
	default:
		return EmbeddedBattleServer{}
	}
}
