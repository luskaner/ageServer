//go:build !windows

package ipc

import (
	"net"
	"os"

	"github.com/luskaner/ageServer/launcher-common/ipc"
)

func SetupServer() (listener net.Listener, err error) {
	ipcPath := ipc.Path()

	if err = os.Remove(ipcPath); err != nil && !os.IsNotExist(err) {
		return
	}

	listener, err = net.Listen("unix", ipcPath)

	if err != nil {
		return
	}

	err = os.Chmod(ipcPath, 0666)
	return
}

func RevertServer() {
	_ = os.Remove(ipc.Path())
}
