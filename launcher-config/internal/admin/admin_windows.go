package admin

import (
	"errors"
	"net"
	"os"

	"github.com/Microsoft/go-winio"
	"github.com/luskaner/ageServer/common/logger"
	commonIpc "github.com/luskaner/ageServer/launcher-common/ipc"
	commonUi "github.com/luskaner/ageServer/launcher-common/ui"
	"golang.org/x/sys/windows"
)

// isAccessDenied reports whether err is the privilege failure we get when a
// non-elevated process tries to terminate an elevated one.
func isAccessDenied(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, os.ErrPermission)
}

func DialIPC() (net.Conn, error) {
	path := commonIpc.Path()
	commonLogger.Println(commonUi.Detail("Using %s", path))
	return winio.DialPipe(path, nil)
}
