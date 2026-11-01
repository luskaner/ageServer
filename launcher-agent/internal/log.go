package internal

import commonLogger "github.com/luskaner/ageServer/common/logger"

var Logger *commonLogger.Root

func Initialize(logRoot string) {
	_, Logger = commonLogger.NewFile(logRoot, "", true)
}
