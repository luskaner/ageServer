package main

// ConfigFile is the bridge between Go and the React editor.
type ConfigFile struct {
	Path             string            `json:"path"`
	Content          string            `json:"content"`
	Config           *Configuration    `json:"config"`
	ValidationErrors []ValidationError `json:"validationErrors"`
}

type InternetConfiguration struct {
	Enabled bool   `toml:"Enabled" json:"Enabled"`
	IP      string `toml:"IP" json:"IP"`
}

type SignConfig struct {
	Enabled                  bool     `toml:"Enabled" json:"Enabled"`
	PlatformUserIdExceptions []string `toml:"PlatformUserIdExceptions" json:"PlatformUserIdExceptions"`
}

type Configuration struct {
	Log                    bool                             `toml:"Log" json:"Log"`
	GeneratePlatformUserID bool                             `toml:"GeneratePlatformUserId" json:"GeneratePlatformUserId"`
	Sign                   SignConfig                       `toml:"Sign" json:"Sign"`
	Authentication         string                           `toml:"Authentication" json:"Authentication"`
	Internet               InternetConfiguration            `toml:"Internet" json:"Internet"`
	Games                  GamesConfiguration               `toml:"Games" json:"Games"`
	Announcement           AnnouncementConfig               `toml:"Announcement" json:"Announcement"`
	CustomFiles            CustomFilesConfiguration         `toml:"CustomFiles" json:"CustomFiles"`
	BattleServerReplays    BattleServerReplaysConfiguration `toml:"BattleServerReplays" json:"BattleServerReplays"`
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
	StartIfMissing   string `toml:"StartIfMissing" json:"StartIfMissing"`
	Region           string `toml:"Region" json:"Region"`
	Name             string `toml:"Name" json:"Name"`
	IPv4             string `toml:"IPv4" json:"IPv4"`
	BsPort           int    `toml:"BsPort" json:"BsPort"`
	WebSocketPort    int    `toml:"WebSocketPort" json:"WebSocketPort"`
	OutOfBandPort    int    `toml:"OutOfBandPort" json:"OutOfBandPort"`
	LogLevel         string `toml:"LogLevel" json:"LogLevel"`
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

// BattleServerReplaysConfiguration configures the optional recording of the
// data the embedded battle server receives for a match. It mirrors CustomFiles:
// sizes and the TTL are kept as strings and validated once at startup.
type BattleServerReplaysConfiguration struct {
	Enabled      bool   `toml:"Enabled" json:"Enabled"`
	Folder       string `toml:"Folder" json:"Folder"`
	MaxFileSize  string `toml:"MaxFileSize" json:"MaxFileSize"`
	MaxTotalSize string `toml:"MaxTotalSize" json:"MaxTotalSize"`
	Expiry       string `toml:"Expiry" json:"Expiry"`
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
