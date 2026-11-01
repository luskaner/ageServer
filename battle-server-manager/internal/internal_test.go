package internal

import (
	"testing"

	"github.com/luskaner/ageServer/common"
)

func TestErrorConstantsAreSequential(t *testing.T) {
	// All error constants must be unique and sequential starting from ErrLast.
	expected := []int{
		common.ErrLast,
		common.ErrLast + 1,
		common.ErrLast + 2,
		common.ErrLast + 3,
		common.ErrLast + 4,
		common.ErrLast + 5,
		common.ErrLast + 6,
		common.ErrLast + 7,
		common.ErrLast + 8,
		common.ErrLast + 9,
		common.ErrLast + 10,
		common.ErrLast + 11,
		common.ErrLast + 12,
		common.ErrLast + 13,
		common.ErrLast + 14,
		common.ErrLast + 15,
	}
	actual := []int{
		ErrGames,
		ErrReadConfig,
		ErrAlreadyRunning,
		ErrAlreadyExists,
		ErrResolveHost,
		ErrInvalidHost,
		ErrBsPortInUse,
		ErrWsPortInUse,
		ErrOobPortInUse,
		ErrGenPorts,
		ErrResolveSSLFiles,
		ErrResolvePath,
		ErrParseArgs,
		ErrStartBattleServer,
		ErrInitBattleServer,
		ErrConfigWrite,
	}
	if len(actual) != len(expected) {
		t.Fatalf("error count = %d, want %d", len(actual), len(expected))
	}
	for i, v := range actual {
		if v != expected[i] {
			t.Errorf("error[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestErrorConstantsAreUnique(t *testing.T) {
	seen := map[int]string{
		ErrGames:             "ErrGames",
		ErrReadConfig:        "ErrReadConfig",
		ErrAlreadyRunning:    "ErrAlreadyRunning",
		ErrAlreadyExists:     "ErrAlreadyExists",
		ErrResolveHost:       "ErrResolveHost",
		ErrInvalidHost:       "ErrInvalidHost",
		ErrBsPortInUse:       "ErrBsPortInUse",
		ErrWsPortInUse:       "ErrWsPortInUse",
		ErrOobPortInUse:      "ErrOobPortInUse",
		ErrGenPorts:          "ErrGenPorts",
		ErrResolveSSLFiles:   "ErrResolveSSLFiles",
		ErrResolvePath:       "ErrResolvePath",
		ErrParseArgs:         "ErrParseArgs",
		ErrStartBattleServer: "ErrStartBattleServer",
		ErrInitBattleServer:  "ErrInitBattleServer",
		ErrConfigWrite:       "ErrConfigWrite",
	}
	counts := map[int]int{}
	for v := range seen {
		counts[v]++
	}
	for v, count := range counts {
		if count > 1 {
			t.Errorf("value %d (%s) appears %d times", v, seen[v], count)
		}
	}
}

func TestConfigurationFields(t *testing.T) {
	c := Configuration{
		Region:    "eu",
		Name:      "TestServer",
		Host:      "192.168.1.1",
		CertsPath: "/certs",
		Executable: Executable{
			Path:      "/path/to/bs",
			ExtraArgs: []string{"--verbose"},
		},
		Ports: Ports{
			Bs:        27015,
			WebSocket: 27016,
			OutOfBand: 27017,
		},
	}
	if c.Region != "eu" || c.Name != "TestServer" || c.Host != "192.168.1.1" {
		t.Error("basic fields mismatch")
	}
	if c.Executable.Path != "/path/to/bs" || len(c.Executable.ExtraArgs) != 1 {
		t.Error("executable fields mismatch")
	}
	if c.Ports.Bs != 27015 || c.Ports.WebSocket != 27016 || c.Ports.OutOfBand != 27017 {
		t.Error("ports fields mismatch")
	}
}
