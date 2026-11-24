package launcher

type Executable struct {
	Path string   `koanf:"Executable"`
	Args []string `koanf:"ExecutableArgs"`
}

type ClientIsolation struct {
	Metadata string
	Profiles string
	Path     string
}

type Config struct {
	CanAddHost               bool
	CanBroadcastBattleServer string
	CanUseInternet           bool
	CanUseDialogs            bool
	Log                      bool
	SetupCommand             []string
	RevertCommand            []string
	Certificate              ConfigCertificate
}

type ConfigCertificate struct {
	CanTrustInPc   string
	CanTrustInGame bool
}

type BattleServerManager struct {
	Executable `koanf:",squash"`
	Run        string
}

// ServerApi configures the Age of Empires API separately from the root server.
//
// Host is '0.0.0.0' (the default) when the API is the one embedded in the root
// server, and a host or IPv4 address when a standalone API server answers it on
// port 443.
type ServerApi struct {
	Host string
}

type Server struct {
	Executable `koanf:",squash"`
	// Games lists the titles the server should serve when this launcher starts
	// it. Empty keeps the old single-game behaviour ('-e {Game}'). When set, the
	// launcher replaces the -e value in ExecutableArgs with one -e per game and
	// the server is no longer stopped automatically when the first launcher
	// exits.
	Games                    []string
	Start                    string
	Host                     string
	Stop                     string
	SingleAutoSelect         bool
	StartWithoutConfirmation bool
	AnnouncePorts            []int
	AnnounceMulticastGroups  []string
	BattleServerManager      BattleServerManager
	Api                      ServerApi
}

type Client struct {
	Executable `koanf:",squash"`
	Path       string
	Isolation  ClientIsolation
}

type Configuration struct {
	Config Config
	Server Server
	Client Client
}
