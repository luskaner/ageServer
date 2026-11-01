package cmd

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/luskaner/ageServer/common"
	commonLogger "github.com/luskaner/ageServer/common/logger"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
)

func runStopAgent(_ []string) (err error, exitCode int) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_, ok := <-sigs
		if ok {
			exitCode = common.ErrSignal
		}
	}()
	commonLogger.Println(commonUi.Step("Stopping agent if needed..."))
	_ = stopAgentIfNeededFn()
	return
}
