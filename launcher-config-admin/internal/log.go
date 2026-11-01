package internal

import commonLogger "github.com/luskaner/ageServer/common/logger"

var Logger *commonLogger.Root

// Initialize creates the file logger. It returns the initialization error so
// callers can warn: a nil Logger makes every subsequent Buffer() call a
// silent no-op (logs are lost without any indication).
func Initialize(logRoot string) error {
	err, l := commonLogger.NewFile(logRoot, "", true)
	if err != nil {
		Logger = nil
		return err
	}
	Logger = l
	return nil
}
