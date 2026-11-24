package ipc

import (
	"net"

	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/common/executables"
)

const (
	Revert byte = iota
	Setup
	Exit
)

const name = common.Name + `-` + executables.LauncherConfigAdminAgent

type (
	SetupCommand struct {
		IP                     net.IP
		MacOsExclusiveMappings bool
		CanUseInternet         bool
		Certificate            []byte
		GameId                 string
		// Api maps only the API domains (a standalone API server's) so a
		// session can point them apart from the root server's. RootOnly maps
		// every domain but them, completing the game's other pass. Both false
		// keep the whole AllHosts mapping. The two are mutually exclusive and
		// gob sends one bool per field, so an older agent ignores what it
		// lacks and maps everything: the message stays wire compatible.
		Api      bool
		RootOnly bool
	}
	RevertCommand struct {
		IPs         bool
		Certificate bool
	}
)
