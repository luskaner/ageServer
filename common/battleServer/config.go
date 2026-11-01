package battleServer

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executables"
	"github.com/luskaner/ageServer/common/process"
	"github.com/pelletier/go-toml/v2"
)

type Base struct {
	// Cannot be an UUID as it can be confused for a LAN one.
	Region string
	// Only used for common.GameAoE2
	Name          string
	IPv4          string
	BsPort        int
	WebSocketPort int
	// Used for all except common.GameAoE1
	OutOfBandPort int
}

type Config struct {
	Base  `toml:",inline"`
	PID   uint32
	index int `toml:"-"`
}

func ParseFileName(fileName string) (int, error) {
	index := executables.BaseNameNoExt(fileName)
	return strconv.Atoi(index)
}

func Folder(gameId string) string {
	return filepath.Join(os.TempDir(), common.Name, "battle-servers", gameId)
}

func Configs(gameId string, onlyValid bool, ignorePid bool) (configs []Config, err error) {
	folder := Folder(gameId)
	var entries []os.DirEntry
	entries, err = os.ReadDir(folder)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
			return
		}
		err = fmt.Errorf("error while reading battle servers config directory \"%s\": %v", folder, err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		index, parseErr := ParseFileName(entry.Name())
		if parseErr != nil {
			continue
		}
		var data []byte
		path := filepath.Join(folder, entry.Name())
		data, err = os.ReadFile(path)
		if err != nil {
			err = fmt.Errorf("error while reading battle server config file \"%s\": %v", entry.Name(), err)
			return
		}
		var config Config
		if err = toml.Unmarshal(data, &config); err != nil {
			err = fmt.Errorf("error while parsing battle server config file \"%s\": %v", entry.Name(), err)
			return
		}
		if !onlyValid || config.Validate(ignorePid) {
			config.index = index
			configs = append(configs, config)
		}
	}
	return
}

func (c Config) Validate(ignorePid bool) bool {
	return c.ValidateWith(ignorePid, nil, nil)
}

type ProcessFinder func(pid int) (*os.Process, error)
type PortDialer func(network, address string, timeout time.Duration) (net.Conn, error)

func (c Config) ValidateWith(ignorePid bool, findProc ProcessFinder, dial PortDialer) bool {
	if c.Region == "" || (c.PID == 0 && !ignorePid) || c.IPv4 == "" || c.BsPort == 0 || c.WebSocketPort == 0 {
		return false
	}
	if !ignorePid {
		if findProc == nil {
			findProc = process.FindProcess
		}
		proc, err := findProc(int(c.PID))
		if err != nil || proc == nil {
			return false
		}
	}
	ports := []int{c.BsPort, c.WebSocketPort}
	if c.OutOfBandPort != 0 {
		ports = append(ports, c.OutOfBandPort)
	}
	IPv4 := c.IPv4
	if IPv4 == "auto" {
		IPv4 = netip.IPv4Unspecified().String()
	}
	if dial == nil {
		dial = net.DialTimeout
	}
	for _, port := range ports {
		target := net.JoinHostPort(IPv4, strconv.Itoa(port))
		conn, portErr := dial("tcp4", target, 100*time.Millisecond)
		if portErr != nil {
			return false
		}
		_ = conn.Close()
	}
	return true
}

func (c Config) Path() string {
	return Name(c.index)
}

func Name(index int) string {
	return fmt.Sprintf("%d.toml", index)
}
