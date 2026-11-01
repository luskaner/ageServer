package logger

import (
	"fmt"
	"os"
	"time"

	commonLogger "github.com/luskaner/ageServer/common/logger"
	"github.com/luskaner/ageServer/launcher-common/cmdlog"
)

var StartTime time.Time

func init() {
	StartTime = time.Now().UTC()
}

func OpenMainFileLog(root string, logEnabled bool) error {
	if logEnabled {
		err := commonLogger.NewOwnFileLogger("server", root, "", true)
		if err != nil {
			return err
		}
	}
	return nil
}

func PrintFile(name string, path string) {
	if commonLogger.FileLogger != nil && path != "" {
		data, _ := os.ReadFile(path)
		commonLogger.PrefixPrintln(name, string(data))
	}
}

func Printf(format string, a ...any) {
	commonLogger.PrefixPrintf("main", format, a...)
	fmt.Printf(format, a...)
}

func Println(a ...any) {
	commonLogger.PrefixPrintln("main", a...)
	fmt.Println(a...)
}

// The styled renderers live in launcher-common/cmdlog so that every program in the
// workspace prints through the same code and the same rules. They are re-exported
// here because the rest of this module already speaks to this package.
func Ok(format string, a ...any)     { cmdlog.Ok(format, a...) }
func Fail(format string, a ...any)   { cmdlog.Fail(format, a...) }
func Warn(format string, a ...any)   { cmdlog.Warn(format, a...) }
func Info(format string, a ...any)   { cmdlog.Info(format, a...) }
func Step(format string, a ...any)   { cmdlog.Step(format, a...) }
func Detail(format string, a ...any) { cmdlog.Detail(format, a...) }
func Fault(format string, a ...any)  { cmdlog.Fault(format, a...) }
