// Package battleconfig provides shared battle server configuration logic
// used by both the embedded battle server and the battle-server-manager.
package battleconfig

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"

	"github.com/luskaner/ageBattleServer"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/game"
)

// SimulationPeriodDefaults maps game IDs to their default simulation periods in milliseconds.
var SimulationPeriodDefaults = map[string]float64{
	game.AoE1: 25,
	game.AoE3: 50,
	game.AoM:  50,
	game.AoE2: 125,
	game.AoE4: 125,
}

// GetDefaultSimulationPeriod returns the default simulation period for a game.
// Returns 125ms for unknown games.
func GetDefaultSimulationPeriod(gameID string) float64 {
	if period, ok := SimulationPeriodDefaults[gameID]; ok {
		return period
	}
	return 125
}

// TLSFiles returns the resolved certificate and key file paths for a game.
// For age4 and athens (AoM), uses cert.pem/key.pem.
// For other games, uses selfsigned_cert.pem/selfsigned_key.pem.
// If certsPath is "auto", it finds the server executable to locate the certs.
func TLSFiles(gameID, certsPath string) (certFile, keyFile string, err error) {
	if certsPath == "auto" {
		serverExe := executables.FindPath(executables.NativeFileName(true, executables.Server))
		if serverExe == "" {
			return "", "", fmt.Errorf("could not find server executable")
		}
		certsPath = common.CertificatePairFolder(serverExe)
	}
	return TLSFilesFromFolder(gameID, certsPath)
}

// TLSFilesFromFolder returns the resolved certificate and key file paths from a given folder.
func TLSFilesFromFolder(gameID, certsPath string) (certFile, keyFile string, err error) {
	ok, cert, key, _, selfSignedCert, selfSignedKey := common.CertificatePairs(certsPath)
	if !ok {
		return "", "", fmt.Errorf("no SSL certificate and keys found in %q", certsPath)
	}
	if gameID == game.AoE4 || gameID == game.AoM {
		certFile = cert
		keyFile = key
	} else {
		certFile = selfSignedCert
		keyFile = selfSignedKey
	}
	if _, err := os.Stat(certFile); err != nil {
		return "", "", fmt.Errorf("certificate %q not found", certFile)
	}
	if _, err := os.Stat(keyFile); err != nil {
		return "", "", fmt.Errorf("key %q not found", keyFile)
	}
	return certFile, keyFile, nil
}

// ResolveIPv4 resolves the IPv4 address for a battle server.
// If ip is "auto", returns the unspecified IPv4 address (0.0.0.0).
// Otherwise validates that the IP is a valid IPv4 address.
func ResolveIPv4(ip string) (string, error) {
	if ip == "auto" {
		return netip.IPv4Unspecified().String(), nil
	}
	if parsed := net.ParseIP(ip); parsed == nil || parsed.To4() == nil {
		return "", fmt.Errorf("invalid IPv4 %q", ip)
	}
	return ip, nil
}

// GameSupportsOutOfBand returns true if the game uses an out-of-band port.
// Only AoE1 does not use out-of-band.
func GameSupportsOutOfBand(gameID string) bool {
	return gameID != game.AoE1
}

// PortConfig holds the three ports for a battle server.
type PortConfig struct {
	BsPort        int
	WebSocketPort int
	OutOfBandPort int
}

// AllocatePorts allocates free ports for any zero-valued ports in the config.
// Returns the updated PortConfig with all ports assigned.
func AllocatePorts(ports PortConfig) (PortConfig, error) {
	if ports.BsPort == 0 {
		p, err := GetFreePort()
		if err != nil {
			return ports, fmt.Errorf("auto-assign BsPort: %w", err)
		}
		ports.BsPort = p
	}
	if ports.WebSocketPort == 0 {
		p, err := GetFreePort()
		if err != nil {
			return ports, fmt.Errorf("auto-assign WebSocketPort: %w", err)
		}
		ports.WebSocketPort = p
	}
	if !GameSupportsOutOfBand("") { // empty string means we don't know the game yet
		ports.OutOfBandPort = 0
	} else if ports.OutOfBandPort == 0 {
		p, err := GetFreePort()
		if err != nil {
			return ports, fmt.Errorf("auto-assign OutOfBandPort: %w", err)
		}
		ports.OutOfBandPort = p
	}
	return ports, nil
}

// AllocatePortsForGame allocates free ports for a specific game.
// For AoE1, OutOfBandPort is always set to 0.
func AllocatePortsForGame(gameID string, ports PortConfig) (PortConfig, error) {
	if ports.BsPort == 0 {
		p, err := GetFreePort()
		if err != nil {
			return ports, fmt.Errorf("auto-assign BsPort: %w", err)
		}
		ports.BsPort = p
	}
	if ports.WebSocketPort == 0 {
		p, err := GetFreePort()
		if err != nil {
			return ports, fmt.Errorf("auto-assign WebSocketPort: %w", err)
		}
		ports.WebSocketPort = p
	}
	if gameID == game.AoE1 {
		ports.OutOfBandPort = 0
	} else if ports.OutOfBandPort == 0 {
		p, err := GetFreePort()
		if err != nil {
			return ports, fmt.Errorf("auto-assign OutOfBandPort: %w", err)
		}
		ports.OutOfBandPort = p
	}
	return ports, nil
}

