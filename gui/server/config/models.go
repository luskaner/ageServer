package main

// ConfigFile is the bridge between Go and the React editor.
type ConfigFile struct {
	Path             string            `json:"path"`
	Content          string            `json:"content"`
	GameFiles        map[string]string `json:"gameFiles"`
	Config           *Configuration    `json:"config"`
	ValidationErrors []ValidationError `json:"validationErrors"`
}

type InternetConfiguration struct {
	Enabled bool   `toml:"Enabled" json:"Enabled"`
	IP      string `toml:"IP" json:"IP"`
}

// HostsConfiguration is the global listener configuration: every listener
// serves every enabled game, and the list only selects the certificate mode it
// presents. The old per-game Hosts key survives in GameConfig so an existing
// document can still be read and migrated.
type HostsConfiguration struct {
	Modern []string `toml:"Modern" json:"Modern"`
	Legacy []string `toml:"Legacy" json:"Legacy"`
}

type SignConfig struct {
	Enabled                  bool     `toml:"Enabled" json:"Enabled"`
	PlatformUserIdExceptions []string `toml:"PlatformUserIdExceptions" json:"PlatformUserIdExceptions"`
}

type Configuration struct {
	Log                    bool                     `toml:"Log" json:"Log"`
	GeneratePlatformUserID bool                     `toml:"GeneratePlatformUserId" json:"GeneratePlatformUserId"`
	Hosts                  HostsConfiguration       `toml:"Hosts" json:"Hosts"`
	Sign                   SignConfig               `toml:"Sign" json:"Sign"`
	Authentication         string                   `toml:"Authentication" json:"Authentication"`
	Internet               InternetConfiguration    `toml:"Internet" json:"Internet"`
	Games                  GamesConfiguration       `toml:"Games" json:"Games"`
	Announcement           AnnouncementConfig       `toml:"Announcement" json:"Announcement"`
	CustomFiles            CustomFilesConfiguration `toml:"CustomFiles" json:"CustomFiles"`
	Replays                ReplaysConfiguration     `toml:"Replays" json:"Replays"`
	Mods                   ModsConfiguration        `toml:"Mods" json:"Mods"`
	Api                    ApiConfiguration         `toml:"Api" json:"Api"`
}

// ApiConfiguration is the master switch of the Age of Empires API tree. With
// it off the server answers those paths with 404 instead of the embedded
// library or the official service.
type ApiConfiguration struct {
	Enabled bool `toml:"Enabled" json:"Enabled"`
}

type GamesConfiguration struct {
	Enabled []string   `toml:"Enabled" json:"Enabled"`
	Age1    GameConfig `toml:"age1" json:"age1"`
	Age2    GameConfig `toml:"age2" json:"age2"`
	Age3    GameConfig `toml:"age3" json:"age3"`
	Age4    GameConfig `toml:"age4" json:"age4"`
	Athens  GameConfig `toml:"athens" json:"athens"`
}

type GameConfig struct {
	Hosts                []string                   `toml:"Hosts" json:"Hosts"`
	BattleServers        []BattleServerConfig       `toml:"BattleServers" json:"BattleServers"`
	EmbeddedBattleServer EmbeddedBattleServerConfig `toml:"EmbeddedBattleServer" json:"EmbeddedBattleServer"`
}

type EmbeddedBattleServerConfig struct {
	StartIfMissing string `toml:"StartIfMissing" json:"StartIfMissing"`
	Region         string `toml:"Region" json:"Region"`
	Name           string `toml:"Name" json:"Name"`
	IPv4           string `toml:"IPv4" json:"IPv4"`
	BsPort         int    `toml:"BsPort" json:"BsPort"`
	WebSocketPort  int    `toml:"WebSocketPort" json:"WebSocketPort"`
	OutOfBandPort  int    `toml:"OutOfBandPort" json:"OutOfBandPort"`
	LogLevel       string `toml:"LogLevel" json:"LogLevel"`
}

type BattleServerConfig struct {
	Region        string `toml:"Region" json:"Region"`
	Name          string `toml:"Name" json:"Name"`
	IPv4          string `toml:"IPv4" json:"IPv4"`
	BsPort        int    `toml:"BsPort" json:"BsPort"`
	WebSocketPort int    `toml:"WebSocketPort" json:"WebSocketPort"`
	OutOfBandPort int    `toml:"OutOfBandPort" json:"OutOfBandPort"`
}

type AnnouncementConfig struct {
	Enabled        bool   `toml:"Enabled" json:"Enabled"`
	Multicast      bool   `toml:"Multicast" json:"Multicast"`
	Port           int    `toml:"Port" json:"Port"`
	MulticastGroup string `toml:"MulticastGroup" json:"MulticastGroup"`
}

// CustomFilesConfiguration configures the /api/MatchContent/ endpoints
// (multiplayer savegame and replay upload/download). Sizes and the expiry are
// kept as strings and validated here, the same way the server does at startup.
type CustomFilesConfiguration struct {
	Enabled      bool   `toml:"Enabled" json:"Enabled"`
	Folder       string `toml:"Folder" json:"Folder"`
	MaxFileSize  string `toml:"MaxFileSize" json:"MaxFileSize"`
	MaxTotalSize string `toml:"MaxTotalSize" json:"MaxTotalSize"`
	Expiry       string `toml:"Expiry" json:"Expiry"`
}

// ReplaysConfiguration configures the replays the game uploads after a match.
// They are always stored under <resources>/responses/cloud/<game>/replays, so
// there is no folder setting. Sizes and the TTL are kept as strings and
// validated once at startup.
type ReplaysConfiguration struct {
	Enabled      bool   `toml:"Enabled" json:"Enabled"`
	MaxFileSize  string `toml:"MaxFileSize" json:"MaxFileSize"`
	MaxTotalSize string `toml:"MaxTotalSize" json:"MaxTotalSize"`
	Expiry       string `toml:"Expiry" json:"Expiry"`
}

// ModsConfiguration configures the mods API served on the Age of Empires
// hosts. The same ageAPIServer instance serves the /api/MatchContent/ endpoints
// with the [CustomFiles] settings, so the two sections are independent switches
// over one process. Sizes are kept as strings and validated here, like
// CustomFiles.
type ModsConfiguration struct {
	Enabled            bool   `toml:"Enabled" json:"Enabled"`
	DBPath             string `toml:"DBPath" json:"DBPath"`
	UploadDir          string `toml:"UploadDir" json:"UploadDir"`
	MaxUploadSize      string `toml:"MaxUploadSize" json:"MaxUploadSize"`
	RequireAuthForRead bool   `toml:"RequireAuthForRead" json:"RequireAuthForRead"`
}

// Severities carried by a ValidationError: an error blocks saving, a warning
// informs about a value that is valid but may not work as expected.
const (
	severityError   = "error"
	severityWarning = "warning"
)

type ValidationError struct {
	Field    string `json:"field"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}
