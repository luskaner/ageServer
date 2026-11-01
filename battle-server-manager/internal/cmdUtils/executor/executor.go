package executor

import (
	"fmt"
	"os"

	"github.com/luskaner/ageServer/common/cmdlog"
	"github.com/luskaner/ageServer/common/executor/exec"
	"github.com/luskaner/ageServer/common/game"
	commonLogger "github.com/luskaner/ageServer/common/logger"
)

func ExecuteBattleServer(gameId string, path string, region string, name string, ports []int, certFile string,
	keyFile string, extraArgs []string, hideWindow bool, logRoot string) (pid uint32, err error) {
	if len(ports) < 2 {
		err = fmt.Errorf("ports slice too short, need at least 2, got %d", len(ports))
		return
	}
	// For AoE1, outOfBand is not used but caller still passes 3 entries with -1; ensure we handle 2 or 3
	if len(ports) == 2 {
		ports = append(ports, -1)
	}
	var simulationPeriod int
	switch gameId {
	case game.AoE1:
		simulationPeriod = 25
	case game.AoE3, game.AoM:
		simulationPeriod = 50
	case game.AoE2, game.AoE4:
		simulationPeriod = 125
	default:
		simulationPeriod = 125
	}
	bsPort := fmt.Sprintf("%d", ports[0])
	args := []string{
		"-region", region,
		"-name", name,
		"-publicPort", bsPort,
		"-relaybroadcastPort", "0",
		"-simulationPeriod", fmt.Sprintf("%d", simulationPeriod),
		"-bsPort", bsPort,
		"-webSocketPort", fmt.Sprintf("%d", ports[1]),
		"-sslCert", certFile,
		"-sslKey", keyFile,
	}
	if ports[2] != -1 {
		args = append(args, "-outOfBandPort", fmt.Sprintf("%d", ports[2]))
	}
	args = append(args, extraArgs...)
	options := exec.Options{
		File:           path,
		UseWorkingPath: true,
		ShowWindow:     !hideWindow,
		Args:           args,
		Pid:            true,
	}
	if hideWindow && logRoot != "" {
		var f *os.File
		if _, f, err = commonLogger.NewFileLogger("battle-server", logRoot, "", true); err != nil {
			return
		} else if f != nil {
			options.Stdout = f
			options.Stderr = f
		}
	}
	cmdlog.Step("Executing: %s", options)
	if result := execWithOptions(gameId, &options); result.Success() {
		pid = result.Pid
	} else {
		err = result.Err
	}
	return
}
