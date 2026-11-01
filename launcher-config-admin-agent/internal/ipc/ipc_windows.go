package ipc

import (
	"fmt"
	"net"
	"os/user"

	"github.com/Microsoft/go-winio"
	"github.com/luskaner/ageServer/launcher-common/ipc"
)

func SetupServer() (listener net.Listener, err error) {
	var u *user.User
	u, err = user.Current()
	if err != nil {
		return nil, err
	}
	pc := &winio.PipeConfig{
		InputBufferSize:    1_024,
		OutputBufferSize:   1,
		SecurityDescriptor: fmt.Sprintf("D:P(A;;GA;;;%s)", u.Uid),
		MessageMode:        true,
	}
	return winio.ListenPipe(ipc.Path(), pc)
}

func RevertServer() {}
