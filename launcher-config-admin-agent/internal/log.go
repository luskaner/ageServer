package internal

import (
	"os"

	"github.com/luskaner/ageServer/common"
	commonLogger "github.com/luskaner/ageServer/common/logger"
)

var (
	newOwnFileLoggerFn = commonLogger.NewOwnFileLogger
	osExitFn           = os.Exit
)

func InitializeOrExit(logRoot string) {
	if err := newOwnFileLoggerFn("config-admin-agent", logRoot, "", true); err != nil {
		osExitFn(common.ErrFileLog)
	}
}