// GetFreePort binds to port 0 to get a free port from the OS.
func GetFreePort() (int, error) {
	addr := net.JoinHostPort(netip.IPv4Unspecified().String(), "0")
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port, nil
}

// ValidatePorts validates that all ports are in the valid range 0-65535.
func ValidatePorts(ports PortConfig) error {
	if ports.BsPort < 0 || ports.BsPort > 65535 {
		return fmt.Errorf("BsPort must be 0..65535")
	}
	if ports.WebSocketPort < 0 || ports.WebSocketPort > 65535 {
		return fmt.Errorf("WebSocketPort must be 0..65535")
	}
	if ports.OutOfBandPort < 0 || ports.OutOfBandPort > 65535 {
		return fmt.Errorf("OutOfBandPort must be 0..65535")
	}
	return nil
}

// CheckPortsAvailable checks if the given ports are available for binding.
func CheckPortsAvailable(ports PortConfig) error {
	for _, port := range []int{ports.BsPort, ports.WebSocketPort} {
		if port > 0 && !IsPortAvailable(port) {
			return fmt.Errorf("port %d is already in use", port)
		}
	}
	if ports.OutOfBandPort > 0 && !IsPortAvailable(ports.OutOfBandPort) {
		return fmt.Errorf("out of band port %d is already in use", ports.OutOfBandPort)
	}
	return nil
}

// IsPortAvailable checks if a TCP port is available for binding.
func IsPortAvailable(port int) bool {
	address := net.JoinHostPort(netip.IPv4Unspecified().String(), strconv.Itoa(port))
	l, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// ConfigBuilder helps build a battleServer.Config with common settings.
type ConfigBuilder struct {
	GameID           string
	Region           string
	Name             string
	IPv4             string
	Ports            PortConfig
	SimulationPeriod float64
	LogLevel         string
	CertFile         string
	KeyFile          string
}

// BuildConfig creates a battleserver.Config with the specified settings.
// Applies defaults for missing values.
func (b *ConfigBuilder) BuildConfig() (*battleserver.Config, error) {
	// Validate and set defaults
	if b.Region == "" {
		return nil, fmt.Errorf("region is required")
	}
	if b.IPv4 == "" {
		b.IPv4 = "auto"
	}

	// Resolve IPv4
	_, err := ResolveIPv4(b.IPv4)
	if err != nil {
		return nil, err
	}

	// Allocate ports for this game
	ports, err := AllocatePortsForGame(b.GameID, b.Ports)
	if err != nil {
		return nil, err
	}

	// Validate ports
	if err := ValidatePorts(ports); err != nil {
		return nil, err
	}

	// Set simulation period default
	simPeriod := b.SimulationPeriod
	if simPeriod == 0 {
		simPeriod = GetDefaultSimulationPeriod(b.GameID)
	}

	// Build config
	cfg := battleserver.DefaultConfig()
	cfg.Name = b.Name
	cfg.BsPort = uint(ports.BsPort)
	cfg.WebSocketPort = uint(ports.WebSocketPort)
	cfg.OutOfBandPort = uint(ports.OutOfBandPort)
	cfg.RelayBroadcastPorts = nil
	cfg.LobbyHost = ""
	cfg.LobbyPort = 0
	cfg.PublicIP = ""
	cfg.PublicPort = uint(ports.BsPort)
	cfg.SSLCert = b.CertFile
	cfg.SSLKey = b.KeyFile
	cfg.UploadVia = ""
	cfg.ReplayHost = ""
	cfg.ReplayPort = 0
	cfg.Region = b.Region
	cfg.SimulationPeriod = simPeriod
	cfg.LogLevel = b.LogLevel
	cfg.LogModLevel = ""

	return &cfg, nil
}

// CLIArgsBuilder builds command-line arguments for the battle server executable.
type CLIArgsBuilder struct {
	GameID           string
	Region           string
	Name             string
	Ports            PortConfig
	SimulationPeriod int
	CertFile         string
	KeyFile          string
	ExtraArgs        []string
}

// BuildArgs constructs the command-line arguments for the battle server executable.
func (b *CLIArgsBuilder) BuildArgs() []string {
	bsPort := strconv.Itoa(b.Ports.BsPort)
	args := []string{
		"-region", b.Region,
		"-name", b.Name,
		"-publicPort", bsPort,
		"-relaybroadcastPort", "0",
		"-simulationPeriod", strconv.Itoa(b.SimulationPeriod),
		"-bsPort", bsPort,
		"-webSocketPort", strconv.Itoa(b.Ports.WebSocketPort),
		"-sslCert", b.CertFile,
		"-sslKey", b.KeyFile,
	}
	if GameSupportsOutOfBand(b.GameID) {
		args = append(args, "-outOfBandPort", strconv.Itoa(b.Ports.OutOfBandPort))
	}
	args = append(args, b.ExtraArgs...)
	return args
}

// WithDefaults applies default values to the builder.
func (b *CLIArgsBuilder) WithDefaults() {
	if b.SimulationPeriod == 0 {
		b.SimulationPeriod = int(GetDefaultSimulationPeriod(b.GameID))
	}
}
